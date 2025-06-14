package middlewares

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/xdavidwu/kube-cgi/internal/cgid"
)

func DrainBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logr.FromContextOrDiscard(r.Context())

		var bytes []byte
		var err error
		if r.ContentLength == -1 {
			log.Info("missing content-length in request, not draining")
		} else if r.ContentLength > int64(cgid.BodyEnvMaxSize) {
			log.Info("request body too large for env, not draining")
		} else {
			bytes, err = io.ReadAll(http.MaxBytesReader(w, r.Body, r.ContentLength))
			if err != nil {
				log.Error(err, "cannot drain body")
				panic(err)
			}
		}

		next.ServeHTTP(w, r.WithContext(cgid.ContextWithBody(
			r.Context(), bytes)))
	})
}

func ValidateJson(next http.Handler, jsonSchema *jsonschema.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes := cgid.BodyFromContext(r.Context())
		if bytes == nil {
			log := logr.FromContextOrDiscard(r.Context())
			log.Info("json not validated due to body not drained")
			next.ServeHTTP(w, r)
			return
		}

		var v any
		if json.Unmarshal(bytes, &v) != nil {
			cgid.WriteError(w, http.StatusUnprocessableEntity, "request body is not json")
			return
		}
		if err := jsonSchema.Validate(v); err != nil {
			cgid.WriteError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func LogWithIdentifier(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.String(5)
		ctx := cgid.ContextWithId(r.Context(), id)
		log := logr.FromContextOrDiscard(ctx).WithName(id)
		ctx = logr.NewContext(ctx, log)

		log.Info("requested", "method", r.Method, "uri", r.RequestURI, "referer", r.Referer(), "agent", r.UserAgent())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
