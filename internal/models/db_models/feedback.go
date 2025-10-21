package db_models

import (
	"github.com/google/uuid"
)

type Feedback struct {
	BaseModel
	UserID  uuid.UUID `gorm:"type:uuid;not null"` // Reference to the user providing feedback
	Comment string    `gorm:"type:text;not null"`
	Rating  int       `gorm:"type:int;not null;check:rating >= 1 AND rating <= 5"` // Rating between 1 and 5

}
