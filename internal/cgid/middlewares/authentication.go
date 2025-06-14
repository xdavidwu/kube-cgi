package middlewares

import (
	"net/http"
	"strings"

	"github.com/xdavidwu/kube-cgi/internal/cgid"
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
			cgid.WriteError(w, http.StatusUnauthorized, "")
			return
		}
		if t != secret {
			cgid.WriteError(w, http.StatusForbidden, "")
			return
		}

		next.ServeHTTP(w, r)
	})
}
