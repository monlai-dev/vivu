package db_models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TransactionStatus string

const (
	TxnStatusPending  TransactionStatus = "pending"
	TxnStatusPaid     TransactionStatus = "paid"
	TxnStatusFailed   TransactionStatus = "failed"
	TxnStatusRefunded TransactionStatus = "refunded"
)

type Transaction struct {
	BaseModel
	AccountID      uuid.UUID         `gorm:"index"`
	SubscriptionID *uuid.UUID        `gorm:"index"` // nullable for one-off purchases
	AmountMinor    int64             // e.g., 999 = $9.99
	Currency       string            `gorm:"size:3"` // ISO 4217 (e.g., "USD","VND")
	Status         TransactionStatus `gorm:"type:transaction_status;index"`

	// Gateway fields
	Provider         string `gorm:"index"`
	ProviderTxnID    string `gorm:"index"` // idempotency across webhooks
	PaymentMethodRef string // last4 / token ref (avoid PCI data)

	// Important timestamps (unix seconds)
	AuthorizedAt *int64
	PaidAt       *int64
	RefundedAt   *int64

	// Raw receipts, webhook payloads, failure reasons, etc.
	Receipt  datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	Metadata datatypes.JSON `gorm:"type:jsonb;default:'{}'"`

	Account      Account       `gorm:"foreignKey:AccountID"`
	Subscription *Subscription `gorm:"foreignKey:SubscriptionID"`
}
