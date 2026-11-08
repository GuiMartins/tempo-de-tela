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
	hits, err := h.Store.PersonHits(r.Context(), id, from, from.AddDate(0, 0, days))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	cat := h.Catalogs.Get()
	writeJSON(w, usage.Summarize(usage.Sessionize(hits, cat), cat, from, days))
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
	hits, err := h.Store.DeviceHits(r.Context(), id, from, from.AddDate(0, 0, days))
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	cat := h.Catalogs.Get()
	writeJSON(w, usage.Summarize(usage.Sessionize(hits, cat), cat, from, days))
}

type personToday struct {
	PersonID     int64            `json:"person_id"`
	TotalSec     int64            `json:"total_sec"`
	LastActivity int64            `json:"last_activity"`
	TopApps      []usage.AppUsage `json:"top_apps"`
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
		out = append(out, personToday{p.ID, sum.TotalSec, sum.LastActivity, top})
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
