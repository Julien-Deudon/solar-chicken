// Import des données v1 (base omlet-postgres) vers le modèle v2 (poulailler → appareils → règles).
// L'ancienne base est ouverte en lecture seule. Les identifiants (utilisateur, porte) sont conservés.
//
//	OLD_DB_DSN="host=... port=5432 user=omlet password=... dbname=omlet sslmode=disable" \
//	DB_HOST=... DB_PORT=... DB_USER=... DB_PASSWORD=... DB_NAME=... \
//	/app/import -coop "Poulailler" [-dry-run] [-force]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/db"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type v1Device struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	OmletAPIKey   string
	OmletDeviceID string
	Name          string
	Latitude      float64
	Longitude     float64
	Timezone      string
	CreatedAt     time.Time
}

func (v1Device) TableName() string { return "devices" }

type v1Schedule struct {
	DeviceID                uuid.UUID
	Mode                    string
	OpenOffsetMinutes       int
	CloseOffsetMinutes      int
	OpenTime                *models.TimeOfDay
	CloseTime               *models.TimeOfDay
	LightBeforeOpenMinutes  int
	LightBeforeCloseMinutes int
	EnableLightMorning      bool
	EnableLightEvening      bool
	LightOffDelayMinutes    int
	Enabled                 bool
}

func (v1Schedule) TableName() string { return "schedules" }

type v1ActionLog struct {
	ID           uuid.UUID
	DeviceID     uuid.UUID
	ActionType   string
	Status       string
	ErrorMessage string
	TriggeredBy  string
	UserID       *uuid.UUID
	ExecutedAt   time.Time
}

func (v1ActionLog) TableName() string { return "action_logs" }

var errDryRun = errors.New("dry-run")

