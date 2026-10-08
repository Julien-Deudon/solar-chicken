package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Le veilleur reçoit un signal au démarrage ; plus rien si la base ne répond plus (il préviendra après le délai choisi).
func TestHeartbeatPingsOnlyWhenHealthy(t *testing.T) {
	b := newBench(t, ModeLive, false)
	b.e.Step(context.Background())
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- r.URL.Path + " " + string(body)
	}))
	defer srv.Close()

	beatOnce := func(wait time.Duration) string {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go b.e.RunHeartbeat(ctx, srv.URL+"/uuid/", "v-test")
		select {
		case s := <-got:
			return s
		case <-time.After(wait):
			return ""
		}
	}
	if s := beatOnce(5 * time.Second); s != "/uuid mode live · v-test" {
		t.Fatalf("signal : %q", s)
	}
	sqlDB, _ := b.e.DB.DB()
	sqlDB.Close()
	if s := beatOnce(500 * time.Millisecond); s != "" {
		t.Fatalf("signal envoyé alors que la base ne répond plus : %q", s)
	}
}

func TestHeartbeatDisabledWithoutURL(t *testing.T) {
	b := newBench(t, ModeLive, false)
	done := make(chan struct{})
	go func() { b.e.RunHeartbeat(context.Background(), "  ", "v"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sans URL, le veilleur ne doit pas tourner")
	}
}
