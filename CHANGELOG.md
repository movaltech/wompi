# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-11-23

Initial release. Requires Go 1.27.4 or newer.

### Added

- `Client` with functional options, environment inference from key prefixes and detection of
  credentials from mixed environments.
- `Merchants`: merchant information and acceptance tokens (`END_USER_POLICY`,
  `PERSONAL_DATA_AUTH`).
- `Tokens`: cards, Nequi and Bancolombia transfer tokenization.
- `PaymentSources`: create, get and void stored payment sources (CARD, NEQUI, DAVIPLATA,
  BANCOLOMBIA_TRANSFER).
- `Transactions`: create (with automatic integrity signature), get, search, void, and polling
  helpers (`Poll`, `WaitForFinalStatus`, `WaitForAsyncPaymentURL`). Recurring charges to stored
  payment sources, with `SourceInstallments` for CARD sources.
- Payment method constructors for CARD, NEQUI, PSE, BANCOLOMBIA_TRANSFER, BANCOLOMBIA_COLLECT,
  BANCOLOMBIA_QR, BANCOLOMBIA_BNPL, DAVIPLATA, PCOL and SU_PLUS.
- `Refunds`: Refunds V2.
- `PaymentLinks`: create (fixed or open amount), get, activate/deactivate.
- `PSE`: financial institutions.
- `IntegritySignature`, `Client.WebCheckoutURL`.
- Events: `ParseEvent`, `VerifyEvent`, `ParseEventRequest`, `NewEventHandler`,
  `Client.EventHandler`.
- Typed `*Error` with sentinels for `errors.Is`; nested validation messages are flattened to
  dotted field paths (e.g. `payment_method.sandbox_status`).
- Retries with exponential backoff, jitter and `Retry-After`, safe for non-idempotent requests.
- Runnable examples under `examples/` and an opt-in sandbox integration suite, verified live
  against the Wompi sandbox for CARD, NEQUI, PSE, BANCOLOMBIA_TRANSFER, BANCOLOMBIA_QR,
  BANCOLOMBIA_COLLECT, DAVIPLATA, SU_PLUS and BANCOLOMBIA_BNPL, recurring charges, voids,
  refunds, payment links and transaction search.

[Unreleased]: https://github.com/movaltech/wompi/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/movaltech/wompi/releases/tag/v0.1.0
