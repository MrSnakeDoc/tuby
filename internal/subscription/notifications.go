package subscription

import (
	"crypto/rand"
	"math/big"

	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
)

type NotificationConfig struct {
	Enabled  bool
	Messages []string
}

func copyNotificationConfig(configuration NotificationConfig) NotificationConfig {
	return NotificationConfig{
		Enabled:  configuration.Enabled,
		Messages: append([]string(nil), configuration.Messages...),
	}
}

func (s *Service) notificationEvents(
	channel *store.Channel,
	channelTitle string,
	videos []store.Video,
) map[string]store.NotificationEvent {
	if !s.notificationEligibleChannel(channel) {
		return nil
	}
	notifications := make(map[string]store.NotificationEvent)
	for _, video := range videos {
		if video.PublishedAt.Before(*channel.NotificationEnabledAt) {
			continue
		}
		message := s.selectMessage(s.notifications.Messages)
		notifications[video.YouTubeID] = store.NotificationEvent{
			VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: channel.YouTubeID,
			ChannelTitle: channelTitle, VideoTitle: video.Title, Level: channel.NotificationLevel,
			Variant: notificationVariant(message), Message: message,
		}
	}
	return notifications
}

func (s *Service) notificationEligibleChannel(channel *store.Channel) bool {
	return s.notifications.Enabled && channel != nil && channel.NotificationEnabledAt != nil &&
		channel.NotificationLevel != store.NotificationOff && channel.NotificationLevel.Valid()
}

func browserNotifications(
	channel *store.Channel,
	channelTitle string,
	videos []store.Video,
) []events.BrowserNotification {
	if channel == nil || channel.NotificationEnabledAt == nil ||
		channel.NotificationLevel == store.NotificationOff || !channel.NotificationLevel.Valid() {
		return nil
	}
	result := make([]events.BrowserNotification, 0, len(videos))
	for _, video := range videos {
		if video.PublishedAt.Before(*channel.NotificationEnabledAt) {
			continue
		}
		result = append(result, events.BrowserNotification{
			VideoID: video.YouTubeID, ChannelTitle: channelTitle,
			VideoTitle: video.Title, ThumbnailURL: video.ThumbnailURL,
		})
	}
	return result
}

func notificationVariant(message string) store.NotificationVariant {
	if message == "" {
		return store.NotificationVariantVideo
	}
	return store.NotificationVariantVideoWithMessage
}

func randomNotificationMessage(messages []string) string {
	if len(messages) == 0 {
		return ""
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(messages))))
	if err != nil {
		return ""
	}
	return messages[index.Int64()]
}
