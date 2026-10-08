package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"gorm.io/gorm"
)

type addDeviceRequest struct {
	OmletDeviceID string            `json:"omletDeviceId" binding:"required"`
	Role          models.DeviceRole `json:"role" binding:"required"`
	Name          string            `json:"name"`
	Strategy      models.Strategy   `json:"strategy"`
}

// AddDevice : POST /coops/:id/devices — ajoute un appareil découvert sur le compte Omlet.
func (a *API) AddDevice(c *gin.Context) {
	coop, ok := a.ownedCoop(c, c.Param("id"))
	if !ok {
		return
	}
	var req addDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.Role.Valid() {
		fail(c, http.StatusBadRequest, tr(c, "Requête invalide (appareil et rôle obligatoires)", "Invalid request (device and role are required)"))
		return
	}
	var count int64
	a.DB.Model(&models.Device{}).Where("omlet_device_id = ?", req.OmletDeviceID).Count(&count)
	if count > 0 {
		fail(c, http.StatusConflict, tr(c, "Cet appareil est déjà ajouté", "This device is already added"))
		return
	}
	od, err := a.Clients(coop.OmletAPIKey).GetDevice(c.Request.Context(), req.OmletDeviceID)
	if err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Appareil introuvable sur le compte Omlet : ", "Device not found on the Omlet account: ")+err.Error())
		return
	}
	isDoor := od.DeviceType == "Autodoor"
	if (req.Role == models.RoleFeeder) == isDoor {
		fail(c, http.StatusBadRequest, tr(c, "Ce rôle ne correspond pas au type d'appareil (", "This role does not match the device type (")+od.DeviceType+")")
		return
	}
	if req.Role == models.RoleMainDoor {
		for _, d := range coop.Devices {
			if d.Role == models.RoleMainDoor {
				fail(c, http.StatusConflict, tr(c, "Ce poulailler a déjà une porte principale (", "This coop already has a main door (")+d.Name+")")
				return
			}
		}
	}
	strategy := req.Strategy
	if strategy == "" {
		strategy = models.DefaultStrategy(req.Role)
	}
	if !models.StrategyFits(od.DeviceType, strategy) {
		fail(c, http.StatusBadRequest, tr(c, "Stratégie invalide pour cet appareil", "Invalid mode for this device"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = od.Name
	}
	dev := models.Device{
		CoopID: coop.ID, OmletDeviceID: od.DeviceID, DeviceType: od.DeviceType, Role: req.Role, Name: name,
		Strategy: strategy, HasLight: od.HasLight(), Enabled: true, Position: len(coop.Devices),
	}
	var mainDoorID *uuid.UUID
	for i := range coop.Devices {
		if coop.Devices[i].Role == models.RoleMainDoor {
			mainDoorID = &coop.Devices[i].ID
		}
	}
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&dev).Error; err != nil {
			return err
		}
		rule := models.DefaultRule(dev.Role, dev.HasLight, mainDoorID)
		if rule != nil {
			rule.DeviceID = dev.ID
			if err := tx.Create(rule).Error; err != nil {
				return err
			}
		}
		if coop.OmletGroupID == "" && od.GroupID != "" {
			return tx.Model(coop).Update("omlet_group_id", od.GroupID).Error
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Ajout impossible : ", "Could not add the device: ")+err.Error())
		return
	}
	a.Engine.Replan(c.Request.Context())
	a.DB.Preload("Rule").First(&dev, "id = ?", dev.ID)
	c.JSON(http.StatusCreated, dev)
}

type updateDeviceRequest struct {
	Name     *string            `json:"name"`
	Role     *models.DeviceRole `json:"role"`
	Strategy *models.Strategy   `json:"strategy"`
	Enabled  *bool              `json:"enabled"`
	Position *int               `json:"position"`
}

