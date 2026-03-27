package atlos

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNewClient_Default(t *testing.T) {
	apiSecret := "test-secret"
	client, err := NewClient(apiSecret)

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client == nil {
		t.Errorf("NewClient() should return non-nil client")
	}
	if client.apiSecret != apiSecret {
		t.Errorf("NewClient() should set apiSecret correctly")
	}
	if len(client.baseURL) == 0 {
		t.Errorf("NewClient() should set default endpoint")
	}
	if _, err := url.Parse(client.baseURL); err != nil {
		t.Errorf("NewClient() should have valid base URL: %v", err)
	}
	if client.httpClient == nil {
		t.Errorf("NewClient() should initialize httpClient")
	}
	if client.internalGen == nil {
		t.Errorf("NewClient() should initialize internalGen")
	}
}

func TestNewClient_WithEndpoint(t *testing.T) {
	apiSecret := "test-secret"
	customEndpoint := "https://custom.example.com"
	client, err := NewClient(apiSecret, WithEndpoint(customEndpoint))

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client == nil {
		t.Errorf("NewClient() should return non-nil client")
	}
	if client.baseURL != customEndpoint {
		t.Errorf("NewClient() should set custom endpoint, got %s, expected %s", client.baseURL, customEndpoint)
	}
}

func TestNewClient_WithAPISecret(t *testing.T) {
	customSecret := "custom-secret"
	client, err := NewClient("test-secret", WithAPISecret(customSecret))

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client.apiSecret != customSecret {
		t.Errorf("NewClient() should set custom API secret")
	}
}

func TestNewClient_MultipleOptions(t *testing.T) {
	customEndpoint := "https://custom.example.com"
	customSecret := "custom-secret"
	client, err := NewClient("default-secret", WithEndpoint(customEndpoint), WithAPISecret(customSecret))

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client.baseURL != customEndpoint {
		t.Errorf("NewClient() should apply endpoint option")
	}
	if client.apiSecret != customSecret {
		t.Errorf("NewClient() should apply API secret option")
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	apiSecret := "test-secret"
	client, err := NewClient(apiSecret, WithEndpoint("://invalid-url"))

	if err == nil {
		t.Errorf("NewClient() should error with invalid URL")
	}
	if client != nil {
		t.Errorf("NewClient() should return nil client on error")
	}
}

func TestNewClient_URLNormalization(t *testing.T) {
	apiSecret := "test-secret"
	urlsWithPaths := []string{
		"https://api.example.com/path",
		"https://api.example.com/path/",
	}

	for _, testURL := range urlsWithPaths {
		client, err := NewClient(apiSecret, WithEndpoint(testURL))
		if err != nil {
			t.Errorf("NewClient() should handle URL with path: %v", err)
			continue
		}

		parsedURL, err := url.Parse(client.baseURL)
		if err != nil {
			t.Errorf("NewClient() should create valid normalized URL: %v", err)
			continue
		}

		if parsedURL.Path != "" {
			t.Errorf("NewClient() should normalize URL path, got %s", parsedURL.Path)
		}
	}
}

func TestClient_BaseURL(t *testing.T) {
	client, _ := NewClient("test-secret", WithEndpoint("https://test.example.com"))

	if client.BaseURL() != "https://test.example.com" {
		t.Errorf("BaseURL() should return correct base URL")
	}
}

func TestClient_APISecret(t *testing.T) {
	apiSecret := "my-secret-key"
	client, _ := NewClient(apiSecret)

	if client.APISecret() != apiSecret {
		t.Errorf("APISecret() should return correct API secret")
	}
}

func TestClient_SetHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	client, _ := NewClient("test-secret")

	client.SetHTTPClient(customClient)

	if client.httpClient != customClient {
		t.Errorf("SetHTTPClient() should set the custom client")
	}
}

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.Endpoint != DefaultEndpoint {
		t.Errorf("DefaultClientConfig() should set default endpoint")
	}
}

func TestWithEndpoint(t *testing.T) {
	cfg := &clientConfig{}
	WithEndpoint("https://test.com")(cfg)

	if cfg.endpoint != "https://test.com" {
		t.Errorf("WithEndpoint() should set endpoint")
	}
}

func TestWithAPISecret(t *testing.T) {
	cfg := &clientConfig{}
	WithAPISecret("my-secret")(cfg)

	if cfg.apiSecret != "my-secret" {
		t.Errorf("WithAPISecret() should set api secret")
	}
}

func TestClient_NilInitializers(t *testing.T) {
	apiSecret := "test-secret"
	client, err := NewClient(apiSecret)

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client.httpClient == nil {
		t.Errorf("NewClient() should initialize httpClient")
	}
	if client.internalGen == nil {
		t.Errorf("NewClient() should initialize internalGen")
	}
}

func TestClient_EmptyAPISecret(t *testing.T) {
	apiSecret := ""
	client, err := NewClient(apiSecret)

	if err != nil {
		t.Errorf("NewClient() should allow empty API secret: %v", err)
	}
	if client.apiSecret != "" {
		t.Errorf("NewClient() should allow empty API secret")
	}
}

func TestClient_URLPathNormalization(t *testing.T) {
	apiSecret := "test-secret"
	endpoints := []string{
		"https://api.example.com",
		"https://api.example.com/",
		"https://api.example.com/v1",
		"https://api.example.com/v1/",
	}

	for i, endpoint := range endpoints {
		client, err := NewClient(apiSecret, WithEndpoint(endpoint))
		if err != nil {
			t.Errorf("Test case %d: NewClient() should handle endpoint %s: %v", i, endpoint, err)
			continue
		}

		parsedURL, err := url.Parse(client.baseURL)
		if err != nil {
			t.Errorf("Test case %d: Invalid normalized URL: %v", i, err)
			continue
		}

		if parsedURL.Path != "" {
			t.Errorf("Test case %d: Expected empty path, got %s", i, parsedURL.Path)
		}
	}
}

func TestNewClient_URLComponentHandling(t *testing.T) {
	apiSecret := "test-secret"
	endpoint := "https://api.example.com:8080"
	
	client, err := NewClient(apiSecret, WithEndpoint(endpoint))
	if err != nil {
		t.Errorf("NewClient() should handle URL with port: %v", err)
	}

	parsedURL, err := url.Parse(client.baseURL)
	if err != nil {
		t.Errorf("NewClient() should create valid URL with port: %v", err)
	}

	if parsedURL.Host != "api.example.com:8080" {
		t.Errorf("NewClient() should preserve host and port, got %s", parsedURL.Host)
	}
}

func TestClient_ConcurrencySafety(t *testing.T) {
	client, _ := NewClient("test-secret")
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			_ = client.BaseURL()
			_ = client.APISecret()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
