package dappy

import (
	"context"
	"log"
)

const (
	BodyEnvKey = "REQUEST_BODY"
)

type ctxKey string

var (
	ctxBody   = ctxKey("body")
	ctxLogger = ctxKey("logger")
	ctxId     = ctxKey("id")
)

func ContextWithLogger(ctx context.Context, l *log.Logger) context.Context {
	return context.WithValue(ctx, ctxLogger, l)
}

func ContextWithId(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxId, id)
}

func ContextWithBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, ctxBody, body)
}

func LoggerFromContext(ctx context.Context) *log.Logger {
	return ctx.Value(ctxLogger).(*log.Logger)
}

func IdFromContext(ctx context.Context) string {
	return ctx.Value(ctxId).(string)
}

func BodyFromContext(ctx context.Context) []byte {
	return ctx.Value(ctxBody).([]byte)
}

type ErrorResponse struct {
	Message string `json:"error"`
}
