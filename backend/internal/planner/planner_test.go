package planner

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/pkg/suncalc"
)

func tod(h, m int) *models.TimeOfDay { return &models.TimeOfDay{Hour: h, Minute: m} }

func mainDoor(id uuid.UUID) models.Device {
	return models.Device{
		ID: id, DeviceType: "Autodoor", Role: models.RoleMainDoor, Name: "Porte principale",
		Strategy: models.StrategyCommand, HasLight: true, Enabled: true,
		Rule: &models.Rule{
			DeviceID:   id,
			OpenAnchor: models.AnchorSunrise, OpenOffsetMinutes: -10,
			CloseAnchor: models.AnchorSunset, CloseOffsetMinutes: 20,
			LightBeforeOpenMinutes: 2, LightBeforeCloseMinutes: 0,
			EnableLightMorning: true, EnableLightEvening: true,
		},
	}
}

func find(t *testing.T, p *DayPlan, dev uuid.UUID, a models.EventAction) time.Time {
	t.Helper()
	for _, e := range p.Events {
		if e.DeviceID == dev && e.Action == a {
			return e.DueAt
		}
	}
	t.Fatalf("%s : événement %s absent (erreurs : %v)", p.Day, a, p.Errors)
	return time.Time{}
}

// Équivalence avec le calcul de la v1 (scheduler.calculateNextActions, mode soleil)
// sur 400 jours, passages heure d'été / heure d'hiver compris.
func TestMainDoorMatchesV1ForEveryDay(t *testing.T) {
	for _, loc := range []struct{ lat, lon float64 }{{50.4, 2.8}, {48.8566, 2.3522}} {
		coop := &models.Coop{Latitude: loc.lat, Longitude: loc.lon, Timezone: "Europe/Paris"}
		id := uuid.New()
		devices := []models.Device{mainDoor(id)}
		paris, _ := time.LoadLocation("Europe/Paris")
		start := time.Date(2026, 10, 1, 12, 0, 0, 0, paris)
		for i := 0; i < 400; i++ {
			day := start.AddDate(0, 0, i)
			p, err := PlanDay(coop, devices, day)
			if err != nil {
				t.Fatal(err)
			}
			v1, err := suncalc.NewCalculator(loc.lat, loc.lon, "Europe/Paris").GetSunTimesWithOffset(day, -10, 20)
			if err != nil {
				t.Fatal(err)
			}
			if got := find(t, p, id, models.EventOpen); !got.Equal(v1.Sunrise) {
				t.Fatalf("%s ouverture %s, v1 %s", p.Day, got, v1.Sunrise)
			}
			if got := find(t, p, id, models.EventClose); !got.Equal(v1.Sunset) {
				t.Fatalf("%s fermeture %s, v1 %s", p.Day, got, v1.Sunset)
			}
			if got := find(t, p, id, models.EventLightBeforeOpen); !got.Equal(v1.Sunrise.Add(-2 * time.Minute)) {
				t.Fatalf("%s lumière %s", p.Day, got)
			}
			if len(p.Events) != 3 {
				t.Fatalf("%s : %d événements, 3 attendus (lumière, ouverture, fermeture)", p.Day, len(p.Events))
			}
		}
	}
}

// Ordre de grandeur : début octobre près d'Arras, la porte s'ouvre entre 07:30 et 08:00 (heure de Paris).
func TestKnownTargetsAroundArras(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	id := uuid.New()
	p, err := PlanDay(coop, []models.Device{mainDoor(id)}, time.Date(2026, 10, 7, 9, 0, 0, 0, paris))
	if err != nil {
		t.Fatal(err)
	}
	open := find(t, p, id, models.EventOpen)
	if open.Hour() != 7 || open.Before(time.Date(2026, 10, 7, 7, 30, 0, 0, paris)) || open.After(time.Date(2026, 10, 7, 8, 0, 0, 0, paris)) {
		t.Fatalf("ouverture inattendue %s", open)
	}
}

