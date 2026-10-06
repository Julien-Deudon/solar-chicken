package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- faux Omlet ----------

type fakeOmlet struct {
	mu       sync.Mutex
	devices  map[string]*omlet.Device
	behavior map[string]string // obey (défaut), ignore, fault
	actions  []string
	setTimes []string
	getErr   error
}

func newFakeOmlet() *fakeOmlet {
	return &fakeOmlet{devices: map[string]*omlet.Device{}, behavior: map[string]string{}}
}

func (f *fakeOmlet) add(id, state string, light bool) {
	d := &omlet.Device{DeviceID: id, DeviceType: "Autodoor", State: omlet.DeviceState{
		General: omlet.StateGeneral{PowerSource: "external"},
		Door:    &omlet.StateDoor{State: state, Fault: "none"},
	}}
	if light {
		d.State.Light = &omlet.StateLight{State: "off"}
	}
	f.devices[id] = d
}

func (f *fakeOmlet) door(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.devices[id].State.Door.State
}

func (f *fakeOmlet) setDoor(id, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.devices[id].State.Door.State = state
}

func (f *fakeOmlet) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, a := range f.actions {
		if strings.HasPrefix(a, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeOmlet) ListDevices(ctx context.Context) ([]omlet.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []omlet.Device
	for _, d := range f.devices {
		out = append(out, *d)
	}
	return out, nil
}

func (f *fakeOmlet) GetDevice(ctx context.Context, id string) (*omlet.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	d := *f.devices[id]
	door := *d.State.Door
	d.State.Door = &door
	return &d, nil
}

func (f *fakeOmlet) Action(ctx context.Context, id, action string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, id+":"+action)
	switch f.behavior[id] {
	case "ignore":
	case "fault":
		f.devices[id].State.Door.Fault = "blocked"
	default:
		switch action {
		case "open":
			f.devices[id].State.Door.State = "open"
		case "close":
			f.devices[id].State.Door.State = "closed"
		}
	}
	return nil
}

func (f *fakeOmlet) SetDoorTimes(ctx context.Context, id, open, close string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setTimes = append(f.setTimes, fmt.Sprintf("%s:%s-%s", id, open, close))
	return nil
}

type fakeNotifier struct {
	mu   sync.Mutex
	msgs []string
}

func (n *fakeNotifier) Notify(ctx context.Context, userID uuid.UUID, kind NotifyKind, m string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.msgs = append(n.msgs, string(kind)+"|"+m)
}

func (n *fakeNotifier) with(sub string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := 0
	for _, m := range n.msgs {
		if strings.Contains(m, sub) {
			c++
		}
	}
	return c
}

// ---------- banc d'essai ----------

type bench struct {
	t     *testing.T
	e     *Engine
	om    *fakeOmlet
	notif *fakeNotifier
	clock time.Time
	coop  models.Coop
	main  models.Device
	nest  models.Device
	paris *time.Location
}

func tod(h, m int) *models.TimeOfDay { return &models.TimeOfDay{Hour: h, Minute: m} }

