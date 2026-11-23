//go:build integration

package wompi_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/movaltech/wompi"
)

// sandboxCard tokenizes Wompi's sandbox approval card. Wompi limits how many
// times a card token can be used, so each caller gets a fresh one.
func sandboxCard(ctx context.Context, t *testing.T, c *wompi.Client) *wompi.CardToken {
	t.Helper()
	card, err := c.Tokens.CreateCard(ctx, wompi.CardParams{
		Number: "4242424242424242", CVC: "123", ExpMonth: "12", ExpYear: "29", CardHolder: "Pedro Pérez",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	return card
}

// approvedCardTransaction creates a card transaction and waits until it is
// APPROVED.
func approvedCardTransaction(ctx context.Context, t *testing.T, c *wompi.Client, pesos int64) *wompi.Transaction {
	t.Helper()
	acceptance, err := c.Merchants.AcceptanceTokens(ctx)
	if err != nil {
		t.Fatalf("AcceptanceTokens: %v", err)
	}
	tx, err := c.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:      wompi.AmountInCents(pesos),
		CustomerEmail:      "sandbox@example.com",
		Reference:          uniqueReference("it-card"),
		AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
		AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
		PaymentMethod:      wompi.CardPayment(sandboxCard(ctx, t, c).ID, 1),
	})
	if err != nil {
		t.Fatalf("Transactions.Create: %v", err)
	}
	tx, err = c.Transactions.WaitForFinalStatus(ctx, tx.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForFinalStatus: %v", err)
	}
	if tx.Status != wompi.StatusApproved {
		t.Fatalf("status = %s (%s), want APPROVED", tx.Status, tx.StatusMessage)
	}
	return tx
}

// TestIntegrationRecurringCardCharge stores a card as a payment source,
// charges it without card data, and voids the source.
func TestIntegrationRecurringCardCharge(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	acceptance, err := c.Merchants.AcceptanceTokens(ctx)
	if err != nil {
		t.Fatalf("AcceptanceTokens: %v", err)
	}
	source, err := c.PaymentSources.Create(ctx, wompi.CreatePaymentSourceParams{
		Type:               wompi.PaymentSourceCard,
		Token:              sandboxCard(ctx, t, c).ID,
		CustomerEmail:      "sandbox@example.com",
		AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
		AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
	})
	if err != nil {
		t.Fatalf("PaymentSources.Create: %v", err)
	}
	t.Logf("source: id=%d status=%s public_data=%v", source.ID, source.Status, source.PublicData)

	got, err := c.PaymentSources.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("PaymentSources.Get: %v", err)
	}
	if got.ID != source.ID || got.Status != wompi.PaymentSourceAvailable {
		t.Errorf("Get = %+v", got)
	}

	tx, err := c.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:   wompi.AmountInCents(12_000),
		CustomerEmail:   "sandbox@example.com",
		Reference:       uniqueReference("it-recurring"),
		PaymentSourceID: source.ID,
		PaymentMethod:   wompi.SourceInstallments(1),
	})
	if err != nil {
		t.Fatalf("charge payment source: %v", err)
	}
	tx, err = c.Transactions.WaitForFinalStatus(ctx, tx.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForFinalStatus: %v", err)
	}
	t.Logf("recurring charge: id=%s status=%s source=%d", tx.ID, tx.Status, tx.PaymentSourceID)
	if tx.Status != wompi.StatusApproved || tx.PaymentSourceID != source.ID {
		t.Errorf("recurring charge = %s (%s), source %d", tx.Status, tx.StatusMessage, tx.PaymentSourceID)
	}

	// Wompi only voids sources whose financial operation is PREAUTHORIZATION;
	// for a regular card source it answers 422 UNPROCESSABLE.
	voided, err := c.PaymentSources.Void(ctx, source.ID)
	switch {
	case errors.Is(err, wompi.ErrUnprocessable):
		t.Logf("void rejected by business rule (expected for this source): %v", err)
	case err != nil:
		t.Fatalf("PaymentSources.Void: %v", err)
	default:
		t.Logf("voided source status=%s", voided.Status)
	}
}

