package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/navire"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

func TestWorkerDeliversEveryPriorityAndDeletesSuccessfulEntries(t *testing.T) {
	now := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	outbox := &stubOutbox{entries: []store.OutboxNotification{
		outboxEntry(1, store.NotificationLow, now),
		outboxEntry(2, store.NotificationMedium, now),
		outboxEntry(3, store.NotificationCritical, now),
	}}
	publisher := &stubPublisher{}
	worker := testWorker(outbox, publisher, &stubLogger{}, now)
	worker.processDue(context.Background())
	if len(publisher.events) != 3 || len(outbox.entries) != 0 {
		t.Fatalf("events = %#v, remaining = %#v", publisher.events, outbox.entries)
	}
	priorities := []string{publisher.events[0].Priority, publisher.events[1].Priority, publisher.events[2].Priority}
	if priorities[0] != "low" || priorities[1] != "normal" || priorities[2] != "critical" {
		t.Fatalf("priorities = %v", priorities)
	}
}

func TestWorkerRetriesWithExponentialBackoffThenDrops(t *testing.T) {
	now := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	outbox := &stubOutbox{entries: []store.OutboxNotification{outboxEntry(1, store.NotificationLow, now)}}
	publisher := &stubPublisher{err: errors.New("Navire unavailable")}
	logs := &stubLogger{}
	worker := testWorker(outbox, publisher, logs, now)

	worker.processDue(context.Background())
	if outbox.entries[0].AttemptCount != 1 || !outbox.entries[0].AvailableAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("first retry = %#v", outbox.entries[0])
	}
	worker.now = func() time.Time { return now.Add(2 * time.Second) }
	worker.processDue(context.Background())
	if outbox.entries[0].AttemptCount != 2 || !outbox.entries[0].AvailableAt.Equal(now.Add(6*time.Second)) {
		t.Fatalf("second retry = %#v", outbox.entries[0])
	}
	worker.now = func() time.Time { return now.Add(6 * time.Second) }
	worker.processDue(context.Background())
	if len(outbox.entries) != 0 || len(publisher.events) != 3 || logs.warnings != 2 || logs.errors != 1 {
		t.Fatalf("remaining = %d, calls = %d, warnings = %d, errors = %d",
			len(outbox.entries), len(publisher.events), logs.warnings, logs.errors)
	}
}

func TestWorkerPreservesEntryWhenShutdownCancelsDelivery(t *testing.T) {
	now := time.Now().UTC()
	outbox := &stubOutbox{entries: []store.OutboxNotification{outboxEntry(1, store.NotificationLow, now)}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := testWorker(outbox, &stubPublisher{err: context.Canceled}, &stubLogger{}, now)
	worker.processDue(ctx)
	if len(outbox.entries) != 1 || outbox.entries[0].AttemptCount != 0 {
		t.Fatalf("outbox after cancellation = %#v", outbox.entries)
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	if delay := retryDelay(1, 2*time.Second, 30*time.Second); delay != 2*time.Second {
		t.Fatalf("first delay = %s", delay)
	}
	if delay := retryDelay(100, 2*time.Second, 30*time.Second); delay != 30*time.Second {
		t.Fatalf("bounded delay = %s", delay)
	}
}

func testWorker(outbox *stubOutbox, publisher *stubPublisher, logs *stubLogger, now time.Time) *Worker {
	worker := NewWorker(outbox, publisher, logs, Config{
		MaxAttempts: 3, InitialWait: 2 * time.Second, MaxWait: 30 * time.Second,
	})
	worker.now = func() time.Time { return now }
	return worker
}

func outboxEntry(id int64, level store.NotificationLevel, availableAt time.Time) store.OutboxNotification {
	return store.OutboxNotification{
		ID: id, AvailableAt: availableAt,
		NotificationEvent: store.NotificationEvent{
			VideoYouTubeID: "video", ChannelYouTubeID: "channel", ChannelTitle: "Channel",
			VideoTitle: "Video", Level: level, Variant: store.NotificationVariantVideo,
		},
	}
}

type stubOutbox struct {
	entries []store.OutboxNotification
}

func (s *stubOutbox) NextNotification(_ context.Context, now time.Time) (*store.OutboxNotification, error) {
	for index := range s.entries {
		if !s.entries[index].AvailableAt.After(now) {
			entry := s.entries[index]
			return &entry, nil
		}
	}
	return nil, nil
}

func (s *stubOutbox) RetryNotification(_ context.Context, id int64, attempts int, availableAt time.Time) error {
	for index := range s.entries {
		if s.entries[index].ID == id {
			s.entries[index].AttemptCount = attempts
			s.entries[index].AvailableAt = availableAt
			return nil
		}
	}
	return errors.New("missing outbox entry")
}

func (s *stubOutbox) DeleteNotification(_ context.Context, id int64) error {
	for index := range s.entries {
		if s.entries[index].ID == id {
			s.entries = append(s.entries[:index], s.entries[index+1:]...)
			return nil
		}
	}
	return errors.New("missing outbox entry")
}

type stubPublisher struct {
	events []navire.Event
	err    error
}

func (p *stubPublisher) Publish(_ context.Context, event navire.Event) error {
	p.events = append(p.events, event)
	return p.err
}

type stubLogger struct {
	warnings int
	errors   int
}

func (*stubLogger) Info(string, ...logger.Field)    {}
func (l *stubLogger) Warn(string, ...logger.Field)  { l.warnings++ }
func (l *stubLogger) Error(string, ...logger.Field) { l.errors++ }
