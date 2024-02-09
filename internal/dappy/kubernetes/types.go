package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

type KubernetesHandler struct {
	Client       client.WithWatch
	OldClient    *kubernetes.Clientset
	ClientConfig *rest.Config
	Namespace    string
	Spec         *fluorescencev1alpha1.API
	APISet       *fluorescencev1alpha1.APISet
}
