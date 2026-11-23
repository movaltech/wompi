package wompi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// RefundService refunds approved transactions (Wompi Refunds V2). Refunds
// must be enabled for the merchant account and require the private key.
type RefundService struct {
	client *Client
}

// RefundStatus is the status of a refund.
type RefundStatus string

// Refund statuses.
const (
	RefundPending   RefundStatus = "PENDING"
	RefundApproved  RefundStatus = "APPROVED"
	RefundDeclined  RefundStatus = "DECLINED"
	RefundError     RefundStatus = "ERROR"
	RefundCancelled RefundStatus = "CANCELLED"
)

// Sandbox refund scenarios (CreateRefundParams.TestScenario).
const (
	RefundScenarioApproved  = "approved"
	RefundScenarioDeclined  = "declined"
	RefundScenarioError     = "error"
	RefundScenarioCancelled = "cancelled"
)

// CreateRefundParams creates a refund.
type CreateRefundParams struct {
	TransactionID string `json:"transaction_id"`
	AmountInCents int64  `json:"amount_in_cents"`
	Reason        string `json:"reason,omitempty"`
	// Reference .. Reference5 are free references for your reconciliation.
	Reference  string `json:"reference,omitempty"`
	Reference2 string `json:"reference_2,omitempty"`
	Reference3 string `json:"reference_3,omitempty"`
	Reference4 string `json:"reference_4,omitempty"`
	Reference5 string `json:"reference_5,omitempty"`
	// TestScenario forces the outcome in sandbox (RefundScenario*).
	TestScenario string `json:"test_scenario,omitempty"`
}

// Refund is a refund of a transaction.
type Refund struct {
	ID            string       `json:"id"`
	Status        RefundStatus `json:"status"`
	StatusMessage string       `json:"status_message"`
	V2RefundID    string       `json:"v2_refund_id"`
	AmountInCents int64        `json:"amount_in_cents"`
	TransactionID string       `json:"transaction_id"`
	Reference     string       `json:"reference"`
	Reference2    string       `json:"reference_2"`
	Reference3    string       `json:"reference_3"`
	Reference4    string       `json:"reference_4"`
	Reference5    string       `json:"reference_5"`
	CreatedAt     string       `json:"created_at"`
}

// UnmarshalJSON accepts identifiers encoded either as JSON strings or
// numbers.
func (r *Refund) UnmarshalJSON(data []byte) error {
	type alias Refund
	aux := struct {
		*alias
		ID         json.RawMessage `json:"id"`
		V2RefundID json.RawMessage `json:"v2_refund_id"`
	}{alias: (*alias)(r)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.ID = flexString(aux.ID)
	r.V2RefundID = flexString(aux.V2RefundID)
	return nil
}

// flexString decodes a JSON string or number into a string.
func flexString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// Create refunds an approved transaction via POST /refunds.
func (s *RefundService) Create(ctx context.Context, params CreateRefundParams) (*Refund, error) {
	if params.TransactionID == "" || params.AmountInCents <= 0 {
		return nil, invalidParams("transaction_id and a positive amount_in_cents are required")
	}
	var env envelope[Refund]
	if err := s.client.do(ctx, request{
		method: http.MethodPost, path: "/refunds", auth: authPrivate, body: params, out: &env,
	}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
