// Package domain contains the core business entities and value objects for the transaction service.
package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AccountStatus represents the status of an account
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "ACTIVE"
	AccountStatusFrozen    AccountStatus = "FROZEN"
	AccountStatusSuspended AccountStatus = "SUSPENDED"
	AccountStatusClosed    AccountStatus = "CLOSED"
)

// AccountType represents the type of account
type AccountType string

const (
	AccountTypeChecking AccountType = "CHECKING"
	AccountTypeSavings  AccountType = "SAVINGS"
	AccountTypeMoney    AccountType = "MONEY_MARKET"
)

// Account represents a bank account with balance tracking
type Account struct {
	ID            uuid.UUID   `json:"id"`
	UserID        uuid.UUID   `json:"user_id"`
	AccountNumber string      `json:"account_number"`
	AccountType   AccountType `json:"account_type"`
	Currency      string      `json:"currency"`

	// Balance tracking
	Balance   decimal.Decimal `json:"balance"`   // Actual balance
	Reserved  decimal.Decimal `json:"reserved"`  // Funds on hold
	Available decimal.Decimal `json:"available"` // Balance - Reserved

	// Limits
	DailyLimit  decimal.Decimal `json:"daily_limit"`   // Max per day
	DailyUsed   decimal.Decimal `json:"daily_used"`    // Used today
	PerTxnLimit decimal.Decimal `json:"per_txn_limit"` // Max per transaction

	// Status
	Status      AccountStatus `json:"status"`
	KYCApproved bool          `json:"kyc_approved"`

	// Timestamps
	LastTxnAt    *time.Time `json:"last_txn_at,omitempty"`
	DailyResetAt time.Time  `json:"daily_reset_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Version      int        `json:"version"`
}

// CanTransfer returns true if the account can initiate transfers
func (a *Account) CanTransfer() bool {
	return a.Status == AccountStatusActive && a.KYCApproved
}

// HasSufficientBalance returns true if the account has enough available balance
func (a *Account) HasSufficientBalance(amount decimal.Decimal) bool {
	return a.Available.GreaterThanOrEqual(amount)
}

// HasDailyLimitAvailable returns true if the daily limit allows this transfer
func (a *Account) HasDailyLimitAvailable(amount decimal.Decimal) bool {
	remaining := a.DailyLimit.Sub(a.DailyUsed)
	return remaining.GreaterThanOrEqual(amount)
}

// IsWithinPerTxnLimit returns true if the amount is within per-transaction limit
func (a *Account) IsWithinPerTxnLimit(amount decimal.Decimal) bool {
	return amount.LessThanOrEqual(a.PerTxnLimit)
}

// Reserve reserves funds for a pending transaction
func (a *Account) Reserve(amount decimal.Decimal) error {
	if !a.HasSufficientBalance(amount) {
		return ErrInsufficientFunds
	}

	a.Reserved = a.Reserved.Add(amount)
	a.Available = a.Balance.Sub(a.Reserved)
	a.DailyUsed = a.DailyUsed.Add(amount)
	a.UpdatedAt = time.Now().UTC()
	a.Version++
	return nil
}

// ReleaseReservation releases reserved funds back to available balance
func (a *Account) ReleaseReservation(amount decimal.Decimal) {
	a.Reserved = a.Reserved.Sub(amount)
	if a.Reserved.IsNegative() {
		a.Reserved = decimal.Zero
	}
	a.Available = a.Balance.Sub(a.Reserved)
	a.DailyUsed = a.DailyUsed.Sub(amount)
	if a.DailyUsed.IsNegative() {
		a.DailyUsed = decimal.Zero
	}
	a.UpdatedAt = time.Now().UTC()
	a.Version++
}

// Debit debits the account (finalizes the reserved amount)
func (a *Account) Debit(amount decimal.Decimal) error {
	if a.Reserved.LessThan(amount) {
		return ErrReservationMismatch
	}

	a.Balance = a.Balance.Sub(amount)
	a.Reserved = a.Reserved.Sub(amount)
	a.Available = a.Balance.Sub(a.Reserved)
	now := time.Now().UTC()
	a.LastTxnAt = &now
	a.UpdatedAt = now
	a.Version++
	return nil
}

// Credit credits the account
func (a *Account) Credit(amount decimal.Decimal) {
	a.Balance = a.Balance.Add(amount)
	a.Available = a.Balance.Sub(a.Reserved)
	now := time.Now().UTC()
	a.LastTxnAt = &now
	a.UpdatedAt = now
	a.Version++
}

// ResetDailyLimit resets the daily used amount
func (a *Account) ResetDailyLimit() {
	a.DailyUsed = decimal.Zero
	a.DailyResetAt = time.Now().UTC()
	a.UpdatedAt = a.DailyResetAt
}

// AccountValidation represents validation results for an account
type AccountValidation struct {
	AccountID     uuid.UUID `json:"account_id"`
	Exists        bool      `json:"exists"`
	IsActive      bool      `json:"is_active"`
	IsKYCApproved bool      `json:"is_kyc_approved"`
	CanReceive    bool      `json:"can_receive"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// IsValid returns true if all validations pass
func (v *AccountValidation) IsValid() bool {
	return v.Exists && v.IsActive && v.IsKYCApproved && v.CanReceive
}

// BalanceInfo represents account balance information for display
type BalanceInfo struct {
	AccountID      uuid.UUID       `json:"account_id"`
	Balance        decimal.Decimal `json:"balance"`
	Reserved       decimal.Decimal `json:"reserved"`
	Available      decimal.Decimal `json:"available"`
	DailyLimit     decimal.Decimal `json:"daily_limit"`
	DailyUsed      decimal.Decimal `json:"daily_used"`
	DailyRemaining decimal.Decimal `json:"daily_remaining"`
	Currency       string          `json:"currency"`
	AsOf           time.Time       `json:"as_of"`
}

// NewBalanceInfo creates a BalanceInfo from an Account
func NewBalanceInfo(a *Account) *BalanceInfo {
	return &BalanceInfo{
		AccountID:      a.ID,
		Balance:        a.Balance,
		Reserved:       a.Reserved,
		Available:      a.Available,
		DailyLimit:     a.DailyLimit,
		DailyUsed:      a.DailyUsed,
		DailyRemaining: a.DailyLimit.Sub(a.DailyUsed),
		Currency:       a.Currency,
		AsOf:           time.Now().UTC(),
	}
}
