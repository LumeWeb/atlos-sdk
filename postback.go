// Package atlos provides a Go SDK for the ATLOS cryptocurrency payment gateway.
//
// The postback functionality provides tools for handling and sending postback notifications
// when payments are confirmed on the blockchain.
package atlos

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// PostbackNotification represents a postback notification sent by ATLOS when a payment is confirmed.
type PostbackNotification struct {
	TransactionId   string  `json:"TransactionId"`
	SubscriptionId  string  `json:"SubscriptionId"`
	MerchantId      string  `json:"MerchantId"`
	OrderId         string  `json:"OrderId"`
	Amount          float64 `json:"Amount"`
	Fee             float64 `json:"Fee"`
	Blockchain      string  `json:"Blockchain"`
	Asset           string  `json:"Asset"`
	BlockchainHash  string  `json:"BlockchainHash"`
	UserWallet      string  `json:"UserWallet"`
	UserName        string  `json:"UserName"`
	UserEmail       string  `json:"UserEmail"`
	OrderAmount     float64 `json:"OrderAmount"`
	OrderCurrency   string  `json:"OrderCurrency"`
	PaidAmount      float64 `json:"PaidAmount"`
	TimeSent        string  `json:"TimeSent"`
	Status          int     `json:"Status"`
}

// SignatureHeader is the HTTP header name for the HMAC signature.
const SignatureHeader = "Signature"

// Validate performs validation on the postback notification fields.
func (p *PostbackNotification) Validate() error {
	if p.TransactionId == "" {
		return fmt.Errorf("TransactionId is required")
	}
	if p.MerchantId == "" {
		return fmt.Errorf("MerchantId is required")
	}
	if p.Amount < 0 {
		return fmt.Errorf("Amount must be non-negative")
	}
	if p.Fee < 0 {
		return fmt.Errorf("Fee must be non-negative")
	}
	if p.Blockchain == "" {
		return fmt.Errorf("Blockchain is required")
	}
	if p.Asset == "" {
		return fmt.Errorf("Asset is required")
	}
	if p.Status == 0 {
		return fmt.Errorf("Status is required")
	}
	if p.Status != 100 {
		return fmt.Errorf("Status must be 100 (success), got %d", p.Status)
	}

	if _, err := time.Parse(time.RFC3339, p.TimeSent); err != nil {
		return fmt.Errorf("TimeSent is not a valid RFC3339 timestamp: %w", err)
	}

	if p.OrderAmount < 0 {
		return fmt.Errorf("OrderAmount must be non-negative")
	}
	if p.PaidAmount < 0 {
		return fmt.Errorf("PaidAmount must be non-negative")
	}

	return nil
}

// VerifySignature verifies the HMAC-SHA256 signature of the postback notification.
func (p *PostbackNotification) VerifySignature(apiSecret, signature string) (bool, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return false, fmt.Errorf("failed to marshal notification: %w", err)
	}

	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write(data)
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature)), nil
}

// PostbackSender handles sending postback notifications to an endpoint.
type PostbackSender struct {
	apiSecret string
	client    *http.Client
	logger    *zap.Logger
}

// NewPostbackSender creates a new PostbackSender.
func NewPostbackSender(apiSecret string) *PostbackSender {
	return NewPostbackSenderWithLogger(apiSecret, zap.NewNop())
}

// NewPostbackSenderWithLogger creates a new PostbackSender with a custom logger.
func NewPostbackSenderWithLogger(apiSecret string, logger *zap.Logger) *PostbackSender {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PostbackSender{
		apiSecret: apiSecret,
		client:    &http.Client{},
		logger:    logger,
	}
}

