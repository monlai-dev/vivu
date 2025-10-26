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
	AddDayToJourneyWithDate(ctx context.Context, journeyId string) (uuid.UUID, error)
	UpdateSelectedPoiInActivityWithGivenTime(ctx context.Context, activityId uuid.UUID, currentPoiId string, startTime, endTime time.Time) error
	ScaleDaysForJourney(
		ctx context.Context,
		journeyId string,
		start time.Time,
		end time.Time,
	) (int, int, error)
	UpdateJourneyWindow(
		ctx context.Context, journeyId string, startUnix, endUnix int64,
	) error
}

func NewJourneyRepository(db *gorm.DB) JourneyRepository {
	return &journeyRepository{db: db}
}

type journeyRepository struct {
	db *gorm.DB
}

func (r *journeyRepository) UpdateSelectedPoiInActivityWithGivenTime(ctx context.Context, activityId uuid.UUID, currentPoiId string, startTimen, endTime time.Time) error {
	poiUUID, err := uuid.Parse(currentPoiId)
	if err != nil {
		return err
	}

	// Validate that the activity exists and is associated with the correct JourneyDay for the given date
	var activity dbm.JourneyActivity
	err = r.db.WithContext(ctx).
		Joins("JOIN journey_days ON journey_activities.journey_day_id = journey_days.id").
		Where("journey_activities.id = ? AND journey_days.date = ?", activityId, startTimen.Truncate(24*time.Hour)).
		First(&activity).Error
	if err != nil {
		return err
	}

	// Update the activity with the new POI and time
	err = r.db.WithContext(ctx).
		Model(&dbm.JourneyActivity{}).
		Where("id = ?", activityId).
		Updates(map[string]interface{}{
			"selected_poi_id": poiUUID,
			"time":            startTimen,
			"end_time":        endTime,
		}).Error

	return err
}

func (r *journeyRepository) AddDayToJourneyWithDate(ctx context.Context, journeyId string) (uuid.UUID, error) {

	var lastDate time.Time
	err := r.db.WithContext(ctx).
		Model(&dbm.JourneyDay{}).
		Where("journey_id = ?", journeyId).
		Select("COALESCE(MAX(date), ?)", time.Now().In(vnLoc)).
		Scan(&lastDate).Error
	if err != nil {
		return uuid.Nil, err
	}
	normalizedDate := lastDate.Add(24 * time.Hour)

	// Calculate the day number based on existing days
	var maxDayNumber int
	err = r.db.WithContext(ctx).
		Model(&dbm.JourneyDay{}).
		Where("journey_id = ?", journeyId).
		Select("COALESCE(MAX(day_number), 0)").
		Scan(&maxDayNumber).Error
	if err != nil {
		return uuid.Nil, err
	}

	newDay := dbm.JourneyDay{
		JourneyID: uuid.MustParse(journeyId),
		Date:      normalizedDate,
		DayNumber: maxDayNumber + 1,
	}

	if err := r.db.WithContext(ctx).Create(&newDay).Error; err != nil {
		return uuid.Nil, err
	}

	return newDay.ID, nil
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
	if l, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return l
	}
	return time.FixedZone("ICT", 7*3600)
}()

