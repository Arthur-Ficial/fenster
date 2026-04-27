package server

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func newReq(method, rawurl string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(method, u.String(), body)
}

func readBody(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}
