package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/banking/transaction-service/internal/config"
	"github.com/banking/transaction-service/internal/domain"
)

var (
	ErrNotFound = errors.New("record not found")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, cfg config.DatabaseConfig) (*PostgresRepository, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

// GetAccount retrieves an account by ID
func (r *PostgresRepository) GetAccount(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	query := `
		SELECT id, user_id, account_number, account_type, currency, 
		       balance, reserved, available, daily_limit, daily_used, per_txn_limit,
		       status, kyc_approved, last_txn_at, daily_reset_at, created_at, updated_at, version
		FROM accounts
		WHERE id = $1`

	var a domain.Account
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.UserID, &a.AccountNumber, &a.AccountType, &a.Currency,
		&a.Balance, &a.Reserved, &a.Available, &a.DailyLimit, &a.DailyUsed, &a.PerTxnLimit,
		&a.Status, &a.KYCApproved, &a.LastTxnAt, &a.DailyResetAt, &a.CreatedAt, &a.UpdatedAt, &a.Version,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &a, nil
}

// UpdateAccount updates an account balance and status (optimistic locking)
func (r *PostgresRepository) UpdateAccount(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts
		SET balance = $1, reserved = $2, available = $3, daily_used = $4,
		    last_txn_at = $5, updated_at = $6, version = version + 1
		WHERE id = $7 AND version = $8`

	tag, err := r.pool.Exec(ctx, query,
		account.Balance, account.Reserved, account.Available, account.DailyUsed,
		account.LastTxnAt, time.Now().UTC(),
		account.ID, account.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic locking failure: account modified concurrently")
	}

	// Update version in memory struct to match DB
	account.Version++

	return nil
}

// CreateTransaction stores a new transaction
func (r *PostgresRepository) CreateTransaction(ctx context.Context, txn *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, user_id, from_account_id, to_account_id, amount, currency, reserved_amount,
			status, transfer_type, initiation_method, memo, reference,
			initiated_at, created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16
		)`

	_, err := r.pool.Exec(ctx, query,
		txn.ID, txn.UserID, txn.FromAccountID, txn.ToAccountID, txn.Amount, txn.Currency, txn.ReservedAmount,
		txn.Status, txn.TransferType, txn.InitiationMethod, txn.Memo, txn.Reference,
		txn.InitiatedAt, txn.CreatedAt, txn.UpdatedAt, txn.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// UpdateTransaction updates a transaction status
func (r *PostgresRepository) UpdateTransaction(ctx context.Context, txn *domain.Transaction) error {
	query := `
		UPDATE transactions
		SET status = $1, previous_status = $2, updated_at = $3, version = version + 1
		WHERE id = $4 AND version = $5`

	// NOTE: Depending on what fields changed, we might need a more dynamic query or massive update.
	// For simplicity, we update Status and Metadata fields primarily involved in transitions.

	tag, err := r.pool.Exec(ctx, query,
		txn.Status, txn.PreviousStatus, time.Now().UTC(),
		txn.ID, txn.Version,
	)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic locking failure: transaction modified concurrently")
	}

	txn.Version++
	return nil
}

// GetTransaction retrieves a transaction by ID
func (r *PostgresRepository) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `SELECT id, user_id, from_account_id, to_account_id, amount, currency, status FROM transactions WHERE id = $1`
	// Simplified fetch for brevity. In real app, fetch all fields.

	var t domain.Transaction
	var amount decimal.Decimal

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.UserID, &t.FromAccountID, &t.ToAccountID, &amount, &t.Currency, &t.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	t.Amount = amount

	return &t, nil
}
