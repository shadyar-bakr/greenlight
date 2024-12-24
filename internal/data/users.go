package data

import (
	"context"
	"errors"
	"time"

	"crypto/sha256"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shadyar-bakr/greenlight/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  Password  `json:"-"`
	Activated bool      `json:"activated"`
	Version   int32     `json:"-"`
}

type Password struct {
	Plaintext *string
	Hash      []byte
}

var (
	ErrPasswordTooShort       = errors.New("password must be at least 8 bytes long")
	ErrPasswordTooLong        = errors.New("password must not be more than 72 bytes long")
	ErrPasswordRequired       = errors.New("password is required")
	ErrPasswordHash           = errors.New("password hash is required")
	ErrPasswordHashGeneration = errors.New("failed to generate password hash")
	ErrPasswordHashComparison = errors.New("failed to compare password hash")
	ErrEmailRequired          = errors.New("email is required")
	ErrEmailInvalid           = errors.New("email must be a valid email address")
	ErrDuplicateEmail         = errors.New("duplicate email")
	ErrNameRequired           = errors.New("name is required")
	ErrNameTooLong            = errors.New("name must not be more than 500 bytes long")
)

type UserModel struct {
	Pool *pgxpool.Pool
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", ErrEmailRequired.Error())
	v.Check(validator.Matches(email, validator.EmailRX), "email", ErrEmailInvalid.Error())
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", ErrPasswordRequired.Error())
	v.Check(len(password) >= 8, "password", ErrPasswordTooShort.Error())
	v.Check(len(password) <= 72, "password", ErrPasswordTooLong.Error())
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", ErrNameRequired.Error())
	v.Check(len(user.Name) <= 500, "name", ErrNameTooLong.Error())

	ValidateEmail(v, user.Email)

	if user.Password.Plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.Plaintext)
	}

	if user.Password.Hash == nil {
		panic(ErrPasswordHash)
	}
}

func (p *Password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return ErrPasswordHashGeneration
	}

	p.Plaintext = &plaintextPassword
	p.Hash = hash
	return nil
}

func (p *Password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.Hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, ErrPasswordHashComparison
		}
	}
	return true, nil
}

func (m UserModel) Insert(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (name, email, password_hash, activated)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version
	`

	args := []any{user.Name, user.Email, user.Password.Hash, user.Activated}

	err := m.Pool.QueryRow(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, created_at, name, email, password_hash, activated, version
		FROM users
		WHERE email = $1
	`

	var user User

	err := m.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.Hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (m UserModel) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version
	`

	args := []any{user.Name, user.Email, user.Password.Hash, user.Activated, user.ID, user.Version}

	err := m.Pool.QueryRow(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetForToken(ctx context.Context, scope string, tokenPlaintext string) (*User, error) {
	// Generate the hash of the plaintext token
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
		FROM users
		INNER JOIN tokens ON users.id = tokens.user_id
		WHERE tokens.hash = $1 AND tokens.scope = $2 AND tokens.expiry > now()
	`

	var user User

	// Use the hashed token value in the query
	err := m.Pool.QueryRow(ctx, query, tokenHash[:], scope).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.Hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}
