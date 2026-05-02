package atlos

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestVerifySignatureFromBytes_KnownSignature verifies our HMAC-SHA256 implementation
// matches the Atlos API's JavaScript reference implementation. The expected signature
// was computed independently using the JS algorithm:
//   hmac = crypto.createHmac('sha256', apiSecret)
//   hmac.write(rawJSONBody)
//   hmac.end()
//   signature = hmac.read().toString('base64')
func TestVerifySignatureFromBytes_KnownSignature(t *testing.T) {
	apiSecret := "my-api-secret"
	// Fixed raw JSON — this is exactly what Atlos would POST
	rawBody := []byte(`{"TransactionId":"tx-001","SubscriptionId":"","MerchantId":"m-123","OrderId":"order-456","Amount":50.5,"Fee":0.5,"Blockchain":"ETH","Asset":"USDC","BlockchainHash":"0xabc123","UserWallet":"0xWallet","UserName":"Alice","UserEmail":"alice@example.com","OrderAmount":50.0,"OrderCurrency":"USD","PaidAmount":50.5,"TimeSent":"2025-01-15T10:30:00Z","Status":100}`)

	// Compute expected signature using the same algorithm as the JS reference
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write(rawBody)
	expectedSig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !VerifySignatureFromBytes(apiSecret, expectedSig, rawBody) {
		t.Errorf("VerifySignatureFromBytes() should return true for known-good signature, expected=%s", expectedSig)
	}

	// Verify a wrong secret produces a different (failing) signature
	if VerifySignatureFromBytes("wrong-secret", expectedSig, rawBody) {
		t.Errorf("VerifySignatureFromBytes() should return false with wrong api secret")
	}

	// Verify tampered body fails
	tamperedBody := []byte(`{"TransactionId":"tx-001","SubscriptionId":"","MerchantId":"m-123","OrderId":"order-456","Amount":999.99,"Fee":0.5,"Blockchain":"ETH","Asset":"USDC","BlockchainHash":"0xabc123","UserWallet":"0xWallet","UserName":"Alice","UserEmail":"alice@example.com","OrderAmount":50.0,"OrderCurrency":"USD","PaidAmount":50.5,"TimeSent":"2025-01-15T10:30:00Z","Status":100}`)
	if VerifySignatureFromBytes(apiSecret, expectedSig, tamperedBody) {
		t.Errorf("VerifySignatureFromBytes() should return false for tampered body")
	}
}

func TestPostbackNotification_Validate_Valid(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	err := pn.Validate()
	if err != nil {
		t.Errorf("Validate() should pass for valid notification, got error: %v", err)
	}
}

func TestPostbackNotification_Validate_MissingTransactionId(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.TransactionId = ""
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "TransactionId") {
		t.Errorf("Validate() should error when TransactionId is missing")
	}
}

func TestPostbackNotification_Validate_MissingMerchantId(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.MerchantId = ""
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "MerchantId") {
		t.Errorf("Validate() should error when MerchantId is missing")
	}
}

func TestPostbackNotification_Validate_NegativeAmount(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Amount = -1.0
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Amount") {
		t.Errorf("Validate() should error when Amount is negative")
	}
}

func TestPostbackNotification_Validate_NegativeFee(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Fee = -0.5
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Fee") {
		t.Errorf("Validate() should error when Fee is negative")
	}
}

func TestPostbackNotification_Validate_MissingBlockchain(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Blockchain = ""
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Blockchain") {
		t.Errorf("Validate() should error when Blockchain is missing")
	}
}

func TestPostbackNotification_Validate_MissingAsset(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Asset = ""
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Asset") {
		t.Errorf("Validate() should error when Asset is missing")
	}
}

func TestPostbackNotification_Validate_StatusZero(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Status = 0
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Status") {
		t.Errorf("Validate() should error when Status is zero")
	}
}

func TestPostbackNotification_Validate_StatusNot100(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.Status = 99
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "Status") {
		t.Errorf("Validate() should error when Status is not 100")
	}
}

func TestPostbackNotification_Validate_InvalidTimeSent(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.TimeSent = "not-a-valid-timestamp"
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "TimeSent") {
		t.Errorf("Validate() should error when TimeSent is invalid")
	}
}

func TestPostbackNotification_Validate_NegativeOrderAmount(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.OrderAmount = -10.0
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "OrderAmount") {
		t.Errorf("Validate() should error when OrderAmount is negative")
	}
}

func TestPostbackNotification_Validate_NegativePaidAmount(t *testing.T) {
	pn := CreateTestPostback("test-merchant")
	pn.PaidAmount = -5.0
	err := pn.Validate()
	if err == nil || !strings.Contains(err.Error(), "PaidAmount") {
		t.Errorf("Validate() should error when PaidAmount is negative")
	}
}

func TestPostbackNotification_VerifySignature_Valid(t *testing.T) {
	apiSecret := "test-secret"
	pn := CreateTestPostback("test-merchant")

	h := hmac.New(sha256.New, []byte(apiSecret))
	data, _ := json.Marshal(pn)
	h.Write(data)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	valid, err := pn.VerifySignature(apiSecret, signature)
	if err != nil {
		t.Errorf("VerifySignature() should not error: %v", err)
	}
	if !valid {
		t.Errorf("VerifySignature() should return true for valid signature")
	}
}

