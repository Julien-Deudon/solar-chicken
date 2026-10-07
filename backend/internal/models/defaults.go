package models

import "github.com/google/uuid"

// DefaultStrategy retourne la stratégie conseillée : portes et mangeoires gardent leurs horaires dans leur
// boîtier (elles fonctionnent même si le serveur ou internet tombe) ; le serveur vérifie et commande en secours.
func DefaultStrategy(role DeviceRole) Strategy {
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
// Pondoir : s'ouvre avec la porte principale et se ferme 2 h avant elle ; mangeoire : s'ouvre avec la porte
// principale, se ferme au coucher du soleil. Sans porte principale, les mêmes horaires calculés sur le soleil.
func DefaultRule(role DeviceRole, hasLight bool, mainDoor *uuid.UUID) *Rule {
	switch role {
	case RoleFeeder:
		// S'ouvre avec la porte principale (ou 10 min avant le lever), se ferme au coucher du soleil.
		r := &Rule{OpenAnchor: AnchorSunrise, OpenOffsetMinutes: defaultOpenOffset, CloseAnchor: AnchorSunset}
		if mainDoor != nil {
			ref := *mainDoor
			r.OpenAnchor, r.OpenOffsetMinutes, r.OpenRefDeviceID = AnchorDeviceOpen, 0, &ref
		}
		return r
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
