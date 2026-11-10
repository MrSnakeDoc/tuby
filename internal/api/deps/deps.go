package deps

import (
	"context"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/subscription"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/web"
)

type Subscriptions interface {
	Add(context.Context, string, subscription.ContentPreferences) (store.Channel, error)
	Delete(context.Context, string) error
	Refresh(context.Context, string) error
	RefreshAll(context.Context) (subscription.RefreshReport, error)
	UpdateSettings(context.Context, string, subscription.ContentPreferences) error
}

type Deps struct {
	Templates     *web.Renderer
	Store         *store.Store
	Subscriptions Subscriptions
	Updates       *events.Hub
	StartTime     time.Time
	Version       string
	Commit        string
	BuildDate     string
	GoVersion     string
}
