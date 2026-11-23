package wompi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TransactionService creates, retrieves, searches and voids transactions.
type TransactionService struct {
	client *Client
}

// TransactionStatus is the status of a transaction.
type TransactionStatus string

// Transaction statuses. Every transaction starts as PENDING.
const (
	StatusPending  TransactionStatus = "PENDING"
	StatusApproved TransactionStatus = "APPROVED"
	StatusDeclined TransactionStatus = "DECLINED"
	StatusVoided   TransactionStatus = "VOIDED"
	StatusError    TransactionStatus = "ERROR"
)

// IsFinal reports whether the status will not change any more.
func (s TransactionStatus) IsFinal() bool {
	switch s {
	case StatusApproved, StatusDeclined, StatusVoided, StatusError:
		return true
	}
	return false
}

// CreateTransactionParams creates a transaction. Provide PaymentMethod for a
// payer-present payment, or PaymentSourceID for a charge to a stored payment
// source. Source charges may still need a PaymentMethod: SourceInstallments
// for CARD sources, or a BancolombiaTransferPayment for Bancolombia sources.
type CreateTransactionParams struct {
	AmountInCents int64
	// Currency defaults to COP.
	Currency      Currency
	CustomerEmail string
	// Reference is your unique reference for the payment. Wompi rejects
	// duplicate references, which also makes retries safe.
	Reference string

	// AcceptanceToken is the end-user policy token the payer accepted
	// (see Merchants.AcceptanceTokens).
	AcceptanceToken string
	// AcceptPersonalAuth is the personal data authorization token the payer
	// accepted.
	AcceptPersonalAuth string

	// Signature is the integrity signature. Leave it empty to have the SDK
	// compute it from the secret configured with WithIntegritySecret.
	Signature string

	PaymentMethod *PaymentMethod
	// PaymentMethodType is sent at the top level; it is filled
	// automatically for methods that require it (DAVIPLATA, SU_PLUS,
	// BANCOLOMBIA_BNPL).
	PaymentMethodType PaymentMethodType

	PaymentSourceID int64
	// Recurrent flags a card-on-file charge to a payment source.
	Recurrent bool

	RedirectURL string
	// ExpirationTime makes the transaction expire if not finalized before it.
	ExpirationTime  time.Time
	CustomerData    *CustomerData
	ShippingAddress *ShippingAddress
	Taxes           []Tax
	TipInCents      int64
	// IP is the payer's IP address.
	IP string
	// ParentTransactionID links the second leg of a Puntos Colombia (PCOL)
	// split payment to its parent transaction.
	ParentTransactionID string
}

type createTransactionBody struct {
	AmountInCents       int64             `json:"amount_in_cents"`
	Currency            Currency          `json:"currency"`
	CustomerEmail       string            `json:"customer_email"`
	Reference           string            `json:"reference"`
	AcceptanceToken     string            `json:"acceptance_token,omitempty"`
	AcceptPersonalAuth  string            `json:"accept_personal_auth,omitempty"`
	Signature           string            `json:"signature,omitempty"`
	PaymentMethod       *PaymentMethod    `json:"payment_method,omitempty"`
	PaymentMethodType   PaymentMethodType `json:"payment_method_type,omitempty"`
	PaymentSourceID     int64             `json:"payment_source_id,omitempty"`
	Recurrent           bool              `json:"recurrent,omitempty"`
	RedirectURL         string            `json:"redirect_url,omitempty"`
	ExpirationTime      string            `json:"expiration_time,omitempty"`
	CustomerData        *CustomerData     `json:"customer_data,omitempty"`
	ShippingAddress     *ShippingAddress  `json:"shipping_address,omitempty"`
	Taxes               []Tax             `json:"taxes,omitempty"`
	TipInCents          int64             `json:"tip_in_cents,omitempty"`
	IP                  string            `json:"ip,omitempty"`
	ParentTransactionID string            `json:"parent_transaction_id,omitempty"`
}

// TransactionPaymentMethod is the payment method of a returned transaction.
type TransactionPaymentMethod struct {
	Type               PaymentMethodType `json:"type"`
	Token              string            `json:"token,omitempty"`
	Installments       int               `json:"installments,omitempty"`
	PhoneNumber        string            `json:"phone_number,omitempty"`
	PaymentDescription string            `json:"payment_description,omitempty"`
	// Extra holds method-specific data returned by Wompi, such as
	// async_payment_url (PSE, Bancolombia transfer, PCOL), url (Daviplata,
	// SU+, BNPL), brand / last_four (cards), qr_image (QR) or
	// business_agreement_code (Bancolombia collect).
	Extra map[string]any `json:"extra,omitempty"`
}

// TransactionMerchant is the merchant summary embedded in a transaction.
type TransactionMerchant struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	LegalName   string `json:"legal_name"`
	ContactName string `json:"contact_name"`
	PhoneNumber string `json:"phone_number"`
	LogoURL     string `json:"logo_url"`
	LegalIDType string `json:"legal_id_type"`
	Email       string `json:"email"`
	LegalID     string `json:"legal_id"`
	PublicKey   string `json:"public_key"`
}

