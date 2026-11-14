package videoduration

import (
	"context"
	"fmt"
	"sync"

	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/logger"
	"github.com/MrSnakeDoc/tuby/internal/store"
	"github.com/MrSnakeDoc/tuby/internal/youtube"
)

type durationStore interface {
	VideoDuration(context.Context, string) (*int64, error)
	SetVideoDuration(context.Context, string, int64) error
}

type fetcher interface {
	FetchDuration(context.Context, string) (int64, error)
}

type publisher interface {
	Publish(events.Update)
}

type warningLogger interface {
	Warn(string, ...logger.Field)
}

type Service struct {
	store    durationStore
	fetcher  fetcher
	updates  publisher
	logger   warningLogger
	mu       sync.Mutex
	inFlight map[string]struct{}
}

func New(database durationStore, client fetcher, log warningLogger, updates publisher) *Service {
	return &Service{
		store: database, fetcher: client, logger: log, updates: updates,
		inFlight: make(map[string]struct{}),
	}
}

func NewDefault(database *store.Store, log warningLogger, updates publisher) *Service {
	return New(database, youtube.NewDurationClient(nil), log, updates)
}

func (s *Service) Fetch(ctx context.Context, videoID string) error {
	duration, err := s.store.VideoDuration(ctx, videoID)
	if err != nil || duration != nil {
		return err
	}
	if !s.start(videoID) {
		return nil
	}
	defer s.finish(videoID)
	return s.fetch(ctx, videoID)
}

func (s *Service) fetch(ctx context.Context, videoID string) error {
	duration, err := s.store.VideoDuration(ctx, videoID)
	if err != nil || duration != nil {
		return err
	}
	seconds, err := s.fetcher.FetchDuration(ctx, videoID)
	if err != nil {
		s.warn(videoID, err)
		return fmt.Errorf("fetch duration for video %q: %w", videoID, err)
	}
	if err := s.store.SetVideoDuration(ctx, videoID, seconds); err != nil {
		return err
	}
	if s.updates != nil {
		s.updates.Publish(events.Update{
			Reason: "video.duration", Scopes: []string{"channel", "missed", "later"}, VideoIDs: []string{videoID},
		})
	}
	return nil
}

func (s *Service) start(videoID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inFlight[videoID]; exists {
		return false
	}
	s.inFlight[videoID] = struct{}{}
	return true
}

func (s *Service) finish(videoID string) {
	s.mu.Lock()
	delete(s.inFlight, videoID)
	s.mu.Unlock()
}

func (s *Service) warn(videoID string, err error) {
	if s.logger == nil || err == nil {
		return
	}
	s.logger.Warn("YouTube video duration fetch failed",
		logger.String("video_id", videoID), logger.Error(err))
}
