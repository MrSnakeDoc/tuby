package store

import (
	"context"
	"fmt"
)

func (s *Store) UpdateChannelSettings(ctx context.Context, channelID string, includeLong, includeShorts bool) error {
	if !includeLong && !includeShorts {
		return fmt.Errorf("at least one video type must be enabled")
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin channel settings update: %w", err)
	}
	result, err := transaction.ExecContext(ctx, `
UPDATE channels SET include_long = ?, include_shorts = ?, feed_etag = '', feed_last_modified = '',
    last_successful_sync_at = NULL, next_check_at = NULL
WHERE youtube_id = ?`, includeLong, includeShorts, channelID)
	if err != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("update channel settings: %w", err)
	}
	if err := requireChannel(result, channelID); err != nil {
		_ = transaction.Rollback()
		return err
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
