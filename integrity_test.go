package wompi

import (
	"errors"
	"testing"
	"time"
)

// TestIntegritySignatureDocumentedVector uses the exact example published in
// Wompi's Widget & Web Checkout documentation.
func TestIntegritySignatureDocumentedVector(t *testing.T) {
	got := IntegritySignature(SignatureParams{
		Reference:     "sk8-438k4-xmxm392-sn2m",
		AmountInCents: 2490000,
		Currency:      CurrencyCOP,
	}, "prod_integrity_Z5mMke9x0k8gpErbDqwrJXMqsI6SFli6")

	const want = "37c8407747e595535433ef8f6a811d853cd943046624a0ec04662b17bbf33bf5"
	if got != want {
		t.Errorf("signature = %s, want %s", got, want)
	}
}

// TestIntegritySignatureWithExpiration checks the expiration variant against
// sha256("ref-1100000COP2026-06-09T20:28:50.000Ztest_integrity_abc"),
// computed independently with sha256sum.
func TestIntegritySignatureWithExpiration(t *testing.T) {
	bogota := time.FixedZone("COT", -5*3600)
	exp := time.Date(2026, 6, 9, 15, 28, 50, 0, bogota) // == 20:28:50Z

	got := IntegritySignature(SignatureParams{
		Reference: "ref-1", AmountInCents: 100000, ExpirationTime: exp, // currency defaults to COP
	}, testIntegritySecret)

	const want = "28708487a6a85ed65bd651212f70f0fd65e7793e5bec400dba6ea740ad444da6"
	if got != want {
		t.Errorf("signature = %s, want %s", got, want)
	}
}

func TestClientIntegritySignatureRequiresSecret(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey)
	if _, err := c.IntegritySignature(SignatureParams{Reference: "r", AmountInCents: 1}); !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("err = %v, want ErrMissingCredential", err)
	}
}

func TestFormatTime(t *testing.T) {
	got := FormatTime(time.Date(2026, 1, 2, 3, 4, 5, 6e6, time.UTC))
	if got != "2026-01-02T03:04:05.006Z" {
		t.Errorf("FormatTime = %q", got)
	}
}
