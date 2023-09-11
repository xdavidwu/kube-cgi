package main

import (
	"k8s.io/client-go/kubernetes"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

const (
	ctxBody   = "body"
	ctxLogger = "logger"
	ctxId     = "id"
)

type handler struct {
	client    *kubernetes.Clientset
	namespace string
	spec      *fluorescencev1alpha1.API
}

type ErrorResponse struct {
	Message string `json:"error"`
}
