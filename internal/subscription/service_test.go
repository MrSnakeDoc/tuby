package subscription

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/youtube"
)

func TestAddSavesChannelAndInitialVideosAsUnseen(t *testing.T) {
	database := openTestStore(t)
	service := New(database, testResolver{}, &testFeedClient{result: feedResult()})
	channel, err := service.Add(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA")
	if err != nil {
		t.Fatal(err)
	}
	if channel.Title != "Veritasium" || channel.AvatarURL != "https://example.com/avatar.jpg" {
		t.Fatalf("channel = %#v", channel)
	}
}

func TestRefreshUsesValidatorsAndPreservesVideoStates(t *testing.T) {
	database := openTestStore(t)
	initial := &testFeedClient{result: feedResult()}
	service := New(database, testResolver{}, initial)
	if _, err := service.Add(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA"); err != nil {
		t.Fatal(err)
	}
	refresh := &testFeedClient{result: youtube.FetchResult{NotModified: true, ETag: "etag-2"}}
	service = New(database, testResolver{}, refresh)
	if err := service.Refresh(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA"); err != nil {
		t.Fatal(err)
	}
	if refresh.options.ETag != "etag-1" || refresh.options.LastModified != "last-modified-1" {
		t.Fatalf("options = %#v", refresh.options)
	}
	channel, err := database.Channel(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA")
	if err != nil {
		t.Fatal(err)
	}
	if channel.FeedETag != "etag-2" || channel.FeedLastModified != "last-modified-1" || channel.FailureCount != 0 {
		t.Fatalf("channel = %#v", channel)
	}
}

func TestRefreshRecordsFailureAndKeepsVideos(t *testing.T) {
	database := openTestStore(t)
	service := New(database, testResolver{}, &testFeedClient{result: feedResult()})
	if _, err := service.Add(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA"); err != nil {
		t.Fatal(err)
	}
	service = New(database, testResolver{}, &testFeedClient{err: errors.New("upstream unavailable")})
	if err := service.Refresh(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA"); err == nil {
		t.Fatal("Refresh succeeded")
	}
	channel, err := database.Channel(context.Background(), "UCHnyfMqiRRG1u-2MsSQLbXA")
	if err != nil {
		t.Fatal(err)
	}
	if channel.FailureCount != 1 || channel.NextCheckAt == nil || channel.LastError == "" {
		t.Fatalf("channel = %#v", channel)
	}
}

func TestRefreshAllRefreshesEveryStoredChannel(t *testing.T) {
	database := openTestStore(t)
	for _, channelID := range []string{"UCHnyfMqiRRG1u-2MsSQLbXA", "UCXuqSBlHAE6Xw-yeJA0Tunw"} {
		err := database.SaveSync(context.Background(), store.ChannelSync{
			Channel: store.Channel{YouTubeID: channelID, SourceURL: "https://www.youtube.com/channel/" + channelID, Title: channelID},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	client := &testFeedClient{result: youtube.FetchResult{NotModified: true}}
	service := New(database, testResolver{}, client)
	report, err := service.RefreshAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Refreshed != 2 || len(report.Failures) != 0 || client.calls != 2 {
		t.Fatalf("report %#v, calls %d", report, client.calls)
	}
}

func TestBackoffCapsAtMaximum(t *testing.T) {
	if got := backoff(1); got != minimumBackoff {
		t.Fatalf("first failure backoff = %s", got)
	}
	if got := backoff(100); got != maximumBackoff {
		t.Fatalf("capped backoff = %s", got)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

type testResolver struct{}

func (testResolver) ResolveChannelID(context.Context, string) (string, error) {
	return "UCHnyfMqiRRG1u-2MsSQLbXA", nil
}

type testFeedClient struct {
	result  youtube.FetchResult
	err     error
	options youtube.FetchOptions
	calls   int
}

func (c *testFeedClient) Fetch(_ context.Context, _ string, options youtube.FetchOptions) (youtube.FetchResult, error) {
	c.calls++
	c.options = options
	return c.result, c.err
}

func feedResult() youtube.FetchResult {
	return youtube.FetchResult{
		Feed: youtube.Feed{
			ChannelID: "UCHnyfMqiRRG1u-2MsSQLbXA",
			Title:     "Veritasium",
			AvatarURL: "https://example.com/avatar.jpg",
			Videos: []youtube.Video{{
				YouTubeID: "abcdefghijk", ChannelID: "UCHnyfMqiRRG1u-2MsSQLbXA",
				Title: "A real video", URL: "https://www.youtube.com/watch?v=abcdefghijk",
				Thumbnail: "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
				Published: time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC),
			}},
		},
		ETag: "etag-1", LastModified: "last-modified-1",
	}
}
