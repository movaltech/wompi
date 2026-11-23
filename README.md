# wompi

[![CI](https://github.com/movaltech/wompi/actions/workflows/ci.yml/badge.svg)](https://github.com/movaltech/wompi/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/movaltech/wompi.svg)](https://pkg.go.dev/github.com/movaltech/wompi)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A professional, dependency-free Go SDK for the [Wompi](https://wompi.com) payments API (Colombia).

- **Complete server-side surface**: acceptance tokens, tokenization, payment sources, transactions
  with every payment method, voids, refunds, payment links, PSE banks, Web Checkout and events.
- **Safe by default**: automatic integrity signatures, constant-time event verification, environment
  mismatch detection, and retries that never resend a charge.
- **Idiomatic**: `context.Context` everywhere, typed `*wompi.Error` compatible with `errors.Is`/`errors.As`,
  functional options, safe for concurrent use.
- **Zero dependencies**: only the Go standard library.

## Installation

```sh
go get github.com/movaltech/wompi
```

Requires Go 1.27.4 or newer.

## Quick start

```go
client, err := wompi.New(
	os.Getenv("WOMPI_PUBLIC_KEY"),  // pub_test_... / pub_prod_...
	os.Getenv("WOMPI_PRIVATE_KEY"), // prv_test_... / prv_prod_...
	wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")),
	wompi.WithEventsSecret(os.Getenv("WOMPI_EVENTS_SECRET")),
)
if err != nil {
	log.Fatal(err)
}

// Acceptance tokens: show the permalinks to the payer and send the tokens
// only after explicit acceptance.
acceptance, err := client.Merchants.AcceptanceTokens(ctx)
if err != nil {
	log.Fatal(err)
}

tx, err := client.Transactions.Create(ctx, wompi.CreateTransactionParams{
	AmountInCents:      wompi.AmountInCents(49_900), // $49.900 COP
	CustomerEmail:      "buyer@example.com",
	Reference:          "order-1001",
	AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
	AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
	PaymentMethod:      wompi.CardPayment(cardToken, 1), // token from the Wompi widget / Wompi.js
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(tx.ID, tx.Status) // PENDING — the final status arrives via events
```

The integrity signature is computed for you from `WithIntegritySecret`. See [`examples/`](examples)
for complete, runnable programs.

## Environments and credentials

The environment is inferred from the key prefixes — there is no separate "test mode" flag:

| Credential | Sandbox | Production | Used for |
|---|---|---|---|
| Public key | `pub_test_` | `pub_prod_` | Tokenization, acceptance tokens, PSE banks |
| Private key | `prv_test_` | `prv_prod_` | Transactions, payment sources, voids, refunds, payment links |
| Integrity secret | `test_integrity_` | `prod_integrity_` | Signing transactions and checkouts |
| Events secret | `test_events_` | `prod_events_` | Verifying event checksums |

`wompi.New` refuses to build a client whose credentials belong to different environments, catching
the most common production misconfiguration before the first request. The private key may be empty
for clients that only perform public operations.

## Resources

```go
client.Merchants.Get(ctx)                                     // merchant info
client.Merchants.AcceptanceTokens(ctx)                        // terms + personal data tokens

client.Tokens.CreateCard(ctx, wompi.CardParams{...})          // card token (see PCI note)
client.Tokens.CreateNequi(ctx, phone)                         // Nequi subscription
client.Tokens.GetNequi(ctx, id)
client.Tokens.CreateBancolombiaTransfer(ctx, wompi.BancolombiaTransferTokenParams{...})
client.Tokens.GetBancolombiaTransfer(ctx, id)

client.PaymentSources.Create(ctx, wompi.CreatePaymentSourceParams{...}) // stored instruments
client.PaymentSources.Get(ctx, id)
client.PaymentSources.Void(ctx, id)

client.Transactions.Create(ctx, wompi.CreateTransactionParams{...})
client.Transactions.Get(ctx, id)
client.Transactions.List(ctx, wompi.ListTransactionsParams{...})
client.Transactions.Void(ctx, id, amountInCents)              // approved card transactions
client.Transactions.WaitForFinalStatus(ctx, id, interval)
client.Transactions.WaitForAsyncPaymentURL(ctx, id, interval)

client.Refunds.Create(ctx, wompi.CreateRefundParams{...})

client.PaymentLinks.Create(ctx, wompi.CreatePaymentLinkParams{...})
client.PaymentLinks.Get(ctx, id)
client.PaymentLinks.SetActive(ctx, id, false)

client.PSE.ListFinancialInstitutions(ctx)

client.WebCheckoutURL(wompi.WebCheckoutParams{...})           // signed hosted checkout URL
client.IntegritySignature(wompi.SignatureParams{...})         // for the Wompi widget
client.EventHandler(fn)                                       // verified webhook handler
```

## Payment methods

Each payment method has a constructor that sets exactly the fields Wompi requires:

| Method | Constructor | Next step for the payer |
|---|---|---|
| Card | `wompi.CardPayment(token, installments)` | None (or 3-D Secure) |
| Nequi | `wompi.NequiPayment(phone)` | Approve the push notification in the Nequi app |
| PSE | `wompi.PSEPayment(wompi.PSEPaymentParams{...})` | Redirect to `tx.AsyncPaymentURL()` |
| Botón Bancolombia | `wompi.BancolombiaTransferPayment(desc, url)` | Redirect to `tx.AsyncPaymentURL()` |
| Bancolombia QR | `wompi.BancolombiaQRPayment(desc)` | Scan `tx.ExtraString("qr_image")` |
| Bancolombia cash | `wompi.BancolombiaCollectPayment()` | Pay at a correspondent with the agreement code |
| Bancolombia BNPL | `wompi.BancolombiaBNPLPayment(wompi.BancolombiaBNPLParams{...})` | Redirect to `tx.AsyncPaymentURL()` |
| Daviplata | `wompi.DaviplataPayment(idType, id, desc)` | Redirect to `tx.AsyncPaymentURL()` (OTP) |
| Puntos Colombia | `wompi.PCOLPayment()` | Redirect to `tx.AsyncPaymentURL()` |
| SU+ Pay | `wompi.SuPlusPayment(idType, id)` | Redirect to `tx.AsyncPaymentURL()` |

Asynchronous methods produce their redirect URL a moment after the transaction is created:

```go
tx, err = client.Transactions.WaitForAsyncPaymentURL(ctx, tx.ID, time.Second)
http.Redirect(w, r, tx.AsyncPaymentURL(), http.StatusSeeOther)
```

### Recurring charges

Store a payment source once and charge it later without the payer present:

```go
source, err := client.PaymentSources.Create(ctx, wompi.CreatePaymentSourceParams{
	Type:               wompi.PaymentSourceCard,
	Token:              cardToken,
	CustomerEmail:      "buyer@example.com",
	AcceptanceToken:    acceptance.EndUserPolicy.AcceptanceToken,
	AcceptPersonalAuth: acceptance.PersonalDataAuth.AcceptanceToken,
})

tx, err := client.Transactions.Create(ctx, wompi.CreateTransactionParams{
	AmountInCents:   wompi.AmountInCents(29_900),
	CustomerEmail:   "buyer@example.com",
	Reference:       "subscription-2026-12",
	PaymentSourceID: source.ID,                    // requires the private key
	PaymentMethod:   wompi.SourceInstallments(1), // Wompi requires installments for card sources
})
```

> **PCI DSS note.** `Tokens.CreateCard` sends raw card data from your server. Unless you are PCI
> compliant, tokenize cards in the browser or app (Wompi widget / Wompi.js) and send only the token
> to your backend.

## Events (webhooks)

`EventHandler` verifies the checksum of every event (constant time, against the events secret)
before your code runs, responds `400` to invalid events, `500` when your handler fails — so Wompi
retries — and `200` otherwise:

```go
http.Handle("/wompi/events", client.EventHandler(func(ctx context.Context, e *wompi.Event) error {
	if e.Event != wompi.EventTransactionUpdated {
		return nil
	}
	tx := e.Data.Transaction
	return orders.UpdateStatus(ctx, tx.Reference, tx.ID, tx.Status) // must be idempotent
}))
```

For custom routers use `wompi.ParseEventRequest(r, secret)`, or `wompi.ParseEvent` +
`wompi.VerifyEvent` when you already hold the body. Treat events as the source of truth for a
transaction's final status; Wompi may deliver the same event more than once.

## Hosted checkout

No front-end integration at all? Redirect the payer to a signed Web Checkout URL, or share a
payment link:

```go
checkoutURL, err := client.WebCheckoutURL(wompi.WebCheckoutParams{
	Reference:      "order-1003",
	AmountInCents:  wompi.AmountInCents(25_000),
	RedirectURL:    "https://shop.example/checkout/done",
	ExpirationTime: time.Now().Add(30 * time.Minute),
})

link, err := client.PaymentLinks.Create(ctx, wompi.CreatePaymentLinkParams{
	Name: "Camiseta", Description: "Talla M", AmountInCents: wompi.AmountInCents(89_900),
})
fmt.Println(link.CheckoutURL()) // https://checkout.wompi.co/l/...
```

For the embedded widget, compute its `data-signature:integrity` server-side with
`client.IntegritySignature`.

## Errors

Every operation returns `(*T, error)`; when non-nil, the error is a `*wompi.Error`:

```go
_, err := client.Transactions.Create(ctx, params)
switch {
case errors.Is(err, wompi.ErrInputValidation):
	var wErr *wompi.Error
	errors.As(err, &wErr)
	fmt.Println(wErr.Messages) // map[reference:[La referencia ya ha sido usada]]
case errors.Is(err, wompi.ErrNotFound):
	// ...
case errors.Is(err, wompi.ErrInvalidAccessToken):
	// wrong key, or a sandbox key against production
}
```

| Sentinel | Meaning |
|---|---|
| `ErrInputValidation` | Wompi rejected one or more fields (`Messages` has the details) |
| `ErrNotFound` | The resource does not exist (or is not visible with that key) |
| `ErrInvalidAccessToken` | Invalid key or wrong environment |
| `ErrUnprocessable` | The operation is not allowed in the resource's current state |
| `ErrInvalidParams` | Rejected locally before any request was sent |
| `ErrMissingCredential` | The operation needs a credential the client was not given |
| `ErrCommunication` | Network failure talking to Wompi |
| `ErrInvalidSignature` | An event checksum did not match |

## Configuration

```go
wompi.New(pub, prv,
	wompi.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}), // default: 30s timeout
	wompi.WithRetries(2),                    // default: 0
	wompi.WithUserAgent("my-shop/1.4.2"),
	wompi.WithEnvironment(wompi.Production), // normally inferred
	wompi.WithBaseURL("http://localhost:9999"), // e.g. a mock server
)
```

Retries use exponential backoff with jitter and honor `Retry-After`. HTTP 429/503 are retried for
every method; network errors and other 5xx only for idempotent methods, so a charge or refund is
never sent twice.

## Testing

```sh
go test ./...
```

runs the full unit suite (60+ tests, >90% coverage) against `httptest` fixtures — no network access
required. The integrity and event checksum tests use vectors computed independently of the SDK,
including the example published in Wompi's own documentation.

An opt-in integration suite runs against the real sandbox:

```sh
WOMPI_PUBLIC_KEY=pub_test_... WOMPI_PRIVATE_KEY=prv_test_... WOMPI_INTEGRITY_SECRET=test_integrity_... \
go test -tags integration -run Integration -v ./...
```

### Sandbox test data

| Method | Approved | Declined |
|---|---|---|
| Card | `4242 4242 4242 4242` | `4111 1111 1111 1111` |
| Nequi | `3991111111` | `3992222222` |
| PSE (`financial_institution_code`) | `1` | `2` |

Any future expiration date and any 3-digit CVC work for cards.

## Versioning

This module follows [Semantic Versioning](https://semver.org). Until `v1.0.0`, minor releases may
contain breaking changes; they are always called out in the [CHANGELOG](CHANGELOG.md).

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). CI runs `gofmt`,
`go vet`, `staticcheck` and `go test -race` on every pull request.

## Security

Please report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## Disclaimer

This is an independent, community-maintained SDK and is not officially affiliated with Wompi or
Bancolombia. For SDK issues, [open an issue](https://github.com/movaltech/wompi/issues). For
account or payment-processing questions, contact [Wompi support](https://wompi.com) directly.
API reference: [docs.wompi.co](https://docs.wompi.co/docs/colombia/referencia/).

## License

[MIT](LICENSE)
