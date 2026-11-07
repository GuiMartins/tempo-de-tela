// Package settings guarda a configuração editável pela UI web, com leitura sem trava.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

type Settings struct {
	// Upstreams: "1.1.1.1", "8.8.8.8:53", "[2606:4700:4700::1111]:53" ou DoH "https://1.1.1.1/dns-query".
	Upstreams []string `json:"upstreams"`
	// MaxClientTTL limita o TTL entregue aos aparelhos (segundos). TTL curto deixa a
	// duração das sessões mais precisa.
	MaxClientTTL int `json:"max_client_ttl"`
	// BlockBypass responde "não existe" para domínios de DNS-over-HTTPS conhecidos e
	// para os domínios-sinal (Firefox, iCloud Private Relay), pra o aparelho usar o DNS da rede.
	BlockBypass bool `json:"block_bypass"`
	// ScanIntervalSec: intervalo da varredura da rede (ARP, mDNS, nomes).
	ScanIntervalSec int `json:"scan_interval_sec"`
	// RetentionDays: por quantos dias guardar as consultas brutas.
	RetentionDays int `json:"retention_days"`
	// Interface de rede da LAN. Vazio = a da rota padrão.
	Interface string `json:"interface"`
}

func Defaults() Settings {
	return Settings{
		Upstreams:       []string{"1.1.1.1", "8.8.8.8"},
		MaxClientTTL:    60,
		BlockBypass:     true,
		ScanIntervalSec: 120,
		RetentionDays:   90,
	}
}

func (s *Settings) Validate() error {
	if len(s.Upstreams) == 0 {
		return fmt.Errorf("informe pelo menos um DNS upstream")
	}
	clean := s.Upstreams[:0]
	for _, u := range s.Upstreams {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := validateUpstream(u); err != nil {
			return err
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return fmt.Errorf("informe pelo menos um DNS upstream")
	}
	s.Upstreams = clean
	if s.MaxClientTTL < 5 || s.MaxClientTTL > 86400 {
		return fmt.Errorf("TTL máximo deve ficar entre 5 e 86400 segundos")
	}
	if s.ScanIntervalSec < 30 || s.ScanIntervalSec > 3600 {
		return fmt.Errorf("intervalo de varredura deve ficar entre 30 e 3600 segundos")
	}
	if s.RetentionDays < 1 || s.RetentionDays > 3650 {
		return fmt.Errorf("retenção deve ficar entre 1 e 3650 dias")
	}
	s.Interface = strings.TrimSpace(s.Interface)
	return nil
}

func validateUpstream(u string) error {
	if strings.HasPrefix(u, "https://") {
		p, err := url.Parse(u)
		if err != nil || p.Host == "" {
			return fmt.Errorf("upstream DoH inválido: %q", u)
		}
		// Hostname em DoH faria o app depender de DNS pra achar o próprio upstream
		// (e os hosts DoH conhecidos ficam bloqueados pelo anti-contorno).
		if _, err := netip.ParseAddr(strings.Trim(p.Hostname(), "[]")); err != nil {
			return fmt.Errorf("upstream DoH precisa usar IP, não nome (ex.: https://1.1.1.1/dns-query): %q", u)
		}
		return nil
	}
	if _, err := netip.ParseAddr(u); err == nil {
		return nil
	}
	if _, err := netip.ParseAddrPort(u); err == nil {
		return nil
	}
	return fmt.Errorf("upstream inválido: %q (use IP, IP:porta ou https://IP/dns-query)", u)
}

const key = "settings"

// Manager mantém a configuração atual em memória e persiste no banco.
type Manager struct {
	st  *store.Store
	cur atomic.Pointer[Settings]
}

func Load(ctx context.Context, st *store.Store) (*Manager, error) {
	m := &Manager{st: st}
	s := Defaults()
	raw, ok, err := st.GetSetting(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("configuração salva inválida: %w", err)
		}
		if err := s.Validate(); err != nil {
			s = Defaults()
		}
	}
	m.cur.Store(&s)
	return m, nil
}

func (m *Manager) Get() Settings {
	s := *m.cur.Load()
	s.Upstreams = append([]string(nil), s.Upstreams...)
	return s
}

func (m *Manager) Save(ctx context.Context, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	if err := m.st.SetSetting(ctx, key, string(b)); err != nil {
		return err
	}
	m.cur.Store(&s)
	return nil
}
