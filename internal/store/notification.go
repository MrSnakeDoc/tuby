package store

import (
	"context"
	"fmt"
	"time"
)

type NotificationLevel string

const (
	NotificationOff      NotificationLevel = "off"
	NotificationLow      NotificationLevel = "low"
	NotificationMedium   NotificationLevel = "medium"
	NotificationCritical NotificationLevel = "critical"
)

func (l NotificationLevel) Valid() bool {
	switch l {
	case NotificationOff, NotificationLow, NotificationMedium, NotificationCritical:
		return true
	default:
		return false
	}
}

func (s *Store) UpdateChannelNotificationLevel(
	ctx context.Context,
	channelID string,
	level NotificationLevel,
	now time.Time,
) error {
	if !level.Valid() {
		return fmt.Errorf("invalid notification level %q", level)
	}
	now = now.UTC()
	result, err := s.db.ExecContext(ctx, `
UPDATE channels SET
    notification_enabled_at = CASE
        WHEN ? = 'off' THEN NULL
        WHEN notification_level = 'off' OR notification_enabled_at IS NULL THEN ?
        ELSE notification_enabled_at
    END,
    notification_level = ?
WHERE youtube_id = ?`, level, formatTime(now), level, channelID)
	if err != nil {
		return fmt.Errorf("update notification level for channel %q: %w", channelID, err)
	}
	return requireChannel(result, channelID)
}
