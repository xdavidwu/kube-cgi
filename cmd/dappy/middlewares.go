package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func validatesJson(next http.Handler, jsonSchema *jsonschema.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := ErrorResponse{Message: "request body is absent"}
			body, _ := json.Marshal(msg)
			w.Write(body)
			return
		}
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			log := r.Context().Value(ctxLogger).(*log.Logger)
			log.Panic(err)
		}
		var v interface{}
		if json.Unmarshal(bytes, &v) != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			msg := ErrorResponse{Message: "request body is not json"}
			body, _ := json.Marshal(msg)
			w.Write(body)
			return
		}
		if jsonSchema != nil {
			if err = jsonSchema.Validate(v); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				msg := ErrorResponse{Message: err.Error()}
				body, _ := json.Marshal(msg)
				w.Write(body)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(
			r.Context(), ctxBody, bytes)))
	})
}

func setContentType(next http.Handler, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", contentType)

		next.ServeHTTP(w, r)
	})
}

func logsWithIdentifier(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.String(5)
		ctx := context.WithValue(r.Context(), ctxId, id)
		parent := log.Default()
		logger := log.New(
			parent.Writer(),
			id+" ",
			parent.Flags()|log.Lmsgprefix,
		)
		ctx = context.WithValue(ctx, ctxLogger, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// XXX should not really inspect handler config
func withMiddlewares(handler *handler) http.Handler {
	var stack http.Handler = handler
	if handler.spec.Request != nil && handler.spec.Request.Schema != "" {
		json, err := yaml.ToJSON([]byte(handler.spec.Request.Schema))
		if err != nil {
			log.Panicf("cannot convert schema to json: %v", err)
		}
		// XXX better name
		schema := jsonschema.MustCompileString("api.schema.json", string(json))
		stack = validatesJson(stack, schema)
	} else {
		// XXX seperate body draining into another middleware?
		stack = validatesJson(stack, nil)
	}

	contentType := "application/json"
	if handler.spec.Response != nil && handler.spec.Response.ContentType != "" {
		contentType = handler.spec.Response.ContentType
	}

	return logsWithIdentifier(setContentType(stack, contentType))
}
