package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const userIDKey = "userID"

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := s.ValidateSession(sessionID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(userIDKey, userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(userIDKey)
	if !exists {
		return 0, false
	}

	userID, ok := value.(int64)
	return userID, ok
}
