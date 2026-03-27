# Atlos SDK

Go SDK for the Atlos Gateway API, generated from OpenAPI specification.

## Overview

This SDK provides a client and mock server implementation for the Atlos Payment Gateway API.

- **Generated Client**: HTTP client methods for all API endpoints
- **Mock Server**: Standard `net/http` server implementation with postback notification support
- **Postback Handling**: HMAC-SHA256 signature verification for payment confirmations

## Installation

```bash
go get go.lumeweb.com/atlos-sdk
```

## Usage

### Client Usage

The client wraps the generated HTTP client and handles API secret authentication automatically.

```go
package main

import (
    "context"
    "log"
    
    "go.lumeweb.com/atlos-sdk"
)

func main() {
    // Create a new client with your API secret
    client, err := atlos.NewClient("your-api-secret")
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // Example: Create a payment
    req := atlos.CreatePaymentPostRequest{
        AssetCode:      "USDC",
        BlockchainCode: 1, // ETH blockchain code (numeric)
        InvoiceId:      "invoice-123",
        IsEvm:          "true",
        UserAddress:    nil,
    }
    payment, err := client.CreatePayment(ctx, req)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Payment ID: %s", *payment.Id)
}
```

**Note:** The SDK re-exports all generated types from the `internal/client` package at the top level for convenience. Types like `CreatePaymentPostRequest`, `Payment`, `Asset`, etc. are available directly from the `atlos` package without needing to import the internal package.

### Client Configuration Options

You can configure the client with options:

```go
client, err := atlos.NewClient(
    "your-api-secret",
    atlos.WithEndpoint("https://custom-endpoint.example.com"),
)
```

### Available Methods

The client provides wrapper methods for all API endpoints:

| Method | Description | Returns |
|--------|-------------|---------|
| `AssetList(ctx, req)` | List available assets | `[]Asset, error` |
| `InvoiceCreate(ctx, req)` | Create a new invoice | `*InvoiceResponse, error` |
| `InvoiceCancel(ctx, req)` | Cancel an invoice | `error` |
| `CreatePayment(ctx, req)` | Create a new payment | `*Payment, error` |
| `PaymentGet(ctx, req)` | Get payment details | `*Payment, error` |
| `SendToken(ctx, req)` | Send tokens to recipient | `*SendTokenPostResponseBody, error` |
| `CancelPayout(ctx, req)` | Cancel a payout | `error` |
| `FindByHash(ctx, req)` | Find transaction by hash | `*FindByHashPostResponseBody, error` |
| `TransactionList(ctx, req)` | List transactions | `*TransactionListPostResponseBody, error` |
| `Cancel(ctx, req)` | Cancel subscription | `error` |
| `PaymentWebSocket(ctx, req)` | Get WebSocket token for updates | `[]byte, error` |

#### Example: Listing Assets

```go
req := atlos.AssetListPostRequest{
    MerchantId:  "merchant-123",
    OrderAmount: 100.0,
}

assets, err := client.AssetList(ctx, req)
if err != nil {
    log.Fatal(err)
}

for _, asset := range assets {
    log.Printf("Asset: %s (Code: %s)", *asset.Name, *asset.Code)
}
```

#### Example: Creating an Invoice

```go
req := atlos.InvoiceCreatePostRequest{
    // Populate request fields
}

invoice, err := client.InvoiceCreate(ctx, req)
if err != nil {
    log.Fatal(err)
}

log.Printf("Invoice created: %s", *invoice.Id)
}
```

### Mock Server Usage

The mock server provides a full implementation of the API interface for testing, with support for postback notifications.

```go
package main

import (
    "log"
    "net/http"
    
    "go.lumeweb.com/atlos-sdk"
)

func main() {
    // Create server with postback configuration
    server, err := atlos.NewServer(
        atlos.WithPostbackURL("https://example.com/postback"),
        atlos.WithSharedSecret("your-api-secret"),
        atlos.WithPostbackMode(atlos.PostbackImmediate),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Start server
    handler := server.Handler()
    log.Println("Starting mock server on :8080")
    if err := http.ListenAndServe(":8080", handler); err != nil {
        log.Fatal(err)
    }
}
```

### Postback Notifications

The SDK includes comprehensive support for handling postback notifications, which are sent when payments are confirmed on the blockchain.

#### Sending Postbacks

```go
sender := atlos.NewPostbackSender("your-api-secret")
notification := &atlos.PostbackNotification{
    TransactionId:  "tx-123",
    MerchantId:     "merchant-456",
    Amount:         20.00,
    Status:         100,
    // ... other fields
}

err := sender.Send("https://example.com/postback", notification)
if err != nil {
    log.Fatal(err)
}
```

