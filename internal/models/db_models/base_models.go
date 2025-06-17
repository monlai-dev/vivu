package db_models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"time"
)

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CreatedAt int64          `gorm:"autoCreateTime"`
	UpdatedAt int64          `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Hooks to manage int64 timestamps
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now().Unix()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

func (b *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = time.Now().Unix()
	return nil
}
