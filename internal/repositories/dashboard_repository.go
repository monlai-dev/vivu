package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	dbm "vivu/internal/models/db_models"
)

type DashboardRepository interface {
	// KPIs / counts
	CountTotalAccounts(ctx context.Context) (int64, error)
	CountNewAccounts(ctx context.Context, start, end time.Time) (int64, error)
	CountTotalJourneys(ctx context.Context) (int64, error)
	CountTotalActivities(ctx context.Context) (int64, error)

	CountSubscriptionsByStatus(ctx context.Context, status dbm.SubscriptionStatus) (int64, error)
	CountCanceledInPeriod(ctx context.Context, start, end time.Time) (int64, error)
	CountSubscribersAt(ctx context.Context, t time.Time) (int64, error)

	// Time series
	RevenueSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error)
	NewUsersSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error)
	NewSubsSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error)

	// MRR compute helpers
	ActiveSubscriptionsWithPlan(ctx context.Context) ([]SubWithPlan, error)

	// Plan mix (active subs)
	PlanMix(ctx context.Context) ([]PlanMixRow, error)

	// Top destinations
	TopDestinations(ctx context.Context, start, end time.Time, limit int) ([]LocationRow, error)

	// Recent payments
	RecentPaidTransactions(ctx context.Context, limit int) ([]RecentPaymentRow, error)
}

type dashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) DashboardRepository {
	return &dashboardRepository{db: db}
}

// ---------- Row helpers ----------
type BucketSum struct {
	Bucket time.Time `gorm:"column:bucket"`
	Sum    int64     `gorm:"column:sum"`
}

type SubWithPlan struct {
	SubID      string `gorm:"column:sub_id"`
	PlanID     string `gorm:"column:plan_id"`
	Period     string `gorm:"column:period"`
	PriceMinor int64  `gorm:"column:price_minor"`
	Status     string `gorm:"column:status"`
}

type PlanMixRow struct {
	PlanID     string `gorm:"column:plan_id"`
	PlanCode   string `gorm:"column:plan_code"`
	PlanName   string `gorm:"column:plan_name"`
	Period     string `gorm:"column:period"`
	PriceMinor int64  `gorm:"column:price_minor"`
	Count      int64  `gorm:"column:count"`
}

type LocationRow struct {
	Location string `gorm:"column:location"`
	Count    int64  `gorm:"column:count"`
}

type RecentPaymentRow struct {
	ID            string     `gorm:"column:id"`
	PaidAt        *time.Time `gorm:"column:paid_at"`
	AmountMinor   int64      `gorm:"column:amount_minor"`
	Currency      string     `gorm:"column:currency"`
	Status        string     `gorm:"column:status"`
	Provider      string     `gorm:"column:provider"`
	ProviderTxnID string     `gorm:"column:provider_txn_id"`
	AccountEmail  string     `gorm:"column:email"`
}

// ---------- Helpers ----------
func dateTrunc(interval, tz string, unixColumn string) string {
	// unixColumn is a column holding UNIX seconds (e.g., paid_at, created_at)
	// We convert UNIX seconds to timestamptz and then date_trunc with a timezone.
	// Example: date_trunc('day', timezone('Asia/Ho_Chi_Minh', to_timestamp(paid_at)))
	if tz == "" {
		return "date_trunc(?, to_timestamp(" + unixColumn + "))"
	}
	return "date_trunc(?, timezone(?, to_timestamp(" + unixColumn + ")))"
}

