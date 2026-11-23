package wompi_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/movaltech/wompi"
)

func ExampleNew() {
	client, err := wompi.New(
		os.Getenv("WOMPI_PUBLIC_KEY"),
		os.Getenv("WOMPI_PRIVATE_KEY"),
		wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")),
		wompi.WithEventsSecret(os.Getenv("WOMPI_EVENTS_SECRET")),
		wompi.WithRetries(2),
		wompi.WithUserAgent("my-shop/1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Environment())
}

func ExampleTransactionService_Create() {
	client, _ := wompi.New(os.Getenv("WOMPI_PUBLIC_KEY"), os.Getenv("WOMPI_PRIVATE_KEY"),
		wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")))
	ctx := context.Background()

	// 1. Acceptance tokens: show the permalinks to the payer and send the
	//    tokens only after explicit acceptance.
	acceptance, err := client.Merchants.AcceptanceTokens(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Charge a card token produced by the Wompi widget / Wompi.js.
	tx, err := client.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:      wompi.AmountInCents(49_900),
		CustomerEmail:      "buyer@example.com",
		Reference:          "order-1001",
		AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
		AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
		PaymentMethod:      wompi.CardPayment("tok_test_...", 1),
	})
	if err != nil {
		var wErr *wompi.Error
		if errors.As(err, &wErr) && errors.Is(err, wompi.ErrInputValidation) {
			log.Fatalf("invalid fields: %v", wErr.Messages)
		}
		log.Fatal(err)
	}
	fmt.Println(tx.ID, tx.Status)
}

func ExampleTransactionService_WaitForAsyncPaymentURL() {
	client, _ := wompi.New(os.Getenv("WOMPI_PUBLIC_KEY"), os.Getenv("WOMPI_PRIVATE_KEY"),
		wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := client.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:   wompi.AmountInCents(120_000),
		CustomerEmail:   "buyer@example.com",
		Reference:       "order-1002",
		AcceptanceToken: "...",
		CustomerData:    &wompi.CustomerData{FullName: "Ana Gómez", PhoneNumber: "573001234567"},
		PaymentMethod: wompi.PSEPayment(wompi.PSEPaymentParams{
			UserType:                 wompi.PSEUserNatural,
			UserLegalIDType:          wompi.LegalIDCC,
			UserLegalID:              "1020304050",
			FinancialInstitutionCode: "1",
			PaymentDescription:       "Pedido 1002",
		}),
	})
	if err != nil {
		log.Fatal(err)
	}

	// PSE returns the bank URL asynchronously.
	tx, err = client.Transactions.WaitForAsyncPaymentURL(ctx, tx.ID, time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("redirect the payer to:", tx.AsyncPaymentURL())
}

func ExampleClient_EventHandler() {
	client, _ := wompi.New(os.Getenv("WOMPI_PUBLIC_KEY"), os.Getenv("WOMPI_PRIVATE_KEY"),
		wompi.WithEventsSecret(os.Getenv("WOMPI_EVENTS_SECRET")))

	http.Handle("/wompi/events", client.EventHandler(func(ctx context.Context, e *wompi.Event) error {
		if e.Event != wompi.EventTransactionUpdated {
			return nil // ignore event types you don't handle
		}
		tx := e.Data.Transaction
		// Update your order idempotently, keyed on tx.Reference / tx.ID.
		log.Printf("transaction %s (%s) is now %s", tx.ID, tx.Reference, tx.Status)
		return nil // a non-nil error makes Wompi retry later
	}))
}

func ExampleClient_WebCheckoutURL() {
	client, _ := wompi.New("pub_test_example", "",
		wompi.WithIntegritySecret("test_integrity_example"))

	checkoutURL, err := client.WebCheckoutURL(wompi.WebCheckoutParams{
		Reference:     "order-1003",
		AmountInCents: wompi.AmountInCents(25_000),
		RedirectURL:   "https://shop.example/checkout/done",
		CustomerEmail: "buyer@example.com",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(checkoutURL != "")
	// Output: true
}

func ExampleIntegritySignature() {
	sig := wompi.IntegritySignature(wompi.SignatureParams{
		Reference:     "sk8-438k4-xmxm392-sn2m",
		AmountInCents: 2490000,
		Currency:      wompi.CurrencyCOP,
	}, "prod_integrity_Z5mMke9x0k8gpErbDqwrJXMqsI6SFli6")
	fmt.Println(sig)
	// Output: 37c8407747e595535433ef8f6a811d853cd943046624a0ec04662b17bbf33bf5
}
