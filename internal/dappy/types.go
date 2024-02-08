package dappy

import (
	"context"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

type ctxKey string

var (
	ctxBody   = ctxKey("body")
	ctxLogger = ctxKey("logger")
	ctxId     = ctxKey("id")
)

func loggerFromContext(ctx context.Context) *log.Logger {
	return ctx.Value(ctxLogger).(*log.Logger)
}

func idFromContext(ctx context.Context) string {
	return ctx.Value(ctxId).(string)
}

func bodyFromContext(ctx context.Context) []byte {
	return ctx.Value(ctxBody).([]byte)
}

type Handler struct {
	Client       client.WithWatch
	OldClient    *kubernetes.Clientset
	ClientConfig *rest.Config
	Namespace    string
	Spec         *fluorescencev1alpha1.API
	APISet       *fluorescencev1alpha1.APISet
}

type ErrorResponse struct {
	Message string `json:"error"`
}
