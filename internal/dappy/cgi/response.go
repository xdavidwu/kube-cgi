package cgi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

func WriteResponse(w http.ResponseWriter, r io.Reader) (string, error) {
	lines := bufio.NewReader(r)
	tp := textproto.NewReader(lines)

	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return "", fmt.Errorf("cannot read headers: %w", err)
	}

	code := 0
	h := w.Header()
	for k, vs := range headers {
		switch k {
		case "Status":
			c, _, _ := strings.Cut(vs[0], " ")
			code, err = strconv.Atoi(c)
			if err != nil {
				return "", fmt.Errorf("cannot decode status: %w", err)
			}
		default:
			for _, v := range vs {
				h.Add(k, v)
			}
		}
	}

	location := h.Get("Location")

	if len(h) == 1 && strings.HasPrefix(location, "/") {
		// local redirects
		return location, nil
	}

	if code == 0 {
		if location != "" {
			code = http.StatusFound
		} else {
			code = http.StatusOK
		}
	}
	w.WriteHeader(code)
	io.Copy(w, lines) // TODO bubble up errors?
	return "", nil
}