// Send sends a postback notification to the specified endpoint.
func (ps *PostbackSender) Send(endpoint string, notification *PostbackNotification) error {
	ps.logger.Info("Sending webhook", 
		zap.String("endpoint", endpoint),
		zap.String("transaction_id", notification.TransactionId),
		zap.String("merchant_id", notification.MerchantId),
		zap.Float64("amount", notification.Amount),
		zap.String("blockchain", notification.Blockchain),
		zap.String("asset", notification.Asset),
	)

	if err := notification.Validate(); err != nil {
		ps.logger.Error("Webhook validation failed", 
			zap.Error(err),
			zap.String("transaction_id", notification.TransactionId),
		)
		return fmt.Errorf("notification validation failed: %w", err)
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		ps.logger.Error("Invalid webhook endpoint URL", 
			zap.Error(err),
			zap.String("endpoint", endpoint),
		)
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	data, err := json.Marshal(notification)
	if err != nil {
		ps.logger.Error("Failed to marshal webhook notification", 
			zap.Error(err),
			zap.String("transaction_id", notification.TransactionId),
		)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	h := hmac.New(sha256.New, []byte(ps.apiSecret))
	h.Write(data)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req, err := http.NewRequest("POST", endpointURL.String(), bytes.NewReader(data))
	if err != nil {
		ps.logger.Error("Failed to create webhook request", 
			zap.Error(err),
			zap.String("endpoint", endpoint),
		)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, signature)

	ps.logger.Debug("Sending webhook request",
		zap.String("endpoint", endpoint),
		zap.String("method", req.Method),
		zap.Int("content_length", len(data)),
	)

	resp, err := ps.client.Do(req)
	if err != nil {
		ps.logger.Error("Failed to send webhook", 
			zap.Error(err),
			zap.String("endpoint", endpoint),
			zap.String("transaction_id", notification.TransactionId),
		)
		return fmt.Errorf("failed to send postback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ps.logger.Warn("Webhook endpoint returned non-success status",
			zap.Int("status", resp.StatusCode),
			zap.String("endpoint", endpoint),
			zap.String("transaction_id", notification.TransactionId),
		)
		return fmt.Errorf("postback endpoint returned non-success status: %d", resp.StatusCode)
	}

	ps.logger.Info("Webhook sent successfully",
		zap.String("endpoint", endpoint),
		zap.String("transaction_id", notification.TransactionId),
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}

// PostbackHandler receives postback notifications and verifies them.
type PostbackHandler struct {
	apiSecret string
}

// NewPostbackHandler creates a new PostbackHandler.
func NewPostbackHandler(apiSecret string) *PostbackHandler {
	return &PostbackHandler{
		apiSecret: apiSecret,
	}
}

// HandleRequest handles an incoming postback request and verifies the signature.
func (ph *PostbackHandler) HandleRequest(req *http.Request) (*PostbackNotification, error) {
	signature := req.Header.Get(SignatureHeader)
	if signature == "" {
		return nil, fmt.Errorf("missing %s header", SignatureHeader)
	}

	var notification PostbackNotification
	if err := json.NewDecoder(req.Body).Decode(&notification); err != nil {
		return nil, fmt.Errorf("failed to decode notification: %w", err)
	}

	if err := notification.Validate(); err != nil {
		return nil, fmt.Errorf("notification validation failed: %w", err)
	}

	valid, err := notification.VerifySignature(ph.apiSecret, signature)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid signature")
	}

	return &notification, nil
}

// MockServerConfig contains configuration for the mock server.
type MockServerConfig struct {
	PostbackURL string
	APISecret   string
}

// MockServer is a mock server implementation for testing.
type MockServer struct {
	config MockServerConfig
	sender *PostbackSender
}

// NewMockServer creates a new MockServer.
func NewMockServer(config MockServerConfig) *MockServer {
	sender := NewPostbackSender(config.APISecret)
	return &MockServer{
		config: config,
		sender: sender,
	}
}

// SendTokenPost handles mock token sending with postback support.
func (ms *MockServer) SendTokenPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// SendPostback sends a postback notification for a test payment.
func (ms *MockServer) SendPostback(notification *PostbackNotification) error {
	if ms.config.PostbackURL == "" {
		return fmt.Errorf("no postback URL configured")
	}
	return ms.sender.Send(ms.config.PostbackURL, notification)
}

// PostbackOption configures a PostbackNotification.
type PostbackOption func(*PostbackNotification)

// WithDefaults returns an option that applies a set of default values including invoice fields.
// This is useful for backward compatibility with existing tests that expect full defaults.
func WithDefaults() PostbackOption {
	return func(pn *PostbackNotification) {
		if pn.OrderId == "" {
			pn.OrderId = "test-order-123"
		}
		if pn.UserName == "" {
			pn.UserName = "Test User"
		}
		if pn.UserEmail == "" {
			pn.UserEmail = "test@example.com"
		}
		if pn.OrderAmount == 0 {
			pn.OrderAmount = 19.95
		}
		if pn.OrderCurrency == "" {
			pn.OrderCurrency = "USD"
		}
		if pn.PaidAmount == 0 {
			pn.PaidAmount = 20.00
		}
	}
}

// CreateTestPostback creates a test postback notification with default values.
// Use WithDefaults() to include default invoice fields (OrderId, UserName, UserEmail, etc.)
// for backward compatibility with existing tests.
func CreateTestPostback(merchantId string, opts ...PostbackOption) *PostbackNotification {
	now := time.Now().UTC().Format(time.RFC3339)
	pn := &PostbackNotification{
		TransactionId:  "test-tx-id",
		SubscriptionId: "",
		MerchantId:     merchantId,
		Amount:         20.00,
		Fee:            0.20,
		Blockchain:     "ETH",
		Asset:          "USDC",
		BlockchainHash: "0x" + "0000000000000000000000000000000000000000000000000000000000000000",
		UserWallet:     "0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b",
		TimeSent:       now,
		Status:         100,
	}
	for _, opt := range opts {
		opt(pn)
	}
	return pn
}
