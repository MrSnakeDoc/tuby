package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrChannelNotFound = errors.New("channel not found")

type ChannelSync struct {
	Channel            Channel
	Videos             []Video
	NotificationEvents map[string]NotificationEvent
	ETag               string
	Modified           string
	PurgeBefore        *time.Time
}

type SyncResult struct {
	InsertedVideos []Video
}

func (s *Store) Channel(ctx context.Context, channelID string) (Channel, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT youtube_id, source_url, title, avatar_url, feed_etag, feed_last_modified,
       last_checked_at, last_successful_sync_at, next_check_at, failure_count,
       last_error, include_long, include_shorts, notification_level, notification_enabled_at,
       category_id, position, created_at, updated_at
FROM channels WHERE youtube_id = ?`, channelID)
	channel, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, fmt.Errorf("%w: %s", ErrChannelNotFound, channelID)
	}
	if err != nil {
		return Channel{}, fmt.Errorf("load channel %q: %w", channelID, err)
	}
	return channel, nil
}

func (s *Store) Channels(ctx context.Context) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT youtube_id, source_url, title, avatar_url, feed_etag, feed_last_modified,
       last_checked_at, last_successful_sync_at, next_check_at, failure_count,
       last_error, include_long, include_shorts, notification_level, notification_enabled_at,
       category_id, position, created_at, updated_at
FROM channels ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	channels := make([]Channel, 0)
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return channels, nil
}

func (s *Store) DeleteChannel(ctx context.Context, channelID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE youtube_id = ?`, channelID)
	if err != nil {
		return fmt.Errorf("delete channel %q: %w", channelID, err)
	}
	return requireChannel(result, channelID)
}

func (s *Store) SaveSync(ctx context.Context, sync ChannelSync) error {
	_, err := s.SaveSyncWithResult(ctx, sync)
	return err
}

func (s *Store) SaveSyncWithResult(ctx context.Context, sync ChannelSync) (SyncResult, error) {
	sync.Channel = channelWithDefaultCategory(sync.Channel)
	if !sync.Channel.IncludeLong && !sync.Channel.IncludeShorts {
		sync.Channel.IncludeLong = true
	}
	if err := validateChannel(sync.Channel); err != nil {
		return SyncResult{}, err
	}
	if err := validateSyncVideos(sync.Videos, sync.Channel.YouTubeID); err != nil {
		return SyncResult{}, err
	}
	if err := validateNotificationEvents(sync.NotificationEvents, sync.Videos, sync.Channel.YouTubeID); err != nil {
		return SyncResult{}, err
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncResult{}, fmt.Errorf("begin channel sync: %w", err)
	}
	inserted, err := saveSync(transaction, sync, time.Now().UTC())
	if err != nil {
		_ = transaction.Rollback()
		return SyncResult{}, err
	}
	if err := transaction.Commit(); err != nil {
		return SyncResult{}, fmt.Errorf("commit channel sync: %w", err)
	}
	return SyncResult{InsertedVideos: inserted}, nil
}

func (s *Store) RecordSyncFailure(ctx context.Context, channelID string, cause error, nextCheck time.Time) error {
	if cause == nil {
		return fmt.Errorf("sync failure cause must not be nil")
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
UPDATE channels SET last_checked_at = ?, next_check_at = ?, failure_count = failure_count + 1,
    last_error = ?, updated_at = ?
WHERE youtube_id = ?`, formatTime(now), formatTime(nextCheck), cause.Error(), formatTime(now), channelID)
	if err != nil {
		return fmt.Errorf("record sync failure for channel %q: %w", channelID, err)
	}
	return requireChannel(result, channelID)
}

func (s *Store) RecordNotModified(ctx context.Context, channelID, etag, modified string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
UPDATE channels SET feed_etag = ?, feed_last_modified = ?, last_checked_at = ?,
    last_successful_sync_at = ?, next_check_at = NULL, failure_count = 0,
    last_error = '', updated_at = ?
WHERE youtube_id = ?`, etag, modified, formatTime(now), formatTime(now), formatTime(now), channelID)
	if err != nil {
		return fmt.Errorf("record unmodified sync for channel %q: %w", channelID, err)
	}
	return requireChannel(result, channelID)
}

func saveSync(transaction *sql.Tx, sync ChannelSync, now time.Time) ([]Video, error) {
	channel := channelWithTimestamps(sync.Channel, now)
	_, err := transaction.Exec(`
INSERT INTO channels(youtube_id, source_url, title, avatar_url, feed_etag, feed_last_modified,
    last_checked_at, last_successful_sync_at, next_check_at, failure_count, last_error,
    include_long, include_shorts, category_id, position, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, NULL, 0, '', ?, ?, ?,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM channels WHERE category_id = ?), ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET source_url = excluded.source_url, title = excluded.title,
    avatar_url = excluded.avatar_url, feed_etag = excluded.feed_etag,
    feed_last_modified = excluded.feed_last_modified, last_checked_at = excluded.last_checked_at,
    last_successful_sync_at = excluded.last_successful_sync_at, next_check_at = NULL,
    failure_count = 0, last_error = '', updated_at = excluded.updated_at`,
		channel.YouTubeID, channel.SourceURL, channel.Title, channel.AvatarURL, sync.ETag,
		sync.Modified, formatTime(now), formatTime(now), channel.IncludeLong, channel.IncludeShorts,
		channel.CategoryID, channel.CategoryID, formatTime(channel.CreatedAt), formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("save channel %q during sync: %w", channel.YouTubeID, err)
	}
	insertedVideos := make([]Video, 0)
	for _, video := range sync.Videos {
		inserted, err := saveSyncVideo(transaction, video, now)
		if err != nil {
			return nil, err
		}
		if inserted {
			insertedVideos = append(insertedVideos, video)
		}
		event, notify := sync.NotificationEvents[video.YouTubeID]
		if inserted && notify {
			if err := insertOutboxNotification(transaction, event, now); err != nil {
				return nil, err
			}
		}
	}
	if err := purgeDisabledVideos(transaction, channel); err != nil {
		return nil, err
	}
	if sync.PurgeBefore != nil {
		if err := purgeStaleVideos(transaction, channel.YouTubeID, *sync.PurgeBefore); err != nil {
			return nil, err
		}
	}
	return insertedVideos, nil
}

