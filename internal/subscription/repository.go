// internal/subscription/repository.go
package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDuplicate is returned when (email, repo) already exists.
var ErrDuplicate = errors.New("subscription already exists")

// ErrNotFound is returned when a subscription cannot be located.
var ErrNotFound = errors.New("subscription not found")

// Repository defines persistence operations for subscriptions.
type Repository interface {
	Create(ctx context.Context, s *Subscription) error
	FindByConfirmToken(ctx context.Context, token string) (*Subscription, error)
	FindByUnsubToken(ctx context.Context, token string) (*Subscription, error)
	FindConfirmedByEmail(ctx context.Context, email string) ([]*Subscription, error)
	FindAllConfirmed(ctx context.Context) ([]*Subscription, error)
	Confirm(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	UpdateLastSeenTag(ctx context.Context, id, tag string) error
}

// PostgresRepository is the pgx/v5-backed implementation of Repository.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL-backed repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a new unconfirmed subscription. Returns ErrDuplicate if (email, repo) already exists.
func (r *PostgresRepository) Create(ctx context.Context, s *Subscription) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subscriptions (email, repo, confirm_token, unsub_token)
		 VALUES ($1, $2, $3, $4)`,
		s.Email, s.Repo, s.ConfirmToken, s.UnsubToken,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

// FindByConfirmToken looks up a subscription by confirmation token.
func (r *PostgresRepository) FindByConfirmToken(ctx context.Context, token string) (*Subscription, error) {
	return r.findByField(ctx, "confirm_token", token)
}

// FindByUnsubToken looks up a subscription by unsubscribe token.
func (r *PostgresRepository) FindByUnsubToken(ctx context.Context, token string) (*Subscription, error) {
	return r.findByField(ctx, "unsub_token", token)
}

// FindConfirmedByEmail returns all confirmed subscriptions for an email address.
func (r *PostgresRepository) FindConfirmedByEmail(ctx context.Context, email string) ([]*Subscription, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, repo, confirmed, confirm_token, unsub_token, last_seen_tag, created_at
		 FROM subscriptions WHERE email = $1 AND confirmed = TRUE`,
		email,
	)
	if err != nil {
		return nil, fmt.Errorf("find confirmed by email: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

// FindAllConfirmed returns every confirmed subscription (used by the scanner).
func (r *PostgresRepository) FindAllConfirmed(ctx context.Context) ([]*Subscription, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, repo, confirmed, confirm_token, unsub_token, last_seen_tag, created_at
		 FROM subscriptions WHERE confirmed = TRUE`,
	)
	if err != nil {
		return nil, fmt.Errorf("find all confirmed: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

// Confirm sets confirmed=true for the subscription with the given id.
func (r *PostgresRepository) Confirm(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE subscriptions SET confirmed = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("confirm subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a subscription by id.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastSeenTag stores the latest seen release tag for a subscription.
func (r *PostgresRepository) UpdateLastSeenTag(ctx context.Context, id, tag string) error {
	_, err := r.pool.Exec(ctx, `UPDATE subscriptions SET last_seen_tag = $1 WHERE id = $2`, tag, id)
	return err
}

func (r *PostgresRepository) findByField(ctx context.Context, field, value string) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		fmt.Sprintf(
			`SELECT id, email, repo, confirmed, confirm_token, unsub_token, last_seen_tag, created_at
			 FROM subscriptions WHERE %s = $1`, field),
		value,
	)
	s, err := scanOne(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

type rowScanner interface{ Scan(dest ...any) error }

func scanOne(s rowScanner) (*Subscription, error) {
	var sub Subscription
	err := s.Scan(
		&sub.ID, &sub.Email, &sub.Repo, &sub.Confirmed,
		&sub.ConfirmToken, &sub.UnsubToken, &sub.LastSeenTag, &sub.CreatedAt,
	)
	return &sub, err
}

func collectRows(rows pgx.Rows) ([]*Subscription, error) {
	var result []*Subscription
	for rows.Next() {
		s, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
