package wompi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testPublicKey       = "pub_test_abc"
	testPrivateKey      = "prv_test_xyz"
	testIntegritySecret = "test_integrity_abc"
	testEventsSecret    = "test_events_secret"
)

// newTestClient starts an httptest server with handler and returns a sandbox
// client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	opts = append([]Option{
		WithBaseURL(srv.URL),
		WithIntegritySecret(testIntegritySecret),
		WithEventsSecret(testEventsSecret),
	}, opts...)
	c, err := New(testPublicKey, testPrivateKey, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

// readBody decodes the JSON request body into a generic map.
func readBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode body %q: %v", raw, err)
	}
	return m
}

func assertRequest(t *testing.T, r *http.Request, method, path, bearer string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("method = %s, want %s", r.Method, method)
	}
	if r.URL.Path != path {
		t.Errorf("path = %s, want %s", r.URL.Path, path)
	}
	want := ""
	if bearer != "" {
		want = "Bearer " + bearer
	}
	if got := r.Header.Get("Authorization"); got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}