func newBench(t *testing.T, mode Mode, withNest bool) *bench {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.User{}, &models.Coop{}, &models.Device{}, &models.Rule{}, &models.PlannedEvent{}, &models.ActionLog{}, &models.NotificationSettings{}); err != nil {
		t.Fatal(err)
	}
	paris, _ := time.LoadLocation("Europe/Paris")
	b := &bench{t: t, om: newFakeOmlet(), notif: &fakeNotifier{}, paris: paris,
		clock: time.Date(2026, 10, 7, 7, 0, 0, 0, paris)}
	user := models.User{Email: "owner@example.org", PasswordHash: "x"}
	db.Create(&user)
	b.coop = models.Coop{UserID: user.ID, Name: "Poulailler", OmletAPIKey: "k", Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	db.Create(&b.coop)
	b.main = models.Device{CoopID: b.coop.ID, OmletDeviceID: "M1", DeviceType: "Autodoor", Role: models.RoleMainDoor, Name: "Porte principale",
		Strategy: models.StrategyCommand, HasLight: true, Enabled: true}
	db.Create(&b.main)
	db.Create(&models.Rule{DeviceID: b.main.ID, OpenAnchor: models.AnchorSunrise, OpenOffsetMinutes: -10,
		CloseAnchor: models.AnchorSunset, CloseOffsetMinutes: 20, LightBeforeOpenMinutes: 2, EnableLightMorning: true, EnableLightEvening: true})
	b.om.add("M1", "closed", true)
	if withNest {
		b.nest = models.Device{CoopID: b.coop.ID, OmletDeviceID: "N1", DeviceType: "Autodoor", Role: models.RoleNestBox, Name: "Pondoir",
			Strategy: models.StrategyOnboard, Enabled: true, Position: 1}
		db.Create(&b.nest)
		db.Create(&models.Rule{DeviceID: b.nest.ID, OpenAnchor: models.AnchorDeviceOpen, OpenOffsetMinutes: 30, OpenRefDeviceID: &b.main.ID,
			OpenNotBefore: tod(8, 0), CloseAnchor: models.AnchorSunset, CloseOffsetMinutes: -60})
		b.om.add("N1", "closed", false)
	}
	db.Create(&models.NotificationSettings{UserID: user.ID, NotifyOnOpen: true, NotifyOnClose: true, NotifyOnLight: true, NotifyOnError: true, NotifyDailySchedule: true})
	b.e = New(db, mode, func(string) Omlet { return b.om }, b.notif)
	b.e.Now = func() time.Time { return b.clock }
	return b
}

// run fait avancer l'horloge par pas de 15 s jusqu'à `until` (heure locale du jour simulé).
func (b *bench) run(h, m int) {
	until := time.Date(b.clock.Year(), b.clock.Month(), b.clock.Day(), h, m, 0, 0, b.paris)
	for !b.clock.After(until) {
		b.e.Step(context.Background())
		b.clock = b.clock.Add(15 * time.Second)
	}
}

func (b *bench) jump(h, m int) {
	b.clock = time.Date(b.clock.Year(), b.clock.Month(), b.clock.Day(), h, m, 0, 0, b.paris)
}

func (b *bench) event(dev models.Device, a models.EventAction) models.PlannedEvent {
	b.t.Helper()
	var ev models.PlannedEvent
	day := planner.DayKey(b.clock, b.paris)
	if err := b.e.DB.Where("device_id = ? AND action = ? AND day = ?", dev.ID, a, day).First(&ev).Error; err != nil {
		b.t.Fatalf("événement %s/%s introuvable : %v", dev.Name, a, err)
	}
	return ev
}

// ---------- tests ----------

func TestOneCommandPerEventAndConfirmation(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.run(20, 30)
	if n := b.om.count("M1:open"); n != 1 {
		t.Fatalf("ouverture envoyée %d fois, attendu 1 (%v)", n, b.om.actions)
	}
	if n := b.om.count("M1:close"); n != 1 {
		t.Fatalf("fermeture envoyée %d fois, attendu 1", n)
	}
	if n := b.om.count("M1:on"); n != 1 {
		t.Fatalf("lumière envoyée %d fois, attendu 1", n)
	}
	for _, a := range []models.EventAction{models.EventOpen, models.EventClose, models.EventLightBeforeOpen} {
		if ev := b.event(b.main, a); ev.Status != models.StatusConfirmed {
			t.Fatalf("%s : statut %s", a, ev.Status)
		}
	}
	open := b.event(b.main, models.EventOpen)
	if open.SentAt.Before(open.DueAt) {
		t.Fatalf("commande partie en avance : %s < %s", open.SentAt, open.DueAt)
	}
	if open.SentAt.Sub(open.DueAt) > 20*time.Second {
		t.Fatalf("commande partie trop tard : %s", open.SentAt.Sub(open.DueAt))
	}
	if b.notif.with("ouverte à") != 1 || b.notif.with("fermée à") != 1 {
		t.Fatalf("notifications inattendues : %v", b.notif.msgs)
	}
}

func TestAlreadyInDesiredStateSendsNothing(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.om.setDoor("M1", "open")
	b.run(9, 0)
	if n := b.om.count("M1:open"); n != 0 {
		t.Fatalf("commande envoyée alors que la porte était ouverte (%d)", n)
	}
	if ev := b.event(b.main, models.EventOpen); ev.Status != models.StatusConfirmed || !strings.Contains(ev.Note, "déjà") {
		t.Fatalf("statut %s, note %q", ev.Status, ev.Note)
	}
	if b.notif.with("l'était déjà") != 1 {
		t.Fatalf("notification attendue : %v", b.notif.msgs)
	}
}

