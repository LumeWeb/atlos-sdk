# Atlos SDK

Go SDK for the Atlos Gateway API, generated from OpenAPI specification.

## Overview

This SDK provides a client and mock server implementation for the Atlos Payment Gateway API.

- **Generated Client**: HTTP client methods for all API endpoints
- **Mock Server**: Standard `net/http` server implementation for testing

## Installation

```bash
go get go.lumeweb.com/atlos-sdk
```

## Configuration

The SDK uses `oapi-codegen` with the following configuration:

- **Client**: Generated in `internal/client/client.gen.go`
- **Mock Server**: Generated with `std-http-server: true` option

## Usage

### Client Usage

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
    
    // Use the internal client to make API calls
    // Example: Create a payment
    req := client.SendTokenPostJSONRequestBody{
        MerchantId:      "your-merchant-id",
        OrderAmount:     100.0,
        OrderCurrency:   "USD",
        TokenAmount:     10.0,
        AssetCode:       "USDC",
        BlockchainCode:  "ETH",
        RecipientAddress: "0x...",
    }
    
    resp, err := client.InternalGen.SendTokenPostWithResponse(ctx, req)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Response: %s", string(resp.Body))
}
```

### Mock Server Usage

```go
package main

import (
    "log"
    "net/http"
    
    client "go.lumeweb.com/atlos-sdk/internal/client"
)

func main() {
    // Create server instance
    server := client.NewServer()
    
    // Create HTTP handler from server
    handler := client.Handler(server)
    
    // Start server
    log.Println("Starting mock server on :8080")
    if err := http.ListenAndServe(":8080", handler); err != nil {
        log.Fatal(err)
    }
}
```

### Custom Server Implementation

To implement custom server logic, replace the mock implementations in `internal/client/server.go`:

```go
package client

import "net/http"

// SendTokenPost handles the SendToken endpoint.
func (s *Server) SendTokenPost(w http.ResponseWriter, r *http.Request) {
    // Parse request body
    var req SendTokenPostJSONRequestBody
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Your custom logic here
    // ...
    
    // Return response
    w.WriteHeader(http.StatusOK)
}
```

## Code Generation

To regenerate the client and server code after modifying the OpenAPI spec:

```bash
go generate ./...
```

This uses the `oapi-codegen` configuration in `oai-codegen.yaml`:

```yaml
package: client
output: internal/client/client.gen.go
generate:
  models: true
  client: true
  std-http-server: true
```

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

## Architecture

```
atlos-sdk/
├── client.go                 # Main SDK client
├── oai-codegen.yaml          # Code generation config
├── internal/
│   └── client/
│       ├── client.gen.go      # Generated client + server (DO NOT EDIT)
│       └── server.go          # Mock server implementation
└── swagger-spec.yaml         # OpenAPI specification
```

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
- `POST /Transaction/List` - List transactions
- `POST /Transaction/FindByHash` - Find transaction by hash
- `POST /Subscription/Cancel` - Cancel subscription

## License

MIT
