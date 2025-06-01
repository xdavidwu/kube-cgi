package middlewares

import (
	"net/http"
	"strings"
)

const (
	bearerType = "Bearer "
)

func bearerTokenFromRequest(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if len(v) <= len(bearerType) || !strings.EqualFold(bearerType, v[:len(bearerType)]) {
		return ""
	}
	return v[len(bearerType):]
}

func AuthnWithPreShared(next http.Handler, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := bearerTokenFromRequest(r)

		if t == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if t != secret {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}
