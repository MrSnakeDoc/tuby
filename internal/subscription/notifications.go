package subscription

import (
	"crypto/rand"
	"math/big"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
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
	events := make(map[string]store.NotificationEvent)
	for _, video := range videos {
		if video.PublishedAt.Before(*channel.NotificationEnabledAt) {
			continue
		}
		message := s.selectMessage(s.notifications.Messages)
		events[video.YouTubeID] = store.NotificationEvent{
			VideoYouTubeID: video.YouTubeID, ChannelYouTubeID: channel.YouTubeID,
			ChannelTitle: channelTitle, VideoTitle: video.Title, Level: channel.NotificationLevel,
			Variant: notificationVariant(message), Message: message,
		}
	}
	return events
}

func (s *Service) notificationEligibleChannel(channel *store.Channel) bool {
	return s.notifications.Enabled && channel != nil && channel.NotificationEnabledAt != nil &&
		channel.NotificationLevel != store.NotificationOff && channel.NotificationLevel.Valid()
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
