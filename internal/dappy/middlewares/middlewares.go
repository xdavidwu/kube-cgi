package middlewares

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/apimachinery/pkg/util/rand"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy"
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
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			dappy.LoggerFromContext(r.Context()).Panic(err)
		}

		next.ServeHTTP(w, r.WithContext(dappy.ContextWithBody(
			r.Context(), bytes)))
	})
}

func ValidateJson(next http.Handler, jsonSchema *jsonschema.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes := dappy.BodyFromContext(r.Context())
		var v interface{}
		if json.Unmarshal(bytes, &v) != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := dappy.ErrorResponse{Message: "request body is not json"}
			body, _ := json.Marshal(msg)
			w.Write(body)
			return
		}
		if err := jsonSchema.Validate(v); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := dappy.ErrorResponse{Message: err.Error()}
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
		ctx := dappy.ContextWithId(r.Context(), id)
		parent := log.Default()
		logger := log.New(
			parent.Writer(),
			id+" ",
			parent.Flags()|log.Lmsgprefix,
		)
		ctx = dappy.ContextWithLogger(ctx, logger)

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
