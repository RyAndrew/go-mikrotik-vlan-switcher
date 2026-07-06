package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"go-mikrotik-vlan-switcher/ent"
	"go-mikrotik-vlan-switcher/internal/reqctx"
	"go-mikrotik-vlan-switcher/internal/store"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logging wraps a handler, writing one RequestLog row per request once it
// completes. It must be the outermost middleware so that it observes the
// final status code and any subject/interface/error info recorded deeper
// in the chain via reqctx.
func Logging(log *slog.Logger, client *ent.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, fields := reqctx.WithFields(r.Context())
			r = r.WithContext(ctx)

			sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(sw, r)

			entry := store.RequestLogEntry{
				Method:     r.Method,
				Path:       r.URL.Path,
				RemoteAddr: r.RemoteAddr,
				Subject:    fields.Subject,
				Interface:  fields.Interface,
				VlanID:     fields.VlanID,
				StatusCode: sw.status,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fields.Error,
			}
			if err := store.WriteRequestLog(context.Background(), client, entry); err != nil {
				log.Error("write request log", slog.Any("error", err))
			}
		})
	}
}
