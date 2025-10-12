package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/payOSHQ/payos-lib-golang"
	"gorm.io/gorm"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	dbm "vivu/internal/models/db_models"
	"vivu/internal/models/response_models"
)

type PayOSConfig struct {
	ClientID     string // e.g. P-xxxxx
	ApiKey       string // public key if required by SDK
	ChecksumKey  string // secret used to sign webhooks (a.k.a. "client secret")
	BaseURL      string // optional: payOS API base if using sandbox
	ReturnURL    string // e.g. https://yourapp.com/pay/return
	CancelURL    string // e.g. https://yourapp.com/pay/cancel
	AppBaseURL   string // for building deep links if needed
	ProviderName string // "payos" (stored on Transaction.Provider)
}

type PaymentService interface {
	CreateCheckoutForPlan(ctx context.Context, accountID uuid.UUID, planCode string) (*response_models.CreateCheckoutResponse, error)
	HandleWebhook(c *gin.Context)
}

type paymentService struct {
	db  *gorm.DB
	cfg PayOSConfig
	loc *time.Location
}

func (p *paymentService) CreateCheckoutForPlan(ctx context.Context, accountID uuid.UUID, planCode string) (*response_models.CreateCheckoutResponse, error) {
	var plan dbm.Plan
	if err := p.db.WithContext(ctx).
		Where("code = ? AND is_active = TRUE", planCode).
		First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("plan not found: %s", planCode)
		}
		return nil, err
	}

	// Amount is in minor units (e.g., VND has 0 decimals, still treat as int64)
	amount := plan.PriceMinor
	if amount <= 0 {
		return nil, fmt.Errorf("plan %s is not billable (amount=%d)", planCode, amount)
	}

	// Generate a unique order code (payOS expects int64). Keep it within 13 digits.
	// We combine unix seconds + short random to reduce collision probability.
	rand.Seed(time.Now().UnixNano())
	orderCode := time.Now().Unix()%1_000_000_000 + int64(rand.Intn(9000)+1000)

	// Create a pending Transaction first (idempotency control via unique ProviderTxnID or OrderCode mapping)
	txn := &dbm.Transaction{
		AccountID:        accountID,
		AmountMinor:      amount,
		Currency:         strings.ToUpper(plan.Currency),
		Status:           dbm.TxnStatusPending,
		Provider:         p.cfg.ProviderName,
		ProviderTxnID:    fmt.Sprintf("payos:%d", orderCode), // link local record <-> provider order
		PaymentMethodRef: "",
	}

	if err := p.db.WithContext(ctx).Create(txn).Error; err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Build payOS items
	item := payos.Item{
		Name:     fmt.Sprintf("%s (%s)", plan.Name, plan.Code),
		Price:    int(amount), // SDK Item.Price is int
		Quantity: 1,
	}

	// Create checkout request
	body := payos.CheckoutRequestType{
		OrderCode:   orderCode,
		Amount:      int(amount),
		Items:       []payos.Item{item},
		Description: fmt.Sprintf("Subscription %s", plan.Code),
		CancelUrl:   p.cfg.CancelURL,
		ReturnUrl:   p.cfg.ReturnURL,
		// Optional: BuyerInfo, ExpireAt, etc.
	}

	clientErr := payos.Key(p.cfg.ClientID, p.cfg.ApiKey, p.cfg.ChecksumKey)

	if clientErr != nil {
		return nil, fmt.Errorf("payos client init: %w", clientErr)
	}

	resp, err := payos.CreatePaymentLink(body)
	if err != nil {
		_ = p.db.WithContext(ctx).Model(txn).
			Updates(map[string]interface{}{"status": dbm.TxnStatusFailed})
		return nil, fmt.Errorf("payos create link: %w", err)
	}

	// Store provider payload snapshot for traceability
	meta := map[string]any{
		"payos_link": resp,
		"plan_id":    plan.ID,
		"plan_code":  plan.Code,
	}

	if bytes, _ := json.Marshal(meta); bytes != nil {
		_ = p.db.WithContext(ctx).Model(txn).Update("metadata", bytes).Error
	}

	return &response_models.CreateCheckoutResponse{
		OrderCode:    orderCode,
		Amount:       amount,
		PaymentURL:   resp.CheckoutUrl, // field name per SDK
		ProviderName: p.cfg.ProviderName,
	}, nil
}

