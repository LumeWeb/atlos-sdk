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

// Re-exported request and response types from internal/client package

// Asset defines model for an asset.
type Asset = internalclient.Asset

// Blockchain defines model for a blockchain.
type Blockchain = internalclient.Blockchain

// InvoiceResponse defines model for invoice response.
type InvoiceResponse = internalclient.InvoiceResponse

// Payment defines model for a payment transaction.
type Payment = internalclient.Payment

// InvoiceCreatePostRequest defines model for invoice creation request.
type InvoiceCreatePostRequest = internalclient.InvoiceCreatePostRequest

// CreatePaymentPostRequest defines model for payment creation request.
type CreatePaymentPostRequest = internalclient.CreatePaymentPostRequest

// PaymentGetPostRequest defines model for getting payment request.
type PaymentGetPostRequest = internalclient.PaymentGetPostRequest

// AssetListPostRequest defines model for asset list request.
type AssetListPostRequest = internalclient.AssetListPostRequest

// InvoiceCancelPostRequest defines model for invoice cancel request.
type InvoiceCancelPostRequest = internalclient.InvoiceCancelPostRequest

// PaymentWebSocketPostRequest defines model for payment websocket request.
type PaymentWebSocketPostRequest = internalclient.PaymentWebSocketPostRequest

// CancelPostRequest defines model for subscription cancel request.
type CancelPostRequest = internalclient.CancelPostRequest

// FindByHashPostRequest defines model for find by hash request.
type FindByHashPostRequest = internalclient.FindByHashPostRequest

// FindByHashPostResponseBody defines model for find by hash response body.
type FindByHashPostResponseBody = internalclient.FindByHashPostResponseBody

// TransactionListPostRequest defines model for transaction list request.
type TransactionListPostRequest = internalclient.TransactionListPostRequest

// TransactionListPostResponseBody defines model for transaction list response body.
type TransactionListPostResponseBody = internalclient.TransactionListPostResponseBody

// CancelPayoutPostRequest defines model for cancel payout request.
type CancelPayoutPostRequest = internalclient.CancelPayoutPostRequest

// SendTokenPostRequest defines model for send token request.
type SendTokenPostRequest = internalclient.SendTokenPostRequest

// SendTokenPostResponseBody defines model for send token response body.
type SendTokenPostResponseBody = internalclient.SendTokenPostResponseBody

// DefaultEndpoint is the default API endpoint for the Atlos Gateway API.
const DefaultEndpoint = "https://api.atlos.io/gateway/rest"

// ApiSecretHeader is the HTTP header name for API secret authentication.
const ApiSecretHeader = "ApiSecret"

func init() {
	// Ensure apiSecret is not empty at package init time
}

// Client represents the main Atlos SDK client.
type Client struct {
	apiSecret   string
	baseURL     string
	httpClient  *http.Client
	internalGen *internalclient.ClientWithResponses
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
	normalizedURL, err := normalizeURL(cfg.endpoint)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}

	// Create request editor with API Secret (uses ApiSecret header, not Bearer)
	requestEditor := internalclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		if cfg.apiSecret != "" {
			req.Header.Set(ApiSecretHeader, cfg.apiSecret)
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
func (c *Client) SetHTTPClient(client *http.Client) error {
	c.httpClient = client
	requestEditor := internalclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		if c.apiSecret != "" {
			req.Header.Set(ApiSecretHeader, c.apiSecret)
		}
		return nil
	})
	internalGen, err := internalclient.NewClientWithResponses(c.baseURL, requestEditor, internalclient.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to update HTTP client: %w", err)
	}
	c.internalGen = internalGen
	return nil
}

// AssetList retrieves a list of assets available for payment.
func (c *Client) AssetList(ctx context.Context, req AssetListPostRequest) ([]Asset, error) {
	resp, err := c.internalGen.AssetListPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve asset list: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no asset data in response")
	}
	return *resp.JSON200, nil
}

// InvoiceCancel cancels an existing invoice.
func (c *Client) InvoiceCancel(ctx context.Context, req InvoiceCancelPostRequest) error {
	resp, err := c.internalGen.InvoiceCancelPostWithResponse(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to cancel invoice: %w", err)
	}
	return checkResponseStatus(resp.StatusCode(), resp.Body)
}

// InvoiceCreate creates a new invoice.
func (c *Client) InvoiceCreate(ctx context.Context, req InvoiceCreatePostRequest) (*InvoiceResponse, error) {
	resp, err := c.internalGen.InvoiceCreatePostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no invoice data in response")
	}
	return resp.JSON200, nil
}

// CreatePayment creates a new payment.
func (c *Client) CreatePayment(ctx context.Context, req CreatePaymentPostRequest) (*Payment, error) {
	resp, err := c.internalGen.CreatePaymentPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no payment data in response")
	}
	return resp.JSON200, nil
}

// PaymentGet retrieves payment details.
func (c *Client) PaymentGet(ctx context.Context, req PaymentGetPostRequest) (*Payment, error) {
	resp, err := c.internalGen.PaymentGetPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no payment data in response")
	}
	return resp.JSON200, nil
}

// PaymentWebSocket returns a WebSocket connection for payment updates.
func (c *Client) PaymentWebSocket(ctx context.Context, req PaymentWebSocketPostRequest) ([]byte, error) {
	resp, err := c.internalGen.PaymentWebSocketPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebSocket token: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Cancel cancels a subscription.
func (c *Client) Cancel(ctx context.Context, req CancelPostRequest) error {
	resp, err := c.internalGen.CancelPostWithResponse(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}
	return checkResponseStatus(resp.StatusCode(), resp.Body)
}

// FindByHash finds a transaction by its blockchain hash.
func (c *Client) FindByHash(ctx context.Context, req FindByHashPostRequest) (*FindByHashPostResponseBody, error) {
	resp, err := c.internalGen.FindByHashPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no transaction data in response")
	}
	return resp.JSON200, nil
}

// TransactionList retrieves a list of transactions.
func (c *Client) TransactionList(ctx context.Context, req TransactionListPostRequest) (*TransactionListPostResponseBody, error) {
	resp, err := c.internalGen.TransactionListPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve transactions: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no transaction data in response")
	}
	return resp.JSON200, nil
}

// CancelPayout cancels a payout.
func (c *Client) CancelPayout(ctx context.Context, req CancelPayoutPostRequest) error {
	resp, err := c.internalGen.CancelPayoutPostWithResponse(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to cancel payout: %w", err)
	}
	return checkResponseStatus(resp.StatusCode(), resp.Body)
}

// SendToken sends tokens to a recipient address.
func (c *Client) SendToken(ctx context.Context, req SendTokenPostRequest) (*SendTokenPostResponseBody, error) {
	resp, err := c.internalGen.SendTokenPostWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token: %w", err)
	}
	if err := checkResponseStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("no transaction data in response")
	}
	return resp.JSON200, nil
}

// checkResponse validates HTTP response status code using type assertion.
// All generated *PostResponse types have StatusCode() int method and Body []byte field.
func checkResponseStatus(statusCode int, body []byte) error {
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("unexpected status code %d: %s", statusCode, string(body))
	}
	return nil
}

// normalizeURL parses and normalizes a URL string.
func normalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	parsedURL.Path = ""
	return parsedURL.String(), nil
}
