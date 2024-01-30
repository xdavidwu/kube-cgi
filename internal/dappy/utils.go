package dappy

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

func ExitCodeToHttpStatus(c int) int {
	switch {
	case c == 0:
		return 200
	case c > 128: // killed by signal
		return 500
	default:
		return c + 399
	}
}

func cgiLikeEnvVars(r *http.Request) map[string]string {
	// mimics CGI/1.1 https://datatracker.ietf.org/doc/html/draft-robinson-www-interface-00
	res := map[string]string{}
	if r.ContentLength != -1 {
		res["CONTENT_LENGTH"] = fmt.Sprint(r.ContentLength)
	}
	res["CONTENT_TYPE"] = r.Header.Get("Content-Type")
	res["SCRIPT_NAME"] = "/"

	if strings.HasPrefix(r.URL.Path, "/") {
		res["PATH_INFO"] = r.URL.Path[1:]
	} else {
		res["PATH_INFO"] = r.URL.Path
	}

	res["QUERY_STRING"] = r.URL.RawQuery

	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		panic(err)
	}

	res["REMOTE_ADDR"] = addr
	res["REQUEST_METHOD"] = r.Method
	res["SERVER_NAME"] = r.Host
	res["SERVER_PROTOCOL"] = r.Proto
	res["SERVER_SOFTWARE"] = "dappy"

	for k, vs := range r.Header {
		res["HTTP_"+strings.ReplaceAll(strings.ToUpper(k), "-", "_")] = strings.Join(vs, ", ")
	}

	// net/http/cgi has this
	res["REQUEST_URI"] = r.URL.RequestURI()

	return res
}
