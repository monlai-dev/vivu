package db_models

import (
	"time"

	"github.com/google/uuid"
)

type Feedback struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"` // Reference to the user providing feedback
	Comment   string    `gorm:"type:text;not null"`
	Rating    int       `gorm:"type:int;not null;check:rating >= 1 AND rating <= 5"` // Rating between 1 and 5
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
