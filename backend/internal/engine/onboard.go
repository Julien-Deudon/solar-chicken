package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
)

// syncOnboard écrit les horaires du jour dans le boîtier des appareils en stratégie "onboard".
// Le boîtier les exécute seul (mode horaire), même si le serveur ou internet tombe :
// au pire il garde les horaires de la veille, à 1 ou 2 minutes près.
func (e *Engine) syncOnboard(ctx context.Context) {
	coops, err := e.loadCoops()
	if err != nil {
		log.Printf("❌ Synchro boîtiers : %v", err)
		return
	}
	for i := range coops {
		coop := &coops[i]
		loc, err := coop.Location()
		if err != nil {
			continue
		}
		today := planner.DayKey(e.now(), loc)
		for j := range coop.Devices {
			dev := &coop.Devices[j]
			if dev.Strategy != models.StrategyOnboard || !dev.Enabled || !dev.IsDoor() {
				continue
			}
			var evs []models.PlannedEvent
			e.DB.Where("device_id = ? AND day = ? AND action IN ?", dev.ID, today, []models.EventAction{models.EventOpen, models.EventClose}).Find(&evs)
			var openHM, closeHM string
			for _, ev := range evs {
				if ev.Status == models.StatusSkipped && ev.Note == noteRuleChanged {
					continue
				}
				if ev.Action == models.EventOpen {
					openHM = hhmm(coop, ev.DueAt)
				} else {
					closeHM = hhmm(coop, ev.DueAt)
				}
			}
			if openHM == "" || closeHM == "" {
				continue // règle invalide aujourd'hui : on ne touche pas au boîtier
			}
			if dev.OnboardSyncedDay == today && dev.OnboardOpenTime == openHM && dev.OnboardCloseTime == closeHM && dev.OnboardSyncError == "" {
				continue
			}
			if e.Mode != ModeLive {
				key := today + openHM + closeHM
				if e.shadowSynced[dev.ID] != key {
					e.shadowSynced[dev.ID] = key
					log.Printf("👻 OMBRE %s · %s · écrirait dans le boîtier : ouverture %s, fermeture %s", coop.Name, dev.Name, openHM, closeHM)
				}
				continue
			}
			err := e.Clients(coop.OmletAPIKey).SetDoorTimes(ctx, dev.OmletDeviceID, openHM, closeHM)
			e.logAction(dev, "configuration", models.ActionTriggerSync, err, fmt.Sprintf(t(coop, "horaires %s / %s", "times %s / %s"), openHM, closeHM))
			if err != nil {
				e.syncFails[dev.ID]++
				e.DB.Model(dev).Update("onboard_sync_error", err.Error())
				if e.syncFails[dev.ID] == 3 {
					e.notify(ctx, coop, NotifyError, fmt.Sprintf(t(coop, "⚠️ <b>%s</b> : impossible d'écrire les horaires du jour dans le boîtier (%s). Il garde les précédents ; le serveur vérifiera quand même.",
						"⚠️ <b>%s</b>: could not write today's times to the unit (%s). It keeps the previous ones; the server will still check."), dev.Name, short(err)))
				}
				continue
			}
			e.syncFails[dev.ID] = 0
			now := e.now()
			e.DB.Model(dev).Updates(map[string]interface{}{
				"onboard_synced_day": today, "onboard_open_time": openHM, "onboard_close_time": closeHM,
				"onboard_synced_at": now, "onboard_sync_error": "",
			})
		}
	}
}