// ---------- Counts ----------
func (r *dashboardRepository) CountTotalAccounts(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&dbm.Account{}).Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountNewAccounts(ctx context.Context, start, end time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&dbm.Account{}).
		Where("created_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountTotalJourneys(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&dbm.Journey{}).Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountTotalActivities(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&dbm.JourneyActivity{}).Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountSubscriptionsByStatus(ctx context.Context, status dbm.SubscriptionStatus) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&dbm.Subscription{}).
		Where("status = ?", status).
		Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountCanceledInPeriod(ctx context.Context, start, end time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&dbm.Subscription{}).
		Where("status = ?", dbm.SubStatusCanceled).
		Where("canceled_at IS NOT NULL AND canceled_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Count(&n).Error
	return n, err
}

func (r *dashboardRepository) CountSubscribersAt(ctx context.Context, t time.Time) (int64, error) {
	// Subscribers active at time t: StartsAt <= t && EndsAt >= t (or status in active-like)
	var n int64
	err := r.db.WithContext(ctx).
		Model(&dbm.Subscription{}).
		Where("starts_at <= ? AND ends_at >= ?", t.Unix(), t.Unix()).
		Count(&n).Error
	return n, err
}

// ---------- Series ----------
func (r *dashboardRepository) RevenueSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error) {
	var rows []BucketSum
	truncExpr := dateTrunc(interval, tz, "paid_at")
	tx := r.db.WithContext(ctx).
		Table("transactions").
		Select(truncExpr+" AS bucket, SUM(amount_minor) AS sum", interval, tz).
		Where("status = ?", dbm.TxnStatusPaid).
		Where("paid_at IS NOT NULL").
		Where("paid_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Group("bucket").
		Order("bucket ASC")
	err := tx.Find(&rows).Error
	return rows, err
}

func (r *dashboardRepository) NewUsersSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error) {
	var rows []BucketSum
	truncExpr := dateTrunc(interval, tz, "created_at")
	tx := r.db.WithContext(ctx).
		Table("accounts").
		Select(truncExpr+" AS bucket, COUNT(*) AS sum", interval, tz).
		Where("created_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Group("bucket").
		Order("bucket ASC")
	err := tx.Find(&rows).Error
	return rows, err
}

func (r *dashboardRepository) NewSubsSeries(ctx context.Context, start, end time.Time, interval, tz string) ([]BucketSum, error) {
	var rows []BucketSum
	truncExpr := dateTrunc(interval, tz, "starts_at")
	tx := r.db.WithContext(ctx).
		Table("subscriptions").
		Select(truncExpr+" AS bucket, COUNT(*) AS sum", interval, tz).
		Where("starts_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Group("bucket").
		Order("bucket ASC")
	err := tx.Find(&rows).Error
	return rows, err
}

// ---------- MRR helpers ----------
func (r *dashboardRepository) ActiveSubscriptionsWithPlan(ctx context.Context) ([]SubWithPlan, error) {
	var rows []SubWithPlan
	// Active = now within window AND status in ('active','trialing','past_due')
	now := time.Now().Unix()
	err := r.db.WithContext(ctx).
		Table("subscriptions s").
		Select("s.id AS sub_id, s.plan_id, p.period, p.price_minor, s.status").
		Joins("JOIN plans p ON p.id = s.plan_id").
		Where("s.starts_at <= ? AND s.ends_at >= ?", now, now).
		Where("s.status IN ?", []dbm.SubscriptionStatus{dbm.SubStatusActive, dbm.SubStatusTrialing, dbm.SubStatusPastDue}).
		Find(&rows).Error
	return rows, err
}

// ---------- Plan mix ----------
func (r *dashboardRepository) PlanMix(ctx context.Context) ([]PlanMixRow, error) {
	var rows []PlanMixRow
	now := time.Now().Unix()
	err := r.db.WithContext(ctx).
		Table("subscriptions s").
		Select(`
			s.plan_id,
			p.code AS plan_code,
			p.name AS plan_name,
			p.period AS period,
			p.price_minor AS price_minor,
			COUNT(*) AS count`).
		Joins("JOIN plans p ON p.id = s.plan_id").
		Where("s.starts_at <= ? AND s.ends_at >= ?", now, now).
		Where("s.status IN ?", []dbm.SubscriptionStatus{dbm.SubStatusActive, dbm.SubStatusTrialing, dbm.SubStatusPastDue}).
		Group("s.plan_id, p.code, p.name, p.period, p.price_minor").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// ---------- Top destinations ----------
func (r *dashboardRepository) TopDestinations(ctx context.Context, start, end time.Time, limit int) ([]LocationRow, error) {
	var rows []LocationRow
	err := r.db.WithContext(ctx).
		Table("journeys").
		Select("location, COUNT(*) AS count").
		Where("created_at BETWEEN ? AND ?", start.Unix(), end.Unix()).
		Where("location <> ''").
		Group("location").
		Order("count DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// ---------- Recent payments ----------
func (r *dashboardRepository) RecentPaidTransactions(ctx context.Context, limit int) ([]RecentPaymentRow, error) {
	var rows []RecentPaymentRow
	// Join accounts for email
	err := r.db.WithContext(ctx).
		Table("transactions t").
		Select(`
			t.id,
			to_timestamp(t.paid_at) AT TIME ZONE 'UTC' AS paid_at,
			t.amount_minor,
			t.currency,
			t.status,
			t.provider,
			t.provider_txn_id,
			a.email`).
		Joins("LEFT JOIN accounts a ON a.id = t.account_id").
		Where("t.status = ?", dbm.TxnStatusPaid).
		Where("t.paid_at IS NOT NULL").
		Order("t.paid_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
