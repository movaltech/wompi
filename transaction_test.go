package wompi

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

const transactionFixture = `{
  "data": {
    "id": "15113-1700000000-12345",
    "created_at": "2026-11-23T15:04:05.000Z",
    "finalized_at": null,
    "amount_in_cents": 4990000,
    "reference": "order-42",
    "customer_email": "buyer@example.com",
    "currency": "COP",
    "payment_method_type": "CARD",
    "payment_method": {
      "type": "CARD",
      "installments": 1,
      "extra": {"name": "VISA-4242", "brand": "VISA", "last_four": "4242", "processor_response_code": "00"}
    },
    "status": "PENDING",
    "status_message": null,
    "redirect_url": null,
    "payment_source_id": null,
    "payment_link_id": null,
    "shipping_address": null,
    "merchant": {"id": 1, "name": "Shop", "public_key": "pub_test_abc"},
    "taxes": []
  },
  "meta": {}
}`

func TestTransactionCreateCardSignsAutomatically(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/transactions", testPrivateKey)
		body := readBody(t, r)

		wantSig := IntegritySignature(SignatureParams{Reference: "order-42", AmountInCents: 4990000, Currency: CurrencyCOP}, testIntegritySecret)
		checks := map[string]any{
			"amount_in_cents":      float64(4990000),
			"currency":             "COP",
			"customer_email":       "buyer@example.com",
			"reference":            "order-42",
			"acceptance_token":     "acc-tok",
			"accept_personal_auth": "pers-tok",
			"signature":            wantSig,
		}
		for k, want := range checks {
			if body[k] != want {
				t.Errorf("body[%s] = %v, want %v", k, body[k], want)
			}
		}
		pm, _ := body["payment_method"].(map[string]any)
		if pm["type"] != "CARD" || pm["token"] != "tok_test_1" || pm["installments"] != float64(1) {
			t.Errorf("payment_method = %v", pm)
		}
		for _, absent := range []string{"payment_source_id", "expiration_time", "payment_method_type", "recurrent"} {
			if _, ok := body[absent]; ok {
				t.Errorf("%s should be omitted", absent)
			}
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(transactionFixture))
	})

	tx, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents:      4990000,
		CustomerEmail:      "buyer@example.com",
		Reference:          "order-42",
		AcceptanceToken:    "acc-tok",
		AcceptPersonalAuth: "pers-tok",
		PaymentMethod:      CardPayment("tok_test_1", 0),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != "15113-1700000000-12345" || tx.Status != StatusPending || tx.AmountInCents != 4990000 {
		t.Errorf("unexpected transaction: %+v", tx)
	}
	if tx.ExtraString("last_four") != "4242" || tx.Merchant == nil || tx.Merchant.Name != "Shop" {
		t.Errorf("extra/merchant not decoded: %+v", tx.PaymentMethod)
	}
}

func TestTransactionCreateWithExpirationAndDaviplata(t *testing.T) {
	exp := time.Date(2026, 6, 9, 20, 28, 50, 0, time.UTC)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["expiration_time"] != "2026-06-09T20:28:50.000Z" {
			t.Errorf("expiration_time = %v", body["expiration_time"])
		}
		want := IntegritySignature(SignatureParams{Reference: "ref-1", AmountInCents: 100000, ExpirationTime: exp}, testIntegritySecret)
		if body["signature"] != want {
			t.Errorf("signature must include expiration time")
		}
		if body["payment_method_type"] != "DAVIPLATA" {
			t.Errorf("payment_method_type = %v, want DAVIPLATA", body["payment_method_type"])
		}
		_, _ = w.Write([]byte(transactionFixture))
	})
	_, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents: 100000, CustomerEmail: "a@b.co", Reference: "ref-1", AcceptanceToken: "t",
		ExpirationTime: exp,
		PaymentMethod:  DaviplataPayment(LegalIDCC, "123", "Pedido"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestTransactionCreatePaymentSourceRequiresPrivateKey(t *testing.T) {
	c, _ := New(testPublicKey, "", WithIntegritySecret(testIntegritySecret))
	_, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents: 100, CustomerEmail: "a@b.co", Reference: "r", PaymentSourceID: 9,
	})
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("err = %v, want ErrMissingCredential", err)
	}
}

