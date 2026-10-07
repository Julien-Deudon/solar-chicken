package db

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Sous Postgres, les heures des règles sont du texte « HH:MM ». Avec la balise type:time, GORM créait
// des colonnes timestamptz et l'ajout d'un pondoir (« pas avant 08:00 ») échouait : les tests SQLite
// ne le voyaient pas, ce test regarde le type que GORM demande à Postgres.
func TestRuleTimeColumnsAreTextOnPostgres(t *testing.T) {
	s, err := schema.Parse(&models.Rule{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	pg := postgres.Dialector{Config: &postgres.Config{}}
	for _, name := range []string{"OpenFixedTime", "OpenNotBefore", "OpenNotAfter", "CloseFixedTime", "CloseNotBefore", "CloseNotAfter"} {
		if got := pg.DataTypeOf(s.LookUpField(name)); got != "varchar(5)" {
			t.Errorf("%s : type Postgres %q, attendu varchar(5)", name, got)
		}
	}
	v, _ := models.TimeOfDay{Hour: 8}.Value()
	var back models.TimeOfDay
	if err := back.Scan(v); err != nil || v != "08:00" || back.Hour != 8 || back.Minute != 0 {
		t.Fatalf("aller-retour HH:MM : %v → %+v (%v)", v, back, err)
	}
}

// Base créée par la v0.1.0 (heures des règles en timestamptz) : Migrate les convertit en texte et une
// règle « pas avant 08:00 » s'enregistre. Lancé seulement avec TEST_POSTGRES_DSN (CI : service Postgres).
func TestMigrateConvertsRuleTimesOnPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN absent")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	schemaName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	gdb.Exec("CREATE SCHEMA " + schemaName)
	t.Cleanup(func() { gdb.Exec("DROP SCHEMA " + schemaName + " CASCADE") })
	sqlDB, _ := gdb.DB()
	sqlDB.SetMaxOpenConns(1) // une seule connexion : le search_path vaut pour toute la suite
	gdb.Exec("SET search_path TO " + schemaName)
	if err := gdb.Exec(`CREATE TABLE rules (id uuid PRIMARY KEY, open_fixed_time timestamptz, open_not_before timestamptz,
		open_not_after timestamptz, close_fixed_time timestamptz, close_not_before timestamptz, close_not_after timestamptz)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	user := models.User{Email: "pg@example.org", PasswordHash: "x"}
	gdb.Create(&user)
	coop := models.Coop{UserID: user.ID, Name: "Poulailler", OmletAPIKey: "k", Latitude: 50.4, Longitude: 2.8, Timezone: "Europe/Paris"}
	gdb.Create(&coop)
	nest := models.Device{CoopID: coop.ID, OmletDeviceID: "N1", DeviceType: "Autodoor", Role: models.RoleNestBox, Name: "Pondoir",
		Strategy: models.StrategyOnboard, Enabled: true}
	if err := gdb.Create(&nest).Error; err != nil {
		t.Fatal(err)
	}
	rule := models.Rule{DeviceID: nest.ID, OpenAnchor: models.AnchorSunrise, CloseAnchor: models.AnchorSunset,
		OpenNotBefore: &models.TimeOfDay{Hour: 8}}
	if err := gdb.Create(&rule).Error; err != nil {
		t.Fatalf("règle avec « pas avant » refusée : %v", err)
	}
	var back models.Rule
	if err := gdb.First(&back, "id = ?", rule.ID).Error; err != nil || back.OpenNotBefore == nil || back.OpenNotBefore.String() != "08:00" {
		t.Fatalf("relecture : %+v (%v)", back.OpenNotBefore, err)
	}
	if err := Migrate(gdb); err != nil { // idempotent
		t.Fatal(err)
	}
}

// Une installation d'avant le chiffrement : les secrets en clair sont chiffrés au démarrage.
func TestEncryptExistingSecrets(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := gdb.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	_ = secrets.Init("")
	user := models.User{Email: "a@example.org", PasswordHash: "x"}
	gdb.Create(&user)
	gdb.Create(&models.Coop{UserID: user.ID, Name: "C", OmletAPIKey: "plain-key", Timezone: "Europe/Paris", Language: "fr"})
	gdb.Create(&models.NotificationSettings{UserID: user.ID, TelegramBotToken: "123:plain-token",
		TelegramAccounts: models.TelegramAccounts{{ID: "1", Name: "moi", BotToken: "456:other-token", ChatID: "42"}}})

	if err := secrets.Init("cle-de-migration"); err != nil {
		t.Fatal(err)
	}
	defer secrets.Init("")
	n, err := EncryptExistingSecrets(gdb)
	if err != nil || n != 2 {
		t.Fatalf("migration : %d %v", n, err)
	}
	var key, token, accounts string
	gdb.Raw("SELECT omlet_api_key FROM coops").Scan(&key)
	gdb.Raw("SELECT telegram_bot_token, CAST(telegram_accounts AS TEXT) FROM notification_settings").Row().Scan(&token, &accounts)
	for _, v := range []string{key, token} {
		if !strings.HasPrefix(v, "enc:v1:") {
			t.Fatalf("valeur restée en clair : %q", v)
		}
	}
	if strings.Contains(accounts, "456:other-token") {
		t.Fatalf("jeton Telegram multi-comptes resté en clair : %s", accounts)
	}
	var s models.NotificationSettings
	gdb.First(&s)
	if s.TelegramBotToken != "123:plain-token" || s.TelegramAccounts[0].BotToken != "456:other-token" {
		t.Fatalf("relecture : %q %q", s.TelegramBotToken, s.TelegramAccounts[0].BotToken)
	}
	if n, _ := EncryptExistingSecrets(gdb); n != 0 {
		t.Fatalf("deuxième passage : %d enregistrement(s) rechiffré(s)", n)
	}
}
