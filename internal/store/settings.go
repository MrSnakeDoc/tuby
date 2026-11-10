package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) UpdateChannelSettings(
	ctx context.Context,
	channelID string,
	categoryID int64,
	includeLong, includeShorts bool,
) error {
	if !includeLong && !includeShorts {
		return fmt.Errorf("at least one video type must be enabled")
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin channel settings update: %w", err)
	}
	previousCategoryID, previousPosition, err := channelPlacement(ctx, transaction, channelID)
	if err != nil {
		_ = transaction.Rollback()
		return err
	}
	if categoryID != 0 {
		if _, err := categoryByID(transaction, categoryID); err != nil {
			_ = transaction.Rollback()
			return err
		}
	}
	result, err := transaction.ExecContext(ctx, `
UPDATE channels SET include_long = ?, include_shorts = ?,
    position = CASE WHEN ? != 0 AND category_id != ?
        THEN (SELECT COALESCE(MAX(target.position), 0) + 1 FROM channels target WHERE target.category_id = ?)
        ELSE position END,
    category_id = CASE WHEN ? = 0 THEN category_id ELSE ? END,
    feed_etag = '', feed_last_modified = '',
    last_successful_sync_at = NULL, next_check_at = NULL
WHERE youtube_id = ?`, includeLong, includeShorts,
		categoryID, categoryID, categoryID, categoryID, categoryID, channelID)
	if err != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("update channel settings: %w", err)
	}
	if err := requireChannel(result, channelID); err != nil {
		_ = transaction.Rollback()
		return err
	}
	if categoryID != 0 && categoryID != previousCategoryID {
		if _, err := transaction.Exec(`
UPDATE channels SET position = position - 1 WHERE category_id = ? AND position > ?`,
			previousCategoryID, previousPosition); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("compact previous category positions: %w", err)
		}
	}
	if err := purgeDisabledVideos(transaction, Channel{
		YouTubeID: channelID, IncludeLong: includeLong, IncludeShorts: includeShorts,
	}); err != nil {
		_ = transaction.Rollback()
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit channel settings update: %w", err)
	}
	return nil
}

func channelPlacement(ctx context.Context, transaction *sql.Tx, channelID string) (int64, int, error) {
	var categoryID int64
	var position int
	err := transaction.QueryRowContext(ctx,
		`SELECT category_id, position FROM channels WHERE youtube_id = ?`, channelID).
		Scan(&categoryID, &position)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("%w: %s", ErrChannelNotFound, channelID)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("load channel placement: %w", err)
	}
	return categoryID, position, nil
}
