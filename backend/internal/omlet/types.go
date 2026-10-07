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

// OnBattery indique un appareil qui n'est pas sur secteur (piles, ou batterie « internal » des portes) :
// il se met en veille entre deux connexions et reçoit les commandes avec retard.
func (d *Device) OnBattery() bool { return d.State.General.PowerSource != "external" }
