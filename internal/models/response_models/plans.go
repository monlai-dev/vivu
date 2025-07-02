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
