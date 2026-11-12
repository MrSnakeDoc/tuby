package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestChannelNotificationLevelLifecyclePreservesOtherSettings(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	category, err := database.CreateCategory(context.Background(), "Tech")
	if err != nil {
		t.Fatal(err)
	}
	channel := Channel{
		YouTubeID: "UC123", SourceURL: "https://youtube.com/@example", Title: "Example",
		CategoryID: category.ID, IncludeShorts: true,
	}
	if err := database.SaveChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	firstActivation := time.Date(2026, time.November, 12, 9, 0, 0, 0, time.UTC)
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), channel.YouTubeID, NotificationMedium, firstActivation,
	); err != nil {
		t.Fatal(err)
	}
	assertChannelNotification(t, database, channel.YouTubeID, NotificationMedium, &firstActivation, category.ID)

	later := firstActivation.Add(time.Hour)
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), channel.YouTubeID, NotificationCritical, later,
	); err != nil {
		t.Fatal(err)
	}
	assertChannelNotification(t, database, channel.YouTubeID, NotificationCritical, &firstActivation, category.ID)

	if err := database.UpdateChannelNotificationLevel(
		context.Background(), channel.YouTubeID, NotificationOff, later,
	); err != nil {
		t.Fatal(err)
	}
	assertChannelNotification(t, database, channel.YouTubeID, NotificationOff, nil, category.ID)

	reactivated := later.Add(time.Hour)
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), channel.YouTubeID, NotificationLow, reactivated,
	); err != nil {
		t.Fatal(err)
	}
	assertChannelNotification(t, database, channel.YouTubeID, NotificationLow, &reactivated, category.ID)
}

func TestUpdateChannelNotificationLevelRejectsInvalidAndMissingChannels(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), "missing", NotificationLevel("urgent"), time.Now(),
	); err == nil {
		t.Fatal("invalid notification level was accepted")
	}
	if err := database.UpdateChannelNotificationLevel(
		context.Background(), "missing", NotificationLow, time.Now(),
	); !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("error = %v, want channel not found", err)
	}
}

func assertChannelNotification(
	t *testing.T,
	database *Store,
	channelID string,
	wantLevel NotificationLevel,
	wantEnabled *time.Time,
	wantCategory int64,
) {
	t.Helper()
	channel, err := database.Channel(context.Background(), channelID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.NotificationLevel != wantLevel || !sameOptionalTime(channel.NotificationEnabledAt, wantEnabled) {
		t.Fatalf("notification = (%q, %v), want (%q, %v)",
			channel.NotificationLevel, channel.NotificationEnabledAt, wantLevel, wantEnabled)
	}
	if channel.CategoryID != wantCategory || channel.IncludeLong || !channel.IncludeShorts {
		t.Fatalf("unrelated settings changed: %#v", channel)
	}
	summaries, err := database.ChannelSummaries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].NotificationLevel != wantLevel ||
		!sameOptionalTime(summaries[0].NotificationEnabledAt, wantEnabled) {
		t.Fatalf("summaries = %#v", summaries)
	}
}

func sameOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}
