package cgid

import (
	"context"
	"os"
)

const (
	BodyEnvKey = "REQUEST_BODY"
)

var (
	// include/uapi/linux/binfmts.h:#define MAX_ARG_STRLEN (PAGE_SIZE * 32)
	MaxArgStrlen = os.Getpagesize() * 32

	BodyEnvMaxSize = MaxArgStrlen - 2 - len(BodyEnvKey)
)

func EnvTooLarge(k, v string) bool {
	// including = and NULL
	return len(k)+len(v)+2 > MaxArgStrlen
}

type ctxKey string

var (
	ctxBody = ctxKey("body")
	ctxId   = ctxKey("id")
)

func ContextWithId(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxId, id)
}

func ContextWithBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, ctxBody, body)
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
