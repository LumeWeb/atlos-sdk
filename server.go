// Package atlos provides a Go SDK for the ATLOS cryptocurrency payment gateway.
package atlos

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	internalclient "go.lumeweb.com/atlos-sdk/internal/client"
)

var (
	pendingStatus = "pending"
	successStatus = "success"
	zeroAmountStr = "0"
	defaultFeeStr = "020000000"
)

// invoiceData stores the full invoice request alongside the response,
// so that postback notifications can include OrderId, amounts, and user info.
type invoiceData struct {
	response   *InvoiceResponse
	request    InvoiceCreatePostRequest
	paidAmount *float64 // Optional override for PaidAmount in postback (simulates crypto-to-fiat conversion difference)
}

// Server represents a lightweight server implementation that extends the internal ServerInterface.
// It implements all methods defined in client.ServerInterface and provides postback notification capability.
// Use client.Handler() or client.HandlerFromMux() to create an HTTP handler from this server.
type Server struct {
	config    serverConfig
	sender    *PostbackSender
	logger    *zap.Logger

	// Test data storage
	mu         sync.RWMutex
	invoices   map[string]*invoiceData
	payments   map[string]*Payment
	// paymentToInvoice maps payment ID → invoice ID so postbacks can look up invoice data
	paymentToInvoice map[string]string
}

// PostbackMode controls when postbacks are sent during payment simulation.
type PostbackMode int

const (
	// PostbackDisabled means no postbacks are sent
	PostbackDisabled PostbackMode = iota
	// PostbackImmediate sends postbacks immediately when payment completes
	PostbackImmediate
	// PostbackManual only sends postbacks when explicitly called via SendPostback()
	PostbackManual
)

// serverConfig holds the configuration for a Server.
type serverConfig struct {
	postbackURL string
	apiSecret   string
	httpClient  *http.Client
	postbackMode PostbackMode
	logger    *zap.Logger
}

// ServerOption configures a Server.
type ServerOption func(*serverConfig)

// WithPostbackURL sets the postback URL for the server.
func WithPostbackURL(url string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.postbackURL = url
	}
}

// WithSharedSecret sets the shared secret for HMAC signature generation.
func WithSharedSecret(secret string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.apiSecret = secret
	}
}

// WithHTTPClient sets a custom HTTP client for all postback requests.
func WithHTTPClient(httpClient *http.Client) ServerOption {
	return func(cfg *serverConfig) {
		cfg.httpClient = httpClient
	}
}

// WithPostbackMode sets when postbacks are sent during simulation.
func WithPostbackMode(mode PostbackMode) ServerOption {
	return func(cfg *serverConfig) {
		cfg.postbackMode = mode
	}
}

// WithLogger sets a custom logger for the server.
func WithLogger(logger *zap.Logger) ServerOption {
	return func(cfg *serverConfig) {
		cfg.logger = logger
	}
}



// NewServer creates a new Server that implements client.ServerInterface.
// The server handles API requests for all endpoints and can optionally send postback notifications.
// Use client.Handler(server) or client.HandlerFromMux(server, mux) to create an HTTP handler.
//
// Example:
//
//	server := NewServer(
//	    WithPostbackURL("https://example.com/postback"),
//	    WithSharedSecret("your-api-secret"),
//	)
func NewServer(opts ...ServerOption) (*Server, error) {
	cfg := serverConfig{
		httpClient: &http.Client{},
		logger:     zap.NewNop(),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.apiSecret == "" {
		return nil, fmt.Errorf("shared secret is required")
	}

	if cfg.logger == nil {
		cfg.logger = zap.NewNop()
	}

	sender := NewPostbackSenderWithLogger(cfg.apiSecret, cfg.logger)

	return &Server{
		config:           cfg,
		sender:           sender,
		logger:           cfg.logger,
		invoices:         make(map[string]*invoiceData),
		payments:         make(map[string]*Payment),
		paymentToInvoice: make(map[string]string),
	}, nil
}

// AssetListPost handles /Asset/List requests and returns available assets and blockchains.
func (s *Server) AssetListPost(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, AssetsFromAtlas)
}

