// Package config lit la configuration depuis l'environnement (et un .env facultatif).
package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/julien-deudon/solar-chicken/backend/internal/db"
	"github.com/julien-deudon/solar-chicken/backend/internal/engine"
)

type Config struct {
	DB                db.Config
	JWTSecret         string
	Port              string
	Mode              engine.Mode
	CORSOrigins       []string
	AllowRegistration bool
	HAToken           string
	SecretKey         string
	HealthcheckURL    string // veilleur extérieur (ex. https://hc-ping.com/<uuid>), facultatif
}

func Load() Config {
	_ = godotenv.Load() // facultatif (développement local)
	c := Config{
		DB: db.Config{
			Host:     env("DB_HOST", "localhost"),
			Port:     env("DB_PORT", "5432"),
			User:     env("DB_USER", "omlet"),
			Password: env("DB_PASSWORD", ""),
			DBName:   env("DB_NAME", "omlet"),
			SSLMode:  env("DB_SSL_MODE", "disable"),
		},
		JWTSecret:         os.Getenv("JWT_SECRET"),
		Port:              env("PORT", "8080"),
		Mode:              engine.Mode(env("EXECUTION_MODE", string(engine.ModeShadow))),
		AllowRegistration: env("ALLOW_REGISTRATION", "false") == "true",
		HAToken:           os.Getenv("HA_TOKEN"),
		SecretKey:         os.Getenv("SECRET_KEY"),
		HealthcheckURL:    os.Getenv("HEALTHCHECK_URL"),
	}
	if c.JWTSecret == "" {
		log.Fatal("❌ JWT_SECRET est obligatoire")
	}
	if c.Mode != engine.ModeShadow && c.Mode != engine.ModeLive {
		log.Fatalf("❌ EXECUTION_MODE doit valoir shadow ou live (reçu %q)", c.Mode)
	}
	for _, o := range strings.Split(os.Getenv("CORS_ALLOW_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	return c
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
