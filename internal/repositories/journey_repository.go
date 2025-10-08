// internal/repositories/journey_repo.go
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	dbm "vivu/internal/models/db_models"
	resp "vivu/internal/models/response_models"
)

type JourneyRepository interface {
	ReplaceMaterializedPlan(ctx context.Context,
		journeyID *uuid.UUID,
		plan *resp.PlanOnly,
		createIn *CreateJourneyInput) (uuid.UUID, error)

	GetListOfJourneyByUserId(ctx context.Context, page int, pagesize int, userId string) ([]dbm.Journey, error)
	GetDetailsOfJourneyById(ctx context.Context, journeyId string) (*dbm.Journey, error)
	RemovePoiFromJourneyWithId(ctx context.Context, journeyId string, poiId string) error
	AddPoiToJourneyWithIdOnGivenDay(ctx context.Context, journeyId string, poiId string, day time.Time) error
	AddPoiToJourneyWithStartEnd(ctx context.Context, journeyId string, poiId string, start time.Time, end *time.Time) error
}

type journeyRepository struct {
	db *gorm.DB
}

func (r *journeyRepository) AddPoiToJourneyWithStartEnd(
	ctx context.Context,
	journeyId string,
	poiId string,
	start time.Time,
	end *time.Time,
) error {
	poiUUID, err := uuid.Parse(poiId)
	if err != nil {
		return err
	}

	// Normalize start to VN and derive the owning JourneyDay by range
	startVN := start.In(vnLoc)
	dayStart := time.Date(startVN.Year(), startVN.Month(), startVN.Day(), 0, 0, 0, 0, vnLoc)
	dayEnd := dayStart.Add(24 * time.Hour)

	var journeyDay dbm.JourneyDay
	if err := r.db.WithContext(ctx).
		Where("journey_id = ? AND date >= ? AND date < ?", journeyId, dayStart, dayEnd).
		First(&journeyDay).Error; err != nil {
		return err
	}

	var endVN *time.Time
	if end != nil {
		evn := end.In(vnLoc)
		// If end is before start, assume cross-midnight
		if evn.Before(startVN) {
			evn = evn.Add(24 * time.Hour)
		}
		endVN = &evn
	}

	act := dbm.JourneyActivity{
		JourneyDayID:  journeyDay.ID,
		Time:          startVN,
		EndTime:       endVN,
		ActivityType:  "poi",
		SelectedPOIID: poiUUID,
		Notes:         "",
	}
	return r.db.WithContext(ctx).Create(&act).Error
}

func (r *journeyRepository) AddPoiToJourneyWithIdOnGivenDay(ctx context.Context, journeyId string, poiId string, day time.Time) error {

	poiUUID, err := uuid.Parse(poiId)
	if err != nil {
		return err
	}

	normalizedDay := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())

	var journeyDay dbm.JourneyDay
	err = r.db.WithContext(ctx).
		Where("journey_id = ? AND date = ?", journeyId, normalizedDay).
		First(&journeyDay).Error

	if err != nil {
		return err
	}

	newActivity := dbm.JourneyActivity{
		JourneyDayID:  journeyDay.ID,
		Time:          day, // You might want to set a specific time here
		ActivityType:  "poi",
		SelectedPOIID: poiUUID,
		Notes:         "",
	}

	return r.db.WithContext(ctx).Create(&newActivity).Error
}

func (r *journeyRepository) RemovePoiFromJourneyWithId(
	ctx context.Context, journeyId string, poiId string,
) error {
	poiUUID, err := uuid.Parse(poiId)
	if err != nil {
		return err
	}

	// (Optional) if your journey_days.journey_id is uuid, parse it too:
	// jUUID, err := uuid.Parse(journeyId); if err != nil { return err }

	// Subquery to collect activity IDs that match the join
	sub := r.db.WithContext(ctx).
		Model(&dbm.JourneyActivity{}).
		Select("journey_activities.id").
		Joins("JOIN journey_days ON journey_activities.journey_day_id = journey_days.id").
		Where("journey_days.journey_id = ? AND journey_activities.selected_poi_id = ?", journeyId, poiUUID)

	// Now delete by those IDs (GORM will generate a plain UPDATE for soft-delete with no JOIN)
	return r.db.WithContext(ctx).
		Where("id IN (?)", sub).
		Delete(&dbm.JourneyActivity{}).Error
}

func (r *journeyRepository) GetDetailsOfJourneyById(ctx context.Context, journeyId string) (*dbm.Journey, error) {

	var journey dbm.Journey
	err := r.db.WithContext(ctx).
		Where("id = ?", journeyId).
		Preload("Days").
		Preload("Days.Activities").
		Preload("Days.Activities.SelectedPOI").
		First(&journey).Error

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &journey, nil
}

var vnLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		// Fallback but you really want the named tz for DST/offset correctness (VN is fixed +07)
		return time.FixedZone("ICT", 7*60*60)
	}
	return loc
}()

