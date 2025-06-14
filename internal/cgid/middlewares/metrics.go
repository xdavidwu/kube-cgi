package middlewares

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func Instrument(next http.Handler, name string) http.Handler {
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
