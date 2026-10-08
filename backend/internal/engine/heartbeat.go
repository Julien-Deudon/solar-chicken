package engine

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// HeartbeatEvery : intervalle entre deux signaux au veilleur extérieur.
const HeartbeatEvery = 5 * time.Minute

// RunHeartbeat signale à un veilleur extérieur (ex. https://hc-ping.com/<uuid> de healthchecks.io) que le
// serveur tourne : un signal toutes les HeartbeatEvery, seulement si la base répond et que la boucle du moteur
// avance. Si le serveur, le courant ou internet tombent, les signaux s'arrêtent et le veilleur prévient
// l'utilisateur après la période et le délai de grâce qu'il a choisis (rien d'immédiat : pas de signal d'échec).
// C'est la seule alerte qui ne dépend pas de ce serveur.
func (e *Engine) RunHeartbeat(ctx context.Context, url, version string) {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	failing := false
	beat := func() {
		if problem := e.healthProblem(ctx); problem != "" {
			log.Printf("⚠️  Pas de signal au veilleur : %s", problem)
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("mode "+string(e.Mode)+" · "+version))
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 300 {
				err = &statusError{resp.StatusCode}
			}
		}
		// Un seul message par série d'échecs : sans internet, le veilleur prévient de toute façon.
		if err != nil && !failing {
			log.Printf("⚠️  Veilleur extérieur injoignable : %v", err)
		} else if err == nil && failing {
			log.Printf("✅ Veilleur extérieur de nouveau joignable")
		}
		failing = err != nil
	}
	beat()
	t := time.NewTicker(HeartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			beat()
		}
	}
}

// healthProblem décrit ce qui ne va pas ("" si tout va bien) : base injoignable ou boucle du moteur arrêtée.
func (e *Engine) healthProblem(ctx context.Context) string {
	sqlDB, err := e.DB.DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		return "base de données injoignable : " + err.Error()
	}
	if last := e.LastTick(); last.Unix() > 0 && e.now().Sub(last) > 2*time.Minute {
		return "boucle du moteur arrêtée depuis " + e.now().Sub(last).Round(time.Second).String()
	}
	return ""
}

type statusError struct{ code int }

func (s *statusError) Error() string { return "HTTP " + http.StatusText(s.code) }