func purgeDisabledVideos(transaction *sql.Tx, channel Channel) error {
	_, err := transaction.Exec(`
DELETE FROM videos
WHERE channel_youtube_id = ? AND ((is_short = 1 AND ? = 0) OR (is_short = 0 AND ? = 0))`,
		channel.YouTubeID, channel.IncludeShorts, channel.IncludeLong)
	if err != nil {
		return fmt.Errorf("purge disabled videos for channel %q: %w", channel.YouTubeID, err)
	}
	return nil
}

func purgeStaleVideos(transaction *sql.Tx, channelID string, cutoff time.Time) error {
	_, err := transaction.Exec(`
DELETE FROM videos
WHERE channel_youtube_id = ? AND last_seen_at < ? AND unseen = 0 AND watch_later = 0`,
		channelID, formatTime(cutoff))
	if err != nil {
		return fmt.Errorf("purge stale videos for channel %q: %w", channelID, err)
	}
	return nil
}

func saveSyncVideo(transaction *sql.Tx, video Video, now time.Time) (bool, error) {
	video = videoWithTimestamps(video, now)
	result, err := transaction.Exec(`
INSERT INTO videos(youtube_id, channel_youtube_id, title, url, thumbnail_url, published_at, discovered_at, last_seen_at, is_short)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(youtube_id) DO NOTHING`,
		video.YouTubeID, video.ChannelYouTubeID, video.Title, video.URL, video.ThumbnailURL,
		formatTime(video.PublishedAt), formatTime(video.DiscoveredAt), formatTime(video.LastSeenAt), video.IsShort)
	if err != nil {
		return false, fmt.Errorf("insert video %q during sync: %w", video.YouTubeID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check video %q insertion: %w", video.YouTubeID, err)
	}
	if rowsAffected == 1 {
		return true, nil
	}
	_, err = transaction.Exec(`
UPDATE videos SET title = ?, url = ?, thumbnail_url = ?, published_at = ?,
    last_seen_at = ?, is_short = ? WHERE youtube_id = ?`,
		video.Title, video.URL, video.ThumbnailURL, formatTime(video.PublishedAt),
		formatTime(video.LastSeenAt), video.IsShort, video.YouTubeID)
	if err != nil {
		return false, fmt.Errorf("update video %q during sync: %w", video.YouTubeID, err)
	}
	return false, nil
}

func validateNotificationEvents(events map[string]NotificationEvent, videos []Video, channelID string) error {
	videoIDs := make(map[string]struct{}, len(videos))
	for _, video := range videos {
		videoIDs[video.YouTubeID] = struct{}{}
	}
	for videoID, event := range events {
		if videoID != event.VideoYouTubeID {
			return fmt.Errorf("notification event key does not match video %q", event.VideoYouTubeID)
		}
		if _, exists := videoIDs[videoID]; !exists {
			return fmt.Errorf("notification event references unknown video %q", videoID)
		}
		if err := validateNotificationEvent(event, channelID); err != nil {
			return err
		}
	}
	return nil
}

func validateSyncVideos(videos []Video, channelID string) error {
	for _, video := range videos {
		if err := validateVideo(video); err != nil {
			return err
		}
		if video.ChannelYouTubeID != channelID {
			return fmt.Errorf("video %q belongs to another channel", video.YouTubeID)
		}
	}
	return nil
}

func requireChannel(result sql.Result, channelID string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check channel update: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("%w: %s", ErrChannelNotFound, channelID)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanChannel(row rowScanner) (Channel, error) {
	var channel Channel
	var lastChecked, lastSuccessful, nextCheck, notificationEnabled sql.NullString
	var created, updated string
	err := row.Scan(&channel.YouTubeID, &channel.SourceURL, &channel.Title, &channel.AvatarURL,
		&channel.FeedETag, &channel.FeedLastModified, &lastChecked, &lastSuccessful, &nextCheck,
		&channel.FailureCount, &channel.LastError, &channel.IncludeLong, &channel.IncludeShorts,
		&channel.NotificationLevel, &notificationEnabled, &channel.CategoryID, &channel.Position, &created, &updated)
	if err != nil {
		return Channel{}, err
	}
	var parseErr error
	channel.LastCheckedAt, parseErr = parseOptionalTime(lastChecked)
	if parseErr != nil {
		return Channel{}, parseErr
	}
	channel.LastSuccessfulSync, parseErr = parseOptionalTime(lastSuccessful)
	if parseErr != nil {
		return Channel{}, parseErr
	}
	channel.NextCheckAt, parseErr = parseOptionalTime(nextCheck)
	if parseErr != nil {
		return Channel{}, parseErr
	}
	channel.NotificationEnabledAt, parseErr = parseOptionalTime(notificationEnabled)
	if parseErr != nil {
		return Channel{}, fmt.Errorf("parse notification activation time: %w", parseErr)
	}
	channel.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, created)
	if parseErr != nil {
		return Channel{}, fmt.Errorf("parse channel creation time: %w", parseErr)
	}
	channel.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updated)
	if parseErr != nil {
		return Channel{}, fmt.Errorf("parse channel update time: %w", parseErr)
	}
	return channel, nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}
	return &parsed, nil
}
