package store

import (
	"context"
	"database/sql"
	"fmt"
)

type CategoryOrder struct {
	CategoryID int64
	ChannelIDs []string
}

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

func (s *Store) SaveLibraryOrder(ctx context.Context, order []CategoryOrder) error {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin library order: %w", err)
	}
	if err := saveLibraryOrder(ctx, transaction, order); err != nil {
		_ = transaction.Rollback()
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit library order: %w", err)
	}
	return nil
}

func saveLibraryOrder(ctx context.Context, transaction *sql.Tx, order []CategoryOrder) error {
	categories, channels, err := libraryOrderIDs(ctx, transaction)
	if err != nil {
		return err
	}
	if len(order) != len(categories) {
		return fmt.Errorf("category order is incomplete")
	}
	seenCategories := make(map[int64]struct{}, len(order))
	seenChannels := make(map[string]struct{}, len(channels))
	for categoryPosition, category := range order {
		if err := saveCategoryOrder(ctx, transaction, category, categoryPosition, categories, seenCategories, channels, seenChannels); err != nil {
			return err
		}
	}
	if len(seenChannels) != len(channels) {
		return fmt.Errorf("channel order is incomplete")
	}
	return nil
}

func saveCategoryOrder(
	ctx context.Context,
	transaction *sql.Tx,
	category CategoryOrder,
	categoryPosition int,
	categories map[int64]struct{},
	seenCategories map[int64]struct{},
	channels map[string]struct{},
	seenChannels map[string]struct{},
) error {
	if _, exists := categories[category.CategoryID]; !exists {
		return fmt.Errorf("unknown category in order")
	}
	if _, exists := seenCategories[category.CategoryID]; exists {
		return fmt.Errorf("duplicate category in order")
	}
	seenCategories[category.CategoryID] = struct{}{}
	if _, err := transaction.ExecContext(ctx, `UPDATE categories SET position = ? WHERE id = ?`, categoryPosition+1, category.CategoryID); err != nil {
		return fmt.Errorf("save category order: %w", err)
	}
	for channelPosition, channelID := range category.ChannelIDs {
		if err := saveOrderedChannel(ctx, transaction, category.CategoryID, channelID, channelPosition, channels, seenChannels); err != nil {
			return err
		}
	}
	return nil
}

func saveOrderedChannel(
	ctx context.Context,
	transaction *sql.Tx,
	categoryID int64,
	channelID string,
	position int,
	channels map[string]struct{},
	seen map[string]struct{},
) error {
	if _, exists := channels[channelID]; !exists {
		return fmt.Errorf("unknown channel in order")
	}
	if _, exists := seen[channelID]; exists {
		return fmt.Errorf("duplicate channel in order")
	}
	seen[channelID] = struct{}{}
	if _, err := transaction.ExecContext(ctx,
		`UPDATE channels SET category_id = ?, position = ? WHERE youtube_id = ?`, categoryID, position+1, channelID,
	); err != nil {
		return fmt.Errorf("save channel category order: %w", err)
	}
	return nil
}

func libraryOrderIDs(ctx context.Context, transaction *sql.Tx) (map[int64]struct{}, map[string]struct{}, error) {
	categories, err := categoryOrderIDs(ctx, transaction)
	if err != nil {
		return nil, nil, err
	}
	channels, err := channelOrderIDs(ctx, transaction)
	if err != nil {
		return nil, nil, err
	}
	return categories, channels, nil
}

func categoryOrderIDs(ctx context.Context, transaction *sql.Tx) (map[int64]struct{}, error) {
	rows, err := transaction.QueryContext(ctx, `SELECT id FROM categories`)
	if err != nil {
		return nil, fmt.Errorf("list ordered categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[int64]struct{})
	for rows.Next() {
		var categoryID int64
		if err := rows.Scan(&categoryID); err != nil {
			return nil, fmt.Errorf("scan ordered category: %w", err)
		}
		result[categoryID] = struct{}{}
	}
	return result, rows.Err()
}

func channelOrderIDs(ctx context.Context, transaction *sql.Tx) (map[string]struct{}, error) {
	rows, err := transaction.QueryContext(ctx, `SELECT youtube_id FROM channels`)
	if err != nil {
		return nil, fmt.Errorf("list ordered channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string]struct{})
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil, fmt.Errorf("scan ordered channel: %w", err)
		}
		result[channelID] = struct{}{}
	}
	return result, rows.Err()
}
