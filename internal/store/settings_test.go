package store

import (
	"context"
	"errors"
	"testing"
)

func TestUpdateChannelSettingsMovesChannelToEndOfCategory(t *testing.T) {
	store := openStore(t, t.TempDir()+"/tuby.sqlite")
	t.Cleanup(func() { closeStore(t, store) })
	ctx := context.Background()
	category, err := store.CreateCategory(ctx, "Tech")
	if err != nil {
		t.Fatal(err)
	}
	saveTestChannel(t, store, "UC-tech-one", category.ID)
	saveTestChannel(t, store, "UC-tech-two", category.ID)
	saveTestChannel(t, store, "UC-moving", DefaultCategoryID)
	saveTestChannel(t, store, "UC-stays", DefaultCategoryID)
	if err := store.UpdateChannelSettings(ctx, "UC-moving", category.ID, true, false); err != nil {
		t.Fatal(err)
	}
	channel, err := store.Channel(ctx, "UC-moving")
	if err != nil {
		t.Fatal(err)
	}
	if channel.CategoryID != category.ID || channel.Position != 3 {
		t.Fatalf("moved channel = %#v", channel)
	}
	staying, err := store.Channel(ctx, "UC-stays")
	if err != nil {
		t.Fatal(err)
	}
	if staying.Position != 1 {
		t.Fatalf("remaining channel position = %d, want 1", staying.Position)
	}
	if err := store.UpdateChannelSettings(ctx, "UC-moving", 999, true, false); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("unknown category error = %v", err)
	}
	channel, err = store.Channel(ctx, "UC-moving")
	if err != nil {
		t.Fatal(err)
	}
	if channel.CategoryID != category.ID || channel.Position != 3 {
		t.Fatalf("channel changed after rejected update = %#v", channel)
	}
}
