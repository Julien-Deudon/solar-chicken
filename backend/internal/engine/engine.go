// Package engine exécute les plannings : création des événements du jour,
// exécution unique et vérifiée, relances, rattrapage, horaires écrits dans les boîtiers.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
	"gorm.io/gorm"
)

// Mode d'exécution.
type Mode string

const (
	// ModeShadow : tout est calculé et journalisé, rien n'est envoyé aux portes ni sur Telegram.
	ModeShadow Mode = "shadow"
	// ModeLive : les actions sont réellement exécutées.
	ModeLive Mode = "live"
)

// Omlet est le sous-ensemble de l'API Omlet utilisé par le moteur.
type Omlet interface {
	ListDevices(ctx context.Context) ([]omlet.Device, error)
	GetDevice(ctx context.Context, deviceID string) (*omlet.Device, error)
	Action(ctx context.Context, deviceID, action string) error
	SetTimes(ctx context.Context, deviceID, openTime, closeTime, timezone string) error
}

// Notifier envoie les messages à l'utilisateur (Telegram).
type Notifier interface {
	Notify(ctx context.Context, userID uuid.UUID, kind NotifyKind, message string)
}

type NotifyKind string

const (
	NotifyOpen  NotifyKind = "open"
	NotifyClose NotifyKind = "close"
	NotifyLight NotifyKind = "light"
	NotifyError NotifyKind = "error"
	NotifyDaily NotifyKind = "daily"
	NotifyInfo  NotifyKind = "info"
)

// Réglages de temporisation (modifiables dans les tests).
type Timing struct {
	Tick            time.Duration // fréquence de la boucle
	CheckEvery      time.Duration // intervalle entre deux vérifications d'une porte
	CommandDeadline time.Duration // délai pour qu'une commande soit exécutée (porte sur secteur)
	BatteryExtra    time.Duration // délai supplémentaire si la porte est sur piles
	OnboardGrace    time.Duration // délai laissé au boîtier pour exécuter son horaire
	MaxAttempts     int           // envois maximum d'une même commande
	LightStaleAfter time.Duration // au-delà, une action lumière en retard est abandonnée
	PlanEvery       time.Duration // fréquence de recalcul des plannings et de synchro des boîtiers
	DailyReportHour int           // heure locale du planning quotidien
}

func DefaultTiming() Timing {
	return Timing{
		Tick:            15 * time.Second,
		CheckEvery:      45 * time.Second,
		CommandDeadline: 4 * time.Minute,
		BatteryExtra:    12 * time.Minute,
		OnboardGrace:    3 * time.Minute,
		MaxAttempts:     3,
		LightStaleAfter: 15 * time.Minute,
		PlanEvery:       10 * time.Minute,
		DailyReportHour: 6,
	}
}

// Engine orchestre l'exécution.
type Engine struct {
	DB       *gorm.DB
	Mode     Mode
	Clients  func(apiKey string) Omlet
	Notifier Notifier
	Now      func() time.Time
	Timing   Timing

	mu           sync.Mutex // une seule passe à la fois
	lastTick     atomic.Int64
	lastPlan     atomic.Int64
	syncFails    map[uuid.UUID]int
	shadowSynced map[uuid.UUID]string
	status       statusCache
	lastPoll     time.Time
}

func New(db *gorm.DB, mode Mode, clients func(string) Omlet, n Notifier) *Engine {
	return &Engine{DB: db, Mode: mode, Clients: clients, Notifier: n, Now: time.Now, Timing: DefaultTiming(), syncFails: map[uuid.UUID]int{}, shadowSynced: map[uuid.UUID]string{}}
}

// now retourne l'heure courante en UTC (stockage homogène en base).
func (e *Engine) now() time.Time { return e.Now().UTC() }

// LastTick retourne l'heure du dernier passage de la boucle (santé).
func (e *Engine) LastTick() time.Time { return time.Unix(0, e.lastTick.Load()) }

