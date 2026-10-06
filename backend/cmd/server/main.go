package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/julien-deudon/solar-chicken/backend/internal/api"
	"github.com/julien-deudon/solar-chicken/backend/internal/api/handlers"
	"github.com/julien-deudon/solar-chicken/backend/internal/config"
	"github.com/julien-deudon/solar-chicken/backend/internal/db"
	"github.com/julien-deudon/solar-chicken/backend/internal/engine"
	"github.com/julien-deudon/solar-chicken/backend/internal/models"
	"github.com/julien-deudon/solar-chicken/backend/internal/omlet"
	"github.com/julien-deudon/solar-chicken/backend/internal/secrets"
	"github.com/julien-deudon/solar-chicken/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// version est renseignée à la compilation (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		resetPassword(os.Args[2:])
		return
	}
	cfg := config.Load()
	log.Printf("🐔 Solar Chicken %s · mode %s", version, cfg.Mode)
	if err := secrets.Init(cfg.SecretKey); err != nil {
		log.Fatalf("❌ SECRET_KEY invalide : %v", err)
	}
	if !secrets.Enabled() {
		log.Println("⚠️  SECRET_KEY absente : les clés Omlet et jetons Telegram sont stockés en clair")
	}

	database, err := db.Connect(cfg.DB, 10*time.Minute)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatalf("❌ Migration : %v", err)
	}
	if n, err := db.EncryptExistingSecrets(database); err != nil {
		log.Fatalf("❌ Chiffrement des secrets existants : %v", err)
	} else if n > 0 {
		log.Printf("🔐 %d enregistrement(s) chiffré(s)", n)
	}

	clients := func(key string) engine.Omlet { return omlet.NewClient(key) }
	notifier := &engine.TelegramNotifier{DB: database, TG: services.NewTelegramService()}
	eng := engine.New(database, cfg.Mode, clients, notifier)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() { eng.Run(ctx); close(done) }()

	a := &handlers.API{DB: database, Engine: eng, Clients: clients, Version: version, Started: time.Now()}
	router := api.SetupRouter(a, api.Options{
		JWTSecret: cfg.JWTSecret, CORSOrigins: cfg.CORSOrigins,
		AllowRegistration: cfg.AllowRegistration, Email: services.NewEmailService(), HAToken: cfg.HAToken,
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("🚀 API sur :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("❌ Serveur HTTP : %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("🛑 Arrêt demandé")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	select {
	case <-done:
	case <-shutdownCtx.Done():
	}
	log.Println("✅ Arrêté proprement")
}

// resetPassword : `server reset-password <email>` puis le nouveau mot de passe sur l'entrée standard.
// Exemple : echo 'nouveau-mot-de-passe' | docker compose exec -T api /app/server reset-password moi@exemple.fr
func resetPassword(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage : server reset-password <email>   (nouveau mot de passe sur l'entrée standard)")
		os.Exit(2)
	}
	cfg := config.Load()
	database, err := db.Connect(cfg.DB, time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprint(os.Stderr, "Nouveau mot de passe (8 caractères minimum) : ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password := strings.TrimRight(line, "\r\n")
	if len(password) < 8 {
		log.Fatal("mot de passe trop court (8 caractères minimum)")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	res := database.Model(&models.User{}).Where("LOWER(email) = LOWER(?)", args[0]).Update("password_hash", string(hash))
	if res.Error != nil {
		log.Fatal(res.Error)
	}
	if res.RowsAffected == 0 {
		log.Fatalf("aucun compte pour %s", args[0])
	}
	fmt.Fprintln(os.Stderr, "\nMot de passe modifié.")
}
