package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCategoryMigrationPreservesExistingChannelsAndOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tuby.sqlite")
	database := openLegacyDatabase(t, path, 3)
	for _, channel := range []struct {
		id       string
		position int
	}{
		{id: "UC-first", position: 1},
		{id: "UC-second", position: 2},
	} {
		_, err := database.Exec(`
INSERT INTO channels(youtube_id, source_url, title, position, created_at, updated_at)
VALUES(?, ?, ?, ?, '2026-11-08T12:00:00Z', '2026-11-08T12:00:00Z')`,
			channel.id, "https://youtube.com/channel/"+channel.id, channel.id, channel.position)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store := openStore(t, path)
	t.Cleanup(func() { closeStore(t, store) })
	var name string
	var isDefault bool
	if err := store.db.QueryRow(`SELECT name, is_default FROM categories WHERE id = ?`, DefaultCategoryID).
		Scan(&name, &isDefault); err != nil {
		t.Fatal(err)
	}
	if name != "Uncategorized" || !isDefault {
		t.Fatalf("default category = (%q, %t)", name, isDefault)
	}
	rows, err := store.db.Query(`SELECT category_id, position FROM channels ORDER BY position`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	position := 0
	for rows.Next() {
		position++
		var categoryID int64
		var storedPosition int
		if err := rows.Scan(&categoryID, &storedPosition); err != nil {
			t.Fatal(err)
		}
		if categoryID != DefaultCategoryID || storedPosition != position {
			t.Fatalf("channel %d = category %d, position %d", position, categoryID, storedPosition)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if position != 2 {
		t.Fatalf("migrated channels = %d, want 2", position)
	}
}

func TestDefaultCategoryNameMigrationProtectsExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tuby.sqlite")
	database := openLegacyDatabase(t, path, 4)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	store := openStore(t, path)
	t.Cleanup(func() { closeStore(t, store) })
	assertStatementFails(t, store,
		`UPDATE categories SET name = 'General', name_key = 'general' WHERE id = ?`, DefaultCategoryID)
}

func TestVideoDurationMigrationLeavesExistingVideosUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tuby.sqlite")
	database := openLegacyDatabase(t, path, 5)
	_, err := database.Exec(`
INSERT INTO channels(youtube_id, source_url, title, created_at, updated_at)
VALUES('UC123', 'https://youtube.com/@example', 'Example', '2026-11-08T12:00:00Z', '2026-11-08T12:00:00Z');
INSERT INTO videos(youtube_id, channel_youtube_id, title, url, published_at, discovered_at, last_seen_at)
VALUES('video', 'UC123', 'Video', 'https://youtube.com/watch?v=video',
       '2026-11-08T12:00:00Z', '2026-11-08T12:00:00Z', '2026-11-08T12:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	store := openStore(t, path)
	t.Cleanup(func() { closeStore(t, store) })
	duration, err := store.VideoDuration(context.Background(), "video")
	if err != nil || duration != nil {
		t.Fatalf("duration = %v, error = %v", duration, err)
	}
}

func openLegacyDatabase(t *testing.T, path string, lastVersion int) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.version > lastVersion {
			break
		}
		if _, err := database.Exec(migration.query); err != nil {
			t.Fatalf("apply legacy migration %d: %v", migration.version, err)
		}
		if _, err := database.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-11-08T12:00:00Z')`,
			migration.version); err != nil {
			t.Fatal(err)
		}
	}
	return database
}
