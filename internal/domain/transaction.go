// Package domain contains the core business entities and value objects for the transaction service.
package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TransactionStatus represents the current state of a transaction
type TransactionStatus string

const (
	// StatusPending - Transaction created, funds reserved, waiting for analysis
	StatusPending TransactionStatus = "PENDING"
	// StatusAnalyzing - Fraud Service is analyzing the transaction
	StatusAnalyzing TransactionStatus = "ANALYZING"
	// StatusApproved - Fraud score < 50, automatic approval
	StatusApproved TransactionStatus = "APPROVED"
	// StatusSuspicious - Fraud score 50-80, needs manual review
	StatusSuspicious TransactionStatus = "SUSPICIOUS"
	// StatusWaitingReview - Waiting for fraud analyst to approve/reject
	StatusWaitingReview TransactionStatus = "WAITING_REVIEW"
	// StatusFinalizing - Approved, ready to deduct and transfer funds
	StatusFinalizing TransactionStatus = "FINALIZING"
	// StatusCompleted - Transfer finished, money moved, final state
	StatusCompleted TransactionStatus = "COMPLETED"
	// StatusRejecting - Transaction rejected, compensation in progress
	StatusRejecting TransactionStatus = "REJECTING"
	// StatusRejected - Transfer blocked, funds released, final state
	StatusRejected TransactionStatus = "REJECTED"
	// StatusCancelled - User cancelled the transfer
	StatusCancelled TransactionStatus = "CANCELLED"
)

// IsFinal returns true if the status is a terminal state
func (s TransactionStatus) IsFinal() bool {
	return s == StatusCompleted || s == StatusRejected || s == StatusCancelled
}

