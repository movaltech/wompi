// Command payment-link creates a hosted Wompi payment link and a signed Web
// Checkout URL — the two integrations that need no front-end work at all.
//
// Run it with:
//
//	WOMPI_PUBLIC_KEY=pub_test_... WOMPI_PRIVATE_KEY=prv_test_... \
//	WOMPI_INTEGRITY_SECRET=test_integrity_... go run ./examples/payment-link
package main

import (
	"context"
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
	)
	if err != nil {
		log.Fatalf("wompi.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// A reusable payment link with a fixed amount and 19% VAT included.
	link, err := client.PaymentLinks.Create(ctx, wompi.CreatePaymentLinkParams{
		Name:          "Camiseta oficial",
		Description:   "Camiseta talla M",
		SingleUse:     false,
		AmountInCents: wompi.AmountInCents(89_900),
		RedirectURL:   "https://shop.example/gracias",
		ExpiresAt:     wompi.FormatTime(time.Now().Add(7 * 24 * time.Hour)),
		Taxes:         []wompi.Tax{{Type: wompi.TaxVAT, AmountInCents: wompi.AmountInCents(14_353)}},
	})
	if err != nil {
		log.Fatalf("create payment link: %v", err)
	}
	fmt.Println("payment link:", link.CheckoutURL())

	// A signed, single-order Web Checkout URL that expires in 30 minutes.
	checkoutURL, err := client.WebCheckoutURL(wompi.WebCheckoutParams{
		Reference:      fmt.Sprintf("order-%d", time.Now().Unix()),
		AmountInCents:  wompi.AmountInCents(89_900),
		RedirectURL:    "https://shop.example/checkout/resultado",
		ExpirationTime: time.Now().Add(30 * time.Minute),
		CustomerEmail:  "buyer@example.com",
	})
	if err != nil {
		log.Fatalf("web checkout url: %v", err)
	}
	fmt.Println("web checkout:", checkoutURL)
}
