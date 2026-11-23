package wompi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestNewInfersSandbox(t *testing.T) {
	c, err := New(testPublicKey, testPrivateKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Environment() != Sandbox {
		t.Errorf("Environment = %q, want sandbox", c.Environment())
	}
	if c.baseURL != SandboxBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, SandboxBaseURL)
	}
	if c.PublicKey() != testPublicKey {
		t.Errorf("PublicKey = %q", c.PublicKey())
	}
	if c.httpClient == nil || c.httpClient == http.DefaultClient || c.httpClient.Timeout != defaultTimeout {
		t.Errorf("expected a dedicated http client with a %v timeout", defaultTimeout)
	}
	if c.Merchants == nil || c.Tokens == nil || c.PaymentSources == nil || c.Transactions == nil ||
		c.PaymentLinks == nil || c.Refunds == nil || c.PSE == nil {
		t.Error("services not initialized")
	}
}

func TestNewInfersProduction(t *testing.T) {
	c, err := New("pub_prod_a", "prv_prod_b", WithIntegritySecret("prod_integrity_c"), WithEventsSecret("prod_events_d"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Environment() != Production || c.baseURL != ProductionBaseURL {
		t.Errorf("env = %q, baseURL = %q", c.Environment(), c.baseURL)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	cases := []struct {
		name       string
		pub, prv   string
		opts       []Option
		wantSubstr string
	}{
		{"missing public key", "  ", testPrivateKey, nil, "public key is required"},
		{"mixed keys", "pub_test_a", "prv_prod_b", nil, "private key belongs to production"},
		{"mixed secret", "pub_prod_a", "prv_prod_b", []Option{WithIntegritySecret("test_integrity_x")}, "integrity secret belongs to sandbox"},
		{"explicit env mismatch", "pub_test_a", "", []Option{WithEnvironment(Production)}, "WithEnvironment(production)"},
		{"unknown env", "pub_test_a", "", []Option{WithEnvironment("staging")}, "unknown environment"},
		{"uninferable", "pub_custom_a", "", nil, "cannot infer environment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.pub, tc.prv, tc.opts...)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %q, want it to contain %q", err, tc.wantSubstr)
			}
		})
	}
}

func TestNewExplicitEnvironmentWithUnknownPrefix(t *testing.T) {
	c, err := New("pub_custom_a", "", WithEnvironment(Production))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Environment() != Production {
		t.Errorf("Environment = %q", c.Environment())
	}
}

func TestNewAppliesOptions(t *testing.T) {
	hc := &http.Client{}
	c, err := New(testPublicKey, testPrivateKey,
		WithHTTPClient(hc),
		WithBaseURL("https://api.example.com/v1/"),
		WithCheckoutBaseURL("https://checkout.example.com/"),
		WithRetries(3),
		WithUserAgent("my-shop/1.0"),
		nil, // nil options are ignored
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.httpClient != hc {
		t.Error("http client not overridden")
	}
	if c.baseURL != "https://api.example.com/v1" || c.checkoutURL != "https://checkout.example.com" {
		t.Errorf("urls not normalized: %q %q", c.baseURL, c.checkoutURL)
	}
	if c.retries != 3 {
		t.Errorf("retries = %d", c.retries)
	}
	if c.userAgent != "my-shop/1.0 wompi-go/"+Version {
		t.Errorf("userAgent = %q", c.userAgent)
	}
}

func TestEnvironmentBaseURL(t *testing.T) {
	if Sandbox.BaseURL() != SandboxBaseURL || Production.BaseURL() != ProductionBaseURL {
		t.Error("unexpected base URLs")
	}
}

func TestAmountInCents(t *testing.T) {
	if got := AmountInCents(50000); got != 5000000 {
		t.Errorf("AmountInCents(50000) = %d", got)
	}
}
