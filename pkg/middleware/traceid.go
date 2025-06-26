package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := uuid.New().String()
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Trace-ID", traceID)
		c.Next()
	}
}
