package response_models

type JourneyResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title" binding:"required"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Location  string `json:"location"`
}
