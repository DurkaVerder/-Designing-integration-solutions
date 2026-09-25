package middleware

import (
	"github.com/gin-gonic/gin"
)

const AuthTokenHeader = "Authorization"

func (m *Middleware) LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		token := c.GetHeader(AuthTokenHeader)
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if err := m.jwtManager.ValidateToken(token); err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"error": err.Error(),
			})
			return
		}

		pl, err := m.jwtManager.GetPayload(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"error": err.Error(),
			})
			return
		}

		userID, ok := pl.Extra["user_id"].(string)
		if !ok || userID == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token payload"})
			return
		}
		c.Set("user_id", userID)

		c.Next()
	}
}
