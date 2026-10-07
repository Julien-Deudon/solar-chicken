// Package planner calcule le planning d'une journée pour un poulailler.
// Fonction pure : aucune base de données, aucun appel réseau.
package planner

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/pkg/suncalc"
)

// Event est une action prévue.
type Event struct {
	DeviceID uuid.UUID
	Action   models.EventAction
	DueAt    time.Time
}

// DoorTimes contient les heures calculées d'une porte.
type DoorTimes struct {
	Open  time.Time
	Close time.Time
}

// DayPlan est le planning d'un poulailler pour un jour local.
type DayPlan struct {
	Day     string // AAAA-MM-JJ, heure locale du poulailler
	Sunrise time.Time
	Sunset  time.Time
	Doors   map[uuid.UUID]DoorTimes
	Events  []Event
	Errors  map[uuid.UUID]string // message lisible (langue du poulailler)
	Codes   map[uuid.UUID]string // code stable : cycle, missing_ref, close_before_open, unknown_anchor
}

// Codes d'erreur stables.
const (
	CodeCycle           = "cycle"
	CodeMissingRef      = "missing_ref"
	CodeCloseBeforeOpen = "close_before_open"
	CodeUnknownAnchor   = "unknown_anchor"
)

// planError porte un code stable et un message dans la langue du poulailler.
type planError struct{ code, msg string }

func (e *planError) Error() string { return e.msg }

func tr(lang, fr, en string) string {
	if lang == "en" {
		return en
	}
	return fr
}

