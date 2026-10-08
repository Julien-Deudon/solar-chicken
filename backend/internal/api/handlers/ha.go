package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
)

type haDevice struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	Strategy  string     `json:"strategy"`
	Enabled   bool       `json:"enabled"`
	Door      string     `json:"door"`
	Fault     string     `json:"fault"`
	Light     string     `json:"light,omitempty"`
	Battery   int        `json:"battery"`
	Power     string     `json:"power"`
	Connected bool       `json:"connected"`
	LastOpen  string     `json:"lastOpen,omitempty"`
	LastClose string     `json:"lastClose,omitempty"`
	NextOpen  *time.Time `json:"nextOpen"`
	NextClose *time.Time `json:"nextClose"`
	LastEvent string     `json:"lastEvent"`
	Alert     bool       `json:"alert"`
	AlertText string     `json:"alertText,omitempty"`
	StatusAge int        `json:"statusAgeSeconds"`
}

// HAState : GET /ha/state — état en lecture seule pour Home Assistant (jeton HA_TOKEN).
// Retourne le premier poulailler, avec un appareil par rôle en accès direct (main_door, nest_box, feeder).
func (a *API) HAState(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var coop models.Coop
		if err := preloadDevices(a.DB).Order("created_at").First(&coop).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"mode": a.Engine.Mode, "devices": []haDevice{}})
			return
		}
		now := time.Now()
		loc, _ := coop.Location()
		out := gin.H{"mode": a.Engine.Mode, "coop": coop.Name}
		if plan, err := planner.PlanDay(&coop, coop.Devices, now.In(loc)); err == nil {
			out["sunrise"], out["sunset"] = plan.Sunrise, plan.Sunset
		}
		devices := []haDevice{}
		for _, d := range coop.Devices {
			hd := haDevice{ID: d.ID.String(), Name: d.Name, Role: string(d.Role), Strategy: string(d.Strategy), Enabled: d.Enabled, Door: "inconnu", Fault: "none"}
			if snap, ok := a.Engine.Snapshot(d.ID); ok {
				hd.StatusAge = int(now.Sub(snap.FetchedAt).Seconds())
				if od := snap.Device; od != nil {
					hd.Battery, hd.Power, hd.Connected = int(od.State.General.BatteryLevel), od.State.General.PowerSource, od.State.Connectivity.Connected
					if od.State.Door != nil {
						hd.Door, hd.Fault, hd.LastOpen, hd.LastClose = od.State.Door.State, od.State.Door.Fault, od.State.Door.LastOpenTime, od.State.Door.LastCloseTime
					}
					if od.State.Feeder != nil {
						hd.Door, hd.Fault, hd.LastOpen, hd.LastClose = od.State.Feeder.State, od.State.Feeder.Fault, od.State.Feeder.LastOpenTime, od.State.Feeder.LastCloseTime
					}
					if od.HasLight() {
						hd.Light = od.State.Light.State
					}
				}
				if snap.Err != "" && hd.StatusAge > 900 {
					hd.Alert, hd.AlertText = true, "état illisible : "+snap.Err
				}
			}
			if hd.Fault != "" && hd.Fault != "none" {
				hd.Alert, hd.AlertText = true, "défaut : "+hd.Fault
			}
			if d.OnboardSyncError != "" {
				hd.Alert, hd.AlertText = true, "horaires du boîtier non écrits : "+d.OnboardSyncError
			}
			for _, act := range []models.EventAction{models.EventOpen, models.EventClose} {
				var ev models.PlannedEvent
				if err := a.DB.Where("device_id = ? AND action = ? AND status = ? AND due_at > ?", d.ID, act, models.StatusPending, now).
					Order("due_at").First(&ev).Error; err == nil {
					t := ev.DueAt.In(loc)
					if act == models.EventOpen {
						hd.NextOpen = &t
					} else {
						hd.NextClose = &t
					}
				}
			}
			var last models.PlannedEvent
			if err := a.DB.Where("device_id = ? AND action IN ? AND status IN ?", d.ID,
				[]models.EventAction{models.EventOpen, models.EventClose},
				[]models.EventStatus{models.StatusConfirmed, models.StatusFailed, models.StatusShadow}).
				Order("due_at DESC").First(&last).Error; err == nil {
				hd.LastEvent = string(last.Action) + " " + string(last.Status) + " " + last.DueAt.In(loc).Format("02/01 15:04")
				if last.Status == models.StatusFailed {
					hd.Alert, hd.AlertText = true, "échec : "+last.Note
				}
			}
			devices = append(devices, hd)
			if _, exists := out[string(d.Role)]; !exists {
				out[string(d.Role)] = hd
			}
		}
		out["devices"] = devices
		alert := false
		for _, d := range devices {
			alert = alert || d.Alert
		}
		out["alert"] = alert
		c.JSON(http.StatusOK, out)
	}
}
