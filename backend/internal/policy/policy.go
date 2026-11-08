// Package policy aplica no DNS os bloqueios de cada pessoa: internet pausada, hora de dormir,
// aula, categorias sempre bloqueadas e categorias que passaram do limite do dia.
//
// A decisão roda em toda consulta e só lê um retrato em memória (aparelho → pessoa → regras),
// refeito a cada 30 s e logo depois de qualquer mudança. Serviços do sistema (push, atualizações,
// horário) nunca são bloqueados, pra o aparelho continuar funcionando.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/alerts"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

// Categorias bloqueadas no horário de aula (quando a regra pede bloqueio).
var schoolBlocked = map[string]bool{"video": true, "social": true, "games": true, "music": true, "shopping": true}

// Reasons do bloqueio.
const (
	ReasonPause    = "pause"
	ReasonBedtime  = "bedtime"
	ReasonSchool   = "school"
	ReasonCategory = "category"
	ReasonLimit    = "limit"
)

type person struct {
	ID          int64
	Name        string
	PausedUntil int64
	Rules       alerts.Rules
	Exceeded    map[string]bool // categorias (ou "total") que passaram do limite hoje
}

type snapshot struct {
	byIP    map[netip.Addr]*person
	persons map[int64]*person
}

type Engine struct {
	st       *store.Store
	catalogs *usage.Catalogs
	snap     atomic.Pointer[snapshot]

	mu        sync.Mutex
	usageAt   time.Time
	exceeded  map[int64]map[string]bool
	refreshCh chan struct{}
}

func New(st *store.Store, catalogs *usage.Catalogs) *Engine {
	e := &Engine{st: st, catalogs: catalogs, refreshCh: make(chan struct{}, 1), exceeded: map[int64]map[string]bool{}}
	e.snap.Store(&snapshot{byIP: map[netip.Addr]*person{}, persons: map[int64]*person{}})
	return e
}

// Run refaz o retrato a cada 30 s, ou na hora quando alguém chama Changed.
func (e *Engine) Run(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		if err := e.Refresh(ctx, time.Now()); err != nil && ctx.Err() == nil {
			slog.Warn("falha ao atualizar os bloqueios", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-e.refreshCh:
		}
	}
}

// Changed pede um retrato novo (regras, pausa ou vínculo mudaram).
func (e *Engine) Changed() {
	e.mu.Lock()
	e.usageAt = time.Time{} // recalcula os limites também
	e.mu.Unlock()
	select {
	case e.refreshCh <- struct{}{}:
	default:
	}
}

// Refresh monta o retrato: IPs dos aparelhos vinculados → pessoa com regras e pausa.
func (e *Engine) Refresh(ctx context.Context, now time.Time) error {
	persons, err := e.st.Persons(ctx)
	if err != nil {
		return err
	}
	devices, err := e.st.DeviceCoverage(ctx, store.AllDevices)
	if err != nil {
		return err
	}
	exceeded, err := e.limitUsage(ctx, persons, now)
	if err != nil {
		return err
	}
	snap := &snapshot{byIP: map[netip.Addr]*person{}, persons: map[int64]*person{}}
	for _, p := range persons {
		rules, err := alerts.Load(ctx, e.st, p.ID)
		if err != nil {
			return err
		}
		snap.persons[p.ID] = &person{ID: p.ID, Name: p.Name, PausedUntil: p.PausedUntil, Rules: rules, Exceeded: exceeded[p.ID]}
	}
	for _, d := range devices {
		if d.PersonID == nil || d.System {
			continue
		}
		p := snap.persons[*d.PersonID]
		if p == nil {
			continue
		}
		for _, raw := range []string{d.IPv4, d.IPv6} {
			if ip, err := netip.ParseAddr(raw); err == nil {
				snap.byIP[ip.Unmap()] = p
			}
		}
	}
	e.snap.Store(snap)
	return nil
}

