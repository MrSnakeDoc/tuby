package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/youtube"
)

const (
	minimumBackoff  = 15 * time.Minute
	maximumBackoff  = 6 * time.Hour
	retentionPeriod = 90 * 24 * time.Hour
)

type resolver interface {
	ResolveChannelID(context.Context, string) (string, error)
}

type feedClient interface {
	Fetch(context.Context, string, youtube.FetchOptions) (youtube.FetchResult, error)
}

type Service struct {
	store         *store.Store
	resolver      resolver
	feeds         feedClient
	updates       publisher
	logger        warningLogger
	notifications NotificationConfig
	selectMessage func([]string) string
	locks         channelLocks
}

type publisher interface{ Publish(events.Update) }

type warningLogger interface {
	Warn(string, ...logger.Field)
}

type RefreshFailure struct {
	ChannelID string
	Err       error
}

type RefreshReport struct {
	Refreshed int
	Failures  []RefreshFailure
}

type ContentPreferences struct {
	IncludeLong   bool
	IncludeShorts bool
	CategoryID    int64
}

func New(database *store.Store, channelResolver resolver, client feedClient, publishers ...publisher) *Service {
	return newService(database, channelResolver, client, nil, NotificationConfig{}, publishers...)
}

func NewWithLogger(
	database *store.Store,
	channelResolver resolver,
	client feedClient,
	log warningLogger,
	publishers ...publisher,
) *Service {
	return newService(database, channelResolver, client, log, NotificationConfig{}, publishers...)
}

func newService(
	database *store.Store,
	channelResolver resolver,
	client feedClient,
	log warningLogger,
	notifications NotificationConfig,
	publishers ...publisher,
) *Service {
	var updates publisher
	if len(publishers) > 0 {
		updates = publishers[0]
	}
	return &Service{
		store: database, resolver: channelResolver, feeds: client, updates: updates, logger: log,
		notifications: copyNotificationConfig(notifications), selectMessage: randomNotificationMessage,
	}
}

func NewDefault(database *store.Store, publishers ...publisher) *Service {
	return New(database, youtube.NewResolver(nil), youtube.NewFeedClient(nil), publishers...)
}

func NewDefaultWithLogger(database *store.Store, log warningLogger, publishers ...publisher) *Service {
	return NewWithLogger(database, youtube.NewResolver(nil), youtube.NewFeedClient(nil), log, publishers...)
}

func NewDefaultWithNotifications(
	database *store.Store,
	log warningLogger,
	notifications NotificationConfig,
	publishers ...publisher,
) *Service {
	return newService(
		database, youtube.NewResolver(nil), youtube.NewFeedClient(nil), log, notifications, publishers...,
	)
}

func (s *Service) Add(ctx context.Context, input string, preferences ContentPreferences) (store.Channel, error) {
	if err := validatePreferences(preferences); err != nil {
		return store.Channel{}, err
	}
	reference, err := youtube.NormalizeChannelReference(input)
	if err != nil {
		return store.Channel{}, fmt.Errorf("%w: %w", ErrInvalidChannelReference, err)
	}
	channelID, err := s.resolve(ctx, reference)
	if err != nil {
		return store.Channel{}, err
	}
	stored, err := s.store.Channel(ctx, channelID)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, store.ErrChannelNotFound) {
		return store.Channel{}, err
	}
	preferences.CategoryID, err = s.categoryID(ctx, preferences.CategoryID)
	if err != nil {
		return store.Channel{}, err
	}
	result, err := s.feeds.Fetch(ctx, channelID, youtube.FetchOptions{})
	if err != nil {
		return store.Channel{}, fmt.Errorf("%w: %w", ErrChannelFeedUnavailable, err)
	}
	if result.NotModified {
		return store.Channel{}, fmt.Errorf("%w: initial channel feed must contain content", ErrChannelFeedUnavailable)
	}
	if err := s.saveFeed(ctx, reference.SourceURL, channelID, preferences, result, nil); err != nil {
		return store.Channel{}, err
	}
	return s.store.Channel(ctx, channelID)
}