// DayKey retourne le jour local (AAAA-MM-JJ) d'un instant.
func DayKey(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006-01-02") }

// PlanDay calcule le planning du jour local contenant `day`.
// Les heures solaires viennent du même calcul que la v1 (pkg/suncalc).
func PlanDay(coop *models.Coop, devices []models.Device, day time.Time) (*DayPlan, error) {
	loc, err := coop.Location()
	if err != nil {
		return nil, fmt.Errorf("fuseau horaire invalide : %w", err)
	}
	d := day.In(loc)
	dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	sun, err := suncalc.NewCalculator(coop.Latitude, coop.Longitude, coop.Timezone).GetSunTimes(dayStart)
	if err != nil {
		return nil, err
	}

	plan := &DayPlan{
		Day:     dayStart.Format("2006-01-02"),
		Sunrise: sun.Sunrise,
		Sunset:  sun.Sunset,
		Doors:   map[uuid.UUID]DoorTimes{},
		Errors:  map[uuid.UUID]string{},
		Codes:   map[uuid.UUID]string{},
	}
	lang := coop.Language
	r := &resolver{plan: plan, loc: loc, day: dayStart, lang: lang, byID: map[uuid.UUID]*models.Device{}, visiting: map[uuid.UUID]bool{}}
	for i := range devices {
		r.byID[devices[i].ID] = &devices[i]
	}
	for i := range devices {
		dev := &devices[i]
		if !dev.Opens() || dev.Rule == nil {
			continue
		}
		if _, err := r.door(dev.ID); err != nil {
			plan.Errors[dev.ID] = err.Error()
			plan.Codes[dev.ID] = CodeMissingRef
			var pe *planError
			if errors.As(err, &pe) {
				plan.Codes[dev.ID] = pe.code
			}
		}
	}

	for i := range devices {
		dev := &devices[i]
		t, ok := plan.Doors[dev.ID]
		if !dev.Automated() || !ok {
			continue
		}
		if !t.Close.After(t.Open) {
			plan.Errors[dev.ID] = fmt.Sprintf(tr(lang, "fermeture (%s) avant l'ouverture (%s) : aucune action prévue ce jour",
				"closing (%s) before opening (%s): nothing scheduled that day"), t.Close.Format("15:04"), t.Open.Format("15:04"))
			plan.Codes[dev.ID] = CodeCloseBeforeOpen
			continue
		}
		open, close := t.Open, t.Close
		if dev.Strategy == models.StrategyOnboard {
			// Le boîtier ne connaît que HH:MM
			open, close = open.Round(time.Minute), close.Round(time.Minute)
		}
		rule := dev.Rule
		add := func(a models.EventAction, at time.Time) {
			plan.Events = append(plan.Events, Event{DeviceID: dev.ID, Action: a, DueAt: at})
		}
		min := func(n int) time.Duration { return time.Duration(n) * time.Minute }
		light := dev.HasLight && dev.Strategy == models.StrategyCommand
		if light && rule.EnableLightMorning && rule.LightBeforeOpenMinutes > 0 {
			add(models.EventLightBeforeOpen, open.Add(-min(rule.LightBeforeOpenMinutes)))
		}
		add(models.EventOpen, open)
		if light && rule.EnableLightMorning && rule.LightOffDelayMinutes > 0 {
			add(models.EventLightAfterOpen, open.Add(min(rule.LightOffDelayMinutes)))
		}
		if light && rule.EnableLightEvening && rule.LightBeforeCloseMinutes > 0 {
			add(models.EventLightBeforeClose, close.Add(-min(rule.LightBeforeCloseMinutes)))
		}
		add(models.EventClose, close)
		if light && rule.EnableLightEvening && rule.LightOffDelayMinutes > 0 {
			add(models.EventLightAfterClose, close.Add(min(rule.LightOffDelayMinutes)))
		}
	}
	sort.SliceStable(plan.Events, func(i, j int) bool { return plan.Events[i].DueAt.Before(plan.Events[j].DueAt) })
	return plan, nil
}

type resolver struct {
	plan     *DayPlan
	lang     string
	loc      *time.Location
	day      time.Time
	byID     map[uuid.UUID]*models.Device
	visiting map[uuid.UUID]bool
}

func (r *resolver) door(id uuid.UUID) (DoorTimes, error) {
	if t, ok := r.plan.Doors[id]; ok {
		return t, nil
	}
	dev, ok := r.byID[id]
	if !ok || !dev.Opens() || dev.Rule == nil {
		return DoorTimes{}, &planError{CodeMissingRef, tr(r.lang, "appareil de référence introuvable ou sans règle", "reference device not found or without a rule")}
	}
	if r.visiting[id] {
		return DoorTimes{}, &planError{CodeCycle, tr(r.lang, "référence circulaire entre appareils", "circular reference between devices")}
	}
	r.visiting[id] = true
	defer delete(r.visiting, id)

	open, err := r.moment(dev.Rule.Open())
	if err != nil {
		return DoorTimes{}, wrap(err, tr(r.lang, "ouverture : ", "opening: "))
	}
	close, err := r.moment(dev.Rule.Close())
	if err != nil {
		return DoorTimes{}, wrap(err, tr(r.lang, "fermeture : ", "closing: "))
	}
	t := DoorTimes{Open: open, Close: close}
	r.plan.Doors[id] = t
	return t, nil
}

func (r *resolver) moment(m models.Moment) (time.Time, error) {
	var base time.Time
	switch m.Anchor {
	case models.AnchorSunrise:
		base = r.plan.Sunrise
	case models.AnchorSunset:
		base = r.plan.Sunset
	case models.AnchorFixed:
		if m.FixedTime == nil {
			return time.Time{}, &planError{CodeMissingRef, tr(r.lang, "heure fixe manquante", "fixed time missing")}
		}
		base = m.FixedTime.On(r.day, r.loc)
	case models.AnchorDeviceOpen, models.AnchorDeviceClose:
		if m.RefDeviceID == nil {
			return time.Time{}, &planError{CodeMissingRef, tr(r.lang, "appareil de référence manquant", "reference device missing")}
		}
		t, err := r.door(*m.RefDeviceID)
		if err != nil {
			return time.Time{}, err
		}
		base = t.Open
		if m.Anchor == models.AnchorDeviceClose {
			base = t.Close
		}
	default:
		return time.Time{}, &planError{CodeUnknownAnchor, fmt.Sprintf(tr(r.lang, "repère %q inconnu", "unknown anchor %q"), m.Anchor)}
	}
	at := base.Add(time.Duration(m.OffsetMinutes) * time.Minute)
	if m.NotBefore != nil {
		if nb := m.NotBefore.On(r.day, r.loc); at.Before(nb) {
			at = nb
		}
	}
	if m.NotAfter != nil {
		if na := m.NotAfter.On(r.day, r.loc); at.After(na) {
			at = na
		}
	}
	return at, nil
}

// wrap préfixe le message en conservant le code.
func wrap(err error, prefix string) error {
	var pe *planError
	if errors.As(err, &pe) {
		return &planError{pe.code, prefix + pe.msg}
	}
	return &planError{CodeMissingRef, prefix + err.Error()}
}
