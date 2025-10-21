package request_models

type UserInputPrompt struct {
	Province       string    `json:"province"`
	PoisId         []*string `json:"pois"`
	AmountOfPeople int       `json:"amount_of_people"`
	Budget         string    `json:"budget"`
	StartTime      int64     `json:"start_time"`
	EndTime        int64     `json:"end_time"`
	Tags           []string  `json:"tags"`
}

type UserInputWildcard struct {
	Prompt string `json:"prompt"`
}

type POISummary struct {
	ID,
	Name,
	Category string
	Description string
}

type AddFeedbackRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	Comment string `json:"comment" binding:"required"`
	Rating  int    `json:"rating" binding:"required"`
}
