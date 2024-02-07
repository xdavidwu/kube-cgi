package cgi_test

import (
	gocgi "net/http/cgi"
	"net/http/httptest"
	"strings"
	"testing"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/cgi"
)

func TestRequestGoCompatibility(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com/0xdead/beef", strings.NewReader("1337"))
	r.Header.Set("Host", r.Host)
	r.Header.Set("X-Clacks-Overhead", "GNU Terry Pratchett")
	m := cgi.VarsFromRequest(r)
	rCGI, err := gocgi.RequestFromMap(m)

	if err != nil {
		t.Fatalf("cannot parse CGI env vars with Go stdlib: %v", err)
	}

	for _, i := range []struct {
		value any
		truth any
		name  string
	}{
		{rCGI.Method, r.Method, "method"},
		{rCGI.URL.String(), r.URL.String(), "url"},
		{rCGI.Proto, r.Proto, "protocol"},
		{rCGI.ContentLength, r.ContentLength, "length"},
		{rCGI.Host, r.Host, "host"},
		{rCGI.RemoteAddr, r.RemoteAddr, "remote addr"},
		{rCGI.Header.Get("X-Clacks-Overhead"), r.Header.Get("X-Clacks-Overhead"), "header"},
	} {
		if i.value != i.truth {
			t.Fatalf("%v does not match after CGI, expected %v, got %v", i.name, i.truth, i.value)
		}
	}
}
