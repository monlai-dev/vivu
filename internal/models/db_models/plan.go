package db_models

import (
	"gorm.io/datatypes"
)

type Plan struct {
	BaseModel
	Code            string `gorm:"uniqueIndex"` // e.g., "basic", "pro_monthly", "pro_yearly"
	Name            string
	Description     *string
	BackgroundImage string
	Period          BillingPeriod `gorm:"type:billing_period"` // "month" | "year"
	PriceMinor      int64         // 999 = $9.99
	Currency        string        `gorm:"size:3"` // "USD", "VND"
	TrialDays       int32         `gorm:"default:0"`
	IsActive        bool          `gorm:"default:true"`
	// Optional: feature flags, limits, etc.
	Features datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
}
