package request_models

import "github.com/google/uuid"

type CreatePoiRequest struct {
	Name         string     `json:"name"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Category     *uuid.UUID `json:"category"`
	Province     uuid.UUID  `json:"province"`
	OpeningHours string     `json:"opening_hours"`
	ContactInfo  string     `json:"contact_info"`
	Address      string     `json:"address"`

	PoiDetails *PoiDetails `json:"poi_details"`
}

type PoiDetails struct {
	Description string   `json:"description"`
	Image       []string `json:"images"`
}

type UpdatePoiRequest struct {
	ID           uuid.UUID  `json:"id" binding:"required,uuid4"`
	Name         string     `json:"name"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Category     *uuid.UUID `json:"category"`
	Province     uuid.UUID  `json:"province"`
	OpeningHours string     `json:"opening_hours"`
	ContactInfo  string     `json:"contact_info"`
	Address      string     `json:"address"`

	PoiDetails *PoiDetails `json:"poi_details"`
}

type DeletePoiRequest struct {
	ID uuid.UUID `json:"id" binding:"required,uuid4"`
}
