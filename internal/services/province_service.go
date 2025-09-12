package services

import (
	"context"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type ProvinceServiceInterface interface {
	GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.ProvinceResponse, error)
}

type ProvinceService struct {
	provinceRepository repositories.ProvinceRepository
}

func NewProvinceService(provinceRepository repositories.ProvinceRepository) ProvinceServiceInterface {
	return &ProvinceService{
		provinceRepository: provinceRepository,
	}
}

func (p *ProvinceService) GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.ProvinceResponse, error) {
	provinces, err := p.provinceRepository.GetListOfProvinces(ctx, page, pageSize)
	if err != nil {
		return nil, utils.ErrDatabaseError
	}

	if len(provinces) == 0 {
		return []response_models.ProvinceResponse{}, utils.ErrTagNotFound
	}

	provinceResponse := make([]response_models.ProvinceResponse, 0, len(provinces))

	for _, province := range provinces {
		provinceResponse = append(provinceResponse, response_models.ProvinceResponse{
			ID:   province.ID.String(),
			Name: province.Name,
		})
	}

	return provinceResponse, nil
}
