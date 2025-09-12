package request_models

type QuizRequest struct {
	UserID    string            `json:"user_id"`
	SessionID string            `json:"session_id,omitempty"`
	Answers   map[string]string `json:"answers,omitempty"`
}

type QuizQuestion struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Type        string   `json:"type"` // "single_choice", "multiple_choice", "text", "range"
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Category    string   `json:"category"` // "destination", "budget", "activities", "accommodation", "dining", "travel_style"
	MinValue    *int     `json:"min_value,omitempty"`
	MaxValue    *int     `json:"max_value,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

type QuizStartRequest struct {
	UserID string `json:"user_id"`
}

type PlanOnlyRequest struct {
	SessionID string `json:"session_id"`
}
