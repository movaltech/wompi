package wompi

import (
	"context"
	"net/http"
)

// MerchantService exposes the merchant's public information, including the
// acceptance tokens required to create transactions and payment sources.
type MerchantService struct {
	client *Client
}

// Acceptance token types.
const (
	AcceptanceEndUserPolicy    = "END_USER_POLICY"
	AcceptancePersonalDataAuth = "PERSONAL_DATA_AUTH"
)

// PresignedAcceptance is a pre-signed acceptance token. Show Permalink (the
// policy PDF) to the payer and send AcceptanceToken only after the payer has
// explicitly accepted it.
type PresignedAcceptance struct {
	AcceptanceToken string `json:"acceptance_token"`
	Permalink       string `json:"permalink"`
	Type            string `json:"type"`
}

// Merchant is the merchant's public information.
type Merchant struct {
	ID                        int64               `json:"id"`
	Name                      string              `json:"name"`
	Email                     string              `json:"email"`
	ContactName               string              `json:"contact_name"`
	PhoneNumber               string              `json:"phone_number"`
	Active                    bool                `json:"active"`
	LogoURL                   string              `json:"logo_url"`
	LegalName                 string              `json:"legal_name"`
	LegalIDType               string              `json:"legal_id_type"`
	LegalID                   string              `json:"legal_id"`
	PublicKey                 string              `json:"public_key"`
	AcceptedCurrencies        []string            `json:"accepted_currencies"`
	AcceptedPaymentMethods    []string            `json:"accepted_payment_methods"`
	PresignedAcceptance       PresignedAcceptance `json:"presigned_acceptance"`
	PresignedPersonalDataAuth PresignedAcceptance `json:"presigned_personal_data_auth"`
}

// AcceptanceTokens groups the two tokens Wompi requires: the end-user policy
// and the personal data processing authorization.
type AcceptanceTokens struct {
	EndUserPolicy    PresignedAcceptance
	PersonalDataAuth PresignedAcceptance
}

// Get returns the merchant associated with the client's public key, via
// GET /merchants/info.
func (s *MerchantService) Get(ctx context.Context) (*Merchant, error) {
	var env envelope[Merchant]
	err := s.client.do(ctx, request{
		method: http.MethodGet,
		path:   "/merchants/info",
		auth:   authNone,
		header: http.Header{"X-Merchant-Public-Key": {s.client.publicKey}},
		out:    &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// AcceptanceTokens returns fresh acceptance tokens. Tokens are single-use
// and short-lived: fetch new ones for every transaction or payment source
// instead of caching them.
func (s *MerchantService) AcceptanceTokens(ctx context.Context) (*AcceptanceTokens, error) {
	m, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &AcceptanceTokens{
		EndUserPolicy:    m.PresignedAcceptance,
		PersonalDataAuth: m.PresignedPersonalDataAuth,
	}, nil
}
