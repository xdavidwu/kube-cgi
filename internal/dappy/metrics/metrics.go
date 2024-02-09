package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/middlewares"
)

func MetricHandler(l *log.Logger) http.Handler {
	prometheus := prometheus.NewRegistry()
	prometheus.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	middlewares.MustRegisterCollectors(prometheus)

	return promhttp.InstrumentMetricHandler(
		prometheus,
		promhttp.HandlerFor(prometheus, promhttp.HandlerOpts{
			ErrorLog: l,
			Registry: prometheus,
		}),
	)
}
