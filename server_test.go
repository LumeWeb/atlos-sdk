package atlos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewServer_ValidOptions(t *testing.T) {
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, err := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)

	if err != nil {
		t.Errorf("NewServer() should not error: %v", err)
	}
	if server == nil {
		t.Errorf("NewServer() should return non-nil server")
	}
	if server.config.postbackURL != postbackServer.URL {
		t.Errorf("NewServer() should set postbackURL correctly")
	}
	if server.config.apiSecret != "test-secret" {
		t.Errorf("NewServer() should set apiSecret correctly")
	}
}

func TestNewServer_RequiredSecret(t *testing.T) {
	_, err := NewServer()
	if err == nil || err.Error() != "shared secret is required" {
		t.Errorf("NewServer() should error when shared secret is missing")
	}
}

func TestWithPostbackURL(t *testing.T) {
	cfg := &serverConfig{}
	WithPostbackURL("https://example.com/postback")(cfg)
	if cfg.postbackURL != "https://example.com/postback" {
		t.Errorf("WithPostbackURL() should set postbackURL")
	}
}

func TestWithSharedSecret(t *testing.T) {
	cfg := &serverConfig{}
	WithSharedSecret("my-secret")(cfg)
	if cfg.apiSecret != "my-secret" {
		t.Errorf("WithSharedSecret() should set apiSecret")
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	cfg := &serverConfig{}
	WithHTTPClient(customClient)(cfg)
	if cfg.httpClient != customClient {
		t.Errorf("WithHTTPClient() should set httpClient")
	}
}

func TestWithPostbackMode(t *testing.T) {
	cfg := &serverConfig{}
	WithPostbackMode(PostbackImmediate)(cfg)
	if cfg.postbackMode != PostbackImmediate {
		t.Errorf("WithPostbackMode() should set postbackMode")
	}
}

func TestServer_AssetListPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Asset/List", nil)
	w := httptest.NewRecorder()

	server.AssetListPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AssetListPost() should return 200 status")
	}

	var response []Asset
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("AssetListPost() should return valid JSON: %v", err)
	}

	if len(response) == 0 {
		t.Errorf("AssetListPost() should return assets list")
	}
}

func TestServer_InvoiceCancelPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Invoice/Cancel", nil)
	w := httptest.NewRecorder()

	server.InvoiceCancelPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("InvoiceCancelPost() should return 200 status")
	}
}

func TestServer_InvoiceCreatePost_Valid(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"MerchantId":"test-merchant","OrderAmount":100.0,"PostbackUrl":"https://example.com/callback"}`
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	server.InvoiceCreatePost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("InvoiceCreatePost() should return 200 status, got %d", w.Code)
	}

	var response InvoiceResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("InvoiceCreatePost() should return valid JSON: %v", err)
	}

	if response.Id == nil || *response.Id == "" {
		t.Errorf("InvoiceCreatePost() should include invoice ID")
	}

	if response.PaymentLink == nil || *response.PaymentLink == "" {
		t.Errorf("InvoiceCreatePost() should include payment link")
	}
}

func TestServer_InvoiceCreatePost_InvalidJSON(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader([]byte("invalid-json")))
	w := httptest.NewRecorder()

	server.InvoiceCreatePost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("InvoiceCreatePost() should return 400 status for invalid JSON")
	}
}

func TestServer_CreatePaymentPost_Valid(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"InvoiceId":"inv-1","AssetCode":"usdc","BlockchainCode":1}`
	req := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	server.CreatePaymentPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CreatePaymentPost() should return 200 status")
	}

	var response Payment
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("CreatePaymentPost() should return valid JSON: %v", err)
	}

	if response.Id == nil || *response.Id == "" {
		t.Errorf("CreatePaymentPost() should include payment ID")
	}

	if response.RecipientAddress == nil || *response.RecipientAddress == "" {
		t.Errorf("CreatePaymentPost() should include recipient address")
	}
}

func TestServer_CreatePaymentPost_InvalidJSON(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte("invalid-json")))
	w := httptest.NewRecorder()

	server.CreatePaymentPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePaymentPost() should return 400 status for invalid JSON")
	}
}

