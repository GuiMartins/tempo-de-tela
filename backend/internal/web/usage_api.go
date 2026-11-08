package web

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/alerts"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/catalog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

func (h *handler) usageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/catalog", h.api(h.getCatalog))
	mux.HandleFunc("GET /api/v1/catalog/unclassified", h.api(h.unclassified))
	mux.HandleFunc("POST /api/v1/catalog/rules", h.api(h.createRule))
	mux.HandleFunc("DELETE /api/v1/catalog/rules/{domain}", h.api(h.deleteRule))
	mux.HandleFunc("GET /api/v1/usage/today", h.api(h.usageToday))
	mux.HandleFunc("GET /api/v1/persons/{id}/usage", h.api(h.personUsage))
	mux.HandleFunc("GET /api/v1/devices/{id}/usage", h.api(h.deviceUsage))
	mux.HandleFunc("GET /api/v1/persons/{id}/rules", h.api(h.getRules))
	mux.HandleFunc("PUT /api/v1/persons/{id}/rules", h.api(h.putRules))
	mux.HandleFunc("GET /api/v1/events", h.api(h.listEvents))
	mux.HandleFunc("GET /api/v1/usage-alerts", h.api(h.listUsageAlerts))
	mux.HandleFunc("POST /api/v1/usage-alerts", h.api(h.createUsageAlert))
	mux.HandleFunc("DELETE /api/v1/usage-alerts/{id}", h.api(h.deleteUsageAlert))
}

func (h *handler) listUsageAlerts(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.UsageAlerts(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, list)
}

// createUsageAlert recebe {"app_id"} ou {"category"} (nenhum dos dois = qualquer uso) e
// {"device_id"} ou {"person_id"}.
func (h *handler) createUsageAlert(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppID    string `json:"app_id"`
		Category string `json:"category"`
		DeviceID *int64 `json:"device_id"`
		PersonID *int64 `json:"person_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	cat := h.Catalogs.Get()
	switch {
	case (body.DeviceID == nil) == (body.PersonID == nil):
		writeJSONError(w, http.StatusBadRequest, "escolha um aparelho ou uma pessoa")
		return
	case body.AppID != "" && body.Category != "":
		writeJSONError(w, http.StatusBadRequest, "escolha um app ou uma categoria, não os dois")
		return
	case body.AppID != "" && cat.App(body.AppID) == nil:
		writeJSONError(w, http.StatusBadRequest, "app não encontrado no catálogo")
		return
	}
	if body.Category != "" {
		if c := cat.Category(body.Category); c == nil || c.Ignore {
			writeJSONError(w, http.StatusBadRequest, "categoria inválida")
			return
		}
	}
	var err error
	if body.DeviceID != nil {
		_, err = h.Store.Device(ctx, *body.DeviceID)
	} else {
		_, err = h.Store.Person(ctx, *body.PersonID)
	}
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusBadRequest, map[bool]string{true: "aparelho não encontrado", false: "pessoa não encontrada"}[body.DeviceID != nil])
		return
	} else if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	created, err := h.Store.CreateUsageAlert(ctx, store.UsageAlert{
		AppID: body.AppID, Category: body.Category, DeviceID: body.DeviceID, PersonID: body.PersonID,
	}, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSONStatus(w, http.StatusCreated, created)
}

func (h *handler) deleteUsageAlert(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.Store.DeleteUsageAlert(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "aviso não encontrado")
		return
	} else if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) getRules(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.Store.Person(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	rules, err := alerts.Load(r.Context(), h.Store, id)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, rules)
}

func (h *handler) putRules(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.Store.Person(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	var rules alerts.Rules
	if !decode(w, r, &rules) {
		return
	}
	if err := rules.Validate(h.Catalogs.Get()); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := alerts.Save(r.Context(), h.Store, id, rules); err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	h.changed()
	writeJSON(w, rules)
}

// listEvents: ?after=ID devolve os alertas novos (para as notificações); sem after, os 50 mais recentes.
func (h *handler) listEvents(w http.ResponseWriter, r *http.Request) {
	after := int64(-1)
	if v := r.URL.Query().Get("after"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			after = n
		}
	}
	events, err := h.Store.Events(r.Context(), after, intParam(r, "limit", 50, 1, 200))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, events)
}

func (h *handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.Catalogs.Get())
}

// unclassified: ?days=7 (padrão) — sites fora do catálogo, dos mais consultados para os menos.
func (h *handler) unclassified(w http.ResponseWriter, r *http.Request) {
	days := intParam(r, "days", 7, 1, 90)
	uses, err := h.Store.DomainUses(r.Context(), time.Now().AddDate(0, 0, -days))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, usage.UnclassifiedSites(uses, h.Catalogs.Get(), 80))
}

var domainRe = regexp.MustCompile(`^([a-z0-9-]+\.)+[a-z0-9-]+$`)

// createRule recebe {"domain": "x.com", "app_id": "youtube"} ou, pra um app novo,
// {"domain": "x.com", "new_app": {"name": "Portal da escola", "category": "study"}}.
func (h *handler) createRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
		AppID  string `json:"app_id"`
		NewApp *struct {
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"new_app"`
	}
	if !decode(w, r, &body) {
		return
	}
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(body.Domain)), ".")
	if !domainRe.MatchString(domain) {
		writeJSONError(w, http.StatusBadRequest, "domínio inválido")
		return
	}
	cat := h.Catalogs.Get()
	ctx := r.Context()
	appID := body.AppID
	if body.NewApp != nil {
		name := strings.TrimSpace(body.NewApp.Name)
		if name == "" || utf8.RuneCountInString(name) > maxNameLen {
			writeJSONError(w, http.StatusBadRequest, "nome do app inválido")
			return
		}
		if cat.Category(body.NewApp.Category) == nil {
			writeJSONError(w, http.StatusBadRequest, "categoria inválida")
			return
		}
		appID = uniqueAppID(cat, name)
		if err := h.Store.CreateCatalogApp(ctx, store.CatalogApp{ID: appID, Name: name, Category: body.NewApp.Category}, time.Now()); err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
			return
		}
	} else if cat.App(appID) == nil {
		writeJSONError(w, http.StatusBadRequest, "app não encontrado no catálogo")
		return
	}
	if err := h.Store.SetCatalogRule(ctx, store.CatalogRule{Domain: domain, AppID: appID}, time.Now()); err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	if err := h.Catalogs.Reload(ctx); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "falha ao recarregar o catálogo")
		return
	}
	writeJSONStatus(w, http.StatusCreated, map[string]string{"domain": domain, "app_id": appID})
}

