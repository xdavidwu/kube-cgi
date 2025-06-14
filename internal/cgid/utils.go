package cgid

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
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

func WriteError(w http.ResponseWriter, statusCode int, msg string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if msg == "" {
		msg = strings.ToLower(http.StatusText(statusCode))
	}

	m := ErrorResponse{Message: msg}
	body, _ := json.Marshal(m)
	_, err := w.Write(body)
	return err
}
