package services

import (
	"context"
	"github.com/google/uuid"
	"log"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type POIServiceInterface interface {
	GetPOIById(id string, ctx context.Context) (response_models.POI, error)
	GetPoisByProvince(province string, page, pageSize int, ctx context.Context) ([]response_models.POI, error)
	CreatePois(pois request_models.CreatePoiRequest, ctx context.Context) error
	UpdatePoi(pois request_models.UpdatePoiRequest, ctx context.Context) error
	DeletePoi(id uuid.UUID, ctx context.Context) error
	ListPois(ctx context.Context, page, pageSize int) ([]db_models.POI, error)
	SearchPoiByNameAndProvince(name, provinceID string, page, pageSize int, ctx context.Context) ([]response_models.POI, error)
}

type PoiService struct {
	poiRepository repositories.POIRepository
}

func (p *PoiService) SearchPoiByNameAndProvince(name, provinceID string, page, pageSize int, ctx context.Context) ([]response_models.POI, error) {

	pois, err := p.poiRepository.SearchPoiByNameAndProvince(ctx, name, provinceID)
	if err != nil {
		log.Printf("Error searching POIs: %v", err)
		return nil, utils.ErrDatabaseError
	}

	if len(pois) == 0 {
		return []response_models.POI{}, nil
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

func (p *PoiService) ListPois(ctx context.Context, page, pageSize int) ([]db_models.POI, error) {

	pois, err := p.poiRepository.List(ctx, page, pageSize)
	if err != nil {
		log.Printf("Error listing POIs: %v", err)
		return nil, utils.ErrDatabaseError
	}

	return pois, nil
}

func (p *PoiService) DeletePoi(id uuid.UUID, ctx context.Context) error {

	existingPOI, err := p.poiRepository.GetByIDWithDetails(ctx, id.String())
	if err != nil {
		log.Printf("Error fetching POI: %v", err)
		return utils.ErrDatabaseError
	}

	if existingPOI == nil {
		return utils.ErrPOINotFound
	}

	if err := p.poiRepository.Delete(ctx, id); err != nil {
		log.Printf("Error deleting POI: %v", err)
		return utils.ErrDatabaseError
	}

	return nil
}

func (p *PoiService) UpdatePoi(pois request_models.UpdatePoiRequest, ctx context.Context) error {
	existingPOI, err := p.poiRepository.GetByIDWithDetails(ctx, pois.ID.String())
	if err != nil {
		log.Printf("Error fetching POI: %v", err)
		return utils.ErrDatabaseError
	}

	if existingPOI == nil {
		return utils.ErrPOINotFound
	}

	existingPOI.Name = pois.Name
	existingPOI.Latitude = pois.Latitude
	existingPOI.Longitude = pois.Longitude
	existingPOI.CategoryID = pois.Category
	existingPOI.ProvinceID = pois.Province
	existingPOI.OpeningHours = pois.OpeningHours
	existingPOI.ContactInfo = pois.ContactInfo
	existingPOI.Address = pois.Address

	if pois.PoiDetails != nil {
		existingPOI.Description = pois.PoiDetails.Description
		existingPOI.Details.Images = pois.PoiDetails.Image
	}

	if err := p.poiRepository.UpdatePoi(ctx, existingPOI); err != nil {
		log.Printf("Error updating POI: %v", err)
		return utils.ErrDatabaseError
	}

	return nil
}

func (p *PoiService) CreatePois(pois request_models.CreatePoiRequest, ctx context.Context) error {

	newPOI := &db_models.POI{
		Name:         pois.Name,
		Latitude:     pois.Latitude,
		Longitude:    pois.Longitude,
		ProvinceID:   pois.Province,
		CategoryID:   pois.Category,
		OpeningHours: pois.OpeningHours,
		ContactInfo:  pois.ContactInfo,
		Address:      pois.Address,
	}

	if pois.PoiDetails != nil {
		newPOI.Description = pois.PoiDetails.Description
		newPOI.Details = db_models.POIDetail{
			Images: pois.PoiDetails.Image,
		}
	}

	if _, err := p.poiRepository.CreatePoi(ctx, newPOI); err != nil {
		log.Printf("Error creating POI: %v", err)

		return utils.ErrDatabaseError
	}

	return nil
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
		return []response_models.POI{}, nil
	}

	poiResponses := make([]response_models.POI, 0, len(pois))

	//SetupPoisIndex()
	//BulkIndexPOIs(ctx, "poi_v1", pois)

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
