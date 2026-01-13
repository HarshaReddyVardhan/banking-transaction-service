// Package events contains event definitions for Kafka messaging.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/banking/transaction-service/internal/domain"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeTransactionInitiated     EventType = "TransactionInitiated"
	EventTypeTransactionAnalyzing     EventType = "TransactionAnalyzing"
	EventTypeTransactionApproved      EventType = "TransactionApproved"
	EventTypeTransactionSuspicious    EventType = "TransactionSuspicious"
	EventTypeTransactionWaitingReview EventType = "TransactionWaitingReview"
	EventTypeTransactionRejected      EventType = "TransactionRejected"
	EventTypeTransactionCompleted     EventType = "TransactionCompleted"
	EventTypeTransactionCompensated   EventType = "TransactionCompensated"
	EventTypeTransactionCancelled     EventType = "TransactionCancelled"
	EventTypeFraudAnalysisComplete    EventType = "FraudAnalysisComplete"
	EventTypeFraudReviewComplete      EventType = "FraudReviewComplete"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	EventID       uuid.UUID `json:"event_id"`
	EventType     EventType `json:"event_type"`
	Timestamp     time.Time `json:"timestamp"`
	Version       string    `json:"version"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CausationID   string    `json:"causation_id,omitempty"`
}

// NewBaseEvent creates a new base event
func NewBaseEvent(eventType EventType) BaseEvent {
	return BaseEvent{
		EventID:   uuid.New(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Version:   "1.0",
	}
}

// TransactionInitiatedEvent is published when a transfer is initiated
type TransactionInitiatedEvent struct {
	BaseEvent
	TransactionID uuid.UUID       `json:"transaction_id"`
	UserID        uuid.UUID       `json:"user_id"`
	FromAccountID uuid.UUID       `json:"from_account_id"`
	ToAccountID   uuid.UUID       `json:"to_account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	TransferType  string          `json:"transfer_type"`
	Reference     string          `json:"reference"`
	Memo          string          `json:"memo,omitempty"`
	Metadata      EventMetadata   `json:"metadata"`
}

// EventMetadata contains context about the event
type EventMetadata struct {
	SourceIP         string `json:"source_ip,omitempty"`
	UserDevice       string `json:"user_device,omitempty"`
	DeviceID         string `json:"device_id,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	InitiationMethod string `json:"initiation_method"`
	UserAgent        string `json:"user_agent,omitempty"`
}

// NewTransactionInitiatedEvent creates a new TransactionInitiated event
func NewTransactionInitiatedEvent(txn *domain.Transaction) *TransactionInitiatedEvent {
	return &TransactionInitiatedEvent{
		BaseEvent:     NewBaseEvent(EventTypeTransactionInitiated),
		TransactionID: txn.ID,
		UserID:        txn.UserID,
		FromAccountID: txn.FromAccountID,
		ToAccountID:   txn.ToAccountID,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		TransferType:  string(txn.TransferType),
		Reference:     txn.Reference,
		Memo:          txn.Memo,
		Metadata: EventMetadata{
			SourceIP:         txn.SourceIP,
			UserDevice:       txn.UserAgent,
			DeviceID:         txn.DeviceID,
			SessionID:        txn.SessionID,
			InitiationMethod: string(txn.InitiationMethod),
			UserAgent:        txn.UserAgent,
		},
	}
}

// TransactionWaitingReviewEvent is published when transaction needs manual review
type TransactionWaitingReviewEvent struct {
	BaseEvent
	TransactionID uuid.UUID       `json:"transaction_id"`
	UserID        uuid.UUID       `json:"user_id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	FraudScore    float64         `json:"fraud_score"`
	Decision      string          `json:"decision"`
	Reasons       []string        `json:"reasons,omitempty"`
}

// NewTransactionWaitingReviewEvent creates a new TransactionWaitingReview event
func NewTransactionWaitingReviewEvent(txn *domain.Transaction) *TransactionWaitingReviewEvent {
	var score float64
	if txn.FraudScore != nil {
		score = *txn.FraudScore
	}
	return &TransactionWaitingReviewEvent{
		BaseEvent:     NewBaseEvent(EventTypeTransactionWaitingReview),
		TransactionID: txn.ID,
		UserID:        txn.UserID,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		FraudScore:    score,
		Decision:      string(domain.FraudDecisionSuspicious),
		Reasons:       txn.FraudReasons,
	}
}

