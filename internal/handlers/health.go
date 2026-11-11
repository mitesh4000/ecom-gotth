package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

// Health reports process liveness. It does not touch the database so it can be
// used as a container liveness probe.
func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{
		"success": true,
		"status":  "ok",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready reports whether the database is reachable. Container orchestration
// should use this for readiness so traffic is not sent before migrations and
// storage are available.
func (a *API) Ready(w http.ResponseWriter, r *http.Request) {
	if a.DB == nil {
		response.FailStatus(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.DB.PingContext(ctx); err != nil {
		response.FailStatus(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	response.OK(w, map[string]any{"success": true, "status": "ready"})
}
