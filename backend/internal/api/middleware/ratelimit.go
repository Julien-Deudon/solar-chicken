package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// Store global partagé pour le rate limiting
var globalStore = memory.NewStore()

// RateLimitMiddleware crée un middleware de rate limiting basé sur l'IP
// rate: format "X-Y" où X requêtes par Y période (ex: "5-H" = 5 par heure)
func RateLimitMiddleware(rate string) gin.HandlerFunc {
	// Parse le rate (ex: "5-H" = 5 requêtes par heure)
	rateLimit := limiter.Rate{
		Period: 1 * time.Hour,
		Limit:  5,
	}

	// Créer le limiter avec le store partagé
	instance := limiter.New(globalStore, rateLimit)

	return func(c *gin.Context) {
		// Récupérer l'IP du client
		ip := c.ClientIP()

		// Vérifier le rate limit
		context, err := instance.Get(c.Request.Context(), ip)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limit error"})
			c.Abort()
			return
		}

		// Ajouter les headers de rate limit
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", context.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", context.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", context.Reset))

		// Si la limite est atteinte
		if context.Reached {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"message":     "Too many registration attempts. Please try again later.",
				"retry_after": context.Reset,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
