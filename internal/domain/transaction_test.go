package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_TransitionTo(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus TransactionStatus
		targetStatus  TransactionStatus
		wantErr       bool
	}{
		{
			name:          "Valid: Pending to Analyzing",
			initialStatus: StatusPending,
			targetStatus:  StatusAnalyzing,
			wantErr:       false,
		},
		{
			name:          "Invalid: Pending to Approved",
			initialStatus: StatusPending,
			targetStatus:  StatusApproved,
			wantErr:       true,
		},
		{
			name:          "Valid: Analyzing to Approved",
			initialStatus: StatusAnalyzing,
			targetStatus:  StatusApproved,
			wantErr:       false,
		},
		{
			name:          "Valid: Analyzing to Suspicious",
			initialStatus: StatusAnalyzing,
			targetStatus:  StatusSuspicious,
			wantErr:       false,
		},
		{
			name:          "Invalid: Completed to Pending",
			initialStatus: StatusCompleted,
			targetStatus:  StatusPending,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := &Transaction{
				Status: tt.initialStatus,
			}
			err := txn.TransitionTo(tt.targetStatus)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.targetStatus, txn.Status)
				assert.Equal(t, tt.initialStatus, txn.PreviousStatus)
				assert.False(t, txn.UpdatedAt.IsZero())
			}
		})
	}
}

func TestAccount_Reserve(t *testing.T) {
	acc := &Account{
		Balance:   decimal.NewFromFloat(100.0),
		Reserved:  decimal.Zero,
		Available: decimal.NewFromFloat(100.0),
		DailyUsed: decimal.Zero,
	}

	err := acc.Reserve(decimal.NewFromFloat(50.0))
	assert.NoError(t, err)
	assert.True(t, acc.Reserved.Equal(decimal.NewFromFloat(50.0)))
	assert.True(t, acc.Available.Equal(decimal.NewFromFloat(50.0)))

	err = acc.Reserve(decimal.NewFromFloat(60.0))
	assert.Error(t, err) // Insufficient funds
}

func TestNewTransaction(t *testing.T) {
	uid := uuid.New()
	txn := NewTransaction(uid, uuid.New(), uuid.New(), decimal.NewFromFloat(100), "USD", TransferTypeInternal, InitiationMethodAPI, "test")

	assert.NotNil(t, txn)
	assert.Equal(t, StatusPending, txn.Status)
	assert.False(t, txn.CreatedAt.IsZero())
	assert.Equal(t, "USD", txn.Currency)
}

func TestAccount_DailyLimit(t *testing.T) {
	acc := &Account{
		DailyLimit: decimal.NewFromFloat(1000.0),
		DailyUsed:  decimal.Zero,
	}

	assert.True(t, acc.HasDailyLimitAvailable(decimal.NewFromFloat(500)))
	assert.False(t, acc.HasDailyLimitAvailable(decimal.NewFromFloat(1500)))
}
