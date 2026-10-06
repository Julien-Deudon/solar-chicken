package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
