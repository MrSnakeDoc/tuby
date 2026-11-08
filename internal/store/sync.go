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
	Channel     Channel
	Videos      []Video
	ETag        string
	Modified    string
	PurgeBefore *time.Time
}

func (s *Store) Channel(ctx context.Context, channelID string) (Channel, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT youtube_id, source_url, title, avatar_url, feed_etag, feed_last_modified,
       last_checked_at, last_successful_sync_at, next_check_at, failure_count,
       last_error, created_at, updated_at
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
       last_error, created_at, updated_at
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
	if err := validateChannel(sync.Channel); err != nil {
		return err
	}
	if err := validateSyncVideos(sync.Videos, sync.Channel.YouTubeID); err != nil {
		return err
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin channel sync: %w", err)
	}
	if err := saveSync(transaction, sync, time.Now().UTC()); err != nil {
		_ = transaction.Rollback()
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit channel sync: %w", err)
	}
	return nil
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

func saveSync(transaction *sql.Tx, sync ChannelSync, now time.Time) error {
	channel := channelWithTimestamps(sync.Channel, now)
	_, err := transaction.Exec(`
INSERT INTO channels(youtube_id, source_url, title, avatar_url, feed_etag, feed_last_modified,
    last_checked_at, last_successful_sync_at, next_check_at, failure_count, last_error, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, NULL, 0, '', ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET source_url = excluded.source_url, title = excluded.title,
    avatar_url = excluded.avatar_url, feed_etag = excluded.feed_etag,
    feed_last_modified = excluded.feed_last_modified, last_checked_at = excluded.last_checked_at,
    last_successful_sync_at = excluded.last_successful_sync_at, next_check_at = NULL,
    failure_count = 0, last_error = '', updated_at = excluded.updated_at`,
		channel.YouTubeID, channel.SourceURL, channel.Title, channel.AvatarURL, sync.ETag,
		sync.Modified, formatTime(now), formatTime(now), formatTime(channel.CreatedAt), formatTime(now))
	if err != nil {
		return fmt.Errorf("save channel %q during sync: %w", channel.YouTubeID, err)
	}
	for _, video := range sync.Videos {
		if err := saveSyncVideo(transaction, video, now); err != nil {
			return err
		}
	}
	if err := purgeShortVideos(transaction, channel.YouTubeID); err != nil {
		return err
	}
	if sync.PurgeBefore != nil {
		if err := purgeStaleVideos(transaction, channel.YouTubeID, *sync.PurgeBefore); err != nil {
			return err
		}
	}
	return nil
}

func purgeShortVideos(transaction *sql.Tx, channelID string) error {
	_, err := transaction.Exec(`DELETE FROM videos WHERE channel_youtube_id = ? AND url LIKE '%/shorts/%'`, channelID)
	if err != nil {
		return fmt.Errorf("purge Shorts for channel %q: %w", channelID, err)
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

func saveSyncVideo(transaction *sql.Tx, video Video, now time.Time) error {
	video = videoWithTimestamps(video, now)
	_, err := transaction.Exec(`
INSERT INTO videos(youtube_id, channel_youtube_id, title, url, thumbnail_url, published_at, discovered_at, last_seen_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET title = excluded.title, url = excluded.url,
    thumbnail_url = excluded.thumbnail_url, published_at = excluded.published_at,
    last_seen_at = excluded.last_seen_at`,
		video.YouTubeID, video.ChannelYouTubeID, video.Title, video.URL, video.ThumbnailURL,
		formatTime(video.PublishedAt), formatTime(video.DiscoveredAt), formatTime(video.LastSeenAt))
	if err != nil {
		return fmt.Errorf("save video %q during sync: %w", video.YouTubeID, err)
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
	var lastChecked, lastSuccessful, nextCheck sql.NullString
	var created, updated string
	err := row.Scan(&channel.YouTubeID, &channel.SourceURL, &channel.Title, &channel.AvatarURL,
		&channel.FeedETag, &channel.FeedLastModified, &lastChecked, &lastSuccessful, &nextCheck,
		&channel.FailureCount, &channel.LastError, &created, &updated)
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
