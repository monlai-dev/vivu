package response_models

type POI struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Latitude     float64     `json:"latitude"`
	Longitude    float64     `json:"longitude"`
	Category     string      `json:"category"`
	OpeningHours string      `json:"opening_hours"`
	ContactInfo  string      `json:"contact_info"`
	Address      string      `json:"address"`
	PoiDetails   *PoiDetails `json:"poi_details"`

	DistanceToNextMeters *int   `json:"distance_to_next_meters,omitempty"`
	NextLegMapURL        string `json:"next_leg_map_url,omitempty"`
}

type PoiDetails struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Image       []string `json:"images"`
}