func (h *handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	err := h.Store.DeleteCatalogRule(r.Context(), strings.ToLower(r.PathValue("domain")))
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "regra não encontrada")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	if err := h.Catalogs.Reload(r.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "falha ao recarregar o catálogo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func uniqueAppID(cat *catalog.Catalog, name string) string {
	r := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "õ", "o", "ô", "o", "ú", "u", "ç", "c")
	slug := strings.Trim(slugRe.ReplaceAllString(r.Replace(strings.ToLower(name)), "-"), "-")
	if slug == "" {
		slug = "app"
	}
	id := "custom-" + slug
	for n := 2; cat.App(id) != nil; n++ {
		id = fmt.Sprintf("custom-%s-%d", slug, n)
	}
	return id
}

// period lê ?date=AAAA-MM-DD (padrão: hoje) e ?days=1..31 (padrão 1), no fuso local.
func period(r *http.Request) (time.Time, int, error) {
	days := intParam(r, "days", 1, 1, 31)
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if v := r.URL.Query().Get("date"); v != "" {
		d, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return day, days, err
		}
		day = d
	}
	// O período termina no dia pedido: days=7 são os 7 dias até ele.
	return day.AddDate(0, 0, 1-days), days, nil
}

func (h *handler) personUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.Store.Person(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	from, days, err := period(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "data inválida (use AAAA-MM-DD)")
		return
	}
	// Lê também o período anterior, para comparar ("esta semana vs a passada").
	hits, err := h.Store.PersonHits(r.Context(), id, from.AddDate(0, 0, -days), from.AddDate(0, 0, days))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, h.summaryWithPrevious(hits, from, days))
}

