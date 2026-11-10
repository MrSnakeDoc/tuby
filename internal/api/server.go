package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/deps"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/mw"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/api/router"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/config"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
)

type Server struct {
	http      *http.Server
	logger    logger.Logger
	startTime time.Time
	updates   *events.Hub
}

func New(cfg *config.Config, log logger.Logger, dependencies deps.Deps) *Server {
	root := chi.NewRouter()
	root.Use(middleware.GetHead)
	root.Use(middleware.RequestID)
	root.Use(mw.LogRequests(log))
	root.Use(mw.AllowOnlyCIDRS(cfg.AllowedCIDRS, false))
	root.Use(mw.RequireCurrentBuild(dependencies.BuildID))
	root.Use(middleware.Recoverer)
	root.Handle("/static/*", http.StripPrefix("/static/", dependencies.Templates.Static()))
	router.RegisterRoutes(root, dependencies)
	server := &Server{logger: log, startTime: dependencies.StartTime, updates: dependencies.Updates}
	server.http = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	server.http.RegisterOnShutdown(func() {
		server.logger.Info("HTTP server shutdown complete", logger.Duration("uptime", time.Since(server.startTime).Round(time.Millisecond)))
	})
	return server
}

func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

func (s *Server) Start() error {
	s.logger.Info("HTTP server listening", logger.String("addr", s.http.Addr))
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("HTTP server shutting down")
	s.http.SetKeepAlivesEnabled(false)
	if s.updates != nil {
		s.updates.Close()
	}
	return s.http.Shutdown(ctx)
}
