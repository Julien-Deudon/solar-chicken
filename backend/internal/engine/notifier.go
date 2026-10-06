package engine

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/services"
	"gorm.io/gorm"
)

// TelegramNotifier envoie les notifications selon les préférences de l'utilisateur.
type TelegramNotifier struct {
	DB *gorm.DB
	TG *services.TelegramService
}

func (n *TelegramNotifier) Notify(ctx context.Context, userID uuid.UUID, kind NotifyKind, message string) {
	var s models.NotificationSettings
	if err := n.DB.Where("user_id = ?", userID).First(&s).Error; err != nil {
		return
	}
	switch kind {
	case NotifyOpen:
		if !s.NotifyOnOpen {
			return
		}
	case NotifyClose:
		if !s.NotifyOnClose {
			return
		}
	case NotifyLight:
		if !s.NotifyOnLight {
			return
		}
	case NotifyError:
		if !s.NotifyOnError {
			return
		}
	case NotifyDaily:
		if !s.NotifyDailySchedule {
			return
		}
	}
	type target struct{ name, token, chat string }
	var targets []target
	for _, a := range s.TelegramAccounts {
		if a.BotToken != "" && a.ChatID != "" {
			targets = append(targets, target{a.Name, a.BotToken, a.ChatID})
		}
	}
	if len(targets) == 0 && s.TelegramBotToken != "" && s.TelegramChatID != "" {
		targets = append(targets, target{"principal", s.TelegramBotToken, s.TelegramChatID})
	}
	for _, t := range targets {
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			if err = n.TG.SendMessage(t.token, t.chat, message); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(attempt+1) * 3 * time.Second):
			}
		}
		if err != nil {
			log.Printf("⚠️  Telegram (%s) : %v", t.name, err)
		}
	}
}
