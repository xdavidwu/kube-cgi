package middlewares

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/apimachinery/pkg/util/rand"

	"git.cs.nctu.edu.tw/aic/infra/kube-cgi/internal/cgid"
)

var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of the http requests received since the server started",
		},
		[]string{"handler", "code"},
	)
	httpRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_requests_duration_seconds",
			Help: "Duration in seconds to serve http requests",
			// TODO consider buckets
		},
		[]string{"handler", "code"},
	)
	httpInflightRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "Number of the inflight http requests",
		},
		[]string{"handler"},
	)
)

func MustRegisterCollectors(r *prometheus.Registry) {
	r.MustRegister(httpRequests, httpRequestsDuration, httpInflightRequests)
}

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

		var v interface{}
		if json.Unmarshal(bytes, &v) != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := cgid.ErrorResponse{Message: "request body is not json"}
			body, _ := json.Marshal(msg)
			w.Write(body)
			return
		}
		if err := jsonSchema.Validate(v); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := cgid.ErrorResponse{Message: err.Error()}
			body, _ := json.Marshal(msg)
			w.Write(body)
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

func Intrument(next http.Handler, name string) http.Handler {
	prepopulateLabels := prometheus.Labels{"handler": name, "code": "200"}
	httpRequests.With(prepopulateLabels)
	httpRequestsDuration.With(prepopulateLabels)

	labels := prometheus.Labels{"handler": name}
	return promhttp.InstrumentHandlerCounter(
		httpRequests.MustCurryWith(labels),
		promhttp.InstrumentHandlerDuration(
			httpRequestsDuration.MustCurryWith(labels),
			promhttp.InstrumentHandlerInFlight(
				httpInflightRequests.With(labels),
				next,
			),
		),
	)
}
