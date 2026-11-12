package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type NotificationVariant string

const (
	NotificationVariantVideo            NotificationVariant = "new_video"
	NotificationVariantVideoWithMessage NotificationVariant = "new_video_with_message"
)

type NotificationEvent struct {
	VideoYouTubeID   string
	ChannelYouTubeID string
	ChannelTitle     string
	VideoTitle       string
	Level            NotificationLevel
	Variant          NotificationVariant
	Message          string
}

type OutboxNotification struct {
	NotificationEvent
	ID           int64
	AttemptCount int
	AvailableAt  time.Time
	CreatedAt    time.Time
}

func (s *Store) PendingNotifications(ctx context.Context) ([]OutboxNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, video_youtube_id, channel_youtube_id, channel_title, video_title,
       notification_level, variant, message, attempt_count, available_at, created_at
FROM notification_outbox ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list notification outbox: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]OutboxNotification, 0)
	for rows.Next() {
		entry, err := scanOutboxNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *Store) NextNotification(ctx context.Context, now time.Time) (*OutboxNotification, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, video_youtube_id, channel_youtube_id, channel_title, video_title,
       notification_level, variant, message, attempt_count, available_at, created_at
FROM notification_outbox WHERE available_at <= ? ORDER BY available_at, id LIMIT 1`, formatTime(now))
	entry, err := scanOutboxNotification(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Store) RetryNotification(
	ctx context.Context,
	id int64,
	attemptCount int,
	availableAt time.Time,
) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE notification_outbox SET attempt_count = ?, available_at = ? WHERE id = ?`,
		attemptCount, formatTime(availableAt), id)
	if err != nil {
		return fmt.Errorf("reschedule notification %d: %w", id, err)
	}
	return requireOutboxEntry(result, id)
}

func (s *Store) DeleteNotification(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM notification_outbox WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete notification %d: %w", id, err)
	}
	return requireOutboxEntry(result, id)
}

func scanOutboxNotification(row interface{ Scan(...any) error }) (OutboxNotification, error) {
	var entry OutboxNotification
	var availableAt, createdAt string
	err := row.Scan(
		&entry.ID, &entry.VideoYouTubeID, &entry.ChannelYouTubeID, &entry.ChannelTitle,
		&entry.VideoTitle, &entry.Level, &entry.Variant, &entry.Message,
		&entry.AttemptCount, &availableAt, &createdAt,
	)
	if err != nil {
		return OutboxNotification{}, fmt.Errorf("scan notification outbox: %w", err)
	}
	entry.AvailableAt, err = time.Parse(time.RFC3339Nano, availableAt)
	if err != nil {
		return OutboxNotification{}, fmt.Errorf("parse notification availability: %w", err)
	}
	entry.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return OutboxNotification{}, fmt.Errorf("parse notification creation: %w", err)
	}
	return entry, nil
}

func validateNotificationEvent(event NotificationEvent, channelID string) error {
	if strings.TrimSpace(event.VideoYouTubeID) == "" || event.ChannelYouTubeID != channelID {
		return fmt.Errorf("notification event has invalid video or channel identity")
	}
	if strings.TrimSpace(event.ChannelTitle) == "" || strings.TrimSpace(event.VideoTitle) == "" {
		return fmt.Errorf("notification event titles must not be empty")
	}
	if event.Level == NotificationOff || !event.Level.Valid() {
		return fmt.Errorf("notification event has invalid level %q", event.Level)
	}
	switch event.Variant {
	case NotificationVariantVideo:
		if event.Message != "" {
			return fmt.Errorf("notification without message variant contains a message")
		}
	case NotificationVariantVideoWithMessage:
		if event.Message == "" {
			return fmt.Errorf("notification message variant has no message")
		}
	default:
		return fmt.Errorf("notification event has invalid variant %q", event.Variant)
	}
	return nil
}

func insertOutboxNotification(transaction *sql.Tx, event NotificationEvent, now time.Time) error {
	_, err := transaction.Exec(`
INSERT INTO notification_outbox(
    video_youtube_id, channel_youtube_id, channel_title, video_title,
    notification_level, variant, message, available_at, created_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(video_youtube_id) DO NOTHING`,
		event.VideoYouTubeID, event.ChannelYouTubeID, event.ChannelTitle, event.VideoTitle,
		event.Level, event.Variant, event.Message, formatTime(now), formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("enqueue notification for video %q: %w", event.VideoYouTubeID, err)
	}
	return nil
}

func requireOutboxEntry(result sql.Result, id int64) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check notification outbox update: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("notification outbox entry %d not found", id)
	}
	return nil
}
