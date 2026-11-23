package wompi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validChecksum is sha256("1234-1610641025-49201APPROVED44900001530291411test_events_secret"),
// computed independently with sha256sum, so the test proves the exact
// concatenation order (properties, timestamp, secret) rather than internal
// consistency.
const validChecksum = "82d7b6fae35f0de10bb9186be459d13d54d3752c4def83a41fb8ceea85b53633"

func eventBody(checksum string) string {
	return `{
  "event": "transaction.updated",
  "data": {
    "transaction": {
      "id": "1234-1610641025-49201",
      "amount_in_cents": 4490000,
      "reference": "MZQ3X2DE2SMX",
      "customer_email": "juan.perez@gmail.com",
      "currency": "COP",
      "payment_method_type": "NEQUI",
      "redirect_url": "https://mitienda.com.co/pagos/redireccion",
      "status": "APPROVED",
      "shipping_address": null,
      "payment_link_id": null,
      "payment_source_id": null
    }
  },
  "environment": "test",
  "signature": {
    "properties": ["transaction.id", "transaction.status", "transaction.amount_in_cents"],
    "checksum": "` + checksum + `"
  },
  "timestamp": 1530291411,
  "sent_at": "2018-07-20T16:45:05.000Z"
}`
}

func TestVerifyEventAcceptsValidChecksum(t *testing.T) {
	e, err := ParseEvent([]byte(eventBody(validChecksum)))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if err := VerifyEvent(e, testEventsSecret, ""); err != nil {
		t.Fatalf("VerifyEvent: %v", err)
	}
	if e.Event != EventTransactionUpdated || e.Data.Transaction == nil ||
		e.Data.Transaction.Status != StatusApproved || e.Data.Transaction.AmountInCents != 4490000 {
		t.Errorf("event not decoded: %+v", e)
	}
}

func TestVerifyEventAcceptsUppercaseChecksum(t *testing.T) {
	e, _ := ParseEvent([]byte(eventBody(strings.ToUpper(validChecksum))))
	if err := VerifyEvent(e, testEventsSecret, ""); err != nil {
		t.Fatalf("VerifyEvent: %v", err)
	}
}

func TestVerifyEventRejects(t *testing.T) {
	cases := map[string]struct {
		body, secret string
	}{
		"tampered amount": {strings.Replace(eventBody(validChecksum), "4490000", "100", 1), testEventsSecret},
		"wrong secret":    {eventBody(validChecksum), "test_events_other"},
		"no checksum":     {eventBody(""), testEventsSecret},
		"missing prop":    {strings.Replace(eventBody(validChecksum), `"status": "APPROVED",`, "", 1), testEventsSecret},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, err := ParseEvent([]byte(tc.body))
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			if err := VerifyEvent(e, tc.secret, ""); !errors.Is(err, ErrInvalidSignature) {
				t.Errorf("err = %v, want ErrInvalidSignature", err)
			}
		})
	}
	e, _ := ParseEvent([]byte(eventBody(validChecksum)))
	if err := VerifyEvent(e, "", ""); !errors.Is(err, ErrMissingCredential) {
		t.Errorf("empty secret: err = %v", err)
	}
}

func TestParseEventRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseEvent([]byte("{")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseEventRequestUsesHeaderChecksum(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/wompi/events", strings.NewReader(eventBody("")))
	r.Header.Set(EventChecksumHeader, validChecksum)
	if _, err := ParseEventRequest(r, testEventsSecret); err != nil {
		t.Fatalf("ParseEventRequest: %v", err)
	}
}

func TestEventHandler(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey, WithEventsSecret(testEventsSecret))

	var got *Event
	var fail bool
	h := c.EventHandler(func(ctx context.Context, e *Event) error {
		got = e
		if fail {
			return errors.New("db down")
		}
		return nil
	})

	cases := []struct {
		name   string
		method string
		body   string
		fail   bool
		want   int
	}{
		{"valid", http.MethodPost, eventBody(validChecksum), false, http.StatusOK},
		{"handler error asks Wompi to retry", http.MethodPost, eventBody(validChecksum), true, http.StatusInternalServerError},
		{"bad signature", http.MethodPost, eventBody(strings.Repeat("0", 64)), false, http.StatusBadRequest},
		{"wrong method", http.MethodGet, "", false, http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, fail = nil, tc.fail
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, "/wompi/events", strings.NewReader(tc.body)))
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && (got == nil || got.Data.Transaction.ID != "1234-1610641025-49201") {
				t.Errorf("handler did not receive the event")
			}
		})
	}
}

func TestClientEventHelpersRequireSecret(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(eventBody(validChecksum)))
	if _, err := c.ParseEvent(r); !errors.Is(err, ErrMissingCredential) {
		t.Errorf("ParseEvent: err = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("EventHandler should panic without an events secret")
		}
	}()
	c.EventHandler(func(context.Context, *Event) error { return nil })
}

func TestStringifyValue(t *testing.T) {
	cases := map[string]any{"": nil, "x": "x", "true": true, `{"a":1}`: map[string]any{"a": 1}}
	for want, v := range cases {
		if got := stringifyValue(v); got != want {
			t.Errorf("stringifyValue(%v) = %q, want %q", v, got, want)
		}
	}
}