func TestCatchUpAfterOutage(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.e.Step(context.Background()) // 07:00 : planning créé
	b.jump(8, 30)                  // serveur indisponible pendant l'ouverture
	b.run(8, 35)
	if n := b.om.count("M1:open"); n != 1 {
		t.Fatalf("rattrapage : %d ouverture(s)", n)
	}
	if b.om.count("M1:on") != 0 {
		t.Fatalf("la lumière d'avant ouverture ne doit pas être rattrapée")
	}
	if ev := b.event(b.main, models.EventLightBeforeOpen); ev.Status != models.StatusSkipped {
		t.Fatalf("lumière : statut %s", ev.Status)
	}
	if b.notif.with("Rattrapage") != 1 {
		t.Fatalf("notification de rattrapage absente : %v", b.notif.msgs)
	}
}

func TestOpenSupersededByCloseIsSkipped(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.e.Step(context.Background())
	b.jump(21, 0) // serveur revenu après le coucher
	b.run(21, 5)
	if n := b.om.count("M1:open"); n != 0 {
		t.Fatalf("ouverture envoyée à 21h : %d", n)
	}
	if ev := b.event(b.main, models.EventOpen); ev.Status != models.StatusSkipped || !strings.Contains(ev.Note, "dépassé") {
		t.Fatalf("ouverture : statut %s note %q", ev.Status, ev.Note)
	}
	if ev := b.event(b.main, models.EventClose); ev.Status != models.StatusConfirmed {
		t.Fatalf("fermeture : statut %s", ev.Status)
	}
}

func TestRetriesThenAlertsWhenDoorDoesNotMove(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.om.behavior["M1"] = "ignore"
	b.run(8, 30)
	if n := b.om.count("M1:open"); n != b.e.Timing.MaxAttempts {
		t.Fatalf("%d envois, attendu %d", n, b.e.Timing.MaxAttempts)
	}
	if ev := b.event(b.main, models.EventOpen); ev.Status != models.StatusFailed {
		t.Fatalf("statut %s", ev.Status)
	}
	if b.notif.with("non confirmée") != 1 {
		t.Fatalf("alerte attendue : %v", b.notif.msgs)
	}
}

func TestFaultIsReportedImmediately(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.om.behavior["M1"] = "fault"
	b.run(8, 30)
	if n := b.om.count("M1:open"); n != 1 {
		t.Fatalf("%d envois malgré le défaut", n)
	}
	if b.notif.with("blocked") != 1 {
		t.Fatalf("alerte défaut attendue : %v", b.notif.msgs)
	}
}

func TestNestBoxOnboardScheduleExecutedByBox(t *testing.T) {
	b := newBench(t, ModeLive, true)
	b.e.Step(context.Background())
	if len(b.om.setTimes) != 1 || !strings.HasPrefix(b.om.setTimes[0], "N1:08:") {
		t.Fatalf("horaires du boîtier : %v", b.om.setTimes)
	}
	nestOpen := b.event(b.nest, models.EventOpen)
	due := nestOpen.DueAt.In(b.paris)
	b.run(due.Hour(), due.Minute()) // jusqu'à l'heure prévue
	b.om.setDoor("N1", "open")      // le boîtier exécute son horaire
	b.run(due.Hour()+1, 0)
	if n := b.om.count("N1:"); n != 0 {
		t.Fatalf("le serveur a commandé le pondoir alors que le boîtier l'a fait : %v", b.om.actions)
	}
	if ev := b.event(b.nest, models.EventOpen); ev.Status != models.StatusConfirmed {
		t.Fatalf("pondoir : statut %s", ev.Status)
	}
	if b.notif.with("Pondoir</b> ouvert à") != 1 {
		t.Fatalf("notification pondoir attendue : %v", b.notif.msgs)
	}
	if len(b.om.setTimes) != 1 {
		t.Fatalf("horaires réécrits sans raison : %v", b.om.setTimes)
	}
}

