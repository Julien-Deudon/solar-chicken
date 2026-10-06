package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/middleware"
	"github.com/julien-deudon/solar-chicken/backend/internal/engine"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"gorm.io/gorm"
)

// API regroupe les handlers v2 (poulaillers, appareils, règles, planning).
type API struct {
	DB      *gorm.DB
	Engine  *engine.Engine
	Clients func(apiKey string) engine.Omlet
	Version string
	Started time.Time
}

// lang retourne "fr" si le navigateur préfère le français, "en" sinon.
func lang(c *gin.Context) string {
	al := strings.ToLower(strings.TrimSpace(c.GetHeader("Accept-Language")))
	if al == "" || strings.HasPrefix(al, "fr") {
		return "fr"
	}
	return "en"
}

// tr choisit le message dans la langue du navigateur.
func tr(c *gin.Context, fr, en string) string {
	if lang(c) == "fr" {
		return fr
	}
	return en
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

func (a *API) user(c *gin.Context) uuid.UUID {
	id, _ := middleware.GetUserID(c)
	return id
}

func preloadDevices(db *gorm.DB) *gorm.DB {
	return db.Preload("Devices", func(db *gorm.DB) *gorm.DB { return db.Order("position, created_at") }).Preload("Devices.Rule")
}

// ownedCoop charge un poulailler de l'utilisateur connecté, avec ses appareils et règles.
func (a *API) ownedCoop(c *gin.Context, id string) (*models.Coop, bool) {
	var coop models.Coop
	if _, err := uuid.Parse(id); err != nil {
		fail(c, http.StatusNotFound, tr(c, "Poulailler introuvable", "Coop not found"))
		return nil, false
	}
	if err := preloadDevices(a.DB).Where("id = ? AND user_id = ?", id, a.user(c)).First(&coop).Error; err != nil {
		fail(c, http.StatusNotFound, tr(c, "Poulailler introuvable", "Coop not found"))
		return nil, false
	}
	return &coop, true
}

// ownedDevice charge un appareil (et son poulailler) appartenant à l'utilisateur connecté.
func (a *API) ownedDevice(c *gin.Context, id string) (*models.Device, *models.Coop, bool) {
	var dev models.Device
	if _, err := uuid.Parse(id); err != nil {
		fail(c, http.StatusNotFound, tr(c, "Appareil introuvable", "Device not found"))
		return nil, nil, false
	}
	if err := a.DB.Preload("Rule").First(&dev, "id = ?", id).Error; err != nil {
		fail(c, http.StatusNotFound, tr(c, "Appareil introuvable", "Device not found"))
		return nil, nil, false
	}
	coop, ok := a.ownedCoop(c, dev.CoopID.String())
	if !ok {
		return nil, nil, false
	}
	for i := range coop.Devices {
		if coop.Devices[i].ID == dev.ID {
			return &coop.Devices[i], coop, true
		}
	}
	return &dev, coop, true
}
