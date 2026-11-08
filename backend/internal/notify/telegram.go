// Package notify manda os alertas por um bot do Telegram, pra chegarem na hora no celular (o app
// Android só busca a cada 15 min, o mínimo do WorkManager).
//
// O bot é do próprio usuário (criado no @BotFather); o token é colado no painel. Cada conversa se
// conecta abrindo o link t.me/<bot>?start=<código>: só quem tem o código recebe os alertas.
package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const (
	configKey = "telegram"
	cursorKey = "notify_last_event_id"
	// Alerta mais velho que isso não é enviado (Telegram ficou fora muito tempo).
	maxAge = time.Hour
	// APIURL é o endereço da API de bots do Telegram.
	APIURL = "https://api.telegram.org"
)

var errUnavailable = errors.New("Telegram indisponível")

// Chat é uma conversa que recebe os alertas.
type Chat struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Config fica salva no banco (settings), em JSON.
type Config struct {
	Token string `json:"token"`
	Bot   string `json:"bot"`  // usuário do bot, sem @
	Code  string `json:"code"` // código do link de conexão
	Chats []Chat `json:"chats"`
	// Offset é o próximo update_id a ler no getUpdates.
	Offset int64 `json:"offset"`
}

type Telegram struct {
	st     *store.Store
	api    string
	client *http.Client

	mu   sync.Mutex
	cfg  Config
	down bool
}

func NewTelegram(ctx context.Context, st *store.Store, api string) (*Telegram, error) {
	t := &Telegram{st: st, api: strings.TrimRight(api, "/"), client: &http.Client{Timeout: 15 * time.Second}}
	raw, ok, err := st.GetSetting(ctx, configKey)
	if err != nil {
		return nil, err
	}
	if ok {
		json.Unmarshal([]byte(raw), &t.cfg)
	}
	return t, nil
}

