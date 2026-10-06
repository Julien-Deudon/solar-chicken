package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/middleware"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB                *gorm.DB
	JWTSecret         string
	EmailService      *services.EmailService
	AllowRegistration bool
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// Register crée un nouveau compte utilisateur
func (h *AuthHandler) Register(c *gin.Context) {
	var users int64
	h.DB.Model(&models.User{}).Count(&users)
	if !h.AllowRegistration && users > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": tr(c, "Les inscriptions sont fermées sur ce serveur", "Sign-ups are closed on this server")})
		return
	}
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vérifier si l'utilisateur existe déjà
	var existing models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Hasher le mot de passe
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Générer le token de vérification
	verificationToken, err := generateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	// Créer l'utilisateur
	user := models.User{
		Email:             req.Email,
		PasswordHash:      string(hashedPassword),
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		EmailVerified:     false,
		VerificationToken: &verificationToken,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Envoyer l'email de vérification
	if h.EmailService != nil {
		if err := h.EmailService.SendVerificationEmail(user.Email, user.FirstName, verificationToken); err != nil {
			// Log l'erreur mais ne pas bloquer l'inscription
			c.JSON(http.StatusCreated, gin.H{
				"message": "Account created but failed to send verification email. Please contact support.",
				"user":    user,
			})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created successfully. Please check your email to verify your account.",
		"user":    user,
	})
}

// Login authentifie un utilisateur
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Trouver l'utilisateur
	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Vérifier le mot de passe
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// TODO: Réactiver la vérification d'email quand le système d'email sera en place
	// if !user.EmailVerified {
	// 	c.JSON(http.StatusForbidden, gin.H{
	// 		"error":   "Email not verified",
	// 		"message": "Please verify your email before logging in. Check your inbox for the verification link.",
	// 	})
	// 	return
	// }

	// Générer le token JWT
	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User:  &user,
	})
}

// Me retourne l'utilisateur actuellement connecté
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.Preload("NotificationSettings").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// VerifyEmail vérifie l'email d'un utilisateur
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	// Trouver l'utilisateur avec ce token
	var user models.User
	if err := h.DB.Where("verification_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired verification token"})
		return
	}

	// Vérifier si déjà vérifié
	if user.EmailVerified {
		c.JSON(http.StatusOK, gin.H{"message": "Email already verified"})
		return
	}

	// Marquer comme vérifié
	now := time.Now()
	user.EmailVerified = true
	user.VerificationToken = nil
	user.VerifiedAt = &now

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully! You can now log in.",
		"user":    user,
	})
}

// ResendVerification renvoie l'email de vérification
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Trouver l'utilisateur
	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Ne pas révéler si l'email existe ou non (sécurité)
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a verification link has been sent"})
		return
	}

	// Vérifier si déjà vérifié
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is already verified"})
		return
	}

	// Générer un nouveau token
	verificationToken, err := generateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	user.VerificationToken = &verificationToken
	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Envoyer l'email
	if h.EmailService != nil {
		if err := h.EmailService.SendVerificationEmail(user.Email, user.FirstName, verificationToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Verification email sent successfully"})
}

// generateToken génère un JWT pour un utilisateur
func (h *AuthHandler) generateToken(userID uuid.UUID, email string) (string, error) {
	claims := middleware.JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)), // 7 jours
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.JWTSecret))
}

// generateSecureToken génère un token sécurisé aléatoire
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