// midnightVN returns 00:00:00 in vnLoc for the calendar date of t (evaluated in vnLoc)
func midnightVN(t time.Time) time.Time {
	l := t.In(vnLoc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, vnLoc)
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// nearestDayVN picks the candidate midnight date closest to ts's date (in vnLoc)
func nearestDayVN(ts time.Time, candidates []time.Time) time.Time {
	t0 := midnightVN(ts)
	best := candidates[0]
	bestDiff := absDuration(t0.Sub(best))
	for _, c := range candidates[1:] {
		if diff := absDuration(t0.Sub(c)); diff < bestDiff {
			best, bestDiff = c, diff
		}
	}
	return best
}

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

func (r *journeyRepository) ScaleDaysForJourney(
	ctx context.Context,
	journeyId string,
	start time.Time,
	end time.Time,
) (int, int, error) {

	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
		}
	}()
	if err := tx.Error; err != nil {
		return 0, 0, err
	}

	// 1) Load existing live days
	var existing []dbm.JourneyDay
	if err := tx.
		Where("journey_id = ?", journeyId).
		Order("date ASC").
		Find(&existing).Error; err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}

	// 2) Define target day set (midnight vnLoc)
	startDate := midnightVN(start)
	endDate := midnightVN(end)

	days := int(endDate.Sub(startDate).Hours()/24) + 1 // inclusive
	target := make(map[time.Time]struct{}, days)
	targetOrder := make([]time.Time, 0, days)
	for i := 0; i < days; i++ {
		d := startDate.Add(time.Duration(i) * 24 * time.Hour)
		target[d] = struct{}{}
		targetOrder = append(targetOrder, d)
	}

	// Index existing by normalized date
	existingByDate := make(map[time.Time]dbm.JourneyDay, len(existing))
	for _, d := range existing {
		key := midnightVN(d.Date)
		existingByDate[key] = d
	}

	// 3) Create missing target days
	added := 0
	for _, d := range targetOrder {
		if _, ok := existingByDate[d]; ok {
			continue
		}
		newDay := dbm.JourneyDay{
			JourneyID: uuid.MustParse(journeyId),
			Date:      d, // midnight vnLoc
		}
		if err := tx.Create(&newDay).Error; err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
		added++
	}

	// 4) Identify removed days (do NOT delete yet)
	removedDays := make([]dbm.JourneyDay, 0)
	for _, d := range existing {
		key := midnightVN(d.Date)
		if _, keep := target[key]; !keep {
			removedDays = append(removedDays, d)
		}
	}

	// 4.1) Reassign activities from removed days to nearest remaining day
	removed := 0
	if len(removedDays) > 0 && len(targetOrder) > 0 {
		// Build actual remaining rows for the target window (we need IDs)
		var remainingRows []dbm.JourneyDay
		if err := tx.
			Where("journey_id = ?", journeyId).
			Where("date >= ? AND date <= ?", startDate, endDate).
			Order("date ASC").
			Find(&remainingRows).Error; err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
		if len(remainingRows) == 0 {
			// Should never happen; defensive
			_ = tx.Rollback()
			return 0, 0, errors.New("no remaining days after scaling")
		}

		// Map midnight date -> remaining row
		remByDate := make(map[time.Time]dbm.JourneyDay, len(remainingRows))
		remainingDates := make([]time.Time, 0, len(remainingRows))
		for _, rd := range remainingRows {
			mn := midnightVN(rd.Date)
			remByDate[mn] = rd
			remainingDates = append(remainingDates, mn)
		}

		// Load all activities of removed days
		removedIDs := make([]uuid.UUID, 0, len(removedDays))
		rmByID := make(map[uuid.UUID]dbm.JourneyDay, len(removedDays))
		for _, d := range removedDays {
			removedIDs = append(removedIDs, d.ID)
			rmByID[d.ID] = d
		}

		var acts []dbm.JourneyActivity
		if err := tx.Where("journey_day_id IN ?", removedIDs).Find(&acts).Error; err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}

		// Reassign each activity
		for i := range acts {
			srcDay := rmByID[acts[i].JourneyDayID]
			targetDate := nearestDayVN(srcDay.Date, remainingDates)
			destRow, ok := remByDate[targetDate]
			if !ok {
				// Fallback (defensive)
				destRow = remainingRows[0]
			}

			// Preserve clock time, swap the date to targetDate (in vnLoc)
			tt := acts[i].Time.In(vnLoc)
			newTime := time.Date(
				targetDate.Year(), targetDate.Month(), targetDate.Day(),
				tt.Hour(), tt.Minute(), tt.Second(), tt.Nanosecond(), vnLoc,
			)
			var newEnd *time.Time
			if acts[i].EndTime != nil {
				et := acts[i].EndTime.In(vnLoc)
				tmp := time.Date(
					targetDate.Year(), targetDate.Month(), targetDate.Day(),
					et.Hour(), et.Minute(), et.Second(), et.Nanosecond(), vnLoc,
				)
				newEnd = &tmp
			}

			if err := tx.Model(&acts[i]).Updates(map[string]any{
				"journey_day_id": destRow.ID,
				"time":           newTime,
				"end_time":       newEnd,
			}).Error; err != nil {
				_ = tx.Rollback()
				return 0, 0, err
			}
		}

		// Now safe to soft-delete removed days
		for _, d := range removedDays {
			if err := tx.Delete(&d).Error; err != nil {
				_ = tx.Rollback()
				return 0, 0, err
			}
			removed++
		}
	}

	// 5) Resequence day_number by date
	var after []dbm.JourneyDay
	if err := tx.
		Where("journey_id = ?", journeyId).
		Order("date ASC").
		Find(&after).Error; err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	for i := range after {
		want := i + 1
		if after[i].DayNumber != want {
			if err := tx.Model(&after[i]).Update("day_number", want).Error; err != nil {
				_ = tx.Rollback()
				return 0, 0, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, 0, err
	}
	return added, removed, nil
}

func (r *journeyRepository) UpdateJourneyWindow(
	ctx context.Context, journeyId string, startUnix, endUnix int64,
) error {
	return r.db.WithContext(ctx).Model(&dbm.Journey{}).
		Where("id = ?", journeyId).
		Updates(map[string]interface{}{
			"start_date": startUnix,
			"end_date":   endUnix,
		}).Error
}

type CreateJourneyInput struct {
	AccountID   uuid.UUID
	Title       string
	StartDate   time.Time  // REQUIRED when creating new journey
	EndDate     *time.Time // optional
	IsShared    bool       // optional
	IsCompleted bool       // optional
}
