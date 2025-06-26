package request_models

type ListTagsRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
