# Contributing

Thanks for your interest in improving `github.com/movaltech/wompi`.

## Ground rules

- **No external dependencies.** The SDK uses only the Go standard library.
- **Backed by the API contract.** New fields and endpoints must be traceable to Wompi's
  documentation (https://docs.wompi.co) or to a captured sandbox response. Mention the source in
  the pull request.
- **Never commit credentials.** Use environment variables; sandbox keys included.
- **Public API changes** need doc comments, tests and a `CHANGELOG.md` entry under `[Unreleased]`.

## Development

```sh
gofmt -l .                       # must print nothing
go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
```

Live checks against the sandbox (optional):

```sh
WOMPI_PUBLIC_KEY=pub_test_... WOMPI_PRIVATE_KEY=prv_test_... WOMPI_INTEGRITY_SECRET=test_integrity_... \
go test -tags integration -run Integration -v ./...
```

## Pull requests

1. Fork and create a branch from `main`.
2. Keep changes focused; one feature or fix per pull request.
3. Add or update tests. Unit tests must not hit the network — use `httptest`.
4. Make sure CI is green.