// limitUsage recalcula, no máximo a cada 2 min, quais limites cada pessoa já passou hoje
// (só para quem pediu bloqueio ao estourar o limite).
func (e *Engine) limitUsage(ctx context.Context, persons []store.Person, now time.Time) (map[int64]map[string]bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.usageAt.IsZero() && now.Sub(e.usageAt) < 2*time.Minute {
		return e.exceeded, nil
	}
	cat := e.catalogs.Get()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := map[int64]map[string]bool{}
	for _, p := range persons {
		rules, err := alerts.Load(ctx, e.st, p.ID)
		if err != nil {
			return nil, err
		}
		if !rules.BlockOnLimit || len(rules.Limits) == 0 {
			continue
		}
		hits, err := e.st.PersonHits(ctx, p.ID, day, day.AddDate(0, 0, 1))
		if err != nil {
			return nil, err
		}
		sum := usage.Summarize(usage.Sessionize(hits, cat), cat, day, 1)
		over := map[string]bool{}
		for c, minutes := range rules.Limits {
			used := sum.TotalSec
			if c != alerts.TotalLimit {
				used = 0
				for _, u := range sum.Categories {
					if u.ID == c {
						used = u.Sec
					}
				}
			}
			if used >= int64(minutes)*60 {
				over[c] = true
			}
		}
		out[p.ID] = over
	}
	e.exceeded, e.usageAt = out, now
	return out, nil
}

// Blocked é a Policy do servidor DNS.
func (e *Engine) Blocked(client netip.Addr, name string) bool {
	_, blocked := e.Decide(client, name, time.Now())
	return blocked
}

// Decide diz se a consulta do aparelho é bloqueada agora, e por quê.
func (e *Engine) Decide(client netip.Addr, name string, now time.Time) (string, bool) {
	p := e.snap.Load().byIP[client.Unmap()]
	if p == nil {
		return "", false
	}
	cat := e.catalogs.Get()
	category := ""
	if app := cat.Match(name); app != nil {
		category = app.Category
		if c := cat.Category(category); c != nil && c.Ignore {
			return "", false // sistema: push, atualizações, horário
		}
	}
	if full, reason := p.fullBlock(now); full {
		return reason, true
	}
	if p.Rules.School != nil && p.Rules.School.Block && p.Rules.School.Contains(now) && schoolBlocked[category] {
		return ReasonSchool, true
	}
	if category != "" {
		for _, c := range p.Rules.BlockCategories {
			if c == category {
				return ReasonCategory, true
			}
		}
		if p.Rules.BlockOnLimit && p.Exceeded[category] {
			return ReasonLimit, true
		}
	}
	return "", false
}

func (p *person) fullBlock(now time.Time) (bool, string) {
	switch {
	case p.PausedUntil == -1 || p.PausedUntil > now.Unix():
		return true, ReasonPause
	case p.Rules.Bedtime != nil && p.Rules.Bedtime.Block && p.Rules.Bedtime.Contains(now):
		return true, ReasonBedtime
	case p.Rules.BlockOnLimit && p.Exceeded[alerts.TotalLimit]:
		return true, ReasonLimit
	}
	return false, ""
}

// Status descreve o bloqueio da pessoa agora, para o app ("Internet pausada até 21:30").
func (e *Engine) Status(personID int64, now time.Time) string {
	p := e.snap.Load().persons[personID]
	if p == nil {
		return ""
	}
	if full, reason := p.fullBlock(now); full {
		switch reason {
		case ReasonPause:
			if p.PausedUntil == -1 {
				return "Internet pausada"
			}
			return "Internet pausada até " + time.Unix(p.PausedUntil, 0).In(now.Location()).Format("15:04")
		case ReasonBedtime:
			return "Hora de dormir: internet bloqueada até " + p.Rules.Bedtime.End
		case ReasonLimit:
			return "Passou do limite do dia: internet bloqueada"
		}
	}
	cat := e.catalogs.Get()
	var parts []string
	seen := map[string]bool{}
	add := func(id, why string) {
		if c := cat.Category(id); c != nil && !seen[id] {
			seen[id] = true
			parts = append(parts, c.Label+why)
		}
	}
	if p.Rules.School != nil && p.Rules.School.Block && p.Rules.School.Contains(now) {
		return "Horário de aula: vídeo, jogos e redes bloqueados"
	}
	for _, c := range p.Rules.BlockCategories {
		add(c, "")
	}
	if p.Rules.BlockOnLimit {
		for c := range p.Exceeded {
			add(c, " (limite)")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Bloqueado: %s", strings.Join(parts, ", "))
}
