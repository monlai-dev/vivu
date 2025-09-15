package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"vivu/pkg/utils"
)

func JWTAuthMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.RespondError(c, http.StatusUnauthorized, "Authorization header missing or invalid")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := utils.ValidateToken(tokenString)

		//ctx := context.Background()
		//isLoggedOut, err2 := IsJwtTokenLogout(ctx, redisClient, tokenString)

		//if isLoggedOut || err2 != nil {
		//	c.JSON(http.StatusOK, response_models.Response{
		//		ResponseCode: http.StatusUnauthorized,
		//		Message:      "Token is logged out",
		//	})
		//	c.Abort()
		//	return
		//}

		if err != nil {
			utils.RespondError(c, http.StatusUnauthorized, "Invalid or expired token")
			c.Abort()
			return
		}

		// Pass user information to the next handler
		c.Set("user_id", claims.ID)
		c.Set("Role", claims.Role)
		c.Next()
	}
}

func RoleMiddleware(requiredRole string) gin.HandlerFunc {

	return func(c *gin.Context) {
		role := c.GetString("Role")

		if role != requiredRole {
			utils.RespondError(c, http.StatusForbidden, "Forbidden: insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
