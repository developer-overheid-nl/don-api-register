package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"

	"context"
	"github.com/coreos/go-oidc"
)

func KeycloakAuthMiddleware(requiredScope string) gin.HandlerFunc {
	provider, err := oidc.NewProvider(context.Background(), "https://auth.don.apps.digilab.network/realms/don")
	if err != nil {
		panic(fmt.Sprintf("Fout bij ophalen OIDC provider: %v", err))
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: "don-admin-client"})

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header ontbreekt of is ongeldig"})
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		idToken, err := verifier.Verify(c.Request.Context(), rawToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Ongeldig token"})
			return
		}

		var claims map[string]interface{}
		if err := idToken.Claims(&claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Claims extractie mislukt"})
			return
		}

		// Optioneel: controleer scopes als access_token scopes bevat
		if requiredScope != "" {
			scopeList, ok := claims["scope"].(string)
			if !ok || !strings.Contains(scopeList, requiredScope) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Vereiste scope ontbreekt"})
				return
			}
		}

		// Middleware passed
		c.Set("user", claims)
		c.Next()
	}
}
