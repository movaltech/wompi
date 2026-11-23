// Command card-payment is a complete, runnable example of a server-side card
// payment with github.com/movaltech/wompi: fetch acceptance tokens, tokenize
// a sandbox card, create a signed transaction and wait for its final status.
//
// Run it against Wompi's sandbox with:
//
//	WOMPI_PUBLIC_KEY=pub_test_... WOMPI_PRIVATE_KEY=prv_test_... \
//	WOMPI_INTEGRITY_SECRET=test_integrity_... go run ./examples/card-payment
//
// In production, tokenize cards in the browser (Wompi widget / Wompi.js) and
// send only the token to your server; tokenizing raw card data server-side
// places your system in PCI DSS scope.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/movaltech/wompi"
)

func main() {
	client, err := wompi.New(
		os.Getenv("WOMPI_PUBLIC_KEY"),
		os.Getenv("WOMPI_PRIVATE_KEY"),
		wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")),
		wompi.WithRetries(2),
	)
	if err != nil {
		log.Fatalf("wompi.New: %v", err)
	}
	fmt.Println("environment:", client.Environment())

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 1. Acceptance tokens. In a real checkout, show both permalinks to the
	//    payer and only proceed after explicit acceptance.
	acceptance, err := client.Merchants.AcceptanceTokens(ctx)
	if err != nil {
		log.Fatalf("acceptance tokens: %v", err)
	}
	fmt.Println("terms:", acceptance.EndUserPolicy.Permalink)
	fmt.Println("personal data:", acceptance.PersonalDataAuth.Permalink)

	// 2. Tokenize Wompi's sandbox approval card.
	card, err := client.Tokens.CreateCard(ctx, wompi.CardParams{
		Number:     "4242424242424242",
		CVC:        "123",
		ExpMonth:   "12",
		ExpYear:    "29",
		CardHolder: "Pedro Pérez",
	})
	if err != nil {
		log.Fatalf("tokenize card: %v", err)
	}
	fmt.Printf("card token: %s (%s ****%s)\n", card.ID, card.Brand, card.LastFour)

	// 3. Create the transaction. The integrity signature is computed
	//    automatically from WithIntegritySecret.
	tx, err := client.Transactions.Create(ctx, wompi.CreateTransactionParams{
		AmountInCents:      wompi.AmountInCents(49_900),
		CustomerEmail:      "pedro.perez@example.com",
		Reference:          fmt.Sprintf("order-%d", time.Now().Unix()),
		AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
		AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
		PaymentMethod:      wompi.CardPayment(card.ID, 1),
		CustomerData: &wompi.CustomerData{
			FullName:    "Pedro Pérez",
			PhoneNumber: "573001234567",
		},
	})
	if err != nil {
		var wErr *wompi.Error
		if errors.As(err, &wErr) && len(wErr.Messages) > 0 {
			log.Fatalf("validation failed: %v", wErr.Messages)
		}
		log.Fatalf("create transaction: %v", err)
	}
	fmt.Printf("transaction %s created: %s\n", tx.ID, tx.Status)

	// 4. Wait for the final status. In production, rely on the
	//    transaction.updated event (see examples/webhook) instead.
	tx, err = client.Transactions.WaitForFinalStatus(ctx, tx.ID, 2*time.Second)
	if err != nil {
		log.Fatalf("wait for final status: %v", err)
	}
	fmt.Printf("transaction %s finished: %s %s\n", tx.ID, tx.Status, tx.StatusMessage)
}