#### Receiving and Verifying Postbacks

```go
handler := atlos.NewPostbackHandler("your-api-secret")

notification, err := handler.HandleRequest(req)
if err != nil {
    log.Printf("Invalid postback: %v", err)
    return
}

log.Printf("Valid postback received for transaction: %s", notification.TransactionId)
```

### Testing with Mock Server

The mock server provides methods to complete payments and trigger postbacks during tests.

#### Complete Payment Endpoint

The `/Payment/Complete` endpoint simulates blockchain confirmation:

```go
reqBody := `{"PaymentId": "pay-123"}`
req, _ := http.NewRequest("POST", "/Payment/Complete", strings.NewReader(reqBody))
resp, err := http.DefaultClient.Do(req)
```

This marks the payment as successful and generates a blockchain transaction hash.

#### Postback Modes

Postback sending behavior is controlled by `PostbackMode`:

- `PostbackDisabled`: No postbacks sent
- `PostbackImmediate`: Postbacks sent immediately when payment completes
- `PostbackManual`: Postbacks only sent via explicit `SendPostback()` call

#### Accessing Test Data

```go
payment, err := server.GetPayment("pay-123")
if err != nil {
    log.Fatal(err)
}

invoice, err := server.GetInvoice("inv-456")
if err != nil {
    log.Fatal(err)
}
```

### CLI Development Server

A CLI tool is provided for running a development server:

```bash
go run cmd/main.go --port 8080 --postback-url https://example.com/callback --shared-secret test-secret
```

## Code Generation

To regenerate the client and server code after modifying the OpenAPI spec:

```bash
go generate ./...
```

This uses the `oapi-codegen` configuration in `oai-codegen.yaml` and `oai-codegen-server.yaml`.

## Development

### Prerequisites

- Go 1.26 or later
- oapi-codegen v2.6.0 or later

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

Run specific test:
```bash
go test -v ./server_test.go ./server.go
go test -v -run TestServer_InvoiceCreatePost
```

Test with race detection and coverage:
```bash
go test -race -coverprofile=coverage.out -covermode=atomic ./...
```

### Regenerate Asset Data

To regenerate `assets.go` from the live API (requires API credentials):

```bash
ATLOS_API_SECRET=xxx ATLOS_MERCHANT_ID=yyy go run scripts/fetch-assets.go > assets.go
```

## Architecture

```
atlos-sdk/
├── client.go                 # Main SDK client wrapper (public API)
├── server.go                 # Mock server implementation for testing
├── postback.go               # Postback notification handlers
├── assets.go                 # Generated asset data (DO NOT EDIT)
├── oai-codegen.yaml          # Code gen config for client/models
├── oai-codegen-server.yaml   # Code gen config for server interface
├── swagger-spec.yaml         # OpenAPI 3.0 specification
├── internal/
│   └── client/
│       ├── client.gen.go     # Generated HTTP client & models (DO NOT EDIT)
│       └── server.gen.go     # Generated server interface (DO NOT EDIT)
├── cmd/main.go               # CLI development server
└── scripts/fetch-assets.go   # Utility to regenerate assets.go
```

#### Code Generation Flow

Two `oapi-codegen` configs handle different generation targets:

- `oai-codegen.yaml`: Generates models and client to `internal/client/client.gen.go`
- `oai-codegen-server.yaml`: Generates std-http-server interface to `internal/client/server.gen.go`

## API Endpoints

The generated client supports all Atlos API endpoints:

- `POST /Wallet/SendToken` - Create outgoing payment
- `POST /Wallet/CancelPayout` - Cancel payout
- `POST /Asset/List` - List available assets
- `POST /Invoice/Create` - Create invoice
- `POST /Invoice/Cancel` - Cancel invoice
- `POST /Payment/WebSocket` - WebSocket endpoint for payment updates
- `POST /Payment/Create` - Create payment
- `POST /Payment/Get` - Get payment details
- `POST /Payment/Complete` - Test endpoint to simulate payment completion (mock only)
- `POST /Transaction/List` - List transactions
- `POST /Transaction/FindByHash` - Find transaction by hash
- `POST /Subscription/Cancel` - Cancel subscription

## Important Notes

### Authentication

API uses `ApiSecret` header (not Bearer token). Set via `WithAPISecret()` client option.

### Generated Files

Never manually edit files marked `// Code generated by...` or `// DO NOT EDIT`.

### Concurrency

The mock server uses `sync.RWMutex` for protecting maps. All methods acquiring locks ensure proper defer execution.

## License

MIT
