package deps

import (
	"context"
	"time"

	"github.com/MrSnakeDoc/tuby/internal/events"
	"github.com/MrSnakeDoc/tuby/internal/store"
	"github.com/MrSnakeDoc/tuby/internal/subscription"
	"github.com/MrSnakeDoc/tuby/internal/web"
)

type Subscriptions interface {
	Add(context.Context, string, subscription.ContentPreferences) (store.Channel, error)
	Delete(context.Context, string) error
	Refresh(context.Context, string) error
	RefreshAll(context.Context) (subscription.RefreshReport, error)
	UpdateSettings(context.Context, string, subscription.ContentPreferences) error
	UpdateNotificationLevel(context.Context, string, store.NotificationLevel) error
}

type VideoDurations interface {
	Fetch(context.Context, string) error
}

type Deps struct {
	Templates            *web.Renderer
	Store                *store.Store
	Subscriptions        Subscriptions
	VideoDurations       VideoDurations
	Updates              *events.Hub
	NotificationsEnabled bool
	StartTime            time.Time
	Version              string
	BuildID              string
	Commit               string
	BuildDate            string
	GoVersion            string
}
