package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/planner"
)

type coopView struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	OmletGroupID string    `json:"omletGroupId"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Timezone     string    `json:"timezone"`
	HasAPIKey    bool      `json:"hasApiKey"`
	DeviceCount  int       `json:"deviceCount"`
	Language     string    `json:"language"`
}

func toCoopView(c *models.Coop) coopView {
	return coopView{c.ID, c.Name, c.OmletGroupID, c.Latitude, c.Longitude, c.Timezone, c.OmletAPIKey != "", len(c.Devices), c.Language}
}

type dayView struct {
	Day     string                `json:"day"`
	Sunrise time.Time             `json:"sunrise"`
	Sunset  time.Time             `json:"sunset"`
	Events  []models.PlannedEvent `json:"events"`
	Errors  map[string]string     `json:"errors"`
}

func (a *API) dayView(coop *models.Coop, day time.Time) (*dayView, error) {
	plan, err := planner.PlanDay(coop, coop.Devices, day)
	if err != nil {
		return nil, err
	}
	v := &dayView{Day: plan.Day, Sunrise: plan.Sunrise, Sunset: plan.Sunset, Errors: map[string]string{}}
	for id, msg := range plan.Errors {
		v.Errors[id.String()] = msg
	}
	if err := a.DB.Where("coop_id = ? AND day = ? AND NOT (status = ? AND note = ?)", coop.ID, plan.Day, models.StatusSkipped, "retiré du planning (règle ou appareil modifié)").
		Order("due_at").Find(&v.Events).Error; err != nil {
		return nil, err
	}
	if len(v.Events) == 0 { // jour pas encore en base : aperçu calculé
		for _, ev := range plan.Events {
			v.Events = append(v.Events, models.PlannedEvent{CoopID: coop.ID, DeviceID: ev.DeviceID, Day: plan.Day, Action: ev.Action, DueAt: ev.DueAt, Status: models.StatusPending})
		}
	}
	return v, nil
}

// stateView est le dernier état connu d'un appareil (relu par le moteur toutes les 5 minutes).
type stateView struct {
	Door      string     `json:"door,omitempty"`
	Fault     string     `json:"fault,omitempty"`
	Light     string     `json:"light,omitempty"`
	Battery   int        `json:"battery"`
	Power     string     `json:"power,omitempty"`
	Connected bool       `json:"connected"`
	LastOpen  string     `json:"lastOpen,omitempty"`
	LastClose string     `json:"lastClose,omitempty"`
	FeedLevel *int       `json:"feedLevel,omitempty"`
	FetchedAt *time.Time `json:"fetchedAt,omitempty"`
	Error     string     `json:"error,omitempty"`
}

func (a *API) states(coop *models.Coop) map[string]stateView {
	out := map[string]stateView{}
	for _, d := range coop.Devices {
		snap, ok := a.Engine.Snapshot(d.ID)
		if !ok {
			continue
		}
		v := stateView{Error: snap.Err}
		if !snap.FetchedAt.IsZero() {
			t := snap.FetchedAt
			v.FetchedAt = &t
		}
		if od := snap.Device; od != nil {
			v.Battery, v.Power, v.Connected = od.State.General.BatteryLevel, od.State.General.PowerSource, od.State.Connectivity.Connected
			if od.State.Door != nil {
				v.Door, v.Fault, v.LastOpen, v.LastClose = od.State.Door.State, od.State.Door.Fault, od.State.Door.LastOpenTime, od.State.Door.LastCloseTime
			}
			if od.State.Feeder != nil {
				lvl := od.State.Feeder.FeedLevel
				v.Door, v.Fault, v.LastOpen, v.LastClose, v.FeedLevel = od.State.Feeder.State, od.State.Feeder.Fault, od.State.Feeder.LastOpenTime, od.State.Feeder.LastCloseTime, &lvl
			}
			if od.State.Light != nil {
				v.Light = od.State.Light.State
			}
		}
		out[d.ID.String()] = v
	}
	return out
}

// ListCoops : GET /coops
func (a *API) ListCoops(c *gin.Context) {
	var coops []models.Coop
	if err := preloadDevices(a.DB).Where("user_id = ?", a.user(c)).Order("created_at").Find(&coops).Error; err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Lecture des poulaillers impossible", "Could not read the coops"))
		return
	}
	out := make([]coopView, 0, len(coops))
	for i := range coops {
		out = append(out, toCoopView(&coops[i]))
	}
	c.JSON(http.StatusOK, out)
}

// GetCoop : GET /coops/:id — poulailler, appareils, planning d'aujourd'hui et de demain.
func (a *API) GetCoop(c *gin.Context) { a.respondCoop(c, http.StatusOK) }

// respondCoop renvoie le détail du poulailler c.Param("id") avec le code HTTP donné.
func (a *API) respondCoop(c *gin.Context, status int) {
	coop, ok := a.ownedCoop(c, c.Param("id"))
	if !ok {
		return
	}
	loc, err := coop.Location()
	if err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Fuseau horaire invalide", "Invalid time zone"))
		return
	}
	now := time.Now().In(loc)
	today, err1 := a.dayView(coop, now)
	tomorrow, err2 := a.dayView(coop, now.AddDate(0, 0, 1))
	if err1 != nil || err2 != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Calcul du planning impossible", "Could not compute the schedule"))
		return
	}
	c.JSON(status, gin.H{
		"coop": toCoopView(coop), "devices": coop.Devices,
		"today": today, "tomorrow": tomorrow,
		"mode": a.Engine.Mode, "states": a.states(coop),
	})
}

// GetPlan : GET /coops/:id/plan?day=AAAA-MM-JJ
func (a *API) GetPlan(c *gin.Context) {
	coop, ok := a.ownedCoop(c, c.Param("id"))
	if !ok {
		return
	}
	loc, _ := coop.Location()
	day, err := time.ParseInLocation("2006-01-02", c.Query("day"), loc)
	if err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Paramètre day attendu au format AAAA-MM-JJ", "The day parameter must be YYYY-MM-DD"))
		return
	}
	v, err := a.dayView(coop, day.Add(12*time.Hour))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, v)
}

type updateCoopRequest struct {
	Name        *string  `json:"name"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Timezone    *string  `json:"timezone"`
	OmletAPIKey *string  `json:"omletApiKey"`
	Language    *string  `json:"language"`
}

