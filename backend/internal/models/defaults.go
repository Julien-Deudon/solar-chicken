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

// DefaultRule retourne la règle proposée à l'ajout d'un appareil.
// Pondoir : ouvre 30 min après la porte principale (pas avant 08:00), ferme 1 h avant le coucher.
func DefaultRule(role DeviceRole, hasLight bool, mainDoor *uuid.UUID) *Rule {
	switch role {
	case RoleFeeder:
		return nil
	case RoleNestBox:
		r := &Rule{
			OpenAnchor: AnchorSunrise, OpenOffsetMinutes: 40,
			OpenNotBefore: &TimeOfDay{Hour: 8},
			CloseAnchor:   AnchorSunset, CloseOffsetMinutes: -60,
		}
		if mainDoor != nil {
			id := *mainDoor
			r.OpenAnchor, r.OpenOffsetMinutes, r.OpenRefDeviceID = AnchorDeviceOpen, 30, &id
		}
		return r
	default:
		r := &Rule{
			OpenAnchor: AnchorSunrise, OpenOffsetMinutes: -10,
			CloseAnchor: AnchorSunset, CloseOffsetMinutes: 20,
		}
		if hasLight {
			r.LightBeforeOpenMinutes, r.EnableLightMorning, r.EnableLightEvening = 2, true, true
		}
		return r
	}
}
