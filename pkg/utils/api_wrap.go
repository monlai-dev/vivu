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
		c.JSON(http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
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
		c.JSON(http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID.(string),
		})
	}
}
