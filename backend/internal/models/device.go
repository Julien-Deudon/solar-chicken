package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceRole décrit le rôle d'un appareil dans le poulailler.
type DeviceRole string

const (
	RoleMainDoor DeviceRole = "main_door" // porte principale
	RoleNestBox  DeviceRole = "nest_box"  // pondoir
	RoleDoor     DeviceRole = "door"      // autre porte
	RoleFeeder   DeviceRole = "feeder"    // mangeoire (couvercle programmé comme une porte, ou surveillé)
)

func (r DeviceRole) Valid() bool {
	switch r {
	case RoleMainDoor, RoleNestBox, RoleDoor, RoleFeeder:
		return true
	}
	return false
}

// Strategy décrit comment le serveur fait exécuter les horaires.
type Strategy string

const (
	// StrategyCommand : le serveur envoie ouvrir/fermer à l'heure prévue (porte principale).
	StrategyCommand Strategy = "command"
	// StrategyOnboard : le serveur écrit chaque jour les horaires dans le boîtier (mode horaire),
	// la porte s'exécute seule même sans serveur ni internet ; le serveur vérifie et rattrape.
	StrategyOnboard Strategy = "onboard"
	// StrategyMonitor : aucune action automatique (ex. mangeoire), état affiché seulement.
	StrategyMonitor Strategy = "monitor"
)

func (s Strategy) Valid() bool {
	switch s {
	case StrategyCommand, StrategyOnboard, StrategyMonitor:
		return true
	}
	return false
}

// Device est un appareil Omlet rattaché à un poulailler.
type Device struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	CoopID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"coopId"`
	OmletDeviceID string     `gorm:"not null;uniqueIndex" json:"omletDeviceId"`
	DeviceType    string     `gorm:"not null" json:"deviceType"` // Autodoor, Feeder
	Role          DeviceRole `gorm:"type:varchar(20);not null" json:"role"`
	Name          string     `gorm:"not null" json:"name"`
	Strategy      Strategy   `gorm:"type:varchar(20);not null" json:"strategy"`
	HasLight      bool       `gorm:"not null" json:"hasLight"`
	Enabled       bool       `gorm:"not null" json:"enabled"`
	Position      int        `gorm:"not null" json:"position"`

	// Dernière synchronisation des horaires dans le boîtier (stratégie onboard)
	OnboardSyncedDay string     `json:"onboardSyncedDay,omitempty"`
	OnboardOpenTime  string     `json:"onboardOpenTime,omitempty"`
	OnboardCloseTime string     `json:"onboardCloseTime,omitempty"`
	OnboardSyncedAt  *time.Time `json:"onboardSyncedAt,omitempty"`
	OnboardSyncError string     `json:"onboardSyncError,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Rule *Rule `gorm:"foreignKey:DeviceID" json:"rule,omitempty"`
}

func (d *Device) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// IsDoor indique si l'appareil est une porte motorisée.
func (d *Device) IsDoor() bool { return d.DeviceType == "Autodoor" }

// IsFeeder indique si l'appareil est une mangeoire (couvercle motorisé).
func (d *Device) IsFeeder() bool { return d.DeviceType == "Feeder" }

// Opens indique si l'appareil s'ouvre et se ferme (porte ou mangeoire) : il peut avoir une règle.
func (d *Device) Opens() bool { return d.IsDoor() || d.IsFeeder() }

// Automated indique si le serveur planifie des actions pour cet appareil.
func (d *Device) Automated() bool {
	return d.Enabled && d.Opens() && d.Strategy != StrategyMonitor && d.Rule != nil
}

// StrategyFits indique si la stratégie convient au type d'appareil : une mangeoire se met en veille
// (commandes reçues avec retard), elle garde donc ses horaires dans son boîtier ou reste surveillée.
func StrategyFits(deviceType string, s Strategy) bool {
	if !s.Valid() {
		return false
	}
	return deviceType != "Feeder" || s != StrategyCommand
}
