// Package atlos provides a Go SDK for the ATLOS cryptocurrency payment gateway.
package atlos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	internalclient "go.lumeweb.com/atlos-sdk/internal/client"
)

var (
	pendingStatus = "pending"
	successStatus = "success"
	zeroAmountStr = "0"
	defaultFeeStr = "020000000"
)

// Server represents a lightweight server implementation that extends the internal ServerInterface.
// It implements all methods defined in client.ServerInterface and provides postback notification capability.
// Use client.Handler() or client.HandlerFromMux() to create an HTTP handler from this server.
type Server struct {
	config    serverConfig
	sender    *PostbackSender

	// Test data storage
	mu         sync.RWMutex
	invoices   map[string]*internalclient.InvoiceResponse
	payments   map[string]*internalclient.Payment
	nextIDs    struct {
		invoice  int
		payment  int
	}
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
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.apiSecret == "" {
		return nil, fmt.Errorf("shared secret is required")
	}

	sender := &PostbackSender{
		apiSecret: cfg.apiSecret,
		client:    cfg.httpClient,
	}

	return &Server{
		config:   cfg,
		sender:   sender,
		invoices: make(map[string]*internalclient.InvoiceResponse),
		payments: make(map[string]*internalclient.Payment),
	}, nil
}

// AssetListPost handles /Asset/List requests and returns available assets and blockchains.
func (s *Server) AssetListPost(w http.ResponseWriter, r *http.Request) {
	response := AssetListResponse{
		Assets: AssetsFromAtlas,
	}
	writeJSONResponse(w, response)
}

// InvoiceCancelPost handles /Invoice/Cancel requests.
func (s *Server) InvoiceCancelPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// InvoiceCreatePost handles /Invoice/Create requests and generates a new invoice.
func (s *Server) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.InvoiceCreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextIDs.invoice++
	invoiceID := fmt.Sprintf("inv-%d", s.nextIDs.invoice)

	invoice := &internalclient.InvoiceResponse{
		Id:          &invoiceID,
		PaymentLink: func() *string { s := fmt.Sprintf("https://atlos.com/payment/%s", invoiceID); return &s }(),
	}
	s.invoices[invoiceID] = invoice

	response := *invoice
	writeJSONResponse(w, response)
	writeJSONResponse(w, response)
}

// CreatePaymentPost handles /Payment/Create requests and generates a wallet address.
func (s *Server) CreatePaymentPost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.CreatePaymentPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextIDs.payment++
	paymentID := fmt.Sprintf("pay-%d", s.nextIDs.payment)

	recipientAddr := fmt.Sprintf("0x%s", generateHexString(40))

	payment := &internalclient.Payment{
		Id:               &paymentID,
		Amount:           &zeroAmountStr,
		AssetCode:        &req.AssetCode,
		BlockchainCode:   &req.BlockchainCode,
		RecipientAddress: &recipientAddr,
		Fee:              &defaultFeeStr,
		Status:           &pendingStatus,
	}
	s.payments[paymentID] = payment

	response := *payment
	writeJSONResponse(w, response)
}

// PaymentGetPost handles /Payment/Get requests and returns payment status.
func (s *Server) PaymentGetPost(w http.ResponseWriter, r *http.Request) {
	var req internalclient.PaymentGetPostRequest
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
	w.WriteHeader(http.StatusOK)
}

// TransactionListPost handles /Transaction/List requests.
func (s *Server) TransactionListPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// CancelPayoutPost handles /Wallet/CancelPayout requests.
func (s *Server) CancelPayoutPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// SendTokenPost handles /Wallet/SendToken requests.
func (s *Server) SendTokenPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// completePaymentRequest represents a request to manually complete a payment.
type completePaymentRequest struct {
	PaymentID string `json:"PaymentId"`
}

// completePaymentResponse represents the response when completing a payment.
type completePaymentResponse struct {
	PaymentID *string `json:"PaymentId"`
	Status    *string `json:"Status"`
}

// CompletePaymentPost handles /Payment/Complete requests for testing purposes.
// This endpoint simulates blockchain confirmation by marking a payment as successful.
// Request body: {"PaymentId": "pay-xxx"}
// Response: {"PaymentId": "pay-xxx", "Status": "success"}
func (s *Server) CompletePaymentPost(w http.ResponseWriter, r *http.Request) {
	var req completePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.completePaymentInternal(req.PaymentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := completePaymentResponse{
		PaymentID: new(req.PaymentID),
		Status:    new("success"),
	}
	writeJSONResponse(w, response)
}

// completePaymentInternal handles the internal logic for completing a payment.
func (s *Server) completePaymentInternal(paymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, exists := s.payments[paymentID]
	if !exists {
		return fmt.Errorf("payment not found: %s", paymentID)
	}

	txID := "0x" + generateHexString(64)
	payment.Status = &successStatus
	payment.Txid = &txID

	if s.config.postbackMode == PostbackImmediate && s.config.postbackURL != "" && s.sender != nil {
		notification := CreateTestPostback("test-merchant")
		notification.TransactionId = paymentID
		notification.BlockchainHash = txID
		notification.Status = 100

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
func (s *Server) CompletePayment(paymentID string) error {
	return s.completePaymentInternal(paymentID)
}

// GetInvoice retrieves a stored invoice by ID.
func (s *Server) GetInvoice(invoiceID string) (*internalclient.InvoiceResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invoice, exists := s.invoices[invoiceID]
	if !exists {
		return nil, fmt.Errorf("invoice not found: %s", invoiceID)
	}
	return invoice, nil
}

// GetPayment retrieves a stored payment by ID.
func (s *Server) GetPayment(paymentID string) (*internalclient.Payment, error) {
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

// Handler creates an http.Handler that can be used with http.ListenAndServe.
// It registers all the Server endpoints using the internal client server generation.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	handler := internalclient.HandlerFromMux(s, mux)

	// Register custom test endpoint for payment completion
	mux.HandleFunc("/Payment/Complete", s.CompletePaymentPost)

	return handler
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
