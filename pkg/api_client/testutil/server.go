package testutil

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewTestServer starts an httptest.Server, or skips the test if binding a port is not permitted.
func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip: cannot listen in sandbox: %v", err)
	}

	srv := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}
