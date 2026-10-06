package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/middleware"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/services"
	"gorm.io/gorm"
)

type NotificationSettingsHandler struct {
	DB *gorm.DB
}

type UpdateNotificationSettingsRequest struct {
	TelegramBotToken    *string                  `json:"telegramBotToken"`
	TelegramChatID      *string                  `json:"telegramChatId"`
	TelegramAccounts    *models.TelegramAccounts `json:"telegramAccounts"`
	NotifyDailySchedule *bool                    `json:"notifyDailySchedule"`
	NotifyOnOpen        *bool                    `json:"notifyOnOpen"`
	NotifyOnClose       *bool                    `json:"notifyOnClose"`
	NotifyOnLight       *bool                    `json:"notifyOnLight"`
	NotifyOnError       *bool                    `json:"notifyOnError"`
	NotifyOnSuccess     *bool                    `json:"notifyOnSuccess"` // Déprécié mais gardé pour compatibilité
}

// GetSettings récupère les paramètres de notification de l'utilisateur
func (h *NotificationSettingsHandler) GetSettings(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var settings models.NotificationSettings
	if err := h.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Créer des settings par défaut
			settings = models.NotificationSettings{
				UserID:              userID,
				NotifyDailySchedule: true,
				NotifyOnOpen:        true,
				NotifyOnClose:       true,
				NotifyOnLight:       true,
				NotifyOnError:       true,
			}
			if err := h.DB.Create(&settings).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create settings"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
			return
		}
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings met à jour les paramètres de notification
func (h *NotificationSettingsHandler) UpdateSettings(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req UpdateNotificationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Récupérer ou créer les settings
	var settings models.NotificationSettings
	if err := h.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			settings = models.NotificationSettings{
				UserID: userID,
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
			return
		}
	}

	// Mettre à jour uniquement les champs fournis
	if req.TelegramBotToken != nil {
		settings.TelegramBotToken = *req.TelegramBotToken
	}
	if req.TelegramChatID != nil {
		settings.TelegramChatID = *req.TelegramChatID
	}
	if req.TelegramAccounts != nil {
		settings.TelegramAccounts = *req.TelegramAccounts
	}
	if req.NotifyDailySchedule != nil {
		settings.NotifyDailySchedule = *req.NotifyDailySchedule
	}
	if req.NotifyOnOpen != nil {
		settings.NotifyOnOpen = *req.NotifyOnOpen
	}
	if req.NotifyOnClose != nil {
		settings.NotifyOnClose = *req.NotifyOnClose
	}
	if req.NotifyOnLight != nil {
		settings.NotifyOnLight = *req.NotifyOnLight
	}
	if req.NotifyOnError != nil {
		settings.NotifyOnError = *req.NotifyOnError
	}
	if req.NotifyOnSuccess != nil {
		settings.NotifyOnSuccess = *req.NotifyOnSuccess
	}

	// Sauvegarder
	if err := h.DB.Save(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// TestNotification envoie une notification de test
func (h *NotificationSettingsHandler) TestNotification(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Structure pour recevoir les credentials depuis le frontend
	var testRequest struct {
		TelegramBotToken string `json:"telegramBotToken"`
		TelegramChatID   string `json:"telegramChatId"`
	}

	// Essayer de lire depuis le body (test sans sauvegarder)
	if err := c.ShouldBindJSON(&testRequest); err == nil && testRequest.TelegramBotToken != "" && testRequest.TelegramChatID != "" {
		// Test avec les credentials fournis directement (pas encore sauvegardés)
		telegramService := services.NewTelegramService()
		testMessage := tr(c, "🧪 <b>Test de notification Solar Chicken</b>\n\nSi tu reçois ce message, tes notifications Telegram fonctionnent ! ✅",
			"🧪 <b>Solar Chicken test notification</b>\n\nIf you got this message, your Telegram notifications work! ✅")

		if err := telegramService.SendMessage(testRequest.TelegramBotToken, testRequest.TelegramChatID, testMessage); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to send test notification",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Test notification sent successfully"})
		return
	}

	// Sinon, utiliser les credentials sauvegardés en base
	var settings models.NotificationSettings
	if err := h.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Notification settings not configured",
				"message": "Please save your Telegram credentials first",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
		return
	}

	if settings.TelegramBotToken == "" || settings.TelegramChatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram credentials not configured"})
		return
	}

	// Envoyer un message de test
	telegramService := services.NewTelegramService()
	testMessage := tr(c, "🧪 <b>Test de notification Solar Chicken</b>\n\nSi tu reçois ce message, tes notifications Telegram fonctionnent ! ✅",
		"🧪 <b>Solar Chicken test notification</b>\n\nIf you got this message, your Telegram notifications work! ✅")

	if err := telegramService.SendMessage(settings.TelegramBotToken, settings.TelegramChatID, testMessage); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send test notification",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test notification sent successfully"})
}
