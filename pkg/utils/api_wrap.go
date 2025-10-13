package utils

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type APIResponse struct {
	Status  string      `json:"status"`
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var errorHandlers = map[error]func(*gin.Context, string){
	ErrTagNotFound: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusOK,
			Message: "Tag not found",
			TraceID: traceID,
		})
	},
	ErrInvalidPage: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Page must be greater than 0",
			TraceID: traceID,
		})
	},
	ErrInvalidPageSize: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Page size must be between 1 and 100",
			TraceID: traceID,
		})
	},
	ErrDatabaseError: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID,
		})
	},
	ErrUnexpectedBehaviorOfAI: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Unexpected error from AI service",
			TraceID: traceID,
		})
	},
	ErrPoorQualityInput: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "improve_input",
			Code:    http.StatusBadRequest,
			Message: "Input quality is too low please consider improving it so we can help you better",
			TraceID: traceID,
		})
	},
	ErrInvalidInput: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "bad Request",
			Code:    http.StatusBadRequest,
			Message: "Invalid input",
			TraceID: traceID,
		})
	},
	ErrAccountNotFound: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Account not found",
			TraceID: traceID,
		})
	},
	ErrInvalidCredentials: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusUnauthorized,
			Message: "User or password is incorrect",
			TraceID: traceID,
		})
	},
	ErrEmailAlreadyExists: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusConflict,
			Message: "Email already exists",
			TraceID: traceID,
		})
	},
	ErrJourneyNotFound: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Journey not found",
			TraceID: traceID,
		})
	},
	ErrThirdService: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusBadGateway,
			Message: "Error from third party service",
			TraceID: traceID,
		})
	},
	ErrInvalidToken: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusUnauthorized,
			Message: "Invalid token",
			TraceID: traceID,
		})
	},
	ErrUserDoNotHavePremium: func(c *gin.Context, traceID string) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Status:  "error",
			Code:    http.StatusForbidden,
			Message: "User do not have premium access to generate plan more than 3 days",
			TraceID: traceID,
		})
	},
}

func RespondSuccess(c *gin.Context, data interface{}, message string) {
	traceID, _ := c.Get("trace_id")
	c.JSON(http.StatusOK, APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: message,
		TraceID: traceID.(string),
		Data:    data,
	})
}

func RespondError(c *gin.Context, code int, message string) {
	traceID, _ := c.Get("trace_id")
	c.JSON(code, APIResponse{
		Status:  "error",
		Code:    code,
		Message: message,
		TraceID: traceID.(string),
	})
}

// HandleServiceError O(1)
func HandleServiceError(c *gin.Context, err error) {
	traceID, _ := c.Get("trace_id")

	if handler, exists := errorHandlers[err]; exists {
		handler(c, traceID.(string))
	} else {
		log.Printf("Unknown error: %v", err)
		c.JSON(http.StatusOK, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID.(string),
		})
	}
}
