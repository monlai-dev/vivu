package services

import (
	"context"
	"github.com/google/uuid"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type POIServiceInterface interface {
	GetPOIById(id string, ctx context.Context) (response_models.POI, error)
	GetPoisByProvince(province string, page, pageSize int, ctx context.Context) ([]response_models.POI, error)
}

type PoiService struct {
	poiRepository repositories.POIRepository
}

func (p *PoiService) GetPOIById(id string, ctx context.Context) (response_models.POI, error) {
	poi, err := p.poiRepository.GetByIDWithDetails(ctx, id)
	if err != nil {
		return response_models.POI{}, utils.ErrDatabaseError
	}

	if poi == nil {
		return response_models.POI{}, utils.ErrPOINotFound
	}

	var poiDetails *response_models.PoiDetails
	if poi.Details.ID != uuid.Nil {
		poiDetails = &response_models.PoiDetails{
			ID:          poi.Details.ID.String(),
			Description: poi.Description, // or poi.Details.Description if preferred
			Image:       poi.Details.Images,
		}
	}

	return response_models.POI{
		ID:           poi.ID.String(),
		Name:         poi.Name,
		Latitude:     poi.Latitude,
		Longitude:    poi.Longitude,
		Category:     poi.Category.Name,
		OpeningHours: poi.OpeningHours,
		ContactInfo:  poi.ContactInfo,
		Address:      poi.Address,
		PoiDetails:   poiDetails,
	}, nil
}

func (p *PoiService) GetPoisByProvince(province string, page, pageSize int, ctx context.Context) ([]response_models.POI, error) {

	pois, err := p.poiRepository.ListPoisByProvinceId(ctx, province, page, pageSize)
	if err != nil {
		return nil, utils.ErrDatabaseError
	}

	if len(pois) == 0 {
		return []response_models.POI{}, utils.ErrPOINotFound
	}

	poiResponses := make([]response_models.POI, 0, len(pois))

	for _, poi := range pois {
		var poiDetails *response_models.PoiDetails
		if poi.Details.ID != uuid.Nil {

			poiDetails = &response_models.PoiDetails{
				ID:          poi.Details.ID.String(),
				Description: poi.Description,
				Image:       poi.Details.Images,
			}
		}

		poiResponses = append(poiResponses, response_models.POI{
			ID:           poi.ID.String(),
			Name:         poi.Name,
			Latitude:     poi.Latitude,
			Longitude:    poi.Longitude,
			Category:     poi.Category.Name,
			OpeningHours: poi.OpeningHours,
			ContactInfo:  poi.ContactInfo,
			Address:      poi.Address,
			PoiDetails:   poiDetails,
		})
	}

	return poiResponses, nil
}

func NewPOIService(poiRepository repositories.POIRepository) POIServiceInterface {
	return &PoiService{
		poiRepository: poiRepository,
	}
}
