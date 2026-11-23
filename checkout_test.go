package wompi

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestWebCheckoutURL(t *testing.T) {
	c, err := New(testPublicKey, testPrivateKey, WithIntegritySecret(testIntegritySecret))
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Date(2026, 6, 9, 20, 28, 50, 0, time.UTC)
	raw, err := c.WebCheckoutURL(WebCheckoutParams{
		Reference: "ref-1", AmountInCents: 100000, RedirectURL: "https://shop.example/done",
		ExpirationTime: exp, CustomerEmail: "a@b.co", TaxVATInCents: 15966,
		ShippingAddress: &ShippingAddress{AddressLine1: "Calle 1", City: "Bogotá", Country: "CO", Region: "DC", PhoneNumber: "3000000000"},
		CollectShipping: true,
	})
	if err != nil {
		t.Fatalf("WebCheckoutURL: %v", err)
	}
	if !strings.HasPrefix(raw, CheckoutBaseURL+"/p/?") {
		t.Fatalf("url = %s", raw)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	want := map[string]string{
		"public-key":                      testPublicKey,
		"currency":                        "COP",
		"amount-in-cents":                 "100000",
		"reference":                       "ref-1",
		"signature:integrity":             "28708487a6a85ed65bd651212f70f0fd65e7793e5bec400dba6ea740ad444da6",
		"redirect-url":                    "https://shop.example/done",
		"expiration-time":                 "2026-06-09T20:28:50.000Z",
		"customer-data:email":             "a@b.co",
		"tax-in-cents:vat":                "15966",
		"shipping-address:city":           "Bogotá",
		"shipping-address:address-line-1": "Calle 1",
		"collect-shipping":                "true",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("customer-data:full-name") || q.Has("collect-customer-legal-id") {
		t.Error("empty optional fields must be omitted")
	}
}

func TestWebCheckoutURLErrors(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey)
	if _, err := c.WebCheckoutURL(WebCheckoutParams{Reference: "r", AmountInCents: 1}); !errors.Is(err, ErrMissingCredential) {
		t.Errorf("no secret: err = %v", err)
	}
	if _, err := c.WebCheckoutURL(WebCheckoutParams{}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("empty params: err = %v", err)
	}
}
