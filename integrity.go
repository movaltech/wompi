package wompi

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// expirationLayout is the ISO 8601 UTC format Wompi uses for
// expiration_time, both in the payload and in the integrity signature.
const expirationLayout = "2006-01-02T15:04:05.000Z"

// FormatTime formats t as the ISO 8601 UTC timestamp Wompi expects
// (e.g. "2026-06-09T20:28:50.000Z").
func FormatTime(t time.Time) string { return t.UTC().Format(expirationLayout) }

// SignatureParams are the transaction fields covered by the integrity
// signature.
type SignatureParams struct {
	Reference     string
	AmountInCents int64
	Currency      Currency
	// ExpirationTime is optional. When non-zero it must be the same instant
	// sent in the transaction or checkout.
	ExpirationTime time.Time
}

// IntegritySignature computes Wompi's integrity signature:
//
//	hex(sha256(reference + amount_in_cents + currency [+ expiration_time] + integritySecret))
//
// Compute it only on the server: the integrity secret must never reach a
// browser or mobile app.
func IntegritySignature(p SignatureParams, integritySecret string) string {
	currency := p.Currency
	if currency == "" {
		currency = CurrencyCOP
	}
	raw := p.Reference + strconv.FormatInt(p.AmountInCents, 10) + string(currency)
	if !p.ExpirationTime.IsZero() {
		raw += FormatTime(p.ExpirationTime)
	}
	raw += integritySecret

	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IntegritySignature computes the integrity signature with the secret
// configured through WithIntegritySecret.
func (c *Client) IntegritySignature(p SignatureParams) (string, error) {
	if c.integritySecret == "" {
		return "", newError(ErrTypeMissingCredential, "integrity secret not configured; use WithIntegritySecret")
	}
	return IntegritySignature(p, c.integritySecret), nil
}
