package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
)

// System : GET /system — mode d'exécution et santé du moteur.
func (a *API) System(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"mode": a.Engine.Mode, "version": a.Version, "now": time.Now(),
		"lastTick": a.Engine.LastTick(), "lastPlan": a.Engine.LastPlan(), "startedAt": a.Started,
	})
}

// Health : GET /health — 503 si la base ne répond pas ou si le moteur est figé.
func (a *API) Health(c *gin.Context) {
	sqlDB, err := a.DB.DB()
	if err == nil {
		err = sqlDB.PingContext(c.Request.Context())
	}
	stale := time.Since(a.Started) > 3*time.Minute && time.Since(a.Engine.LastTick()) > 2*time.Minute
	status := http.StatusOK
	state := "ok"
	if err != nil || stale {
		status, state = http.StatusServiceUnavailable, "degraded"
	}
	c.JSON(status, gin.H{"status": state, "mode": a.Engine.Mode, "lastTick": a.Engine.LastTick(), "dbOk": err == nil})
}

// Bootstrap : GET /system/bootstrap (sans connexion) — premier démarrage ?
func (a *API) Bootstrap(allowRegistration bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users int64
		a.DB.Model(&models.User{}).Count(&users)
		c.JSON(http.StatusOK, gin.H{"needsSetup": users == 0, "registrationOpen": allowRegistration || users == 0, "version": a.Version})
	}
}
