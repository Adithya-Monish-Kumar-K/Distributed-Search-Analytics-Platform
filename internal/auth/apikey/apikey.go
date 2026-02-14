// Package apikey provides SHA-256-based API key validation against PostgreSQL.
// Raw keys are generated with crypto/rand, hashed before storage, and validated
// by comparing the hash of the presented key with the stored hash. Keys can
// be created, revoked, and listed.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

var (
	ErrInvalidKey = errors.New("invalid api key")
	ErrExpiredKey = errors.New("api key expired")
)

// KeyInfo holds metadata about a validated API key.
type KeyInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	RateLimit int        `json:"rate_limit"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Validator validates API keys against the api_keys table in PostgreSQL.
type Validator struct {
	db     *postgres.Client
	logger *slog.Logger
}

// NewValidator creates a new API key validator backed by PostgreSQL.
func NewValidator(db *postgres.Client) *Validator {
	return &Validator{
		db:     db,
		logger: slog.Default().With("component", "apikey-validator"),
	}
}

// Validate checks a raw API key against the database.
// Returns KeyInfo on success, or ErrInvalidKey / ErrExpiredKey on failure.
func (v *Validator) Validate(ctx context.Context, rawKey string) (*KeyInfo, error) {
	hash := HashKey(rawKey)

	var info KeyInfo
	var expiresAt sql.NullTime
	var createdAt time.Time

	err := v.db.DB.QueryRowContext(ctx,
		`SELECT id, name, rate_limit, is_active, created_at, expires_at
		 FROM api_keys
		 WHERE key_hash = $1 AND is_active = true`,
		hash,
	).Scan(&info.ID, &info.Name, &info.RateLimit, &info.IsActive, &createdAt, &expiresAt)

	info.CreatedAt = createdAt

	if err == sql.ErrNoRows {
		return nil, ErrInvalidKey
	}
	if err != nil {
		return nil, fmt.Errorf("querying api key: %w", err)
	}

	if expiresAt.Valid {
		if expiresAt.Time.Before(time.Now()) {
			return nil, ErrExpiredKey
		}
		info.ExpiresAt = &expiresAt.Time
	}

	return &info, nil
}

// CreateKey generates a new API key, stores its hash, and returns the raw key.
// The raw key is returned only once and cannot be retrieved again.
func (v *Validator) CreateKey(ctx context.Context, name string, rateLimit int, expiresAt *time.Time) (string, error) {
	rawKey := generateRawKey()
	hash := HashKey(rawKey)

	var expiry sql.NullTime
	if expiresAt != nil {
		expiry = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	_, err := v.db.DB.ExecContext(ctx,
		`INSERT INTO api_keys (key_hash, name, rate_limit, expires_at) VALUES ($1, $2, $3, $4)`,
		hash, name, rateLimit, expiry,
	)
	if err != nil {
		return "", fmt.Errorf("creating api key: %w", err)
	}

	v.logger.Info("api key created", "name", name, "rate_limit", rateLimit)
	return rawKey, nil
}

// RevokeKey deactivates an API key so it can no longer be used.
func (v *Validator) RevokeKey(ctx context.Context, rawKey string) error {
	hash := HashKey(rawKey)

	result, err := v.db.DB.ExecContext(ctx,
		`UPDATE api_keys SET is_active = false WHERE key_hash = $1`,
		hash,
	)
	if err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvalidKey
	}

	v.logger.Info("api key revoked")
	return nil
}

// ListKeys returns all active API keys (without the raw key / hash).
func (v *Validator) ListKeys(ctx context.Context) ([]KeyInfo, error) {
	rows, err := v.db.DB.QueryContext(ctx,
		`SELECT id, name, rate_limit, is_active, created_at, expires_at FROM api_keys WHERE is_active = true ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()

	var keys []KeyInfo
	for rows.Next() {
		var k KeyInfo
		var expiresAt sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.RateLimit, &k.IsActive, &k.CreatedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scanning api key row: %w", err)
		}
		if expiresAt.Valid {
			k.ExpiresAt = &expiresAt.Time
		}
		keys = append(keys, k)
	}

	return keys, rows.Err()
}

// HashKey returns the SHA-256 hex digest of a raw API key.
func HashKey(raw string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}

// generateRawKey returns a cryptographically random 32-byte hex-encoded string
// suitable for use as an API key.
func generateRawKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
