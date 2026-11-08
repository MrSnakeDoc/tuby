package store

import (
	"fmt"
)

type migration struct {
	version int
	query   string
}

var migrations = []migration{
	{
		version: 1,
		query: `
CREATE TABLE channels (
    youtube_id TEXT PRIMARY KEY,
    source_url TEXT NOT NULL,
    title TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    feed_etag TEXT NOT NULL DEFAULT '',
    feed_last_modified TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT,
    last_successful_sync_at TEXT,
    next_check_at TEXT,
    failure_count INTEGER NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE videos (
    youtube_id TEXT PRIMARY KEY,
    channel_youtube_id TEXT NOT NULL REFERENCES channels(youtube_id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    thumbnail_url TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL,
    discovered_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    unseen INTEGER NOT NULL DEFAULT 1 CHECK (unseen IN (0, 1)),
    watch_later INTEGER NOT NULL DEFAULT 0 CHECK (watch_later IN (0, 1)),
    watched_at TEXT
);

CREATE INDEX videos_channel_published_at_idx ON videos(channel_youtube_id, published_at DESC);
CREATE INDEX videos_unseen_published_at_idx ON videos(unseen, published_at DESC);
CREATE INDEX videos_watch_later_published_at_idx ON videos(watch_later, published_at DESC);`,
	},
	{
		version: 2,
		query: `ALTER TABLE channels ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
UPDATE channels SET position = rowid WHERE position = 0;`,
	},
}

func (s *Store) initialize() error {
	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("connect to SQLite database: %w", err)
	}
	if err := s.configure(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	for _, migration := range migrations {
		if err := s.applyMigration(migration); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) configure() error {
	for _, query := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("configure SQLite database: %w", err)
		}
	}
	return nil
}

func (s *Store) applyMigration(migration migration) error {
	var applied bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, migration.version).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %d: %w", migration.version, err)
	}
	if applied {
		return nil
	}
	transaction, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.version, err)
	}
	if _, err := transaction.Exec(migration.query); err != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("apply migration %d: %w", migration.version, err)
	}
	if _, err := transaction.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, migration.version); err != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("record migration %d: %w", migration.version, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.version, err)
	}
	return nil
}