func (s *Service) Refresh(ctx context.Context, channelID string) error {
	unlock := s.lockChannel(channelID)
	defer unlock()
	return s.refresh(ctx, channelID)
}

func (s *Service) refresh(ctx context.Context, channelID string) error {
	channel, err := s.store.Channel(ctx, channelID)
	if err != nil {
		return err
	}
	options := youtube.FetchOptions{ETag: channel.FeedETag, LastModified: channel.FeedLastModified}
	result, err := s.feeds.Fetch(ctx, channelID, options)
	if err != nil {
		return s.recordFailure(ctx, channel, err)
	}
	if result.NotModified {
		return s.store.RecordNotModified(ctx, channelID, keepValue(result.ETag, channel.FeedETag), keepValue(result.LastModified, channel.FeedLastModified))
	}
	preferences := ContentPreferences{
		IncludeLong: channel.IncludeLong, IncludeShorts: channel.IncludeShorts, CategoryID: channel.CategoryID,
	}
	if err := s.saveFeed(ctx, channel.SourceURL, channelID, preferences, result, &channel); err != nil {
		return err
	}
	return nil
}

func (s *Service) RefreshAll(ctx context.Context) (RefreshReport, error) {
	channels, err := s.store.Channels(ctx)
	if err != nil {
		return RefreshReport{}, err
	}
	report := RefreshReport{Failures: make([]RefreshFailure, 0)}
	for _, channel := range channels {
		if err := s.Refresh(ctx, channel.YouTubeID); err != nil {
			report.Failures = append(report.Failures, RefreshFailure{ChannelID: channel.YouTubeID, Err: err})
			continue
		}
		report.Refreshed++
	}
	return report, nil
}

func (s *Service) Delete(ctx context.Context, channelID string) error {
	if err := s.store.DeleteChannel(ctx, channelID); err != nil {
		return err
	}
	s.publish("channel.deleted", channelID)
	return nil
}

func (s *Service) UpdateSettings(ctx context.Context, channelID string, preferences ContentPreferences) error {
	if err := validatePreferences(preferences); err != nil {
		return err
	}
	unlock := s.lockChannel(channelID)
	if err := s.store.UpdateChannelSettings(
		ctx, channelID, preferences.CategoryID, preferences.IncludeLong, preferences.IncludeShorts,
	); err != nil {
		unlock()
		return err
	}
	unlock()
	s.publish("channel.settings_changed", channelID, preferences.CategoryID)
	if err := s.Refresh(ctx, channelID); err != nil && s.logger != nil && ctx.Err() == nil {
		s.logger.Warn("Channel refresh after settings update failed",
			logger.String("channel_id", channelID), logger.Error(err))
	}
	return nil
}

func (s *Service) UpdateNotificationLevel(
	ctx context.Context,
	channelID string,
	level store.NotificationLevel,
) error {
	unlock := s.lockChannel(channelID)
	defer unlock()
	return s.store.UpdateChannelNotificationLevel(ctx, channelID, level, time.Now())
}

func (s *Service) resolve(ctx context.Context, reference youtube.ChannelReference) (string, error) {
	if reference.ID != "" {
		return reference.ID, nil
	}
	channelID, err := s.resolver.ResolveChannelID(ctx, reference.SourceURL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrChannelResolution, err)
	}
	return channelID, nil
}

