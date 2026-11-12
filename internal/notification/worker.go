package notification

import (
	"context"
	"fmt"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/navire"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
)

const defaultPollInterval = time.Second

type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
}

type outbox interface {
	NextNotification(context.Context, time.Time) (*store.OutboxNotification, error)
	RetryNotification(context.Context, int64, int, time.Time) error
	DeleteNotification(context.Context, int64) error
}

type publisher interface {
	Publish(context.Context, navire.Event) error
}

type eventLogger interface {
	Info(string, ...logger.Field)
	Warn(string, ...logger.Field)
	Error(string, ...logger.Field)
}

type Worker struct {
	outbox       outbox
	publisher    publisher
	logger       eventLogger
	config       Config
	now          func() time.Time
	pollInterval time.Duration
}

func NewWorker(database outbox, client publisher, log eventLogger, configuration Config) *Worker {
	return &Worker{
		outbox: database, publisher: client, logger: log, config: configuration,
		now: time.Now, pollInterval: defaultPollInterval,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.processDue(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processDue(ctx)
		}
	}
}

func (w *Worker) processDue(ctx context.Context) {
	for ctx.Err() == nil && w.processNext(ctx) {
	}
}

func (w *Worker) processNext(ctx context.Context) bool {
	entry, err := w.outbox.NextNotification(ctx, w.now())
	if err != nil {
		w.logError(ctx, "Notification outbox read failed", err)
		return false
	}
	if entry == nil {
		return false
	}
	if err := w.deliver(ctx, *entry); err != nil {
		w.logError(ctx, "Notification outbox update failed", err,
			logger.String("video_id", entry.VideoYouTubeID))
		return false
	}
	return true
}

func (w *Worker) deliver(ctx context.Context, entry store.OutboxNotification) error {
	err := w.publisher.Publish(ctx, navireEvent(entry))
	if err == nil {
		if err := w.outbox.DeleteNotification(ctx, entry.ID); err != nil {
			return fmt.Errorf("delete delivered notification: %w", err)
		}
		w.logger.Info("Navire notification delivered",
			logger.String("channel_id", entry.ChannelYouTubeID),
			logger.String("video_id", entry.VideoYouTubeID))
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return w.handleFailure(ctx, entry, err)
}

func (w *Worker) handleFailure(ctx context.Context, entry store.OutboxNotification, cause error) error {
	attempt := entry.AttemptCount + 1
	if attempt >= w.config.MaxAttempts {
		if err := w.outbox.DeleteNotification(ctx, entry.ID); err != nil {
			return fmt.Errorf("delete abandoned notification: %w", err)
		}
		w.logger.Error("Navire notification permanently dropped",
			logger.String("channel_id", entry.ChannelYouTubeID),
			logger.String("video_id", entry.VideoYouTubeID),
			logger.Int("attempts", attempt), logger.Error(cause))
		return nil
	}
	delay := retryDelay(attempt, w.config.InitialWait, w.config.MaxWait)
	if err := w.outbox.RetryNotification(ctx, entry.ID, attempt, w.now().Add(delay)); err != nil {
		return fmt.Errorf("reschedule failed notification: %w", err)
	}
	w.logger.Warn("Navire notification delivery failed; retry scheduled",
		logger.String("channel_id", entry.ChannelYouTubeID),
		logger.String("video_id", entry.VideoYouTubeID),
		logger.Int("attempt", attempt), logger.Int("max_attempts", w.config.MaxAttempts),
		logger.Duration("retry_in", delay), logger.Error(cause))
	return nil
}

func (w *Worker) logError(ctx context.Context, message string, err error, fields ...logger.Field) {
	if ctx.Err() != nil {
		return
	}
	w.logger.Error(message, append(fields, logger.Error(err))...)
}

func navireEvent(entry store.OutboxNotification) navire.Event {
	return navire.Event{
		Variant: string(entry.Variant), Priority: navirePriority(entry.Level),
		ChannelName: entry.ChannelTitle, VideoTitle: entry.VideoTitle, Message: entry.Message,
	}
}

func navirePriority(level store.NotificationLevel) string {
	switch level {
	case store.NotificationLow:
		return "low"
	case store.NotificationCritical:
		return "critical"
	default:
		return "normal"
	}
}

func retryDelay(attempt int, initialWait, maxWait time.Duration) time.Duration {
	delay := initialWait
	for current := 1; current < attempt && delay < maxWait; current++ {
		if delay > maxWait/2 {
			return maxWait
		}
		delay *= 2
	}
	if delay > maxWait {
		return maxWait
	}
	return delay
}
