package videoduration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
)

func TestFetchDeduplicatesConcurrentRequestsAndPublishesUpdate(t *testing.T) {
	database := &durationStoreStub{exists: true}
	fetcher := &blockingFetcher{started: make(chan struct{}, 1), release: make(chan struct{})}
	updates := &updateRecorder{}
	service := New(database, fetcher, nil, updates)
	firstResult := make(chan error, 1)
	go func() { firstResult <- service.Fetch(context.Background(), "dQw4w9WgXcQ") }()
	<-fetcher.started
	for index := 0; index < 8; index++ {
		if err := service.Fetch(context.Background(), "dQw4w9WgXcQ"); err != nil {
			t.Fatal(err)
		}
	}
	close(fetcher.release)
	if err := <-firstResult; err != nil {
		t.Fatal(err)
	}
	if fetcher.callCount() != 1 || database.storedDuration() != 754 {
		t.Fatalf("fetches = %d, duration = %d", fetcher.callCount(), database.storedDuration())
	}
	update := updates.lastUpdate()
	if update.Reason != "video.duration" || len(update.VideoIDs) != 1 || update.VideoIDs[0] != "dQw4w9WgXcQ" {
		t.Fatalf("update = %#v", update)
	}
}

func TestFetchKnownDurationDoesNotContactYouTubeOrPublish(t *testing.T) {
	duration := int64(42)
	database := &durationStoreStub{exists: true, duration: &duration}
	fetcher := &blockingFetcher{}
	updates := &updateRecorder{}
	service := New(database, fetcher, nil, updates)
	if err := service.Fetch(context.Background(), "dQw4w9WgXcQ"); err != nil {
		t.Fatal(err)
	}
	if fetcher.callCount() != 0 || updates.count() != 0 {
		t.Fatalf("fetches = %d, updates = %d", fetcher.callCount(), updates.count())
	}
}

func TestFetchFailureReleasesVideoForRetry(t *testing.T) {
	database := &durationStoreStub{exists: true}
	fetcher := &sequenceFetcher{errors: []error{errors.New("unavailable"), nil}, duration: 91}
	service := New(database, fetcher, nil, nil)
	if err := service.Fetch(context.Background(), "dQw4w9WgXcQ"); err == nil {
		t.Fatal("first fetch unexpectedly succeeded")
	}
	if err := service.Fetch(context.Background(), "dQw4w9WgXcQ"); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 2 || database.storedDuration() != 91 {
		t.Fatalf("fetches = %d, duration = %d", fetcher.calls, database.storedDuration())
	}
}

func TestFetchRejectsMissingVideoBeforeContactingYouTube(t *testing.T) {
	database := &durationStoreStub{}
	fetcher := &blockingFetcher{}
	service := New(database, fetcher, nil, nil)
	err := service.Fetch(context.Background(), "dQw4w9WgXcQ")
	if !errors.Is(err, store.ErrVideoNotFound) || fetcher.callCount() != 0 {
		t.Fatalf("error = %v, fetches = %d", err, fetcher.callCount())
	}
}

type durationStoreStub struct {
	mu       sync.Mutex
	exists   bool
	duration *int64
}

func (s *durationStoreStub) VideoDuration(context.Context, string) (*int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exists {
		return nil, store.ErrVideoNotFound
	}
	if s.duration == nil {
		return nil, nil
	}
	value := *s.duration
	return &value, nil
}

func (s *durationStoreStub) SetVideoDuration(_ context.Context, _ string, seconds int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exists {
		return store.ErrVideoNotFound
	}
	s.duration = &seconds
	return nil
}

func (s *durationStoreStub) storedDuration() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.duration == nil {
		return 0
	}
	return *s.duration
}

type blockingFetcher struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (f *blockingFetcher) FetchDuration(ctx context.Context, _ string) (int64, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return 754, nil
}

func (f *blockingFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type sequenceFetcher struct {
	calls    int
	errors   []error
	duration int64
}

func (f *sequenceFetcher) FetchDuration(context.Context, string) (int64, error) {
	err := f.errors[f.calls]
	f.calls++
	return f.duration, err
}

type updateRecorder struct {
	mu      sync.Mutex
	updates []events.Update
}

func (r *updateRecorder) Publish(update events.Update) {
	r.mu.Lock()
	r.updates = append(r.updates, update)
	r.mu.Unlock()
}

func (r *updateRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.updates)
}

func (r *updateRecorder) lastUpdate() events.Update {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updates[len(r.updates)-1]
}
