package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrVideoNotFound = errors.New("video not found")

func (s *Store) SaveVideo(ctx context.Context, video Video) error {
	if err := validateVideo(video); err != nil {
		return err
	}
	video = videoWithTimestamps(video, time.Now())
	_, err := s.db.ExecContext(ctx, `
INSERT INTO videos(youtube_id, channel_youtube_id, title, url, thumbnail_url, published_at, discovered_at, last_seen_at, is_short)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET
    channel_youtube_id = excluded.channel_youtube_id,
    title = excluded.title,
    url = excluded.url,
    thumbnail_url = excluded.thumbnail_url,
    published_at = excluded.published_at,
    last_seen_at = excluded.last_seen_at,
    is_short = excluded.is_short`,
		video.YouTubeID,
		video.ChannelYouTubeID,
		video.Title,
		video.URL,
		video.ThumbnailURL,
		formatTime(video.PublishedAt),
		formatTime(video.DiscoveredAt),
		formatTime(video.LastSeenAt),
		video.IsShort,
	)
	if err != nil {
		return fmt.Errorf("save video %q: %w", video.YouTubeID, err)
	}
	return nil
}

func (s *Store) MarkVideoWatched(ctx context.Context, videoID string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
UPDATE videos SET unseen = 0, watch_later = 0, watched_at = ? WHERE youtube_id = ?`,
		formatTime(now), videoID)
	if err != nil {
		return fmt.Errorf("mark video %q as watched: %w", videoID, err)
	}
	return requireVideo(result, videoID)
}

func (s *Store) MarkChannelWatched(ctx context.Context, channelID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE videos SET unseen = 0, watch_later = 0, watched_at = ? WHERE channel_youtube_id = ?`, formatTime(now), channelID)
	if err != nil {
		return fmt.Errorf("mark channel %q as watched: %w", channelID, err)
	}
	return nil
}

func validateVideo(video Video) error {
	if strings.TrimSpace(video.YouTubeID) == "" {
		return fmt.Errorf("video YouTube ID must not be empty")
	}
	if strings.TrimSpace(video.ChannelYouTubeID) == "" {
		return fmt.Errorf("video channel YouTube ID must not be empty")
	}
	if strings.TrimSpace(video.Title) == "" {
		return fmt.Errorf("video title must not be empty")
	}
	if strings.TrimSpace(video.URL) == "" {
		return fmt.Errorf("video URL must not be empty")
	}
	if video.PublishedAt.IsZero() {
		return fmt.Errorf("video publication time must not be empty")
	}
	return nil
}

func requireVideo(result sql.Result, videoID string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check video update: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("%w: %s", ErrVideoNotFound, videoID)
	}
	return nil
}

func videoWithTimestamps(video Video, now time.Time) Video {
	now = now.UTC()
	if video.DiscoveredAt.IsZero() {
		video.DiscoveredAt = now
	}
	if video.LastSeenAt.IsZero() {
		video.LastSeenAt = now
	}
	return video
}
