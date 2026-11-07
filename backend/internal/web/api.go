package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/inventory"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const maxNameLen = 64

var deviceKinds = map[string]bool{
	"": true, inventory.KindPhone: true, inventory.KindTablet: true, inventory.KindComputer: true,
	inventory.KindTV: true, inventory.KindConsole: true, inventory.KindSpeaker: true, inventory.KindIoT: true,
	inventory.KindRouter: true, inventory.KindServer: true, inventory.KindOther: true,
}

var personKinds = map[string]bool{"child": true, "adult": true, "shared": true}

func (h *handler) apiRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/devices", h.api(h.listDevices))
	mux.HandleFunc("GET /api/v1/devices/{id}", h.api(h.getDevice))
	mux.HandleFunc("PATCH /api/v1/devices/{id}", h.api(h.patchDevice))
	mux.HandleFunc("PUT /api/v1/devices/{id}/person", h.api(h.assignDevice))
	mux.HandleFunc("GET /api/v1/persons", h.api(h.listPersons))
	mux.HandleFunc("POST /api/v1/persons", h.api(h.createPerson))
	mux.HandleFunc("GET /api/v1/persons/{id}", h.api(h.getPerson))
	mux.HandleFunc("PATCH /api/v1/persons/{id}", h.api(h.patchPerson))
	mux.HandleFunc("DELETE /api/v1/persons/{id}", h.api(h.deletePerson))
}

// listDevices: ?filter=all inclui os ocultos; ?filter=hidden lista só os ocultos.
func (h *handler) listDevices(w http.ResponseWriter, r *http.Request) {
	filter := store.VisibleDevices
	switch r.URL.Query().Get("filter") {
	case "all":
		filter = store.AllDevices
	case "hidden":
		filter = store.HiddenDevices
	}
	devices, err := h.Store.DeviceCoverage(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "inventário indisponível")
		return
	}
	writeJSON(w, devices)
}

func (h *handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.writeDevice(w, r, id, http.StatusOK)
}

func (h *handler) writeDevice(w http.ResponseWriter, r *http.Request, id int64, status int) {
	d, err := h.Store.Device(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "aparelho não encontrado")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSONStatus(w, status, d)
}

func (h *handler) patchDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Kind   *string `json:"kind"`
		Hidden *bool   `json:"hidden"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if utf8.RuneCountInString(name) > maxNameLen {
			writeJSONError(w, http.StatusBadRequest, "nome muito longo (máximo 64 caracteres)")
			return
		}
		body.Name = &name
	}
	if body.Kind != nil && !deviceKinds[*body.Kind] {
		writeJSONError(w, http.StatusBadRequest, "tipo de aparelho inválido")
		return
	}
	err := h.Store.UpdateDevice(r.Context(), id, store.DeviceUpdate{Name: body.Name, Kind: body.Kind, Hidden: body.Hidden})
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "aparelho não encontrado")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	h.writeDevice(w, r, id, http.StatusOK)
}

// assignDevice recebe {"person_id": 3} ou {"person_id": null} (sem dono).
func (h *handler) assignDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		PersonID *int64 `json:"person_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	err := h.Store.AssignDevice(r.Context(), id, body.PersonID, time.Now())
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "aparelho ou pessoa não encontrado")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	h.writeDevice(w, r, id, http.StatusOK)
}

func (h *handler) listPersons(w http.ResponseWriter, r *http.Request) {
	persons, err := h.Store.Persons(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSON(w, persons)
}

func (h *handler) getPerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.writePerson(w, r, id, http.StatusOK)
}

func (h *handler) writePerson(w http.ResponseWriter, r *http.Request, id int64, status int) {
	p, err := h.Store.Person(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSONStatus(w, status, p)
}

type personBody struct {
	Name       *string `json:"name"`
	Emoji      *string `json:"emoji"`
	Kind       *string `json:"kind"`
	Monitoring *bool   `json:"monitoring"`
}

// apply valida e copia os campos enviados para p.
func (b personBody) apply(p *store.Person) string {
	if b.Name != nil {
		p.Name = strings.TrimSpace(*b.Name)
	}
	if b.Emoji != nil {
		p.Emoji = strings.TrimSpace(*b.Emoji)
	}
	if b.Kind != nil {
		p.Kind = *b.Kind
	}
	if b.Monitoring != nil {
		p.Monitoring = *b.Monitoring
	}
	switch {
	case p.Name == "":
		return "informe o nome"
	case utf8.RuneCountInString(p.Name) > maxNameLen:
		return "nome muito longo (máximo 64 caracteres)"
	case utf8.RuneCountInString(p.Emoji) > 8:
		return "emoji inválido"
	case !personKinds[p.Kind]:
		return "tipo de pessoa inválido (child, adult ou shared)"
	}
	return ""
}

func (h *handler) createPerson(w http.ResponseWriter, r *http.Request) {
	var body personBody
	if !decode(w, r, &body) {
		return
	}
	p := store.Person{Kind: "child", Monitoring: true}
	if msg := body.apply(&p); msg != "" {
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}
	created, err := h.Store.CreatePerson(r.Context(), p, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	writeJSONStatus(w, http.StatusCreated, created)
}

func (h *handler) patchPerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body personBody
	if !decode(w, r, &body) {
		return
	}
	p, err := h.Store.Person(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	if msg := body.apply(&p); msg != "" {
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.Store.UpdatePerson(r.Context(), p); err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	h.writePerson(w, r, id, http.StatusOK)
}

func (h *handler) deletePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := h.Store.DeletePerson(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "pessoa não encontrada")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusNotFound, "registro não encontrado")
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
