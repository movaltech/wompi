// Command webhook is a runnable events (webhook) receiver for
// github.com/movaltech/wompi. Every request is verified against the events
// secret before it reaches your code.
//
// Run it with:
//
//	WOMPI_PUBLIC_KEY=pub_test_... WOMPI_EVENTS_SECRET=test_events_... \
//	go run ./examples/webhook
//
// then expose it (e.g. with a tunnel) and configure
// https://<your-host>/wompi/events as the events URL in the Wompi dashboard.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/movaltech/wompi"
)

func main() {
	client, err := wompi.New(
		os.Getenv("WOMPI_PUBLIC_KEY"),
		os.Getenv("WOMPI_PRIVATE_KEY"), // optional for this example
		wompi.WithEventsSecret(os.Getenv("WOMPI_EVENTS_SECRET")),
	)
	if err != nil {
		log.Fatalf("wompi.New: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/wompi/events", client.EventHandler(handleEvent))

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("listening for Wompi events on %s/wompi/events (%s)", addr, client.Environment())
	log.Fatal(srv.ListenAndServe())
}

// handleEvent runs only for events whose checksum is valid. Wompi may deliver
// the same event more than once, so the processing must be idempotent.
// Returning an error responds 500 and makes Wompi retry later.
func handleEvent(ctx context.Context, e *wompi.Event) error {
	switch e.Event {
	case wompi.EventTransactionUpdated:
		tx := e.Data.Transaction
		log.Printf("transaction %s reference=%s status=%s amount=%d",
			tx.ID, tx.Reference, tx.Status, tx.AmountInCents)
		// e.g. orders.MarkPaid(ctx, tx.Reference, tx.ID) when tx.Status == wompi.StatusApproved

	case wompi.EventNequiTokenUpdated:
		log.Printf("nequi token %s is %s", e.Data.NequiToken.ID, e.Data.NequiToken.Status)

	case wompi.EventBancolombiaTransferTokenUpdated:
		tok := e.Data.BancolombiaTransferToken
		log.Printf("bancolombia token %s is %s", tok.ID, tok.Status)

	default:
		log.Printf("ignoring event type %q", e.Event)
	}
	return nil
}