// Config devolve uma cópia da configuração atual.
func (t *Telegram) Config() Config {
	if t == nil {
		return Config{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.cfg
	c.Chats = append([]Chat(nil), t.cfg.Chats...)
	return c
}

// LinkURL é o link que conecta uma conversa ao bot ("" sem bot).
func (t *Telegram) LinkURL() string {
	c := t.Config()
	if c.Bot == "" {
		return ""
	}
	return "https://t.me/" + c.Bot + "?start=" + c.Code
}

// Active diz se há bot e pelo menos uma conversa recebendo os alertas.
func (t *Telegram) Active() bool {
	c := t.Config()
	return c.Token != "" && len(c.Chats) > 0
}

// SetToken confere o token com o Telegram e passa a usar o bot. Trocar de bot desconecta as conversas.
func (t *Telegram) SetToken(ctx context.Context, token string) error {
	if t == nil {
		return errUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " /?#") {
		return errors.New("token inválido")
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := t.call(ctx, token, "getMe", map[string]any{}, &me); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && (apiErr.Code == http.StatusUnauthorized || apiErr.Code == http.StatusNotFound) {
			return errors.New("o Telegram recusou o token (confira se copiou inteiro)")
		}
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if me.Username != t.cfg.Bot {
		t.cfg = Config{Bot: me.Username}
	}
	t.cfg.Token = token
	if t.cfg.Code == "" {
		t.cfg.Code = randomCode()
	}
	return t.saveLocked(ctx)
}

// Remove esquece o bot e as conversas.
func (t *Telegram) Remove(ctx context.Context) error {
	if t == nil {
		return errUnavailable
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cfg = Config{}
	return t.saveLocked(ctx)
}

// RemoveChat desconecta uma conversa; o link ganha código novo (o antigo deixa de valer).
func (t *Telegram) RemoveChat(ctx context.Context, id int64) error {
	if t == nil {
		return errUnavailable
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := make([]Chat, 0, len(t.cfg.Chats))
	for _, c := range t.cfg.Chats {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	if len(kept) == len(t.cfg.Chats) {
		return store.ErrNotFound
	}
	t.cfg.Chats = kept
	t.cfg.Code = randomCode()
	return t.saveLocked(ctx)
}

// Test manda uma mensagem de teste pra todas as conversas.
func (t *Telegram) Test(ctx context.Context) error {
	c := t.Config()
	if c.Token == "" || len(c.Chats) == 0 {
		return errors.New("nenhuma conversa conectada")
	}
	for _, chat := range c.Chats {
		if err := t.send(ctx, c.Token, chat.ID, "✅ <b>Teste do Tempo de Tela</b>\nOs alertas vão chegar aqui."); err != nil {
			return err
		}
	}
	return nil
}

// Run atende o bot e envia os alertas novos a cada 10 s até o contexto acabar.
func (t *Telegram) Run(ctx context.Context) {
	if t == nil {
		return
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		err := t.Poll(ctx)
		if err == nil {
			err = t.Flush(ctx, time.Now())
		}
		switch {
		case err != nil && ctx.Err() == nil && !t.down:
			t.down = true
			slog.Warn("falha ao falar com o Telegram (tento de novo a cada 10 s)", "erro", err)
		case err == nil && t.down:
			t.down = false
			slog.Info("Telegram de volta")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

type reply struct {
	chat int64
	text string
}

// Poll lê as mensagens mandadas ao bot e conecta quem abriu o link com o código certo.
func (t *Telegram) Poll(ctx context.Context) error {
	c := t.Config()
	if c.Token == "" {
		return nil
	}
	var updates []struct {
		UpdateID int64 `json:"update_id"`
		Message  *struct {
			Text string `json:"text"`
			Chat struct {
				ID        int64  `json:"id"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Username  string `json:"username"`
				Title     string `json:"title"`
			} `json:"chat"`
		} `json:"message"`
	}
	params := map[string]any{"offset": c.Offset, "timeout": 0, "allowed_updates": []string{"message"}}
	if err := t.call(ctx, c.Token, "getUpdates", params, &updates); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	var replies []reply
	t.mu.Lock()
	if t.cfg.Token != c.Token { // trocaram o bot no meio
		t.mu.Unlock()
		return nil
	}
	for _, u := range updates {
		t.cfg.Offset = max(t.cfg.Offset, u.UpdateID+1)
		m := u.Message
		if m == nil {
			continue
		}
		cmd, arg, _ := strings.Cut(strings.TrimSpace(m.Text), " ")
		switch {
		case cmd != "/start" && !strings.HasPrefix(cmd, "/start@"):
			replies = append(replies, reply{m.Chat.ID, "Este bot só manda os alertas do Tempo de Tela."})
		case strings.TrimSpace(arg) != t.cfg.Code:
			replies = append(replies, reply{m.Chat.ID, "Para receber os alertas, abra o link de conexão do painel do Tempo de Tela (Configurações)."})
		default:
			name := firstNonEmpty(m.Chat.Title, strings.TrimSpace(m.Chat.FirstName+" "+m.Chat.LastName), m.Chat.Username, strconv.FormatInt(m.Chat.ID, 10))
			known := false
			for _, existing := range t.cfg.Chats {
				known = known || existing.ID == m.Chat.ID
			}
			if !known {
				t.cfg.Chats = append(t.cfg.Chats, Chat{ID: m.Chat.ID, Name: name})
			}
			replies = append(replies, reply{m.Chat.ID, "✅ <b>Pronto!</b> Os alertas do Tempo de Tela vão chegar aqui."})
		}
	}
	err := t.saveLocked(ctx)
	t.mu.Unlock()
	if err != nil {
		return err
	}
	for _, r := range replies {
		t.send(ctx, c.Token, r.chat, r.text) // resposta de cortesia: falha não importa
	}
	return nil
}

// Flush envia, em ordem, os alertas depois do último já enviado. Para no primeiro erro de rede
// sem avançar (o próximo giro tenta de novo); conversa que bloqueou o bot é pulada.
func (t *Telegram) Flush(ctx context.Context, now time.Time) error {
	raw, ok, err := t.st.GetSetting(ctx, cursorKey)
	if err != nil {
		return err
	}
	c := t.Config()
	if !ok || c.Token == "" || len(c.Chats) == 0 {
		// Sem ninguém pra receber (ou primeira vez): acompanha o último alerta, sem acumular
		// histórico pra despejar quando alguém conectar.
		latest, err := t.st.Events(ctx, -1, 1)
		if err != nil {
			return err
		}
		id := "0"
		if len(latest) > 0 {
			id = strconv.FormatInt(latest[0].ID, 10)
		}
		if ok && raw == id {
			return nil
		}
		return t.st.SetSetting(ctx, cursorKey, id)
	}
	cursor, _ := strconv.ParseInt(raw, 10, 64)
	events, err := t.st.Events(ctx, cursor, 20)
	if err != nil {
		return err
	}
	for _, e := range events {
		if now.Sub(time.Unix(e.TS, 0)) <= maxAge {
			text := emoji(e.Type) + " <b>" + html.EscapeString(e.Title) + "</b>"
			if e.Message != "" {
				text += "\n" + html.EscapeString(e.Message)
			}
			for _, chat := range c.Chats {
				err := t.send(ctx, c.Token, chat.ID, text)
				var apiErr *apiError
				if errors.As(err, &apiErr) && (apiErr.Code == http.StatusForbidden || apiErr.Code == http.StatusBadRequest) {
					slog.Warn("conversa do Telegram não recebeu o alerta", "conversa", chat.Name, "erro", err)
					continue
				}
				if err != nil {
					return err
				}
			}
		}
		if err := t.st.SetSetting(ctx, cursorKey, strconv.FormatInt(e.ID, 10)); err != nil {
			return err
		}
	}
	return nil
}

func emoji(kind string) string {
	switch kind {
	case "usage":
		return "👀"
	case "limit":
		return "⏳"
	case "bedtime":
		return "😴"
	case "school":
		return "🏫"
	case "new_app":
		return "🆕"
	case "new_device":
		return "📶"
	}
	return "🔔"
}

func (t *Telegram) send(ctx context.Context, token string, chat int64, text string) error {
	return t.call(ctx, token, "sendMessage", map[string]any{
		"chat_id": chat, "text": text, "parse_mode": "HTML", "link_preview_options": map[string]bool{"is_disabled": true},
	}, nil)
}

type apiError struct {
	Code        int
	Description string
}

func (e *apiError) Error() string { return fmt.Sprintf("Telegram: %d %s", e.Code, e.Description) }

func (t *Telegram) call(ctx context.Context, token, method string, params, out any) error {
	body, _ := json.Marshal(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.api+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return errors.New("token inválido")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		// O erro do http traz a URL, que tem o token: nunca deixar ir pro log.
		return errors.New(strings.ReplaceAll(err.Error(), token, "<token>"))
	}
	defer resp.Body.Close()
	var r struct {
		OK          bool            `json:"ok"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("Telegram respondeu %s", resp.Status)
	}
	if !r.OK {
		return &apiError{Code: r.ErrorCode, Description: r.Description}
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

func (t *Telegram) saveLocked(ctx context.Context) error {
	b, _ := json.Marshal(t.cfg)
	return t.st.SetSetting(ctx, configKey, string(b))
}

func randomCode() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