// TransactionApprovedEvent is published when transaction is approved
type TransactionApprovedEvent struct {
	BaseEvent
	TransactionID  uuid.UUID `json:"transaction_id"`
	UserID         uuid.UUID `json:"user_id"`
	FraudScore     float64   `json:"fraud_score"`
	Decision       string    `json:"decision"`
	ApprovalMethod string    `json:"approval_method"` // "auto" or "manual"
}

// NewTransactionApprovedEvent creates a new TransactionApproved event
func NewTransactionApprovedEvent(txn *domain.Transaction, method string) *TransactionApprovedEvent {
	var score float64
	if txn.FraudScore != nil {
		score = *txn.FraudScore
	}
	return &TransactionApprovedEvent{
		BaseEvent:      NewBaseEvent(EventTypeTransactionApproved),
		TransactionID:  txn.ID,
		UserID:         txn.UserID,
		FraudScore:     score,
		Decision:       string(domain.FraudDecisionApproved),
		ApprovalMethod: method,
	}
}

// TransactionRejectedEvent is published when transaction is rejected
type TransactionRejectedEvent struct {
	BaseEvent
	TransactionID      uuid.UUID       `json:"transaction_id"`
	UserID             uuid.UUID       `json:"user_id"`
	Amount             decimal.Decimal `json:"amount"`
	Currency           string          `json:"currency"`
	FraudScore         float64         `json:"fraud_score,omitempty"`
	Decision           string          `json:"decision"`
	Reason             string          `json:"reason"`
	CompensationAmount decimal.Decimal `json:"compensation_amount"`
	CompensationStatus string          `json:"compensation_status"`
}

// NewTransactionRejectedEvent creates a new TransactionRejected event
func NewTransactionRejectedEvent(txn *domain.Transaction, reason string) *TransactionRejectedEvent {
	var score float64
	if txn.FraudScore != nil {
		score = *txn.FraudScore
	}
	var compStatus string
	if txn.CompensationStatus != nil {
		compStatus = string(*txn.CompensationStatus)
	}
	return &TransactionRejectedEvent{
		BaseEvent:          NewBaseEvent(EventTypeTransactionRejected),
		TransactionID:      txn.ID,
		UserID:             txn.UserID,
		Amount:             txn.Amount,
		Currency:           txn.Currency,
		FraudScore:         score,
		Decision:           string(domain.FraudDecisionRejected),
		Reason:             reason,
		CompensationAmount: txn.ReservedAmount,
		CompensationStatus: compStatus,
	}
}

// TransactionCompletedEvent is published when transaction is completed
type TransactionCompletedEvent struct {
	BaseEvent
	TransactionID    uuid.UUID       `json:"transaction_id"`
	UserID           uuid.UUID       `json:"user_id"`
	FromAccountID    uuid.UUID       `json:"from_account_id"`
	ToAccountID      uuid.UUID       `json:"to_account_id"`
	Amount           decimal.Decimal `json:"amount"`
	Currency         string          `json:"currency"`
	Status           string          `json:"status"`
	FraudScore       float64         `json:"fraud_score,omitempty"`
	ProcessingTimeMs int64           `json:"processing_time_ms"`
	Reference        string          `json:"reference"`
}

// NewTransactionCompletedEvent creates a new TransactionCompleted event
func NewTransactionCompletedEvent(txn *domain.Transaction) *TransactionCompletedEvent {
	var score float64
	if txn.FraudScore != nil {
		score = *txn.FraudScore
	}
	return &TransactionCompletedEvent{
		BaseEvent:        NewBaseEvent(EventTypeTransactionCompleted),
		TransactionID:    txn.ID,
		UserID:           txn.UserID,
		FromAccountID:    txn.FromAccountID,
		ToAccountID:      txn.ToAccountID,
		Amount:           txn.Amount,
		Currency:         txn.Currency,
		Status:           string(domain.StatusCompleted),
		FraudScore:       score,
		ProcessingTimeMs: txn.ProcessingTimeMs,
		Reference:        txn.Reference,
	}
}

