package wompi

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WebCheckoutParams describes a Wompi Web Checkout redirect
// (https://checkout.wompi.co/p/). The payer chooses the payment method on
// Wompi's hosted page, so no card data or acceptance token touches your
// server.
type WebCheckoutParams struct {
	Reference     string
	AmountInCents int64
	// Currency defaults to COP.
	Currency Currency
	// RedirectURL receives the payer after the payment, with ?id=<transaction id>.
	RedirectURL    string
	ExpirationTime time.Time

	CustomerEmail       string
	CustomerFullName    string
	CustomerPhoneNumber string
	CustomerPhonePrefix string // e.g. "+57"
	CustomerLegalID     string
	CustomerLegalIDType LegalIDType

	ShippingAddress *ShippingAddress

	TaxVATInCents         int64
	TaxConsumptionInCents int64

	CollectShipping        bool
	CollectCustomerLegalID bool
}

// WebCheckoutURL returns a signed Web Checkout URL to redirect the payer to.
// It requires the integrity secret (WithIntegritySecret).
func (c *Client) WebCheckoutURL(p WebCheckoutParams) (string, error) {
	if strings.TrimSpace(p.Reference) == "" || p.AmountInCents <= 0 {
		return "", invalidParams("reference and a positive amount_in_cents are required")
	}
	if p.Currency == "" {
		p.Currency = CurrencyCOP
	}
	signature, err := c.IntegritySignature(SignatureParams{
		Reference:      p.Reference,
		AmountInCents:  p.AmountInCents,
		Currency:       p.Currency,
		ExpirationTime: p.ExpirationTime,
	})
	if err != nil {
		return "", err
	}

	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	q.Set("public-key", c.publicKey)
	q.Set("currency", string(p.Currency))
	q.Set("amount-in-cents", strconv.FormatInt(p.AmountInCents, 10))
	q.Set("reference", p.Reference)
	q.Set("signature:integrity", signature)
	set("redirect-url", p.RedirectURL)
	if !p.ExpirationTime.IsZero() {
		q.Set("expiration-time", FormatTime(p.ExpirationTime))
	}

	set("customer-data:email", p.CustomerEmail)
	set("customer-data:full-name", p.CustomerFullName)
	set("customer-data:phone-number", p.CustomerPhoneNumber)
	set("customer-data:phone-number-prefix", p.CustomerPhonePrefix)
	set("customer-data:legal-id", p.CustomerLegalID)
	set("customer-data:legal-id-type", string(p.CustomerLegalIDType))

	if a := p.ShippingAddress; a != nil {
		set("shipping-address:address-line-1", a.AddressLine1)
		set("shipping-address:address-line-2", a.AddressLine2)
		set("shipping-address:country", a.Country)
		set("shipping-address:region", a.Region)
		set("shipping-address:city", a.City)
		set("shipping-address:name", a.Name)
		set("shipping-address:phone-number", a.PhoneNumber)
		set("shipping-address:postal-code", a.PostalCode)
	}

	if p.TaxVATInCents > 0 {
		q.Set("tax-in-cents:vat", strconv.FormatInt(p.TaxVATInCents, 10))
	}
	if p.TaxConsumptionInCents > 0 {
		q.Set("tax-in-cents:consumption", strconv.FormatInt(p.TaxConsumptionInCents, 10))
	}
	if p.CollectShipping {
		q.Set("collect-shipping", "true")
	}
	if p.CollectCustomerLegalID {
		q.Set("collect-customer-legal-id", "true")
	}

	return c.checkoutURL + "/p/?" + q.Encode(), nil
}
