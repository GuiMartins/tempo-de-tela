// Package notify publica os alertas num ntfy (https://ntfy.sh) da casa, pra chegarem no celular
// na hora, sem esperar o app buscar (o WorkManager só roda a cada 15 min).
package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const (
	topicKey  = "ntfy_topic"
	cursorKey = "ntfy_last_event_id"
	// Alerta mais velho que isso não é publicado (ntfy ficou fora muito tempo).
	maxAge = time.Hour
)

type Ntfy struct {
	st     *store.Store
	base   string // URL do servidor ntfy, sem barra no fim
	topic  string
	client *http.Client
	down   bool
}

// New liga o ntfy em base (ex.: "http://100.97.73.4:8996"); vazio = desligado (devolve nil).
// O tópico é gerado aleatório na primeira vez e fica salvo: ninguém da casa adivinha o nome.
func New(ctx context.Context, st *store.Store, base string) (*Ntfy, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil, nil
	}
	if u, err := url.Parse(base); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("endereço do ntfy inválido: %q", base)
	}
	topic, ok, err := st.GetSetting(ctx, topicKey)
	if err != nil {
		return nil, err
	}
	if !ok || topic == "" {
		b := make([]byte, 12)
		rand.Read(b)
		topic = "tdt-" + hex.EncodeToString(b)
		if err := st.SetSetting(ctx, topicKey, topic); err != nil {
			return nil, err
		}
	}
	return &Ntfy{st: st, base: base, topic: topic, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

// SubscribeURL é o endereço do tópico para assinar no app ntfy ("" se desligado).
func (n *Ntfy) SubscribeURL() string {
	if n == nil {
		return ""
	}
	return n.base + "/" + n.topic
}

// Run publica os alertas novos a cada 10 s até o contexto acabar.
func (n *Ntfy) Run(ctx context.Context) {
	if n == nil {
		return
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		err := n.Flush(ctx, time.Now())
		switch {
		case err != nil && ctx.Err() == nil && !n.down:
			n.down = true
			slog.Warn("falha ao publicar alerta no ntfy (tento de novo a cada 10 s)", "erro", err)
		case err == nil && n.down:
			n.down = false
			slog.Info("ntfy de volta")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Flush publica, em ordem, os alertas depois do último já enviado. Para no primeiro erro, sem
// avançar: o próximo giro tenta de novo.
func (n *Ntfy) Flush(ctx context.Context, now time.Time) error {
	raw, ok, err := n.st.GetSetting(ctx, cursorKey)
	if err != nil {
		return err
	}
	if !ok {
		// Primeira vez: começa do alerta mais recente, sem despejar o histórico.
		latest, err := n.st.Events(ctx, -1, 1)
		if err != nil {
			return err
		}
		var cursor int64
		if len(latest) > 0 {
			cursor = latest[0].ID
		}
		return n.st.SetSetting(ctx, cursorKey, strconv.FormatInt(cursor, 10))
	}
	cursor, _ := strconv.ParseInt(raw, 10, 64)
	events, err := n.st.Events(ctx, cursor, 20)
	if err != nil {
		return err
	}
	for _, e := range events {
		if now.Sub(time.Unix(e.TS, 0)) <= maxAge {
			if err := n.publish(ctx, e); err != nil {
				return err
			}
		}
		if err := n.st.SetSetting(ctx, cursorKey, strconv.FormatInt(e.ID, 10)); err != nil {
			return err
		}
	}
	return nil
}

// Emoji do ntfy (https://docs.ntfy.sh/emojis/) por tipo de alerta.
var tags = map[string]string{
	"usage":      "eyes",
	"limit":      "hourglass_flowing_sand",
	"bedtime":    "zzz",
	"school":     "school",
	"new_app":    "new",
	"new_device": "signal_strength",
}

func (n *Ntfy) publish(ctx context.Context, e store.Event) error {
	msg := map[string]any{"topic": n.topic, "title": e.Title, "message": e.Message}
	if e.Message == "" {
		msg["title"], msg["message"] = "Tempo de Tela", e.Title
	}
	if t, ok := tags[e.Type]; ok {
		msg["tags"] = []string{t}
	}
	body, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.base, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ntfy respondeu %s", resp.Status)
	}
	return nil
}