// applyCoopFields valide et applique les champs fournis ; retourne les colonnes modifiées.
func (a *API) applyCoopFields(c *gin.Context, coop *models.Coop, req updateCoopRequest, creating bool) ([]string, bool) {
	var cols []string
	if req.Name != nil || creating {
		name := ""
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
		}
		if name == "" {
			fail(c, http.StatusBadRequest, tr(c, "Le nom ne peut pas être vide", "The name cannot be empty"))
			return nil, false
		}
		coop.Name, cols = name, append(cols, "name")
	}
	if req.Latitude != nil {
		if *req.Latitude < -90 || *req.Latitude > 90 {
			fail(c, http.StatusBadRequest, tr(c, "Latitude invalide", "Invalid latitude"))
			return nil, false
		}
		coop.Latitude, cols = *req.Latitude, append(cols, "latitude")
	}
	if req.Longitude != nil {
		if *req.Longitude < -180 || *req.Longitude > 180 {
			fail(c, http.StatusBadRequest, tr(c, "Longitude invalide", "Invalid longitude"))
			return nil, false
		}
		coop.Longitude, cols = *req.Longitude, append(cols, "longitude")
	}
	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil || *req.Timezone == "" {
			fail(c, http.StatusBadRequest, tr(c, "Fuseau horaire inconnu", "Unknown time zone"))
			return nil, false
		}
		coop.Timezone, cols = *req.Timezone, append(cols, "timezone")
	}
	if req.Language != nil {
		if *req.Language != "fr" && *req.Language != "en" {
			fail(c, http.StatusBadRequest, tr(c, "Langue non prise en charge (fr ou en)", "Unsupported language (fr or en)"))
			return nil, false
		}
		coop.Language, cols = *req.Language, append(cols, "language")
	}
	if req.OmletAPIKey != nil && strings.TrimSpace(*req.OmletAPIKey) != "" {
		key := strings.TrimSpace(*req.OmletAPIKey)
		if _, err := a.Clients(key).ListDevices(c.Request.Context()); err != nil {
			fail(c, http.StatusBadRequest, tr(c, "Clé API Omlet refusée : ", "Omlet API key rejected: ")+err.Error())
			return nil, false
		}
		coop.OmletAPIKey, cols = key, append(cols, "omlet_api_key")
	}
	return cols, true
}

