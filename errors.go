package wompi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Error types returned by the Wompi API.
const (
	ErrTypeInputValidation    = "INPUT_VALIDATION_ERROR"
	ErrTypeNotFound           = "NOT_FOUND_ERROR"
	ErrTypeInvalidAccessToken = "INVALID_ACCESS_TOKEN"
	ErrTypeUnprocessable      = "UNPROCESSABLE"
)

// Error types produced by the SDK itself, before or after reaching the API.
const (
	// ErrTypeInvalidConfig reports an invalid client configuration.
	ErrTypeInvalidConfig = "SDK_INVALID_CONFIG"
	// ErrTypeInvalidParams reports parameters rejected before any request.
	ErrTypeInvalidParams = "SDK_INVALID_PARAMS"
	// ErrTypeMissingCredential reports an operation that needs a credential
	// (private key, integrity secret, events secret) the client lacks.
	ErrTypeMissingCredential = "SDK_MISSING_CREDENTIAL"
	// ErrTypeCommunication reports a network failure talking to Wompi.
	ErrTypeCommunication = "SDK_COMMUNICATION_ERROR"
	// ErrTypeDecode reports a response that could not be decoded.
	ErrTypeDecode = "SDK_DECODE_ERROR"
	// ErrTypeInvalidSignature reports an event whose checksum does not match.
	ErrTypeInvalidSignature = "SDK_INVALID_SIGNATURE"
	// ErrTypeAPI is used for API errors whose body carries no error type.
	ErrTypeAPI = "API_ERROR"
)

// Sentinel errors for use with errors.Is. Matching is done on Error.Type, so
// errors.Is(err, wompi.ErrNotFound) is true for any 404 NOT_FOUND_ERROR.
var (
	ErrInputValidation    = &Error{Type: ErrTypeInputValidation}
	ErrNotFound           = &Error{Type: ErrTypeNotFound}
	ErrInvalidAccessToken = &Error{Type: ErrTypeInvalidAccessToken}
	ErrUnprocessable      = &Error{Type: ErrTypeUnprocessable}
	ErrInvalidConfig      = &Error{Type: ErrTypeInvalidConfig}
	ErrInvalidParams      = &Error{Type: ErrTypeInvalidParams}
	ErrMissingCredential  = &Error{Type: ErrTypeMissingCredential}
	ErrCommunication      = &Error{Type: ErrTypeCommunication}
	ErrInvalidSignature   = &Error{Type: ErrTypeInvalidSignature}
)

// Error is the error returned by every operation of this SDK.
type Error struct {
	// Type is the Wompi error type (e.g. INPUT_VALIDATION_ERROR) or one of
	// the SDK_* types for client-side failures.
	Type string
	// Reason is a human-readable description of the failure.
	Reason string
	// Messages holds per-field validation messages, as returned by Wompi in
	// INPUT_VALIDATION_ERROR responses (e.g. "reference": ["..."]).
	Messages map[string][]string
	// HTTPStatus is the HTTP status returned by Wompi, or zero when the
	// request never produced a response.
	HTTPStatus int
	// Err is the underlying error, if any.
	Err error

	retryAfter time.Duration
}

func newError(typ, reason string) *Error { return &Error{Type: typ, Reason: reason} }

func invalidParams(reason string) *Error { return newError(ErrTypeInvalidParams, reason) }

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("wompi: ")
	b.WriteString(e.Type)
	if e.HTTPStatus != 0 {
		fmt.Fprintf(&b, " (http %d)", e.HTTPStatus)
	}
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	if len(e.Messages) > 0 {
		fields := make([]string, 0, len(e.Messages))
		for f := range e.Messages {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		parts := make([]string, 0, len(fields))
		for _, f := range fields {
			parts = append(parts, f+": "+strings.Join(e.Messages[f], ", "))
		}
		b.WriteString(" [")
		b.WriteString(strings.Join(parts, "; "))
		b.WriteString("]")
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error { return e.Err }

// Is reports whether target is an *Error with the same Type.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Type != "" && t.Type == e.Type
}

// parseAPIError maps a non-2xx response to an *Error. Wompi uses
// {"error": {"type", "reason" | "messages"}}; some products (e.g. Daviplata)
// use {"code", "message"} instead, which is handled as a fallback.
func parseAPIError(status int, header http.Header, raw []byte) *Error {
	e := &Error{HTTPStatus: status, retryAfter: parseRetryAfter(header.Get("Retry-After"))}

	var body struct {
		Error *struct {
			Type     string          `json:"type"`
			Reason   string          `json:"reason"`
			Messages json.RawMessage `json:"messages"`
		} `json:"error"`
		Code    any    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &body) == nil {
		switch {
		case body.Error != nil:
			e.Type = body.Error.Type
			e.Reason = body.Error.Reason
			e.Messages = decodeMessages(body.Error.Messages)
		case body.Message != "":
			if code, ok := body.Code.(string); ok {
				e.Type = code
			}
			e.Reason = body.Message
		}
	}

	if e.Type == "" {
		e.Type = ErrTypeAPI
	}
	if e.Reason == "" && len(e.Messages) == 0 {
		e.Reason = http.StatusText(status)
	}
	return e
}

// decodeMessages flattens Wompi's validation messages into dotted field
// paths. It accepts {"field": ["msg"]}, {"field": "msg"} and nested objects
// such as {"payment_method": {"messages": {"sandbox_status": ["msg"]}}},
// which becomes {"payment_method.sandbox_status": ["msg"]}.
func decodeMessages(raw json.RawMessage) map[string][]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return nil
	}
	out := map[string][]string{}
	flattenMessages("", root, out)
	return out
}

func flattenMessages(prefix string, v any, out map[string][]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			key := k
			switch {
			case k == "messages" && prefix != "":
				key = prefix // {"field": {"messages": {...}}} -> "field.<sub>"
			case prefix != "":
				key = prefix + "." + k
			}
			flattenMessages(key, child, out)
		}
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				out[prefix] = append(out[prefix], s)
			} else {
				flattenMessages(prefix, item, out)
			}
		}
	case nil:
	default:
		out[prefix] = append(out[prefix], fmt.Sprint(t))
	}
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
