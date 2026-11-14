package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestVideoDurationPersistsAndSurvivesFeedUpdates(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	video := saveDurationTestVideo(t, database)
	duration, err := database.VideoDuration(context.Background(), video.YouTubeID)
	if err != nil || duration != nil {
		t.Fatalf("initial duration = %v, error = %v", duration, err)
	}
	if err := database.SetVideoDuration(context.Background(), video.YouTubeID, 754); err != nil {
		t.Fatal(err)
	}
	video.Title = "Updated title"
	if err := database.SaveVideo(context.Background(), video); err != nil {
		t.Fatal(err)
	}
	duration, err = database.VideoDuration(context.Background(), video.YouTubeID)
	if err != nil || duration == nil || *duration != 754 {
		t.Fatalf("stored duration = %v, error = %v", duration, err)
	}
	videos, err := database.ChannelVideos(context.Background(), video.ChannelYouTubeID)
	if err != nil || len(videos) != 1 || videos[0].DurationSeconds == nil || *videos[0].DurationSeconds != 754 {
		t.Fatalf("videos = %#v, error = %v", videos, err)
	}
}

func TestVideoDurationRejectsMissingVideoAndInvalidValue(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	if _, err := database.VideoDuration(context.Background(), "missing"); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("read error = %v", err)
	}
	if err := database.SetVideoDuration(context.Background(), "missing", 12); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("write error = %v", err)
	}
	if err := database.SetVideoDuration(context.Background(), "missing", 0); err == nil {
		t.Fatal("zero duration was accepted")
	}
}

func saveDurationTestVideo(t *testing.T, database *Store) Video {
	t.Helper()
	channel := Channel{YouTubeID: "UC123", SourceURL: "https://youtube.com/@example", Title: "Example"}
	if err := database.SaveChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	video := Video{
		YouTubeID: "dQw4w9WgXcQ", ChannelYouTubeID: channel.YouTubeID, Title: "Video",
		URL: "https://youtube.com/watch?v=dQw4w9WgXcQ", PublishedAt: time.Now().UTC(),
	}
	if err := database.SaveVideo(context.Background(), video); err != nil {
		t.Fatal(err)
	}
	return video
}