// LastPlan retourne l'heure du dernier calcul des plannings.
func (e *Engine) LastPlan() time.Time { return time.Unix(0, e.lastPlan.Load()) }

// Run boucle jusqu'à l'annulation du contexte.
func (e *Engine) Run(ctx context.Context) {
	log.Printf("⚙️  Moteur démarré en mode %s", e.Mode)
	e.Step(ctx)
	t := time.NewTicker(e.Timing.Tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("⚙️  Moteur arrêté")
			return
		case <-t.C:
			e.Step(ctx)
		}
	}
}

// Step effectue une passe complète. Une panique n'arrête jamais la boucle.
func (e *Engine) Step(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ Panique dans le moteur : %v", r)
		}
	}()
	now := e.now()
	if now.Sub(e.LastPlan()) >= e.Timing.PlanEvery {
		if err := e.EnsurePlans(ctx); err != nil {
			log.Printf("❌ Calcul des plannings : %v", err)
		} else {
			e.lastPlan.Store(now.UnixNano())
		}
		e.syncOnboard(ctx)
		e.dailyReports(ctx)
	}
	e.processDue(ctx)
	if e.now().Sub(e.lastPoll) >= statusPollEvery {
		e.lastPoll = e.now()
		e.pollStatuses(ctx)
	}
	e.lastTick.Store(e.now().UnixNano())
}

// Replan force le recalcul (après modification d'une règle ou d'un appareil).
func (e *Engine) Replan(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.EnsurePlans(ctx); err != nil {
		log.Printf("❌ Recalcul des plannings : %v", err)
		return
	}
	e.lastPlan.Store(e.now().UnixNano())
	e.syncOnboard(ctx)
}

func (e *Engine) loadCoops() ([]models.Coop, error) {
	var coops []models.Coop
	err := e.DB.Preload("Devices", func(db *gorm.DB) *gorm.DB { return db.Order("position, created_at") }).
		Preload("Devices.Rule").Find(&coops).Error
	return coops, err
}

