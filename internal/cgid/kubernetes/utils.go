package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func OwnerReferenceOf(c client.Client, obj client.Object) (
	metav1.OwnerReference, error) {
	gvk, err := c.GroupVersionKindFor(obj)
	if err != nil {
		return metav1.OwnerReference{}, err
	}
	ver, kind := gvk.ToAPIVersionAndKind()
	return metav1.OwnerReference{
		APIVersion: ver,
		Kind:       kind,
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
	}, nil
}
