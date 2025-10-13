package services

import (
	"context"
	"github.com/google/uuid"
	"time"
	"vivu/internal/models/db_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type JourneyServiceInterface interface {
	GetListOfJourneyByUserId(ctx context.Context, page int, pagesize int, userId string) ([]response_models.JourneyResponse, error)
	GetDetailsInfoOfJourneyById(ctx context.Context, journeyId string) (*response_models.JourneyDetailResponse, error)
	AddPoiToJourneyWithGivenStartAndEndDate(ctx context.Context, journeyId string, poiId string, startDate time.Time, endDate time.Time) error
	RemovePoiFromJourney(ctx context.Context, journeyId string, poiId string) error
	AddDayToJourney(ctx context.Context, journeyId string) (uuid.UUID, error)
	UpdateSelectedPoiInActivity(ctx context.Context, activityId uuid.UUID, currentPoiId string, startTimen, endTime time.Time) error
}

type JourneyService struct {
	journeyRepo repositories.JourneyRepository
}

func (j *JourneyService) UpdateSelectedPoiInActivity(ctx context.Context,
	activityId uuid.UUID,
	currentPoiId string,
	startTimen, endTime time.Time) error {
	if currentPoiId == "" {
		return utils.ErrInvalidInput
	}

	// Call the repository method
	err := j.journeyRepo.UpdateSelectedPoiInActivityWithGivenTime(ctx, activityId, currentPoiId, startTimen, endTime)
	if err != nil {
		return utils.ErrDatabaseError
	}

	return nil
}

func (j *JourneyService) AddDayToJourney(ctx context.Context, journeyId string) (uuid.UUID, error) {

	journey, err := j.journeyRepo.GetDetailsOfJourneyById(ctx, journeyId)
	if err != nil {
		return uuid.Nil, utils.ErrDatabaseError
	}
	if journey == nil {
		return uuid.Nil, utils.ErrJourneyNotFound
	}

	newId, err := j.journeyRepo.AddDayToJourneyWithDate(ctx, journeyId)
	if err != nil {
		return uuid.Nil, utils.ErrDatabaseError
	}

	return newId, nil
}

func (j *JourneyService) RemovePoiFromJourney(ctx context.Context, journeyId string, poiId string) error {

	err := j.journeyRepo.RemovePoiFromJourneyWithId(ctx, journeyId, poiId)
	if err != nil {
		return utils.ErrDatabaseError
	}

	return nil
}

func (j *JourneyService) AddPoiToJourneyWithGivenStartAndEndDate(ctx context.Context, journeyId string, poiId string, startDate time.Time, endDate time.Time) error {

	err := j.journeyRepo.AddPoiToJourneyWithStartEnd(ctx, journeyId, poiId, startDate, &endDate)
	if err != nil {
		return utils.ErrDatabaseError
	}

	return nil
}

func NewJourneyService(journeyRepo repositories.JourneyRepository) JourneyServiceInterface {
	return &JourneyService{
		journeyRepo: journeyRepo,
	}
}

func (j *JourneyService) GetListOfJourneyByUserId(
	ctx context.Context, page, pagesize int, userId string,
) ([]response_models.JourneyResponse, error) {

	journeys, err := j.journeyRepo.GetListOfJourneyByUserId(ctx, page, pagesize, userId)
	if err != nil {
		return nil, err
	}

	out := make([]response_models.JourneyResponse, 0, len(journeys))
	for _, journey := range journeys {
		startVN := utils.FromUnixSecondsVN(journey.StartDate) // expects seconds
		endVN := utils.FromUnixSecondsVN(*journey.EndDate)

		out = append(out, response_models.JourneyResponse{
			ID:    journey.ID.String(),
			Title: journey.Title,
			// Prefer stable ISO strings for APIs
			StartDate: utils.FormatRFC3339VN(startVN), // "" if zero
			EndDate:   utils.FormatRFC3339VN(endVN),   // "" if zero
			Location:  journey.Location,
		})
	}
	return out, nil
}

func (j *JourneyService) GetDetailsInfoOfJourneyById(ctx context.Context, journeyId string) (*response_models.JourneyDetailResponse, error) {
	journey, err := j.journeyRepo.GetDetailsOfJourneyById(ctx, journeyId)
	if err != nil {
		return nil, err
	}
	if journey == nil {
		return nil, utils.ErrJourneyNotFound
	}

	out := db_models.BuildJourneyDetailResponse(journey)

	return out, nil
}
