package wompi

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestMerchantAcceptanceTokens(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/merchants/info", "")
		if got := r.Header.Get("X-Merchant-Public-Key"); got != testPublicKey {
			t.Errorf("X-Merchant-Public-Key = %q", got)
		}
		writeJSON(t, w, 200, map[string]any{"data": map[string]any{
			"id": 1, "name": "Shop", "public_key": testPublicKey,
			"accepted_payment_methods": []string{"CARD", "NEQUI", "PSE"},
			"presigned_acceptance": map[string]any{
				"acceptance_token": "eyJ.policy", "permalink": "https://wompi.example/policy.pdf", "type": "END_USER_POLICY",
			},
			"presigned_personal_data_auth": map[string]any{
				"acceptance_token": "eyJ.personal", "permalink": "https://wompi.example/data.pdf", "type": "PERSONAL_DATA_AUTH",
			},
		}})
	})
	tokens, err := c.Merchants.AcceptanceTokens(context.Background())
	if err != nil {
		t.Fatalf("AcceptanceTokens: %v", err)
	}
	if tokens.EndUserPolicy.AcceptanceToken != "eyJ.policy" || tokens.EndUserPolicy.Type != AcceptanceEndUserPolicy {
		t.Errorf("EndUserPolicy = %+v", tokens.EndUserPolicy)
	}
	if tokens.PersonalDataAuth.AcceptanceToken != "eyJ.personal" || tokens.PersonalDataAuth.Type != AcceptancePersonalDataAuth {
		t.Errorf("PersonalDataAuth = %+v", tokens.PersonalDataAuth)
	}
}

func TestPaymentSources(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		source := map[string]any{
			"id": 3891, "type": "CARD", "token": "", "status": "AVAILABLE", "customer_email": "a@b.co",
			"public_data": map[string]any{"type": "CARD"},
		}
		switch {
		case r.Method == http.MethodPost:
			assertRequest(t, r, http.MethodPost, "/payment_sources", testPrivateKey)
			body := readBody(t, r)
			if body["type"] != "CARD" || body["token"] != "tok_1" || body["acceptance_token"] != "acc" || body["accept_personal_auth"] != "pers" {
				t.Errorf("body = %v", body)
			}
			writeJSON(t, w, 201, map[string]any{"data": source})
		case r.Method == http.MethodGet:
			assertRequest(t, r, http.MethodGet, "/payment_sources/3891", testPrivateKey)
			writeJSON(t, w, 200, map[string]any{"data": source})
		default:
			assertRequest(t, r, http.MethodPut, "/payment_sources/3891/void", testPrivateKey)
			source["status"] = "VOIDED"
			writeJSON(t, w, 200, map[string]any{"data": source})
		}
	})
	ctx := context.Background()
	src, err := c.PaymentSources.Create(ctx, CreatePaymentSourceParams{
		Type: PaymentSourceCard, Token: "tok_1", CustomerEmail: "a@b.co", AcceptanceToken: "acc", AcceptPersonalAuth: "pers",
	})
	if err != nil || src.ID != 3891 || src.Status != PaymentSourceAvailable || src.PublicData["type"] != "CARD" {
		t.Fatalf("Create: %+v, %v", src, err)
	}
	if src, err = c.PaymentSources.Get(ctx, 3891); err != nil || src.ID != 3891 {
		t.Fatalf("Get: %+v, %v", src, err)
	}
	if src, err = c.PaymentSources.Void(ctx, 3891); err != nil || src.Status != PaymentSourceVoided {
		t.Fatalf("Void: %+v, %v", src, err)
	}
	if _, err := c.PaymentSources.Create(ctx, CreatePaymentSourceParams{Type: PaymentSourceCard}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("incomplete params: err = %v", err)
	}
	if _, err := c.PaymentSources.Get(ctx, 0); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("zero id: err = %v", err)
	}
}

