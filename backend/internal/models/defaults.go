package models

import "github.com/google/uuid"

// DefaultStrategy retourne la stratégie conseillée pour un rôle : les portes gardent leurs horaires
// dans leur boîtier (elles fonctionnent même si le serveur ou internet tombe), la mangeoire est surveillée.
func DefaultStrategy(role DeviceRole) Strategy {
	if role == RoleFeeder {
		return StrategyMonitor
	}
	return StrategyOnboard
}

// Règle proposée à l'ajout d'une porte : 10 min avant le lever, 20 min après le coucher.
const (
	defaultOpenOffset  = -10
	defaultCloseOffset = 20
	// Le pondoir se ferme 2 h avant la porte principale, pour que les poules n'y dorment pas.
	nestBoxCloseEarly = 120
)

// DefaultRule retourne la règle proposée à l'ajout d'un appareil ; elle se modifie ensuite dans ses réglages.
// Pondoir : s'ouvre avec la porte principale et se ferme 2 h avant elle ; sans porte principale,
// les mêmes horaires calculés directement sur le soleil.
func DefaultRule(role DeviceRole, hasLight bool, mainDoor *uuid.UUID) *Rule {
	switch role {
	case RoleFeeder:
		return nil
	case RoleNestBox:
		r := &Rule{
			OpenAnchor: AnchorSunrise, OpenOffsetMinutes: defaultOpenOffset,
			CloseAnchor: AnchorSunset, CloseOffsetMinutes: defaultCloseOffset - nestBoxCloseEarly,
		}
		if mainDoor != nil {
			openRef, closeRef := *mainDoor, *mainDoor
			r.OpenAnchor, r.OpenOffsetMinutes, r.OpenRefDeviceID = AnchorDeviceOpen, 0, &openRef
			r.CloseAnchor, r.CloseOffsetMinutes, r.CloseRefDeviceID = AnchorDeviceClose, -nestBoxCloseEarly, &closeRef
		}
		return r
	default:
		r := &Rule{
			OpenAnchor: AnchorSunrise, OpenOffsetMinutes: defaultOpenOffset,
			CloseAnchor: AnchorSunset, CloseOffsetMinutes: defaultCloseOffset,
		}
		if hasLight {
			r.LightBeforeOpenMinutes, r.EnableLightMorning, r.EnableLightEvening = 2, true, true
		}
		return r
	}
}
