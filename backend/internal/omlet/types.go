package omlet

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Int lit un entier envoyé en nombre ou en texte : la documentation d'Omlet annonce du texte ("-60")
// pour des champs que l'API renvoie aujourd'hui en nombre. Une valeur non numérique ("notPresent") vaut 0.
type Int int

func (n *Int) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	*n = 0
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*n = Int(math.Round(v))
	}
	return nil
}

// Device représente un appareil Omlet (porte, mangeoire...).
type Device struct {
	DeviceID      string         `json:"deviceId"`
	Name          string         `json:"name"`
	DeviceType    string         `json:"deviceType"` // "Autodoor", "Feeder"
	GroupID       string         `json:"groupId"`    // coop de l'application Omlet
	State         DeviceState    `json:"state"`
	Configuration *Configuration `json:"configuration,omitempty"`
	Actions       []Action       `json:"actions"`
	// Appareils sur piles : ils dorment entre deux connexions (pollFreq) et toute la nuit
	// (overnightSleep) ; l'état lu chez Omlet est celui de leur dernière connexion.
	LastConnected     string `json:"lastConnected"`
	NextConnection    string `json:"nextConnection"`    // null pour un appareil sur secteur, toujours connecté
	OverdueConnection Int    `json:"overdueConnection"` // > 0 : l'appareil a manqué sa connexion prévue
}

// Configuration : seuls les réglages utiles à l'appli.
type Configuration struct {
	General *ConfigGeneral `json:"general,omitempty"`
	Light   *ConfigLight   `json:"light,omitempty"`
}

type ConfigGeneral struct {
	Timezone string `json:"timezone"` // ex. "Europe/Paris" : les horaires du boîtier sont dans ce fuseau
}

type ConfigLight struct {
	Equipped Int `json:"equipped"` // 2 : lampe branchée ; 1 : aucune lampe détectée
}

// DeviceState contient l'état actuel de l'appareil.
type DeviceState struct {
	General      StateGeneral      `json:"general"`
	Connectivity StateConnectivity `json:"connectivity"`
	Door         *StateDoor        `json:"door,omitempty"`
	Light        *StateLight       `json:"light,omitempty"`
	Feeder       *StateFeeder      `json:"feeder,omitempty"`
}

type StateGeneral struct {
	FirmwareVersionCurrent string `json:"firmwareVersionCurrent"`
	BatteryLevel           Int    `json:"batteryLevel"`
	PowerSource            string `json:"powerSource"` // external (secteur), internal (piles d'une porte), battery (mangeoire)
}

type StateConnectivity struct {
	WifiStrength Int  `json:"wifiStrength"` // dBm, 0 si non connecté
	Connected    bool `json:"connected"`
}

// StateDoor : state vaut open, closed, opening, closing, openpending, closepending,
// stopping, stopped, faulted, calibrating ou unknown selon le firmware.
type StateDoor struct {
	State         string `json:"state"`
	LastOpenTime  string `json:"lastOpenTime"`
	LastCloseTime string `json:"lastCloseTime"`
	Fault         string `json:"fault"` // none, blocked, crush, wiring : défaut de la dernière tentative de fermeture
	LightLevel    Int    `json:"lightLevel"`
}

type StateLight struct {
	State string `json:"state"` // on, off
}

type StateFeeder struct {
	State         string `json:"state"`
	LastOpenTime  string `json:"lastOpenTime"`
	LastCloseTime string `json:"lastCloseTime"`
	Fault         string `json:"fault"`
	FeedLevel     Int    `json:"feedLevel"`
}

// Action représente une action disponible sur l'appareil.
type Action struct {
	ActionName   string `json:"actionName"`
	Description  string `json:"description"`
	PendingValue string `json:"pendingValue"`
}

// ErrorResponse représente une erreur de l'API.
type ErrorResponse struct {
	Message string `json:"message"`
}

// mechanism retourne l'état et le défaut de la partie mobile : la porte, ou le couvercle d'une mangeoire.
func (d *Device) mechanism() (state, fault string, ok bool) {
	switch {
	case d.State.Door != nil:
		return d.State.Door.State, d.State.Door.Fault, true
	case d.State.Feeder != nil:
		return d.State.Feeder.State, d.State.Feeder.Fault, true
	}
	return "", "", false
}

// OpenState retourne l'état de la porte ou du couvercle de la mangeoire ("open", "closed", "opening"…).
func (d *Device) OpenState() string {
	state, _, _ := d.mechanism()
	return state
}

// DoorIs indique si la porte (ou le couvercle de la mangeoire) est dans l'état voulu ("open" ou "closed").
func (d *Device) DoorIs(want string) bool {
	state, _, ok := d.mechanism()
	return ok && state == want
}

// DoorMovingTo indique si la porte (ou le couvercle) est en train d'aller vers l'état voulu.
func (d *Device) DoorMovingTo(want string) bool {
	state, _, ok := d.mechanism()
	if !ok {
		return false
	}
	switch want {
	case "open":
		return state == "opening" || state == "openpending"
	case "closed":
		return state == "closing" || state == "closepending"
	}
	return false
}

// DoorFault retourne le défaut de la porte ou du couvercle ("" si aucun).
func (d *Device) DoorFault() string {
	state, fault, ok := d.mechanism()
	if !ok {
		return ""
	}
	if fault != "" && fault != "none" {
		return fault
	}
	if state == "faulted" {
		return "faulted"
	}
	return ""
}

// HasLight indique qu'une lampe est branchée. Omlet renvoie un état de lampe même sans lampe :
// c'est configuration.light.equipped qui le dit (2 = lampe, 1 = aucune).
func (d *Device) HasLight() bool {
	if d.State.Light == nil {
		return false
	}
	if c := d.Configuration; c != nil && c.Light != nil && c.Light.Equipped != 0 {
		return c.Light.Equipped >= 2
	}
	return true
}

// Asleep indique un appareil endormi (sur piles, entre deux connexions) : son état chez Omlet date de
// sa dernière connexion et les commandes attendent la suivante.
func (d *Device) Asleep() bool { return !d.State.Connectivity.Connected && d.OnBattery() }

// LastSeen retourne la dernière connexion de l'appareil (zéro si inconnue).
func (d *Device) LastSeen() time.Time { return parseTime(d.LastConnected) }

// WakesAt retourne la prochaine connexion prévue d'un appareil endormi (zéro si inconnue ou s'il est connecté).
func (d *Device) WakesAt() time.Time {
	if !d.Asleep() {
		return time.Time{}
	}
	return parseTime(d.NextConnection)
}

// Timezone retourne le fuseau réglé dans l'appareil ("" si inconnu).
func (d *Device) Timezone() string {
	if c := d.Configuration; c != nil && c.General != nil {
		return c.General.Timezone
	}
	return ""
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// OnBattery indique un appareil qui n'est pas sur secteur (piles, ou batterie « internal » des portes) :
// il se met en veille entre deux connexions et reçoit les commandes avec retard.
func (d *Device) OnBattery() bool { return d.State.General.PowerSource != "external" }
