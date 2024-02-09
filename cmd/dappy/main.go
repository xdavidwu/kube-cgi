package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal"
	kubedappy "git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/kubernetes"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/middlewares"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	prometheus := prometheus.NewRegistry()
	prometheus.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	middlewares.MustRegisterCollectors(prometheus)

	listen, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.DAPIPort))
	if err != nil {
		panic(err)
	}
	promlisten, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.DAPIMetricsPort))
	if err != nil {
		panic(err)
	}
	go http.Serve(promlisten, promhttp.InstrumentMetricHandler(
		prometheus,
		promhttp.HandlerFor(prometheus, promhttp.HandlerOpts{
			ErrorLog: log.Default(),
			Registry: prometheus,
		}),
	))

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	oldClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	namespace := os.Getenv(internal.DAPIEnvAPISetNamespace)
	apiSetName := os.Getenv(internal.DAPIEnvAPISetName)
	apiSetVersion := os.Getenv(internal.DAPIEnvAPISetResourceVersion)

	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	fluorescencev1alpha1.AddToScheme(scheme)

	dynamicClient, err := client.NewWithWatch(config, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}

	var apiSet fluorescencev1alpha1.APISet
	dynamicClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: namespace, Name: apiSetName},
		&apiSet,
		&client.GetOptions{Raw: &metav1.GetOptions{ResourceVersion: apiSetVersion}},
	)

	// XXX WithWatch cannot be mixed with NewNamespacedClient yet

	mux := &http.ServeMux{}
	for i := range apiSet.Spec.APIs {
		mux.Handle(apiSet.Spec.APIs[i].Path, kubedappy.KubernetesHandler{
			Client:       dynamicClient,
			OldClient:    oldClient,
			ClientConfig: config,
			Spec:         &apiSet.Spec.APIs[i],
			Namespace:    namespace,
			APISet:       &apiSet,
		})
	}

	healthz.InstallReadyzHandler(mux, healthz.PingHealthz, healthz.NamedCheck(
		"apiserver",
		func(r *http.Request) error {
			err = oldClient.CoreV1().RESTClient().Get().AbsPath("/readyz").Do(r.Context()).Error()
			if err != nil {
				log.Printf("cannot reach apiserver: %v", err)
			}
			return err
		},
	))

	server := &http.Server{Handler: mux}
	server.Serve(listen)
}
