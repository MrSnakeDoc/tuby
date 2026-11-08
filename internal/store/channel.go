package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Store) SaveChannel(ctx context.Context, channel Channel) error {
	if err := validateChannel(channel); err != nil {
		return err
	}
	channel = channelWithTimestamps(channel, time.Now())
	_, err := s.db.ExecContext(ctx, `
INSERT INTO channels(youtube_id, source_url, title, avatar_url, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(youtube_id) DO UPDATE SET
    source_url = excluded.source_url,
    title = excluded.title,
    avatar_url = excluded.avatar_url,
    updated_at = excluded.updated_at`,
		channel.YouTubeID,
		channel.SourceURL,
		channel.Title,
		channel.AvatarURL,
		formatTime(channel.CreatedAt),
		formatTime(channel.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("save channel %q: %w", channel.YouTubeID, err)
	}
	return nil
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
