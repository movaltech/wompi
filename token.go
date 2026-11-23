package wompi

import (
	"context"
	"net/http"
	"strings"
)

// TokenService tokenizes payment instruments with the public key.
//
// PCI notice: tokenizing raw card data on your server places it in PCI DSS
// scope. Most integrations should tokenize cards in the browser or app (Wompi
// widget / Wompi.js) and send only the resulting token to the server.
// CreateCard exists for PCI-compliant merchants, back-office tools and tests.
type TokenService struct {
	client *Client
}

// TokenStatus is the status of an asynchronous token (Nequi, Bancolombia).
type TokenStatus string

// Token statuses.
const (
	TokenPending   TokenStatus = "PENDING"
	TokenApproved  TokenStatus = "APPROVED"
	TokenDeclined  TokenStatus = "DECLINED"
	TokenAvailable TokenStatus = "AVAILABLE"
)

// CardParams is the card data to tokenize.
type CardParams struct {
	Number     string `json:"number"`      // no spaces
	CVC        string `json:"cvc"`         // 3 or 4 digits
	ExpMonth   string `json:"exp_month"`   // two digits, e.g. "08"
	ExpYear    string `json:"exp_year"`    // two digits, e.g. "29"
	CardHolder string `json:"card_holder"` // name as printed on the card
}

// CardToken is a tokenized card. Use ID as PaymentMethod.Token or as the
// token of a CARD payment source.
type CardToken struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	Brand      string `json:"brand"`
	Name       string `json:"name"`
	LastFour   string `json:"last_four"`
	BIN        string `json:"bin"`
	ExpYear    string `json:"exp_year"`
	ExpMonth   string `json:"exp_month"`
	CardHolder string `json:"card_holder"`
	ExpiresAt  string `json:"expires_at"`
}

// CreateCard tokenizes a card via POST /tokens/cards.
func (s *TokenService) CreateCard(ctx context.Context, params CardParams) (*CardToken, error) {
	params.Number = strings.ReplaceAll(params.Number, " ", "")
	if params.Number == "" || params.CVC == "" || params.ExpMonth == "" || params.ExpYear == "" || params.CardHolder == "" {
		return nil, invalidParams("number, cvc, exp_month, exp_year and card_holder are required")
	}
	var env envelope[CardToken]
	err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/tokens/cards", auth: authPublic, body: params, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// NequiToken is a Nequi account subscription. The payer approves it from the
// Nequi app; poll GetNequi (or listen to the nequi_token.updated event) until
// Status is APPROVED before creating a payment source with it.
type NequiToken struct {
	ID          string      `json:"id"`
	Status      TokenStatus `json:"status"`
	PhoneNumber string      `json:"phone_number"`
	Name        string      `json:"name"`
}

// CreateNequi starts a Nequi subscription for a 10-digit Colombian phone
// number, via POST /tokens/nequi.
func (s *TokenService) CreateNequi(ctx context.Context, phoneNumber string) (*NequiToken, error) {
	if strings.TrimSpace(phoneNumber) == "" {
		return nil, invalidParams("phone number is required")
	}
	body := struct {
		PhoneNumber string `json:"phone_number"`
	}{phoneNumber}
	var env envelope[NequiToken]
	err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/tokens/nequi", auth: authPublic, body: body, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// GetNequi returns the current state of a Nequi token, via
// GET /tokens/nequi/{id}.
func (s *TokenService) GetNequi(ctx context.Context, id string) (*NequiToken, error) {
	if id == "" {
		return nil, invalidParams("token id is required")
	}
	var env envelope[NequiToken]
	err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/tokens/nequi/" + pathEscape(id), auth: authPublic, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Bancolombia transfer authorization types.
const (
	BancolombiaAuthTransaction = "TRANSACTION"
	BancolombiaAuthToken       = "TOKEN"
)

// BancolombiaTransferTokenParams starts a Bancolombia account subscription.
type BancolombiaTransferTokenParams struct {
	// RedirectURL is where the payer returns after authorizing.
	RedirectURL string `json:"redirect_url"`
	// TypeAuth is BancolombiaAuthTransaction or BancolombiaAuthToken.
	TypeAuth string `json:"type_auth"`
}

// BancolombiaTransferToken is a Bancolombia account subscription. When Status
// is PENDING, redirect the payer to AuthorizationURL, then poll
// GetBancolombiaTransfer (or listen to bancolombia_transfer_token.updated).
type BancolombiaTransferToken struct {
	ID               string      `json:"id"`
	Status           TokenStatus `json:"status"`
	AuthorizationURL string      `json:"authorization_url"`
}

// CreateBancolombiaTransfer starts a Bancolombia account subscription, via
// POST /tokens/bancolombia_transfer.
func (s *TokenService) CreateBancolombiaTransfer(ctx context.Context, params BancolombiaTransferTokenParams) (*BancolombiaTransferToken, error) {
	if params.TypeAuth == "" {
		params.TypeAuth = BancolombiaAuthToken
	}
	var env envelope[BancolombiaTransferToken]
	err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/tokens/bancolombia_transfer", auth: authPublic, body: params, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// GetBancolombiaTransfer returns the current state of a Bancolombia token,
// via GET /tokens/bancolombia_transfer/{id}.
func (s *TokenService) GetBancolombiaTransfer(ctx context.Context, id string) (*BancolombiaTransferToken, error) {
	if id == "" {
		return nil, invalidParams("token id is required")
	}
	var env envelope[BancolombiaTransferToken]
	err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/tokens/bancolombia_transfer/" + pathEscape(id), auth: authPublic, out: &env,
	})
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}