// InvoiceCancelPost handles /Invoice/Cancel requests.
func (s *Server) InvoiceCancelPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// InvoiceCreatePost handles /Invoice/Create requests and generates a new invoice.
func (s *Server) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	var req InvoiceCreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	invoiceID := generateTransactionID()

	invoice := &InvoiceResponse{
		Id:          &invoiceID,
		PaymentLink: func() *string { s := fmt.Sprintf("https://atlos.com/payment/%s", invoiceID); return &s }(),
	}
	s.invoices[invoiceID] = &invoiceData{
		response: invoice,
		request:  req,
	}

	response := *invoice
	writeJSONResponse(w, response)
}

// CreatePaymentPost handles /Payment/Create requests and generates a wallet address.
func (s *Server) CreatePaymentPost(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	paymentID := generateTransactionID()

	recipientAddr := fmt.Sprintf("0x%s", generateHexString(40))

	payment := &Payment{
		Id:               &paymentID,
		Amount:           &zeroAmountStr,
		AssetCode:        &req.AssetCode,
		BlockchainCode:   &req.BlockchainCode,
		RecipientAddress: &recipientAddr,
		Fee:              &defaultFeeStr,
		Status:           &pendingStatus,
	}
	s.payments[paymentID] = payment
	s.paymentToInvoice[paymentID] = req.InvoiceId

	response := *payment
	writeJSONResponse(w, response)
}

// PaymentGetPost handles /Payment/Get requests and returns payment status.
func (s *Server) PaymentGetPost(w http.ResponseWriter, r *http.Request) {
	var req PaymentGetPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

payment, exists := s.payments[req.PaymentId]
	if !exists {
		http.Error(w, "payment not found", http.StatusNotFound)
		return
	}

	response := *payment
	writeJSONResponse(w, response)
}

// PaymentWebSocketPost handles /Payment/WebSocket requests.
func (s *Server) PaymentWebSocketPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// CancelPost handles /Subscription/Cancel requests.
func (s *Server) CancelPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// FindByHashPost handles /Transaction/FindByHash requests.
func (s *Server) FindByHashPost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.FindByHashPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// For test purposes, return mock transaction data
	isFound := req.BlockchainHash != nil && *req.BlockchainHash != ""
	amount := float32(100.0)
	status := float32(100.0)

	response := internalclient.FindByHashPostResponseBody{
		IsFound: &isFound,
		Transaction: &internalclient.Transaction{
			Id:             new("txn-123"),
			MerchantId:     new("test-merchant"),
			Amount:         &amount,
			AssetCode:      new("BTC"),
			BlockchainCode: new("BTC"),
			BlockchainHash: req.BlockchainHash,
			Status:         &status,
		},
	}
	writeJSONResponse(w, response)
}

// TransactionListPost handles /Transaction/List requests.
func (s *Server) TransactionListPost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.TransactionListPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// For test purposes, return mock transaction list
	totalCount := float32(1)
	amount := float32(100.0)
	status := float32(100)

	response := internalclient.TransactionListPostResponseBody{
		TotalCount: &totalCount,
		Transactions: &[]internalclient.Transaction{
			{
				Id:             new("txn-456"),
				MerchantId:     new("test-merchant"),
				Amount:         &amount,
				AssetCode:      new("BTC"),
				BlockchainCode: new("BTC"),
				Status:         &status,
			},
		},
	}
	writeJSONResponse(w, response)
}

// CancelPayoutPost handles /Wallet/CancelPayout requests.
func (s *Server) CancelPayoutPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// SendTokenPost handles /Wallet/SendToken requests.
func (s *Server) SendTokenPost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.SendTokenPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// For test purposes, return mock send token response
	response := internalclient.SendTokenPostResponseBody{
		Id:               new("inv-789"),
		TokenAmount:      new("100.0"),
		AssetCode:        &req.AssetCode,
		BlockchainCode:   &req.BlockchainCode,
		RecipientAddress: &req.RecipientAddress,
	}
	writeJSONResponse(w, response)
}

