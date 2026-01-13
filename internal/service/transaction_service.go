package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/banking/transaction-service/internal/config"
	"github.com/banking/transaction-service/internal/domain"
	"github.com/banking/transaction-service/internal/events"
	"github.com/banking/transaction-service/internal/repository"
)

type TransactionService struct {
	repo     *repository.PostgresRepository
	producer *events.Producer
	logger   *zap.Logger
	config   config.TransactionConfig
}

func NewTransactionService(
	repo *repository.PostgresRepository,
	producer *events.Producer,
	logger *zap.Logger,
	cfg config.TransactionConfig,
) *TransactionService {
	return &TransactionService{
		repo:     repo,
		producer: producer,
		logger:   logger,
		config:   cfg,
	}
}

type CreateTransferRequest struct {
	UserID           uuid.UUID
	FromAccountID    uuid.UUID
	ToAccountID      uuid.UUID
	Amount           decimal.Decimal
	Currency         string
	TransferType     domain.TransferType
	InitiationMethod domain.InitiationMethod
	Memo             string
	Metadata         map[string]string // IP, Device, etc
}

// InitiateTransfer handles the initial request to transfer money
func (s *TransactionService) InitiateTransfer(ctx context.Context, req CreateTransferRequest) (*domain.Transaction, error) {
	s.logger.Info("Initiating transfer", zap.String("user_id", req.UserID.String()))

	// 1. Validate Amount
	if req.Amount.LessThanOrEqual(decimal.NewFromFloat(s.config.MinTransferAmount)) {
		return nil, fmt.Errorf("amount must be at least %f", s.config.MinTransferAmount)
	}
	if req.Amount.GreaterThan(decimal.NewFromFloat(s.config.MaxTransferAmount)) {
		return nil, fmt.Errorf("amount exceeds limit of %f", s.config.MaxTransferAmount)
	}

	// 2. Fetch FromAccount
	fromAccount, err := s.repo.GetAccount(ctx, req.FromAccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid source account: %w", err)
	}

	// 3. User Ownership Check
	if fromAccount.UserID != req.UserID {
		return nil, fmt.Errorf("account does not belong to user")
	}

	// 4. Validate FromAccount State
	if !fromAccount.CanTransfer() {
		return nil, fmt.Errorf("account is not active or KYC not approved")
	}

	// 5. Check Balance & Limits
	if !fromAccount.HasSufficientBalance(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}
	if !fromAccount.HasDailyLimitAvailable(req.Amount) {
		return nil, domain.ErrDailyLimitExceeded
	}

	// 6. Create Transaction (Pending)
	txn := domain.NewTransaction(
		req.UserID, req.FromAccountID, req.ToAccountID,
		req.Amount, req.Currency, req.TransferType, req.InitiationMethod, req.Memo,
	)

	// Add Metadata
	if req.Metadata != nil {
		txn.SetMetadata(
			req.Metadata["source_ip"],
			req.Metadata["user_agent"],
			req.Metadata["device_id"],
			req.Metadata["session_id"],
		)
	}

	// 7. Reserve Funds (Optimistic Locking)
	if err := fromAccount.Reserve(req.Amount); err != nil {
		return nil, err
	}

	// 8. Transactional Save (Ideally wrapped in DB Trx, simplified here to sequential calls)
	// In a real system, we must use tx.Begin() -> repo.WithTx(tx) ...
	if err := s.repo.UpdateAccount(ctx, fromAccount); err != nil {
		return nil, fmt.Errorf("failed to reserve funds: %w", err)
	}

	if err := s.repo.CreateTransaction(ctx, txn); err != nil {
		// Compensate: Release reservation? Or rely on periodic cleanup?
		// ideally rollback.
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	// 9. Publish Event (Async/Sync)
	event := events.NewTransactionInitiatedEvent(txn)
	// We use the producer from the existing code context
	if err := s.producer.PublishTransactionInitiated(ctx, event); err != nil {
		s.logger.Error("Failed to publish initiated event", zap.Error(err))
		// Don't fail the request, but log it. Background job should pick up stuck pending txns.
	}

	return txn, nil
}

// GetTransaction retrieves a transaction
func (s *TransactionService) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	return s.repo.GetTransaction(ctx, id)
}
