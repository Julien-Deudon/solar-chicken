package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TelegramService struct {
	httpClient *http.Client
}

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func NewTelegramService() *TelegramService {
	return &TelegramService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage envoie un message via Telegram
func (s *TelegramService) SendMessage(botToken, chatID, message string) error {
	if botToken == "" || chatID == "" {
		// Si les credentials ne sont pas configurés, on ne fait rien (mode silencieux)
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := TelegramMessage{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// L'URL contient le jeton du bot : on le masque avant de remonter l'erreur.
		return fmt.Errorf("failed to send telegram message: %s", strings.ReplaceAll(err.Error(), botToken, "***"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// FormatSuccessMessage formatte un message de succès
func (s *TelegramService) FormatSuccessMessage(action, deviceName, timezone string) string {
	emoji := "✅"
	if action == "open" {
		emoji = "🚪 ✅"
	} else if action == "close" {
		emoji = "🔒 ✅"
	} else if action == "light_on" {
		emoji = "💡 ✅"
	} else if action == "light_off" {
		emoji = "🌙 ✅"
	}

	// Convertir l'heure au timezone du device
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC // Fallback sur UTC si timezone invalide
	}
	localTime := time.Now().In(loc)

	return fmt.Sprintf(
		"%s <b>Action réussie</b>\n\n"+
			"<b>Appareil:</b> %s\n"+
			"<b>Action:</b> %s\n"+
			"<b>Heure:</b> %s",
		emoji,
		deviceName,
		action,
		localTime.Format("15:04:05"),
	)
}

// FormatErrorMessage formatte un message d'erreur
func (s *TelegramService) FormatErrorMessage(action, deviceName, errorMsg, timezone string) string {
	// Convertir l'heure au timezone du device
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC // Fallback sur UTC si timezone invalide
	}
	localTime := time.Now().In(loc)

	return fmt.Sprintf(
		"❌ <b>Erreur d'action</b>\n\n"+
			"<b>Appareil:</b> %s\n"+
			"<b>Action:</b> %s\n"+
			"<b>Erreur:</b> %s\n"+
			"<b>Heure:</b> %s",
		deviceName,
		action,
		errorMsg,
		localTime.Format("15:04:05"),
	)
}

// FormatDailySchedule formatte le programme quotidien
func (s *TelegramService) FormatDailySchedule(openTime, closeTime, lightOnTime, lightOffTime string) string {
	return fmt.Sprintf(
		"📅 <b>Programme du jour</b>\n\n"+
			"🌅 <b>Matin:</b>\n"+
			"  💡 Lumière ON: %s\n"+
			"  🚪 Ouverture: %s\n\n"+
			"🌆 <b>Soir:</b>\n"+
			"  💡 Lumière ON: %s\n"+
			"  🔒 Fermeture: %s",
		lightOnTime,
		openTime,
		lightOffTime,
		closeTime,
	)
}
