package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSaveLibraryOrderMovesCategoriesAndChannelsAtomically(t *testing.T) {
	store := openOrderTestStore(t)
	ctx := context.Background()
	tech := createOrderCategory(t, store, "Tech")
	science := createOrderCategory(t, store, "Science")
	saveOrderChannel(t, store, "UC-a", DefaultCategoryID)
	saveOrderChannel(t, store, "UC-b", DefaultCategoryID)
	saveOrderChannel(t, store, "UC-c", tech.ID)
	saveOrderChannel(t, store, "UC-d", science.ID)

	err := store.SaveLibraryOrder(ctx, []CategoryOrder{
		{CategoryID: science.ID, ChannelIDs: []string{"UC-d", "UC-b"}},
		{CategoryID: DefaultCategoryID, ChannelIDs: []string{}},
		{CategoryID: tech.ID, ChannelIDs: []string{"UC-c", "UC-a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCategoryOrder(t, store, []int64{science.ID, DefaultCategoryID, tech.ID})
	assertChannelOrder(t, store, science.ID, []string{"UC-d", "UC-b"})
	assertChannelOrder(t, store, DefaultCategoryID, []string{})
	assertChannelOrder(t, store, tech.ID, []string{"UC-c", "UC-a"})
}

func TestSaveLibraryOrderRejectsIncompleteTreeWithoutPartialWrites(t *testing.T) {
	store := openOrderTestStore(t)
	ctx := context.Background()
	tech := createOrderCategory(t, store, "Tech")
	saveOrderChannel(t, store, "UC-a", DefaultCategoryID)
	saveOrderChannel(t, store, "UC-b", tech.ID)

	err := store.SaveLibraryOrder(ctx, []CategoryOrder{
		{CategoryID: tech.ID, ChannelIDs: []string{"UC-b"}},
		{CategoryID: DefaultCategoryID, ChannelIDs: []string{}},
	})
	if err == nil {
		t.Fatal("expected incomplete order to fail")
	}
	assertCategoryOrder(t, store, []int64{DefaultCategoryID, tech.ID})
	assertChannelOrder(t, store, DefaultCategoryID, []string{"UC-a"})
	assertChannelOrder(t, store, tech.ID, []string{"UC-b"})
}

func openOrderTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func createOrderCategory(t *testing.T, store *Store, name string) Category {
	t.Helper()
	category, err := store.CreateCategory(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return category
}

func saveOrderChannel(t *testing.T, store *Store, channelID string, categoryID int64) {
	t.Helper()
	err := store.SaveChannel(context.Background(), Channel{
		YouTubeID: channelID, CategoryID: categoryID,
		SourceURL: "https://youtube.com/channel/" + channelID, Title: channelID,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertCategoryOrder(t *testing.T, store *Store, want []int64) {
	t.Helper()
	categories, err := store.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != len(want) {
		t.Fatalf("categories = %d, want %d", len(categories), len(want))
	}
	for index, categoryID := range want {
		if categories[index].ID != categoryID || categories[index].Position != index+1 {
			t.Fatalf("category %d = %#v, want ID %d", index, categories[index], categoryID)
		}
	}
}

func assertChannelOrder(t *testing.T, store *Store, categoryID int64, want []string) {
	t.Helper()
	rows, err := store.db.Query(
		`SELECT youtube_id, position FROM channels WHERE category_id = ? ORDER BY position`, categoryID,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	index := 0
	for rows.Next() {
		var channelID string
		var position int
		if err := rows.Scan(&channelID, &position); err != nil {
			t.Fatal(err)
		}
		if index >= len(want) || channelID != want[index] || position != index+1 {
			t.Fatalf("channel %d = (%q, %d), want %#v", index, channelID, position, want)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(want) {
		t.Fatalf("channels = %d, want %d", index, len(want))
	}
}
