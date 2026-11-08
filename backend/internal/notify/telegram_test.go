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

// fakeTelegram imita a API de bots: um token válido, fila de updates e mensagens enviadas.
type fakeTelegram struct {
	mu      sync.Mutex
	token   string
	updates []map[string]any
	sent    []map[string]any
	fail    bool
	blocked map[int64]bool
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token, method, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/bot"), "/")
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	answer := func(result any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result}) }
	fail := func(code int, desc string) {
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": code, "description": desc})
	}
	switch {
	case f.fail:
		fail(http.StatusBadGateway, "Bad Gateway")
	case token != f.token:
		fail(http.StatusUnauthorized, "Unauthorized")
	case method == "getMe":
		answer(map[string]any{"id": 1, "username": "casa_bot"})
	case method == "getUpdates":
		offset := int64(body["offset"].(float64))
		var out []map[string]any
		for _, u := range f.updates {
			if int64(u["update_id"].(int)) >= offset {
				out = append(out, u)
			}
		}
		answer(out)
	case method == "sendMessage":
		chat := int64(body["chat_id"].(float64))
		if f.blocked[chat] {
			fail(http.StatusForbidden, "Forbidden: bot was blocked by the user")
			return
		}
		f.sent = append(f.sent, body)
		answer(map[string]any{"message_id": len(f.sent)})
	default:
		fail(http.StatusNotFound, "Not Found")
	}
}

func (f *fakeTelegram) start(id int, chat int64, name, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, map[string]any{"update_id": id, "message": map[string]any{
		"text": text, "chat": map[string]any{"id": chat, "first_name": name},
	}})
}

func (f *fakeTelegram) take() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.sent
	f.sent = nil
	return out
}

func TestTelegram(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	fake := &fakeTelegram{token: "123:abc", blocked: map[int64]bool{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	tg, _ := NewTelegram(ctx, st, srv.URL)
	if tg.LinkURL() != "" || tg.Active() {
		t.Fatal("sem bot não deveria ter link")
	}
	if err := tg.SetToken(ctx, "999:errado"); err == nil || !strings.Contains(err.Error(), "recusou") {
		t.Fatalf("token errado: %v", err)
	}
	if err := tg.SetToken(ctx, " 123:abc "); err != nil {
		t.Fatal(err)
	}
	link := tg.LinkURL()
	code := strings.TrimPrefix(link, "https://t.me/casa_bot?start=")
	if code == link || len(code) < 16 {
		t.Fatalf("link = %q", link)
	}
	// Configuração sobrevive a reiniciar.
	if again, _ := NewTelegram(ctx, st, srv.URL); again.LinkURL() != link {
		t.Fatalf("link depois de reiniciar = %q", again.LinkURL())
	}

	add := func(key, kind, title, msg string, ts time.Time) {
		if _, err := st.AddEvent(ctx, store.Event{TS: ts.Unix(), Type: kind, Key: key, Title: title, Message: msg}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	add("velho", "limit", "Antes de conectar", "x", now)

	// Código errado e mensagem qualquer: não conecta, só responde.
	fake.start(1, 50, "Intruso", "/start chute")
	fake.start(2, 50, "Intruso", "oi")
	if err := tg.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if tg.Active() || len(fake.take()) != 2 {
		t.Fatal("código errado não deveria conectar")
	}
	if err := tg.Flush(ctx, now); err != nil {
		t.Fatal(err)
	}

	fake.start(3, 77, "Guilherme", "/start "+code)
	if err := tg.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if !tg.Active() || tg.Config().Chats[0].Name != "Guilherme" {
		t.Fatalf("conversa = %+v", tg.Config().Chats)
	}
	if sent := fake.take(); len(sent) != 1 || !strings.Contains(sent[0]["text"].(string), "Pronto") {
		t.Fatalf("resposta = %v", sent)
	}
	// Ler de novo não reprocessa os mesmos updates.
	tg.Poll(ctx)
	if len(fake.take()) != 0 || len(tg.Config().Chats) != 1 {
		t.Fatal("reprocessou updates")
	}

	// O que já existia antes de conectar não é despejado.
	if err := tg.Flush(ctx, now); err != nil || len(fake.take()) != 0 {
		t.Fatalf("despejou histórico (%v)", err)
	}

	add("a", "usage", "TV da sala: Netflix <em uso>", "Desde 20:14.", now)
	add("b", "limit", "Atrasado demais", "x", now.Add(-2*time.Hour))
	fake.mu.Lock()
	fake.fail = true
	fake.mu.Unlock()
	if err := tg.Flush(ctx, now); err == nil {
		t.Fatal("Telegram fora deveria dar erro")
	}
	fake.mu.Lock()
	fake.fail = false
	fake.mu.Unlock()
	if err := tg.Flush(ctx, now); err != nil {
		t.Fatal(err)
	}
	sent := fake.take()
	if len(sent) != 1 || sent[0]["text"] != "👀 <b>TV da sala: Netflix &lt;em uso&gt;</b>\nDesde 20:14." || sent[0]["parse_mode"] != "HTML" {
		t.Fatalf("enviados = %v", sent)
	}

	// Segunda conversa bloqueou o bot: a primeira continua recebendo.
	fake.start(4, 88, "Carolina", "/start "+code)
	tg.Poll(ctx)
	fake.take()
	fake.mu.Lock()
	fake.blocked[88] = true
	fake.mu.Unlock()
	add("c", "new_device", "Aparelho novo na rede: X", "", now)
	if err := tg.Flush(ctx, now); err != nil {
		t.Fatal(err)
	}
	if sent := fake.take(); len(sent) != 1 || sent[0]["text"] != "📶 <b>Aparelho novo na rede: X</b>" {
		t.Fatalf("com conversa bloqueada = %v", sent)
	}

	// Desconectar troca o código do link.
	if err := tg.RemoveChat(ctx, 88); err != nil || len(tg.Config().Chats) != 1 || tg.LinkURL() == link {
		t.Fatalf("remover conversa: %v %+v", err, tg.Config())
	}
	if err := tg.Test(ctx); err != nil || len(fake.take()) != 1 {
		t.Fatalf("teste: %v", err)
	}
	if err := tg.Remove(ctx); err != nil || tg.Active() || tg.LinkURL() != "" {
		t.Fatalf("remover bot: %v", err)
	}
}

func TestTelegramErrorHidesToken(t *testing.T) {
	ctx := context.Background()
	st, _ := store.OpenMemory()
	defer st.Close()
	tg, _ := NewTelegram(ctx, st, "http://127.0.0.1:1") // porta fechada
	err := tg.SetToken(ctx, "123:segredo")
	if err == nil || strings.Contains(err.Error(), "segredo") {
		t.Fatalf("erro vazou o token: %v", err)
	}
}
