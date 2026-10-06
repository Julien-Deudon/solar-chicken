package omlet

// Device représente un appareil Omlet (porte, mangeoire...).
type Device struct {
	DeviceID   string      `json:"deviceId"`
	Name       string      `json:"name"`
	DeviceType string      `json:"deviceType"` // "Autodoor", "Feeder"
	GroupID    string      `json:"groupId"`    // coop de l'application Omlet
	State      DeviceState `json:"state"`
	Actions    []Action    `json:"actions"`
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
	BatteryLevel           int    `json:"batteryLevel"`
	PowerSource            string `json:"powerSource"` // external, battery
}

type StateConnectivity struct {
	WifiStrength int  `json:"wifiStrength"`
	Connected    bool `json:"connected"`
}

// StateDoor : state vaut open, closed, opening, closing, openpending, closepending,
// stopping, stopped, faulted, calibrating ou unknown selon le firmware.
type StateDoor struct {
	State         string `json:"state"`
	LastOpenTime  string `json:"lastOpenTime"`
	LastCloseTime string `json:"lastCloseTime"`
	Fault         string `json:"fault"` // none, blocked, crush, wiring
	LightLevel    int    `json:"lightLevel"`
}

type StateLight struct {
	State string `json:"state"` // on, off
}

type StateFeeder struct {
	State         string `json:"state"`
	LastOpenTime  string `json:"lastOpenTime"`
	LastCloseTime string `json:"lastCloseTime"`
	Fault         string `json:"fault"`
	FeedLevel     int    `json:"feedLevel"`
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

// DoorIs indique si la porte est dans l'état voulu ("open" ou "closed").
func (d *Device) DoorIs(want string) bool {
	return d.State.Door != nil && d.State.Door.State == want
}

// DoorMovingTo indique si la porte est en train d'aller vers l'état voulu.
func (d *Device) DoorMovingTo(want string) bool {
	if d.State.Door == nil {
		return false
	}
	switch want {
	case "open":
		return d.State.Door.State == "opening" || d.State.Door.State == "openpending"
	case "closed":
		return d.State.Door.State == "closing" || d.State.Door.State == "closepending"
	}
	return false
}

// DoorFault retourne le défaut de la porte ("" si aucun).
func (d *Device) DoorFault() string {
	if d.State.Door == nil {
		return ""
	}
	if f := d.State.Door.Fault; f != "" && f != "none" {
		return f
	}
	if d.State.Door.State == "faulted" {
		return "faulted"
	}
	return ""
}

// OnBattery indique un appareil sur piles (les commandes peuvent arriver avec retard).
func (d *Device) OnBattery() bool { return d.State.General.PowerSource == "battery" }
