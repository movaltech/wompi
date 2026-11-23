package wompi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// PaymentSourceService stores tokenized payment instruments so they can be
// charged later without the payer present (subscriptions, one-click,
// recurring billing). It requires the private key.
type PaymentSourceService struct {
	client *Client
}

// PaymentSourceType is the kind of instrument behind a payment source.
type PaymentSourceType string

// Payment source types.
const (
	PaymentSourceCard                PaymentSourceType = "CARD"
	PaymentSourceNequi               PaymentSourceType = "NEQUI"
	PaymentSourceDaviplata           PaymentSourceType = "DAVIPLATA"
	PaymentSourceBancolombiaTransfer PaymentSourceType = "BANCOLOMBIA_TRANSFER"
)

// PaymentSourceStatus is the status of a payment source.
type PaymentSourceStatus string

// Payment source statuses.
const (
	PaymentSourceAvailable PaymentSourceStatus = "AVAILABLE"
	PaymentSourcePending   PaymentSourceStatus = "PENDING"
	PaymentSourceVoided    PaymentSourceStatus = "VOIDED"
)

// CreatePaymentSourceParams creates a payment source.
type CreatePaymentSourceParams struct {
	Type PaymentSourceType `json:"type"`
	// Token is the card / Nequi / Daviplata / Bancolombia token id.
	Token         string `json:"token"`
	CustomerEmail string `json:"customer_email"`
	// AcceptanceToken is the end-user policy token the payer accepted.
	AcceptanceToken string `json:"acceptance_token"`
	// AcceptPersonalAuth is the personal data authorization token the payer
	// accepted.
	AcceptPersonalAuth string `json:"accept_personal_auth,omitempty"`
	// PaymentDescription is used by BANCOLOMBIA_TRANSFER sources.
	PaymentDescription string `json:"payment_description,omitempty"`
}

// PaymentSource is a stored payment instrument.
type PaymentSource struct {
	ID            int64               `json:"id"`
	Type          PaymentSourceType   `json:"type"`
	Token         string              `json:"token"`
	Status        PaymentSourceStatus `json:"status"`
	CustomerEmail string              `json:"customer_email"`
	// PublicData holds non-sensitive details of the instrument (type, phone
	// number, card brand, ...); its keys vary by type.
	PublicData map[string]any `json:"public_data"`
}

// Create stores a payment source via POST /payment_sources.
func (s *PaymentSourceService) Create(ctx context.Context, params CreatePaymentSourceParams) (*PaymentSource, error) {
	if params.Type == "" || strings.TrimSpace(params.Token) == "" || params.CustomerEmail == "" || params.AcceptanceToken == "" {
		return nil, invalidParams("type, token, customer_email and acceptance_token are required")
	}
	var env envelope[PaymentSource]
	err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/payment_sources", auth: authPrivate, body: params, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Get returns a payment source via GET /payment_sources/{id}.
func (s *PaymentSourceService) Get(ctx context.Context, id int64) (*PaymentSource, error) {
	if id <= 0 {
		return nil, invalidParams("payment source id is required")
	}
	var env envelope[PaymentSource]
	err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/payment_sources/" + strconv.FormatInt(id, 10), auth: authPrivate, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Void permanently disables a payment source via
// PUT /payment_sources/{id}/void. Its status becomes VOIDED.
//
// Wompi only voids sources created with the PREAUTHORIZATION financial
// operation; for other sources it returns ErrUnprocessable.
func (s *PaymentSourceService) Void(ctx context.Context, id int64) (*PaymentSource, error) {
	if id <= 0 {
		return nil, invalidParams("payment source id is required")
	}
	var env envelope[PaymentSource]
	err := s.client.do(ctx, request{
		method: http.MethodPut, path: "/payment_sources/" + strconv.FormatInt(id, 10) + "/void", auth: authPrivate, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}
