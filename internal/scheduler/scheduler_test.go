package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

func TestRefreshDueSkipsRecentAndBackedOffChannels(t *testing.T) {
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	later := now.Add(time.Minute)
	refresher := &recordingRefresher{}
	scheduler := &Scheduler{
		channels: staticChannels{channels: []store.Channel{
			{YouTubeID: "due"},
			{YouTubeID: "recent", LastSuccessfulSync: &recent},
			{YouTubeID: "backoff", NextCheckAt: &later},
		}},
		refresher: refresher, interval: Interval, stagger: 0, now: func() time.Time { return now },
	}
	scheduler.refreshDue(context.Background())
	if got := refresher.ids(); len(got) != 1 || got[0] != "due" {
		t.Fatalf("refreshed = %v, want [due]", got)
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	scheduler := &Scheduler{channels: staticChannels{}, refresher: &recordingRefresher{}, interval: time.Hour, stagger: 0, now: time.Now}
	go func() { scheduler.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}

type staticChannels struct{ channels []store.Channel }

func (s staticChannels) Channels(context.Context) ([]store.Channel, error) { return s.channels, nil }

type recordingRefresher struct {
	mutex      sync.Mutex
	channelIDs []string
}

func (r *recordingRefresher) Refresh(_ context.Context, channelID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.channelIDs = append(r.channelIDs, channelID)
	return nil
}

func (r *recordingRefresher) ids() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return append([]string(nil), r.channelIDs...)
}
