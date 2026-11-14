package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeCategoryName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantKey     string
		wantErrText string
	}{
		{name: "normalizes whitespace and case", input: "  Tech   News  ", wantName: "Tech News", wantKey: "tech news"},
		{name: "supports Unicode", input: "Éducation", wantName: "Éducation", wantKey: "éducation"},
		{name: "rejects empty", input: " \t ", wantErrText: "must not be empty"},
		{name: "rejects oversized", input: strings.Repeat("é", MaxCategoryName+1), wantErrText: "too long"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, key, err := normalizeCategoryName(test.input)
			if test.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
					t.Fatalf("error = %v, want text %q", err, test.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name != test.wantName || key != test.wantKey {
				t.Fatalf("normalized = (%q, %q), want (%q, %q)", name, key, test.wantName, test.wantKey)
			}
		})
	}
}

func TestCategoryLifecycle(t *testing.T) {
	store := openStore(t, t.TempDir()+"/tuby.sqlite")
	t.Cleanup(func() { closeStore(t, store) })
	ctx := context.Background()
	tech, err := store.CreateCategory(ctx, "  Tech  ")
	if err != nil {
		t.Fatal(err)
	}
	if tech.Name != "Tech" || tech.Position != 2 || tech.IsDefault {
		t.Fatalf("created category = %#v", tech)
	}
	if _, err := store.CreateCategory(ctx, "tech"); !errors.Is(err, ErrCategoryNameExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	renamed, err := store.RenameCategory(ctx, tech.ID, "Science")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Science" || renamed.Position != tech.Position {
		t.Fatalf("renamed category = %#v", renamed)
	}
	if _, err := store.RenameCategory(ctx, DefaultCategoryID, "General"); !errors.Is(err, ErrDefaultCategoryName) {
		t.Fatalf("rename default category error = %v", err)
	}
	if _, err := store.RenameCategory(ctx, tech.ID, " uncategorized "); !errors.Is(err, ErrCategoryNameExists) {
		t.Fatalf("rename duplicate error = %v", err)
	}
	categories, err := store.Categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 2 || categories[0].Name != "Uncategorized" || categories[1].Name != "Science" {
		t.Fatalf("categories = %#v", categories)
	}
}

func TestDeleteCategoryMovesChannelsToEndOfDefaultCategory(t *testing.T) {
	store := openStore(t, t.TempDir()+"/tuby.sqlite")
	t.Cleanup(func() { closeStore(t, store) })
	ctx := context.Background()
	category, err := store.CreateCategory(ctx, "Tech")
	if err != nil {
		t.Fatal(err)
	}
	saveTestChannel(t, store, "UC-default", DefaultCategoryID)
	saveTestChannel(t, store, "UC-tech-one", category.ID)
	saveTestChannel(t, store, "UC-tech-two", category.ID)
	moved, err := store.DeleteCategory(ctx, category.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(moved, ",") != "UC-tech-one,UC-tech-two" {
		t.Fatalf("moved channels = %v", moved)
	}
	rows, err := store.db.Query(`SELECT youtube_id, category_id, position FROM channels ORDER BY position`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	wantIDs := []string{"UC-default", "UC-tech-one", "UC-tech-two"}
	index := 0
	for rows.Next() {
		var channelID string
		var categoryID int64
		var position int
		if err := rows.Scan(&channelID, &categoryID, &position); err != nil {
			t.Fatal(err)
		}
		if channelID != wantIDs[index] || categoryID != DefaultCategoryID || position != index+1 {
			t.Fatalf("channel %d = (%q, %d, %d)", index, channelID, categoryID, position)
		}
		index++
	}
	if index != len(wantIDs) {
		t.Fatalf("channels = %d, want %d", index, len(wantIDs))
	}
}

func saveTestChannel(t *testing.T, store *Store, channelID string, categoryID int64) {
	t.Helper()
	err := store.SaveChannel(context.Background(), Channel{
		YouTubeID: channelID, CategoryID: categoryID, SourceURL: "https://youtube.com/channel/" + channelID,
		Title: channelID,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCategoryConstraints(t *testing.T) {
	store := openStore(t, t.TempDir()+"/tuby.sqlite")
	t.Cleanup(func() { closeStore(t, store) })

	assertStatementFails(t, store, `INSERT INTO categories(name, name_key, position) VALUES('Other', 'UNCATEGORIZED', 2)`)
	assertStatementFails(t, store, `INSERT INTO categories(name, name_key, position) VALUES('', 'empty', 2)`)
	assertStatementFails(t, store, `INSERT INTO categories(name, name_key, position) VALUES(?, 'long', 2)`, strings.Repeat("a", 51))
	assertStatementFails(t, store, `DELETE FROM categories WHERE id = ?`, DefaultCategoryID)
	assertStatementFails(t, store, `UPDATE categories SET is_default = 0 WHERE id = ?`, DefaultCategoryID)
	assertStatementFails(t, store, `UPDATE categories SET name = 'General', name_key = 'general' WHERE id = ?`, DefaultCategoryID)

	result, err := store.db.Exec(`INSERT INTO categories(name, name_key, position) VALUES('Tech', 'tech', 2)`)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	channel := Channel{
		YouTubeID: "UC123", CategoryID: categoryID, SourceURL: "https://youtube.com/@example",
		Title: "Example", CreatedAt: time.Now(),
	}
	if err := store.SaveChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	assertStatementFails(t, store, `DELETE FROM categories WHERE id = ?`, categoryID)
	assertStatementFails(t, store, `UPDATE channels SET category_id = 999 WHERE youtube_id = ?`, channel.YouTubeID)
}

func assertStatementFails(t *testing.T, store *Store, query string, args ...any) {
	t.Helper()
	if _, err := store.db.Exec(query, args...); err == nil {
		t.Fatalf("statement unexpectedly succeeded: %s", query)
	}
}
