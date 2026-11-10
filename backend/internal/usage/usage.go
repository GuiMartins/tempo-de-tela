// Package usage transforma consultas DNS em sessões de uso por app e resume o tempo de tela.
//
// A duração é estimada: consultas do mesmo app no mesmo aparelho formam uma sessão enquanto o
// intervalo entre elas for menor que o gap da categoria; a sessão termina 1 min depois da última
// consulta (o DNS do app entrega TTL de no máximo 60 s, então quem está usando pergunta de novo).
// Uma consulta isolada não vira sessão: costuma ser o app sincronizando em segundo plano.
package usage

import (
	"context"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/catalog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const (
	tail       = 60 // segundos contados depois da última consulta
	minQueries = 2
)

// Catalogs mantém o catálogo atual (embutido + regras do usuário) e recarrega quando muda.
type Catalogs struct {
	st  *store.Store
	cur atomic.Pointer[catalog.Catalog]
}

func NewCatalogs(ctx context.Context, st *store.Store) (*Catalogs, error) {
	c := &Catalogs{st: st}
	return c, c.Reload(ctx)
}

func (c *Catalogs) Get() *catalog.Catalog { return c.cur.Load() }

func (c *Catalogs) Reload(ctx context.Context) error {
	apps, err := c.st.CatalogApps(ctx)
	if err != nil {
		return err
	}
	rules, err := c.st.CatalogRules(ctx)
	if err != nil {
		return err
	}
	custom := make([]catalog.App, len(apps))
	for i, a := range apps {
		custom[i] = catalog.App{ID: a.ID, Name: a.Name, Category: a.Category}
	}
	rs := make([]catalog.Rule, len(rules))
	for i, r := range rules {
		rs[i] = catalog.Rule{Domain: r.Domain, AppID: r.AppID}
	}
	cat, err := catalog.Build(custom, rs)
	if err != nil {
		return err
	}
	c.cur.Store(cat)
	return nil
}

type Session struct {
	AppID    string
	DeviceID int64
	Start    int64
	End      int64
	Queries  int
}

// Sessionize agrupa as consultas (em ordem de tempo) em sessões por aparelho e app. Categorias
// ignoradas (sistema) ficam de fora: não são tempo de tela.
func Sessionize(hits []store.Hit, cat *catalog.Catalog) []Session {
	return sessionize(hits, cat, false)
}

// SessionizeAll é o Sessionize com as categorias ignoradas também, pros avisos de uso de apps
// que não contam como tempo de tela (ex.: pesquisa no Google no navegador da TV).
func SessionizeAll(hits []store.Hit, cat *catalog.Catalog) []Session {
	return sessionize(hits, cat, true)
}

func sessionize(hits []store.Hit, cat *catalog.Catalog, includeIgnored bool) []Session {
	type key struct {
		device int64
		app    string
	}
	open := map[key]*Session{}
	var out []Session
	closeSession := func(s *Session) {
		if s.Queries >= minQueries {
			s.End += tail
			out = append(out, *s)
		}
	}
	for _, h := range hits {
		app := cat.Match(h.Domain)
		if app == nil {
			continue
		}
		category := cat.Category(app.Category)
		if category == nil || (category.Ignore && !includeIgnored) {
			continue
		}
		k := key{h.DeviceID, app.ID}
		gap := int64(category.GapMin) * 60
		if s := open[k]; s != nil && h.TS-s.End <= gap {
			s.End = h.TS
			s.Queries++
			continue
		} else if s != nil {
			closeSession(s)
		}
		open[k] = &Session{AppID: app.ID, DeviceID: h.DeviceID, Start: h.TS, End: h.TS, Queries: 1}
	}
	for _, s := range open {
		closeSession(s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

type AppUsage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Sec      int64  `json:"sec"`
	Sessions int    `json:"sessions"`
}

type CategoryUsage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Sec   int64  `json:"sec"`
}

type DayUsage struct {
	Date string `json:"date"`
	Sec  int64  `json:"sec"`
}

type SessionView struct {
	AppID    string `json:"app_id"`
	AppName  string `json:"app_name"`
	Category string `json:"category"`
	DeviceID int64  `json:"device_id"`
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
}

// Summary é o resumo do período. Heatmap: minutos de uso por dia da semana (0 = segunda) e hora.
type Summary struct {
	From          int64           `json:"from"`
	To            int64           `json:"to"`
	Days          int             `json:"days"`
	TotalSec      int64           `json:"total_sec"`
	FirstActivity int64           `json:"first_activity"`
	LastActivity  int64           `json:"last_activity"`
	Categories    []CategoryUsage `json:"categories"`
	Apps          []AppUsage      `json:"apps"`
	Daily         []DayUsage      `json:"daily"`
	Heatmap       [][]int         `json:"heatmap,omitempty"`
	Sessions      []SessionView   `json:"sessions,omitempty"`
	Estimated     bool            `json:"estimated"`
	// PreviousTotalSec é o total do período anterior de mesmo tamanho (ontem, semana passada).
	PreviousTotalSec int64 `json:"previous_total_sec"`
}

type interval struct{ start, end int64 }

// Summarize resume as sessões no período [from, from + days). O tempo total e o de cada
// categoria são a união dos intervalos: dois apps abertos ao mesmo tempo não somam em dobro.
func Summarize(sessions []Session, cat *catalog.Catalog, from time.Time, days int) Summary {
	to := from.AddDate(0, 0, days)
	sum := Summary{From: from.Unix(), To: to.Unix(), Days: days, Estimated: true,
		Categories: []CategoryUsage{}, Apps: []AppUsage{}, Daily: []DayUsage{}}

	var all []interval
	byApp := map[string][]interval{}
	sessionsByApp := map[string]int{}
	byCategory := map[string][]interval{}
	for _, s := range sessions {
		iv, ok := clip(interval{s.Start, s.End}, from.Unix(), to.Unix())
		if !ok {
			continue
		}
		app := cat.App(s.AppID)
		if app == nil {
			continue
		}
		all = append(all, iv)
		byApp[app.ID] = append(byApp[app.ID], iv)
		sessionsByApp[app.ID]++
		byCategory[app.Category] = append(byCategory[app.Category], iv)
		if days == 1 {
			sum.Sessions = append(sum.Sessions, SessionView{app.ID, app.Name, app.Category, s.DeviceID, iv.start, iv.end})
		}
	}
	merged := union(all)
	sum.TotalSec = total(merged)
	if len(merged) > 0 {
		sum.FirstActivity, sum.LastActivity = merged[0].start, merged[len(merged)-1].end
	}
	for id, ivs := range byApp {
		app := cat.App(id)
		sum.Apps = append(sum.Apps, AppUsage{app.ID, app.Name, app.Category, total(union(ivs)), sessionsByApp[id]})
	}
	sort.Slice(sum.Apps, func(i, j int) bool {
		if sum.Apps[i].Sec != sum.Apps[j].Sec {
			return sum.Apps[i].Sec > sum.Apps[j].Sec
		}
		return sum.Apps[i].Name < sum.Apps[j].Name
	})
	for id, ivs := range byCategory {
		label := id
		if c := cat.Category(id); c != nil {
			label = c.Label
		}
		sum.Categories = append(sum.Categories, CategoryUsage{id, label, total(union(ivs))})
	}
	sort.Slice(sum.Categories, func(i, j int) bool { return sum.Categories[i].Sec > sum.Categories[j].Sec })

	for d := 0; d < days; d++ {
		dayStart := from.AddDate(0, 0, d)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var secs int64
		for _, iv := range merged {
			if c, ok := clip(iv, dayStart.Unix(), dayEnd.Unix()); ok {
				secs += c.end - c.start
			}
		}
		sum.Daily = append(sum.Daily, DayUsage{dayStart.Format("2006-01-02"), secs})
	}
	if days > 1 {
		sum.Heatmap = heatmap(merged, from.Location())
	}
	return sum
}

func heatmap(merged []interval, loc *time.Location) [][]int {
	grid := make([][]int, 7)
	for i := range grid {
		grid[i] = make([]int, 24)
	}
	for _, iv := range merged {
		t := iv.start
		for t < iv.end {
			at := time.Unix(t, 0).In(loc)
			hourEnd := time.Date(at.Year(), at.Month(), at.Day(), at.Hour()+1, 0, 0, 0, loc).Unix()
			end := min(hourEnd, iv.end)
			weekday := (int(at.Weekday()) + 6) % 7 // segunda = 0
			grid[weekday][at.Hour()] += int((end - t + 30) / 60)
			t = end
		}
	}
	return grid
}

func clip(iv interval, from, to int64) (interval, bool) {
	iv.start, iv.end = max(iv.start, from), min(iv.end, to)
	return iv, iv.end > iv.start
}

func union(ivs []interval) []interval {
	if len(ivs) == 0 {
		return nil
	}
	sorted := append([]interval(nil), ivs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start < sorted[j].start })
	out := []interval{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &out[len(out)-1]
		if iv.start <= last.end {
			last.end = max(last.end, iv.end)
		} else {
			out = append(out, iv)
		}
	}
	return out
}

func total(ivs []interval) int64 {
	var s int64
	for _, iv := range ivs {
		s += iv.end - iv.start
	}
	return s
}

// Unclassified é um site que os aparelhos acessam e que não está no catálogo.
type Unclassified struct {
	Site     string   `json:"site"`
	Count    int      `json:"count"`
	Devices  int      `json:"devices"`
	Examples []string `json:"examples"`
}

// UnclassifiedSites agrupa por site os domínios fora do catálogo, dos mais consultados para os menos.
func UnclassifiedSites(uses []store.DomainUse, cat *catalog.Catalog, limit int) []Unclassified {
	bySite := map[string]*Unclassified{}
	for _, u := range uses {
		// Nome sem ponto é da rede local (o Mac procurando "macbook"), não um site.
		if cat.Match(u.Domain) != nil || strings.HasSuffix(u.Domain, ".arpa") || !strings.Contains(u.Domain, ".") {
			continue
		}
		site, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimSuffix(strings.ToLower(u.Domain), "."))
		if err != nil {
			site = u.Domain
		}
		e := bySite[site]
		if e == nil {
			e = &Unclassified{Site: site}
			bySite[site] = e
		}
		e.Count += u.Count
		e.Devices = max(e.Devices, u.Devices)
		if len(e.Examples) < 3 && u.Domain != site {
			e.Examples = append(e.Examples, u.Domain)
		}
	}
	out := make([]Unclassified, 0, len(bySite))
	for _, e := range bySite {
		if e.Examples == nil {
			e.Examples = []string{}
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Site < out[j].Site
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