func TestPaymentLinks(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		link := map[string]any{"id": "test_AbC123", "active": true, "name": "Camiseta", "amount_in_cents": 5000000, "currency": "COP"}
		switch r.Method {
		case http.MethodPost:
			assertRequest(t, r, http.MethodPost, "/payment_links", testPrivateKey)
			body := readBody(t, r)
			if body["single_use"] != false || body["collect_shipping"] != true || body["currency"] != "COP" {
				t.Errorf("required booleans and default currency must be sent: %v", body)
			}
			writeJSON(t, w, 201, map[string]any{"data": link})
		case http.MethodGet:
			assertRequest(t, r, http.MethodGet, "/payment_links/test_AbC123", "")
			writeJSON(t, w, 200, map[string]any{"data": link})
		case http.MethodPatch:
			assertRequest(t, r, http.MethodPatch, "/payment_links/test_AbC123", testPrivateKey)
			if body := readBody(t, r); body["active"] != false {
				t.Errorf("body = %v", body)
			}
			link["active"] = false
			writeJSON(t, w, 200, map[string]any{"data": link})
		}
	})
	ctx := context.Background()
	link, err := c.PaymentLinks.Create(ctx, CreatePaymentLinkParams{
		Name: "Camiseta", Description: "Talla M", CollectShipping: true, AmountInCents: 5000000,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if link.CheckoutURL() != "https://checkout.wompi.co/l/test_AbC123" {
		t.Errorf("CheckoutURL = %q", link.CheckoutURL())
	}
	if link, err = c.PaymentLinks.Get(ctx, "test_AbC123"); err != nil || !link.Active {
		t.Fatalf("Get: %+v, %v", link, err)
	}
	if link, err = c.PaymentLinks.SetActive(ctx, "test_AbC123", false); err != nil || link.Active {
		t.Fatalf("SetActive: %+v, %v", link, err)
	}
	_, err = c.PaymentLinks.Create(ctx, CreatePaymentLinkParams{
		Name: "x", Description: "y",
		CustomerData: &PaymentLinkCustomerData{CustomerReferences: make([]CustomerReference, 3)},
	})
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("3 references: err = %v", err)
	}
}

// TestPaymentLinkOpenAmountOmitsCurrency covers the live sandbox rule that
// currency and amount_in_cents must be both set or both null.
func TestPaymentLinkOpenAmountOmitsCurrency(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if _, ok := body["currency"]; ok {
			t.Errorf("currency must be omitted for open-amount links: %v", body)
		}
		if _, ok := body["amount_in_cents"]; ok {
			t.Errorf("amount_in_cents must be omitted: %v", body)
		}
		writeJSON(t, w, 201, map[string]any{"data": map[string]any{"id": "l1"}})
	})
	_, err := c.PaymentLinks.Create(context.Background(), CreatePaymentLinkParams{
		Name: "Donación", Description: "Monto libre", Currency: CurrencyCOP,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestRefundCreate(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/refunds", testPrivateKey)
		body := readBody(t, r)
		if body["transaction_id"] != "tx-1" || body["amount_in_cents"] != float64(150000) || body["test_scenario"] != "approved" {
			t.Errorf("body = %v", body)
		}
		writeJSON(t, w, 201, map[string]any{"data": map[string]any{
			"id": 98765, "status": "PENDING", "v2_refund_id": "rf_abc", "amount_in_cents": 150000, "transaction_id": "tx-1",
		}})
	})
	refund, err := c.Refunds.Create(context.Background(), CreateRefundParams{
		TransactionID: "tx-1", AmountInCents: 150000, Reason: "Devolución", TestScenario: RefundScenarioApproved,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if refund.ID != "98765" || refund.V2RefundID != "rf_abc" || refund.Status != RefundPending || refund.AmountInCents != 150000 {
		t.Errorf("refund = %+v", refund)
	}
	if _, err := c.Refunds.Create(context.Background(), CreateRefundParams{TransactionID: "tx"}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("zero amount: err = %v", err)
	}
}

func TestPSEListFinancialInstitutions(t *testing.T) {
	for name, payload := range map[string]string{
		"envelope":   `{"data":[{"financial_institution_code":"1","financial_institution_name":"Banco que aprueba"},{"financial_institution_code":"2","financial_institution_name":"Banco que rechaza"}]}`,
		"bare array": `[{"financial_institution_code":"1","financial_institution_name":"Banco que aprueba"},{"financial_institution_code":"2","financial_institution_name":"Banco que rechaza"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assertRequest(t, r, http.MethodGet, "/pse/financial_institutions", testPublicKey)
				_, _ = w.Write([]byte(payload))
			})
			banks, err := c.PSE.ListFinancialInstitutions(context.Background())
			if err != nil {
				t.Fatalf("ListFinancialInstitutions: %v", err)
			}
			if len(banks) != 2 || banks[0].Code != "1" || banks[1].Name != "Banco que rechaza" {
				t.Errorf("banks = %+v", banks)
			}
		})
	}
}