// TransactionCompensatedEvent is published when compensation completes
type TransactionCompensatedEvent struct {
	BaseEvent
	TransactionID  uuid.UUID       `json:"transaction_id"`
	UserID         uuid.UUID       `json:"user_id"`
	Reason         string          `json:"reason"`
	ReleasedAmount decimal.Decimal `json:"released_amount"`
	AccountID      uuid.UUID       `json:"account_id"`
}

// NewTransactionCompensatedEvent creates a new TransactionCompensated event
func NewTransactionCompensatedEvent(txn *domain.Transaction) *TransactionCompensatedEvent {
	return &TransactionCompensatedEvent{
		BaseEvent:      NewBaseEvent(EventTypeTransactionCompensated),
		TransactionID:  txn.ID,
		UserID:         txn.UserID,
		Reason:         txn.CompensationReason,
		ReleasedAmount: txn.ReservedAmount,
		AccountID:      txn.FromAccountID,
	}
}

// FraudAnalysisCompleteEvent is received from the Fraud Service
type FraudAnalysisCompleteEvent struct {
	BaseEvent
	TransactionID    uuid.UUID `json:"transaction_id"`
	FraudScore       float64   `json:"fraud_score"`
	Decision         string    `json:"decision"` // APPROVED, SUSPICIOUS, REJECTED
	Reasons          []string  `json:"reasons,omitempty"`
	RiskLevel        string    `json:"risk_level,omitempty"` // LOW, MEDIUM, HIGH, CRITICAL
	ProcessingTimeMs int64     `json:"processing_time_ms"`
	ModelVersion     string    `json:"model_version,omitempty"`
}

// FraudReviewCompleteEvent is received when manual review is completed
type FraudReviewCompleteEvent struct {
	BaseEvent
	TransactionID uuid.UUID `json:"transaction_id"`
	ReviewerID    uuid.UUID `json:"reviewer_id"`
	Decision      string    `json:"decision"` // APPROVED, REJECTED
	Notes         string    `json:"notes,omitempty"`
	ReviewTimeMs  int64     `json:"review_time_ms"`
}

// Topic returns the appropriate Kafka topic for the event
func (e EventType) Topic(cfg TopicConfig) string {
	switch e {
	case EventTypeTransactionInitiated:
		return cfg.TransferInitiatedTopic
	case EventTypeTransactionApproved:
		return cfg.TransferApprovedTopic
	case EventTypeTransactionRejected:
		return cfg.TransferRejectedTopic
	case EventTypeTransactionCompleted:
		return cfg.TransferCompletedTopic
	case EventTypeTransactionWaitingReview:
		return cfg.TransferWaitingTopic
	case EventTypeTransactionCompensated:
		return cfg.AuditTopic
	default:
		return cfg.AuditTopic
	}
}

// TopicConfig holds topic names for event routing
type TopicConfig struct {
	TransferInitiatedTopic string
	TransferApprovedTopic  string
	TransferRejectedTopic  string
	TransferCompletedTopic string
	TransferWaitingTopic   string
	AuditTopic             string
}

// MarshalJSON serializes the event to JSON
func (e *TransactionInitiatedEvent) MarshalJSON() ([]byte, error) {
	type Alias TransactionInitiatedEvent
	return json.Marshal((*Alias)(e))
}

// Key returns the partition key for Kafka
func (e *TransactionInitiatedEvent) Key() string {
	return e.TransactionID.String()
}

// Key returns the partition key for Kafka
func (e *TransactionCompletedEvent) Key() string {
	return e.TransactionID.String()
}

// Key returns the partition key for Kafka
func (e *TransactionRejectedEvent) Key() string {
	return e.TransactionID.String()
}

// Key returns the partition key for Kafka
func (e *TransactionWaitingReviewEvent) Key() string {
	return e.TransactionID.String()
}

// Key returns the partition key for Kafka
func (e *TransactionApprovedEvent) Key() string {
	return e.TransactionID.String()
}

// Key returns the partition key for Kafka
func (e *TransactionCompensatedEvent) Key() string {
	return e.TransactionID.String()
}
