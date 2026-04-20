package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
	"go.lumeweb.com/atlos-sdk"
)

var version = "dev"

func main() {
	app := &cli.Command{
		Name:    "atlos-sdk",
		Version: version,
		Usage:   "ATLOS Payment Gateway SDK development server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "port",
				Value: "8080",
				Usage: "Port to run the server on",
			},
			&cli.StringFlag{
				Name:  "postback-url",
				Value: "",
				Usage: "Postback URL for payment notifications",
			},
			&cli.StringFlag{
				Name:  "shared-secret",
				Value: "test-secret",
				Usage: "Shared secret for HMAC signature generation",
			},
			&cli.StringFlag{
				Name:  "postback-mode",
				Value: "immediate",
				Usage: "Postback mode: disabled|immediate|manual",
			},
		},
		Action: runServer,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func runServer(ctx context.Context, cmd *cli.Command) error {
	port := cmd.String("port")
	postbackURL := cmd.String("postback-url")
	sharedSecret := cmd.String("shared-secret")
	postbackModeStr := cmd.String("postback-mode")

	postbackMode, err := parsePostbackMode(postbackModeStr)
	if err != nil {
		return fmt.Errorf("invalid postback mode: %w", err)
	}

	server, err := atlos.NewServer(
		atlos.WithPostbackURL(postbackURL),
		atlos.WithSharedSecret(sharedSecret),
		atlos.WithPostbackMode(postbackMode),
	)
	if err != nil {
		return err
	}

	fmt.Printf("Starting ATLOS SDK server on port %s\n", port)
	fmt.Printf("Postback URL: %s\n", postbackURL)
	fmt.Printf("Postback Mode: %s\n", postbackModeStr)
	fmt.Println()
	fmt.Println("Available endpoints:")
	fmt.Println("  POST /Asset/List       - Get available assets")
	fmt.Println("  POST /Invoice/Create    - Create invoice")
	fmt.Println("  POST /Payment/Create    - Create payment")
	fmt.Println("  POST /Payment/Get       - Get payment status")
	fmt.Println("  POST /Payment/Complete  - Simulate blockchain confirmation")
	fmt.Println("  POST /Reset             - Clear all mock server state")
	fmt.Println()
	fmt.Println("To simulate payment completion:")
	fmt.Println(`  curl -X POST http://localhost:8080/Payment/Complete -d '{"PaymentId":"pay-1"}'`)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	handler := server.Handler()
	log.Printf("Server listening on :%s", port)
	return http.ListenAndServe(":"+port, handler)
}

func parsePostbackMode(s string) (atlos.PostbackMode, error) {
	switch s {
	case "disabled":
		return atlos.PostbackDisabled, nil
	case "immediate":
		return atlos.PostbackImmediate, nil
	case "manual":
		return atlos.PostbackManual, nil
	default:
		return 0, fmt.Errorf("unknown postback mode: %s", s)
	}
}
