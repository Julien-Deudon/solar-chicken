package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/handlers"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/middleware"
	"github.com/julien-deudon/solar-chicken/backend/internal/db"
	"github.com/julien-deudon/solar-chicken/backend/internal/engine"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const secret = "test-secret"

type fakeOmlet struct {
	mu      sync.Mutex
	devices []omlet.Device
	actions []string
}

func (f *fakeOmlet) ListDevices(ctx context.Context) ([]omlet.Device, error) { return f.devices, nil }
func (f *fakeOmlet) GetDevice(ctx context.Context, id string) (*omlet.Device, error) {
	for i := range f.devices {
		if f.devices[i].DeviceID == id {
			d := f.devices[i]
			return &d, nil
		}
	}
	return nil, fmt.Errorf("introuvable")
}
func (f *fakeOmlet) Action(ctx context.Context, id, action string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, id+":"+action)
	return nil
}
func (f *fakeOmlet) SetDoorTimes(ctx context.Context, id, o, c string) error { return nil }

type env struct {
	t      *testing.T
	router *gin.Engine
	db     *gorm.DB
	om     *fakeOmlet
	token  string
	coop   models.Coop
	main   models.Device
}

func door(id, group string, light bool) omlet.Device {
	d := omlet.Device{DeviceID: id, Name: "Omlet " + id, DeviceType: "Autodoor", GroupID: group,
		State: omlet.DeviceState{General: omlet.StateGeneral{PowerSource: "external"}, Door: &omlet.StateDoor{State: "closed", Fault: "none"}}}
	if light {
		d.State.Light = &omlet.StateLight{State: "off"}
	}
	return d
}

