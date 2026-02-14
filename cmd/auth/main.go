package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// auth is a CLI tool for managing API keys.
//
// Usage:
//
//	auth create  --name "my-app" [--rate-limit 100] [--expires-in 720h]
//	auth revoke  --key <raw-key>
//	auth list
func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)

	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	validator := apikey.NewValidator(db)
	ctx := context.Background()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		cmdCreate(ctx, validator, args[1:])
	case "revoke":
		cmdRevoke(ctx, validator, args[1:])
	case "list":
		cmdList(ctx, validator)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func cmdCreate(ctx context.Context, v *apikey.Validator, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "name for the api key")
	rateLimit := fs.Int("rate-limit", 100, "requests per minute")
	expiresIn := fs.String("expires-in", "", "expiry duration, e.g. 720h (optional)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
		os.Exit(1)
	}

	var expiresAt *time.Time
	if *expiresIn != "" {
		d, err := time.ParseDuration(*expiresIn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --expires-in: %v\n", err)
			os.Exit(1)
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	key, err := v.CreateKey(ctx, *name, *rateLimit, expiresAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("API key created successfully.")
	fmt.Println("Store this key securely — it cannot be retrieved again.")
	fmt.Println()
	fmt.Printf("  Key:        %s\n", key)
	fmt.Printf("  Name:       %s\n", *name)
	fmt.Printf("  Rate Limit: %d req/min\n", *rateLimit)
	if expiresAt != nil {
		fmt.Printf("  Expires:    %s\n", expiresAt.Format(time.RFC3339))
	} else {
		fmt.Println("  Expires:    never")
	}
}

func cmdRevoke(ctx context.Context, v *apikey.Validator, args []string) {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)
	key := fs.String("key", "", "raw api key to revoke")
	fs.Parse(args)

	if *key == "" {
		fmt.Fprintln(os.Stderr, "error: --key is required")
		os.Exit(1)
	}

	if err := v.RevokeKey(ctx, *key); err != nil {
		fmt.Fprintf(os.Stderr, "failed to revoke key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("API key revoked successfully.")
}

func cmdList(ctx context.Context, v *apikey.Validator) {
	keys, err := v.ListKeys(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list keys: %v\n", err)
		os.Exit(1)
	}

	if len(keys) == 0 {
		fmt.Println("No active API keys.")
		return
	}

	fmt.Printf("%-36s  %-20s  %-10s  %s\n", "ID", "Name", "Rate Limit", "Expires")
	fmt.Println("------------------------------------  --------------------  ----------  -------------------------")
	for _, k := range keys {
		expires := "never"
		if k.ExpiresAt != nil {
			expires = k.ExpiresAt.Format(time.RFC3339)
		}
		fmt.Printf("%-36s  %-20s  %-10d  %s\n", k.ID, k.Name, k.RateLimit, expires)
	}

	fmt.Printf("\nTotal: %d active key(s)\n", len(keys))
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: auth <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  create   Create a new API key")
	fmt.Fprintln(os.Stderr, "  revoke   Revoke an existing API key")
	fmt.Fprintln(os.Stderr, "  list     List all active API keys")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, `  auth create --name "my-app" --rate-limit 100 --expires-in 720h`)
	fmt.Fprintln(os.Stderr, `  auth revoke --key "abc123..."`)
	fmt.Fprintln(os.Stderr, `  auth list`)
}
