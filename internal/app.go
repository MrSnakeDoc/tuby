package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/config"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/navire"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/notification"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/scheduler"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/store"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/subscription"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/version"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/videoduration"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/web"
)

const shutdownTimeout = 10 * time.Second

type App struct {
	cfg       *config.Config
	logger    logger.Logger
	server    *api.Server
	store     *store.Store
	scheduler *scheduler.Scheduler
	notifier  *notification.Worker
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	log := logger.NewWithOptions(logger.Options{
		Level:   cfg.LogLevel,
		Pretty:  cfg.PrettyLog,
		Service: "tuby",
	})
	templates, err := web.New()
	if err != nil {
		return nil, fmt.Errorf("initialize web templates: %w", err)
	}
	database, err := store.Open(store.DefaultPath)
	if err != nil {
		return nil, fmt.Errorf("open data store: %w", err)
	}
	buildID := version.BuildID()
	updates := events.New(buildID)
	subscriptions := subscription.NewDefaultWithNotifications(
		database,
		log.Named("subscriptions"),
		subscription.NotificationConfig{Enabled: cfg.Navire.Enabled, Messages: cfg.Navire.Messages},
		updates,
	)
	videoDurations := videoduration.NewDefault(database, log.Named("video_duration"), updates)
	var notifier *notification.Worker
	if cfg.Navire.Enabled {
		notifier = notification.NewWorker(
			database,
			navire.NewClient(cfg.Navire.URL, cfg.Navire.RequestTimeout),
			log.Named("notifications"),
			notification.Config{
				MaxAttempts: cfg.Navire.MaxAttempts,
				InitialWait: cfg.Navire.RetryInitialWait,
				MaxWait:     cfg.Navire.RetryMaxWait,
			},
		)
	}
	dependencies := deps.Deps{
		Templates:            templates,
		Store:                database,
		Subscriptions:        subscriptions,
		VideoDurations:       videoDurations,
		Updates:              updates,
		NotificationsEnabled: cfg.Navire.Enabled,
		StartTime:            time.Now(),
		Version:              version.Version,
		BuildID:              buildID,
		Commit:               version.Commit,
		BuildDate:            version.BuildDate,
		GoVersion:            runtime.Version(),
	}
	return &App{
		cfg: cfg, logger: log, server: api.New(cfg, log, dependencies), store: database,
		scheduler: scheduler.New(database, subscriptions, log.Named("scheduler")), notifier: notifier,
	}, nil
}

func (a *App) Run() error {
	defer func() { _ = a.logger.Sync() }()
	defer func() { _ = a.store.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.RunContext(ctx)
}

func (a *App) RunContext(ctx context.Context) error {
	a.logStartup()
	stopBackground := a.startBackground(ctx)
	defer stopBackground()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- a.server.Start()
	}()
	select {
	case <-ctx.Done():
		a.logger.Info("Shutdown requested")
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := a.server.Stop(shutdownContext); err != nil {
			return fmt.Errorf("stop HTTP server: %w", err)
		}
		return nil
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Error("HTTP server stopped unexpectedly", logger.Error(err))
			return fmt.Errorf("run HTTP server: %w", err)
		}
		return nil
	}
}

func (a *App) startBackground(ctx context.Context) func() {
	backgroundContext, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		a.scheduler.Run(backgroundContext)
	}()
	if a.notifier != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			a.notifier.Run(backgroundContext)
		}()
	}
	return func() {
		cancel()
		workers.Wait()
	}
}

func (a *App) logStartup() {
	a.logger.Info("Starting Tuby",
		logger.String("addr", a.cfg.ListenAddr),
		logger.String("allowed_cidrs", strings.Join(a.cfg.AllowedCIDRS, ",")),
		logger.String("version", version.Version),
		logger.String("commit", version.Commit),
		logger.String("build_date", version.BuildDate),
		logger.String("go", runtime.Version()),
		logger.Bool("navire_enabled", a.cfg.Navire.Enabled),
	)
}
