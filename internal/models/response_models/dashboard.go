package response_models

import (
	"time"

	"github.com/google/uuid"
)

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	// "day" | "week" | "month"
	Interval string `json:"interval"`
	// Optional: timezone used for bucketing (defaults to UTC if empty)
	Timezone string `json:"timezone,omitempty"`
}

type KPIBlock struct {
	TotalAccounts         int64 `json:"total_accounts"`
	NewAccounts           int64 `json:"new_accounts"`
	TotalJourneys         int64 `json:"total_journeys"`
	TotalActivities       int64 `json:"total_activities"`
	ActiveSubscriptions   int64 `json:"active_subscriptions"`
	TrialingSubscriptions int64 `json:"trialing_subscriptions"`
	CanceledSubscriptions int64 `json:"canceled_subscriptions"`
	ExpiredSubscriptions  int64 `json:"expired_subscriptions"`

	// Financial KPIs
	MRRMinor  int64   `json:"mrr_minor"`  // monthly recurring revenue (minor units)
	ARRMinor  int64   `json:"arr_minor"`  // ARR = 12 * MRR
	ARPUMinor float64 `json:"arpu_minor"` // avg revenue per active subscriber (minor units)
	ChurnPct  float64 `json:"churn_pct"`  // (canceled during period / subscribers at period start) * 100
}

type SeriesPoint struct {
	Bucket time.Time `json:"bucket"`
	Value  int64     `json:"value"`
}

type RevenueSeries struct {
	Currency   string        `json:"currency"`
	Points     []SeriesPoint `json:"points"`
	TotalMinor int64         `json:"total_minor"`
}

type CountSeries struct {
	Points []SeriesPoint `json:"points"`
}

type PlanMixItem struct {
	PlanID     uuid.UUID `json:"plan_id"`
	PlanCode   string    `json:"plan_code"`
	PlanName   string    `json:"plan_name"`
	Count      int64     `json:"count"`
	Percent    float64   `json:"percent"`
	Period     string    `json:"period"` // "month" | "year"
	PriceMinor int64     `json:"price_minor"`
}

type PlanMix struct {
	Items []PlanMixItem `json:"items"`
}

type TopDestination struct {
	Location string `json:"location"`
	Count    int64  `json:"count"`
}

type RecentPayment struct {
	ID            uuid.UUID  `json:"id"`
	PaidAt        *time.Time `json:"paid_at"`
	AmountMinor   int64      `json:"amount_minor"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	Provider      string     `json:"provider"`
	ProviderTxnID string     `json:"provider_txn_id"`
	AccountEmail  string     `json:"account_email"`
}

type DashboardReport struct {
	Range           TimeRange        `json:"range"`
	KPIs            KPIBlock         `json:"kpis"`
	Revenue         RevenueSeries    `json:"revenue"`
	NewUsers        CountSeries      `json:"new_users"`
	NewSubs         CountSeries      `json:"new_subscriptions"`
	PlanMix         PlanMix          `json:"plan_mix"`
	TopDestinations []TopDestination `json:"top_destinations"`
	RecentPayments  []RecentPayment  `json:"recent_payments"`
}