func TestServer_PaymentGetPost_Existing(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	createBody := `{"InvoiceId":"inv-1","AssetCode":"usdc","BlockchainCode":1}`
	createReq := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte(createBody)))
	createW := httptest.NewRecorder()
	server.CreatePaymentPost(createW, createReq)

	var createResp Payment
	json.NewDecoder(createW.Body).Decode(&createResp)

	getBody := `{"PaymentId":"` + *createResp.Id + `"}`
	req := httptest.NewRequest("POST", "/Payment/Get", bytes.NewReader([]byte(getBody)))
	w := httptest.NewRecorder()

	server.PaymentGetPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PaymentGetPost() should return 200 status for existing payment")
	}

	var response Payment
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("PaymentGetPost() should return valid JSON: %v", err)
	}

	if response.Id == nil || *response.Id != *createResp.Id {
		t.Errorf("PaymentGetPost() should return correct payment ID")
	}
}

func TestServer_PaymentGetPost_NotFound(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"PaymentId":"non-existent"}`
	req := httptest.NewRequest("POST", "/Payment/Get", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	server.PaymentGetPost(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("PaymentGetPost() should return 404 status for non-existent payment")
	}
}

func TestServer_PaymentGetPost_InvalidJSON(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Payment/Get", bytes.NewReader([]byte("invalid-json")))
	w := httptest.NewRecorder()

	server.PaymentGetPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("PaymentGetPost() should return 400 status for invalid JSON")
	}
}

func TestServer_PaymentWebSocketPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Payment/WebSocket", nil)
	w := httptest.NewRecorder()

	server.PaymentWebSocketPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PaymentWebSocketPost() should return 200 status")
	}
}

func TestServer_CancelPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Subscription/Cancel", nil)
	w := httptest.NewRecorder()

	server.CancelPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CancelPost() should return 200 status")
	}
}

func TestServer_FindByHashPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"MerchantId":"test-merchant","BlockchainHash":"0x123"}`
	req := httptest.NewRequest("POST", "/Transaction/FindByHash", strings.NewReader(body))
	w := httptest.NewRecorder()

	server.FindByHashPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("FindByHashPost() should return 200 status")
	}
}

func TestServer_TransactionListPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"MerchantId":"test-merchant"}`
	req := httptest.NewRequest("POST", "/Transaction/List", strings.NewReader(body))
	w := httptest.NewRecorder()

	server.TransactionListPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("TransactionListPost() should return 200 status")
	}
}

func TestServer_CancelPayoutPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Wallet/CancelPayout", nil)
	w := httptest.NewRecorder()

	server.CancelPayoutPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CancelPayoutPost() should return 200 status")
	}
}

func TestServer_SendTokenPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"MerchantId":"test-merchant","RecipientAddress":"0x1234567890","AssetCode":"BTC","BlockchainCode":"BTC"}`
	req := httptest.NewRequest("POST", "/Wallet/SendToken", strings.NewReader(body))
	w := httptest.NewRecorder()

	server.SendTokenPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SendTokenPost() should return 200 status")
	}
}

func TestServer_CompletePaymentPost_Valid(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	createBody := `{"InvoiceId":"inv-1","AssetCode":"usdc","BlockchainCode":1}`
	createReq := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte(createBody)))
	createW := httptest.NewRecorder()
	server.CreatePaymentPost(createW, createReq)

	var createResp Payment
	json.NewDecoder(createW.Body).Decode(&createResp)

	completeBody := `{"PaymentId":"` + *createResp.Id + `"}`
	req := httptest.NewRequest("POST", "/Payment/Complete", bytes.NewReader([]byte(completeBody)))
	w := httptest.NewRecorder()

	server.CompletePaymentPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CompletePaymentPost() should return 200 status")
	}

	var response completePaymentResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("CompletePaymentPost() should return valid JSON: %v", err)
	}

	if response.PaymentID == nil || *response.PaymentID != *createResp.Id {
		t.Errorf("CompletePaymentPost() should return correct payment ID")
	}

	if response.Status == nil || *response.Status != "success" {
		t.Errorf("CompletePaymentPost() should return success status")
	}
}