// TestIntegrationNequiToken starts a Nequi subscription with the sandbox
// approval number and polls it until it is final.
func TestIntegrationNequiToken(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tok, err := c.Tokens.CreateNequi(ctx, "3991111111")
	if errors.Is(err, wompi.ErrNotFound) {
		// Nequi tokenization needs a payment identity configured by Wompi
		// for the merchant ("No hay una identidad de pago para NEQUI...").
		t.Skipf("Nequi tokenization not enabled for this merchant: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateNequi: %v", err)
	}
	t.Logf("created: id=%s status=%s", tok.ID, tok.Status)

	for tok.Status == wompi.TokenPending {
		select {
		case <-ctx.Done():
			t.Fatalf("token still PENDING: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
		if tok, err = c.Tokens.GetNequi(ctx, tok.ID); err != nil {
			t.Fatalf("GetNequi: %v", err)
		}
	}
	t.Logf("final: id=%s status=%s phone=%s", tok.ID, tok.Status, tok.PhoneNumber)
	if tok.Status != wompi.TokenApproved {
		t.Errorf("status = %s, want APPROVED", tok.Status)
	}
}

// TestIntegrationListTransactions searches by reference and by date range.
func TestIntegrationListTransactions(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tx := approvedCardTransaction(ctx, t, c, 11_000)

	byRef, err := c.Transactions.List(ctx, wompi.ListTransactionsParams{Reference: tx.Reference})
	if err != nil {
		t.Fatalf("List by reference: %v", err)
	}
	if len(byRef.Data) != 1 || byRef.Data[0].ID != tx.ID {
		t.Errorf("List by reference returned %d transactions", len(byRef.Data))
	}

	now := time.Now()
	byDate, err := c.Transactions.List(ctx, wompi.ListTransactionsParams{
		FromDate: now.AddDate(0, 0, -1), UntilDate: now.AddDate(0, 0, 1),
		Page: 1, PageSize: 5, Status: wompi.StatusApproved, Order: wompi.OrderDesc, OrderBy: "created_at",
	})
	if err != nil {
		t.Fatalf("List by date: %v", err)
	}
	t.Logf("by date: %d results, meta=%v", len(byDate.Data), byDate.Meta)
	if len(byDate.Data) == 0 || len(byDate.Data) > 5 {
		t.Errorf("by date returned %d results, want 1..5", len(byDate.Data))
	}
}

// TestIntegrationVoidCardTransaction voids an approved card transaction.
func TestIntegrationVoidCardTransaction(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tx := approvedCardTransaction(ctx, t, c, 13_000)
	if err := c.Transactions.Void(ctx, tx.ID, 0); err != nil {
		t.Fatalf("Void: %v", err)
	}
	tx, err := c.Transactions.Poll(ctx, tx.ID, 2*time.Second, func(tx *wompi.Transaction) bool {
		return tx.Status == wompi.StatusVoided
	})
	if err != nil {
		t.Fatalf("waiting for VOIDED: %v (last status %v)", err, tx)
	}
	t.Logf("voided: id=%s status=%s", tx.ID, tx.Status)
}

// TestIntegrationRefund refunds an approved card transaction using the
// sandbox "approved" scenario. Skipped if refunds are not enabled.
func TestIntegrationRefund(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tx := approvedCardTransaction(ctx, t, c, 14_000)
	refund, err := c.Refunds.Create(ctx, wompi.CreateRefundParams{
		TransactionID: tx.ID,
		AmountInCents: tx.AmountInCents,
		Reason:        "Prueba SDK",
		Reference:     uniqueReference("it-refund"),
		TestScenario:  wompi.RefundScenarioApproved,
	})
	var wErr *wompi.Error
	if errors.As(err, &wErr) && (wErr.HTTPStatus == 401 || wErr.HTTPStatus == 403 || wErr.HTTPStatus == 404) {
		t.Skipf("refunds not available for this merchant: %v", err)
	}
	if err != nil {
		t.Fatalf("Refunds.Create: %v", err)
	}
	t.Logf("refund: %+v", refund)
	if refund.TransactionID != tx.ID && refund.ID == "" {
		t.Errorf("unexpected refund: %+v", refund)
	}
}

// TestIntegrationPaymentLinkGetIsPublic reads a link with no credentials.
func TestIntegrationPaymentLinkGetIsPublic(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	link, err := c.PaymentLinks.Create(ctx, wompi.CreatePaymentLinkParams{
		Name: "SDK public get", Description: "Created by wompi-go", SingleUse: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	public, err := wompi.New(os.Getenv("WOMPI_PUBLIC_KEY"), "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := public.PaymentLinks.Get(ctx, link.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != link.ID || got.Name != "SDK public get" || got.AmountInCents != 0 {
		t.Errorf("Get = %+v", got)
	}
	if _, err := c.PaymentLinks.SetActive(ctx, link.ID, false); err != nil {
		t.Errorf("deactivate: %v", err)
	}
}

// TestIntegrationSignatureWithExpiration checks that Wompi accepts the
// integrity signature variant that includes expiration_time (the same one
// used by WebCheckoutURL) and rejects a tampered signature.
func TestIntegrationSignatureWithExpiration(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Acceptance tokens are single-use, so each attempt gets a fresh one.
	params := func(signature string) wompi.CreateTransactionParams {
		acceptance, err := c.Merchants.AcceptanceTokens(ctx)
		if err != nil {
			t.Fatalf("AcceptanceTokens: %v", err)
		}
		return wompi.CreateTransactionParams{
			AmountInCents:      wompi.AmountInCents(16_000),
			CustomerEmail:      "sandbox@example.com",
			Reference:          uniqueReference("it-exp"),
			AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
			AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
			ExpirationTime:     time.Now().Add(30 * time.Minute),
			Signature:          signature,
			PaymentMethod:      wompi.CardPayment(sandboxCard(ctx, t, c).ID, 1),
		}
	}

	tx, err := c.Transactions.Create(ctx, params(""))
	if err != nil {
		t.Fatalf("signed with expiration: %v", err)
	}
	t.Logf("accepted: id=%s status=%s", tx.ID, tx.Status)

	_, err = c.Transactions.Create(ctx, params("0000000000000000000000000000000000000000000000000000000000000000"))
	t.Logf("tampered signature: %v", err)
	var wErr *wompi.Error
	if !errors.As(err, &wErr) || len(wErr.Messages["signature"]) == 0 {
		t.Errorf("expected a signature validation error, got %v", err)
	}
}

// TestIntegrationInvalidKey checks how an invalid private key is reported.
func TestIntegrationInvalidKey(t *testing.T) {
	liveClient(t) // skip without credentials
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bad, err := wompi.New(os.Getenv("WOMPI_PUBLIC_KEY"), "prv_test_invalid000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	_, err = bad.Transactions.List(ctx, wompi.ListTransactionsParams{Reference: "does-not-exist"})
	t.Logf("invalid key error: %v", err)
	if !errors.Is(err, wompi.ErrInvalidAccessToken) {
		t.Errorf("err = %v, want ErrInvalidAccessToken", err)
	}
}
