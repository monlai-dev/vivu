package db_models

import "gorm.io/datatypes"

type Account struct {
	BaseModel
	Name         string
	Email        string `gorm:"unique"`
	PasswordHash string
	Role         string `gorm:"default:'user'"`

	// Store the entire subscription object as JSON in case of changes
	SubscriptionSnapshot datatypes.JSON `gorm:"type:jsonb;default:'{}'"`

	Journeys []Journey      `gorm:"foreignKey:AccountID"`
	CheckIns []CheckIn      `gorm:"foreignKey:AccountID"`
	Subs     []Subscription `gorm:"foreignKey:AccountID"`
	Payments []Transaction  `gorm:"foreignKey:AccountID"`
}
