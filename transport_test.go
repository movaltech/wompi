package wompi

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSendsHeadersAndDecodes(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/anything", testPublicKey)
		if got := r.Header.Get("User-Agent"); got != "wompi-go/"+Version {
			t.Errorf("User-Agent = %q", got)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("Content-Type must not be set without a body")
		}
		writeJSON(t, w, 200, map[string]any{"data": map[string]string{"value": "hello"}})
	})

	var out envelope[struct {
		Value string `json:"value"`
	}]
	if err := c.do(context.Background(), request{method: http.MethodGet, path: "/anything", auth: authPublic, out: &out}); err != nil {
		t.Fatalf("do: %v", err)
	}
	if out.Data.Value != "hello" {
		t.Errorf("Value = %q", out.Data.Value)
	}
}

func TestDoMissingPrivateKey(t *testing.T) {
	c, err := New(testPublicKey, "")
	if err != nil {
		t.Fatal(err)
	}
	err = c.do(context.Background(), request{method: http.MethodGet, path: "/x", auth: authPrivate})
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("err = %v, want ErrMissingCredential", err)
	}
	if key, _ := c.keyFor(authPrivateOrPublic); key != testPublicKey {
		t.Errorf("authPrivateOrPublic should fall back to the public key, got %q", key)
	}
}

func TestDoDecodeError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	})
	var out map[string]any
	err := c.do(context.Background(), request{method: http.MethodGet, path: "/x", out: &out})
	var wErr *Error
	if !errors.As(err, &wErr) || wErr.Type != ErrTypeDecode || wErr.HTTPStatus != 200 {
		t.Fatalf("err = %v", err)
	}
}

func TestDoCommunicationError(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey, WithBaseURL("http://127.0.0.1:1"))
	err := c.do(context.Background(), request{method: http.MethodGet, path: "/x"})
	if !errors.Is(err, ErrCommunication) {
		t.Fatalf("err = %v, want ErrCommunication", err)
	}
}

func TestRetryPolicy(t *testing.T) {
	cases := []struct {
		name      string
		method    string
		status    int
		wantCalls int32
	}{
		{"GET 500 retried", http.MethodGet, 500, 3},
		{"POST 500 not retried", http.MethodPost, 500, 1},
		{"POST 503 retried", http.MethodPost, 503, 3},
		{"POST 429 retried", http.MethodPost, 429, 3},
		{"GET 422 not retried", http.MethodGet, 422, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				writeJSON(t, w, tc.status, map[string]any{"error": map[string]string{"type": "X", "reason": "y"}})
			}, WithRetries(2))

			err := c.do(context.Background(), request{method: tc.method, path: "/x"})
			if err == nil {
				t.Fatal("expected error")
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Errorf("calls = %d, want %d", got, tc.wantCalls)
			}
		})
	}
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			writeJSON(t, w, 503, map[string]any{})
			return
		}
		writeJSON(t, w, 200, map[string]any{"data": map[string]any{}})
	}, WithRetries(1))

	if err := c.do(context.Background(), request{method: http.MethodPost, path: "/x"}); err != nil {
		t.Fatalf("do: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestRetryStopsOnContextCancel(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "5")
		writeJSON(t, w, 503, map[string]any{})
	}, WithRetries(5))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.do(ctx, request{method: http.MethodGet, path: "/x"}); err == nil {
		t.Fatal("expected error")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("retry loop ignored context cancellation")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

func TestRetryDelayBounds(t *testing.T) {
	for attempt := range 10 {
		d := retryDelay(attempt, errors.New("x"))
		if d <= 0 || d > 5*time.Second {
			t.Errorf("attempt %d: delay %v out of bounds", attempt, d)
		}
	}
	if d := retryDelay(0, &Error{retryAfter: time.Minute}); d != 10*time.Second {
		t.Errorf("Retry-After should be capped at 10s, got %v", d)
	}
}