func (r *journeyRepository) GetListOfJourneyByUserId(ctx context.Context, page int, pagesize int, userId string) ([]dbm.Journey, error) {

	var journeys []dbm.Journey
	err := r.db.WithContext(ctx).
		Where("account_id = ?", userId).
		Offset((page - 1) * pagesize).
		Limit(pagesize).
		Find(&journeys).Error

	if err != nil {
		return nil, err
	}

	return journeys, nil
}

func NewJourneyRepository(db *gorm.DB) JourneyRepository {
	return &journeyRepository{db: db}
}

func (r *journeyRepository) ReplaceMaterializedPlan(
	ctx context.Context,
	journeyID *uuid.UUID,
	plan *resp.PlanOnly,
	createIn *CreateJourneyInput,
) (uuid.UUID, error) {

	var outID uuid.UUID

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var j dbm.Journey
		needCreate := false

		switch {
		case journeyID == nil || *journeyID == uuid.Nil:
			needCreate = true
		default:
			if err := tx.First(&j, "id = ?", *journeyID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					needCreate = true
				} else {
					return err
				}
			}
		}

		if needCreate {
			if createIn == nil {
				return errors.New("createIn is required to create a new journey")
			}
			// Ensure createIn times are in Vietnam timezone, then store Unix seconds
			startVN := createIn.StartDate.In(vnLoc)
			var endUnix int64
			if createIn.EndDate != nil {
				endVN := createIn.EndDate.In(vnLoc)
				endUnix = endVN.Unix()
			} else if len(plan.Days) > 0 {
				// Calculate end date based on the number of days in the plan
				endVN := startVN.Add(time.Duration(len(plan.Days)-1) * 24 * time.Hour)
				endUnix = endVN.Unix()
			}

			j = dbm.Journey{
				AccountID:   createIn.AccountID,
				Title:       createIn.Title,
				StartDate:   startVN.Unix(), // store seconds
				EndDate:     &endUnix,       // store seconds or 0
				IsShared:    createIn.IsShared,
				IsCompleted: createIn.IsCompleted,
				Location:    plan.Destination,
			}
			if err := tx.Create(&j).Error; err != nil {
				return err
			}
		}

		outID = j.ID

		// Base day: VN midnight of StartDate
		startVN := time.Unix(j.StartDate, 0).In(vnLoc)
		baseDate := time.Date(
			startVN.Year(), startVN.Month(), startVN.Day(),
			0, 0, 0, 0, vnLoc,
		)

		// 1) Wipe previous materialized data
		subDayIDs := tx.Model(&dbm.JourneyDay{}).
			Select("id").
			Where("journey_id = ?", j.ID)

		if err := tx.Where("journey_day_id IN (?)", subDayIDs).
			Delete(&dbm.JourneyActivity{}).Error; err != nil {
			return err
		}
		if err := tx.Where("journey_id = ?", j.ID).
			Delete(&dbm.JourneyDay{}).Error; err != nil {
			return err
		}

		// 2) Create days + activities
		for _, d := range plan.Days {
			dayDate := baseDate.Add(time.Duration(d.Day-1) * 24 * time.Hour) // in vnLoc

			jd := dbm.JourneyDay{
				JourneyID: j.ID,
				Date:      dayDate, // GORM should store with tz; if you store as timestamp w/o tz, keep consistency
				DayNumber: d.Day,
			}
			if err := tx.Create(&jd).Error; err != nil {
				return err
			}

			// inside the for _, d := range plan.Days { ... } loop
			acts := make([]dbm.JourneyActivity, 0, len(d.Activities))
			for _, a := range d.Activities {
				if a.MainPOIID == "" {
					continue
				}
				poiID, err := uuid.Parse(a.MainPOIID)
				if err != nil {
					continue
				}

				// VN-local base day
				actStart := dayDate
				if t, err := time.ParseInLocation("15:04", a.StartTime, vnLoc); err == nil {
					actStart = time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(),
						t.Hour(), t.Minute(), 0, 0, vnLoc)
				}

				// Parse end time if provided
				var actEndPtr *time.Time
				if a.EndTime != "" {
					if et, err := time.ParseInLocation("15:04", a.EndTime, vnLoc); err == nil {
						etFull := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(),
							et.Hour(), et.Minute(), 0, 0, vnLoc)
						// ensure end >= start (adjust to next day if user meant crossing midnight)
						if etFull.Before(actStart) {
							etFull = etFull.Add(24 * time.Hour)
						}
						actEndPtr = &etFull
					}
				}

				acts = append(acts, dbm.JourneyActivity{
					JourneyDayID:  jd.ID,
					Time:          actStart,  // start
					EndTime:       actEndPtr, // end (nullable)
					ActivityType:  "poi",
					SelectedPOIID: poiID,
					Notes:         "",
				})
			}
			if len(acts) > 0 {
				if err := tx.Create(&acts).Error; err != nil {
					return err
				}
			}

		}

		return nil
	})

	return outID, err
}

type CreateJourneyInput struct {
	AccountID   uuid.UUID
	Title       string
	StartDate   time.Time  // REQUIRED when creating new journey
	EndDate     *time.Time // optional
	IsShared    bool       // optional
	IsCompleted bool       // optional
}
