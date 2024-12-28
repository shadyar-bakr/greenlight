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
	"github.com/shadyar-bakr/greenlight/internal/validator"
)

var (
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidToken = errors.New("token is invalid")
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopeRefresh        = "refresh"
)

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
	IsRefresh bool      `json:"-"`
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil
}

func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

type TokenModel struct {
	Pool *pgxpool.Pool
}

func (m *TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}

func (m *TokenModel) Insert(token *Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope, is_refresh)
		VALUES ($1, $2, $3, $4, $5)
	`
	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope, token.IsRefresh}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, args...)
	return err
}

func (m *TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `
		DELETE FROM tokens
		WHERE scope = $1 AND user_id = $2
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, scope, userID)
	return err
}

// NewPair creates both an access token and refresh token for a user
func (m *TokenModel) NewPair(userID int64, accessTTL, refreshTTL time.Duration) (*Token, *Token, error) {
	accessToken, err := generateToken(userID, accessTTL, ScopeAuthentication)
	if err != nil {
		return nil, nil, err
	}

	refreshToken, err := generateToken(userID, refreshTTL, ScopeRefresh)
	if err != nil {
		return nil, nil, err
	}
	refreshToken.IsRefresh = true

	err = m.Insert(accessToken)
	if err != nil {
		return nil, nil, err
	}

	err = m.Insert(refreshToken)
	if err != nil {
		return nil, nil, err
	}

	return accessToken, refreshToken, nil
}

// GetRefreshToken retrieves a refresh token from the database
func (m *TokenModel) GetRefreshToken(tokenPlaintext string) (*Token, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT user_id, expiry, scope
		FROM tokens
		WHERE hash = $1 AND scope = $2 AND is_refresh = true
	`

	var token Token
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Pool.QueryRow(ctx, query, tokenHash[:], ScopeRefresh).Scan(
		&token.UserID,
		&token.Expiry,
		&token.Scope,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrInvalidToken
		default:
			return nil, err
		}
	}

	// Check if the token has expired
	if time.Now().After(token.Expiry) {
		return nil, ErrExpiredToken
	}

	return &token, nil
}
