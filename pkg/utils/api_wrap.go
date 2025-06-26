package utils

import (
	"errors"
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

func HandleServiceError(c *gin.Context, err error) {
	traceID, _ := c.Get("trace_id")

	switch {
	case errors.Is(err, ErrTagNotFound):
		c.JSON(http.StatusNotFound, APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Tag not found",
			TraceID: traceID.(string),
		})
	case errors.Is(err, ErrInvalidPage):
		c.JSON(http.StatusBadRequest, APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Page must be greater than 0",
			TraceID: traceID.(string),
		})
	case errors.Is(err, ErrInvalidPageSize):
		c.JSON(http.StatusBadRequest, APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Page size must be between 1 and 100",
			TraceID: traceID.(string),
		})
	case errors.Is(err, ErrDatabaseError):
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID.(string),
		})
	default:
		log.Printf("Unknown error: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID.(string),
		})
	}
}
