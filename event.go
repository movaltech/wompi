package wompi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Event types sent by Wompi to the events URL. The list may grow; handle
// unknown types gracefully.
const (
	EventTransactionUpdated              = "transaction.updated"
	EventNequiTokenUpdated               = "nequi_token.updated"
	EventBancolombiaTransferTokenUpdated = "bancolombia_transfer_token.updated"
)

// EventChecksumHeader is the HTTP header carrying the event checksum.
const EventChecksumHeader = "X-Event-Checksum"

// maxEventBytes caps the size of an event body.
const maxEventBytes = 1 << 20

// EventSignature is the signature block of an event.
type EventSignature struct {
	// Properties are dotted paths into Data (e.g. "transaction.id") whose
	// values, in order, are covered by Checksum.
	Properties []string `json:"properties"`
	Checksum   string   `json:"checksum"`
}

// EventData is the payload of an event. Exactly one field is set,
// depending on Event.
type EventData struct {
	Transaction              *Transaction              `json:"transaction,omitempty"`
	NequiToken               *NequiToken               `json:"nequi_token,omitempty"`
	BancolombiaTransferToken *BancolombiaTransferToken `json:"bancolombia_transfer_token,omitempty"`
}

// Event is a notification sent by Wompi to the merchant's events URL.
type Event struct {
	Event       string         `json:"event"`
	Data        EventData      `json:"data"`
	Environment string         `json:"environment"` // "test" or "prod"
	Signature   EventSignature `json:"signature"`
	Timestamp   int64          `json:"timestamp"`
	SentAt      string         `json:"sent_at"`

	// RawData is the undecoded "data" object, used to verify the checksum
	// and available for fields not modeled by EventData.
	RawData json.RawMessage `json:"-"`
}

// ParseEvent decodes an event body without verifying it. Use VerifyEvent or
// ParseEventRequest before trusting its contents.
func ParseEvent(body []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, &Error{Type: ErrTypeDecode, Reason: "invalid event body", Err: err}
	}
	var raw struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(body, &raw)
	e.RawData = raw.Data
	return &e, nil
}

// EventChecksum computes the checksum Wompi sends for e: the SHA-256 of the
// values of e.Signature.Properties (read from the event data, in order),
// followed by the timestamp and the events secret.
func EventChecksum(e *Event, eventsSecret string) (string, error) {
	var data any
	dec := json.NewDecoder(bytes.NewReader(e.RawData))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return "", &Error{Type: ErrTypeDecode, Reason: "event data is not valid JSON", Err: err}
	}

	var b strings.Builder
	for _, prop := range e.Signature.Properties {
		v, ok := lookupPath(data, prop)
		if !ok {
			return "", &Error{Type: ErrTypeInvalidSignature, Reason: "signed property not found in event data: " + prop}
		}
		b.WriteString(stringifyValue(v))
	}
	b.WriteString(strconv.FormatInt(e.Timestamp, 10))
	b.WriteString(eventsSecret)

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

// VerifyEvent checks e's checksum against the events secret, in constant
// time. checksum overrides e.Signature.Checksum when non-empty (e.g. the
// X-Event-Checksum header).
func VerifyEvent(e *Event, eventsSecret, checksum string) error {
	if eventsSecret == "" {
		return newError(ErrTypeMissingCredential, "events secret is required")
	}
	if checksum == "" {
		checksum = e.Signature.Checksum
	}
	if checksum == "" {
		return newError(ErrTypeInvalidSignature, "event has no checksum")
	}
	expected, err := EventChecksum(e, eventsSecret)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(checksum))) != 1 {
		return newError(ErrTypeInvalidSignature, "event checksum mismatch")
	}
	return nil
}

// ParseEventRequest reads, decodes and verifies an event from an incoming
// HTTP request.
func ParseEventRequest(r *http.Request, eventsSecret string) (*Event, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxEventBytes))
	if err != nil {
		return nil, &Error{Type: ErrTypeCommunication, Reason: "failed to read event body", Err: err}
	}
	e, err := ParseEvent(body)
	if err != nil {
		return nil, err
	}
	checksum := e.Signature.Checksum
	if checksum == "" {
		checksum = r.Header.Get(EventChecksumHeader)
	}
	if err := VerifyEvent(e, eventsSecret, checksum); err != nil {
		return nil, err
	}
	return e, nil
}

// EventHandlerFunc processes a verified event. Return a non-nil error to
// make Wompi retry the delivery later.
type EventHandlerFunc func(ctx context.Context, e *Event) error

// NewEventHandler returns an http.Handler for the events URL. It verifies
// every event, responds 400 to invalid ones, 500 when fn fails (so Wompi
// retries) and 200 otherwise. Wompi may deliver an event more than once:
// make fn idempotent (e.g. key on transaction id + status).
func NewEventHandler(eventsSecret string, fn EventHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		e, err := ParseEventRequest(r, eventsSecret)
		if err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		if err := fn(r.Context(), e); err != nil {
			http.Error(w, "event processing failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// ParseEvent reads and verifies an event with the secret configured through
// WithEventsSecret.
func (c *Client) ParseEvent(r *http.Request) (*Event, error) {
	if c.eventsSecret == "" {
		return nil, newError(ErrTypeMissingCredential, "events secret not configured; use WithEventsSecret")
	}
	return ParseEventRequest(r, c.eventsSecret)
}

// EventHandler is NewEventHandler using the secret configured through
// WithEventsSecret. It panics if no events secret is configured, surfacing
// the misconfiguration at startup instead of on the first event.
func (c *Client) EventHandler(fn EventHandlerFunc) http.Handler {
	if c.eventsSecret == "" {
		panic("wompi: EventHandler requires WithEventsSecret")
	}
	return NewEventHandler(c.eventsSecret, fn)
}

func lookupPath(data any, path string) (any, bool) {
	cur := data
	for part := range strings.SplitSeq(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func stringifyValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