func main() {
	coopName := flag.String("coop", "Poulailler", "nom du poulailler créé")
	dryRun := flag.Bool("dry-run", false, "tout simuler puis annuler")
	force := flag.Bool("force", false, "importer même si la base v2 contient déjà des poulaillers")
	flag.Parse()

	oldDSN := os.Getenv("OLD_DB_DSN")
	if oldDSN == "" {
		log.Fatal("OLD_DB_DSN manquant")
	}
	old, err := gorm.Open(postgres.Open(oldDSN), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		log.Fatalf("ancienne base : %v", err)
	}
	oldSQL, _ := old.DB()
	oldSQL.SetMaxOpenConns(1)
	if err := old.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY").Error; err != nil {
		log.Fatalf("ancienne base en lecture seule : %v", err)
	}

	cfg := db.Config{Host: getenv("DB_HOST", "localhost"), Port: getenv("DB_PORT", "5432"), User: getenv("DB_USER", "omlet"),
		Password: os.Getenv("DB_PASSWORD"), DBName: getenv("DB_NAME", "omlet"), SSLMode: getenv("DB_SSL_MODE", "disable")}
	nw, err := db.Connect(cfg, time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(nw); err != nil {
		log.Fatal(err)
	}
	var existing int64
	nw.Model(&models.Coop{}).Count(&existing)
	if existing > 0 && !*force {
		log.Fatalf("la base v2 contient déjà %d poulailler(s) : import annulé (-force pour passer outre)", existing)
	}

	var devices []v1Device
	if err := old.Order("created_at").Find(&devices).Error; err != nil {
		log.Fatalf("lecture des portes v1 : %v", err)
	}
	if len(devices) == 0 {
		log.Fatal("aucune porte dans la base v1")
	}
	schedules := map[uuid.UUID]v1Schedule{}
	var sl []v1Schedule
	old.Find(&sl)
	for _, s := range sl {
		schedules[s.DeviceID] = s
	}
	userIDs := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	deviceIDs := []uuid.UUID{}
	for _, d := range devices {
		deviceIDs = append(deviceIDs, d.ID)
		if !seen[d.UserID] {
			seen[d.UserID] = true
			userIDs = append(userIDs, d.UserID)
		}
	}
	var users []models.User
	old.Where("id IN ?", userIDs).Find(&users)
	var settings []models.NotificationSettings
	old.Where("user_id IN ?", userIDs).Find(&settings)
	var logs []v1ActionLog
	old.Where("device_id IN ?", deviceIDs).Order("executed_at").Find(&logs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	summary := map[string]int{}
	err = nw.Transaction(func(tx *gorm.DB) error {
		for i := range users {
			u := users[i]
			u.VerificationToken = nil
			if err := tx.Create(&u).Error; err != nil {
				return fmt.Errorf("utilisateur %s : %w", u.Email, err)
			}
			summary["utilisateurs"]++
		}
		for i := range settings {
			if err := tx.Create(&settings[i]).Error; err != nil {
				return fmt.Errorf("notifications : %w", err)
			}
			summary["réglages de notification"]++
		}
		coops := map[string]*models.Coop{} // utilisateur + clé API
		for i, d := range devices {
			key := d.UserID.String() + "|" + d.OmletAPIKey
			coop := coops[key]
			od, odErr := omlet.NewClient(d.OmletAPIKey).GetDevice(ctx, d.OmletDeviceID)
			if odErr != nil {
				log.Printf("⚠️  Omlet injoignable pour %s (%v) : lumière supposée présente", d.Name, odErr)
			}
			if coop == nil {
				coop = &models.Coop{UserID: d.UserID, Name: *coopName, OmletAPIKey: d.OmletAPIKey,
					Latitude: d.Latitude, Longitude: d.Longitude, Timezone: d.Timezone}
				if od != nil {
					coop.OmletGroupID = od.GroupID
				}
				if err := tx.Create(coop).Error; err != nil {
					return fmt.Errorf("poulailler : %w", err)
				}
				coops[key] = coop
				summary["poulaillers"]++
			}
			role := models.RoleMainDoor
			if i > 0 {
				role = models.RoleDoor
			}
			s, hasSchedule := schedules[d.ID]
			dev := models.Device{ID: d.ID, CoopID: coop.ID, OmletDeviceID: d.OmletDeviceID, DeviceType: "Autodoor", Role: role,
				Name: "Porte principale", Strategy: models.StrategyCommand, HasLight: od == nil || od.HasLight(),
				Enabled: hasSchedule && s.Enabled, Position: i, CreatedAt: d.CreatedAt}
			if role != models.RoleMainDoor {
				dev.Name = d.Name
			}
			if err := tx.Create(&dev).Error; err != nil {
				return fmt.Errorf("porte %s : %w", d.Name, err)
			}
			summary["portes"]++
			if !hasSchedule {
				continue
			}
			r := models.Rule{DeviceID: d.ID,
				OpenAnchor: models.AnchorSunrise, OpenOffsetMinutes: s.OpenOffsetMinutes,
				CloseAnchor: models.AnchorSunset, CloseOffsetMinutes: s.CloseOffsetMinutes,
				LightBeforeOpenMinutes: s.LightBeforeOpenMinutes, LightBeforeCloseMinutes: s.LightBeforeCloseMinutes,
				EnableLightMorning: s.EnableLightMorning, EnableLightEvening: s.EnableLightEvening, LightOffDelayMinutes: s.LightOffDelayMinutes}
			if s.Mode == "fixed" && s.OpenTime != nil && s.CloseTime != nil {
				r.OpenAnchor, r.OpenOffsetMinutes, r.OpenFixedTime = models.AnchorFixed, 0, s.OpenTime
				r.CloseAnchor, r.CloseOffsetMinutes, r.CloseFixedTime = models.AnchorFixed, 0, s.CloseTime
			}
			if err := r.Validate(); err != nil {
				return fmt.Errorf("règle %s : %w", d.Name, err)
			}
			if err := tx.Create(&r).Error; err != nil {
				return fmt.Errorf("règle %s : %w", d.Name, err)
			}
			summary["règles"]++
			log.Printf("✓ %s → « %s » : %s %+d min / %s %+d min, lumière %d min avant l'ouverture, automatisation %v",
				d.Name, dev.Name, r.OpenAnchor, r.OpenOffsetMinutes, r.CloseAnchor, r.CloseOffsetMinutes, r.LightBeforeOpenMinutes, dev.Enabled)
		}
		for _, l := range logs {
			entry := models.ActionLog{ID: l.ID, DeviceID: l.DeviceID, ActionType: l.ActionType, Status: models.ActionStatus(l.Status),
				ErrorMessage: l.ErrorMessage, TriggeredBy: models.ActionTrigger(l.TriggeredBy), UserID: l.UserID, ExecutedAt: l.ExecutedAt, Note: "historique v1"}
			if err := tx.Create(&entry).Error; err != nil {
				return fmt.Errorf("historique : %w", err)
			}
			summary["actions historiques"]++
		}
		if *dryRun {
			return errDryRun
		}
		return nil
	})
	for k, v := range summary {
		log.Printf("  %s : %d", k, v)
	}
	switch {
	case errors.Is(err, errDryRun):
		log.Println("🧪 Simulation terminée : rien n'a été écrit")
	case err != nil:
		log.Fatalf("❌ Import annulé : %v", err)
	default:
		log.Println("✅ Import terminé")
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
