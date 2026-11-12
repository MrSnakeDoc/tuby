package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSyncEnqueuesNotificationOnlyForNewVideoAndFreezesPayload(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	now := time.Date(2026, time.November, 12, 10, 0, 0, 0, time.UTC)
	video := Video{
		YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "Original video",
		URL: "https://youtube.com/watch?v=video-1", PublishedAt: now,
	}
	sync := notificationSync(video, NotificationEvent{
		VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: "UC123",
		ChannelTitle: "Original channel", VideoTitle: video.Title, Level: NotificationMedium,
		Variant: NotificationVariantVideoWithMessage, Message: "Original message",
	})
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}

	video.Title = "Updated video"
	sync = notificationSync(video, NotificationEvent{
		VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: "UC123",
		ChannelTitle: "Updated channel", VideoTitle: video.Title, Level: NotificationCritical,
		Variant: NotificationVariantVideoWithMessage, Message: "Updated message",
	})
	if err := database.SaveSync(context.Background(), sync); err != nil {
		t.Fatal(err)
	}

	entries, err := database.PendingNotifications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.ChannelTitle != "Original channel" || entry.VideoTitle != "Original video" ||
		entry.Level != NotificationMedium || entry.Variant != NotificationVariantVideoWithMessage ||
		entry.Message != "Original message" || entry.AttemptCount != 0 {
		t.Fatalf("outbox entry = %#v", entry)
	}
}

func TestSaveSyncRollsBackVideoWhenOutboxInsertFails(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	_, err := database.db.Exec(`CREATE TRIGGER reject_notification_outbox
BEFORE INSERT ON notification_outbox BEGIN SELECT RAISE(ABORT, 'reject outbox'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	video := Video{
		YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "Video",
		URL: "https://youtube.com/watch?v=video-1", PublishedAt: time.Now().UTC(),
	}
	sync := notificationSync(video, NotificationEvent{
		VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: "UC123",
		ChannelTitle: "Channel", VideoTitle: video.Title, Level: NotificationLow,
		Variant: NotificationVariantVideo,
	})
	if err := database.SaveSync(context.Background(), sync); err == nil {
		t.Fatal("SaveSync succeeded while the outbox insert was rejected")
	}
	if _, err := database.Channel(context.Background(), "UC123"); !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("channel error = %v, want not found after rollback", err)
	}
	var videos int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&videos); err != nil {
		t.Fatal(err)
	}
	if videos != 0 {
		t.Fatalf("videos after rollback = %d, want 0", videos)
	}
}

func TestNotificationOutboxRetryAndDeleteLifecycle(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	video := Video{
		YouTubeID: "video-1", ChannelYouTubeID: "UC123", Title: "Video",
		URL: "https://youtube.com/watch?v=video-1", PublishedAt: time.Now().UTC(),
	}
	if err := database.SaveSync(context.Background(), notificationSync(video, NotificationEvent{
		VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: "UC123",
		ChannelTitle: "Channel", VideoTitle: video.Title, Level: NotificationLow,
		Variant: NotificationVariantVideo,
	})); err != nil {
		t.Fatal(err)
	}
	entry, err := database.NextNotification(context.Background(), time.Now().UTC().Add(time.Minute))
	if err != nil || entry == nil {
		t.Fatalf("next notification = %#v, error = %v", entry, err)
	}
	nextAttempt := time.Now().UTC().Add(time.Hour)
	if err := database.RetryNotification(context.Background(), entry.ID, 1, nextAttempt); err != nil {
		t.Fatal(err)
	}
	entry, err = database.NextNotification(context.Background(), nextAttempt.Add(-time.Second))
	if err != nil || entry != nil {
		t.Fatalf("early notification = %#v, error = %v", entry, err)
	}
	entry, err = database.NextNotification(context.Background(), nextAttempt)
	if err != nil || entry == nil || entry.AttemptCount != 1 {
		t.Fatalf("retried notification = %#v, error = %v", entry, err)
	}
	if err := database.DeleteNotification(context.Background(), entry.ID); err != nil {
		t.Fatal(err)
	}
	entries, err := database.PendingNotifications(context.Background())
	if err != nil || len(entries) != 0 {
		t.Fatalf("remaining notifications = %#v, error = %v", entries, err)
	}
}

func notificationSync(video Video, event NotificationEvent) ChannelSync {
	return ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://youtube.com/@channel", Title: "Channel"},
		Videos:  []Video{video}, NotificationEvents: map[string]NotificationEvent{video.YouTubeID: event},
	}
}
