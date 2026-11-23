// Package wompi is a professional, dependency-free Go SDK for the Wompi
// payments API (https://wompi.com), Colombia.
//
// It covers the full server-side integration surface: merchant acceptance
// tokens, card / Nequi / Bancolombia tokenization, payment sources for
// recurring charges, transactions with every supported payment method (CARD,
// NEQUI, PSE, BANCOLOMBIA_TRANSFER, BANCOLOMBIA_COLLECT, BANCOLOMBIA_QR,
// DAVIPLATA, PCOL, SU_PLUS, BANCOLOMBIA_BNPL), voids, refunds, payment links,
// PSE financial institutions, integrity signatures, Web Checkout URLs and
// verification of event (webhook) checksums.
//
// # Getting started
//
//	client, err := wompi.New(
//		os.Getenv("WOMPI_PUBLIC_KEY"),
//		os.Getenv("WOMPI_PRIVATE_KEY"),
//		wompi.WithIntegritySecret(os.Getenv("WOMPI_INTEGRITY_SECRET")),
//		wompi.WithEventsSecret(os.Getenv("WOMPI_EVENTS_SECRET")),
//	)
//
// The environment (sandbox or production) is inferred from the key prefixes
// (pub_test_/prv_test_ vs. pub_prod_/prv_prod_), and New refuses to build a
// client whose keys and secrets belong to different environments.
//
// # Credentials
//
// Wompi issues four credentials per environment:
//
//   - Public key (pub_*): tokenization, acceptance tokens, PSE banks.
//   - Private key (prv_*): transactions, payment sources, voids, refunds,
//     payment links. Never expose it to a browser or mobile app.
//   - Integrity secret (*_integrity_*): signs transactions and checkouts.
//   - Events secret (*_events_*): verifies event (webhook) checksums.
//
// # Errors
//
// Every failed operation returns a *Error. Use errors.Is with the exported
// sentinels (ErrNotFound, ErrInputValidation, ...) or errors.As to inspect
// the Wompi error type, reason and per-field validation messages.
//
// The Client is safe for concurrent use and should be reused.
package wompi
