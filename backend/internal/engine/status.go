package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
)

// Snapshot est le dernier état connu d'un appareil (relu toutes les 5 minutes).
type Snapshot struct {
	Device    *omlet.Device
	FetchedAt time.Time
	Err       string
}

type statusCache struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]Snapshot
}

const statusPollEvery = 5 * time.Minute

// Snapshot retourne le dernier état connu d'un appareil.
func (e *Engine) Snapshot(deviceID uuid.UUID) (Snapshot, bool) {
	e.status.mu.RLock()
	defer e.status.mu.RUnlock()
	s, ok := e.status.byID[deviceID]
	return s, ok
}

// pollStatuses relit l'état de tous les appareils (un appel Omlet par poulailler).
func (e *Engine) pollStatuses(ctx context.Context) {
	coops, err := e.loadCoops()
	if err != nil {
		return
	}
	for i := range coops {
		coop := &coops[i]
		if len(coop.Devices) == 0 {
			continue
		}
		list, err := e.Clients(coop.OmletAPIKey).ListDevices(ctx)
		now := e.now()
		byOmlet := map[string]*omlet.Device{}
		for j := range list {
			byOmlet[list[j].DeviceID] = &list[j]
		}
		e.status.mu.Lock()
		if e.status.byID == nil {
			e.status.byID = map[uuid.UUID]Snapshot{}
		}
		for _, d := range coop.Devices {
			prev := e.status.byID[d.ID]
			switch {
			case err != nil:
				prev.Err = err.Error()
				e.status.byID[d.ID] = prev
			case byOmlet[d.OmletDeviceID] == nil:
				e.status.byID[d.ID] = Snapshot{FetchedAt: now, Err: "appareil absent du compte Omlet"}
			default:
				e.status.byID[d.ID] = Snapshot{Device: byOmlet[d.OmletDeviceID], FetchedAt: now}
			}
		}
		e.status.mu.Unlock()
		if err != nil {
			log.Printf("⚠️  État des appareils %s illisible : %v", coop.Name, err)
			continue
		}
		// Lampe branchée ou retirée depuis l'ajout de l'appareil : on suit ce que dit Omlet.
		for _, d := range coop.Devices {
			if od := byOmlet[d.OmletDeviceID]; od != nil && od.HasLight() != d.HasLight {
				e.DB.Model(&models.Device{}).Where("id = ?", d.ID).Update("has_light", od.HasLight())
				log.Printf("💡 %s · %s : lampe %s", coop.Name, d.Name, map[bool]string{true: "détectée", false: "absente"}[od.HasLight()])
			}
		}
	}
}
