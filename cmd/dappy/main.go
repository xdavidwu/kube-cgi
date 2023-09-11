package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	listen, err := net.Listen("tcp", "0.0.0.0:1234")
	if err != nil {
		panic(err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	namespace := os.Getenv("APISET_NAMESPACE")
	apiSetName := os.Getenv("APISET_NAME")
	apiSetVersion := os.Getenv("APISET_RESOURCE_VERSION")

	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	fluorescencev1alpha1.AddToScheme(scheme)

	dynamicClient, err := client.New(config, client.Options{Scheme: scheme})
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

	mux := &http.ServeMux{}
	for i := range apiSet.Spec.APIs {
		mux.Handle(apiSet.Spec.APIs[i].Path, withMiddlewares(&handler{
			client:    clientset,
			spec:      &apiSet.Spec.APIs[i],
			namespace: namespace,
		}))
	}
	server := &http.Server{Handler: mux}
	server.Serve(listen)
}
