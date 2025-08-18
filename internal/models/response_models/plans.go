package response_models

type ActivityPOI struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ProvinceID  string   `json:"province_id"`
	CategoryID  string   `json:"category_id"`
	Tags        []string `json:"tags"`
}

type ActivityPlanBlock struct {
	Activity     string        `json:"activity"`
	StartTime    string        `json:"start_time"`
	EndTime      string        `json:"end_time"`
	MainPOI      ActivityPOI   `json:"main_poi"`
	Alternatives []ActivityPOI `json:"alternatives"`
	WhatToDo     string        `json:"what_to_do"`
}

type StayOption struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	CheckIn  string `json:"check_in,omitempty"`
	CheckOut string `json:"check_out,omitempty"`
}

type DailyPlan struct {
	Day        int                 `json:"day"`
	Date       string              `json:"date,omitempty"`
	Stay       *StayOption         `json:"stay,omitempty"`
	Activities []ActivityPlanBlock `json:"activities"`
}
