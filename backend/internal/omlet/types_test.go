package omlet

import (
	"encoding/json"
	"testing"
)

// Réponse réelle de GET /device (octobre 2026, noms et identifiants remplacés) : porte sur secteur avec lampe,
// porte sur piles internes sans lampe (equipped = 1, mais un état de lampe quand même), mangeoire endormie.
// La force du WiFi est passée en texte pour une porte, comme l'annonce la documentation d'Omlet.
const realDevices = `[
 {"deviceId": "main", "name": "Main door", "deviceType": "Autodoor", "groupId": "g", "lastConnected": "2026-10-02T11:15:21+00:00",
  "nextConnection": null, "overdueConnection": 0,
  "state": {"light": {"state": "on"}, "general": {"firmwareVersionCurrent": "1.0.55", "batteryLevel": 98, "powerSource": "external"},
   "connectivity": {"wifiStrength": -42, "ssid": "x", "connected": true},
   "door": {"state": "open", "lastOpenTime": "2026-10-08T07:48:09+02:00", "lastCloseTime": "2026-10-07T19:34:18+02:00", "fault": "none", "lightLevel": 1}},
  "configuration": {"general": {"timezone": "Europe/Paris", "pollFreq": 600}, "light": {"mode": "auto", "equipped": 2}}},
 {"deviceId": "nest", "name": "Nest box", "deviceType": "Autodoor", "groupId": "g", "lastConnected": "2026-10-08T05:48:17.000000Z",
  "nextConnection": "2026-10-08T05:58:17.000000Z", "overdueConnection": 0,
  "state": {"light": {"state": "off"}, "general": {"batteryLevel": 100, "powerSource": "internal"},
   "connectivity": {"wifiStrength": "-34", "connected": false},
   "door": {"state": "open", "lastOpenTime": "2026-10-08T07:48:14+02:00", "lastCloseTime": "2026-10-07T18:55:50+02:00", "fault": "none", "lightLevel": 0}},
  "configuration": {"general": {"timezone": "Europe/Paris"}, "light": {"mode": "auto", "equipped": 1}}},
 {"deviceId": "feeder", "name": "Feeder", "deviceType": "Feeder", "groupId": "g", "lastConnected": "2026-10-08T05:48:58+00:00",
  "nextConnection": "2026-10-08T06:48:58+00:00", "overdueConnection": 0, "remainingDays": 7,
  "state": {"general": {"batteryLevel": 100, "powerSource": "battery"}, "connectivity": {"wifiStrength": -52, "connected": false},
   "feeder": {"state": "open", "lastOpenTime": "2026-10-08T07:48:10+02:00", "fault": "none", "feedLevel": 59, "lightLevel": 0, "mode": "time"}},
  "configuration": {"general": {"timezone": "Europe/Paris"}, "feeder": {"mode": "time", "openTime1": "07:48", "closeTime1": "19:12"}}}
]`

func TestDecodeRealDevices(t *testing.T) {
	var devices []Device
	if err := json.Unmarshal([]byte(realDevices), &devices); err != nil {
		t.Fatalf("décodage : %v", err)
	}
	main, nest, feeder := devices[0], devices[1], devices[2]
	if !main.HasLight() || nest.HasLight() || feeder.HasLight() {
		t.Errorf("lampes : porte %v, pondoir %v, mangeoire %v (attendu true, false, false)", main.HasLight(), nest.HasLight(), feeder.HasLight())
	}
	if main.OnBattery() || !nest.OnBattery() || !feeder.OnBattery() {
		t.Errorf("piles : porte %v, pondoir (internal) %v, mangeoire %v", main.OnBattery(), nest.OnBattery(), feeder.OnBattery())
	}
	if main.Asleep() || !nest.Asleep() || !main.WakesAt().IsZero() {
		t.Errorf("veille : porte %v (réveil %v), pondoir %v", main.Asleep(), main.WakesAt(), nest.Asleep())
	}
	if w := nest.WakesAt(); w.Format("15:04") != "05:58" || nest.LastSeen().Format("15:04") != "05:48" {
		t.Errorf("connexions du pondoir : réveil %v, vu %v", w, nest.LastSeen())
	}
	if nest.State.Connectivity.WifiStrength != -34 || main.State.General.BatteryLevel != 98 || feeder.State.Feeder.FeedLevel != 59 {
		t.Errorf("valeurs numériques : wifi %d, batterie %d, grain %d", nest.State.Connectivity.WifiStrength, main.State.General.BatteryLevel, feeder.State.Feeder.FeedLevel)
	}
	if !feeder.DoorIs("open") || feeder.OpenState() != "open" || !main.DoorIs("open") || main.Timezone() != "Europe/Paris" {
		t.Errorf("états : mangeoire %q, porte %v, fuseau %q", feeder.OpenState(), main.DoorIs("open"), main.Timezone())
	}
}

func TestIntAcceptsNumbersAndText(t *testing.T) {
	for in, want := range map[string]Int{`-60`: -60, `"-60"`: -60, `"0"`: 0, `""`: 0, `"notPresent"`: 0, `null`: 0, `83.6`: 84} {
		var n Int
		if err := json.Unmarshal([]byte(in), &n); err != nil || n != want {
			t.Errorf("%s → %d (%v), attendu %d", in, n, err, want)
		}
	}
}
