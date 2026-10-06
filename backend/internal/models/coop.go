package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Coop est un poulailler : un lieu (position, fuseau) et un compte Omlet.
// Il correspond à une "coop" de l'application Omlet (groupId dans l'API).
type Coop struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	Name         string    `gorm:"not null" json:"name"`
	OmletAPIKey  string    `gorm:"not null;serializer:encrypted" json:"-"`
	OmletGroupID string    `gorm:"index" json:"omletGroupId"`
	Latitude     float64   `gorm:"not null" json:"latitude"`
	Longitude    float64   `gorm:"not null" json:"longitude"`
	Timezone     string    `gorm:"not null" json:"timezone"`
	// Langue des notifications et des messages du planning ("fr" ou "en")
	Language string `gorm:"type:varchar(5);not null;default:'fr'" json:"language"`

	// Jour (AAAA-MM-JJ local) du dernier planning quotidien envoyé
	LastDailyReportDay string `json:"-"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Devices []Device `gorm:"foreignKey:CoopID" json:"devices,omitempty"`
}

func (c *Coop) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// Location retourne le fuseau horaire du poulailler.
func (c *Coop) Location() (*time.Location, error) { return time.LoadLocation(c.Timezone) }
