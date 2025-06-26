package repositories

import (
	"errors"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type POIDetailsRepository interface {
	GetPOIDetails(poiID string) (*db_models.POIDetail, error)
	UpdatePOIDetails(poiID string, details *db_models.POIDetail) error
	CreatePOIDetails(details *db_models.POIDetail) error
}

type poiDetailsRepository struct {
	db *gorm.DB
}

func (p poiDetailsRepository) GetPOIDetails(poiID string) (*db_models.POIDetail, error) {
	var details db_models.POIDetail
	err := p.db.Where("poi_id = ?", poiID).First(&details).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No details found for the given POI ID
		}
		return nil, err // Return any other error encountered
	}
	return &details, nil // Return the found details
}

func (p poiDetailsRepository) UpdatePOIDetails(poiID string, details *db_models.POIDetail) error {

	return p.db.Transaction(func(tx *gorm.DB) error {
		// First, check if the details exist
		var existingDetails db_models.POIDetail
		if err := tx.Where("poi_id = ?", poiID).First(&existingDetails).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // No details to update
			}
			return err // Return any other error encountered
		}

		// Update the existing details
		existingDetails.Description = details.Description
		existingDetails.Images = details.Images
		existingDetails.Reviews = details.Reviews

		if err := tx.Save(&existingDetails).Error; err != nil {
			return err // Return any error encountered during save
		}

		return nil // Successfully updated details
	})
}

func (p poiDetailsRepository) CreatePOIDetails(details *db_models.POIDetail) error {

	if details == nil {
		return errors.New("details cannot be nil")
	}

	//if details.POIID == "" {
	//	return errors.New("POI ID cannot be empty")
	//}

	if err := p.db.Create(details).Error; err != nil {
		return err // Return any error encountered during creation
	}
	return nil // Successfully created details
}

func NewPOIDetailsRepository(db *gorm.DB) POIDetailsRepository {
	return &poiDetailsRepository{db: db}
}