func (p *paymentService) HandleWebhook(c *gin.Context) {

	// 3) Parse minimal fields we need (adjust to actual payOS webhook schema)
	if err := payos.Key(os.Getenv("PAYOS_CLIENT_ID"),
		os.Getenv("PAYOS_API_KEY"),
		os.Getenv("CHECK_SUM_KEY")); err != nil {
		log.Fatalf("Error setting payos key: %v", err)
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	}

	var body payos.WebhookType
	if err := json.Unmarshal(rawBody, &body); err != nil {
		log.Printf("Error parsing webhook data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid webhook payload",
		})
		return
	}

	data, payosErr := payos.VerifyPaymentWebhookData(body)

	if payosErr != nil {
		log.Printf("Error verifying webhook data: %v", payosErr)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "Failed to verify webhook data",
		})
		return
	}

	if data.OrderCode == 123 {
		c.JSON(http.StatusOK, gin.H{
			"message": "COnfirm webhook complete",
		})
		return
	}

	orderCode := data.OrderCode
	providerTxn := fmt.Sprintf("payos:%d", orderCode)

	// 4) Load the pending transaction
	var txn dbm.Transaction
	if err := p.db.
		Where("provider_txn_id = ?", providerTxn).
		First(&txn).Error; err != nil {
		// If not found, ack 200 to avoid retries storm, but log for investigation.
		log.Printf("webhook: transaction not found for order %d", orderCode)

		return
	}

	// Idempotency: update only if currently pending/failed
	if txn.Status != dbm.TxnStatusPaid {
		now := time.Now().Unix()
		err = p.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&txn).Updates(map[string]interface{}{
				"status":  dbm.TxnStatusPaid,
				"paid_at": now,
			}).Error; err != nil {
				return err
			}
			// Activate/Create subscription
			return p.activateSubscription(tx, &txn)
		})
		if err != nil {
			log.Printf("webhook: failed to update txn/subscription for order %d: %v", orderCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to process transaction",
			})
			return
		}

	}
}

func (p *paymentService) activateSubscription(tx *gorm.DB,
	txn *dbm.Transaction) error {
	// Extract plan_code from txn.metadata (or store PlanID/PlanCode on Transaction explicitly)
	type meta struct {
		PlanID   uuid.UUID `json:"plan_id"`
		PlanCode string    `json:"plan_code"`
	}
	var m meta
	if err := json.Unmarshal(txn.Metadata, &m); err != nil || m.PlanCode == "" {
		// Fallback: resolve by amount/currency if pricing unique; safer to require plan_code in metadata
		return fmt.Errorf("missing plan info in transaction metadata")
	}

	var plan dbm.Plan
	if err := tx.Where("id = ? AND is_active = TRUE", m.PlanID).First(&plan).Error; err != nil {
		return fmt.Errorf("plan not found while activating: %w", err)
	}

	// Determine new period
	now := time.Now().In(p.loc)
	starts := now
	// If there is an active (or trialing) subscription that auto-renews, extend from its EndsAt
	var current dbm.Subscription
	err := tx.
		Where("account_id = ? AND status IN ? AND ends_at >= ?",
			txn.AccountID,
			[]dbm.SubscriptionStatus{dbm.SubStatusActive, dbm.SubStatusTrialing, dbm.SubStatusPastDue},
			now.Add(-24*time.Hour).Unix()).
		Order("ends_at DESC").
		First(&current).Error

	if err == nil && current.Status == dbm.SubStatusActive && current.AutoRenew && current.EndsAt > now.Unix() {
		starts = time.Unix(current.EndsAt, 0).In(p.loc) // extend from end
	}

	// Compute end
	var ends time.Time
	switch plan.Period {
	case dbm.PeriodYear:
		ends = starts.AddDate(1, 0, 0)
	default:
		ends = starts.AddDate(0, 1, 0)
	}

	// If plan has TrialDays and the account is new/no active sub, apply trial before paid window if desired.
	// In paid flow, many systems set trial before charging; here we bought already, so we just grant period.
	startsAt := starts.Unix()
	endsAt := ends.Unix()

	sub := dbm.Subscription{
		AccountID: txn.AccountID,
		PlanID:    plan.ID,
		Status:    dbm.SubStatusActive,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		AutoRenew: true,

		Provider:           p.cfg.ProviderName,
		ProviderCustomerID: "",                                           // payOS may not have customer concept; leave blank
		ProviderSubID:      strconv.FormatInt(time.Now().UnixNano(), 10), // unique placeholder

		Metadata: jsonRaw(map[string]any{
			"activated_by_txn": txn.ID,
			"amount_minor":     txn.AmountMinor,
			"currency":         txn.Currency,
		}),
	}

	if err := tx.Create(&sub).Error; err != nil {
		return err
	}

	// Optional: snapshot subscription on Account
	_ = tx.Model(&dbm.Account{BaseModel: dbm.BaseModel{ID: txn.AccountID}}).
		Update("subscription_snapshot", jsonRaw(sub)).Error

	return nil
}

func jsonRaw(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func NewPaymentService(db *gorm.DB, cfg PayOSConfig) (PaymentService, error) {
	if cfg.ClientID == "" || cfg.ApiKey == "" || cfg.ChecksumKey == "" {
		return nil, errors.New("missing payOS credentials")
	}
	// VN timezone normalization
	vnLoc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		vnLoc = time.FixedZone("ICT", 7*3600)
	}

	return &paymentService{
		db:  db,
		cfg: cfg,
		loc: vnLoc,
	}, nil
}