// CanTransitionTo returns true if the current status can transition to the target status
func (s TransactionStatus) CanTransitionTo(target TransactionStatus) bool {
	transitions := map[TransactionStatus][]TransactionStatus{
		StatusPending:       {StatusAnalyzing, StatusCancelled},
		StatusAnalyzing:     {StatusApproved, StatusSuspicious, StatusRejecting},
		StatusApproved:      {StatusFinalizing},
		StatusSuspicious:    {StatusWaitingReview},
		StatusWaitingReview: {StatusFinalizing, StatusRejecting, StatusCancelled},
		StatusFinalizing:    {StatusCompleted, StatusRejecting},
		StatusRejecting:     {StatusRejected},
	}

	allowed, ok := transitions[s]
	if !ok {
		return false
	}

	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// String returns the string representation of the status
func (s TransactionStatus) String() string {
	return string(s)
}

// FraudDecision represents the decision made by fraud analysis
type FraudDecision string

const (
	FraudDecisionApproved   FraudDecision = "APPROVED"
	FraudDecisionSuspicious FraudDecision = "SUSPICIOUS"
	FraudDecisionRejected   FraudDecision = "REJECTED"
)

// TransferType represents the type of transfer
type TransferType string

const (
	TransferTypeInternal TransferType = "INTERNAL" // Between accounts in same bank
	TransferTypeExternal TransferType = "EXTERNAL" // To external bank
	TransferTypeWire     TransferType = "WIRE"     // Wire transfer
	TransferTypeACH      TransferType = "ACH"      // ACH transfer
)

// InitiationMethod represents how the transfer was initiated
type InitiationMethod string

const (
	InitiationMethodWeb    InitiationMethod = "WEB"
	InitiationMethodMobile InitiationMethod = "MOBILE"
	InitiationMethodAPI    InitiationMethod = "API"
	InitiationMethodBranch InitiationMethod = "BRANCH"
)

// Transaction represents a money transfer between accounts
type Transaction struct {
	// Core identifiers
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	FromAccountID uuid.UUID `json:"from_account_id"`
	ToAccountID   uuid.UUID `json:"to_account_id"`

	// Financial details
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
	ReservedAmount decimal.Decimal `json:"reserved_amount"`

	// Status tracking
	Status         TransactionStatus `json:"status"`
	PreviousStatus TransactionStatus `json:"previous_status,omitempty"`

	// Transfer details
	TransferType     TransferType     `json:"transfer_type"`
	InitiationMethod InitiationMethod `json:"initiation_method"`
	Memo             string           `json:"memo,omitempty"`
	Reference        string           `json:"reference"`

	// Fraud analysis
	FraudScore      *float64       `json:"fraud_score,omitempty"`
	FraudDecision   *FraudDecision `json:"fraud_decision,omitempty"`
	FraudReasons    []string       `json:"fraud_reasons,omitempty"`
	FraudAnalyzedAt *time.Time     `json:"fraud_analyzed_at,omitempty"`

	// Review details (for manual review)
	ReviewerID  *uuid.UUID `json:"reviewer_id,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	ReviewNotes string     `json:"review_notes,omitempty"`

	// Metadata
	SourceIP  string                 `json:"source_ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	DeviceID  string                 `json:"device_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`

	// Compensation tracking
	CompensationStatus *CompensationStatus `json:"compensation_status,omitempty"`
	CompensationReason string              `json:"compensation_reason,omitempty"`
	CompensatedAt      *time.Time          `json:"compensated_at,omitempty"`

	// Error tracking
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Timestamps
	InitiatedAt time.Time  `json:"initiated_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	RejectedAt  *time.Time `json:"rejected_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Version     int        `json:"version"` // Optimistic locking

	// Processing metrics
	ProcessingTimeMs int64 `json:"processing_time_ms,omitempty"`
}

// CompensationStatus represents the status of compensation for a failed transaction
type CompensationStatus string

const (
	CompensationStatusPending    CompensationStatus = "PENDING"
	CompensationStatusInProgress CompensationStatus = "IN_PROGRESS"
	CompensationStatusCompleted  CompensationStatus = "COMPLETED"
	CompensationStatusFailed     CompensationStatus = "FAILED"
)

// NewTransaction creates a new transaction with initial values
func NewTransaction(
	userID, fromAccountID, toAccountID uuid.UUID,
	amount decimal.Decimal,
	currency string,
	transferType TransferType,
	initiationMethod InitiationMethod,
	memo string,
) *Transaction {
	now := time.Now().UTC()
	return &Transaction{
		ID:               uuid.New(),
		UserID:           userID,
		FromAccountID:    fromAccountID,
		ToAccountID:      toAccountID,
		Amount:           amount,
		Currency:         currency,
		ReservedAmount:   amount,
		Status:           StatusPending,
		TransferType:     transferType,
		InitiationMethod: initiationMethod,
		Memo:             memo,
		Reference:        generateReference(),
		InitiatedAt:      now,
		CreatedAt:        now,
		UpdatedAt:        now,
		Version:          1,
	}
}

// TransitionTo transitions the transaction to a new status with validation
func (t *Transaction) TransitionTo(newStatus TransactionStatus) error {
	if !t.Status.CanTransitionTo(newStatus) {
		return &InvalidTransitionError{
			Current: t.Status,
			Target:  newStatus,
		}
	}

	t.PreviousStatus = t.Status
	t.Status = newStatus
	t.UpdatedAt = time.Now().UTC()
	t.Version++

	// Set completion timestamps
	switch newStatus {
	case StatusApproved:
		now := time.Now().UTC()
		t.ApprovedAt = &now
	case StatusCompleted:
		now := time.Now().UTC()
		t.CompletedAt = &now
	case StatusRejected:
		now := time.Now().UTC()
		t.RejectedAt = &now
	}

	return nil
}

// SetFraudAnalysis sets the fraud analysis results
func (t *Transaction) SetFraudAnalysis(score float64, decision FraudDecision, reasons []string) {
	t.FraudScore = &score
	t.FraudDecision = &decision
	t.FraudReasons = reasons
	now := time.Now().UTC()
	t.FraudAnalyzedAt = &now
	t.UpdatedAt = now
}

// SetReviewResult sets the manual review results
func (t *Transaction) SetReviewResult(reviewerID uuid.UUID, approved bool, notes string) {
	t.ReviewerID = &reviewerID
	now := time.Now().UTC()
	t.ReviewedAt = &now
	t.ReviewNotes = notes
	t.UpdatedAt = now

	if approved {
		t.FraudDecision = ptr(FraudDecisionApproved)
	} else {
		t.FraudDecision = ptr(FraudDecisionRejected)
	}
}

// SetCompensation sets compensation details
func (t *Transaction) SetCompensation(status CompensationStatus, reason string) {
	t.CompensationStatus = &status
	t.CompensationReason = reason
	if status == CompensationStatusCompleted {
		now := time.Now().UTC()
		t.CompensatedAt = &now
	}
	t.UpdatedAt = time.Now().UTC()
}

// SetError sets error details
func (t *Transaction) SetError(code, message string) {
	t.ErrorCode = code
	t.ErrorMessage = message
	t.UpdatedAt = time.Now().UTC()
}

// SetMetadata sets request metadata
func (t *Transaction) SetMetadata(sourceIP, userAgent, deviceID, sessionID string) {
	t.SourceIP = sourceIP
	t.UserAgent = userAgent
	t.DeviceID = deviceID
	t.SessionID = sessionID
}

// CalculateProcessingTime calculates and sets the processing time
func (t *Transaction) CalculateProcessingTime() {
	if t.CompletedAt != nil {
		t.ProcessingTimeMs = t.CompletedAt.Sub(t.InitiatedAt).Milliseconds()
	} else if t.RejectedAt != nil {
		t.ProcessingTimeMs = t.RejectedAt.Sub(t.InitiatedAt).Milliseconds()
	}
}

// InvalidTransitionError represents an invalid state transition
type InvalidTransitionError struct {
	Current TransactionStatus
	Target  TransactionStatus
}

func (e *InvalidTransitionError) Error() string {
	return "invalid state transition from " + string(e.Current) + " to " + string(e.Target)
}

// generateReference creates a unique reference number
func generateReference() string {
	return "TXN" + uuid.New().String()[:8]
}

// ptr is a helper to get a pointer to a value
func ptr[T any](v T) *T {
	return &v
}
