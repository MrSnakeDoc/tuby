package store

import (
	"context"
	"fmt"
)

func (s *Store) SaveChannelOrder(ctx context.Context, channelIDs []string) error {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channels`).Scan(&total); err != nil {
		return fmt.Errorf("count channels: %w", err)
	}
	if len(channelIDs) != total {
		return fmt.Errorf("channel order is incomplete")
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin channel order: %w", err)
	}
	seen := make(map[string]struct{}, len(channelIDs))
	for position, channelID := range channelIDs {
		if _, exists := seen[channelID]; exists {
			_ = transaction.Rollback()
			return fmt.Errorf("duplicate channel in order")
		}
		seen[channelID] = struct{}{}
		result, err := transaction.ExecContext(ctx, `UPDATE channels SET position = ? WHERE youtube_id = ?`, position+1, channelID)
		if err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("save channel order: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			_ = transaction.Rollback()
			return fmt.Errorf("unknown channel in order")
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit channel order: %w", err)
	}
	return nil
}
