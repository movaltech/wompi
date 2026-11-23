package wompi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseAPIErrorValidation(t *testing.T) {
	raw := []byte(`{"error":{"type":"INPUT_VALIDATION_ERROR","messages":{"reference":["La referencia ya ha sido usada"],"amount_in_cents":["Debe ser mayor a 0"]}}}`)
	e := parseAPIError(422, http.Header{}, raw)

	if !errors.Is(e, ErrInputValidation) {
		t.Fatalf("expected ErrInputValidation, got %v", e)
	}
	if e.HTTPStatus != 422 {
		t.Errorf("HTTPStatus = %d", e.HTTPStatus)
	}
	if got := e.Messages["reference"]; len(got) != 1 || got[0] != "La referencia ya ha sido usada" {
		t.Errorf("Messages = %v", e.Messages)
	}
	want := "wompi: INPUT_VALIDATION_ERROR (http 422) [amount_in_cents: Debe ser mayor a 0; reference: La referencia ya ha sido usada]"
	if e.Error() != want {
		t.Errorf("Error() = %q\nwant      %q", e.Error(), want)
	}
}

func TestParseAPIErrorShapes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantType   string
		wantReason string
	}{
		{"not found", 404, `{"error":{"type":"NOT_FOUND_ERROR","reason":"La entidad solicitada no existe"}}`, ErrTypeNotFound, "La entidad solicitada no existe"},
		{"invalid token", 401, `{"error":{"type":"INVALID_ACCESS_TOKEN","reason":"Se esperaba una llave"}}`, ErrTypeInvalidAccessToken, "Se esperaba una llave"},
		{"code/message", 422, `{"status":422,"code":"ERROR","message":"OTP inválido"}`, "ERROR", "OTP inválido"},
		{"not json", 502, `<html>bad gateway</html>`, ErrTypeAPI, "Bad Gateway"},
		{"empty", 500, ``, ErrTypeAPI, "Internal Server Error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := parseAPIError(tc.status, http.Header{}, []byte(tc.body))
			if e.Type != tc.wantType || e.Reason != tc.wantReason || e.HTTPStatus != tc.status {
				t.Errorf("got %+v", e)
			}
		})
	}
}

func TestDecodeMessagesVariants(t *testing.T) {
	if m := decodeMessages([]byte(`{"a":"x"}`)); m["a"][0] != "x" {
		t.Errorf("single: %v", m)
	}
	if m := decodeMessages([]byte(`{"a":{"b":1}}`)); m["a.b"][0] != "1" {
		t.Errorf("nested: %v", m)
	}
	// Shape returned by the live sandbox for a missing payment_method field.
	live := `{"payment_method":{"messages":{"sandbox_status_mandatory_if_sandbox":["Debe ser completado"]}}}`
	if m := decodeMessages([]byte(live)); len(m) != 1 || m["payment_method.sandbox_status_mandatory_if_sandbox"][0] != "Debe ser completado" {
		t.Errorf("nested messages: %v", m)
	}
	if m := decodeMessages([]byte(`null`)); m != nil {
		t.Errorf("null: %v", m)
	}
}

func TestErrorIsAndAs(t *testing.T) {
	inner := errors.New("boom")
	var err error = fmt.Errorf("wrapped: %w", &Error{Type: ErrTypeCommunication, Reason: "x", Err: inner})

	if !errors.Is(err, ErrCommunication) {
		t.Error("errors.Is should match on Type through wrapping")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("errors.Is should not match a different Type")
	}
	if !errors.Is(err, inner) {
		t.Error("errors.Is should reach the underlying error")
	}
	var wErr *Error
	if !errors.As(err, &wErr) || wErr.Reason != "x" {
		t.Error("errors.As should expose *Error")
	}
	if !strings.HasSuffix(wErr.Error(), ": boom") {
		t.Errorf("Error() = %q", wErr.Error())
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter("3"); d != 3*time.Second {
		t.Errorf("seconds: %v", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Errorf("empty: %v", d)
	}
	if d := parseRetryAfter("-1"); d != 0 {
		t.Errorf("negative: %v", d)
	}
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(future); d <= 0 || d > 5*time.Second {
		t.Errorf("http date: %v", d)
	}
}
