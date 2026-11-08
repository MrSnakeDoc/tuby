package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenCreatesDatabaseAndAppliesMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tuby.sqlite")
	store := openStore(t, path)
	t.Cleanup(func() { closeStore(t, store) })
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file: %v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(migrations) {
		t.Fatalf("applied migrations = %d, want %d", count, len(migrations))
	}
}

func TestOpenReusesExistingDatabaseAndPreservesVideoState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tuby.sqlite")
	store := openStore(t, path)
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	channel := Channel{
		YouTubeID: "UC123",
		SourceURL: "https://www.youtube.com/@example",
		Title:     "Example",
		AvatarURL: "https://example.com/avatar.jpg",
		CreatedAt: now,
	}
	if err := store.SaveChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	video := Video{
		YouTubeID:        "video-123",
		ChannelYouTubeID: channel.YouTubeID,
		Title:            "Original title",
		URL:              "https://www.youtube.com/watch?v=video-123",
		ThumbnailURL:     "https://i.ytimg.com/vi/video-123/hqdefault.jpg",
		PublishedAt:      now,
		DiscoveredAt:     now,
		LastSeenAt:       now,
	}
	if err := store.SaveVideo(context.Background(), video); err != nil {
		t.Fatal(err)
	}
	var initiallyUnseen int
	if err := store.db.QueryRow(`SELECT unseen FROM videos WHERE youtube_id = ?`, video.YouTubeID).Scan(&initiallyUnseen); err != nil {
		t.Fatal(err)
	}
	if initiallyUnseen != 1 {
		t.Fatalf("initial unseen state = %d, want 1", initiallyUnseen)
	}
	if _, err := store.db.Exec(`UPDATE videos SET unseen = 0, watch_later = 1 WHERE youtube_id = ?`, video.YouTubeID); err != nil {
		t.Fatal(err)
	}
	video.Title = "Updated title"
	if err := store.SaveVideo(context.Background(), video); err != nil {
		t.Fatal(err)
	}
	closeStore(t, store)

	reopened := openStore(t, path)
	t.Cleanup(func() { closeStore(t, reopened) })
	var title string
	var unseen, watchLater int
	if err := reopened.db.QueryRow(`SELECT title, unseen, watch_later FROM videos WHERE youtube_id = ?`, video.YouTubeID).Scan(&title, &unseen, &watchLater); err != nil {
		t.Fatal(err)
	}
	if title != "Updated title" || unseen != 0 || watchLater != 1 {
		t.Fatalf("video = title %q, unseen %d, watch_later %d", title, unseen, watchLater)
	}
}

func openStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func closeStore(t *testing.T, store *Store) {
	t.Helper()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