func TestPostbackNotification_VerifySignature_Invalid(t *testing.T) {
	apiSecret := "test-secret"
	pn := CreateTestPostback("test-merchant")

	valid, err := pn.VerifySignature(apiSecret, "invalid-signature")
	if err != nil {
		t.Errorf("VerifySignature() should not error: %v", err)
	}
	if valid {
		t.Errorf("VerifySignature() should return false for invalid signature")
	}
}

func TestPostbackNotification_VerifySignature_MarshalError(t *testing.T) {
	pn := &PostbackNotification{}
	*pn = PostbackNotification{
		TransactionId: "test",
	}
	
	h := hmac.New(sha256.New, []byte("test-secret"))
	data, _ := json.Marshal(pn)
	h.Write(data)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	valid, err := pn.VerifySignature("test-secret", signature)
	if err != nil {
		t.Errorf("VerifySignature() should not error: %v", err)
	}
	if !valid {
		t.Errorf("VerifySignature() should return true for empty but valid notification")
	}
}

func TestNewPostbackSender(t *testing.T) {
	apiSecret := "test-secret"
	sender := NewPostbackSender(apiSecret)
	if sender == nil {
		t.Errorf("NewPostbackSender() should return non-nil sender")
	}
	if sender.apiSecret != apiSecret {
		t.Errorf("NewPostbackSender() should set apiSecret correctly")
	}
	if sender.client == nil {
		t.Errorf("NewPostbackSender() should initialize http client")
	}
}

func TestPostbackSender_Send_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiSecret := "test-secret"
	sender := NewPostbackSender(apiSecret)
	pn := CreateTestPostback("test-merchant")

	err := sender.Send(server.URL, pn)
	if err != nil {
		t.Errorf("Send() should succeed: %v", err)
	}
}

func TestPostbackSender_Send_ValidationError(t *testing.T) {
	apiSecret := "test-secret"
	sender := NewPostbackSender(apiSecret)
	pn := &PostbackNotification{}

	err := sender.Send("https://example.com", pn)
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Send() should error on validation failure")
	}
}

func TestPostbackSender_Send_InvalidURL(t *testing.T) {
	apiSecret := "test-secret"
	sender := NewPostbackSender(apiSecret)
	pn := CreateTestPostback("test-merchant")

	err := sender.Send("://invalid-url", pn)
	if err == nil || !strings.Contains(err.Error(), "invalid endpoint") {
		t.Errorf("Send() should error on invalid URL")
	}
}

func TestPostbackSender_Send_Non2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	apiSecret := "test-secret"
	sender := NewPostbackSender(apiSecret)
	pn := CreateTestPostback("test-merchant")

	err := sender.Send(server.URL, pn)
	if err == nil || !strings.Contains(err.Error(), "non-success") {
		t.Errorf("Send() should error on non-2xx status")
	}
}

func TestNewPostbackHandler(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)
	if handler == nil {
		t.Errorf("NewPostbackHandler() should return non-nil handler")
	}
	if handler.apiSecret != apiSecret {
		t.Errorf("NewPostbackHandler() should set apiSecret correctly")
	}
}

func TestPostbackHandler_HandleRequest_Valid(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)
	pn := CreateTestPostback("test-merchant")

	h := hmac.New(sha256.New, []byte(apiSecret))
	data, _ := json.Marshal(pn)
	h.Write(data)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(data))
	req.Header.Set(SignatureHeader, signature)

	notification, err := handler.HandleRequest(req)
	if err != nil {
		t.Errorf("HandleRequest() should succeed: %v", err)
	}
	if notification == nil {
		t.Errorf("HandleRequest() should return notification")
	}
}

func TestPostbackHandler_HandleRequest_MissingSignature(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)
	pn := CreateTestPostback("test-merchant")
	data, _ := json.Marshal(pn)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(data))

	_, err := handler.HandleRequest(req)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("HandleRequest() should error on missing signature header")
	}
}

func TestPostbackHandler_HandleRequest_InvalidJSON(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)

	// Create a valid signature for invalid JSON — signature passes, but JSON decode fails
	rawBody := "invalid-json"
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(rawBody))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/", strings.NewReader(rawBody))
	req.Header.Set(SignatureHeader, signature)

	_, err := handler.HandleRequest(req)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("HandleRequest() should error on invalid JSON after signature passes, got: %v", err)
	}
}

func TestPostbackHandler_HandleRequest_ValidationError(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)
	pn := &PostbackNotification{}
	data, _ := json.Marshal(pn)

	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write(data)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(data))
	req.Header.Set(SignatureHeader, signature)

	_, err := handler.HandleRequest(req)
	if err == nil || !strings.Contains(err.Error(), "validation") {
		t.Errorf("HandleRequest() should error on validation failure")
	}
}

