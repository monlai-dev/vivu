package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"vivu/internal/models/request_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type PaymentController struct {
	paymentService services.PaymentService
}

func NewPaymentController(paymentService services.PaymentService) *PaymentController {
	return &PaymentController{
		paymentService: paymentService,
	}
}

// CreateCheckoutRequest godoc
// @Summary Create a checkout request for a subscription plan
// @Description Create a checkout request for a subscription plan
// @Tags Payments
// @Accept json
// @Produce json
// @Param request body request_models.CreatePaymentRequest true "Create Payment Request"
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Router /payments/create-checkout [post]
func (p *PaymentController) CreateCheckoutRequest(c *gin.Context) {

	var request request_models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	userid := c.GetString("user_id")

	if userid == "" {
		utils.RespondError(c, http.StatusBadRequest, "user_id is required")
		return
	}

	userId, _ := uuid.Parse(userid)

	checkoutURL, err := p.paymentService.CreateCheckoutForPlan(c.Request.Context(), userId, request.PlanCode)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, gin.H{"checkout_url": checkoutURL}, "Checkout URL created successfully")
}

func (p *PaymentController) HandleWebhook(c *gin.Context) {
	p.paymentService.HandleWebhook(c)
}

// GetListOfAvailablePlans godoc
// @Summary Get list of available subscription plans
// @Description Retrieve a list of available subscription plans
// @Tags Payments
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse
// @Router /payments/plans [get]
func (p *PaymentController) GetListOfAvailablePlans(c *gin.Context) {
	plans, err := p.paymentService.GetListOfPlans(c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, plans, "List of available plans retrieved successfully")
}

// GetSubscriptionDetails godoc
// @Summary Get subscription details for the authenticated user
// @Description Retrieve subscription details for the authenticated user
// @Tags Payments
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Router /payments/subscription-details [get]
func (p *PaymentController) GetSubscriptionDetails(c *gin.Context) {
	userid := c.GetString("user_id")

	if userid == "" {
		utils.RespondError(c, http.StatusBadRequest, "user_id is required")
		return
	}

	userId, _ := uuid.Parse(userid)

	subscription, err := p.paymentService.GetStatusOfSubscription(c.Request.Context(), userId)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, subscription, "Subscription details retrieved successfully")
}

// GetAllTransactionHistory godoc
// @Summary Get all transaction history
// @Description Retrieve all transaction history
// @Tags Payments
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse
// @Router /payments/transaction-history [get]
func (p *PaymentController) GetAllTransactionHistory(c *gin.Context) {

	data, err := p.paymentService.GetAllTransactions(c.Request.Context())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, data, "Transaction history retrieved successfully")
}
