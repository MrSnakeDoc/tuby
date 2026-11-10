package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
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
	store    *store.Store
	resolver resolver
	feeds    feedClient
	updates  publisher
}

type publisher interface{ Publish(events.Update) }

type RefreshFailure struct {
	ChannelID string
	Err       error
}

type RefreshReport struct {
	Refreshed int
	Failures  []RefreshFailure
}

func New(database *store.Store, channelResolver resolver, client feedClient, publishers ...publisher) *Service {
	var updates publisher
	if len(publishers) > 0 {
		updates = publishers[0]
	}
	return &Service{store: database, resolver: channelResolver, feeds: client, updates: updates}
}

func NewDefault(database *store.Store, publishers ...publisher) *Service {
	return New(database, youtube.NewResolver(nil), youtube.NewFeedClient(nil), publishers...)
}

func (s *Service) Add(ctx context.Context, input string) (store.Channel, error) {
	reference, err := youtube.NormalizeChannelReference(input)
	if err != nil {
		return store.Channel{}, err
	}
	channelID, err := s.resolve(ctx, reference)
	if err != nil {
		return store.Channel{}, err
	}
	result, err := s.feeds.Fetch(ctx, channelID, youtube.FetchOptions{})
	if err != nil {
		return store.Channel{}, fmt.Errorf("fetch initial channel feed: %w", err)
	}
	if result.NotModified {
		return store.Channel{}, fmt.Errorf("initial channel feed must contain content")
	}
	if err := s.saveFeed(ctx, reference.SourceURL, channelID, result); err != nil {
		return store.Channel{}, err
	}
	return s.store.Channel(ctx, channelID)
}

func (s *Service) Refresh(ctx context.Context, channelID string) error {
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
	if err := s.saveFeed(ctx, channel.SourceURL, channelID, result); err != nil {
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

func (s *Service) resolve(ctx context.Context, reference youtube.ChannelReference) (string, error) {
	if reference.ID != "" {
		return reference.ID, nil
	}
	channelID, err := s.resolver.ResolveChannelID(ctx, reference.SourceURL)
	if err != nil {
		return "", fmt.Errorf("resolve YouTube channel: %w", err)
	}
	return channelID, nil
}

func (s *Service) saveFeed(ctx context.Context, sourceURL, channelID string, result youtube.FetchResult) error {
	if err := validFeed(result.Feed); err != nil {
		return err
	}
	if result.Feed.ChannelID != channelID {
		return fmt.Errorf("YouTube RSS feed resolved to another channel")
	}
	before, err := s.contentVersion(ctx, channelID)
	if err != nil {
		return err
	}
	sync := store.ChannelSync{
		Channel: store.Channel{
			YouTubeID: result.Feed.ChannelID,
			SourceURL: sourceURL,
			Title:     result.Feed.Title,
			AvatarURL: result.Feed.AvatarURL,
		},
		Videos:      storeVideos(result.Feed.Videos),
		ETag:        result.ETag,
		Modified:    result.LastModified,
		PurgeBefore: retentionCutoff(),
	}
	if err := s.store.SaveSync(ctx, sync); err != nil {
		return fmt.Errorf("save channel feed: %w", err)
	}
	after, err := s.store.ChannelContentVersion(ctx, channelID)
	if err != nil {
		s.publish("channel.updated", channelID)
		return nil
	}
	if before != after {
		s.publish("channel.updated", channelID)
	}
	return nil
}

func (s *Service) contentVersion(ctx context.Context, channelID string) (string, error) {
	version, err := s.store.ChannelContentVersion(ctx, channelID)
	if errors.Is(err, store.ErrChannelNotFound) {
		return "", nil
	}
	return version, err
}

func (s *Service) publish(reason, channelID string) {
	if s.updates == nil {
		return
	}
	s.updates.Publish(events.Update{Reason: reason, Scopes: events.AllScopes(), ChannelIDs: []string{channelID}})
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
		return fmt.Errorf("YouTube RSS feed has incomplete channel metadata")
	}
	return nil
}

func storeVideos(videos []youtube.Video) []store.Video {
	stored := make([]store.Video, 0, len(videos))
	for _, video := range videos {
		stored = append(stored, store.Video{
			YouTubeID:        video.YouTubeID,
			ChannelYouTubeID: video.ChannelID,
			Title:            video.Title,
			URL:              video.URL,
			ThumbnailURL:     video.Thumbnail,
			PublishedAt:      video.Published,
		})
	}
	return stored
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
