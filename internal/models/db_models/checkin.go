package db_models

import "github.com/google/uuid"

type CheckIn struct {
	BaseModel
	UserID    uuid.UUID
	JourneyID uuid.UUID
	POIID     uuid.UUID
	Notes     string

	Photos []Photo
}