func newEnv(t *testing.T) *env {
	gin.SetMode(gin.TestMode)
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := gdb.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	om := &fakeOmlet{devices: []omlet.Device{door("M1", "G", true), door("N1", "G", false),
		{DeviceID: "F1", Name: "Mangeoire", DeviceType: "Feeder", GroupID: "G"}}}
	clients := func(string) engine.Omlet { return om }
	eng := engine.New(gdb, engine.ModeShadow, clients, nil)
	a := &handlers.API{DB: gdb, Engine: eng, Clients: clients, Version: "test", Started: time.Now()}
	e := &env{t: t, db: gdb, om: om, router: SetupRouter(a, Options{JWTSecret: secret})}

	user := models.User{Email: "j@example.org", PasswordHash: "x"}
	gdb.Create(&user)
	e.coop = models.Coop{UserID: user.ID, Name: "Poulailler", OmletAPIKey: "k", OmletGroupID: "G", Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	gdb.Create(&e.coop)
	e.main = models.Device{CoopID: e.coop.ID, OmletDeviceID: "M1", DeviceType: "Autodoor", Role: models.RoleMainDoor, Name: "Porte principale",
		Strategy: models.StrategyCommand, HasLight: true, Enabled: true}
	gdb.Create(&e.main)
	r := models.DefaultRule(models.RoleMainDoor, true, nil)
	r.DeviceID = e.main.ID
	gdb.Create(r)
	e.token = tokenFor(t, user.ID)
	return e
}

func tokenFor(t *testing.T, id uuid.UUID) string {
	claims := middleware.JWTClaims{UserID: id, Email: "x@example.org", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) do(method, path string, body interface{}, token string) (int, map[string]interface{}, []interface{}) {
	e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	var obj map[string]interface{}
	var arr []interface{}
	raw := w.Body.Bytes()
	if len(raw) > 0 && raw[0] == '[' {
		json.Unmarshal(raw, &arr)
	} else {
		json.Unmarshal(raw, &obj)
	}
	return w.Code, obj, arr
}

func TestAuthAndRegistration(t *testing.T) {
	e := newEnv(t)
	if code, _, _ := e.do("GET", "/api/v2/coops", nil, ""); code != http.StatusUnauthorized {
		t.Fatalf("sans jeton : %d", code)
	}
	if code, _, _ := e.do("POST", "/api/v2/auth/register", map[string]string{"email": "a@b.fr", "password": "12345678", "firstName": "A", "lastName": "B"}, ""); code != http.StatusForbidden {
		t.Fatalf("inscription : %d, attendu 403", code)
	}
	other := tokenFor(t, uuid.New())
	if code, _, _ := e.do("GET", "/api/v2/coops/"+e.coop.ID.String(), nil, other); code != http.StatusNotFound {
		t.Fatalf("poulailler d'un autre utilisateur : %d", code)
	}
}

func TestCoopDetailHasTodayPlan(t *testing.T) {
	e := newEnv(t)
	code, _, list := e.do("GET", "/api/v2/coops", nil, e.token)
	if code != 200 || len(list) != 1 {
		t.Fatalf("liste : %d %v", code, list)
	}
	code, obj, _ := e.do("GET", "/api/v2/coops/"+e.coop.ID.String(), nil, e.token)
	if code != 200 || obj["mode"] != "shadow" {
		t.Fatalf("détail : %d %v", code, obj["mode"])
	}
	today := obj["today"].(map[string]interface{})
	if evs := today["events"].([]interface{}); len(evs) != 3 {
		t.Fatalf("événements du jour : %d, attendu 3", len(evs))
	}
	if obj["coop"].(map[string]interface{})["hasApiKey"] != true {
		t.Fatalf("hasApiKey attendu, et la clé ne doit jamais sortir")
	}
}

func TestDiscoverAndAddNestBox(t *testing.T) {
	e := newEnv(t)
	cid := e.coop.ID.String()
	_, _, found := e.do("GET", "/api/v2/coops/"+cid+"/omlet-devices", nil, e.token)
	added := map[string]bool{}
	for _, it := range found {
		m := it.(map[string]interface{})
		added[m["deviceId"].(string)] = m["alreadyAdded"].(bool)
	}
	if !added["M1"] || added["N1"] || len(found) != 3 {
		t.Fatalf("découverte : %v", found)
	}
	code, dev, _ := e.do("POST", "/api/v2/coops/"+cid+"/devices", map[string]string{"omletDeviceId": "N1", "role": "nest_box", "name": "Pondoir"}, e.token)
	if code != http.StatusCreated || dev["strategy"] != "onboard" || dev["hasLight"] != false {
		t.Fatalf("ajout pondoir : %d %v", code, dev)
	}
	rule := dev["rule"].(map[string]interface{})
	// Par défaut : s'ouvre avec la porte principale, se ferme 2 h avant elle.
	if rule["openAnchor"] != "device_open" || rule["openRefDeviceId"] != e.main.ID.String() || rule["openOffsetMinutes"].(float64) != 0 ||
		rule["openNotBefore"] != nil || rule["closeAnchor"] != "device_close" || rule["closeRefDeviceId"] != e.main.ID.String() ||
		rule["closeOffsetMinutes"].(float64) != -120 {
		t.Fatalf("règle par défaut du pondoir : %v", rule)
	}
	if code, _, _ := e.do("POST", "/api/v2/coops/"+cid+"/devices", map[string]string{"omletDeviceId": "N1", "role": "nest_box"}, e.token); code != http.StatusConflict {
		t.Fatalf("doublon : %d, attendu 409", code)
	}
	if code, _, _ := e.do("POST", "/api/v2/coops/"+cid+"/devices", map[string]string{"omletDeviceId": "F1", "role": "main_door"}, e.token); code != http.StatusBadRequest {
		t.Fatalf("mangeoire en porte principale : %d, attendu 400", code)
	}
	code, feeder, _ := e.do("POST", "/api/v2/coops/"+cid+"/devices", map[string]string{"omletDeviceId": "F1", "role": "feeder"}, e.token)
	if code != http.StatusCreated || feeder["strategy"] != "monitor" || feeder["rule"] != nil {
		t.Fatalf("mangeoire : %d %v", code, feeder)
	}
	// Le planning du jour contient maintenant le pondoir
	_, obj, _ := e.do("GET", "/api/v2/coops/"+cid, nil, e.token)
	n := 0
	for _, ev := range obj["today"].(map[string]interface{})["events"].([]interface{}) {
		if ev.(map[string]interface{})["deviceId"] == dev["id"] {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("pondoir : %d événements aujourd'hui, attendu 2", n)
	}
}

func TestRuleValidationPreviewAndDelete(t *testing.T) {
	e := newEnv(t)
	cid := e.coop.ID.String()
	_, nest, _ := e.do("POST", "/api/v2/coops/"+cid+"/devices", map[string]string{"omletDeviceId": "N1", "role": "nest_box"}, e.token)
	nestID := nest["id"].(string)
	mainPath := "/api/v2/devices/" + e.main.ID.String()

	self := map[string]interface{}{"openAnchor": "device_open", "openOffsetMinutes": 0, "openRefDeviceId": e.main.ID.String(),
		"closeAnchor": "sunset", "closeOffsetMinutes": 0}
	if code, obj, _ := e.do("PUT", mainPath+"/rule", self, e.token); code != 400 || !strings.Contains(obj["error"].(string), "lui-même") {
		t.Fatalf("auto-référence : %d %v", code, obj)
	}
	cycle := map[string]interface{}{"openAnchor": "device_open", "openOffsetMinutes": -30, "openRefDeviceId": nestID,
		"closeAnchor": "sunset", "closeOffsetMinutes": 20}
	if code, obj, _ := e.do("PUT", mainPath+"/rule", cycle, e.token); code != 400 || !strings.Contains(obj["error"].(string), "circulaire") {
		t.Fatalf("cycle : %d %v", code, obj)
	}
	draft := map[string]interface{}{"openAnchor": "sunrise", "openOffsetMinutes": -15, "closeAnchor": "sunset", "closeOffsetMinutes": 30,
		"lightBeforeOpenMinutes": 2, "enableLightMorning": true}
	code, _, days := e.do("POST", mainPath+"/rule/preview", draft, e.token)
	if code != 200 || len(days) != 7 || days[0].(map[string]interface{})["open"] == nil {
		t.Fatalf("aperçu : %d %v", code, days)
	}
	if code, obj, _ := e.do("PUT", mainPath+"/rule", draft, e.token); code != 200 || obj["openOffsetMinutes"].(float64) != -15 {
		t.Fatalf("enregistrement : %d %v", code, obj)
	}
	if code, _, _ := e.do("DELETE", mainPath, nil, e.token); code != http.StatusConflict {
		t.Fatalf("suppression d'une porte référencée : %d, attendu 409", code)
	}
	if code, _, _ := e.do("DELETE", "/api/v2/devices/"+nestID, nil, e.token); code != http.StatusNoContent {
		t.Fatalf("suppression du pondoir : %d", code)
	}
}

func TestManualActionIsLogged(t *testing.T) {
	e := newEnv(t)
	path := "/api/v2/devices/" + e.main.ID.String()
	if code, obj, _ := e.do("POST", path+"/actions/open", nil, e.token); code != 200 {
		t.Fatalf("action : %d %v", code, obj)
	}
	if len(e.om.actions) != 1 || e.om.actions[0] != "M1:open" {
		t.Fatalf("appel Omlet : %v", e.om.actions)
	}
	_, _, logs := e.do("GET", path+"/logs", nil, e.token)
	if len(logs) != 1 || logs[0].(map[string]interface{})["triggeredBy"] != "manual" {
		t.Fatalf("journal : %v", logs)
	}
	if code, _, _ := e.do("POST", path+"/actions/explode", nil, e.token); code != 400 {
		t.Fatalf("action inconnue : %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/v2/coops/"+e.coop.ID.String(), map[string]string{"timezone": "Mars/Olympus"}, e.token); code != 400 {
		t.Fatalf("fuseau invalide accepté : %d", code)
	}
}

func TestHomeAssistantState(t *testing.T) {
	e := newEnv(t)
	a := &handlers.API{DB: e.db, Engine: engine.New(e.db, engine.ModeShadow, func(string) engine.Omlet { return e.om }, nil), Started: time.Now()}
	r := SetupRouter(a, Options{JWTSecret: secret, HAToken: "ha-secret"})
	a.Engine.Step(context.Background()) // planning + relevé d'état
	call := func(tok string) (int, map[string]interface{}) {
		req := httptest.NewRequest("GET", "/ha/state", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var m map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}
	if code, _ := call("mauvais"); code != http.StatusUnauthorized {
		t.Fatalf("jeton invalide accepté : %d", code)
	}
	code, m := call("ha-secret")
	if code != 200 || m["mode"] != "shadow" {
		t.Fatalf("état HA : %d %v", code, m)
	}
	main := m["main_door"].(map[string]interface{})
	if main["door"] != "closed" || main["nextOpen"] == nil && main["nextClose"] == nil || main["alert"] != false {
		t.Fatalf("porte principale : %v", main)
	}
}

func freshRouter(t *testing.T) (*gin.Engine, *gorm.DB, *fakeOmlet) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := gdb.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	om := &fakeOmlet{devices: []omlet.Device{door("M1", "G", true)}}
	clients := func(string) engine.Omlet { return om }
	a := &handlers.API{DB: gdb, Engine: engine.New(gdb, engine.ModeLive, clients, nil), Clients: clients, Started: time.Now()}
	return SetupRouter(a, Options{JWTSecret: secret}), gdb, om
}

func call(r *gin.Engine, method, path string, body interface{}, token, lang string) (int, map[string]interface{}) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestFirstRunSignupThenClosed(t *testing.T) {
	r, _, _ := freshRouter(t)
	_, boot := call(r, "GET", "/api/v2/system/bootstrap", nil, "", "")
	if boot["needsSetup"] != true || boot["registrationOpen"] != true {
		t.Fatalf("premier démarrage : %v", boot)
	}
	user := map[string]string{"email": "owner@example.org", "password": "motdepasse", "firstName": "A", "lastName": "B"}
	if code, m := call(r, "POST", "/api/v2/auth/register", user, "", ""); code != http.StatusCreated {
		t.Fatalf("premier compte refusé : %d %v", code, m)
	}
	_, boot = call(r, "GET", "/api/v2/system/bootstrap", nil, "", "")
	if boot["needsSetup"] != false || boot["registrationOpen"] != false {
		t.Fatalf("après le premier compte : %v", boot)
	}
	user["email"] = "other@example.org"
	if code, m := call(r, "POST", "/api/v2/auth/register", user, "", "en-GB,en;q=0.9"); code != http.StatusForbidden || m["error"] != "Sign-ups are closed on this server" {
		t.Fatalf("deuxième compte : %d %v", code, m)
	}
}

func TestCreateCoopInEnglishWithEncryptedKey(t *testing.T) {
	r, gdb, _ := freshRouter(t)
	if err := secrets.Init("cle-de-test-pour-le-chiffrement"); err != nil {
		t.Fatal(err)
	}
	defer secrets.Init("")
	u := models.User{Email: "owner@example.org", PasswordHash: "x"}
	gdb.Create(&u)
	tok := tokenFor(t, u.ID)

	code, m := call(r, "POST", "/api/v2/coops", map[string]interface{}{"name": "Backyard"}, tok, "en-US")
	if code != http.StatusBadRequest || !strings.Contains(m["error"].(string), "required") {
		t.Fatalf("champs manquants : %d %v", code, m)
	}
	body := map[string]interface{}{"name": "Backyard", "latitude": 51.5, "longitude": -0.12, "timezone": "Europe/London", "omletApiKey": "my-omlet-key"}
	code, m = call(r, "POST", "/api/v2/coops", body, tok, "en-US")
	if code != http.StatusCreated {
		t.Fatalf("création : %d %v", code, m)
	}
	coop := m["coop"].(map[string]interface{})
	if coop["language"] != "en" || coop["hasApiKey"] != true {
		t.Fatalf("poulailler créé : %v", coop)
	}
	var raw string
	gdb.Raw("SELECT omlet_api_key FROM coops").Scan(&raw)
	if !strings.HasPrefix(raw, "enc:v1:") || strings.Contains(raw, "my-omlet-key") {
		t.Fatalf("clé non chiffrée en base : %q", raw)
	}
	var stored models.Coop
	gdb.First(&stored)
	if stored.OmletAPIKey != "my-omlet-key" {
		t.Fatalf("clé relue : %q", stored.OmletAPIKey)
	}
	if code, m := call(r, "GET", "/api/v2/coops/"+uuid.NewString(), nil, tok, "en"); code != http.StatusNotFound || m["error"] != "Coop not found" {
		t.Fatalf("erreur en anglais : %d %v", code, m)
	}
}
