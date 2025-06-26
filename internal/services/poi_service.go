package services

import "vivu/internal/models/response_models"

type POIServiceInterface interface {
	GetPOIById(id string) (response_models.POI, error)
	GetPoisByProvince(province string, page, pageSize int) ([]response_models.POI, error)
}