// CreateCoop : POST /coops — premier poulailler d'un compte (assistant de démarrage).
func (a *API) CreateCoop(c *gin.Context) {
	var req updateCoopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Requête invalide", "Invalid request"))
		return
	}
	if req.Latitude == nil || req.Longitude == nil || req.Timezone == nil || req.OmletAPIKey == nil || strings.TrimSpace(*req.OmletAPIKey) == "" {
		fail(c, http.StatusBadRequest, tr(c, "Nom, position, fuseau horaire et clé API Omlet sont obligatoires",
			"Name, location, time zone and Omlet API key are required"))
		return
	}
	coop := models.Coop{UserID: a.user(c), Language: lang(c)}
	if _, ok := a.applyCoopFields(c, &coop, req, true); !ok {
		return
	}
	if err := a.DB.Create(&coop).Error; err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Enregistrement impossible", "Could not save"))
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "id", Value: coop.ID.String()})
	a.respondCoop(c, http.StatusCreated)
}

// UpdateCoop : PUT /coops/:id
func (a *API) UpdateCoop(c *gin.Context) {
	coop, ok := a.ownedCoop(c, c.Param("id"))
	if !ok {
		return
	}
	var req updateCoopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Requête invalide", "Invalid request"))
		return
	}
	cols, ok := a.applyCoopFields(c, coop, req, false)
	if !ok {
		return
	}
	if len(cols) > 0 {
		if err := a.DB.Model(coop).Select(cols).Updates(coop).Error; err != nil {
			fail(c, http.StatusInternalServerError, tr(c, "Enregistrement impossible", "Could not save"))
			return
		}
		a.Engine.Replan(c.Request.Context())
	}
	a.GetCoop(c)
}

type omletDeviceView struct {
	DeviceID     string `json:"deviceId"`
	Name         string `json:"name"`
	DeviceType   string `json:"deviceType"`
	GroupID      string `json:"groupId"`
	SameGroup    bool   `json:"sameGroup"`
	AlreadyAdded bool   `json:"alreadyAdded"`
	PowerSource  string `json:"powerSource"`
	HasLight     bool   `json:"hasLight"`
	DoorState    string `json:"doorState,omitempty"`
}

// DiscoverDevices : GET /coops/:id/omlet-devices — appareils du compte Omlet.
func (a *API) DiscoverDevices(c *gin.Context) {
	coop, ok := a.ownedCoop(c, c.Param("id"))
	if !ok {
		return
	}
	list, err := a.Clients(coop.OmletAPIKey).ListDevices(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, tr(c, "API Omlet : ", "Omlet API: ")+err.Error())
		return
	}
	var added []string
	a.DB.Model(&models.Device{}).Pluck("omlet_device_id", &added)
	known := map[string]bool{}
	for _, id := range added {
		known[id] = true
	}
	out := make([]omletDeviceView, 0, len(list))
	for _, d := range list {
		v := omletDeviceView{
			DeviceID: d.DeviceID, Name: d.Name, DeviceType: d.DeviceType, GroupID: d.GroupID,
			SameGroup: coop.OmletGroupID == "" || d.GroupID == coop.OmletGroupID, AlreadyAdded: known[d.DeviceID],
			PowerSource: d.State.General.PowerSource, HasLight: d.State.Light != nil,
		}
		if d.State.Door != nil {
			v.DoorState = d.State.Door.State
		}
		out = append(out, v)
	}
	c.JSON(http.StatusOK, out)
}
