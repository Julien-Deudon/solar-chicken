package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/handlers"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/middleware"
	"github.com/julien-deudon/solar-chicken/backend/internal/services"
)

type Options struct {
	JWTSecret         string
	CORSOrigins       []string
	AllowRegistration bool
	Email             *services.EmailService
	HAToken           string // jeton du point /ha/state (vide = désactivé)
}

// SetupRouter déclare les routes de l'API v2.
func SetupRouter(a *handlers.API, opt Options) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health"}}))
	if len(opt.CORSOrigins) > 0 {
		cfg := cors.DefaultConfig()
		cfg.AllowOrigins = opt.CORSOrigins
		cfg.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
		router.Use(cors.New(cfg))
	}
	router.GET("/health", a.Health)
	router.GET("/ha/state", a.HAState(opt.HAToken))

	v2 := router.Group("/api/v2")
	v2.GET("/system/bootstrap", a.Bootstrap(opt.AllowRegistration))
	auth := &handlers.AuthHandler{DB: a.DB, JWTSecret: opt.JWTSecret, EmailService: opt.Email, AllowRegistration: opt.AllowRegistration}
	v2.POST("/auth/register", middleware.RateLimitMiddleware("5-H"), auth.Register)
	v2.POST("/auth/login", middleware.RateLimitMiddleware("20-M"), auth.Login)
	v2.GET("/auth/verify-email", auth.VerifyEmail)
	v2.POST("/auth/resend-verification", middleware.RateLimitMiddleware("5-H"), auth.ResendVerification)

	p := v2.Group("")
	p.Use(middleware.AuthMiddleware(opt.JWTSecret))
	p.GET("/auth/me", auth.Me)
	p.GET("/system", a.System)

	p.GET("/coops", a.ListCoops)
	p.POST("/coops", a.CreateCoop)
	p.GET("/coops/:id", a.GetCoop)
	p.PUT("/coops/:id", a.UpdateCoop)
	p.GET("/coops/:id/plan", a.GetPlan)
	p.GET("/coops/:id/omlet-devices", a.DiscoverDevices)
	p.POST("/coops/:id/devices", a.AddDevice)

	p.PUT("/devices/:id", a.UpdateDevice)
	p.DELETE("/devices/:id", a.DeleteDevice)
	p.GET("/devices/:id/status", a.DeviceStatus)
	p.POST("/devices/:id/actions/:action", a.DeviceAction)
	p.GET("/devices/:id/logs", a.DeviceLogs)
	p.GET("/devices/:id/rule", a.GetRule)
	p.PUT("/devices/:id/rule", a.PutRule)
	p.POST("/devices/:id/rule/preview", a.PreviewRule)

	notif := &handlers.NotificationSettingsHandler{DB: a.DB}
	p.GET("/settings/notifications", notif.GetSettings)
	p.PUT("/settings/notifications", notif.UpdateSettings)
	p.POST("/settings/notifications/test", notif.TestNotification)
	return router
}
