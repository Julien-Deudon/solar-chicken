package db

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// quietLogger journalise erreurs et requêtes lentes sans jamais écrire les valeurs
// des paramètres (clés API, jetons...).
func quietLogger() logger.Interface {
	return logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	})
}

// Connect ouvre la base en réessayant tant qu'elle n'est pas prête (démarrage du serveur),
// au lieu de quitter et d'être relancé en boucle.
func Connect(cfg Config, maxWait time.Duration) (*gorm.DB, error) {
	deadline := time.Now().Add(maxWait)
	delay := time.Second
	for {
		db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{Logger: quietLogger()})
		if err == nil {
			sqlDB, _ := db.DB()
			if err = sqlDB.Ping(); err == nil {
				log.Println("✅ Base de données connectée")
				return db, nil
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("base de données indisponible après %s : %w", maxWait, err)
		}
		log.Printf("⏳ Base de données pas encore prête, nouvel essai dans %s", delay)
		time.Sleep(delay)
		if delay *= 2; delay > 15*time.Second {
			delay = 15 * time.Second
		}
	}
}

// Migrate crée ou met à jour le schéma v2.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.NotificationSettings{},
		&models.Coop{},
		&models.Device{},
		&models.Rule{},
		&models.PlannedEvent{},
		&models.ActionLog{},
	)
}

// EncryptExistingSecrets chiffre les secrets encore stockés en clair (installations d'avant le chiffrement).
func EncryptExistingSecrets(db *gorm.DB) (int, error) {
	if !secrets.Enabled() {
		return 0, nil
	}
	count := 0
	var coops []models.Coop
	if err := db.Find(&coops).Error; err != nil {
		return 0, err
	}
	for i := range coops {
		var raw string
		db.Raw("SELECT omlet_api_key FROM coops WHERE id = ?", coops[i].ID).Scan(&raw)
		if raw != "" && !secrets.IsEncrypted(raw) {
			if err := db.Model(&coops[i]).Select("omlet_api_key").Updates(&coops[i]).Error; err != nil {
				return count, err
			}
			count++
		}
	}
	var settings []models.NotificationSettings
	if err := db.Find(&settings).Error; err != nil {
		return count, err
	}
	for i := range settings {
		var token, accounts string
		db.Raw("SELECT COALESCE(telegram_bot_token, ''), COALESCE(CAST(telegram_accounts AS TEXT), '') FROM notification_settings WHERE id = ?", settings[i].ID).Row().Scan(&token, &accounts)
		needs := token != "" && !secrets.IsEncrypted(token)
		for _, a := range settings[i].TelegramAccounts {
			if a.BotToken != "" && storedInClear(accounts, a.BotToken) {
				needs = true
			}
		}
		if needs {
			if err := db.Model(&settings[i]).Select("telegram_bot_token", "telegram_accounts").Updates(&settings[i]).Error; err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

// storedInClear : le jeton apparaît-il en clair dans le JSON stocké ?
func storedInClear(storedJSON, plainToken string) bool {
	return strings.Contains(storedJSON, plainToken)
}
