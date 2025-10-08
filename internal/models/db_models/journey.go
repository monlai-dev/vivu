package db_models

import (
	"github.com/google/uuid"
	"time"
	resp "vivu/internal/models/response_models"
)

type Journey struct {
	BaseModel
	AccountID   uuid.UUID // Change from UserID
	Title       string
	StartDate   int64
	EndDate     *int64
	IsShared    bool
	IsCompleted bool
	Location    string

	Account  Account      `gorm:"foreignKey:AccountID"`
	Days     []JourneyDay `gorm:"foreignKey:JourneyID"`
	CheckIns []CheckIn    `gorm:"foreignKey:JourneyID"`
}

func toRFC3339(sec int64) string {
	if sec == 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

// formatTime converts time.Time to RFC3339 string.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func BuildJourneyDetailResponse(j *Journey) *resp.JourneyDetailResponse {
	if j == nil {
		return nil
	}

	out := &resp.JourneyDetailResponse{
		ID:          j.ID,
		Title:       j.Title,
		StartDate:   toRFC3339(j.StartDate),
		EndDate:     toRFC3339(*j.EndDate),
		IsShared:    j.IsShared,
		IsCompleted: j.IsCompleted,
	}

	// Duration (inclusive days). If you prefer exclusive, remove the +1 when >0.
	if j.StartDate > 0 && *j.EndDate >= j.StartDate {
		start := time.Unix(j.StartDate, 0).UTC().Truncate(24 * time.Hour)
		end := time.Unix(*j.EndDate, 0).UTC().Truncate(24 * time.Hour)
		out.DurationDays = int(end.Sub(start).Hours()/24) + 1
	}

	out.TotalDays = len(j.Days)

	// Days & activities
	out.Days = make([]resp.JourneyDayResponse, 0, len(j.Days))
	totalActivities := 0

	for _, d := range j.Days {
		dayResp := resp.JourneyDayResponse{
			ID:         d.ID,
			DayNumber:  d.DayNumber,
			Date:       formatTime(d.Date),
			Activities: make([]resp.JourneyActivityDetail, 0, len(d.Activities)),
		}

		for _, a := range d.Activities {
			ad := resp.JourneyActivityDetail{
				ID:           a.ID,
				Time:         formatTime(a.Time),
				EndTime:      formatTimeIfNotNil(a.EndTime),
				ActivityType: a.ActivityType,
				Notes:        a.Notes,
			}

			// POI summary (only if preloaded)
			if a.SelectedPOI.ID != uuid.Nil {
				ad.SelectedPOI = &resp.POISummary{
					ID:        a.SelectedPOI.ID,
					Name:      a.SelectedPOI.Name,
					Address:   a.SelectedPOI.Address,
					Latitude:  a.SelectedPOI.Latitude,
					Longitude: a.SelectedPOI.Longitude,
					Status:    a.SelectedPOI.Status,
				}
			}

			dayResp.Activities = append(dayResp.Activities, ad)
		}

		totalActivities += len(d.Activities)
		out.Days = append(out.Days, dayResp)
	}

	out.TotalActivities = totalActivities
	return out
}

func formatTimeIfNotNil(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}
