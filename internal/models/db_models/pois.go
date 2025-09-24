package db_models

import "github.com/google/uuid"

type POI struct {
	BaseModel
	Name         string
	Latitude     float64
	Longitude    float64
	ProvinceID   uuid.UUID
	CategoryID   uuid.UUID
	Category     Category `gorm:"foreignKey:CategoryID"`
	Status       string
	OpeningHours string
	ContactInfo  string
	Description  string
	Address      string
	Province     Province          // Add this relationship
	Details      POIDetail         `gorm:"foreignKey:POIID"`
	Tags         []*Tag            `gorm:"many2many:poi_tags"`
	Activities   []JourneyActivity `gorm:"foreignKey:SelectedPOIID"`
	CheckIns     []CheckIn
}

type POISearchDoc struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	ProvinceID string  `json:"provinceId"`
	CategoryID string  `json:"categoryId"`
	Status     string  `json:"status"`
	Address    string  `json:"address"`
}

func ToSearchDoc(p *POI) POISearchDoc {
	return POISearchDoc{
		ID:         p.ID.String(),
		Name:       p.Name,
		Latitude:   p.Latitude,
		Longitude:  p.Longitude,
		ProvinceID: p.ProvinceID.String(),
		CategoryID: p.CategoryID.String(),
		Status:     p.Status,
		Address:    p.Address,
	}
}
