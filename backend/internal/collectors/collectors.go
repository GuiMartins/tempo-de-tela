// Package collectors observa a LAN sem responder DHCP nem interceptar tráfego.
package collectors

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

type Status struct {
	Interface string `json:"interface"`
	ARP       string `json:"arp"`
	DHCP      string `json:"dhcp"`
	MDNS      string `json:"mdns"`
}

type Collectors struct {
	mu     sync.Mutex
	status Status
}

func (c *Collectors) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Collectors) update(change func(*Status)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	change(&c.status)
}

func Start(ctx context.Context, st *store.Store, cfg func() settings.Settings, enabled bool, interfaceName string) (*Collectors, <-chan struct{}) {
	c := &Collectors{status: Status{ARP: "desativado", DHCP: "desativado", MDNS: "desativado"}}
	done := make(chan struct{})
	if !enabled {
		close(done)
		return c, done
	}
	c.status = Status{ARP: "iniciando", DHCP: "iniciando", MDNS: "iniciando"}
	go func() {
		defer close(done)
		if err := c.run(ctx, st, cfg, interfaceName); err != nil {
			c.update(func(s *Status) {
				s.ARP, s.DHCP, s.MDNS = err.Error(), err.Error(), err.Error()
			})
			slog.Warn("coletores LAN indisponíveis", "erro", err)
		}
	}()
	return c, done
}

func scanInterval(cfg func() settings.Settings) time.Duration {
	return time.Duration(cfg().ScanIntervalSec) * time.Second
}
