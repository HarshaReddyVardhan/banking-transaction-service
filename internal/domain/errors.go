// Package domain contains the core business entities and value objects for the transaction service.
package domain

import "errors"

// Domain errors for transaction processing
var (
	// Account errors
	ErrAccountNotFound       = errors.New("account not found")
	ErrAccountInactive       = errors.New("account is not active")
	ErrAccountFrozen         = errors.New("account is frozen")
	ErrAccountClosed         = errors.New("account is closed")
	ErrAccountNotKYCApproved = errors.New("account KYC not approved")
	ErrAccountCannotReceive  = errors.New("account cannot receive transfers")

	// Balance errors
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrReservationMismatch = errors.New("reservation amount mismatch")
	ErrDailyLimitExceeded  = errors.New("daily transfer limit exceeded")
	ErrPerTxnLimitExceeded = errors.New("per-transaction limit exceeded")

	// Transaction errors
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrInvalidAmount       = errors.New("invalid transfer amount")
	ErrAmountTooSmall      = errors.New("transfer amount too small")
	ErrAmountTooLarge      = errors.New("transfer amount exceeds maximum")
	ErrSameAccount         = errors.New("cannot transfer to same account")
	ErrInvalidCurrency     = errors.New("invalid or unsupported currency")
	ErrCurrencyMismatch    = errors.New("currency mismatch between accounts")

	// State errors
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrTransactionFinalized   = errors.New("transaction already finalized")
	ErrTransactionCancelled   = errors.New("transaction already cancelled")
	ErrOptimisticLock         = errors.New("concurrent modification detected")

	// Fraud errors
	ErrFraudDetected        = errors.New("fraud detected")
	ErrFraudAnalysisTimeout = errors.New("fraud analysis timeout")
	ErrManualReviewRequired = errors.New("manual review required")

	// Authorization errors
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrAccountOwnerMismatch = errors.New("account owner mismatch")

	// Rate limiting errors
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrTooManyRequests   = errors.New("too many requests")

	// Validation errors
	ErrValidationFailed    = errors.New("validation failed")
	ErrInvalidRecipient    = errors.New("invalid recipient")
	ErrSanctionedRecipient = errors.New("recipient on sanctions list")

	// System errors
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
	ErrDatabaseError      = errors.New("database error")
	ErrKafkaError         = errors.New("kafka error")
	ErrRedisError         = errors.New("redis error")
)

// ErrorCode maps errors to API error codes
type ErrorCode string

const (
	CodeAccountNotFound ErrorCode = "ACCOUNT_NOT_FOUND"
	CodeAccountInactive ErrorCode = "ACCOUNT_INACTIVE"
	CodeAccountFrozen   ErrorCode = "ACCOUNT_FROZEN"
	CodeAccountClosed   ErrorCode = "ACCOUNT_CLOSED"
	CodeAccountNotKYC   ErrorCode = "ACCOUNT_NOT_KYC_APPROVED"

	CodeInsufficientFunds   ErrorCode = "INSUFFICIENT_FUNDS"
	CodeDailyLimitExceeded  ErrorCode = "DAILY_LIMIT_EXCEEDED"
	CodePerTxnLimitExceeded ErrorCode = "PER_TXN_LIMIT_EXCEEDED"

	CodeTransactionNotFound ErrorCode = "TRANSACTION_NOT_FOUND"
	CodeInvalidAmount       ErrorCode = "INVALID_AMOUNT"
	CodeAmountTooSmall      ErrorCode = "AMOUNT_TOO_SMALL"
	CodeAmountTooLarge      ErrorCode = "AMOUNT_TOO_LARGE"
	CodeSameAccount         ErrorCode = "SAME_ACCOUNT"
	CodeInvalidCurrency     ErrorCode = "INVALID_CURRENCY"

	CodeInvalidStateTransition ErrorCode = "INVALID_STATE_TRANSITION"
	CodeTransactionFinalized   ErrorCode = "TRANSACTION_FINALIZED"
	CodeOptimisticLock         ErrorCode = "CONCURRENT_MODIFICATION"

	CodeFraudDetected        ErrorCode = "FRAUD_DETECTED"
	CodeManualReviewRequired ErrorCode = "MANUAL_REVIEW_REQUIRED"

	CodeUnauthorized         ErrorCode = "UNAUTHORIZED"
	CodeForbidden            ErrorCode = "FORBIDDEN"
	CodeAccountOwnerMismatch ErrorCode = "ACCOUNT_OWNER_MISMATCH"

	CodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"

	CodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	CodeSanctionedRecipient ErrorCode = "SANCTIONED_RECIPIENT"

	CodeInternalError      ErrorCode = "INTERNAL_ERROR"
	CodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// ErrorToCode maps domain errors to error codes
func ErrorToCode(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrAccountNotFound):
		return CodeAccountNotFound
	case errors.Is(err, ErrAccountInactive):
		return CodeAccountInactive
	case errors.Is(err, ErrAccountFrozen):
		return CodeAccountFrozen
	case errors.Is(err, ErrAccountClosed):
		return CodeAccountClosed
	case errors.Is(err, ErrAccountNotKYCApproved):
		return CodeAccountNotKYC
	case errors.Is(err, ErrInsufficientFunds):
		return CodeInsufficientFunds
	case errors.Is(err, ErrDailyLimitExceeded):
		return CodeDailyLimitExceeded
	case errors.Is(err, ErrPerTxnLimitExceeded):
		return CodePerTxnLimitExceeded
	case errors.Is(err, ErrTransactionNotFound):
		return CodeTransactionNotFound
	case errors.Is(err, ErrInvalidAmount):
		return CodeInvalidAmount
	case errors.Is(err, ErrAmountTooSmall):
		return CodeAmountTooSmall
	case errors.Is(err, ErrAmountTooLarge):
		return CodeAmountTooLarge
	case errors.Is(err, ErrSameAccount):
		return CodeSameAccount
	case errors.Is(err, ErrInvalidCurrency):
		return CodeInvalidCurrency
	case errors.Is(err, ErrInvalidStateTransition):
		return CodeInvalidStateTransition
	case errors.Is(err, ErrTransactionFinalized):
		return CodeTransactionFinalized
	case errors.Is(err, ErrOptimisticLock):
		return CodeOptimisticLock
	case errors.Is(err, ErrFraudDetected):
		return CodeFraudDetected
	case errors.Is(err, ErrManualReviewRequired):
		return CodeManualReviewRequired
	case errors.Is(err, ErrUnauthorized):
		return CodeUnauthorized
	case errors.Is(err, ErrForbidden):
		return CodeForbidden
	case errors.Is(err, ErrAccountOwnerMismatch):
		return CodeAccountOwnerMismatch
	case errors.Is(err, ErrRateLimitExceeded):
		return CodeRateLimitExceeded
	case errors.Is(err, ErrValidationFailed):
		return CodeValidationFailed
	case errors.Is(err, ErrSanctionedRecipient):
		return CodeSanctionedRecipient
	case errors.Is(err, ErrServiceUnavailable):
		return CodeServiceUnavailable
	default:
		return CodeInternalError
	}
}

// DomainError wraps an error with additional context
type DomainError struct {
	Code    ErrorCode
	Message string
	Err     error
	Details map[string]interface{}
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new domain error
func NewDomainError(code ErrorCode, message string, err error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// WithDetails adds details to the error
func (e *DomainError) WithDetails(key string, value interface{}) *DomainError {
	e.Details[key] = value
	return e
}