func (h *handler) summaryWithPrevious(hits []store.Hit, from time.Time, days int) usage.Summary {
	cat := h.Catalogs.Get()
	sessions := usage.Sessionize(hits, cat)
	sum := usage.Summarize(sessions, cat, from, days)
	// Período que ainda não acabou (hoje, esta semana) compara com o anterior até a mesma hora.
	previous := sessions
	if now := time.Now(); now.Before(from.AddDate(0, 0, days)) {
		cutoff := now.AddDate(0, 0, -days).Unix()
		previous = nil
		for _, s := range sessions {
			if s.Start < cutoff {
				s.End = min(s.End, cutoff)
				previous = append(previous, s)
			}
		}
	}
	sum.PreviousTotalSec = usage.Summarize(previous, cat, from.AddDate(0, 0, -days), days).TotalSec
	return sum
}

func (h *handler) deviceUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := h.Store.Device(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "aparelho não encontrado")
		return
	}
	from, days, err := period(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "data inválida (use AAAA-MM-DD)")
		return
	}
	hits, err := h.Store.DeviceHits(r.Context(), id, from.AddDate(0, 0, -days), from.AddDate(0, 0, days))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, h.summaryWithPrevious(hits, from, days))
}

type personToday struct {
	PersonID     int64            `json:"person_id"`
	TotalSec     int64            `json:"total_sec"`
	LastActivity int64            `json:"last_activity"`
	TopApps      []usage.AppUsage `json:"top_apps"`
	// NowApp é o app em uso agora (sessão que terminou há menos de 1 min além da folga).
	NowApp string `json:"now_app"`
	// Limits: uso de hoje das categorias com limite (e "total"), para as barras de progresso.
	Limits []limitUse `json:"limits"`
	// Block descreve o bloqueio valendo agora ("Internet pausada até 21:30"); vazio se nenhum.
	Block string `json:"block"`
	// BlockedToday: acessos (consultas) barrados hoje pelas regras, pra mostrar que o bloqueio pegou.
	BlockedToday int `json:"blocked_today"`
}

type limitUse struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	UsedSec  int64  `json:"used_sec"`
	LimitSec int64  `json:"limit_sec"`
}

// usageToday: tempo de tela de hoje de cada pessoa, com os 3 apps mais usados.
func (h *handler) usageToday(w http.ResponseWriter, r *http.Request) {
	persons, err := h.Store.Persons(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	from, _, _ := period(r)
	cat := h.Catalogs.Get()
	out := make([]personToday, 0, len(persons))
	for _, p := range persons {
		hits, err := h.Store.PersonHits(r.Context(), p.ID, from, from.AddDate(0, 0, 1))
		if err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
			return
		}
		sum := usage.Summarize(usage.Sessionize(hits, cat), cat, from, 1)
		top := sum.Apps
		if len(top) > 3 {
			top = top[:3]
		}
		item := personToday{PersonID: p.ID, TotalSec: sum.TotalSec, LastActivity: sum.LastActivity, TopApps: top, Limits: []limitUse{}}
		cutoff := time.Now().Add(-90 * time.Second).Unix()
		for _, s := range sum.Sessions {
			if s.End >= cutoff {
				item.NowApp = s.AppName
			}
		}
		rules, err := alerts.Load(r.Context(), h.Store, p.ID)
		if err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
			return
		}
		item.Limits = limitUses(rules, sum, cat)
		if h.Policy != nil {
			item.Block = h.Policy.Status(p.ID, time.Now())
		}
		if item.BlockedToday, err = h.Store.PersonBlockedCount(r.Context(), p.ID, from); err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
			return
		}
		out = append(out, item)
	}
	writeJSON(w, out)
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return min(max(n, lo), hi)
}

func limitUses(rules alerts.Rules, sum usage.Summary, cat *catalog.Catalog) []limitUse {
	out := []limitUse{}
	if m, ok := rules.Limits[alerts.TotalLimit]; ok {
		out = append(out, limitUse{alerts.TotalLimit, "Total", sum.TotalSec, int64(m) * 60})
	}
	for _, c := range cat.Categories {
		m, ok := rules.Limits[c.ID]
		if !ok {
			continue
		}
		var used int64
		for _, u := range sum.Categories {
			if u.ID == c.ID {
				used = u.Sec
			}
		}
		out = append(out, limitUse{c.ID, c.Label, used, int64(m) * 60})
	}
	return out
}
