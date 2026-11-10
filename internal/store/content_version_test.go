package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestChannelContentVersionChangesOnlyWithVisibleContent(t *testing.T) {
	database := openStore(t, filepath.Join(t.TempDir(), "tuby.sqlite"))
	t.Cleanup(func() { closeStore(t, database) })
	ctx := context.Background()
	now := time.Now().UTC()
	sync := ChannelSync{
		Channel: Channel{YouTubeID: "UC123", SourceURL: "https://youtube.com/@example", Title: "Example"},
		Videos: []Video{{
			YouTubeID: "video", ChannelYouTubeID: "UC123", Title: "Video",
			URL: "https://youtube.com/watch?v=video", PublishedAt: now,
		}},
	}
	if err := database.SaveSync(ctx, sync); err != nil {
		t.Fatal(err)
	}
	initial, err := database.ChannelContentVersion(ctx, "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveSync(ctx, sync); err != nil {
		t.Fatal(err)
	}
	unchanged, err := database.ChannelContentVersion(ctx, "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if initial != unchanged {
		t.Fatal("content version changed after an identical sync")
	}
	sync.Videos[0].Title = "Updated video"
	if err := database.SaveSync(ctx, sync); err != nil {
		t.Fatal(err)
	}
	changed, err := database.ChannelContentVersion(ctx, "UC123")
	if err != nil {
		t.Fatal(err)
	}
	if changed == unchanged {
		t.Fatal("content version did not change after a title update")
	}
}
