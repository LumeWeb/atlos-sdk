package atlos

//go:generate oapi-codegen -config oai-codegen.yaml swagger-spec.yaml
//go:generate oapi-codegen -config oai-codegen-server.yaml swagger-spec.yaml

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	internalclient "go.lumeweb.com/atlos-sdk/internal/client"
)

// DefaultEndpoint is the default API endpoint for the Atlos Gateway API.
const DefaultEndpoint = "https://api.atlos.io/gateway/rest"

// Client represents the main Atlos SDK client.
type Client struct {
	apiSecret    string
	baseURL      string
	httpClient   *http.Client
	internalGen  *internalclient.ClientWithResponses
}

// ClientConfig holds configuration for creating a new Client.
type ClientConfig struct {
	Endpoint  string
	APISecret string
}

// DefaultClientConfig returns default configuration for the client.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Endpoint: DefaultEndpoint,
	}
}

// ClientOption applies configuration to ClientConfig.
type ClientOption func(*clientConfig)

type clientConfig struct {
	endpoint  string
	apiSecret string
}

// WithEndpoint sets the API endpoint URL.
func WithEndpoint(endpoint string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.endpoint = endpoint
	}
}

// WithAPISecret sets the API secret for authentication.
func WithAPISecret(secret string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.apiSecret = secret
	}
}

// NewClient creates a new Atlos SDK client.
// The apiSecret parameter specifies your API secret from the Merchant Panel.
func NewClient(apiSecret string, opts ...ClientOption) (*Client, error) {
	cfg := clientConfig{
		endpoint:  DefaultEndpoint,
		apiSecret: apiSecret,
	}

	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	// Normalize URL to ensure consistent behavior
	parsedURL, err := url.Parse(cfg.endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	parsedURL.Path = ""
	normalizedURL := parsedURL.String()

	httpClient := &http.Client{}

	// Create request editor with API Secret (uses ApiSecret header, not Bearer)
	requestEditor := internalclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		if cfg.apiSecret != "" {
			req.Header.Set("ApiSecret", cfg.apiSecret)
		}
		return nil
	})

	// Create internal generated client
	internalGen, err := internalclient.NewClientWithResponses(normalizedURL, requestEditor, internalclient.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create internal client: %w", err)
	}

	c := &Client{
		apiSecret:   cfg.apiSecret,
		baseURL:     normalizedURL,
		httpClient:  httpClient,
		internalGen: internalGen,
	}

	return c, nil
}

// BaseURL returns the base URL for the API endpoint.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// APISecret returns the current API secret.
func (c *Client) APISecret() string {
	return c.apiSecret
}

// SetHTTPClient sets a custom HTTP client for all API requests.
// This is useful for testing or customizing HTTP behavior.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
	var err error
	requestEditor := internalclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		if c.apiSecret != "" {
			req.Header.Set("ApiSecret", c.apiSecret)
		}
		return nil
	})
	c.internalGen, err = internalclient.NewClientWithResponses(c.baseURL, requestEditor, internalclient.WithHTTPClient(client))
	if err != nil {
		panic(err)
	}
}
