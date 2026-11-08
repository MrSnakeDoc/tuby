package mw

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal/logger"
)

func LogRequests(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			response := middleware.NewWrapResponseWriter(writer, request.ProtoMajor)
			next.ServeHTTP(response, request)
			logRequest(log, request, response.Status(), time.Since(started))
		})
	}
}

func logRequest(log logger.Logger, request *http.Request, status int, duration time.Duration) {
	fields := []logger.Field{
		logger.String("request_id", middleware.GetReqID(request.Context())),
		logger.String("method", request.Method),
		logger.String("path", request.URL.Path),
		logger.Int("status", status),
		logger.Duration("duration", duration),
	}
	if status >= http.StatusInternalServerError {
		log.Error("HTTP request completed", fields...)
		return
	}
	if status >= http.StatusBadRequest {
		log.Warn("HTTP request completed", fields...)
		return
	}
	log.Info("HTTP request completed", fields...)
}
