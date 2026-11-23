package wompi

import (
	"encoding/json"
	"testing"
)

func marshalMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPaymentMethodConstructors(t *testing.T) {
	cases := []struct {
		name string
		pm   *PaymentMethod
		want map[string]any
	}{
		{"card", CardPayment("tok", 3), map[string]any{"type": "CARD", "token": "tok", "installments": float64(3)}},
		{"nequi", NequiPayment("3991111111"), map[string]any{"type": "NEQUI", "phone_number": "3991111111"}},
		{"pse natural person keeps user_type 0", PSEPayment(PSEPaymentParams{
			UserType: PSEUserNatural, UserLegalIDType: LegalIDCC, UserLegalID: "123",
			FinancialInstitutionCode: "1", PaymentDescription: "Pedido",
		}), map[string]any{
			"type": "PSE", "user_type": float64(0), "user_legal_id_type": "CC", "user_legal_id": "123",
			"financial_institution_code": "1", "payment_description": "Pedido",
		}},
		{"bancolombia transfer", BancolombiaTransferPayment("Pedido", "https://shop.example"), map[string]any{
			"type": "BANCOLOMBIA_TRANSFER", "user_type": "PERSON", "payment_description": "Pedido", "ecommerce_url": "https://shop.example",
		}},
		{"bancolombia collect", BancolombiaCollectPayment(), map[string]any{"type": "BANCOLOMBIA_COLLECT"}},
		{"bancolombia qr", BancolombiaQRPayment("Pedido"), map[string]any{"type": "BANCOLOMBIA_QR", "payment_description": "Pedido"}},
		{"daviplata", DaviplataPayment(LegalIDCC, "123", "Pedido"), map[string]any{
			"type": "DAVIPLATA", "user_legal_id_type": "CC", "user_legal_id": "123", "payment_description": "Pedido",
		}},
		{"pcol", PCOLPayment(), map[string]any{"type": "PCOL"}},
		{"su plus", SuPlusPayment(LegalIDCC, "123"), map[string]any{"type": "SU_PLUS", "user_legal_id_type": "CC", "user_legal_id": "123"}},
		{"bnpl", BancolombiaBNPLPayment(BancolombiaBNPLParams{
			Name: "Ana", LastName: "Gómez", UserLegalIDType: LegalIDCC, UserLegalID: "1", PhoneNumber: "300", PhoneCode: "+57",
			RedirectURL: "https://r", PaymentDescription: "d",
		}), map[string]any{
			"type": "BANCOLOMBIA_BNPL", "name": "Ana", "last_name": "Gómez", "user_legal_id_type": "CC", "user_legal_id": "1",
			"phone_number": "300", "phone_code": "+57", "redirect_url": "https://r", "payment_description": "d",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := marshalMap(t, tc.pm)
			if len(got) != len(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("%s = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}
