package metrics

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/middlewares"
)

type promhttpLogrAdaptor struct {
	logr.Logger
}

func (p promhttpLogrAdaptor) Println(v ...interface{}) {
	p.Info(fmt.Sprintln(v...))
}

func MetricHandler(l logr.Logger) http.Handler {
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
			ErrorLog: promhttpLogrAdaptor{l},
			Registry: prometheus,
		}),
	)
}
