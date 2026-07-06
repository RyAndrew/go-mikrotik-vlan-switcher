package api

import (
	"log/slog"
	"net/http"

	jwtverifier "github.com/okta/okta-jwt-verifier-golang"

	"go-mikrotik-vlan-switcher/ent"
	"go-mikrotik-vlan-switcher/internal/auth"
	"go-mikrotik-vlan-switcher/internal/mikrotik"
)

// NewRouter builds the HTTP handler for the service: the two /vlan routes,
// each requiring requiredScope, wrapped in a request-logging middleware.
// If enableAuth is false, the OAuth token check is skipped entirely.
func NewRouter(log *slog.Logger, entClient *ent.Client, mikClient *mikrotik.Client, verifier *jwtverifier.JwtVerifier, requiredScope string, enableAuth bool) http.Handler {
	deps := &Deps{Ent: entClient, Mikrotik: mikClient}

	requireScope := func(next http.Handler) http.Handler { return next }
	if enableAuth {
		requireScope = auth.RequireScope(verifier, requiredScope)
	} else {
		log.Warn("authentication is disabled (enable_authentication=false); /vlan routes are unprotected")
	}

	mux := http.NewServeMux()
	mux.Handle("POST /vlan", requireScope(http.HandlerFunc(deps.HandleSwitchVlan)))
	mux.Handle("GET /vlan/{interface}", requireScope(http.HandlerFunc(deps.HandleGetVlan)))
	mux.Handle("POST /vlan/{interface}/sync", requireScope(http.HandlerFunc(deps.HandleSyncVlan)))
	mux.Handle("POST /vlan/sync", requireScope(http.HandlerFunc(deps.HandleSyncAllVlans)))

	return Logging(log, entClient)(mux)
}