// Transaction is a Wompi transaction.
type Transaction struct {
	ID                string                   `json:"id"`
	CreatedAt         string                   `json:"created_at"`
	FinalizedAt       string                   `json:"finalized_at"`
	AmountInCents     int64                    `json:"amount_in_cents"`
	Reference         string                   `json:"reference"`
	CustomerEmail     string                   `json:"customer_email"`
	Currency          Currency                 `json:"currency"`
	PaymentMethodType PaymentMethodType        `json:"payment_method_type"`
	PaymentMethod     TransactionPaymentMethod `json:"payment_method"`
	Status            TransactionStatus        `json:"status"`
	StatusMessage     string                   `json:"status_message"`
	RedirectURL       string                   `json:"redirect_url"`
	PaymentSourceID   int64                    `json:"payment_source_id"`
	PaymentLinkID     string                   `json:"payment_link_id"`
	CustomerData      *CustomerData            `json:"customer_data"`
	ShippingAddress   *ShippingAddress         `json:"shipping_address"`
	Taxes             []Tax                    `json:"taxes"`
	TipInCents        int64                    `json:"tip_in_cents"`
	Merchant          *TransactionMerchant     `json:"merchant"`
}

// AsyncPaymentURL returns the URL the payer must be redirected to in order
// to complete an asynchronous payment (PSE, Bancolombia transfer, PCOL,
// Daviplata, SU+, BNPL), or "" while Wompi has not produced it yet.
func (t *Transaction) AsyncPaymentURL() string {
	if u := t.ExtraString("async_payment_url"); u != "" {
		return u
	}
	return t.ExtraString("url")
}

// ExtraString returns payment_method.extra[key] as a string, or "" if absent.
func (t *Transaction) ExtraString(key string) string {
	v, ok := t.PaymentMethod.Extra[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// Create creates a transaction via POST /transactions.
//
// The private key is used when configured (required for payment sources),
// otherwise the public key.
func (s *TransactionService) Create(ctx context.Context, params CreateTransactionParams) (*Transaction, error) {
	if params.AmountInCents <= 0 {
		return nil, invalidParams("amount_in_cents must be greater than zero")
	}
	if strings.TrimSpace(params.Reference) == "" {
		return nil, invalidParams("reference is required")
	}
	if params.CustomerEmail == "" {
		return nil, invalidParams("customer_email is required")
	}
	if params.PaymentMethod == nil && params.PaymentSourceID == 0 {
		return nil, invalidParams("payment_method or payment_source_id is required")
	}
	if params.PaymentSourceID == 0 && params.PaymentMethod.Type == "" {
		return nil, invalidParams("payment_method.type is required for payer-present payments")
	}
	if params.PaymentSourceID == 0 && params.AcceptanceToken == "" {
		return nil, invalidParams("acceptance_token is required (see Merchants.AcceptanceTokens)")
	}
	if params.Currency == "" {
		params.Currency = CurrencyCOP
	}

	if params.Signature == "" && s.client.integritySecret != "" {
		params.Signature = IntegritySignature(SignatureParams{
			Reference:      params.Reference,
			AmountInCents:  params.AmountInCents,
			Currency:       params.Currency,
			ExpirationTime: params.ExpirationTime,
		}, s.client.integritySecret)
	}
	if params.Signature == "" {
		return nil, newError(ErrTypeMissingCredential,
			"signature is required: configure WithIntegritySecret or set CreateTransactionParams.Signature")
	}

	body := createTransactionBody{
		AmountInCents:       params.AmountInCents,
		Currency:            params.Currency,
		CustomerEmail:       params.CustomerEmail,
		Reference:           params.Reference,
		AcceptanceToken:     params.AcceptanceToken,
		AcceptPersonalAuth:  params.AcceptPersonalAuth,
		Signature:           params.Signature,
		PaymentMethod:       params.PaymentMethod,
		PaymentMethodType:   params.PaymentMethodType,
		PaymentSourceID:     params.PaymentSourceID,
		Recurrent:           params.Recurrent,
		RedirectURL:         params.RedirectURL,
		CustomerData:        params.CustomerData,
		ShippingAddress:     params.ShippingAddress,
		Taxes:               params.Taxes,
		TipInCents:          params.TipInCents,
		IP:                  params.IP,
		ParentTransactionID: params.ParentTransactionID,
	}
	if !params.ExpirationTime.IsZero() {
		body.ExpirationTime = FormatTime(params.ExpirationTime)
	}
	if body.PaymentMethodType == "" && params.PaymentMethod != nil && requiresTopLevelType(params.PaymentMethod.Type) {
		body.PaymentMethodType = params.PaymentMethod.Type
	}

	auth := authPrivateOrPublic
	if params.PaymentSourceID != 0 {
		auth = authPrivate
	}

	var env envelope[Transaction]
	if err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/transactions", auth: auth, body: body, out: &env,
	}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Get returns a transaction via GET /transactions/{id}. The private key is
// used when configured, otherwise the public key.
func (s *TransactionService) Get(ctx context.Context, id string) (*Transaction, error) {
	if id == "" {
		return nil, invalidParams("transaction id is required")
	}
	var env envelope[Transaction]
	if err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/transactions/" + pathEscape(id), auth: authPrivateOrPublic, out: &env,
	}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// SortOrder is the sort direction of a search.
type SortOrder string

// Sort orders.
const (
	OrderAsc  SortOrder = "ASC"
	OrderDesc SortOrder = "DESC"
)

// ListTransactionsParams filters a transaction search. Either Reference or
// both FromDate and UntilDate are required.
type ListTransactionsParams struct {
	Reference         string
	FromDate          time.Time // inclusive, date part only
	UntilDate         time.Time // inclusive, date part only
	Page              int
	PageSize          int
	ID                string
	PaymentMethodType PaymentMethodType
	Status            TransactionStatus
	CustomerEmail     string
	OrderBy           string // e.g. "created_at"
	Order             SortOrder
}

// TransactionList is a page of transactions.
type TransactionList struct {
	Data []Transaction
	// Meta holds pagination metadata as returned by Wompi.
	Meta map[string]any
}

// List searches transactions via GET /transactions. Requires the private
// key.
func (s *TransactionService) List(ctx context.Context, params ListTransactionsParams) (*TransactionList, error) {
	if params.Reference == "" && (params.FromDate.IsZero() || params.UntilDate.IsZero()) {
		return nil, invalidParams("reference, or both from_date and until_date, are required")
	}
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("reference", params.Reference)
	if !params.FromDate.IsZero() {
		q.Set("from_date", params.FromDate.Format(time.DateOnly))
	}
	if !params.UntilDate.IsZero() {
		q.Set("until_date", params.UntilDate.Format(time.DateOnly))
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(params.PageSize))
	}
	set("id", params.ID)
	set("payment_method_type", string(params.PaymentMethodType))
	set("status", string(params.Status))
	set("customer_email", params.CustomerEmail)
	set("order_by", params.OrderBy)
	set("order", string(params.Order))

	var env envelope[[]Transaction]
	if err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/transactions", auth: authPrivate, query: q, out: &env,
	}); err != nil {
		return nil, err
	}
	return &TransactionList{Data: env.Data, Meta: env.Meta}, nil
}

