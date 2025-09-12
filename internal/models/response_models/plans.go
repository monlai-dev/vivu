package response_models

import "time"

// Enhanced POI structure with more travel-focused details
type TravelPOI struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"` // e.g., "Restaurant", "Attraction", "Hotel"
	Tags        []string `json:"tags"`     // e.g., ["romantic", "scenic", "local-favorite"]
	Address     string   `json:"address,omitempty"`
	Rating      float32  `json:"rating,omitempty"`
	PriceLevel  string   `json:"price_level,omitempty"` // "$", "$$", "$$$", "$$$$"
	Duration    string   `json:"duration,omitempty"`    // "2-3 hours", "1 hour"
	Tips        string   `json:"tips,omitempty"`        // Special tips or notes
}

// Time block for activities with more context
type TimeBlock struct {
	Period      string `json:"period"`      // "Morning", "Afternoon", "Evening"
	StartTime   string `json:"start_time"`  // "09:00"
	EndTime     string `json:"end_time"`    // "12:00"
	Description string `json:"description"` // Narrative description of the time period
}

// Enhanced activity with travel context
type TravelActivity struct {
	Title         string      `json:"title"` // "City Discovery & French Colonial Vibes"
	TimeBlock     TimeBlock   `json:"time_block"`
	MainPOI       TravelPOI   `json:"main_poi"`
	SupportPOIs   []TravelPOI `json:"support_pois"`             // Additional POIs for this activity block
	Description   string      `json:"description"`              // Detailed narrative description
	Highlights    []string    `json:"highlights"`               // Key highlights of this activity
	TravelTips    []string    `json:"travel_tips,omitempty"`    // Practical tips
	EstimatedCost string      `json:"estimated_cost,omitempty"` // "200,000 - 400,000 VND"
}

// Accommodation details
type Accommodation struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"` // "Luxury Resort", "Boutique Hotel", "Hostel"
	Address     string   `json:"address"`
	Rating      float32  `json:"rating,omitempty"`
	PriceRange  string   `json:"price_range,omitempty"` // "1,500,000 - 2,500,000 VND/night"
	Highlights  []string `json:"highlights"`            // Key features
	BookingTips string   `json:"booking_tips,omitempty"`
	CheckIn     string   `json:"check_in,omitempty"`
	CheckOut    string   `json:"check_out,omitempty"`
}

// Transportation details
type Transportation struct {
	Method      string `json:"method"` // "Flight", "Bus", "Train", "Car"
	From        string `json:"from"`
	To          string `json:"to"`
	Duration    string `json:"duration"`
	Cost        string `json:"cost,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Tips        string `json:"tips,omitempty"`
	BookingInfo string `json:"booking_info,omitempty"`
}

// Enhanced daily plan with travel narrative
type TravelDayPlan struct {
	Day            int              `json:"day"`
	Date           string           `json:"date"`
	Title          string           `json:"title"`             // "Arrival & Da Lat City Discovery"
	Theme          string           `json:"theme"`             // "Charming streets, French colonial vibes"
	Location       string           `json:"location"`          // "Da Lat City Center"
	Weather        string           `json:"weather,omitempty"` // "Cool, 18-25°C"
	Overview       string           `json:"overview"`          // Day summary
	Activities     []TravelActivity `json:"activities"`
	Accommodation  *Accommodation   `json:"accommodation,omitempty"`
	Transportation []Transportation `json:"transportation,omitempty"`
	DailyTips      []string         `json:"daily_tips,omitempty"`
	DailyCost      string           `json:"daily_cost,omitempty"` // Estimated daily cost
}

// Complete travel itinerary
type TravelItinerary struct {
	Title         string          `json:"title"`               // "Da Lat, Vietnam – 2-Day Itinerary"
	Subtitle      string          `json:"subtitle"`            // "A breezy, romantic escape..."
	Duration      string          `json:"duration"`            // "2 days"
	Destination   string          `json:"destination"`         // "Da Lat, Vietnam"
	BestTime      string          `json:"best_time,omitempty"` // "Year-round, especially Oct-Mar"
	TravelStyle   []string        `json:"travel_style"`        // ["romantic", "nature", "cultural"]
	Overview      string          `json:"overview"`            // Trip overview
	Days          []TravelDayPlan `json:"days"`
	TotalCost     string          `json:"total_cost,omitempty"` // "2,000,000 - 4,000,000 VND per person"
	PackingTips   []string        `json:"packing_tips,omitempty"`
	GeneralTips   []string        `json:"general_tips,omitempty"`
	EmergencyInfo string          `json:"emergency_info,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Quick reference structures for different response types
type QuickItinerary struct {
	Destination string          `json:"destination"`
	Duration    string          `json:"duration"`
	Highlights  []string        `json:"highlights"`
	Days        []TravelDayPlan `json:"days"`
}

// For shorter responses or previews
type ItinerarySummary struct {
	Title         string   `json:"title"`
	Duration      string   `json:"duration"`
	Destination   string   `json:"destination"`
	Highlights    []string `json:"highlights"`
	KeyPOIs       []string `json:"key_pois"`
	EstimatedCost string   `json:"estimated_cost"`
}
