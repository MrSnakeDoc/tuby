package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSyncDeduplicatesVideosAndPreservesUserState(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	sync := ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos: []Video{{
			YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "First title",
			URL: "https://www.youtube.com/watch?v=video-1", PublishedAt: now,
		}},
		ETag: "etag-1", Modified: "last-modified-1",
	}
	result, err := database.SaveSyncWithResult(context.Background(), sync)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.InsertedVideos) != 1 || result.InsertedVideos[0].YouTubeID != "video-1" {
		t.Fatalf("initial sync result = %#v", result)
	}
	if _, err := database.db.Exec(`UPDATE videos SET unseen = 0, watch_later = 1 WHERE youtube_id = 'video-1'`); err != nil {
		t.Fatal(err)
	}
	sync.Videos[0].Title = "Updated title"
	result, err = database.SaveSyncWithResult(context.Background(), sync)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.InsertedVideos) != 0 {
		t.Fatalf("repeated sync result = %#v", result)
	}
	var title string
	var unseen, later, count int
	if err := database.db.QueryRow(`SELECT title, unseen, watch_later FROM videos WHERE youtube_id = 'video-1'`).Scan(&title, &unseen, &later); err != nil {
		t.Fatal(err)
	}
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if title != "Updated title" || unseen != 0 || later != 1 || count != 1 {
		t.Fatalf("video title %q, unseen %d, later %d, count %d", title, unseen, later, count)
	}
}

func TestRecordSyncFailurePreservesVideos(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	sync := ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos: []Video{{
			YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "A video",
			URL: "https://www.youtube.com/watch?v=video-1", PublishedAt: time.Now().UTC(),
		}},
	}
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}
	if err := database.RecordSyncFailure(context.Background(), "UC123", errTestFailure, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	channel, err := database.Channel(context.Background(), "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || channel.FailureCount != 1 || channel.LastError != errTestFailure.Error() {
		t.Fatalf("video count %d, channel %#v", count, channel)
	}
}

func TestDeleteChannelCascadesToVideos(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	sync := ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos: []Video{{
			YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "A video",
			URL: "https://www.youtube.com/watch?v=video-1", PublishedAt: time.Now().UTC(),
		}},
	}
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteChannel(context.Background(), "UC123"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("videos = %d, want 0", count)
	}
}

func TestSaveSyncPurgesOnlyOldAbsentWatchedVideos(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	if err := database.SaveSync(context.Background(), ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos:  retentionVideos(now),
	}); err != nil {
		t.Fatal(err)
	}
	old := now.AddDate(0, 0, -91)
	if _, err := database.db.Exec(`UPDATE videos SET last_seen_at = ?`, formatTime(old)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`UPDATE videos SET unseen = 0 WHERE youtube_id IN ('old-watched', 'old-later')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`UPDATE videos SET watch_later = 1 WHERE youtube_id = 'old-later'`); err != nil {
		t.Fatal(err)
	}
	cutoff := now.AddDate(0, 0, -90)
	if err := database.SaveSync(context.Background(), ChannelSync{
		Channel:     Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos:      []Video{retentionVideos(now)[0]},
		PurgeBefore: &cutoff,
	}); err != nil {
		t.Fatal(err)
	}
	assertVideoPresent(t, database, "current")
	assertVideoPresent(t, database, "old-unseen")
	assertVideoPresent(t, database, "old-later")
	assertVideoMissing(t, database, "old-watched")
}

func TestMarkVideoWatchedRemovesWatchLater(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Now().UTC()
	if err := database.SaveSync(context.Background(), ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos:  []Video{{YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "A video", URL: "https://www.youtube.com/watch?v=video-1", PublishedAt: now}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`UPDATE videos SET watch_later = 1 WHERE youtube_id = 'video-1'`); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkVideoWatched(context.Background(), "video-1"); err != nil {
		t.Fatal(err)
	}
	var unseen, later int
	var watchedAt string
	if err := database.db.QueryRow(`SELECT unseen, watch_later, watched_at FROM videos WHERE youtube_id = 'video-1'`).Scan(&unseen, &later, &watchedAt); err != nil {
		t.Fatal(err)
	}
	if unseen != 0 || later != 0 || watchedAt == "" {
		t.Fatalf("unseen %d, later %d, watched at %q", unseen, later, watchedAt)
	}
}

func TestSaveSyncDeletesPreviouslyStoredShorts(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Now().UTC()
	sync := ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example"},
		Videos: []Video{{
			YouTubeID: "short", ChannelYouTubeID: "UC123", Title: "Short",
			URL: "https://www.youtube.com/shorts/short", PublishedAt: now, IsShort: true,
		}},
	}
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Short videos = %d, want 0", count)
	}
}

func TestChannelThumbnailsPreferLongVideosAndFallbackToShorts(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Now().UTC()
	sync := ChannelSync{
		Channel: Channel{
			YouTubeID: "UC123", SourceURL: "https://www.youtube.com/@example", Title: "Example",
			IncludeLong: true, IncludeShorts: true,
		},
		Videos: []Video{
			{YouTubeID: "long", ChannelYouTubeID: "UC123", Title: "Long", URL: "https://youtube.com/watch?v=long", ThumbnailURL: "long.jpg", PublishedAt: now},
			{YouTubeID: "short", ChannelYouTubeID: "UC123", Title: "Short", URL: "https://youtube.com/shorts/short", ThumbnailURL: "short.jpg", PublishedAt: now.Add(time.Minute), IsShort: true},
		},
	}
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}
	summaries, err := database.ChannelSummaries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || len(summaries[0].ThumbnailURLs) != 1 || summaries[0].ThumbnailURLs[0] != "long.jpg" {
		t.Fatalf("summaries = %#v", summaries)
	}
	if _, err := database.db.Exec(`DELETE FROM videos WHERE is_short = 0`); err != nil {
		t.Fatal(err)
	}
	summaries, err = database.ChannelSummaries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries[0].ThumbnailURLs) != 1 || summaries[0].ThumbnailURLs[0] != "short.jpg" {
		t.Fatalf("fallback thumbnails = %#v", summaries[0].ThumbnailURLs)
	}
}

func retentionVideos(published time.Time) []Video {
	return []Video{
		{YouTubeID: "current", ChannelYouTubeID: "UC123", Title: "Current", URL: "https://www.youtube.com/watch?v=current", PublishedAt: published},
		{YouTubeID: "old-watched", ChannelYouTubeID: "UC123", Title: "Old watched", URL: "https://www.youtube.com/watch?v=old-watched", PublishedAt: published},
		{YouTubeID: "old-unseen", ChannelYouTubeID: "UC123", Title: "Old unseen", URL: "https://www.youtube.com/watch?v=old-unseen", PublishedAt: published},
		{YouTubeID: "old-later", ChannelYouTubeID: "UC123", Title: "Old later", URL: "https://www.youtube.com/watch?v=old-later", PublishedAt: published},
	}
}

func assertVideoPresent(t *testing.T, database *Store, videoID string) {
	t.Helper()
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos WHERE youtube_id = ?`, videoID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("video %q count = %d, want 1", videoID, count)
	}
}

func assertVideoMissing(t *testing.T, database *Store, videoID string) {
	t.Helper()
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos WHERE youtube_id = ?`, videoID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("video %q count = %d, want 0", videoID, count)
	}
}

var errTestFailure = testError("upstream unavailable")

type testError string

func (e testError) Error() string { return string(e) }
