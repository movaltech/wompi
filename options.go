package wompi

import "net/http"

// Option configures a Client created with New.
type Option func(*Client)

// WithHTTPClient overrides the *http.Client used for every request. Useful to
// set timeouts, proxies, instrumentation (e.g. OpenTelemetry transports) or
// test doubles. The default client has a 30 second timeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithEnvironment sets the environment explicitly. It is normally inferred
// from the key prefixes; when set, every credential with a recognizable
// prefix must belong to it.
func WithEnvironment(env Environment) Option {
	return func(c *Client) { c.env = env }
}

// WithBaseURL overrides the API base URL (for example a mock server in
// tests). Defaults to the environment's base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithCheckoutBaseURL overrides the hosted checkout base URL used by
// Client.WebCheckoutURL and PaymentLink.CheckoutURL. Defaults to
// CheckoutBaseURL.
func WithCheckoutBaseURL(url string) Option {
	return func(c *Client) { c.checkoutURL = url }
}

// WithIntegritySecret sets the integrity secret (test_integrity_... or
// prod_integrity_...). When set, Transactions.Create signs transactions
// automatically and Client.WebCheckoutURL builds signed checkout links.
func WithIntegritySecret(secret string) Option {
	return func(c *Client) { c.integritySecret = secret }
}

// WithEventsSecret sets the events secret (test_events_... or
// prod_events_...) used by Client.ParseEvent and Client.EventHandler to
// verify event checksums.
func WithEventsSecret(secret string) Option {
	return func(c *Client) { c.eventsSecret = secret }
}

// WithRetries sets how many times a request is retried after a transient
// failure. Defaults to 0 (no retries).
//
// HTTP 429 and 503 responses are retried for every method. Network errors
// and other 5xx responses are retried only for idempotent methods (GET, PUT,
// PATCH), so a POST that may already have been processed — a charge, a
// refund — is never sent twice.
func WithRetries(n int) Option {
	return func(c *Client) { c.retries = n }
}

// WithUserAgent prepends an application identifier (e.g. "my-shop/1.4.2")
// to the SDK's User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}