func TestNestBoxFallbackWhenBoxDoesNothing(t *testing.T) {
	b := newBench(t, ModeLive, true)
	b.run(9, 30)
	if n := b.om.count("N1:open"); n != 1 {
		t.Fatalf("commande de secours : %d", n)
	}
	if b.notif.with("n'a pas exécuté") != 1 {
		t.Fatalf("avertissement attendu : %v", b.notif.msgs)
	}
	if ev := b.event(b.nest, models.EventOpen); ev.Status != models.StatusConfirmed {
		t.Fatalf("statut %s", ev.Status)
	}
}

func TestShadowModeSendsNothing(t *testing.T) {
	b := newBench(t, ModeShadow, true)
	b.run(21, 0)
	if len(b.om.actions) != 0 || len(b.om.setTimes) != 0 {
		t.Fatalf("mode ombre : envois %v, horaires %v", b.om.actions, b.om.setTimes)
	}
	if len(b.notif.msgs) != 0 {
		t.Fatalf("mode ombre : notifications %v", b.notif.msgs)
	}
	for _, a := range []models.EventAction{models.EventOpen, models.EventClose} {
		if ev := b.event(b.main, a); ev.Status != models.StatusShadow {
			t.Fatalf("%s : statut %s", a, ev.Status)
		}
	}
}

func TestPlansAreIdempotentAndFollowRuleChanges(t *testing.T) {
	b := newBench(t, ModeLive, true)
	ctx := context.Background()
	b.e.Step(ctx) // planning + première écriture des horaires du pondoir
	var n1, n2 int64
	b.e.DB.Model(&models.PlannedEvent{}).Count(&n1)
	b.e.EnsurePlans(ctx)
	b.e.DB.Model(&models.PlannedEvent{}).Count(&n2)
	if n1 != n2 || n1 == 0 {
		t.Fatalf("planning non idempotent : %d puis %d", n1, n2)
	}
	before := b.event(b.main, models.EventOpen).DueAt
	nestBefore := b.event(b.nest, models.EventOpen).DueAt
	b.e.DB.Model(&models.Rule{}).Where("device_id = ?", b.main.ID).
		Updates(map[string]interface{}{"open_offset_minutes": -40, "light_before_open_minutes": 0})
	b.e.Replan(ctx)
	after := b.event(b.main, models.EventOpen).DueAt
	if d := before.Sub(after); d != 30*time.Minute {
		t.Fatalf("ouverture décalée de %s, attendu 30 min", d)
	}
	if nestAfter := b.event(b.nest, models.EventOpen).DueAt; !nestAfter.Before(nestBefore) && nestBefore.In(b.paris).Hour() > 8 {
		t.Fatalf("le pondoir doit suivre la porte principale")
	}
	if ev := b.event(b.main, models.EventLightBeforeOpen); ev.Status != models.StatusSkipped {
		t.Fatalf("lumière supprimée : statut %s", ev.Status)
	}
	if len(b.om.setTimes) != 2 || !strings.HasPrefix(b.om.setTimes[1], "N1:08:00-") {
		t.Fatalf("les nouveaux horaires du pondoir doivent être réécrits une fois (borne 08:00) : %v", b.om.setTimes)
	}
}

func TestDailyReportOncePerDay(t *testing.T) {
	b := newBench(t, ModeLive, true)
	b.jump(5, 40)
	b.run(6, 40)
	if n := b.notif.with("Planning du jour"); n != 1 {
		t.Fatalf("%d planning(s) envoyés", n)
	}
	if b.notif.with("Pondoir") == 0 || b.notif.with("Lever") == 0 {
		t.Fatalf("contenu du planning : %v", b.notif.msgs)
	}
}

func TestNotificationsInEnglish(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.e.DB.Model(&models.Coop{}).Where("id = ?", b.coop.ID).Update("language", "en")
	b.jump(5, 40)
	b.run(8, 30)
	if b.notif.with("opened at") != 1 || b.notif.with("Today's schedule") != 1 {
		t.Fatalf("notifications en anglais attendues : %v", b.notif.msgs)
	}
	if b.notif.with("ouverte") != 0 {
		t.Fatalf("texte français dans un poulailler anglais : %v", b.notif.msgs)
	}
}
