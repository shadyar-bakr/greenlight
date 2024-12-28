package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TrustedClient struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	APIKey         string    `json:"api_key,omitempty"` // Only populated when creating a new key
	RateLimitRPS   int       `json:"rate_limit_rps"`
	RateLimitBurst int       `json:"rate_limit_burst"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	Version        int32     `json:"version"`
}

type TrustedClientModel struct {
	Pool *pgxpool.Pool
}

// Insert adds a new trusted client to the database
func (m TrustedClientModel) Insert(client *TrustedClient) error {
	// Generate a random API key
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return err
	}

	// Convert to base32 for easier handling
	client.APIKey = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)

	// Hash the API key for storage
	hash := sha256.Sum256([]byte(client.APIKey))

	query := `
		INSERT INTO trusted_clients (name, description, api_key_hash, rate_limit_rps, rate_limit_burst, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, version`

	args := []any{
		client.Name,
		client.Description,
		hash[:],
		client.RateLimitRPS,
		client.RateLimitBurst,
		client.Enabled,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.Pool.QueryRow(ctx, query, args...).Scan(&client.ID, &client.CreatedAt, &client.Version)
}

// GetByAPIKey retrieves a trusted client by their API key
func (m TrustedClientModel) GetByAPIKey(apiKey string) (*TrustedClient, error) {
	// Hash the API key for lookup
	hash := sha256.Sum256([]byte(apiKey))

	query := `
		SELECT id, name, description, rate_limit_rps, rate_limit_burst, enabled, created_at, version
		FROM trusted_clients
		WHERE api_key_hash = $1`

	var client TrustedClient

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Pool.QueryRow(ctx, query, hash[:]).Scan(
		&client.ID,
		&client.Name,
		&client.Description,
		&client.RateLimitRPS,
		&client.RateLimitBurst,
		&client.Enabled,
		&client.CreatedAt,
		&client.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &client, nil
}

// Update updates a trusted client's details
func (m TrustedClientModel) Update(client *TrustedClient) error {
	query := `
		UPDATE trusted_clients
		SET name = $1, description = $2, rate_limit_rps = $3, rate_limit_burst = $4, enabled = $5, version = version + 1
		WHERE id = $6 AND version = $7
		RETURNING version`

	args := []any{
		client.Name,
		client.Description,
		client.RateLimitRPS,
		client.RateLimitBurst,
		client.Enabled,
		client.ID,
		client.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Pool.QueryRow(ctx, query, args...).Scan(&client.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

// LogRequest logs an API request from a trusted client
func (m TrustedClientModel) LogRequest(clientID int64, endpoint, method string, statusCode int) error {
	query := `
		INSERT INTO trusted_client_logs (client_id, endpoint, method, status_code)
		VALUES ($1, $2, $3, $4)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, clientID, endpoint, method, statusCode)
	return err
}

// GetAll retrieves all trusted clients
func (m TrustedClientModel) GetAll() ([]*TrustedClient, error) {
	query := `
		SELECT id, name, description, rate_limit_rps, rate_limit_burst, enabled, created_at, version
		FROM trusted_clients
		ORDER BY id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []*TrustedClient

	for rows.Next() {
		var client TrustedClient

		err := rows.Scan(
			&client.ID,
			&client.Name,
			&client.Description,
			&client.RateLimitRPS,
			&client.RateLimitBurst,
			&client.Enabled,
			&client.CreatedAt,
			&client.Version,
		)
		if err != nil {
			return nil, err
		}

		clients = append(clients, &client)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return clients, nil
}

// Delete removes a trusted client
func (m TrustedClientModel) Delete(id int64) error {
	query := `
		DELETE FROM trusted_clients
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.Pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// RegenerateAPIKey generates a new API key for a trusted client
func (m TrustedClientModel) RegenerateAPIKey(id int64) (string, error) {
	// Generate a new random API key
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}

	// Convert to base32 for easier handling
	apiKey := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)

	// Hash the API key for storage
	hash := sha256.Sum256([]byte(apiKey))

	query := `
		UPDATE trusted_clients
		SET api_key_hash = $1, version = version + 1
		WHERE id = $2
		RETURNING version`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var version int32
	err = m.Pool.QueryRow(ctx, query, hash[:], id).Scan(&version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return "", ErrRecordNotFound
		default:
			return "", err
		}
	}

	return apiKey, nil
}
