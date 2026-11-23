//go:build integration

// Live tests against Wompi's sandbox API. Excluded from `go test ./...` by
// the "integration" build tag — run explicitly with:
//
//	WOMPI_PUBLIC_KEY=pub_test_... WOMPI_PRIVATE_KEY=prv_test_... \
//	WOMPI_INTEGRITY_SECRET=test_integrity_... \
//	go test -tags integration -run Integration -v ./...
//
// They use Wompi's documented sandbox data (card 4242 4242 4242 4242 is
// approved, Nequi 3991111111 is approved, PSE institution "1" approves).
package wompi_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/movaltech/wompi"
)

func liveClient(t *testing.T) *wompi.Client {
	t.Helper()
	pub, prv := os.Getenv("WOMPI_PUBLIC_KEY"), os.Getenv("WOMPI_PRIVATE_KEY")
	secret := os.Getenv("WOMPI_INTEGRITY_SECRET")
	if pub == "" || prv == "" || secret == "" {
		t.Skip("WOMPI_PUBLIC_KEY / WOMPI_PRIVATE_KEY / WOMPI_INTEGRITY_SECRET not set")
	}
	c, err := wompi.New(pub, prv, wompi.WithIntegritySecret(secret), wompi.WithRetries(2))
	if err != nil {
		t.Fatalf("wompi.New: %v", err)
	}
	if c.Environment() != wompi.Sandbox {
		t.Fatal("integration tests must run against sandbox keys")
	}
	return c
}

func uniqueReference(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestIntegrationCardPaymentApproved(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	acceptance, err := c.Merchants.AcceptanceTokens(ctx)
	if err != nil {
		t.Fatalf("AcceptanceTokens: %v", err)
	}

	card, err := c.Tokens.CreateCard(ctx, wompi.CardParams{
		Number: "4242424242424242", CVC: "123", ExpMonth: "12", ExpYear: "29", CardHolder: "Pedro Pérez",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	tx, err := c.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:      wompi.AmountInCents(15_000),
		CustomerEmail:      "sandbox@example.com",
		Reference:          uniqueReference("it-card"),
		AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
		AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
		PaymentMethod:      wompi.CardPayment(card.ID, 1),
	})
	if err != nil {
		t.Fatalf("Transactions.Create: %v", err)
	}

	tx, err = c.Transactions.WaitForFinalStatus(ctx, tx.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForFinalStatus: %v", err)
	}
	if tx.Status != wompi.StatusApproved {
		t.Errorf("status = %s (%s), want APPROVED", tx.Status, tx.StatusMessage)
	}
}

// TestIntegrationPaymentMethods creates one sandbox transaction per payment
// method the merchant has enabled, and checks that Wompi accepts the payload
// built by each constructor and returns what the payer needs next (a final
// status, or an async payment URL / method-specific extra data).
func TestIntegrationPaymentMethods(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	merchant, err := c.Merchants.Get(ctx)
	if err != nil {
		t.Fatalf("Merchants.Get: %v", err)
	}
	enabled := map[string]bool{}
	for _, m := range merchant.AcceptedPaymentMethods {
		enabled[m] = true
	}
	t.Logf("merchant accepts: %v", merchant.AcceptedPaymentMethods)

	customer := &wompi.CustomerData{FullName: "Pedro Pérez", PhoneNumber: "573001234567", LegalID: "1020304050", LegalIDType: wompi.LegalIDCC}

	cases := []struct {
		name   string
		method *wompi.PaymentMethod
		amount int64
		// done reports whether the transaction reached the state the payer
		// needs next for this method.
		done func(*wompi.Transaction) bool
	}{
		{"NEQUI approved", wompi.NequiPayment("3991111111"), 15_000, func(tx *wompi.Transaction) bool { return tx.Status.IsFinal() }},
		{"PSE", wompi.PSEPayment(wompi.PSEPaymentParams{
			UserType: wompi.PSEUserNatural, UserLegalIDType: wompi.LegalIDCC, UserLegalID: "1020304050",
			FinancialInstitutionCode: "1", PaymentDescription: "Prueba SDK",
		}), 15_000, hasAsyncURL},
		{"BANCOLOMBIA_TRANSFER", wompi.BancolombiaTransferPayment("Prueba SDK", ""), 15_000, hasAsyncURL},
		{"BANCOLOMBIA_QR", withSandboxStatus(wompi.BancolombiaQRPayment("Prueba SDK"), "APPROVED"), 15_000, func(tx *wompi.Transaction) bool {
			return tx.ExtraString("qr_image") != "" || tx.Status.IsFinal()
		}},
		{"BANCOLOMBIA_COLLECT", withSandboxStatus(wompi.BancolombiaCollectPayment(), "APPROVED"), 15_000, func(tx *wompi.Transaction) bool {
			return tx.ExtraString("business_agreement_code") != "" || tx.Status.IsFinal()
		}},
		{"DAVIPLATA", wompi.DaviplataPayment(wompi.LegalIDCC, "1020304050", "Prueba SDK"), 15_000, hasAsyncURL},
		{"PCOL", withSandboxStatus(wompi.PCOLPayment(), "APPROVED_ONLY_POINTS"), 15_000, hasAsyncURL},
		{"SU_PLUS", wompi.SuPlusPayment(wompi.LegalIDCC, "1020304050"), 50_000, hasAsyncURL},
		{"BANCOLOMBIA_BNPL", wompi.BancolombiaBNPLPayment(wompi.BancolombiaBNPLParams{
			Name: "Pedro", LastName: "Pérez", UserLegalIDType: wompi.LegalIDCC, UserLegalID: "1020304050",
			PhoneNumber: "3001234567", PhoneCode: "+57", RedirectURL: "https://example.com/done", PaymentDescription: "Prueba SDK",
		}), 150_000, hasAsyncURL},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !enabled[string(tc.method.Type)] {
				t.Skipf("%s is not enabled for this merchant", tc.method.Type)
			}
			acceptance, err := c.Merchants.AcceptanceTokens(ctx)
			if err != nil {
				t.Fatalf("AcceptanceTokens: %v", err)
			}
			tx, err := c.Transactions.Create(ctx, wompi.CreateTransactionParams{
				AmountInCents:      wompi.AmountInCents(tc.amount),
				CustomerEmail:      "sandbox@example.com",
				Reference:          uniqueReference("it-" + string(tc.method.Type)),
				AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
				AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
				RedirectURL:        "https://example.com/done",
				CustomerData:       customer,
				PaymentMethod:      tc.method,
			})
			if err != nil {
				t.Fatalf("Transactions.Create: %v", err)
			}

			pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			tx, err = c.Transactions.Poll(pollCtx, tx.ID, 2*time.Second, tc.done)
			if err != nil {
				t.Fatalf("Poll: %v (last: %+v)", err, tx)
			}
			if tx.Status == wompi.StatusError {
				t.Errorf("status ERROR: %s", tx.StatusMessage)
			}
			t.Logf("id=%s status=%s async_url=%t extra_keys=%v",
				tx.ID, tx.Status, tx.AsyncPaymentURL() != "", extraKeys(tx))
		})
	}
}

