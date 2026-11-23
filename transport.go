package wompi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"
)

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 10 << 20

type authMode int

const (
	authNone authMode = iota
	authPublic
	authPrivate
	// authPrivateOrPublic uses the private key when configured and falls
	// back to the public key otherwise.
	authPrivateOrPublic
)

// request describes a single call to the Wompi API.
type request struct {
	method string
	path   string
	auth   authMode
	query  url.Values
	header http.Header
	body   any // JSON-encoded when non-nil
	out    any // JSON-decoded from a 2xx response when non-nil
}

// envelope is Wompi's standard success response: {"data": ..., "meta": ...}.
type envelope[T any] struct {
	Data T              `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

// do executes r, retrying transient failures according to the client's
// retry budget (see WithRetries).
func (c *Client) do(ctx context.Context, r request) error {
	if ctx == nil {
		ctx = context.Background()
	}
	key, err := c.keyFor(r.auth)
	if err != nil {
		return err
	}

	var payload []byte
	if r.body != nil {
		payload, err = json.Marshal(r.body)
		if err != nil {
			return &Error{Type: ErrTypeInvalidParams, Reason: "failed to encode request body", Err: err}
		}
	}

	for attempt := 0; ; attempt++ {
		err = c.send(ctx, r, key, payload)
		if err == nil || attempt >= c.retries || ctx.Err() != nil || !shouldRetry(r.method, err) {
			return err
		}
		timer := time.NewTimer(retryDelay(attempt, err))
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

func (c *Client) send(ctx context.Context, r request, key string, payload []byte) error {
	u := c.baseURL + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, u, body)
	if err != nil {
		return &Error{Type: ErrTypeInvalidConfig, Reason: "failed to build request", Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, vs := range r.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &Error{Type: ErrTypeCommunication, Reason: "failed to reach Wompi", Err: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return &Error{Type: ErrTypeCommunication, Reason: "failed to read response body", HTTPStatus: resp.StatusCode, Err: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp.StatusCode, resp.Header, raw)
	}

	if r.out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, r.out); err != nil {
			return &Error{Type: ErrTypeDecode, Reason: "failed to decode response body", HTTPStatus: resp.StatusCode, Err: err}
		}
	}
	return nil
}

func (c *Client) keyFor(mode authMode) (string, error) {
	switch mode {
	case authPublic:
		return c.publicKey, nil
	case authPrivate:
		if c.privateKey == "" {
			return "", newError(ErrTypeMissingCredential, "this operation requires the private key (prv_...)")
		}
		return c.privateKey, nil
	case authPrivateOrPublic:
		if c.privateKey != "" {
			return c.privateKey, nil
		}
		return c.publicKey, nil
	default:
		return "", nil
	}
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

// shouldRetry reports whether a failed request is worth retrying. 429/503
// mean the request was not processed, so they are safe for any method;
// network errors and other 5xx are only retried for idempotent methods.
func shouldRetry(method string, err error) bool {
	var wErr *Error
	if !errors.As(err, &wErr) {
		return false
	}
	switch {
	case wErr.HTTPStatus == http.StatusTooManyRequests, wErr.HTTPStatus == http.StatusServiceUnavailable:
		return true
	case wErr.Type == ErrTypeCommunication && wErr.HTTPStatus == 0:
		return isIdempotent(method)
	case wErr.HTTPStatus >= 500:
		return isIdempotent(method)
	}
	return false
}

// retryDelay returns the wait before the next attempt: the server's
// Retry-After when present, otherwise exponential backoff with jitter.
func retryDelay(attempt int, err error) time.Duration {
	const maxDelay = 10 * time.Second
	var wErr *Error
	if errors.As(err, &wErr) && wErr.retryAfter > 0 {
		return min(wErr.retryAfter, maxDelay)
	}
	base := min(250*time.Millisecond<<min(attempt, 5), 5*time.Second)
	return base/2 + rand.N(base/2+1)
}

func pathEscape(s string) string { return url.PathEscape(s) }
