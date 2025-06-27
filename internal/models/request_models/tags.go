package request_models

type ListTagsRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type CreateTagRequest struct {
	Vi   string `json:"vi" binding:"required"`
	En   string `json:"en" binding:"required"`
	Icon string `json:"icon" binding:"required"`
}
