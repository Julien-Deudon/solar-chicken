package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActionStatus string

const (
	ActionStatusSuccess ActionStatus = "success"
	ActionStatusError   ActionStatus = "error"
)

type ActionTrigger string

const (
	ActionTriggerAuto     ActionTrigger = "auto"     // planning
	ActionTriggerManual   ActionTrigger = "manual"   // bouton dans l'interface
	ActionTriggerCatchup  ActionTrigger = "catchup"  // rattrapage après indisponibilité
	ActionTriggerFallback ActionTrigger = "fallback" // le boîtier n'a pas exécuté son horaire
	ActionTriggerSync     ActionTrigger = "sync"     // écriture des horaires dans le boîtier
)

// ActionLog trace chaque appel réellement envoyé à Omlet.
type ActionLog struct {
	ID           uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	DeviceID     uuid.UUID     `gorm:"type:uuid;not null;index" json:"deviceId"`
	ActionType   string        `gorm:"type:varchar(50);not null" json:"actionType"`
	Status       ActionStatus  `gorm:"type:varchar(20);not null" json:"status"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	TriggeredBy  ActionTrigger `gorm:"type:varchar(50)" json:"triggeredBy"`
	UserID       *uuid.UUID    `gorm:"type:uuid;index" json:"userId,omitempty"`
	Note         string        `json:"note,omitempty"`
	ExecutedAt   time.Time     `gorm:"index" json:"executedAt"`
}

func (a *ActionLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.ExecutedAt.IsZero() {
		a.ExecutedAt = time.Now()
	}
	return nil
}