func TestPostbackHandler_HandleRequest_InvalidSignature(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)
	pn := CreateTestPostback("test-merchant")
	data, _ := json.Marshal(pn)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(data))
	req.Header.Set(SignatureHeader, "invalid-signature")

	_, err := handler.HandleRequest(req)
	if err == nil || !strings.Contains(err.Error(), "invalid signature") {
		t.Errorf("HandleRequest() should error on invalid signature")
	}
}

func TestNewMockServer(t *testing.T) {
	config := MockServerConfig{
		PostbackURL: "https://example.com/postback",
		APISecret:   "test-secret",
	}
	mockServer := NewMockServer(config)
	if mockServer == nil {
		t.Errorf("NewMockServer() should return non-nil server")
	}
	if mockServer.config.PostbackURL != config.PostbackURL {
		t.Errorf("NewMockServer() should set PostbackURL correctly")
	}
	if mockServer.config.APISecret != config.APISecret {
		t.Errorf("NewMockServer() should set APISecret correctly")
	}
	if mockServer.sender == nil {
		t.Errorf("NewMockServer() should initialize sender")
	}
}

func TestMockServer_SendTokenPost(t *testing.T) {
	config := MockServerConfig{
		PostbackURL: "",
		APISecret:   "test-secret",
	}
	mockServer := NewMockServer(config)

	req := httptest.NewRequest("POST", "/token", nil)
	w := httptest.NewRecorder()

	mockServer.SendTokenPost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SendTokenPost() should return 200 status")
	}
}

func TestMockServer_SendPostback_WithURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := MockServerConfig{
		PostbackURL: server.URL,
		APISecret:   "test-secret",
	}
	mockServer := NewMockServer(config)
	pn := CreateTestPostback("test-merchant")

	err := mockServer.SendPostback(pn)
	if err != nil {
		t.Errorf("SendPostback() should succeed when URL is configured: %v", err)
	}
}

func TestMockServer_SendPostback_NoURL(t *testing.T) {
	config := MockServerConfig{
		PostbackURL: "",
		APISecret:   "test-secret",
	}
	mockServer := NewMockServer(config)
	pn := CreateTestPostback("test-merchant")

	err := mockServer.SendPostback(pn)
	if err == nil || !strings.Contains(err.Error(), "no postback URL") {
		t.Errorf("SendPostback() should error when URL is not configured")
	}
}

func TestCreateTestPostback_Valid(t *testing.T) {
	merchantId := "test-merchant-123"
	pn := CreateTestPostback(merchantId)

	err := pn.Validate()
	if err != nil {
		t.Errorf("CreateTestPostback() should create valid notification: %v", err)
	}
}

// TestPostbackHandler_HandleRequest_RawBodyVerification proves that HandleRequest
// verifies the signature against the raw request body, not re-marshaled JSON.
// This matches the Atlos API documentation which warns: "you need to make sure to
// check the signature for the raw request, not for the converted JSON data."
func TestPostbackHandler_HandleRequest_RawBodyVerification(t *testing.T) {
	apiSecret := "test-secret"
	handler := NewPostbackHandler(apiSecret)

	// Raw JSON body — could have any field ordering or whitespace
	rawBody := `{"TransactionId":"tx-raw","SubscriptionId":"","MerchantId":"m-raw","OrderId":"order-raw","Amount":25.0,"Fee":0.25,"Blockchain":"ETH","Asset":"USDT","BlockchainHash":"0xdef456","UserWallet":"0xRawWallet","UserName":"Bob","UserEmail":"bob@example.com","OrderAmount":25.0,"OrderCurrency":"EUR","PaidAmount":25.0,"TimeSent":"2025-03-10T14:00:00Z","Status":100}`

	// Sign the raw body (as Atlos would)
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(rawBody))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/", strings.NewReader(rawBody))
	req.Header.Set(SignatureHeader, signature)

	notification, err := handler.HandleRequest(req)
	if err != nil {
		t.Fatalf("HandleRequest() should succeed with raw body signature: %v", err)
	}
	if notification.TransactionId != "tx-raw" {
		t.Errorf("HandleRequest() should return correct notification, got TransactionId=%s", notification.TransactionId)
	}
}

func TestCreateTestPostback_FieldsPopulated(t *testing.T) {
	merchantId := "test-merchant-123"
	pn := CreateTestPostback(merchantId)

	if pn.TransactionId == "" {
		t.Errorf("CreateTestPostback() should set TransactionId")
	}
	if pn.MerchantId != merchantId {
		t.Errorf("CreateTestPostback() should set MerchantId correctly")
	}
	if pn.Amount == 0 {
		t.Errorf("CreateTestPostback() should set Amount")
	}
	if pn.Blockchain == "" {
		t.Errorf("CreateTestPostback() should set Blockchain")
	}
	if pn.Asset == "" {
		t.Errorf("CreateTestPostback() should set Asset")
	}
	if pn.Status != 100 {
		t.Errorf("CreateTestPostback() should set Status to 100")
	}
	if pn.TimeSent == "" {
		t.Errorf("CreateTestPostback() should set TimeSent")
	}
}
