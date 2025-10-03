package db_models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type SubscriptionStatus string

const (
	SubStatusTrialing SubscriptionStatus = "trialing"
	SubStatusActive   SubscriptionStatus = "active"
	SubStatusPastDue  SubscriptionStatus = "past_due"
	SubStatusCanceled SubscriptionStatus = "canceled"
	SubStatusExpired  SubscriptionStatus = "expired"
)

type BillingPeriod string

const (
	PeriodMonth BillingPeriod = "month"
	PeriodYear  BillingPeriod = "year"
)

type Subscription struct {
	BaseModel
	AccountID uuid.UUID `gorm:"index"`
	PlanID    uuid.UUID `gorm:"index"`

	Status     SubscriptionStatus `gorm:"type:subscription_status;index"`
	StartsAt   int64              `gorm:"not null"`
	EndsAt     int64              `gorm:"not null"`
	CanceledAt *int64
	AutoRenew  bool `gorm:"default:true"`

	// Optional: couple to payment provider (keep if you bill through Stripe/PayPal)
	Provider           string `gorm:"index"` // "stripe","paypal","local"
	ProviderCustomerID string `gorm:"index"`
	ProviderSubID      string `gorm:"uniqueIndex"`

	Metadata datatypes.JSON `gorm:"type:jsonb;default:'{}'"`

	Account Account `gorm:"foreignKey:AccountID"`
	Plan    Plan    `gorm:"foreignKey:PlanID"`
}
