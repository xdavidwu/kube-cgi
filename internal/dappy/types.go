package dappy

import (
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

const (
	ctxBody   = "body"
	ctxLogger = "logger"
	ctxId     = "id"
)

type Handler struct {
	Client    client.WithWatch
	OldClient *kubernetes.Clientset
	Namespace string
	Spec      *fluorescencev1alpha1.API
	APISet    *fluorescencev1alpha1.APISet
}

type ErrorResponse struct {
	Message string `json:"error"`
}
