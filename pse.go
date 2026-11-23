package wompi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// PSEService exposes PSE-specific resources.
type PSEService struct {
	client *Client
}

// FinancialInstitution is a bank available for PSE payments.
type FinancialInstitution struct {
	Code string `json:"financial_institution_code"`
	Name string `json:"financial_institution_name"`
}

// ListFinancialInstitutions returns the banks available for PSE via
// GET /pse/financial_institutions. Show them to the payer and send the
// chosen Code in PSEPaymentParams.FinancialInstitutionCode.
func (s *PSEService) ListFinancialInstitutions(ctx context.Context) ([]FinancialInstitution, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, request{
		method: http.MethodGet, path: "/pse/financial_institutions", auth: authPublic, out: &raw,
	}); err != nil {
		return nil, err
	}

	// Wompi documents both a {"data": [...]} envelope and a bare array.
	var list []FinancialInstitution
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, &Error{Type: ErrTypeDecode, Reason: "failed to decode financial institutions", Err: err}
		}
		return list, nil
	}
	var env envelope[[]FinancialInstitution]
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, &Error{Type: ErrTypeDecode, Reason: "failed to decode financial institutions", Err: err}
	}
	return env.Data, nil
}
