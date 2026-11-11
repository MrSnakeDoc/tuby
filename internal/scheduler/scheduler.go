package scheduler

import (
	"context"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

const (
	Interval = 15 * time.Minute
	stagger  = 2 * time.Second
)

type channelSource interface {
	Channels(context.Context) ([]store.Channel, error)
}

type refresher interface {
	Refresh(context.Context, string) error
}

type warningLogger interface {
	Warn(string, ...logger.Field)
}

type Scheduler struct {
	channels  channelSource
	refresher refresher
	interval  time.Duration
	stagger   time.Duration
	now       func() time.Time
	logger    warningLogger
}

func New(channels channelSource, refresher refresher, logs ...warningLogger) *Scheduler {
	var log warningLogger
	if len(logs) > 0 {
		log = logs[0]
	}
	return &Scheduler{
		channels: channels, refresher: refresher, interval: Interval, stagger: stagger, now: time.Now, logger: log,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.refreshDue(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshDue(ctx)
		}
	}
}

func (s *Scheduler) refreshDue(ctx context.Context) {
	channels, err := s.channels.Channels(ctx)
	if err != nil {
		s.warn(ctx, "Scheduler channel listing failed", logger.Error(err))
		return
	}
	for _, channel := range channels {
		if !due(channel, s.now(), s.interval) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if err := s.refresher.Refresh(ctx, channel.YouTubeID); err != nil {
			s.warn(ctx, "Scheduled channel refresh failed",
				logger.String("channel_id", channel.YouTubeID), logger.Error(err))
		}
		if !wait(ctx, s.stagger) {
			return
		}
	}
}

func (s *Scheduler) warn(ctx context.Context, message string, fields ...logger.Field) {
	if s.logger != nil && ctx.Err() == nil {
		s.logger.Warn(message, fields...)
	}
}

func due(channel store.Channel, now time.Time, interval time.Duration) bool {
	if channel.NextCheckAt != nil {
		return !channel.NextCheckAt.After(now)
	}
	if channel.LastSuccessfulSync == nil {
		return true
	}
	return now.Sub(*channel.LastSuccessfulSync) >= interval
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
