package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	dbm "vivu/internal/models/db_models"
	resp "vivu/internal/models/response_models"
	"vivu/internal/repositories"
)

type DashboardService interface {
	BuildDashboard(ctx context.Context, rng resp.TimeRange, currency string) (*resp.DashboardReport, error)
}

type dashboardService struct {
	repo repositories.DashboardRepository
}

func NewDashboardService(repo repositories.DashboardRepository) DashboardService {
	return &dashboardService{repo: repo}
}

// normalizeRange ensures sane defaults and ordering
func normalizeRange(r resp.TimeRange) resp.TimeRange {
	out := r
	if out.Interval == "" {
		out.Interval = "day"
	}
	if out.End.IsZero() {
		out.End = time.Now().UTC()
	}
	if out.Start.IsZero() {
		out.Start = out.End.AddDate(0, 0, -30) // last 30 days default
	}
	if out.Start.After(out.End) {
		out.Start, out.End = out.End, out.Start
	}
	return out
}

func monthlyEquivalent(priceMinor int64, period string) int64 {
	switch period {
	case string(dbm.PeriodMonth):
		return priceMinor
	case string(dbm.PeriodYear):
		// Integer floor division; adjust if you want rounding
		return priceMinor / 12
	default:
		return 0
	}
}

func (s *dashboardService) BuildDashboard(ctx context.Context, rng resp.TimeRange, currency string) (*resp.DashboardReport, error) {
	rng = normalizeRange(rng)

	// ---------- Core counts ----------
	totalAccounts, err := s.repo.CountTotalAccounts(ctx)
	if err != nil {
		return nil, err
	}

	newAccounts, err := s.repo.CountNewAccounts(ctx, rng.Start, rng.End)
	if err != nil {
		return nil, err
	}

	totalJourneys, err := s.repo.CountTotalJourneys(ctx)
	if err != nil {
		return nil, err
	}

	totalActivities, err := s.repo.CountTotalActivities(ctx)
	if err != nil {
		return nil, err
	}

	activeSubs, err := s.repo.CountSubscriptionsByStatus(ctx, dbm.SubStatusActive)
	if err != nil {
		return nil, err
	}
	trialSubs, err := s.repo.CountSubscriptionsByStatus(ctx, dbm.SubStatusTrialing)
	if err != nil {
		return nil, err
	}
	canceledSubs, err := s.repo.CountSubscriptionsByStatus(ctx, dbm.SubStatusCanceled)
	if err != nil {
		return nil, err
	}
	expiredSubs, err := s.repo.CountSubscriptionsByStatus(ctx, dbm.SubStatusExpired)
	if err != nil {
		return nil, err
	}

	// ---------- Series ----------
	revenueRows, err := s.repo.RevenueSeries(ctx, rng.Start, rng.End, rng.Interval, rng.Timezone)
	if err != nil {
		return nil, err
	}
	var revenuePoints []resp.SeriesPoint
	var totalRevenue int64
	for _, r := range revenueRows {
		revenuePoints = append(revenuePoints, resp.SeriesPoint{Bucket: r.Bucket, Value: r.Sum})
		totalRevenue += r.Sum
	}

	newUsersRows, err := s.repo.NewUsersSeries(ctx, rng.Start, rng.End, rng.Interval, rng.Timezone)
	if err != nil {
		return nil, err
	}
	var newUsersPoints []resp.SeriesPoint
	for _, r := range newUsersRows {
		newUsersPoints = append(newUsersPoints, resp.SeriesPoint{Bucket: r.Bucket, Value: r.Sum})
	}

	newSubsRows, err := s.repo.NewSubsSeries(ctx, rng.Start, rng.End, rng.Interval, rng.Timezone)
	if err != nil {
		return nil, err
	}
	var newSubsPoints []resp.SeriesPoint
	for _, r := range newSubsRows {
		newSubsPoints = append(newSubsPoints, resp.SeriesPoint{Bucket: r.Bucket, Value: r.Sum})
	}

	// ---------- Financials: MRR/ARR/ARPU ----------
	activeWithPlan, err := s.repo.ActiveSubscriptionsWithPlan(ctx)
	if err != nil {
		return nil, err
	}

	var mrr int64
	var activeCount int64
	for _, srow := range activeWithPlan {
		mrr += monthlyEquivalent(srow.PriceMinor, srow.Period)
		activeCount++
	}
	var arpu float64
	if activeCount > 0 {
		arpu = float64(mrr) / float64(activeCount)
	}

	// ---------- Churn ----------
	canceledInPeriod, err := s.repo.CountCanceledInPeriod(ctx, rng.Start, rng.End)
	if err != nil {
		return nil, err
	}
	subscribersAtStart, err := s.repo.CountSubscribersAt(ctx, rng.Start)
	if err != nil {
		return nil, err
	}
	var churnPct float64
	if subscribersAtStart > 0 {
		churnPct = (float64(canceledInPeriod) / float64(subscribersAtStart)) * 100.0
	}

	// ---------- Plan mix ----------
	planRows, err := s.repo.PlanMix(ctx)
	if err != nil {
		return nil, err
	}
	var planMixItems []resp.PlanMixItem
	var totalActive float64
	for _, r := range planRows {
		totalActive += float64(r.Count)
	}
	for _, r := range planRows {
		var pct float64
		if totalActive > 0 {
			pct = float64(r.Count) * 100.0 / totalActive
		}
		planMixItems = append(planMixItems, resp.PlanMixItem{
			PlanID:     uuid.MustParse(r.PlanID),
			PlanCode:   r.PlanCode,
			PlanName:   r.PlanName,
			Count:      r.Count,
			Percent:    pct,
			Period:     r.Period,
			PriceMinor: r.PriceMinor,
		})
	}

	// ---------- Top locations ----------
	locRows, err := s.repo.TopDestinations(ctx, rng.Start, rng.End, 10)
	if err != nil {
		return nil, err
	}
	var topDestinations []resp.TopDestination
	for _, r := range locRows {
		topDestinations = append(topDestinations, resp.TopDestination{
			Location: r.Location,
			Count:    r.Count,
		})
	}

	// ---------- Recent payments ----------
	payRows, err := s.repo.RecentPaidTransactions(ctx, 10)
	if err != nil {
		return nil, err
	}
	var recent []resp.RecentPayment
	for _, r := range payRows {
		var id uuid.UUID
		var err error
		if r.ID != "" {
			id, err = uuid.Parse(r.ID)
			if err != nil {
				return nil, errors.New("invalid transaction UUID in recent payments")
			}
		}
		recent = append(recent, resp.RecentPayment{
			ID:            id,
			PaidAt:        r.PaidAt,
			AmountMinor:   r.AmountMinor,
			Currency:      r.Currency,
			Status:        r.Status,
			Provider:      r.Provider,
			ProviderTxnID: r.ProviderTxnID,
			AccountEmail:  r.AccountEmail,
		})
	}

	report := &resp.DashboardReport{
		Range: resp.TimeRange{
			Start:    rng.Start,
			End:      rng.End,
			Interval: rng.Interval,
			Timezone: rng.Timezone,
		},
		KPIs: resp.KPIBlock{
			TotalAccounts:         totalAccounts,
			NewAccounts:           newAccounts,
			TotalJourneys:         totalJourneys,
			TotalActivities:       totalActivities,
			ActiveSubscriptions:   activeSubs,
			TrialingSubscriptions: trialSubs,
			CanceledSubscriptions: canceledSubs,
			ExpiredSubscriptions:  expiredSubs,

			MRRMinor:  mrr,
			ARRMinor:  mrr * 12,
			ARPUMinor: arpu,
			ChurnPct:  churnPct,
		},
		Revenue: resp.RevenueSeries{
			Currency:   currency,
			Points:     revenuePoints,
			TotalMinor: totalRevenue,
		},
		NewUsers: resp.CountSeries{
			Points: newUsersPoints,
		},
		NewSubs: resp.CountSeries{
			Points: newSubsPoints,
		},
		PlanMix: resp.PlanMix{
			Items: planMixItems,
		},
		TopDestinations: topDestinations,
		RecentPayments:  recent,
	}

	return report, nil
}