// UpdateDevice : PUT /devices/:id
func (a *API) UpdateDevice(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	var req updateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, tr(c, "Requête invalide", "Invalid request"))
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Role != nil {
		if !req.Role.Valid() || (*req.Role == models.RoleFeeder) == dev.IsDoor() {
			fail(c, http.StatusBadRequest, tr(c, "Rôle invalide pour cet appareil", "Invalid role for this device"))
			return
		}
		if *req.Role == models.RoleMainDoor {
			for _, d := range coop.Devices {
				if d.Role == models.RoleMainDoor && d.ID != dev.ID {
					fail(c, http.StatusConflict, tr(c, "Ce poulailler a déjà une porte principale (", "This coop already has a main door (")+d.Name+")")
					return
				}
			}
		}
		updates["role"] = *req.Role
	}
	if req.Strategy != nil {
		if !models.StrategyFits(dev.DeviceType, *req.Strategy) {
			fail(c, http.StatusBadRequest, tr(c, "Stratégie invalide pour cet appareil", "Invalid mode for this device"))
			return
		}
		updates["strategy"] = *req.Strategy
		if *req.Strategy != models.StrategyOnboard {
			updates["onboard_synced_day"] = "" // forcera une réécriture si on y revient
		}
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Position != nil {
		updates["position"] = *req.Position
	}
	// Un appareil qui passe d'une simple surveillance à des horaires reçoit la règle proposée par défaut.
	var newRule *models.Rule
	if req.Strategy != nil && *req.Strategy != models.StrategyMonitor && dev.Rule == nil && dev.Opens() {
		var mainDoorID *uuid.UUID
		for i := range coop.Devices {
			if coop.Devices[i].Role == models.RoleMainDoor && coop.Devices[i].ID != dev.ID {
				mainDoorID = &coop.Devices[i].ID
			}
		}
		role := dev.Role
		if req.Role != nil {
			role = *req.Role
		}
		if newRule = models.DefaultRule(role, dev.HasLight, mainDoorID); newRule != nil {
			newRule.DeviceID = dev.ID
		}
	}
	if len(updates) > 0 || newRule != nil {
		err := a.DB.Transaction(func(tx *gorm.DB) error {
			if len(updates) > 0 {
				if err := tx.Model(&models.Device{}).Where("id = ?", dev.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			if newRule != nil {
				return tx.Create(newRule).Error
			}
			return nil
		})
		if err != nil {
			fail(c, http.StatusInternalServerError, tr(c, "Enregistrement impossible", "Could not save"))
			return
		}
		a.Engine.Replan(c.Request.Context())
	}
	var out models.Device
	a.DB.Preload("Rule").First(&out, "id = ?", dev.ID)
	c.JSON(http.StatusOK, out)
}

// DeleteDevice : DELETE /devices/:id
func (a *API) DeleteDevice(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	for _, d := range coop.Devices {
		if d.Rule == nil || d.ID == dev.ID {
			continue
		}
		if (d.Rule.OpenRefDeviceID != nil && *d.Rule.OpenRefDeviceID == dev.ID) || (d.Rule.CloseRefDeviceID != nil && *d.Rule.CloseRefDeviceID == dev.ID) {
			fail(c, http.StatusConflict, tr(c, "« "+d.Name+" » utilise cet appareil comme référence : modifie d'abord sa règle",
				"“"+d.Name+"” uses this device as its reference: change its rule first"))
			return
		}
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", dev.ID).Delete(&models.PlannedEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", dev.ID).Delete(&models.Rule{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Device{}, "id = ?", dev.ID).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, tr(c, "Suppression impossible", "Could not delete"))
		return
	}
	a.Engine.Replan(c.Request.Context())
	c.Status(http.StatusNoContent)
}

// DeviceStatus : GET /devices/:id/status — état en direct depuis Omlet.
func (a *API) DeviceStatus(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	od, err := a.Clients(coop.OmletAPIKey).GetDevice(c.Request.Context(), dev.OmletDeviceID)
	if err != nil {
		fail(c, http.StatusBadGateway, tr(c, "API Omlet : ", "Omlet API: ")+err.Error())
		return
	}
	var light *omlet.StateLight
	if od.HasLight() {
		light = od.State.Light
	}
	status := gin.H{
		"name": od.Name, "deviceType": od.DeviceType, "groupId": od.GroupID,
		"door": od.State.Door, "light": light, "feeder": od.State.Feeder,
		"batteryLevel": od.State.General.BatteryLevel, "powerSource": od.State.General.PowerSource,
		"firmware":     od.State.General.FirmwareVersionCurrent,
		"wifiStrength": od.State.Connectivity.WifiStrength, "connected": od.State.Connectivity.Connected,
		"asleep": od.Asleep(), "overdue": od.OverdueConnection > 0,
	}
	if wake := od.WakesAt(); !wake.IsZero() {
		status["nextWake"] = wake
	}
	if seen := od.LastSeen(); !seen.IsZero() && od.Asleep() {
		status["lastSeen"] = seen
	}
	c.JSON(http.StatusOK, status)
}

// DeviceAction : POST /devices/:id/actions/:action (open, close, stop, light_on, light_off)
func (a *API) DeviceAction(c *gin.Context) {
	dev, coop, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	action := map[string]string{"open": "open", "close": "close", "stop": "stop", "light_on": "on", "light_off": "off"}[c.Param("action")]
	if action == "" || (!dev.IsDoor() && action != "open" && action != "close") {
		fail(c, http.StatusBadRequest, tr(c, "Action inconnue", "Unknown action"))
		return
	}
	if err := a.Engine.ManualAction(c.Request.Context(), coop, dev, action, a.user(c)); err != nil {
		fail(c, http.StatusBadGateway, tr(c, "Action refusée par Omlet : ", "Omlet refused the command: ")+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Action envoyée"})
}

// DeviceLogs : GET /devices/:id/logs?limit=50
func (a *API) DeviceLogs(c *gin.Context) {
	dev, _, ok := a.ownedDevice(c, c.Param("id"))
	if !ok {
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}
	var logs []models.ActionLog
	a.DB.Where("device_id = ?", dev.ID).Order("executed_at DESC").Limit(limit).Find(&logs)
	c.JSON(http.StatusOK, logs)
}
