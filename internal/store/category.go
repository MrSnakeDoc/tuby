package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultCategoryID = int64(1)
	MaxCategoryName   = 50
)

var (
	ErrCategoryNotFound    = errors.New("category not found")
	ErrCategoryNameExists  = errors.New("category name already exists")
	ErrCategoryNameEmpty   = errors.New("category name must not be empty")
	ErrCategoryNameTooLong = errors.New("category name is too long")
	ErrDefaultCategory     = errors.New("default category cannot be deleted")
	ErrDefaultCategoryName = errors.New("default category cannot be renamed")
)

func (s *Store) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, position, is_default FROM categories ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Position, &category.IsDefault); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		result = append(result, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return result, nil
}

func (s *Store) Category(ctx context.Context, categoryID int64) (Category, error) {
	var category Category
	err := s.db.QueryRowContext(ctx, `SELECT id, name, position, is_default FROM categories WHERE id = ?`, categoryID).
		Scan(&category.ID, &category.Name, &category.Position, &category.IsDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, fmt.Errorf("%w: %d", ErrCategoryNotFound, categoryID)
	}
	if err != nil {
		return Category{}, fmt.Errorf("load category: %w", err)
	}
	return category, nil
}

func (s *Store) CreateCategory(ctx context.Context, name string) (Category, error) {
	name, key, err := normalizeCategoryName(name)
	if err != nil {
		return Category{}, err
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Category{}, fmt.Errorf("begin category creation: %w", err)
	}
	category, err := createCategory(transaction, name, key)
	if err != nil {
		_ = transaction.Rollback()
		return Category{}, err
	}
	if err := transaction.Commit(); err != nil {
		return Category{}, fmt.Errorf("commit category creation: %w", err)
	}
	return category, nil
}

func (s *Store) RenameCategory(ctx context.Context, categoryID int64, name string) (Category, error) {
	name, key, err := normalizeCategoryName(name)
	if err != nil {
		return Category{}, err
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Category{}, fmt.Errorf("begin category rename: %w", err)
	}
	category, err := renameCategory(transaction, categoryID, name, key)
	if err != nil {
		_ = transaction.Rollback()
		return Category{}, err
	}
	if err := transaction.Commit(); err != nil {
		return Category{}, fmt.Errorf("commit category rename: %w", err)
	}
	return category, nil
}

func (s *Store) DeleteCategory(ctx context.Context, categoryID int64) ([]string, error) {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin category deletion: %w", err)
	}
	channelIDs, err := deleteCategory(transaction, categoryID)
	if err != nil {
		_ = transaction.Rollback()
		return nil, err
	}
	if err := transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit category deletion: %w", err)
	}
	return channelIDs, nil
}

func createCategory(transaction *sql.Tx, name, key string) (Category, error) {
	if err := requireAvailableCategoryName(transaction, key, 0); err != nil {
		return Category{}, err
	}
	var position int
	if err := transaction.QueryRow(`SELECT COALESCE(MAX(position), 0) + 1 FROM categories`).Scan(&position); err != nil {
		return Category{}, fmt.Errorf("select category position: %w", err)
	}
	result, err := transaction.Exec(`INSERT INTO categories(name, name_key, position) VALUES(?, ?, ?)`, name, key, position)
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	categoryID, err := result.LastInsertId()
	if err != nil {
		return Category{}, fmt.Errorf("read category ID: %w", err)
	}
	return Category{ID: categoryID, Name: name, Position: position}, nil
}

func renameCategory(transaction *sql.Tx, categoryID int64, name, key string) (Category, error) {
	category, err := categoryByID(transaction, categoryID)
	if err != nil {
		return Category{}, err
	}
	if category.IsDefault {
		return Category{}, ErrDefaultCategoryName
	}
	if err := requireAvailableCategoryName(transaction, key, categoryID); err != nil {
		return Category{}, err
	}
	if _, err := transaction.Exec(`UPDATE categories SET name = ?, name_key = ? WHERE id = ?`, name, key, categoryID); err != nil {
		return Category{}, fmt.Errorf("rename category: %w", err)
	}
	category.Name = name
	return category, nil
}

func deleteCategory(transaction *sql.Tx, categoryID int64) ([]string, error) {
	category, err := categoryByID(transaction, categoryID)
	if err != nil {
		return nil, err
	}
	if category.IsDefault {
		return nil, ErrDefaultCategory
	}
	channelIDs, err := categoryChannelIDs(transaction, categoryID)
	if err != nil {
		return nil, err
	}
	if err := moveChannelsToDefault(transaction, categoryID); err != nil {
		return nil, err
	}
	if _, err := transaction.Exec(`DELETE FROM categories WHERE id = ?`, categoryID); err != nil {
		return nil, fmt.Errorf("delete category: %w", err)
	}
	if _, err := transaction.Exec(`UPDATE categories SET position = position - 1 WHERE position > ?`, category.Position); err != nil {
		return nil, fmt.Errorf("compact category positions: %w", err)
	}
	return channelIDs, nil
}

func categoryByID(transaction *sql.Tx, categoryID int64) (Category, error) {
	var category Category
	err := transaction.QueryRow(`SELECT id, name, position, is_default FROM categories WHERE id = ?`, categoryID).
		Scan(&category.ID, &category.Name, &category.Position, &category.IsDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, fmt.Errorf("%w: %d", ErrCategoryNotFound, categoryID)
	}
	if err != nil {
		return Category{}, fmt.Errorf("load category: %w", err)
	}
	return category, nil
}

func requireAvailableCategoryName(transaction *sql.Tx, key string, excludedID int64) error {
	var exists bool
	if err := transaction.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE name_key = ? AND id != ?)`, key, excludedID).
		Scan(&exists); err != nil {
		return fmt.Errorf("check category name: %w", err)
	}
	if exists {
		return ErrCategoryNameExists
	}
	return nil
}

func categoryChannelIDs(transaction *sql.Tx, categoryID int64) ([]string, error) {
	rows, err := transaction.Query(`SELECT youtube_id FROM channels WHERE category_id = ? ORDER BY position, youtube_id`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list category channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]string, 0)
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil, fmt.Errorf("scan category channel: %w", err)
		}
		result = append(result, channelID)
	}
	return result, rows.Err()
}

func moveChannelsToDefault(transaction *sql.Tx, categoryID int64) error {
	var lastPosition int
	if err := transaction.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM channels WHERE category_id = ?`, DefaultCategoryID).
		Scan(&lastPosition); err != nil {
		return fmt.Errorf("select default category position: %w", err)
	}
	_, err := transaction.Exec(`
UPDATE channels SET category_id = ?, position = position + ? WHERE category_id = ?`,
		DefaultCategoryID, lastPosition, categoryID)
	if err != nil {
		return fmt.Errorf("move channels to default category: %w", err)
	}
	return nil
}

func normalizeCategoryName(name string) (string, string, error) {
	normalized := strings.Join(strings.Fields(name), " ")
	if normalized == "" {
		return "", "", ErrCategoryNameEmpty
	}
	if utf8.RuneCountInString(normalized) > MaxCategoryName {
		return "", "", fmt.Errorf("%w: maximum is %d characters", ErrCategoryNameTooLong, MaxCategoryName)
	}
	return normalized, strings.ToLower(normalized), nil
}
