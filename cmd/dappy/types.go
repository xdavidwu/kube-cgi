package main

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

type handler struct {
	client    client.WithWatch
	oldClient *kubernetes.Clientset
	namespace string
	spec      *fluorescencev1alpha1.API
	apiSet    *fluorescencev1alpha1.APISet
}

type ErrorResponse struct {
	Message string `json:"error"`
}
