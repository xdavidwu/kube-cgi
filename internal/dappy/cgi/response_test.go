package cgi_test

import (
	"net/http"
	gocgi "net/http/cgi"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/cgi"
)

func TestResponseGoCompatibility(t *testing.T) {
	// TODO should probably hardcode envs instead to get rid of dependency
	req := httptest.NewRequest("GET", "http://example.com/0xdead/beef", strings.NewReader("1337"))
	vars := cgi.VarsFromRequest(req)
	for k, v := range vars {
		t.Setenv(k, v)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Clacks-Overhead", "GNU Terry Pratchett")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"error\":\"test\"}"))
	})

	resr, resw, _ := os.Pipe()
	reqr, reqw, _ := os.Pipe()
	in, out := os.Stdin, os.Stdout
	os.Stdout = resw
	os.Stdin = reqr
	reqw.WriteString("1337")
	gocgi.Serve(h)
	resw.Close()
	os.Stdin, os.Stdout = in, out

	response := httptest.NewRecorder()
	cgi.WriteResponse(response, resr)

	for _, i := range []struct {
		value any
		truth any
		name  string
	}{
		{response.Code, http.StatusBadRequest, "method"},
		{response.Header().Get("Content-Type"), "application/json", "header"},
		{response.Header().Get("X-Clacks-Overhead"), "GNU Terry Pratchett", "header"},
		{response.Body.String(), "{\"error\":\"test\"}", "body"},
	} {
		if i.value != i.truth {
			t.Fatalf("%v does not match after CGI, expected %v, got %v", i.name, i.truth, i.value)
		}
	}
}