// completePaymentRequest represents a request to manually complete a payment.
type completePaymentRequest struct {
	PaymentID  string   `json:"PaymentId"`
	PaidAmount *float64 `json:"PaidAmount,omitempty"`
}

// completePaymentResponse represents the response when completing a payment.
type completePaymentResponse struct {
	PaymentID *string `json:"PaymentId"`
	Status    *string `json:"Status"`
}

// CompletePaymentPost handles /Payment/Complete requests for testing purposes.
// This endpoint simulates blockchain confirmation by marking a payment as successful.
// Request body: {"PaymentId": "pay-xxx", "PaidAmount": 95.50}
// PaidAmount is optional and overrides the postback's PaidAmount (simulates crypto-to-fiat conversion difference).
// Response: {"PaymentId": "pay-xxx", "Status": "success"}
func (s *Server) CompletePaymentPost(w http.ResponseWriter, r *http.Request) {
	var req completePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.completePaymentInternal(req.PaymentID, req.PaidAmount); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := completePaymentResponse{
		PaymentID: new(req.PaymentID),
		Status:    new("success"),
	}
	writeJSONResponse(w, response)
}

// buildPostbackNotification creates a postback notification for a completed payment.
// If the payment is linked to an invoice, the notification includes invoice data (OrderId, etc.)
// and uses WithDefaults(). For standalone payments, invoice fields are left empty.
func (s *Server) buildPostbackNotification(paymentID, txID string, payment *Payment) *PostbackNotification {
	// Check if payment is linked to an invoice
	invoiceID, hasInvoice := s.paymentToInvoice[paymentID]
	hasInvoice = hasInvoice && invoiceID != ""

	// Only apply defaults if we have a linked invoice
	var opts []PostbackOption
	if hasInvoice {
		opts = append(opts, WithDefaults())
	}
	notification := CreateTestPostback("test-merchant", opts...)

	// Populate from payment data
	notification.TransactionId = paymentID
	notification.BlockchainHash = txID
	notification.Status = 100
	if payment.AssetCode != nil {
		notification.Asset = *payment.AssetCode
	}

	// Populate from linked invoice data if available
	if hasInvoice {
		if invData, ok := s.invoices[invoiceID]; ok {
			req := invData.request
			if req.OrderId != nil {
				notification.OrderId = *req.OrderId
			}
			notification.OrderAmount = float64(req.OrderAmount)
			notification.Amount = float64(req.OrderAmount)
			if req.OrderCurrency != nil {
				notification.OrderCurrency = *req.OrderCurrency
			}
			if invData.paidAmount != nil {
				notification.PaidAmount = *invData.paidAmount
			} else {
				notification.PaidAmount = float64(req.OrderAmount)
			}
			if req.Subscription != nil {
				notification.SubscriptionId = *req.Subscription
			}
			if req.UserName != nil {
				notification.UserName = *req.UserName
			}
			if req.UserEmail != nil {
				notification.UserEmail = *req.UserEmail
			}
			notification.MerchantId = req.MerchantId
		}
	}

	return notification
}

// completePaymentInternal handles the internal logic for completing a payment.
// paidAmountOverride, if provided, overrides the PaidAmount in the postback notification.
func (s *Server) completePaymentInternal(paymentID string, paidAmountOverride *float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, exists := s.payments[paymentID]
	if !exists {
		return fmt.Errorf("payment not found: %s", paymentID)
	}

	txID := "0x" + generateHexString(64)
	payment.Status = &successStatus
	payment.Txid = &txID

	// Store PaidAmount override on the invoice data before building postback
	if paidAmountOverride != nil {
		if invoiceID, ok := s.paymentToInvoice[paymentID]; ok && invoiceID != "" {
			if invData, ok := s.invoices[invoiceID]; ok {
				invData.paidAmount = paidAmountOverride
			}
		}
	}

	if s.config.postbackMode == PostbackImmediate && s.config.postbackURL != "" && s.sender != nil {
		notification := s.buildPostbackNotification(paymentID, txID, payment)

		go func() {
			_ = s.sender.Send(s.config.postbackURL, notification)
		}()
	}

	return nil
}

