package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func TestNtfy(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var mu sync.Mutex
	var got []map[string]any
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			http.Error(w, "fora", http.StatusBadGateway)
			return
		}
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		got = append(got, m)
	}))
	defer srv.Close()

	if n, err := New(ctx, st, ""); n != nil || err != nil || n.SubscribeURL() != "" {
		t.Fatalf("sem endereço deveria ficar desligado: %v %v", n, err)
	}
	if _, err := New(ctx, st, "ftp://x"); err == nil {
		t.Fatal("endereço inválido deveria dar erro")
	}
	n, err := New(ctx, st, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := New(ctx, st, srv.URL)
	if !strings.HasPrefix(n.SubscribeURL(), srv.URL+"/tdt-") || again.SubscribeURL() != n.SubscribeURL() {
		t.Fatalf("tópico = %q / %q", n.SubscribeURL(), again.SubscribeURL())
	}

	now := time.Now()
	add := func(key, kind, title, msg string, ts time.Time) {
		if _, err := st.AddEvent(ctx, store.Event{TS: ts.Unix(), Type: kind, Key: key, Title: title, Message: msg}); err != nil {
			t.Fatal(err)
		}
	}
	add("velho", "limit", "Antes de ligar", "x", now)
	// Primeira vez: não despeja o que já existia.
	if err := n.Flush(ctx, now); err != nil || len(got) != 0 {
		t.Fatalf("primeiro giro publicou %v (%v)", got, err)
	}

	add("a", "usage", "TV da sala: Netflix em uso", "Desde 20:14.", now)
	add("b", "new_device", "Aparelho novo na rede: X", "", now)
	add("c", "limit", "Atrasado demais", "x", now.Add(-2*time.Hour))
	setFail := func(v bool) { mu.Lock(); fail = v; mu.Unlock() }
	setFail(true)
	if err := n.Flush(ctx, now); err == nil {
		t.Fatal("ntfy fora deveria dar erro")
	}
	setFail(false)
	if err := n.Flush(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("publicados = %v", got)
	}
	topic := strings.TrimPrefix(n.SubscribeURL(), srv.URL+"/")
	if got[0]["topic"] != topic || got[0]["title"] != "TV da sala: Netflix em uso" || got[0]["message"] != "Desde 20:14." || got[0]["tags"].([]any)[0] != "eyes" {
		t.Errorf("primeiro = %v", got[0])
	}
	if got[1]["title"] != "Tempo de Tela" || got[1]["message"] != "Aparelho novo na rede: X" {
		t.Errorf("sem mensagem = %v", got[1])
	}
	// Nada novo: não republica.
	if err := n.Flush(ctx, now); err != nil || len(got) != 2 {
		t.Fatalf("republicou: %v %v", got, err)
	}
}
