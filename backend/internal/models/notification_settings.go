package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TelegramAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
}

type TelegramAccounts []TelegramAccount

// Scan implements sql.Scanner
func (t *TelegramAccounts) Scan(value interface{}) error {
	if value == nil {
		*t = TelegramAccounts{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		*t = TelegramAccounts{}
		return nil
	}
	if err := json.Unmarshal(raw, t); err != nil {
		return err
	}
	for i := range *t {
		plain, err := secrets.Decrypt((*t)[i].BotToken)
		if err != nil {
			return err
		}
		(*t)[i].BotToken = plain
	}
	return nil
}

// Value implements driver.Valuer
func (t TelegramAccounts) Value() (driver.Value, error) {
	if len(t) == 0 {
		return "[]", nil
	}
	enc := make(TelegramAccounts, len(t))
	for i, a := range t {
		token, err := secrets.Encrypt(a.BotToken)
		if err != nil {
			return nil, err
		}
		a.BotToken = token
		enc[i] = a
	}
	b, err := json.Marshal(enc)
	return string(b), err
}

type NotificationSettings struct {
	ID               uuid.UUID        `gorm:"type:uuid;primary_key" json:"id"`
	UserID           uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex" json:"userId"`
	TelegramBotToken string           `gorm:"serializer:encrypted" json:"telegramBotToken,omitempty"` // Gardé pour rétrocompatibilité
	TelegramChatID   string           `json:"telegramChatId,omitempty"`                               // Gardé pour rétrocompatibilité
	TelegramAccounts TelegramAccounts `gorm:"type:jsonb;default:'[]'" json:"telegramAccounts"`        // Nouveau champ pour multiple comptes

	// Préférences de notification granulaires
	NotifyDailySchedule bool `gorm:"default:true" json:"notifyDailySchedule"` // Notification du planning le matin
	NotifyOnOpen        bool `gorm:"default:true" json:"notifyOnOpen"`        // Notification à l'ouverture
	NotifyOnClose       bool `gorm:"default:true" json:"notifyOnClose"`       // Notification à la fermeture
	NotifyOnLight       bool `gorm:"default:true" json:"notifyOnLight"`       // Notification pour la lumière
	NotifyOnError       bool `gorm:"default:true" json:"notifyOnError"`       // Notification en cas d'erreur

	// Anciens champs (dépréciés, gardés pour compatibilité)
	NotifyOnSuccess bool `gorm:"default:true" json:"notifyOnSuccess,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (n *NotificationSettings) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