// SendPostback sends a postback notification to the configured postback URL.
// Returns an error if the postback URL is not configured or sending fails.
func (s *Server) SendPostback(notification *PostbackNotification) error {
	if s.config.postbackURL == "" {
		return fmt.Errorf("postback URL not configured")
	}
	return s.sender.Send(s.config.postbackURL, notification)
}

// CompletePayment simulates completing a payment by updating its status to success
// and optionally sending a postback notification if configured.
// paidAmountOverride, if provided, overrides the PaidAmount in the postback notification.
func (s *Server) CompletePayment(paymentID string, paidAmountOverride ...float64) error {
	var override *float64
	if len(paidAmountOverride) > 0 {
		override = &paidAmountOverride[0]
	}
	return s.completePaymentInternal(paymentID, override)
}

// GetInvoice retrieves a stored invoice by ID.
func (s *Server) GetInvoice(invoiceID string) (*InvoiceResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invData, exists := s.invoices[invoiceID]
	if !exists {
		return nil, fmt.Errorf("invoice not found: %s", invoiceID)
	}
	return invData.response, nil
}

// GetPayment retrieves a stored payment by ID.
func (s *Server) GetPayment(paymentID string) (*Payment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payment, exists := s.payments[paymentID]
	if !exists {
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}
	return payment, nil
}

// writeJSONResponse writes a JSON response to the provided http.ResponseWriter.
func writeJSONResponse(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// PostbackURL returns the configured postback URL.
func (s *Server) PostbackURL() string {
	return s.config.postbackURL
}

// ResetPost handles /Reset requests for clearing all mock server state.
// This endpoint clears all invoices, payments, and resets nextID counters.
// Useful for resetting the mock server between test scenarios without restarting.
func (s *Server) ResetPost(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all state
	s.invoices = make(map[string]*invoiceData)
	s.payments = make(map[string]*Payment)
	s.paymentToInvoice = make(map[string]string)

	w.WriteHeader(http.StatusOK)
}

// loggingMiddleware wraps an http.Handler with zap logging.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.logger.Info("Handling HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
		)

		// Wrap the ResponseWriter to capture status code
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		s.logger.Info("HTTP request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status_code", wrapped.statusCode),
			zap.Duration("duration_ms", duration),
		)
	})
}

// responseWriterWrapper wraps http.ResponseWriter to capture the status code.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// withLogging wraps an http.HandlerFunc with logging.
func withLogging(handler http.HandlerFunc, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.Info("Handling HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
		)

		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		handler(wrapped, r)

		duration := time.Since(start)
		logger.Info("HTTP request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status_code", wrapped.statusCode),
			zap.Duration("duration_ms", duration),
		)
	}
}

// Handler creates an http.Handler that can be used with http.ListenAndServe.
// It registers all the Server endpoints using the internal client server generation.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	handler := internalclient.HandlerFromMux(s, mux)

	// Wrap logging middleware around the handler
	loggedHandler := s.loggingMiddleware(handler)

	// Register custom test endpoint for payment completion
	mux.HandleFunc("/Payment/Complete", withLogging(s.CompletePaymentPost, s.logger))
	// Register custom test endpoint for resetting server state
	mux.HandleFunc("/Reset", withLogging(s.ResetPost, s.logger))

	return loggedHandler
}

// Helper functions

func generateHexString(length int) string {
	chars := "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[i%16]
	}
	return string(result)
}

// generateTransactionID creates a realistic-looking transaction ID
// similar to real blockchain transaction IDs (e.g., "rxoI1U24RCFUvS")
// Uses URL-safe base64 alphabet to avoid special characters
func generateTransactionID() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const length = 15

	result := make([]byte, length)
	if _, err := rand.Read(result); err != nil {
		// Fallback to deterministic values if crypto/rand fails
		// This shouldn't happen in practice
		for i := range result {
			result[i] = charset[i%len(charset)]
		}
		return string(result)
	}

	for i := range result {
		result[i] = charset[result[i]%byte(len(charset))]
	}
	return string(result)
}
