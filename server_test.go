package atlos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

	var response AssetListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("AssetListPost() should return valid JSON: %v", err)
	}

	if len(response.Assets) == 0 {
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

	req := httptest.NewRequest("POST", "/Transaction/FindByHash", nil)
	w := httptest.NewRecorder()

	server.FindByHashPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("FindByHashPost() should return 200 status")
	}
}

func TestServer_TransactionListPost(t *testing.T) {
	server, _ := NewServer(WithSharedSecret("test-secret"))

	req := httptest.NewRequest("POST", "/Transaction/List", nil)
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

	req := httptest.NewRequest("POST", "/Wallet/SendToken", nil)
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


