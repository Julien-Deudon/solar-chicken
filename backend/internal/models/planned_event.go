package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EventAction string

const (
	EventOpen             EventAction = "open"
	EventClose            EventAction = "close"
	EventLightBeforeOpen  EventAction = "light_before_open"  // lumière allumée avant l'ouverture
	EventLightAfterOpen   EventAction = "light_after_open"   // lumière éteinte après l'ouverture
	EventLightBeforeClose EventAction = "light_before_close" // lumière allumée avant la fermeture
	EventLightAfterClose  EventAction = "light_after_close"  // lumière éteinte après la fermeture
)

// IsDoor indique une action de porte (ouvrir/fermer).
func (a EventAction) IsDoor() bool { return a == EventOpen || a == EventClose }

// OmletCommand retourne l'action de l'API Omlet correspondante.
func (a EventAction) OmletCommand() string {
	switch a {
	case EventOpen:
		return "open"
	case EventClose:
		return "close"
	case EventLightBeforeOpen, EventLightBeforeClose:
		return "on"
	default:
		return "off"
	}
}

// DesiredDoorState retourne l'état attendu de la porte après l'action ("open"/"closed").
func (a EventAction) DesiredDoorState() string {
	if a == EventOpen {
		return "open"
	}
	return "closed"
}

type EventStatus string

const (
	StatusPending   EventStatus = "pending"   // à exécuter
	StatusSent      EventStatus = "sent"      // commande envoyée (ou horaire boîtier attendu), vérification en cours
	StatusConfirmed EventStatus = "confirmed" // état vérifié sur la porte
	StatusFailed    EventStatus = "failed"    // échec après relances (alerte envoyée)
	StatusSkipped   EventStatus = "skipped"   // dépassé, désactivé ou règle modifiée
	StatusShadow    EventStatus = "shadow"    // mode ombre : rien n'a été envoyé
)

// PlannedEvent est une action prévue pour un appareil, un jour donné.
// Une seule ligne par (appareil, jour, action) : pas de doublon possible.
type PlannedEvent struct {
	ID       uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	CoopID   uuid.UUID   `gorm:"type:uuid;not null;index" json:"coopId"`
	DeviceID uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex:ux_event_device_day_action" json:"deviceId"`
	Day      string      `gorm:"type:varchar(10);not null;uniqueIndex:ux_event_device_day_action;index" json:"day"`
	Action   EventAction `gorm:"type:varchar(20);not null;uniqueIndex:ux_event_device_day_action" json:"action"`
	DueAt    time.Time   `gorm:"not null;index" json:"dueAt"`
	Status   EventStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	Strategy Strategy    `gorm:"type:varchar(20);not null" json:"strategy"`

	Attempts    int        `gorm:"not null" json:"attempts"`
	NextCheckAt *time.Time `gorm:"index" json:"nextCheckAt,omitempty"`
	SentAt      *time.Time `json:"sentAt,omitempty"`
	DoneAt      *time.Time `json:"doneAt,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
	Note        string     `json:"note,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (e *PlannedEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
