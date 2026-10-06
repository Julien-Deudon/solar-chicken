package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
)

// GetRule : GET /devices/:id/rule
func (a *API) GetRule(c *gin.Context) {
	dev, _, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	if dev.Rule == nil {
		fail(c, http.StatusNotFound, tr(c, "Cet appareil n'a pas de règle", "This device has no rule"))
		return
	}
	c.JSON(http.StatusOK, dev.Rule)
}

// checkRule valide une règle candidate dans le contexte de son poulailler.
func (a *API) checkRule(c *gin.Context, coop *models.Coop, dev *models.Device, rule *models.Rule) (devices []models.Device, msg string) {
	rule.DeviceID = dev.ID
	if err := rule.ValidateLang(lang(c)); err != nil {
		return nil, err.Error()
	}
	if !dev.IsDoor() {
		return nil, tr(c, "seules les portes ont des horaires", "only doors have schedules")
	}
	for _, ref := range []*models.Moment{ptr(rule.Open()), ptr(rule.Close())} {
		if ref.RefDeviceID == nil || (ref.Anchor != models.AnchorDeviceOpen && ref.Anchor != models.AnchorDeviceClose) {
			continue
		}
		found := false
		for _, d := range coop.Devices {
			if d.ID == *ref.RefDeviceID && d.IsDoor() {
				found = true
			}
		}
		if !found {
			return nil, tr(c, "l'appareil de référence doit être une porte de ce poulailler", "the reference device must be a door of this coop")
		}
	}
	// Copie des appareils avec la règle candidate, puis contrôle sur 7 jours (cycles, fermeture avant ouverture)
	devices = make([]models.Device, len(coop.Devices))
	copy(devices, coop.Devices)
	for i := range devices {
		if devices[i].ID == dev.ID {
			r := *rule
			devices[i].Rule = &r
		}
	}
	loc, _ := coop.Location()
	now := time.Now().In(loc)
	for i := 0; i < 7; i++ {
		plan, err := planner.PlanDay(coop, devices, now.AddDate(0, 0, i))
		if err != nil {
			return nil, err.Error()
		}
		if plan.Codes[dev.ID] == planner.CodeCycle {
			return nil, tr(c, "référence circulaire entre appareils", "circular reference between devices")
		}
	}
	return devices, ""
}

func ptr(m models.Moment) *models.Moment { return &m }

// PutRule : PUT /devices/:id/rule — remplace la règle et recalcule les plannings.
func (a *API) PutRule(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Règle illisible : ", "Unreadable rule: ")+err.Error())
		return
	}
	if _, msg := a.checkRule(c, coop, dev, &rule); msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	if dev.Rule != nil {
		rule.ID, rule.CreatedAt = dev.Rule.ID, dev.Rule.CreatedAt
	}
	if err := a.DB.Save(&rule).Error; err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Enregistrement impossible", "Could not save"))
		return
	}
	a.Engine.Replan(c.Request.Context())
	c.JSON(http.StatusOK, rule)
}

type previewDay struct {
	Day    string         `json:"day"`
	Open   *time.Time     `json:"open,omitempty"`
	Close  *time.Time     `json:"close,omitempty"`
	Events []previewEvent `json:"events"`
	Error  string         `json:"error,omitempty"`
}

type previewEvent struct {
	Action models.EventAction `json:"action"`
	DueAt  time.Time          `json:"dueAt"`
}

// PreviewRule : POST /devices/:id/rule/preview — horaires des 7 prochains jours avec la règle proposée.
func (a *API) PreviewRule(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Règle illisible : ", "Unreadable rule: ")+err.Error())
		return
	}
	devices, msg := a.checkRule(c, coop, dev, &rule)
	if msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	loc, _ := coop.Location()
	now := time.Now().In(loc)
	out := []previewDay{}
	for i := 0; i < 7; i++ {
		plan, err := planner.PlanDay(coop, devices, now.AddDate(0, 0, i))
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		pd := previewDay{Day: plan.Day, Error: plan.Errors[dev.ID], Events: []previewEvent{}}
		if t, ok := plan.Doors[dev.ID]; ok {
			o, cl := t.Open, t.Close
			pd.Open, pd.Close = &o, &cl
		}
		for _, ev := range plan.Events {
			if ev.DeviceID == dev.ID {
				pd.Events = append(pd.Events, previewEvent{ev.Action, ev.DueAt})
			}
		}
		out = append(out, pd)
	}
	c.JSON(http.StatusOK, out)
}