// Void voids an APPROVED card transaction via POST /transactions/{id}/void.
// amountInCents may be zero to void the full amount. Call Get afterwards to
// observe the VOIDED status.
func (s *TransactionService) Void(ctx context.Context, id string, amountInCents int64) error {
	if id == "" {
		return invalidParams("transaction id is required")
	}
	body := struct {
		AmountInCents int64 `json:"amount_in_cents,omitempty"`
	}{amountInCents}
	return s.client.do(ctx, request{
		method: http.MethodPost, path: "/transactions/" + pathEscape(id) + "/void", auth: authPrivate, body: body,
	})
}

// DefaultPollInterval is the interval used by Poll when none is given.
const DefaultPollInterval = 2 * time.Second

// Poll fetches the transaction every interval until done returns true or ctx
// ends. It returns the last transaction observed; on context expiry the
// error wraps ctx.Err().
//
// Prefer events (webhooks) as the source of truth in production; polling is
// useful for scripts, back-office tools and to wait for an async payment URL.
func (s *TransactionService) Poll(ctx context.Context, id string, interval time.Duration, done func(*Transaction) bool) (*Transaction, error) {
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var last *Transaction
	for {
		tx, err := s.Get(ctx, id)
		switch {
		case err == nil:
			last = tx
			if done(tx) {
				return tx, nil
			}
		case errors.Is(err, ErrNotFound):
			// Freshly created transactions can take a moment to be readable.
		case ctx.Err() == nil:
			return last, err
		}

		select {
		case <-ctx.Done():
			return last, &Error{Type: ErrTypeCommunication, Reason: "polling stopped", Err: ctx.Err()}
		case <-ticker.C:
		}
	}
}

// WaitForFinalStatus polls the transaction until its status is final
// (APPROVED, DECLINED, VOIDED or ERROR).
func (s *TransactionService) WaitForFinalStatus(ctx context.Context, id string, interval time.Duration) (*Transaction, error) {
	return s.Poll(ctx, id, interval, func(t *Transaction) bool { return t.Status.IsFinal() })
}

// WaitForAsyncPaymentURL polls the transaction until Wompi produces the URL
// the payer must be redirected to (or the transaction reaches a final
// status first).
func (s *TransactionService) WaitForAsyncPaymentURL(ctx context.Context, id string, interval time.Duration) (*Transaction, error) {
	return s.Poll(ctx, id, interval, func(t *Transaction) bool {
		return t.AsyncPaymentURL() != "" || t.Status.IsFinal()
	})
}
