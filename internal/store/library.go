package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) ChannelSummaries(ctx context.Context) ([]ChannelSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.youtube_id, c.source_url, c.title, c.avatar_url, c.feed_etag, c.feed_last_modified,
       c.last_checked_at, c.last_successful_sync_at, c.next_check_at, c.failure_count,
       c.last_error, c.created_at, c.updated_at, COUNT(v.youtube_id),
       COALESCE(SUM(v.unseen), 0), COALESCE(NULLIF(c.avatar_url, ''), (
           SELECT thumbnail_url FROM videos latest WHERE latest.channel_youtube_id = c.youtube_id
           ORDER BY published_at DESC LIMIT 1), '')
FROM channels c LEFT JOIN videos v ON v.channel_youtube_id = c.youtube_id
GROUP BY c.youtube_id ORDER BY c.position ASC`)
	if err != nil {
		return nil, fmt.Errorf("list channel summaries: %w", err)
	}
	result := make([]ChannelSummary, 0)
	for rows.Next() {
		var summary ChannelSummary
		var lastChecked, lastSuccessful, nextCheck sql.NullString
		var created, updated string
		if err := rows.Scan(&summary.YouTubeID, &summary.SourceURL, &summary.Title, &summary.AvatarURL,
			&summary.FeedETag, &summary.FeedLastModified, &lastChecked, &lastSuccessful, &nextCheck,
			&summary.FailureCount, &summary.LastError, &created, &updated, &summary.VideoCount,
			&summary.UnseenCount, &summary.ImageURL); err != nil {
			return nil, fmt.Errorf("scan channel summary: %w", err)
		}
		if err := setChannelTimes(&summary.Channel, lastChecked, lastSuccessful, nextCheck, created, updated); err != nil {
			return nil, err
		}
		result = append(result, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		thumbnails, err := s.channelThumbnails(ctx, result[index].YouTubeID)
		if err != nil {
			return nil, err
		}
		result[index].ThumbnailURLs = thumbnails
		if len(thumbnails) > 0 {
			result[index].ImageURL = thumbnails[0]
		}
	}
	return result, nil
}

func (s *Store) channelThumbnails(ctx context.Context, channelID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT thumbnail_url FROM videos WHERE channel_youtube_id = ? AND thumbnail_url != '' ORDER BY published_at DESC`, channelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]string, 0)
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		result = append(result, url)
	}
	return result, rows.Err()
}

func (s *Store) ChannelVideos(ctx context.Context, channelID string) ([]Video, error) {
	return s.videos(ctx, `WHERE channel_youtube_id = ?`, channelID)
}

func (s *Store) UnseenVideos(ctx context.Context) ([]Video, error) {
	return s.videos(ctx, `WHERE unseen = 1`)
}

func (s *Store) WatchLaterVideos(ctx context.Context) ([]Video, error) {
	return s.videos(ctx, `WHERE watch_later = 1`)
}

func (s *Store) WatchLaterCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos WHERE watch_later = 1`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count watch later videos: %w", err)
	}
	return count, nil
}

func (s *Store) SetVideoWatchLater(ctx context.Context, videoID string, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE videos SET watch_later = ? WHERE youtube_id = ?`, enabled, videoID)
	if err != nil {
		return fmt.Errorf("set watch later for video %q: %w", videoID, err)
	}
	return requireVideo(result, videoID)
}

func (s *Store) videos(ctx context.Context, clause string, args ...any) ([]Video, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT youtube_id, channel_youtube_id, title, url, thumbnail_url,
published_at, discovered_at, last_seen_at, unseen, watch_later, watched_at FROM videos `+clause+` ORDER BY published_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]Video, 0)
	for rows.Next() {
		video, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, video)
	}
	return result, rows.Err()
}

func scanVideo(row interface{ Scan(...any) error }) (Video, error) {
	var video Video
	var published, discovered, seen string
	var watched sql.NullString
	if err := row.Scan(&video.YouTubeID, &video.ChannelYouTubeID, &video.Title, &video.URL, &video.ThumbnailURL,
		&published, &discovered, &seen, &video.Unseen, &video.WatchLater, &watched); err != nil {
		return Video{}, fmt.Errorf("scan video: %w", err)
	}
	var err error
	if video.PublishedAt, err = time.Parse(time.RFC3339Nano, published); err != nil {
		return Video{}, err
	}
	if video.DiscoveredAt, err = time.Parse(time.RFC3339Nano, discovered); err != nil {
		return Video{}, err
	}
	if video.LastSeenAt, err = time.Parse(time.RFC3339Nano, seen); err != nil {
		return Video{}, err
	}
	if watched.Valid {
		value, err := time.Parse(time.RFC3339Nano, watched.String)
		if err != nil {
			return Video{}, err
		}
		video.WatchedAt = &value
	}
	return video, nil
}

func setChannelTimes(channel *Channel, lastChecked, lastSuccessful, nextCheck sql.NullString, created, updated string) error {
	var err error
	if channel.LastCheckedAt, err = parseOptionalTime(lastChecked); err != nil {
		return err
	}
	if channel.LastSuccessfulSync, err = parseOptionalTime(lastSuccessful); err != nil {
		return err
	}
	if channel.NextCheckAt, err = parseOptionalTime(nextCheck); err != nil {
		return err
	}
	if channel.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return err
	}
	channel.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	return err
}
