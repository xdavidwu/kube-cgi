package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal"
	kubedappy "git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/kubernetes"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/metrics"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/log"
)

func main() {
	opts := log.BuildZapOptions(flag.CommandLine)
	log := zap.New(zap.UseFlagOptions(&opts))
	flag.Parse()

	listen, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.DAPIPort))
	if err != nil {
		panic(err)
	}
	promlisten, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.DAPIMetricsPort))
	if err != nil {
		panic(err)
	}
	go http.Serve(promlisten, metrics.MetricHandler(log.WithName("metrics")))

	config, err := config.GetConfig()
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

	// XXX WithWatch cannot be mixed with NewNamespacedClient yet
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

	ref, err := kubedappy.OwnerReferenceOf(dynamicClient, &apiSet)
	if err != nil {
		panic(err)
	}

	mux := &http.ServeMux{}
	for i := range apiSet.Spec.APIs {
		mux.Handle(apiSet.Spec.APIs[i].Path, kubedappy.KubernetesHandler{
			Client:         dynamicClient,
			OldClient:      oldClient,
			ClientConfig:   config,
			Spec:           &apiSet.Spec.APIs[i],
			Namespace:      namespace,
			OwnerReference: ref,
			Generation:     apiSet.Generation,
		})
	}

	readinessHandler := http.StripPrefix(internal.DAPIReadinessEndpointPath, &healthz.Handler{
		Checks: map[string]healthz.Checker{
			"ping": healthz.Ping,
			"apiserver": func(r *http.Request) error {
				err = oldClient.CoreV1().RESTClient().Get().AbsPath("/readyz").Do(r.Context()).Error()
				if err != nil {
					log.WithName("healthcheck").Error(err, "cannot reach apiserver")
				}
				return err
			},
		},
	})

	mux.Handle(internal.DAPIReadinessEndpointPath, readinessHandler)
	mux.Handle(internal.DAPIReadinessEndpointPath+"/", readinessHandler)

	server := &http.Server{Handler: mux, BaseContext: func(net.Listener) context.Context {
		return logr.NewContext(context.Background(), log)
	}}
	server.Serve(listen)
}
