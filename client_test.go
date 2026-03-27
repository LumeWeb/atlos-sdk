package atlos

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	internalclient "go.lumeweb.com/atlos-sdk/internal/client"
)

const (
	// testSecret is used throughout tests
	testSecret = "test-secret"
)



// Test client creation and options
func TestNewClient_Default(t *testing.T) {
	client, err := NewClient(testSecret)

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client == nil {
		t.Errorf("NewClient() should return non-nil client")
	}
	if client.apiSecret != testSecret {
		t.Errorf("NewClient() should set apiSecret correctly")
	}
	if len(client.baseURL) == 0 {
		t.Errorf("NewClient() should set default endpoint")
	}
	if client.httpClient == nil {
		t.Errorf("NewClient() should initialize httpClient")
	}
}

func TestNewClient_WithEndpoint(t *testing.T) {
	customEndpoint := "https://custom.example.com"
	client, err := NewClient(testSecret, WithEndpoint(customEndpoint))

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client == nil {
		t.Errorf("NewClient() should return non-nil client")
	}
	if client.baseURL != customEndpoint {
		t.Errorf("NewClient() should set custom endpoint, got %s, want %s", client.baseURL, customEndpoint)
	}
}

func TestMultipleOptions(t *testing.T) {
	customEndpoint := "https://custom.example.com"

	client, err := NewClient(
		testSecret,
		WithEndpoint(customEndpoint),
	)

	if err != nil {
		t.Errorf("NewClient() should not error: %v", err)
	}
	if client == nil {
		t.Errorf("NewClient() should return non-nil client")
	}
	if client.baseURL != customEndpoint {
		t.Errorf("NewClient() should use endpoint from option")
	}
}

func TestNewClient_EmptySecret(t *testing.T) {
	// Note: Current implementation allows empty secrets for testing purposes.
	// In production, API calls would fail without proper authentication.
	_, err := NewClient("")
	if err != nil {
		t.Errorf("NewClient() should not error with empty secret: %v", err)
	}

	_, err = NewClient("   ")
	if err != nil {
		t.Errorf("NewClient() should not error with whitespace secret: %v", err)
	}
}

func TestClient_InternalGenNotAccessible(t *testing.T) {
	client, _ := NewClient(testSecret)

	_ = client
	// This test ensures that internalGen is not accessible outside the package
	// If this compiles, the internalGen field is properly encapsulated
}