func hasAsyncURL(tx *wompi.Transaction) bool {
	return tx.AsyncPaymentURL() != "" || tx.Status.IsFinal()
}

func withSandboxStatus(pm *wompi.PaymentMethod, status string) *wompi.PaymentMethod {
	pm.SandboxStatus = status
	return pm
}

func extraKeys(tx *wompi.Transaction) []string {
	keys := make([]string, 0, len(tx.PaymentMethod.Extra))
	for k := range tx.PaymentMethod.Extra {
		keys = append(keys, k)
	}
	return keys
}

func TestIntegrationPSEFinancialInstitutions(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	banks, err := c.PSE.ListFinancialInstitutions(ctx)
	if err != nil {
		t.Fatalf("ListFinancialInstitutions: %v", err)
	}
	if len(banks) == 0 {
		t.Fatal("expected at least one financial institution")
	}
	t.Logf("%d institutions, first: %+v", len(banks), banks[0])
}

func TestIntegrationPaymentLinkLifecycle(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	link, err := c.PaymentLinks.Create(ctx, wompi.CreatePaymentLinkParams{
		Name: "SDK integration test", Description: "Created by wompi-go", SingleUse: true,
		AmountInCents: wompi.AmountInCents(10_000),
	})
	if err != nil {
		t.Fatalf("PaymentLinks.Create: %v", err)
	}
	t.Logf("checkout URL: %s", link.CheckoutURL())

	if link, err = c.PaymentLinks.SetActive(ctx, link.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if link.Active {
		t.Error("link should be inactive")
	}
}
