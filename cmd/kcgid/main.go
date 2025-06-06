package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubecgiv1alpha1 "github.com/xdavidwu/kube-cgi/api/v1alpha1"
	"github.com/xdavidwu/kube-cgi/internal"
	kcgid "github.com/xdavidwu/kube-cgi/internal/cgid/kubernetes"
	"github.com/xdavidwu/kube-cgi/internal/cgid/metrics"
	"github.com/xdavidwu/kube-cgi/internal/log"
)

func main() {
	opts := log.BuildZapOptions(flag.CommandLine)
	log := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(log)
	ctrl.SetLogger(log)
	flag.Parse()

	must := func(err error, op string) {
		if err != nil {
			log.Error(err, "cannot "+op)
			panic(err)
		}
	}

	listen, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.KcgidPort))
	must(err, "listen for http")
	promlisten, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", internal.KcgidMetricsPort))
	must(err, "listen for metrics")
	go http.Serve(promlisten, metrics.MetricHandler(log.WithName("metrics")))

	config, err := config.GetConfig()
	must(err, "get kubeconfig")
	oldClient, err := kubernetes.NewForConfig(config)
	must(err, "create client-go client")

	namespace := os.Getenv(internal.KcgidEnvAPISetNamespace)
	apiSetName := os.Getenv(internal.KcgidEnvAPISetName)
	apiSetGeneration, _ := strconv.ParseInt(os.Getenv(internal.KcgidEnvAPISetGeneration), 10, 0)

	scheme := runtime.NewScheme()
	must(clientgoscheme.AddToScheme(scheme), "register client-go scheme")
	must(kubecgiv1alpha1.AddToScheme(scheme), "register our scheme")

	// XXX WithWatch cannot be mixed with NewNamespacedClient yet
	dynamicClient, err := client.NewWithWatch(config, client.Options{Scheme: scheme})
	must(err, "create controller-runtime client")

	var apiSet kubecgiv1alpha1.APISet
	err = dynamicClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: namespace, Name: apiSetName},
		&apiSet,
	)
	must(err, "get apiset")

	if apiSet.Generation < apiSetGeneration {
		log.Error(nil, "stale APISet obtained", "generation", apiSet.Generation, "expected", apiSetGeneration)
		panic("stale APISet obtained")
	}

	ref, err := kcgid.OwnerReferenceOf(dynamicClient, &apiSet)
	must(err, "set up ownerreference")

	go kcgid.CollectGarbage(log.WithName("gc"), dynamicClient, &apiSet)

	mux := &http.ServeMux{}
	for i := range apiSet.Spec.APIs {
		mux.Handle(apiSet.Spec.APIs[i].Path, kcgid.KubernetesHandler{
			Client:         dynamicClient,
			OldClient:      oldClient,
			ClientConfig:   config,
			Spec:           &apiSet.Spec.APIs[i],
			Namespace:      namespace,
			OwnerReference: ref,
			Generation:     apiSet.Generation,
		})
	}

	readinessHandler := http.StripPrefix(internal.KcgidReadinessEndpointPath, &healthz.Handler{
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

	mux.Handle(internal.KcgidReadinessEndpointPath, readinessHandler)
	mux.Handle(internal.KcgidReadinessEndpointPath+"/", readinessHandler)

	server := &http.Server{Handler: mux, BaseContext: func(net.Listener) context.Context {
		return logr.NewContext(context.Background(), log)
	}}
	must(server.Serve(listen), "serve http")
}
