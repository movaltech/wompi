package wompi

import (
	"context"
	"net/http"
	"strings"
)

// PaymentLinkService creates and manages hosted payment links
// (https://checkout.wompi.co/l/{id}).
type PaymentLinkService struct {
	client *Client
}

// CustomerReference is an extra field the payer is asked for on the
// checkout page (at most two per link).
type CustomerReference struct {
	Label      string `json:"label"` // max 24 characters
	IsRequired bool   `json:"is_required"`
}

// PaymentLinkCustomerData configures extra payer data requested by a link.
type PaymentLinkCustomerData struct {
	CustomerReferences []CustomerReference `json:"customer_references,omitempty"`
}

// CreatePaymentLinkParams creates a payment link.
type CreatePaymentLinkParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// SingleUse disables the link after its first approved payment.
	SingleUse bool `json:"single_use"`
	// CollectShipping asks the payer for a shipping address.
	CollectShipping        bool `json:"collect_shipping"`
	CollectCustomerLegalID bool `json:"collect_customer_legal_id,omitempty"`
	// Currency defaults to COP when AmountInCents is set, and is omitted for
	// open-amount links.
	Currency Currency `json:"currency,omitempty"`
	// AmountInCents fixes the amount. Leave it zero to let the payer choose.
	AmountInCents int64 `json:"amount_in_cents,omitempty"`
	// ExpiresAt is an ISO 8601 date-time after which the link stops working
	// (see FormatTime).
	ExpiresAt    string                   `json:"expires_at,omitempty"`
	RedirectURL  string                   `json:"redirect_url,omitempty"`
	ImageURL     string                   `json:"image_url,omitempty"`
	SKU          string                   `json:"sku,omitempty"` // max 36 characters
	CustomerData *PaymentLinkCustomerData `json:"customer_data,omitempty"`
	Taxes        []Tax                    `json:"taxes,omitempty"`
}

// PaymentLink is a hosted payment link.
type PaymentLink struct {
	ID                     string                   `json:"id"`
	Active                 bool                     `json:"active"`
	Name                   string                   `json:"name"`
	Description            string                   `json:"description"`
	SingleUse              bool                     `json:"single_use"`
	CollectShipping        bool                     `json:"collect_shipping"`
	CollectCustomerLegalID bool                     `json:"collect_customer_legal_id"`
	Currency               Currency                 `json:"currency"`
	AmountInCents          int64                    `json:"amount_in_cents"`
	ExpiresAt              string                   `json:"expires_at"`
	RedirectURL            string                   `json:"redirect_url"`
	ImageURL               string                   `json:"image_url"`
	SKU                    string                   `json:"sku"`
	CustomerData           *PaymentLinkCustomerData `json:"customer_data"`
	Taxes                  []Tax                    `json:"taxes"`
	MerchantPublicKey      string                   `json:"merchant_public_key"`
	CreatedAt              string                   `json:"created_at"`
	UpdatedAt              string                   `json:"updated_at"`

	checkoutBaseURL string
}

// CheckoutURL returns the URL to share with the payer.
func (l *PaymentLink) CheckoutURL() string {
	base := l.checkoutBaseURL
	if base == "" {
		base = CheckoutBaseURL
	}
	return base + "/l/" + l.ID
}

// Create creates a payment link via POST /payment_links. Requires the
// private key.
func (s *PaymentLinkService) Create(ctx context.Context, params CreatePaymentLinkParams) (*PaymentLink, error) {
	if strings.TrimSpace(params.Name) == "" || strings.TrimSpace(params.Description) == "" {
		return nil, invalidParams("name and description are required")
	}
	if params.CustomerData != nil && len(params.CustomerData.CustomerReferences) > 2 {
		return nil, invalidParams("at most two customer references are allowed")
	}
	// Wompi requires currency and amount_in_cents to be both set or both
	// null (open amount).
	switch {
	case params.AmountInCents > 0 && params.Currency == "":
		params.Currency = CurrencyCOP
	case params.AmountInCents == 0:
		params.Currency = ""
	}
	return s.call(ctx, http.MethodPost, "/payment_links", authPrivate, params)
}

// Get returns a payment link via GET /payment_links/{id}.
func (s *PaymentLinkService) Get(ctx context.Context, id string) (*PaymentLink, error) {
	if id == "" {
		return nil, invalidParams("payment link id is required")
	}
	return s.call(ctx, http.MethodGet, "/payment_links/"+pathEscape(id), authNone, nil)
}

// SetActive activates or deactivates a payment link via
// PATCH /payment_links/{id}. Requires the private key.
func (s *PaymentLinkService) SetActive(ctx context.Context, id string, active bool) (*PaymentLink, error) {
	if id == "" {
		return nil, invalidParams("payment link id is required")
	}
	body := struct {
		Active bool `json:"active"`
	}{active}
	return s.call(ctx, http.MethodPatch, "/payment_links/"+pathEscape(id), authPrivate, body)
}

func (s *PaymentLinkService) call(ctx context.Context, method, path string, auth authMode, body any) (*PaymentLink, error) {
	var env envelope[PaymentLink]
	if err := s.client.do(ctx, request{method: method, path: path, auth: auth, body: body, out: &env}); err != nil {
		return nil, err
	}
	env.Data.checkoutBaseURL = s.client.checkoutURL
	return &env.Data, nil
}