// EnsurePlans crée ou met à jour les événements d'aujourd'hui et de demain pour chaque poulailler.
// Seuls les événements encore "pending" peuvent changer d'heure ; les autres sont figés.
func (e *Engine) EnsurePlans(ctx context.Context) error {
	coops, err := e.loadCoops()
	if err != nil {
		return err
	}
	for i := range coops {
		coop := &coops[i]
		loc, err := coop.Location()
		if err != nil {
			log.Printf("❌ Poulailler %s : fuseau invalide %q", coop.Name, coop.Timezone)
			continue
		}
		now := e.now().In(loc)
		for _, day := range []time.Time{now, now.AddDate(0, 0, 1)} {
			plan, err := planner.PlanDay(coop, coop.Devices, day)
			if err != nil {
				log.Printf("❌ Poulailler %s, %s : %v", coop.Name, planner.DayKey(day, loc), err)
				continue
			}
			for devID, msg := range plan.Errors {
				log.Printf("⚠️  Poulailler %s, %s, appareil %s : %s", coop.Name, plan.Day, devID, msg)
			}
			if err := e.upsertDay(coop, plan); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) upsertDay(coop *models.Coop, plan *planner.DayPlan) error {
	var existing []models.PlannedEvent
	if err := e.DB.Where("coop_id = ? AND day = ?", coop.ID, plan.Day).Find(&existing).Error; err != nil {
		return err
	}
	type key struct {
		dev uuid.UUID
		a   models.EventAction
	}
	byKey := map[key]*models.PlannedEvent{}
	for i := range existing {
		byKey[key{existing[i].DeviceID, existing[i].Action}] = &existing[i]
	}
	strategies := map[uuid.UUID]models.Strategy{}
	for _, d := range coop.Devices {
		strategies[d.ID] = d.Strategy
	}
	seen := map[key]bool{}
	for _, ev := range plan.Events {
		k := key{ev.DeviceID, ev.Action}
		seen[k] = true
		cur, ok := byKey[k]
		if !ok {
			pe := models.PlannedEvent{
				CoopID: coop.ID, DeviceID: ev.DeviceID, Day: plan.Day, Action: ev.Action,
				DueAt: ev.DueAt.UTC(), Status: models.StatusPending, Strategy: strategies[ev.DeviceID],
			}
			if err := e.DB.Create(&pe).Error; err != nil {
				return err
			}
			continue
		}
		if cur.Status == models.StatusPending && (!cur.DueAt.Equal(ev.DueAt) || cur.Strategy != strategies[ev.DeviceID]) {
			if err := e.DB.Model(cur).Updates(map[string]interface{}{"due_at": ev.DueAt.UTC(), "strategy": strategies[ev.DeviceID]}).Error; err != nil {
				return err
			}
		}
		if cur.Status == models.StatusSkipped && cur.Note == noteRuleChanged && cur.DueAt.After(e.now()) {
			// L'événement revient dans le planning (règle remodifiée)
			e.DB.Model(cur).Updates(map[string]interface{}{"status": models.StatusPending, "due_at": ev.DueAt.UTC(), "note": ""})
		}
	}
	for k, cur := range byKey {
		if !seen[k] && cur.Status == models.StatusPending {
			e.DB.Model(cur).Updates(map[string]interface{}{"status": models.StatusSkipped, "note": noteRuleChanged})
		}
	}
	return nil
}

const noteRuleChanged = "retiré du planning (règle ou appareil modifié)" // clé interne, aussi affichée en français

// processDue traite les événements arrivés à échéance et les vérifications en attente.
func (e *Engine) processDue(ctx context.Context) {
	now := e.now()
	var events []models.PlannedEvent
	err := e.DB.Where("(status = ? AND due_at <= ? AND (next_check_at IS NULL OR next_check_at <= ?)) OR (status = ? AND next_check_at <= ?)",
		models.StatusPending, now, now, models.StatusSent, now).
		Order("due_at").Find(&events).Error
	if err != nil {
		log.Printf("❌ Lecture des événements : %v", err)
		return
	}
	for i := range events {
		if ctx.Err() != nil {
			return
		}
		e.handle(ctx, &events[i])
	}
}

func (e *Engine) handle(ctx context.Context, ev *models.PlannedEvent) {
	var coop models.Coop
	if err := e.DB.First(&coop, "id = ?", ev.CoopID).Error; err != nil {
		e.finish(ev, models.StatusSkipped, "poulailler supprimé / coop removed", "")
		return
	}
	var dev models.Device
	if err := e.DB.First(&dev, "id = ?", ev.DeviceID).Error; err != nil {
		e.finish(ev, models.StatusSkipped, t(&coop, "appareil supprimé", "device removed"), "")
		return
	}
	if !dev.Enabled || dev.Strategy == models.StrategyMonitor {
		e.finish(ev, models.StatusSkipped, t(&coop, "automatisation désactivée pour cet appareil", "automation paused for this device"), "")
		return
	}
	if ev.Status == models.StatusPending {
		if reason := e.staleReason(&coop, ev); reason != "" {
			e.finish(ev, models.StatusSkipped, reason, "")
			return
		}
	}
	if e.Mode == ModeShadow {
		e.shadow(ctx, &coop, &dev, ev)
		return
	}
	if !ev.Action.IsDoor() {
		e.light(ctx, &coop, &dev, ev)
		return
	}
	if ev.Status == models.StatusPending {
		e.startDoor(ctx, &coop, &dev, ev)
	} else {
		e.verifyDoor(ctx, &coop, &dev, ev)
	}
}

// staleReason indique pourquoi un événement en retard ne doit plus être exécuté.
func (e *Engine) staleReason(coop *models.Coop, ev *models.PlannedEvent) string {
	now := e.now()
	if !ev.Action.IsDoor() {
		if now.Sub(ev.DueAt) > e.Timing.LightStaleAfter {
			return t(coop, "lumière abandonnée : trop tard", "light skipped: too late")
		}
		return ""
	}
	// Une action de porte est dépassée si l'action inverse suivante est déjà due.
	opposite := models.EventClose
	if ev.Action == models.EventClose {
		opposite = models.EventOpen
	}
	var later models.PlannedEvent
	err := e.DB.Where("device_id = ? AND action = ? AND due_at > ? AND due_at <= ?", ev.DeviceID, opposite, ev.DueAt, now).
		Order("due_at").First(&later).Error
	if err == nil {
		return fmt.Sprintf(t(coop, "dépassé par « %s » de %s", "superseded by the %s at %s"), label(coop, opposite), hhmm(coop, later.DueAt))
	}
	return ""
}

func (e *Engine) shadow(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent) {
	what := t(coop, "aurait envoyé « "+label(coop, ev.Action)+" »", "would have sent the "+label(coop, ev.Action))
	if ev.Strategy == models.StrategyOnboard && ev.Action.IsDoor() {
		what = t(coop, "aurait vérifié l'exécution par le boîtier", "would have checked the unit ran its schedule")
	}
	observed := ""
	if ev.Action.IsDoor() {
		if d, err := e.Clients(coop.OmletAPIKey).GetDevice(ctx, dev.OmletDeviceID); err == nil && d.OpenState() != "" {
			observed = t(coop, " ; "+partName(coop, dev)+" : ", "; "+partName(coop, dev)+": ") + d.OpenState()
		} else if err != nil {
			observed = t(coop, " ; lecture Omlet impossible : ", "; could not read Omlet: ") + err.Error()
		}
	}
	late := ""
	if d := e.now().Sub(ev.DueAt); d > time.Minute {
		late = fmt.Sprintf(t(coop, " (traité %s après l'heure)", " (handled %s late)"), d.Round(time.Second))
	}
	log.Printf("👻 OMBRE %s · %s · prévu %s · %s%s%s", coop.Name, dev.Name, inLoc(coop, ev.DueAt).Format("15:04:05"), what, late, observed)
	e.finish(ev, models.StatusShadow, what+late+observed, "")
}

func (e *Engine) light(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent) {
	err := e.Clients(coop.OmletAPIKey).Action(ctx, dev.OmletDeviceID, ev.Action.OmletCommand())
	e.logAction(dev, ev.Action.OmletCommand(), models.ActionTriggerAuto, err, "")
	if err != nil {
		ev.Attempts++
		if ev.Attempts < 2 {
			next := e.now().Add(e.Timing.CheckEvery)
			e.DB.Model(ev).Updates(map[string]interface{}{"attempts": ev.Attempts, "next_check_at": next, "last_error": err.Error()})
			return
		}
		e.finish(ev, models.StatusFailed, t(coop, "lumière : échec", "light: failed"), err.Error())
		e.notify(ctx, coop, NotifyError, fmt.Sprintf(t(coop, "⚠️ <b>%s</b> : lumière non commandée (%s)", "⚠️ <b>%s</b>: light command failed (%s)"), dev.Name, short(err)))
		return
	}
	e.finish(ev, models.StatusConfirmed, "", "")
	e.notify(ctx, coop, NotifyLight, fmt.Sprintf("💡 <b>%s</b>%s %s", dev.Name, t(coop, " :", ":"), label(coop, ev.Action)))
}

// startDoor : premier traitement d'une ouverture/fermeture.
func (e *Engine) startDoor(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent) {
	want := ev.Action.DesiredDoorState()
	catchup := e.now().Sub(ev.DueAt) > 2*time.Minute
	client := e.Clients(coop.OmletAPIKey)

	if ev.Strategy == models.StrategyOnboard {
		// Le boîtier exécute lui-même : on vérifie après le délai de grâce.
		next := ev.DueAt.Add(e.Timing.OnboardGrace)
		if next.Before(e.now()) {
			next = e.now()
		}
		sent := e.now()
		note := t(coop, "en attente de l'horaire du boîtier", "waiting for the unit's schedule")
		e.DB.Model(ev).Updates(map[string]interface{}{"status": models.StatusSent, "sent_at": sent, "next_check_at": next, "note": note})
		ev.Status, ev.SentAt, ev.NextCheckAt, ev.Note = models.StatusSent, &sent, &next, note
		return
	}

	d, err := client.GetDevice(ctx, dev.OmletDeviceID)
	if err != nil {
		if e.now().Sub(ev.DueAt) < e.Timing.CommandDeadline {
			log.Printf("⚠️  %s · %s : état illisible (%v), nouvelle lecture dans %s", coop.Name, dev.Name, err, e.Timing.CheckEvery)
			e.recheck(ev, err)
			return
		}
		// Lecture impossible depuis trop longtemps : la commande part quand même.
		e.send(ctx, coop, dev, ev, nil, catchup)
		return
	}
	if d.DoorIs(want) {
		e.finish(ev, models.StatusConfirmed, t(coop, "déjà "+doneLabel(coop, dev, ev.Action)+" : aucune commande envoyée", "already "+doneLabel(coop, dev, ev.Action)+": no command sent"), "")
		e.notifyDone(ctx, coop, dev, ev, catchup, true)
		return
	}
	if d.DoorMovingTo(want) {
		e.markSent(ev, t(coop, "déjà en mouvement", "already moving"), 0)
		return
	}
	e.send(ctx, coop, dev, ev, d, catchup)
}

func (e *Engine) send(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent, d *omlet.Device, catchup bool) {
	trigger := models.ActionTriggerAuto
	note := ""
	if catchup {
		trigger, note = models.ActionTriggerCatchup, t(coop, "rattrapage", "catch-up")
	}
	if ev.Strategy == models.StrategyOnboard {
		trigger, note = models.ActionTriggerFallback, t(coop, "le boîtier n'a pas exécuté son horaire", "the unit did not run its schedule")
	}
	err := e.Clients(coop.OmletAPIKey).Action(ctx, dev.OmletDeviceID, ev.Action.OmletCommand())
	e.logAction(dev, ev.Action.OmletCommand(), trigger, err, note)
	ev.Attempts++
	if err != nil {
		if ev.Attempts < e.Timing.MaxAttempts {
			e.DB.Model(ev).Update("attempts", ev.Attempts)
			e.recheck(ev, err)
			return
		}
		deadlines.Delete(ev.ID)
		e.finish(ev, models.StatusFailed, t(coop, "commande refusée", "command refused"), err.Error())
		e.notify(ctx, coop, NotifyError, fmt.Sprintf(t(coop, "🚨 <b>%s</b> : %s impossible (%s). Vérifie "+partArticle(coop, dev)+" !", "🚨 <b>%s</b>: %s failed (%s). Check the "+partName(coop, dev)+"!"), dev.Name, label(coop, ev.Action), short(err)))
		return
	}
	extra := time.Duration(0)
	if d != nil && d.OnBattery() {
		extra = e.Timing.BatteryExtra
	}
	e.markSent(ev, note, extra)
}

func (e *Engine) markSent(ev *models.PlannedEvent, note string, extra time.Duration) {
	now := e.now()
	next := now.Add(e.Timing.CheckEvery)
	deadline := now.Add(e.Timing.CommandDeadline + extra)
	e.DB.Model(ev).Updates(map[string]interface{}{
		"status": models.StatusSent, "sent_at": now, "next_check_at": next, "attempts": ev.Attempts,
		"note": note, "last_error": "",
	})
	ev.Status, ev.SentAt, ev.NextCheckAt, ev.Note = models.StatusSent, &now, &next, note
	deadlines.Store(ev.ID, deadline)
}

// deadlines garde l'échéance de vérification de chaque commande envoyée.
// Après un redémarrage, l'échéance est recalculée depuis sent_at.
var deadlines sync.Map

func (e *Engine) deadline(ev *models.PlannedEvent, d *omlet.Device) time.Time {
	if v, ok := deadlines.Load(ev.ID); ok {
		return v.(time.Time)
	}
	base := ev.DueAt
	if ev.SentAt != nil {
		base = *ev.SentAt
	}
	extra := time.Duration(0)
	if d != nil && d.OnBattery() {
		extra = e.Timing.BatteryExtra
	}
	return base.Add(e.Timing.CommandDeadline + extra)
}

// verifyDoor : contrôle d'une porte après commande (ou après l'heure du boîtier).
func (e *Engine) verifyDoor(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent) {
	want := ev.Action.DesiredDoorState()
	catchup := ev.SentAt != nil && ev.SentAt.Sub(ev.DueAt) > 2*time.Minute
	d, err := e.Clients(coop.OmletAPIKey).GetDevice(ctx, dev.OmletDeviceID)
	if err == nil && d.DoorIs(want) {
		deadlines.Delete(ev.ID)
		note := ev.Note
		if ev.Strategy == models.StrategyOnboard && ev.Attempts == 0 {
			note = t(coop, "fait par le boîtier", "done by the unit")
		}
		e.finish(ev, models.StatusConfirmed, note, "")
		e.notifyDone(ctx, coop, dev, ev, catchup, false)
		return
	}
	if err == nil && want == "closed" { // le défaut décrit la dernière tentative de fermeture
		if f := d.DoorFault(); f != "" {
			deadlines.Delete(ev.ID)
			e.finish(ev, models.StatusFailed, t(coop, "défaut "+partName(coop, dev)+" : ", partName(coop, dev)+" fault: ")+f, f)
			e.notify(ctx, coop, NotifyError, fmt.Sprintf(t(coop, "🚨 <b>%s</b> : %s impossible, défaut « %s ». Vérifie "+partArticle(coop, dev)+".", "🚨 <b>%s</b>: %s impossible, fault “%s”. Check the "+partName(coop, dev)+"."), dev.Name, label(coop, ev.Action), f))
			return
		}
	}
	// Appareil endormi sans nouvelles depuis l'heure prévue (ou l'envoi) : son état chez Omlet est ancien et
	// une commande attendrait sa prochaine connexion. On relit à ce moment-là plutôt que de conclure à un échec.
	if err == nil {
		if at := e.wakeWait(d, ev); !at.IsZero() {
			e.rescheduleAt(ev, at)
			return
		}
	}
	// Stratégie boîtier : premier contrôle raté → on envoie la commande nous-mêmes.
	if ev.Strategy == models.StrategyOnboard && ev.Attempts == 0 {
		if err == nil && d.DoorMovingTo(want) {
			e.reschedule(ev)
			return
		}
		if e.now().Before(ev.DueAt.Add(e.Timing.OnboardGrace + e.extraFor(d))) {
			e.reschedule(ev)
			return
		}
		e.notify(ctx, coop, NotifyInfo, fmt.Sprintf(t(coop, "⚠️ <b>%s</b> : le boîtier n'a pas exécuté « %s » de %s, commande envoyée par le serveur.",
			"⚠️ <b>%s</b>: the unit did not run the %s at %s, the server sent the command."), dev.Name, label(coop, ev.Action), hhmm(coop, ev.DueAt)))
		e.send(ctx, coop, dev, ev, d, false)
		return
	}
	if e.now().Before(e.deadline(ev, d)) {
		e.reschedule(ev)
		return
	}
	// Échéance dépassée
	if ev.Attempts < e.Timing.MaxAttempts {
		if err == nil && d.DoorMovingTo(want) {
			e.reschedule(ev)
			return
		}
		deadlines.Delete(ev.ID)
		e.send(ctx, coop, dev, ev, d, catchup)
		return
	}
	deadlines.Delete(ev.ID)
	reason := t(coop, "non "+doneLabel(coop, dev, ev.Action)+" après "+fmt.Sprint(ev.Attempts)+" envoi(s)", "not "+doneLabel(coop, dev, ev.Action)+" after "+fmt.Sprint(ev.Attempts)+" attempt(s)")
	if err != nil {
		reason = t(coop, "état illisible : ", "state unreadable: ") + short(err)
	} else if s := d.OpenState(); s != "" {
		reason += t(coop, " (état : ", " (state: ") + s + ")"
	}
	e.finish(ev, models.StatusFailed, reason, reason)
	e.notify(ctx, coop, NotifyError, fmt.Sprintf(t(coop, "🚨 <b>%s</b> : %s non confirmée, %s. Vérifie sur place !", "🚨 <b>%s</b>: %s not confirmed, %s. Check on site!"), dev.Name, label(coop, ev.Action), reason))
}

func (e *Engine) extraFor(d *omlet.Device) time.Duration {
	if d != nil && d.OnBattery() {
		return e.Timing.BatteryExtra
	}
	return 0
}

func (e *Engine) reschedule(ev *models.PlannedEvent) {
	e.rescheduleAt(ev, e.now().Add(e.Timing.CheckEvery))
}

func (e *Engine) rescheduleAt(ev *models.PlannedEvent, next time.Time) {
	e.DB.Model(ev).Update("next_check_at", next)
	ev.NextCheckAt = &next
}

const (
	wakeMargin  = 2 * time.Minute // relecture après la connexion annoncée d'un appareil endormi
	maxWakeWait = 10 * time.Hour  // au-delà, la prochaine connexion annoncée n'est pas crédible
)

// wakeWait retourne quand relire un appareil endormi qui ne s'est pas connecté depuis l'heure prévue de
// l'événement (ou depuis l'envoi d'une commande) ; zéro s'il n'y a pas lieu d'attendre : appareil connecté,
// connecté depuis, ou connexion annoncée déjà passée sans nouvelles (là, on applique les délais habituels).
func (e *Engine) wakeWait(d *omlet.Device, ev *models.PlannedEvent) time.Time {
	if !d.Asleep() {
		return time.Time{}
	}
	since := ev.DueAt
	if ev.Attempts > 0 && ev.SentAt != nil && ev.SentAt.After(since) {
		since = *ev.SentAt
	}
	if seen := d.LastSeen(); !seen.IsZero() && !seen.Before(since) {
		return time.Time{}
	}
	now := e.now()
	wake := d.WakesAt()
	if wake.IsZero() || wake.Sub(now) > maxWakeWait {
		return time.Time{}
	}
	if at := wake.Add(wakeMargin); at.After(now) {
		return at
	}
	return time.Time{}
}

// recheck reporte le traitement d'un événement (lecture ou commande en échec).
func (e *Engine) recheck(ev *models.PlannedEvent, err error) {
	next := e.now().Add(e.Timing.CheckEvery)
	e.DB.Model(ev).Updates(map[string]interface{}{"next_check_at": next, "last_error": err.Error()})
	ev.NextCheckAt, ev.LastError = &next, err.Error()
}

func (e *Engine) finish(ev *models.PlannedEvent, status models.EventStatus, note, lastErr string) {
	now := e.now()
	e.DB.Model(ev).Updates(map[string]interface{}{"status": status, "done_at": now, "note": note, "last_error": lastErr, "next_check_at": nil})
	ev.Status, ev.DoneAt, ev.Note, ev.LastError, ev.NextCheckAt = status, &now, note, lastErr, nil
}

func (e *Engine) notifyDone(ctx context.Context, coop *models.Coop, dev *models.Device, ev *models.PlannedEvent, catchup, already bool) {
	kind, icon := NotifyOpen, "🔓"
	if ev.Action == models.EventClose {
		kind, icon = NotifyClose, "🔒"
	}
	msg := fmt.Sprintf(t(coop, "%s <b>%s</b> %s à %s", "%s <b>%s</b> %s at %s"), icon, dev.Name, doneLabel(coop, dev, ev.Action), hhmm(coop, e.now()))
	if already {
		msg += t(coop, agree(dev, " (elle l'était déjà)", " (il l'était déjà)"), " (it already was)")
	}
	if catchup {
		msg += fmt.Sprintf(t(coop, "\n⏱ Rattrapage : prévu à %s", "\n⏱ Catch-up: scheduled at %s"), hhmm(coop, ev.DueAt))
		e.notify(ctx, coop, NotifyInfo, msg)
		return
	}
	e.notify(ctx, coop, kind, msg)
}

func (e *Engine) notify(ctx context.Context, coop *models.Coop, kind NotifyKind, msg string) {
	if e.Mode != ModeLive || e.Notifier == nil {
		return
	}
	e.Notifier.Notify(ctx, coop.UserID, kind, msg)
}

func (e *Engine) logAction(dev *models.Device, action string, trigger models.ActionTrigger, err error, note string) {
	entry := models.ActionLog{DeviceID: dev.ID, ActionType: action, TriggeredBy: trigger, Status: models.ActionStatusSuccess, Note: note, ExecutedAt: e.now()}
	if err != nil {
		entry.Status, entry.ErrorMessage = models.ActionStatusError, err.Error()
	}
	if dbErr := e.DB.Create(&entry).Error; dbErr != nil {
		log.Printf("❌ Journal d'action : %v", dbErr)
	}
	if err != nil {
		log.Printf("❌ %s · %s : %v", dev.Name, action, err)
	} else {
		log.Printf("✅ %s · %s (%s)", dev.Name, action, trigger)
	}
}

// ManualAction exécute une action demandée depuis l'interface.
func (e *Engine) ManualAction(ctx context.Context, coop *models.Coop, dev *models.Device, action string, userID uuid.UUID) error {
	var err error
	switch action {
	case "open", "close", "stop", "on", "off":
		err = e.Clients(coop.OmletAPIKey).Action(ctx, dev.OmletDeviceID, action)
	default:
		return errors.New("action inconnue")
	}
	entry := models.ActionLog{DeviceID: dev.ID, ActionType: action, TriggeredBy: models.ActionTriggerManual, UserID: &userID, Status: models.ActionStatusSuccess, ExecutedAt: e.now()}
	if err != nil {
		entry.Status, entry.ErrorMessage = models.ActionStatusError, err.Error()
	}
	e.DB.Create(&entry)
	return err
}

func inLoc(coop *models.Coop, t time.Time) time.Time {
	if loc, err := coop.Location(); err == nil {
		return t.In(loc)
	}
	return t
}

func hhmm(coop *models.Coop, t time.Time) string { return inLoc(coop, t).Format("15:04") }

// agree accorde un adjectif : « la porte » (féminin) ou « le pondoir » (masculin).
// partName : « porte » ou « mangeoire » (« door » / « feeder »).
func partName(coop *models.Coop, dev *models.Device) string {
	if dev.IsFeeder() {
		return t(coop, "mangeoire", "feeder")
	}
	return t(coop, "porte", "door")
}

// partArticle : « la porte » ou « la mangeoire ».
func partArticle(coop *models.Coop, dev *models.Device) string {
	return t(coop, "la ", "the ") + partName(coop, dev)
}

func agree(dev *models.Device, fem, masc string) string {
	if dev.Role == models.RoleNestBox {
		return masc
	}
	return fem
}

func doneLabel(coop *models.Coop, dev *models.Device, a models.EventAction) string {
	if a == models.EventOpen {
		return t(coop, agree(dev, "ouverte", "ouvert"), "opened")
	}
	return t(coop, agree(dev, "fermée", "fermé"), "closed")
}

// t choisit le texte dans la langue du poulailler.
func t(coop *models.Coop, fr, en string) string {
	if coop != nil && coop.Language == "en" {
		return en
	}
	return fr
}

func label(coop *models.Coop, a models.EventAction) string {
	switch a {
	case models.EventOpen:
		return t(coop, "ouverture", "opening")
	case models.EventClose:
		return t(coop, "fermeture", "closing")
	case models.EventLightBeforeOpen, models.EventLightBeforeClose:
		return t(coop, "lumière allumée", "light on")
	default:
		return t(coop, "lumière éteinte", "light off")
	}
}

func short(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}