// Règle par défaut du pondoir : il s'ouvre avec la porte principale et se ferme 2 h avant elle
// (à 30 s près : son boîtier ne garde que HH:MM, l'heure est arrondie à la minute la plus proche) ;
// sans porte principale, les mêmes horaires calculés sur le soleil.
func TestDefaultNestBoxRuleFollowsMainDoor(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	mainID, nestID := uuid.New(), uuid.New()
	rule := models.DefaultRule(models.RoleNestBox, false, &mainID)
	rule.DeviceID = nestID
	nest := models.Device{ID: nestID, DeviceType: "Autodoor", Role: models.RoleNestBox, Name: "Pondoir",
		Strategy: models.StrategyOnboard, Enabled: true, Rule: rule}
	for day := 0; day < 365; day += 7 {
		p, err := PlanDay(coop, []models.Device{nest, mainDoor(mainID)}, time.Date(2026, 10, 8+day, 12, 0, 0, 0, paris))
		if err != nil {
			t.Fatal(err)
		}
		if d := find(t, p, nestID, models.EventOpen).Sub(find(t, p, mainID, models.EventOpen)); d.Abs() > 30*time.Second {
			t.Fatalf("%s : pondoir ouvert %s après la porte", p.Day, d)
		}
		if d := find(t, p, nestID, models.EventClose).Sub(find(t, p, mainID, models.EventClose).Add(-2 * time.Hour)); d.Abs() > 30*time.Second {
			t.Fatalf("%s : fermeture du pondoir décalée de %s par rapport à « 2 h avant la porte »", p.Day, d)
		}
	}
	alone := models.DefaultRule(models.RoleNestBox, false, nil)
	if alone.OpenAnchor != models.AnchorSunrise || alone.OpenOffsetMinutes != -10 || alone.CloseAnchor != models.AnchorSunset || alone.CloseOffsetMinutes != -100 {
		t.Fatalf("pondoir sans porte principale : %+v", alone)
	}
}

// Porte principale qui garde ses horaires dans son boîtier : heures à la minute, et la lampe reste allumée
// par le serveur 2 min avant l'ouverture (Omlet ne sait pas le faire seul le matin).
func TestOnboardMainDoorKeepsServerLight(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	id := uuid.New()
	door := mainDoor(id)
	door.Strategy = models.StrategyOnboard
	p, err := PlanDay(coop, []models.Device{door}, time.Date(2026, 10, 8, 12, 0, 0, 0, paris))
	if err != nil {
		t.Fatal(err)
	}
	open := find(t, p, id, models.EventOpen)
	if open.Second() != 0 {
		t.Fatalf("ouverture %s : le boîtier ne garde que HH:MM", open)
	}
	if light := find(t, p, id, models.EventLightBeforeOpen); !light.Equal(open.Add(-2 * time.Minute)) {
		t.Fatalf("lampe à %s, ouverture à %s", light, open)
	}
}

func TestNestBoxRelativeToMainDoor(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	mainID, nestID := uuid.New(), uuid.New()
	nest := models.Device{
		ID: nestID, DeviceType: "Autodoor", Role: models.RoleNestBox, Name: "Pondoir",
		Strategy: models.StrategyOnboard, Enabled: true,
		Rule: &models.Rule{
			DeviceID:   nestID,
			OpenAnchor: models.AnchorDeviceOpen, OpenOffsetMinutes: 30, OpenRefDeviceID: &mainID, OpenNotBefore: tod(8, 0),
			CloseAnchor: models.AnchorSunset, CloseOffsetMinutes: -60,
		},
	}
	devices := []models.Device{nest, mainDoor(mainID)} // ordre volontairement inversé
	// Octobre : porte vers 07:46 → +30 = 08:16 (au-dessus de 08:00)
	p, err := PlanDay(coop, devices, time.Date(2026, 10, 7, 12, 0, 0, 0, paris))
	if err != nil {
		t.Fatal(err)
	}
	mainOpen := find(t, p, mainID, models.EventOpen)
	nestOpen := find(t, p, nestID, models.EventOpen)
	if want := mainOpen.Add(30 * time.Minute).Round(time.Minute); !nestOpen.Equal(want) {
		t.Fatalf("pondoir ouvre %s, attendu %s", nestOpen, want)
	}
	if nestOpen.Second() != 0 {
		t.Fatalf("stratégie boîtier : heure non arrondie à la minute (%s)", nestOpen)
	}
	nestClose := find(t, p, nestID, models.EventClose)
	if want := p.Sunset.Add(-60 * time.Minute).Round(time.Minute); !nestClose.Equal(want) {
		t.Fatalf("pondoir ferme %s, attendu %s", nestClose, want)
	}
	for _, e := range p.Events {
		if e.DeviceID == nestID && !e.Action.IsDoor() {
			t.Fatalf("pondoir : aucune lumière attendue, trouvé %s", e.Action)
		}
	}
	// Juin : porte vers 05:2x → +30 = 05:5x, borné à 08:00
	p, _ = PlanDay(coop, devices, time.Date(2027, 6, 15, 12, 0, 0, 0, paris))
	if got := find(t, p, nestID, models.EventOpen); got.Hour() != 8 || got.Minute() != 0 {
		t.Fatalf("borne « pas avant 08:00 » non appliquée : %s", got)
	}
}