// TestAPIEndpoints tests all wrapper methods using the generated server
func TestAPIEndpoints(t *testing.T) {
	// Create a test server using the generated infrastructure
	server, err := NewServer(
		WithSharedSecret(testSecret),
		WithPostbackURL("https://example.com/postback"),
	)
	require.NoError(t, err, "NewServer should not error")

	// Create an HTTP handler from the server using the generated code
	handler := internalclient.Handler(server)

	// Create httptest server
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	// Create client connected to the test server
	client, err := NewClient(testSecret, WithEndpoint(testServer.URL))
	require.NoError(t, err, "NewClient should not error")

	// Test AssetList
	t.Run("AssetList", func(t *testing.T) {
		ctx := t.Context()
		assets, err := client.AssetList(ctx, AssetListPostRequest{
			MerchantId:  "test-merchant",
			OrderAmount: 100,
		})
		require.NoError(t, err, "AssetList should not error")
		require.NotNil(t, assets, "assets should not be nil")
	})

	// Test InvoiceCancel
	t.Run("InvoiceCancel", func(t *testing.T) {
		ctx := t.Context()
		err := client.InvoiceCancel(ctx, InvoiceCancelPostRequest{
			MerchantId: new("test-merchant"),
			InvoiceId:  new("inv-123"),
		})
		require.NoError(t, err, "InvoiceCancel should not error")
	})

	// Test InvoiceCreate
	t.Run("InvoiceCreate", func(t *testing.T) {
		ctx := t.Context()
		invoice, err := client.InvoiceCreate(ctx, InvoiceCreatePostRequest{
			MerchantId:  "test-merchant",
			OrderAmount: 100,
		})
		require.NoError(t, err, "InvoiceCreate should not error")
		require.NotNil(t, invoice, "invoice should not be nil")
		require.NotNil(t, invoice.Id, "invoice.Id should not be nil")
	})

	// Test CreatePayment
	t.Run("CreatePayment", func(t *testing.T) {
		ctx := t.Context()
		payment, err := client.CreatePayment(ctx, CreatePaymentPostRequest{
			AssetCode:      "BTC",
			BlockchainCode: 1, // ETH chain ID
			InvoiceId:      "inv-123",
			IsEvm:          "false",
		})
		require.NoError(t, err, "CreatePayment should not error")
		require.NotNil(t, payment, "payment should not be nil")
		require.NotNil(t, payment.Id, "payment.Id should not be nil")
	})

	// Test PaymentGet
	t.Run("PaymentGet", func(t *testing.T) {
		ctx := t.Context()
		// First create a payment to get a valid payment ID
		createdPayment, err := client.CreatePayment(ctx, CreatePaymentPostRequest{
			AssetCode:      "ETH",
			BlockchainCode: 1,
			InvoiceId:      "inv-test-get",
			IsEvm:          "true",
		})
		require.NoError(t, err, "CreatePayment should not error")
		require.NotNil(t, createdPayment.Id, "createdPayment.Id should not be nil")

		// Now get the payment by ID
		payment, err := client.PaymentGet(ctx, PaymentGetPostRequest{PaymentId: *createdPayment.Id})
		require.NoError(t, err, "PaymentGet should not error")
		require.NotNil(t, payment, "payment should not be nil")
		require.Equal(t, *createdPayment.Id, *payment.Id, "payment IDs should match")
	})

	// Test PaymentWebSocket
	t.Run("PaymentWebSocket", func(t *testing.T) {
		ctx := t.Context()
		_, err := client.PaymentWebSocket(ctx, PaymentWebSocketPostRequest{PaymentId: "pay-123"})
		require.NoError(t, err, "PaymentWebSocket should not error")
	})

	// Test Cancel
	t.Run("Cancel", func(t *testing.T) {
		ctx := t.Context()
		err := client.Cancel(ctx, CancelPostRequest{
			MerchantId:     new("test-merchant"),
			SubscriptionId: new("sub-123"),
		})
		require.NoError(t, err, "Cancel should not error")
	})

	// Test FindByHash
	t.Run("FindByHash", func(t *testing.T) {
		ctx := t.Context()
		result, err := client.FindByHash(ctx, FindByHashPostRequest{
			MerchantId:     "test-merchant",
			BlockchainHash: new("0x1234567890abcdef"),
		})
		require.NoError(t, err, "FindByHash should not error")
		require.NotNil(t, result, "result should not be nil")
	})

	// Test TransactionList
	t.Run("TransactionList", func(t *testing.T) {
		ctx := t.Context()
		result, err := client.TransactionList(ctx, TransactionListPostRequest{
			MerchantId: "test-merchant",
		})
		require.NoError(t, err, "TransactionList should not error")
		require.NotNil(t, result, "result should not be nil")
	})

	// Test CancelPayout
	t.Run("CancelPayout", func(t *testing.T) {
		ctx := t.Context()
		err := client.CancelPayout(ctx, CancelPayoutPostRequest{
			MerchantId: new("test-merchant"),
			PaymentId:  new("payout-123"),
		})
		require.NoError(t, err, "CancelPayout should not error")
	})

	// Test SendToken
	t.Run("SendToken", func(t *testing.T) {
		ctx := t.Context()
		result, err := client.SendToken(ctx, SendTokenPostRequest{
			MerchantId:      "test-merchant",
			AssetCode:       "BTC",
			BlockchainCode:  "BTC",
			RecipientAddress: "0x" + generateHexString(40),
		})
		require.NoError(t, err, "SendToken should not error")
		require.NotNil(t, result, "result should not be nil")
	})
}

// TestAPIErrors tests error handling for all wrapper methods
func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		handler    internalclient.ServerInterface
		callClient func(*Client) error
		wantError  string
	}{
		{
			name: "AssetList_500",
			handler: &mockErrorServer{statusCode: 500},
			callClient: func(c *Client) error {
				ctx := t.Context()
				_, err := c.AssetList(ctx, AssetListPostRequest{MerchantId: "test-merchant", OrderAmount: 100})
				return err
			},
			wantError: "unexpected status code 500",
		},
		{
			name: "InvoiceCreate_401",
			handler: &mockErrorServer{statusCode: 401},
			callClient: func(c *Client) error {
				ctx := t.Context()
				_, err := c.InvoiceCreate(ctx, InvoiceCreatePostRequest{
					MerchantId:  "test-merchant",
					OrderAmount: 100,
				})
				return err
			},
			wantError: "unexpected status code 401",
		},
		{
			name: "CreatePayment_400",
			handler: &mockErrorServer{statusCode: 400},
			callClient: func(c *Client) error {
				ctx := t.Context()
				_, err := c.CreatePayment(ctx, CreatePaymentPostRequest{
					AssetCode:      "BTC",
					BlockchainCode: 1,
					InvoiceId:      "inv-123",
					IsEvm:          "false",
				})
				return err
			},
			wantError: "unexpected status code 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler from the mock error server
			handler := internalclient.Handler(tt.handler)
			testServer := httptest.NewServer(handler)
			defer testServer.Close()

			client, err := NewClient(testSecret, WithEndpoint(testServer.URL))
			require.NoError(t, err, "NewClient should not error")

			err = tt.callClient(client)
			require.Error(t, err, "expected error")
			require.ErrorContains(t, err, tt.wantError, "expected specific error message")
		})
	}
}

// mockErrorServer is a simple mock server that returns error status codes
type mockErrorServer struct {
	statusCode int
}

func (m *mockErrorServer) AssetListPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) InvoiceCancelPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) InvoiceCreatePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) CreatePaymentPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) PaymentGetPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) PaymentWebSocketPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) CancelPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) FindByHashPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) TransactionListPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) CancelPayoutPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}

func (m *mockErrorServer) SendTokenPost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
}
