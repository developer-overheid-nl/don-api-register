package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"strings"
)

func RequireAccess(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Als er een geldige x-api-key was (door APISIX gevalideerd)
		if c.GetHeader("x-api-key") != "" {
			if c.Request.Method != http.MethodGet {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "x-api-key only grants read access"})
				return
			}

			c.Set("auth_method", "api_key")
			c.Next()
			return
		}

		// Anders: JWT token check
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if !hasScope(tokenStr, requiredScope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access token missing required scope"})
			return
		}

		c.Set("auth_method", "jwt_token")
		c.Next()
	}
}

func hasScope(tokenStr, requiredScope string) bool {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}

	scopeStr, ok := claims["scope"].(string)
	if !ok {
		return false
	}

	for _, scope := range strings.Split(scopeStr, " ") {
		if scope == requiredScope {
			return true
		}
	}

	return false
}
