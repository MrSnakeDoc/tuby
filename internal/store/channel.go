package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Store) SaveChannel(ctx context.Context, channel Channel) error {
	channel = channelWithDefaultCategory(channel)
	if !channel.IncludeLong && !channel.IncludeShorts {
		channel.IncludeLong = true
	}
	if err := validateChannel(channel); err != nil {
		return err
	}
	channel = channelWithTimestamps(channel, time.Now())
	_, err := s.db.ExecContext(ctx, `
INSERT INTO channels(youtube_id, category_id, position, source_url, title, avatar_url,
    include_long, include_shorts, created_at, updated_at)
VALUES(?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM channels WHERE category_id = ?), ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET
    source_url = excluded.source_url,
    title = excluded.title,
    avatar_url = excluded.avatar_url,
	include_long = excluded.include_long,
	include_shorts = excluded.include_shorts,
    updated_at = excluded.updated_at`,
		channel.YouTubeID,
		channel.CategoryID,
		channel.CategoryID,
		channel.SourceURL,
		channel.Title,
		channel.AvatarURL,
		channel.IncludeLong,
		channel.IncludeShorts,
		formatTime(channel.CreatedAt),
		formatTime(channel.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("save channel %q: %w", channel.YouTubeID, err)
	}
	return nil
}

func channelWithDefaultCategory(channel Channel) Channel {
	if channel.CategoryID == 0 {
		channel.CategoryID = DefaultCategoryID
	}
	return channel
}

func validateChannel(channel Channel) error {
	if strings.TrimSpace(channel.YouTubeID) == "" {
		return fmt.Errorf("channel YouTube ID must not be empty")
	}
	if strings.TrimSpace(channel.SourceURL) == "" {
		return fmt.Errorf("channel source URL must not be empty")
	}
	if strings.TrimSpace(channel.Title) == "" {
		return fmt.Errorf("channel title must not be empty")
	}
	return nil
}

func channelWithTimestamps(channel Channel, now time.Time) Channel {
	now = now.UTC()
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = now
	}
	channel.UpdatedAt = now
	return channel
}