func TestCycleIsReported(t *testing.T) {
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	a, b := uuid.New(), uuid.New()
	mk := func(id, ref uuid.UUID) models.Device {
		return models.Device{ID: id, DeviceType: "Autodoor", Strategy: models.StrategyCommand, Enabled: true,
			Rule: &models.Rule{DeviceID: id, OpenAnchor: models.AnchorDeviceOpen, OpenRefDeviceID: &ref,
				CloseAnchor: models.AnchorSunset}}
	}
	p, err := PlanDay(coop, []models.Device{mk(a, b), mk(b, a)}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != 0 || p.Errors[a] == "" || p.Errors[b] == "" {
		t.Fatalf("cycle non détecté : events=%d errors=%v", len(p.Events), p.Errors)
	}
}

func TestCloseBeforeOpenIsRejected(t *testing.T) {
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	id := uuid.New()
	d := models.Device{ID: id, DeviceType: "Autodoor", Strategy: models.StrategyCommand, Enabled: true,
		Rule: &models.Rule{DeviceID: id, OpenAnchor: models.AnchorFixed, OpenFixedTime: tod(20, 0),
			CloseAnchor: models.AnchorFixed, CloseFixedTime: tod(8, 0)}}
	p, _ := PlanDay(coop, []models.Device{d}, time.Now())
	if len(p.Events) != 0 || p.Errors[id] == "" {
		t.Fatalf("fermeture avant ouverture acceptée")
	}
}

func TestDisabledOrMonitorDevicesHaveNoEvents(t *testing.T) {
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	off := mainDoor(uuid.New())
	off.Enabled = false
	feeder := models.Device{ID: uuid.New(), DeviceType: "Feeder", Role: models.RoleFeeder, Strategy: models.StrategyMonitor, Enabled: true}
	p, err := PlanDay(coop, []models.Device{off, feeder}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != 0 {
		t.Fatalf("%d événements pour des appareils non automatisés", len(p.Events))
	}
	if _, ok := p.Doors[off.ID]; !ok {
		t.Fatalf("une porte désactivée doit rester calculable comme référence")
	}
}

func TestErrorsFollowCoopLanguage(t *testing.T) {
	coop := &models.Coop{Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris", Language: "en"}
	id := uuid.New()
	d := models.Device{ID: id, DeviceType: "Autodoor", Strategy: models.StrategyCommand, Enabled: true,
		Rule: &models.Rule{DeviceID: id, OpenAnchor: models.AnchorFixed, OpenFixedTime: tod(20, 0),
			CloseAnchor: models.AnchorFixed, CloseFixedTime: tod(8, 0)}}
	p, _ := PlanDay(coop, []models.Device{d}, time.Now())
	if p.Codes[id] != CodeCloseBeforeOpen || !strings.Contains(p.Errors[id], "before opening") {
		t.Fatalf("erreur en anglais attendue : %q (%s)", p.Errors[id], p.Codes[id])
	}
}
