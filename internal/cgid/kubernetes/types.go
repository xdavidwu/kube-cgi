package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubecgiv1alpha1 "github.com/xdavidwu/kube-cgi/api/v1alpha1"
)

var (
	generationKey = kubecgiv1alpha1.GroupVersion.Group + "/generation"
	pathKey       = kubecgiv1alpha1.GroupVersion.Group + "/path"
	gcKey         = kubecgiv1alpha1.GroupVersion.Group + "/released"
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
