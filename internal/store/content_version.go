package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

func (s *Store) ChannelContentVersion(ctx context.Context, channelID string) (string, error) {
	hash := sha256.New()
	var title, avatar string
	if err := s.db.QueryRowContext(ctx, `SELECT title, avatar_url FROM channels WHERE youtube_id = ?`, channelID).Scan(&title, &avatar); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: %s", ErrChannelNotFound, channelID)
		}
		return "", fmt.Errorf("load channel content version: %w", err)
	}
	writeVersionValue(hash, title, avatar)
	rows, err := s.db.QueryContext(ctx, `
SELECT youtube_id, title, url, thumbnail_url, published_at
FROM videos WHERE channel_youtube_id = ? ORDER BY youtube_id`, channelID)
	if err != nil {
		return "", fmt.Errorf("load channel videos content version: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var videoID, videoTitle, url, thumbnail, published string
		if err := rows.Scan(&videoID, &videoTitle, &url, &thumbnail, &published); err != nil {
			return "", fmt.Errorf("scan channel content version: %w", err)
		}
		writeVersionValue(hash, videoID, videoTitle, url, thumbnail, published)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate channel content version: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeVersionValue(hash interface{ Write([]byte) (int, error) }, values ...string) {
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
}
