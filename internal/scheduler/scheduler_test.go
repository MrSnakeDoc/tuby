package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MrSnakeDoc/tuby/internal/logger"
	"github.com/MrSnakeDoc/tuby/internal/store"
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

func TestRefreshDueLogsListingAndRefreshFailures(t *testing.T) {
	listingLog := &warningRecorder{}
	listing := &Scheduler{
		channels: staticChannels{err: errors.New("database unavailable")}, refresher: &recordingRefresher{},
		interval: Interval, stagger: 0, now: time.Now, logger: listingLog,
	}
	listing.refreshDue(context.Background())
	if len(listingLog.messages) != 1 || listingLog.messages[0] != "Scheduler channel listing failed" {
		t.Fatalf("listing warnings = %v", listingLog.messages)
	}

	refreshLog := &warningRecorder{}
	refresh := &Scheduler{
		channels:  staticChannels{channels: []store.Channel{{YouTubeID: "UC-failing"}}},
		refresher: &recordingRefresher{err: errors.New("upstream unavailable")},
		interval:  Interval, stagger: 0, now: time.Now, logger: refreshLog,
	}
	refresh.refreshDue(context.Background())
	if len(refreshLog.messages) != 1 || refreshLog.messages[0] != "Scheduled channel refresh failed" ||
		!hasStringField(refreshLog.fields[0], "channel_id", "UC-failing") {
		t.Fatalf("refresh warnings = %v, fields = %#v", refreshLog.messages, refreshLog.fields)
	}
}

type staticChannels struct {
	channels []store.Channel
	err      error
}

func (s staticChannels) Channels(context.Context) ([]store.Channel, error) { return s.channels, s.err }

type recordingRefresher struct {
	mutex      sync.Mutex
	channelIDs []string
	err        error
}

func (r *recordingRefresher) Refresh(_ context.Context, channelID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.channelIDs = append(r.channelIDs, channelID)
	return r.err
}

type warningRecorder struct {
	messages []string
	fields   [][]logger.Field
}

func (r *warningRecorder) Warn(message string, fields ...logger.Field) {
	r.messages = append(r.messages, message)
	r.fields = append(r.fields, fields)
}

func hasStringField(fields []logger.Field, key, value string) bool {
	for _, field := range fields {
		if field.Key == key && field.String == value {
			return true
		}
	}
	return false
}

func (r *recordingRefresher) ids() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return append([]string(nil), r.channelIDs...)
}
