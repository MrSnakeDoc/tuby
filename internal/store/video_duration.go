package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) VideoDuration(ctx context.Context, videoID string) (*int64, error) {
	var duration sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT duration_seconds FROM videos WHERE youtube_id = ?`, videoID).Scan(&duration)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrVideoNotFound, videoID)
	}
	if err != nil {
		return nil, fmt.Errorf("read duration for video %q: %w", videoID, err)
	}
	if !duration.Valid {
		return nil, nil
	}
	return &duration.Int64, nil
}

func (s *Store) SetVideoDuration(ctx context.Context, videoID string, seconds int64) error {
	if seconds <= 0 {
		return fmt.Errorf("video duration must be positive")
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE videos SET duration_seconds = ? WHERE youtube_id = ?`, seconds, videoID)
	if err != nil {
		return fmt.Errorf("save duration for video %q: %w", videoID, err)
	}
	return requireVideo(result, videoID)
}