func TestServer_CompletePaymentPost_NotFound(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"PaymentId":"non-existent"}`
	req := httptest.NewRequest("POST", "/Payment/Complete", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	server.CompletePaymentPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CompletePaymentPost() should return 400 status for non-existent payment")
	}
}

func TestServer_CompletePaymentPost_InvalidJSON(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Payment/Complete", bytes.NewReader([]byte("invalid-json")))
	w := httptest.NewRecorder()

	server.CompletePaymentPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CompletePaymentPost() should return 400 status for invalid JSON")
	}
}

func TestServer_GetInvoice_Existing(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	body := `{"MerchantId":"test-merchant","OrderAmount":100.0}`
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var response InvoiceResponse
	json.NewDecoder(w.Body).Decode(&response)

	invoice, err := server.GetInvoice(*response.Id)
	if err != nil {
		t.Errorf("GetInvoice() should not error: %v", err)
	}
	if invoice == nil {
		t.Errorf("GetInvoice() should return invoice")
	}
	if invoice.Id == nil || *invoice.Id != *response.Id {
		t.Errorf("GetInvoice() should return correct invoice ID")
	}
	if invoice.PaymentLink == nil {
		t.Errorf("GetInvoice() should have payment link")
	}
}

func TestServer_GetInvoice_NotFound(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	_, err := server.GetInvoice("non-existent")
	if err == nil {
		t.Errorf("GetInvoice() should error for non-existent invoice")
	}
}

func TestServer_GetPayment_Existing(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	createBody := `{"InvoiceId":"inv-1","AssetCode":"usdc","BlockchainCode":1}`
	createReq := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte(createBody)))
	createW := httptest.NewRecorder()
	server.CreatePaymentPost(createW, createReq)

	var createResp Payment
	json.NewDecoder(createW.Body).Decode(&createResp)

	payment, err := server.GetPayment(*createResp.Id)
	if err != nil {
		t.Errorf("GetPayment() should not error: %v", err)
	}
	if payment == nil {
		t.Errorf("GetPayment() should return payment")
	}
	if payment.Id == nil || *payment.Id != *createResp.Id {
		t.Errorf("GetPayment() should return correct payment ID")
	}
}

func TestServer_GetPayment_NotFound(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	_, err := server.GetPayment("non-existent")
	if err == nil {
		t.Errorf("GetPayment() should error for non-existent payment")
	}
}

func TestServer_SendPostback_Configured(t *testing.T) {
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)

	notification := CreateTestPostback("test-merchant")
	err := server.SendPostback(notification)
	if err != nil {
		t.Errorf("SendPostback() should succeed when configured: %v", err)
	}
}

func TestServer_SendPostback_NotConfigured(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	notification := CreateTestPostback("test-merchant")
	err := server.SendPostback(notification)
	if err == nil || err.Error() != "postback URL not configured" {
		t.Errorf("SendPostback() should error when postback URL not configured")
	}
}

func TestServer_PostbackURL(t *testing.T) {
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)

	if server.PostbackURL() != postbackServer.URL {
		t.Errorf("PostbackURL() should return configured URL")
	}
}

func TestServer_Handler(t *testing.T) {
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)

	handler := server.Handler()
	if handler == nil {
		t.Errorf("Handler() should return non-nil handler")
	}

	req := httptest.NewRequest("POST", "/Asset/List", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Handler() should handle requests")
	}
}

func TestServer_CompletePayment_Existing(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	createBody := `{"InvoiceId":"inv-1","AssetCode":"usdc","BlockchainCode":1}`
	createReq := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader([]byte(createBody)))
	createW := httptest.NewRecorder()
	server.CreatePaymentPost(createW, createReq)

	var createResp Payment
	json.NewDecoder(createW.Body).Decode(&createResp)

	err := server.CompletePayment(*createResp.Id)
	if err != nil {
		t.Errorf("CompletePayment() should not error: %v", err)
	}

	payment, _ := server.GetPayment(*createResp.Id)
	if payment.Status == nil || *payment.Status != "success" {
		t.Errorf("CompletePayment() should update payment status to success")
	}
	if payment.Txid == nil || *payment.Txid == "" {
		t.Errorf("CompletePayment() should set Txid")
	}
}

func TestServer_CompletePayment_NotFound(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	err := server.CompletePayment("non-existent")
	if err == nil {
		t.Errorf("CompletePayment() should error for non-existent payment")
	}
}

func TestServer_CompletePayment_PopulatesPostbackFromInvoice(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	// Create invoice with all the invoice-specific fields
	merchantId := "test-merchant"
	orderId := "order-123"
	orderAmount := float32(99.99)
	orderCurrency := "USD"
	userName := "John Doe"
	userEmail := "john@example.com"

	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId:    merchantId,
		OrderId:       &orderId,
		OrderAmount:   orderAmount,
		OrderCurrency: &orderCurrency,
		UserName:      &userName,
		UserEmail:     &userEmail,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var invoiceResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&invoiceResp)
	invoiceID := *invoiceResp.Id

	// Create payment linked to the invoice
	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "BTC",
		BlockchainCode: bcCode,
		InvoiceId:      invoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	var createResp Payment
	json.NewDecoder(w2.Body).Decode(&createResp)
	paymentID := *createResp.Id

	// Complete payment - this should trigger postback with invoice data
	err := server.CompletePayment(paymentID)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	// Wait for postback with timeout
	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	// Verify all invoice fields are populated in the postback
	if notification.OrderId != orderId {
		t.Errorf("Postback OrderId = %q, want %q", notification.OrderId, orderId)
	}
	if notification.OrderAmount != float64(orderAmount) {
		t.Errorf("Postback OrderAmount = %f, want %f", notification.OrderAmount, orderAmount)
	}
	if notification.OrderCurrency != orderCurrency {
		t.Errorf("Postback OrderCurrency = %q, want %q", notification.OrderCurrency, orderCurrency)
	}
	if notification.PaidAmount != float64(orderAmount) {
		t.Errorf("Postback PaidAmount = %f, want %f", notification.PaidAmount, orderAmount)
	}
	if notification.UserName != userName {
		t.Errorf("Postback UserName = %q, want %q", notification.UserName, userName)
	}
	if notification.UserEmail != userEmail {
		t.Errorf("Postback UserEmail = %q, want %q", notification.UserEmail, userEmail)
	}
	if notification.MerchantId != merchantId {
		t.Errorf("Postback MerchantId = %q, want %q", notification.MerchantId, merchantId)
	}
	if notification.Asset != "BTC" {
		t.Errorf("Postback Asset = %q, want %q", notification.Asset, "BTC")
	}
	if notification.TransactionId != paymentID {
		t.Errorf("Postback TransactionId = %q, want %q", notification.TransactionId, paymentID)
	}
	if notification.Status != 100 {
		t.Errorf("Postback Status = %d, want %d", notification.Status, 100)
	}
	if notification.BlockchainHash == "" {
		t.Errorf("Postback BlockchainHash should be set")
	}
}

func TestServer_CompletePayment_StandalonePayment_NoInvoiceData(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	// Create a standalone payment (no invoice ID)
	// Using empty or invalid invoice ID
	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "ETH",
		BlockchainCode: bcCode,
		InvoiceId:      "", // No invoice ID
		IsEvm:          "1",
	}
	reqBody, _ := json.Marshal(createPaymentReq)
	req := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.CreatePaymentPost(w, req)

	var createResp Payment
	json.NewDecoder(w.Body).Decode(&createResp)
	paymentID := *createResp.Id

	// Complete payment - this should trigger postback without invoice data
	err := server.CompletePayment(paymentID)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	// Wait for postback with timeout
	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	// Verify payment data is populated
	if notification.TransactionId != paymentID {
		t.Errorf("Postback TransactionId = %q, want %q", notification.TransactionId, paymentID)
	}
	if notification.Asset != "ETH" {
		t.Errorf("Postback Asset = %q, want %q", notification.Asset, "ETH")
	}
	if notification.Status != 100 {
		t.Errorf("Postback Status = %d, want %d", notification.Status, 100)
	}
	if notification.BlockchainHash == "" {
		t.Errorf("Postback BlockchainHash should be set")
	}

	// Invoice-specific fields should be empty/zero
	if notification.OrderId != "" {
		t.Errorf("Postback OrderId should be empty for standalone payment, got %q", notification.OrderId)
	}
	if notification.OrderAmount != 0 {
		t.Errorf("Postback OrderAmount should be 0 for standalone payment, got %f", notification.OrderAmount)
	}
	if notification.OrderCurrency != "" {
		t.Errorf("Postback OrderCurrency should be empty for standalone payment, got %q", notification.OrderCurrency)
	}
	if notification.UserName != "" {
		t.Errorf("Postback UserName should be empty for standalone payment, got %q", notification.UserName)
	}
	if notification.UserEmail != "" {
		t.Errorf("Postback UserEmail should be empty for standalone payment, got %q", notification.UserEmail)
	}
}

func TestGenerateHexString_FixedLength(t *testing.T) {
	length := 40
	result := generateHexString(length)
	if len(result) != length {
		t.Errorf("generateHexString() should return string of length %d, got %d", length, len(result))
	}

	for _, char := range result {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			t.Errorf("generateHexString() should only contain hex characters")
		}
	}
}

func TestWriteJSONResponse_Valid(t *testing.T) {
	w := httptest.NewRecorder()

	response := map[string]string{"key": "value"}
	writeJSONResponse(w, response)

	if w.Code != http.StatusOK {
		t.Errorf("writeJSONResponse() should set status 200")
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("writeJSONResponse() should set Content-Type header")
	}

	var decoded map[string]string
	if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
		t.Errorf("writeJSONResponse() should write valid JSON")
	}
	if decoded["key"] != "value" {
		t.Errorf("writeJSONResponse() should write correct data")
	}
}

func TestServer_ResetPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	// Create some test data first
	server.invoices["test-id-1"] = &invoiceData{
		response: &InvoiceResponse{Id: new("1")},
		request:  InvoiceCreatePostRequest{},
	}
	server.payments["test-id-2"] = &Payment{Id: new("1")}

	// Verify data exists
	if len(server.invoices) != 1 {
		t.Errorf("Expected 1 invoice before reset")
	}
	if len(server.payments) != 1 {
		t.Errorf("Expected 1 payment before reset")
	}

	// Call reset endpoint
	req := httptest.NewRequest("POST", "/Reset", nil)
	w := httptest.NewRecorder()
	server.ResetPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ResetPost() should return 200 status")
	}

	// Verify all state is cleared
	if len(server.invoices) != 0 {
		t.Errorf("ResetPost() should clear all invoices")
	}
	if len(server.payments) != 0 {
		t.Errorf("ResetPost() should clear all payments")
	}
}


func TestServer_ResetPost_AfterCreateOperations(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	// Create invoice
	merchantId := "test-merchant"
	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId: merchantId,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var createResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	firstInvoiceID := *createResp.Id

	// Create payment
	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "BTC",
		BlockchainCode: bcCode,
		InvoiceId:      firstInvoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	// Call reset
	req3 := httptest.NewRequest("POST", "/Reset", nil)
	w3 := httptest.NewRecorder()
	server.ResetPost(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("ResetPost() should return 200 status")
	}

	// Create new invoice after reset - should start from inv-1 if counters were reset
	createInvoiceReq2 := InvoiceCreatePostRequest{
		MerchantId: merchantId,
	}
	reqBody4, _ := json.Marshal(createInvoiceReq2)
	req4 := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody4))
	w4 := httptest.NewRecorder()
	server.InvoiceCreatePost(w4, req4)

	var createResp4 InvoiceResponse
	json.NewDecoder(w4.Body).Decode(&createResp4)
	newInvoiceID := *createResp4.Id

	// Verify that after reset, only the newly created invoice exists
	// by checking the count
	if len(server.invoices) != 1 {
		t.Errorf("After reset and creating 1 invoice, should have exactly 1 invoice in storage, got %d", len(server.invoices))
	}
	if len(server.payments) != 0 {
		t.Errorf("After reset, payments map should be empty, got %d items", len(server.payments))
	}

	// Verify the only invoice is the new one, not the old one
	newInvoice, err := server.GetInvoice(newInvoiceID)
	if err != nil {
		t.Errorf("Should be able to retrieve new invoice after reset")
	}
	if newInvoice == nil {
		t.Errorf("New invoice should exist")
	}
}

func TestServer_Handler_RegistersResetEndpoint(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))
	handler := server.Handler()

	// Create some test data
	server.invoices["test"] = &invoiceData{
		response: &InvoiceResponse{Id: new("test")},
		request:  InvoiceCreatePostRequest{},
	}

	// Call /Reset through handler
	req := httptest.NewRequest("POST", "/Reset", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Handler should route /Reset to ResetPost")
	}

	// Verify state was cleared
	if len(server.invoices) != 0 {
		t.Errorf("Handler's /Reset endpoint should clear state")
	}
}

func TestServer_CompletePayment_PaidAmountDiffersFromOrderAmount(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	merchantId := "test-merchant"
	orderAmount := float32(99.99)
	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId:  merchantId,
		OrderAmount: orderAmount,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var invoiceResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&invoiceResp)
	invoiceID := *invoiceResp.Id

	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "USDC",
		BlockchainCode: bcCode,
		InvoiceId:      invoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	var createResp Payment
	json.NewDecoder(w2.Body).Decode(&createResp)
	paymentID := *createResp.Id

	paidAmount := 95.50
	err := server.CompletePayment(paymentID, paidAmount)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	if notification.OrderAmount != float64(orderAmount) {
		t.Errorf("Postback OrderAmount = %f, want %f", notification.OrderAmount, orderAmount)
	}
	if notification.PaidAmount != paidAmount {
		t.Errorf("Postback PaidAmount = %f, want %f", notification.PaidAmount, paidAmount)
	}
	if notification.PaidAmount == notification.OrderAmount {
		t.Errorf("PaidAmount should differ from OrderAmount when override is provided")
	}
}

func TestServer_CompletePayment_SubscriptionIdInPostback(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	merchantId := "test-merchant"
	subId := "sub-abc123"
	orderAmount := float32(50.0)
	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId:  merchantId,
		OrderAmount: orderAmount,
		Subscription: &subId,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var invoiceResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&invoiceResp)
	invoiceID := *invoiceResp.Id

	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "USDC",
		BlockchainCode: bcCode,
		InvoiceId:      invoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	var createResp Payment
	json.NewDecoder(w2.Body).Decode(&createResp)
	paymentID := *createResp.Id

	err := server.CompletePayment(paymentID)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	if notification.SubscriptionId != subId {
		t.Errorf("Postback SubscriptionId = %q, want %q", notification.SubscriptionId, subId)
	}
}

func TestServer_CompletePayment_PaidAmountDefaultsToOrderAmount(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	merchantId := "test-merchant"
	orderAmount := float32(50.0)
	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId:  merchantId,
		OrderAmount: orderAmount,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var invoiceResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&invoiceResp)
	invoiceID := *invoiceResp.Id

	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "USDC",
		BlockchainCode: bcCode,
		InvoiceId:      invoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	var createResp Payment
	json.NewDecoder(w2.Body).Decode(&createResp)
	paymentID := *createResp.Id

	err := server.CompletePayment(paymentID)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	if notification.PaidAmount != float64(orderAmount) {
		t.Errorf("Postback PaidAmount = %f, want %f (should default to OrderAmount)", notification.PaidAmount, orderAmount)
	}
	if notification.PaidAmount != notification.OrderAmount {
		t.Errorf("PaidAmount should equal OrderAmount when no override is provided")
	}
}

func TestServer_CompletePayment_FeeNotSubtractedFromPaidAmount(t *testing.T) {
	receivedNotification := make(chan *PostbackNotification, 1)
	postbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pn PostbackNotification
		if err := json.NewDecoder(r.Body).Decode(&pn); err == nil {
			receivedNotification <- &pn
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer postbackServer.Close()

	server, _ := NewServer(
		WithPostbackURL(postbackServer.URL),
		WithSharedSecret("test-secret"),
	)
	server.config.postbackMode = PostbackImmediate

	merchantId := "test-merchant"
	orderAmount := float32(100.0)
	createInvoiceReq := InvoiceCreatePostRequest{
		MerchantId:  merchantId,
		OrderAmount: orderAmount,
	}
	reqBody, _ := json.Marshal(createInvoiceReq)
	req := httptest.NewRequest("POST", "/Invoice/Create", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	server.InvoiceCreatePost(w, req)

	var invoiceResp InvoiceResponse
	json.NewDecoder(w.Body).Decode(&invoiceResp)
	invoiceID := *invoiceResp.Id

	bcCode := float32(1)
	createPaymentReq := CreatePaymentPostRequest{
		AssetCode:      "USDC",
		BlockchainCode: bcCode,
		InvoiceId:      invoiceID,
		IsEvm:          "1",
	}
	reqBody2, _ := json.Marshal(createPaymentReq)
	req2 := httptest.NewRequest("POST", "/Payment/Create", bytes.NewReader(reqBody2))
	w2 := httptest.NewRecorder()
	server.CreatePaymentPost(w2, req2)

	var createResp Payment
	json.NewDecoder(w2.Body).Decode(&createResp)
	paymentID := *createResp.Id

	err := server.CompletePayment(paymentID)
	if err != nil {
		t.Fatalf("CompletePayment() should not error: %v", err)
	}

	var notification *PostbackNotification
	select {
	case notification = <-receivedNotification:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Postback notification was not received")
	}

	if notification.PaidAmount != 100.0 {
		t.Errorf("Postback PaidAmount = %f, want 100.0 (Fee should NOT reduce PaidAmount)", notification.PaidAmount)
	}
	if notification.Fee <= 0 {
		t.Errorf("Postback Fee should be a positive value, got %f", notification.Fee)
	}
}

