package wompi

import (
	"net/http"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "0.1.0"

// Environment identifies a Wompi environment.
type Environment string

// Supported environments.
const (
	Sandbox    Environment = "sandbox"
	Production Environment = "production"
)

// Base URLs of the Wompi API and hosted checkout.
const (
	SandboxBaseURL    = "https://sandbox.wompi.co/v1"
	ProductionBaseURL = "https://production.wompi.co/v1"
	CheckoutBaseURL   = "https://checkout.wompi.co"
)

// BaseURL returns the API base URL of the environment.
func (e Environment) BaseURL() string {
	if e == Production {
		return ProductionBaseURL
	}
	return SandboxBaseURL
}

func (e Environment) valid() bool { return e == Sandbox || e == Production }

const defaultTimeout = 30 * time.Second

// Client is the entry point to the Wompi API. Create one with New and reuse
// it: it is safe for concurrent use by multiple goroutines.
type Client struct {
	publicKey       string
	privateKey      string
	integritySecret string
	eventsSecret    string

	env         Environment
	baseURL     string
	checkoutURL string
	httpClient  *http.Client
	retries     int
	userAgent   string

	// Merchants exposes merchant information and acceptance tokens.
	Merchants *MerchantService
	// Tokens tokenizes cards, Nequi accounts and Bancolombia accounts.
	Tokens *TokenService
	// PaymentSources manages stored payment sources for recurring charges.
	PaymentSources *PaymentSourceService
	// Transactions creates, retrieves, searches and voids transactions.
	Transactions *TransactionService
	// PaymentLinks creates and manages hosted payment links.
	PaymentLinks *PaymentLinkService
	// Refunds creates refunds of approved transactions.
	Refunds *RefundService
	// PSE lists the financial institutions available for PSE payments.
	PSE *PSEService
}

// New creates a Client from a Wompi public key and private key.
//
// The private key may be empty when the client is only used for public
// operations (tokenization, acceptance tokens, PSE banks, public payment
// links); private operations then fail with ErrMissingCredential.
//
// New performs no network call. The environment is inferred from the key
// prefixes unless WithEnvironment is given, and an error is returned if the
// keys and secrets belong to different environments.
func New(publicKey, privateKey string, opts ...Option) (*Client, error) {
	publicKey = strings.TrimSpace(publicKey)
	privateKey = strings.TrimSpace(privateKey)
	if publicKey == "" {
		return nil, newError(ErrTypeInvalidConfig, "public key is required")
	}

	c := &Client{
		publicKey:   publicKey,
		privateKey:  privateKey,
		checkoutURL: CheckoutBaseURL,
		httpClient:  &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	if err := c.resolveEnvironment(); err != nil {
		return nil, err
	}
	if c.baseURL == "" {
		c.baseURL = c.env.BaseURL()
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	c.checkoutURL = strings.TrimRight(c.checkoutURL, "/")
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if c.retries < 0 {
		c.retries = 0
	}

	ua := "wompi-go/" + Version
	if c.userAgent != "" {
		ua = c.userAgent + " " + ua
	}
	c.userAgent = ua

	c.Merchants = &MerchantService{client: c}
	c.Tokens = &TokenService{client: c}
	c.PaymentSources = &PaymentSourceService{client: c}
	c.Transactions = &TransactionService{client: c}
	c.PaymentLinks = &PaymentLinkService{client: c}
	c.Refunds = &RefundService{client: c}
	c.PSE = &PSEService{client: c}
	return c, nil
}

// Environment returns the environment the client targets.
func (c *Client) Environment() Environment { return c.env }

// PublicKey returns the merchant public key the client was built with. It is
// safe to share with front-end code (e.g. the Wompi widget).
func (c *Client) PublicKey() string { return c.publicKey }

// resolveEnvironment infers the environment from the credential prefixes and
// rejects credentials that belong to different environments — a classic
// source of INVALID_ACCESS_TOKEN errors in production.
func (c *Client) resolveEnvironment() error {
	creds := []struct{ name, value string }{
		{"public key", c.publicKey},
		{"private key", c.privateKey},
		{"integrity secret", c.integritySecret},
		{"events secret", c.eventsSecret},
	}

	env := c.env
	if env != "" && !env.valid() {
		return newError(ErrTypeInvalidConfig, "unknown environment "+string(env))
	}
	explicit := env != ""

	for _, cred := range creds {
		got, ok := envFromCredential(cred.value)
		if !ok {
			continue
		}
		if env == "" {
			env = got
			continue
		}
		if got != env {
			source := "the other credentials"
			if explicit {
				source = "WithEnvironment(" + string(env) + ")"
			}
			return newError(ErrTypeInvalidConfig,
				cred.name+" belongs to "+string(got)+" but "+source+" target "+string(env))
		}
	}

	if env == "" {
		return newError(ErrTypeInvalidConfig,
			"cannot infer environment from key prefixes; use WithEnvironment(wompi.Sandbox) or WithEnvironment(wompi.Production)")
	}
	c.env = env
	return nil
}

func envFromCredential(v string) (Environment, bool) {
	switch {
	case v == "":
		return "", false
	case strings.HasPrefix(v, "pub_test_"), strings.HasPrefix(v, "prv_test_"),
		strings.HasPrefix(v, "test_integrity_"), strings.HasPrefix(v, "test_events_"):
		return Sandbox, true
	case strings.HasPrefix(v, "pub_prod_"), strings.HasPrefix(v, "prv_prod_"),
		strings.HasPrefix(v, "prod_integrity_"), strings.HasPrefix(v, "prod_events_"):
		return Production, true
	default:
		return "", false
	}
}
