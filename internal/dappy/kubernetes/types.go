package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubecgiv1alpha1 "git.cs.nctu.edu.tw/aic/infra/kube-cgi/api/v1alpha1"
)

var (
	generationKey = kubecgiv1alpha1.GroupVersion.Group + "/generation"
	pathKey       = kubecgiv1alpha1.GroupVersion.Group + "/path"
)

type KubernetesHandler struct {
	Client         client.WithWatch
	OldClient      *kubernetes.Clientset
	ClientConfig   *rest.Config
	Namespace      string
	Spec           *kubecgiv1alpha1.API
	OwnerReference metav1.OwnerReference
	Generation     int64
}
