package db_models

import "github.com/google/uuid"

type Photo struct {
	BaseModel
	CheckInID uuid.UUID
	URL       string
}