func (s *Service) saveFeed(
	ctx context.Context,
	sourceURL, channelID string,
	preferences ContentPreferences,
	result youtube.FetchResult,
	notificationChannel *store.Channel,
) error {
	if err := validFeed(result.Feed); err != nil {
		return err
	}
	if result.Feed.ChannelID != channelID {
		return fmt.Errorf("%w: resolved to another channel", ErrChannelFeedUnavailable)
	}
	before, err := s.contentVersion(ctx, channelID)
	if err != nil {
		return err
	}
	videos := storeVideos(result.Feed.Videos, preferences)
	sync := store.ChannelSync{
		Channel: store.Channel{
			YouTubeID:   result.Feed.ChannelID,
			CategoryID:  preferences.CategoryID,
			SourceURL:   sourceURL,
			Title:       result.Feed.Title,
			AvatarURL:   result.Feed.AvatarURL,
			IncludeLong: preferences.IncludeLong, IncludeShorts: preferences.IncludeShorts,
		},
		Videos:             videos,
		NotificationEvents: s.notificationEvents(notificationChannel, result.Feed.Title, videos),
		ETag:               result.ETag,
		Modified:           result.LastModified,
		PurgeBefore:        retentionCutoff(),
	}
	if err := s.store.SaveSync(ctx, sync); err != nil {
		return fmt.Errorf("save channel feed: %w", err)
	}
	after, err := s.store.ChannelContentVersion(ctx, channelID)
	if err != nil {
		s.publish("channel.updated", channelID, preferences.CategoryID)
		return nil
	}
	if before != after {
		s.publish("channel.updated", channelID, preferences.CategoryID)
	}
	return nil
}

func (s *Service) categoryID(ctx context.Context, categoryID int64) (int64, error) {
	if categoryID == 0 {
		return store.DefaultCategoryID, nil
	}
	if _, err := s.store.Category(ctx, categoryID); err != nil {
		return 0, err
	}
	return categoryID, nil
}

func (s *Service) contentVersion(ctx context.Context, channelID string) (string, error) {
	version, err := s.store.ChannelContentVersion(ctx, channelID)
	if errors.Is(err, store.ErrChannelNotFound) {
		return "", nil
	}
	return version, err
}

func (s *Service) publish(reason, channelID string, categoryIDs ...int64) {
	if s.updates == nil {
		return
	}
	validCategoryIDs := make([]int64, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		if categoryID > 0 {
			validCategoryIDs = append(validCategoryIDs, categoryID)
		}
	}
	s.updates.Publish(events.Update{
		Reason: reason, Scopes: events.AllScopes(), CategoryIDs: validCategoryIDs, ChannelIDs: []string{channelID},
	})
}

func retentionCutoff() *time.Time {
	cutoff := time.Now().UTC().Add(-retentionPeriod)
	return &cutoff
}

func (s *Service) recordFailure(ctx context.Context, channel store.Channel, cause error) error {
	nextCheck := time.Now().UTC().Add(backoff(channel.FailureCount + 1))
	if err := s.store.RecordSyncFailure(ctx, channel.YouTubeID, cause, nextCheck); err != nil {
		return err
	}
	return fmt.Errorf("refresh channel %q: %w", channel.YouTubeID, cause)
}

func validFeed(feed youtube.Feed) error {
	if feed.ChannelID == "" || feed.Title == "" {
		return fmt.Errorf("%w: incomplete channel metadata", ErrChannelFeedUnavailable)
	}
	return nil
}

func storeVideos(videos []youtube.Video, preferences ContentPreferences) []store.Video {
	stored := make([]store.Video, 0, len(videos))
	for _, video := range videos {
		if video.IsShort && !preferences.IncludeShorts || !video.IsShort && !preferences.IncludeLong {
			continue
		}
		stored = append(stored, store.Video{
			YouTubeID:        video.YouTubeID,
			ChannelYouTubeID: video.ChannelID,
			Title:            video.Title,
			URL:              video.URL,
			ThumbnailURL:     video.Thumbnail,
			PublishedAt:      video.Published,
			IsShort:          video.IsShort,
		})
	}
	return stored
}

func validatePreferences(preferences ContentPreferences) error {
	if !preferences.IncludeLong && !preferences.IncludeShorts {
		return ErrVideoTypesRequired
	}
	return nil
}

func backoff(failures int) time.Duration {
	delay := minimumBackoff
	for failures > 1 && delay < maximumBackoff {
		delay *= 2
		failures--
	}
	if delay > maximumBackoff {
		return maximumBackoff
	}
	return delay
}

func keepValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
