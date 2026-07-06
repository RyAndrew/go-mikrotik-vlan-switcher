// Package auth validates Okta-issued OAuth2 access tokens and enforces
// that a required scope is present before letting a request through.
package auth

import (
	"net/http"
	"strings"

	jwtverifier "github.com/okta/okta-jwt-verifier-golang"

	"go-mikrotik-vlan-switcher/internal/reqctx"
)

// NewVerifier builds an Okta JWT verifier for the given issuer/audience.
func NewVerifier(issuer, audience string) *jwtverifier.JwtVerifier {
	jv := &jwtverifier.JwtVerifier{
		Issuer:           issuer,
		ClaimsToValidate: map[string]string{"aud": audience},
	}
	return jv.New()
}

// RequireScope returns middleware that validates the request's bearer
// access token against verifier and requires requiredScope to be present
// in the token's "scp" claim.
func RequireScope(verifier *jwtverifier.JwtVerifier, requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			jwt, err := verifier.VerifyAccessToken(token)
			if err != nil {
				http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if !hasScope(jwt.Claims, requiredScope) {
				http.Error(w, "token missing required scope", http.StatusForbidden)
				return
			}

			if subject, ok := jwt.Claims["sub"].(string); ok {
				reqctx.From(r.Context()).Subject = subject
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

func hasScope(claims map[string]interface{}, required string) bool {
	switch v := claims["scp"].(type) {
	case []interface{}:
		for _, s := range v {
			if str, ok := s.(string); ok && str == required {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == required {
				return true
			}
		}
	case string:
		for _, s := range strings.Fields(v) {
			if s == required {
				return true
			}
		}
	}
	return false
}