// TestTransactionCreatePaymentSourceWithInstallments reproduces the body the
// live sandbox requires to charge a CARD payment source.
func TestTransactionCreatePaymentSourceWithInstallments(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/transactions", testPrivateKey)
		body := readBody(t, r)
		if body["payment_source_id"] != float64(422001) {
			t.Errorf("payment_source_id = %v", body["payment_source_id"])
		}
		pm, _ := body["payment_method"].(map[string]any)
		if len(pm) != 1 || pm["installments"] != float64(1) {
			t.Errorf("payment_method = %v, want only installments", pm)
		}
		if _, ok := body["acceptance_token"]; ok {
			t.Error("acceptance_token is not needed for source charges")
		}
		_, _ = w.Write([]byte(transactionFixture))
	})
	_, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents: 1200000, CustomerEmail: "a@b.co", Reference: "sub-1",
		PaymentSourceID: 422001, PaymentMethod: SourceInstallments(0),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestTransactionCreateValidation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	})
	base := CreateTransactionParams{
		AmountInCents: 100, CustomerEmail: "a@b.co", Reference: "r", AcceptanceToken: "t",
		PaymentMethod: NequiPayment("3991111111"),
	}
	cases := map[string]func(p *CreateTransactionParams){
		"zero amount":         func(p *CreateTransactionParams) { p.AmountInCents = 0 },
		"no reference":        func(p *CreateTransactionParams) { p.Reference = " " },
		"no email":            func(p *CreateTransactionParams) { p.CustomerEmail = "" },
		"no method":           func(p *CreateTransactionParams) { p.PaymentMethod = nil },
		"method without type": func(p *CreateTransactionParams) { p.PaymentMethod = &PaymentMethod{Installments: 1} },
		"no acceptance token": func(p *CreateTransactionParams) { p.AcceptanceToken = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			if _, err := c.Transactions.Create(context.Background(), p); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("err = %v, want ErrInvalidParams", err)
			}
		})
	}
}

func TestTransactionCreateRequiresSignature(t *testing.T) {
	c, _ := New(testPublicKey, testPrivateKey)
	_, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents: 100, CustomerEmail: "a@b.co", Reference: "r", AcceptanceToken: "t",
		PaymentMethod: NequiPayment("3991111111"),
	})
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("err = %v, want ErrMissingCredential", err)
	}
}

func TestTransactionCreateValidationErrorFromAPI(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 422, map[string]any{"error": map[string]any{
			"type": "INPUT_VALIDATION_ERROR", "messages": map[string][]string{"reference": {"La referencia ya ha sido usada"}},
		}})
	})
	_, err := c.Transactions.Create(context.Background(), CreateTransactionParams{
		AmountInCents: 100, CustomerEmail: "a@b.co", Reference: "r", AcceptanceToken: "t",
		PaymentMethod: NequiPayment("3991111111"),
	})
	var wErr *Error
	if !errors.As(err, &wErr) || wErr.Messages["reference"][0] != "La referencia ya ha sido usada" {
		t.Fatalf("err = %v", err)
	}
}

func TestTransactionGet(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/transactions/15113-1700000000-12345", testPrivateKey)
		_, _ = w.Write([]byte(transactionFixture))
	})
	tx, err := c.Transactions.Get(context.Background(), "15113-1700000000-12345")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tx.Reference != "order-42" {
		t.Errorf("Reference = %q", tx.Reference)
	}
	if _, err := c.Transactions.Get(context.Background(), ""); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("empty id: err = %v", err)
	}
}

func TestTransactionList(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/transactions", testPrivateKey)
		q := r.URL.Query()
		want := map[string]string{
			"from_date": "2026-11-01", "until_date": "2026-11-23", "page": "2", "page_size": "50",
			"status": "APPROVED", "payment_method_type": "PSE", "order_by": "created_at", "order": "DESC",
		}
		for k, v := range want {
			if q.Get(k) != v {
				t.Errorf("query %s = %q, want %q", k, q.Get(k), v)
			}
		}
		if q.Has("reference") {
			t.Error("empty reference should be omitted")
		}
		writeJSON(t, w, 200, map[string]any{
			"data": []map[string]any{{"id": "a", "status": "APPROVED"}, {"id": "b", "status": "APPROVED"}},
			"meta": map[string]any{"page": 2, "page_size": 50, "total_results": 52},
		})
	})
	list, err := c.Transactions.List(context.Background(), ListTransactionsParams{
		FromDate:  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		UntilDate: time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC),
		Page:      2, PageSize: 50, Status: StatusApproved, PaymentMethodType: PaymentMethodPSE,
		OrderBy: "created_at", Order: OrderDesc,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Data) != 2 || list.Meta["total_results"] != float64(52) {
		t.Errorf("unexpected list: %+v", list)
	}
	if _, err := c.Transactions.List(context.Background(), ListTransactionsParams{}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("empty filter: err = %v", err)
	}
}

func TestTransactionVoid(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/transactions/tx-1/void", testPrivateKey)
		if body := readBody(t, r); body["amount_in_cents"] != float64(1000) {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusCreated)
	})
	if err := c.Transactions.Void(context.Background(), "tx-1", 1000); err != nil {
		t.Fatalf("Void: %v", err)
	}
}

func TestTransactionWaitForFinalStatus(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			writeJSON(t, w, 404, map[string]any{"error": map[string]string{"type": "NOT_FOUND_ERROR", "reason": "x"}})
		case 2:
			writeJSON(t, w, 200, map[string]any{"data": map[string]any{"id": "tx", "status": "PENDING"}})
		default:
			writeJSON(t, w, 200, map[string]any{"data": map[string]any{"id": "tx", "status": "APPROVED"}})
		}
	})
	tx, err := c.Transactions.WaitForFinalStatus(context.Background(), "tx", 5*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForFinalStatus: %v", err)
	}
	if tx.Status != StatusApproved || calls.Load() != 3 {
		t.Errorf("status = %s after %d calls", tx.Status, calls.Load())
	}
}

func TestTransactionPollTimeout(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 200, map[string]any{"data": map[string]any{"id": "tx", "status": "PENDING"}})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	tx, err := c.Transactions.WaitForFinalStatus(ctx, "tx", 5*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if tx == nil || tx.Status != StatusPending {
		t.Errorf("expected last observed transaction, got %+v", tx)
	}
}

func TestTransactionWaitForAsyncPaymentURL(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		extra := map[string]any{}
		if calls.Add(1) > 1 {
			extra["async_payment_url"] = "https://bank.example/pay"
		}
		writeJSON(t, w, 200, map[string]any{"data": map[string]any{
			"id": "tx", "status": "PENDING", "payment_method": map[string]any{"type": "PSE", "extra": extra},
		}})
	})
	tx, err := c.Transactions.WaitForAsyncPaymentURL(context.Background(), "tx", 5*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForAsyncPaymentURL: %v", err)
	}
	if tx.AsyncPaymentURL() != "https://bank.example/pay" {
		t.Errorf("AsyncPaymentURL = %q", tx.AsyncPaymentURL())
	}
}

func TestTransactionAsyncPaymentURLFallsBackToURL(t *testing.T) {
	tx := &Transaction{PaymentMethod: TransactionPaymentMethod{Extra: map[string]any{"url": "https://daviplata.example", "n": 3.0}}}
	if tx.AsyncPaymentURL() != "https://daviplata.example" {
		t.Errorf("AsyncPaymentURL = %q", tx.AsyncPaymentURL())
	}
	if tx.ExtraString("n") != "3" || tx.ExtraString("missing") != "" {
		t.Error("ExtraString conversions")
	}
}

func TestTransactionStatusIsFinal(t *testing.T) {
	for _, s := range []TransactionStatus{StatusApproved, StatusDeclined, StatusVoided, StatusError} {
		if !s.IsFinal() {
			t.Errorf("%s should be final", s)
		}
	}
	if StatusPending.IsFinal() {
		t.Error("PENDING should not be final")
	}
}
