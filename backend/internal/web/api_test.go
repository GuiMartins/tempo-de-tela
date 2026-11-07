package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/auth"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func apiCall(t *testing.T, srv, token, method, path string, body any, out any) int {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, srv+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestPersonsAndAssignmentsAPI(t *testing.T) {
	srv, st, _ := newServer(t)
	c := newClient(t)
	post(t, c, srv.URL+"/setup", url.Values{"password": {"senha-boa-123"}, "confirm": {"senha-boa-123"}})
	page := body(t, post(t, c, srv.URL+"/pair", url.Values{"name": {"Teste"}}))
	i := strings.Index(page, auth.TokenPrefix)
	token := page[i : i+len(auth.TokenPrefix)+52]

	var id int64
	st.DB.QueryRow(`INSERT INTO devices(key, first_seen, last_seen) VALUES('ip:192.168.1.7', 1, 1) RETURNING id`).Scan(&id)
	dev := "/api/v1/devices/" + strconv.FormatInt(id, 10)

	var person store.Person
	if code := apiCall(t, srv.URL, token, "POST", "/api/v1/persons", map[string]any{"name": "Helena", "emoji": "🦄"}, &person); code != http.StatusCreated {
		t.Fatalf("criar pessoa: %d", code)
	}
	if person.Kind != "child" || !person.Monitoring {
		t.Fatalf("padrões da pessoa = %+v", person)
	}
	if code := apiCall(t, srv.URL, token, "POST", "/api/v1/persons", map[string]any{"name": " "}, nil); code != http.StatusBadRequest {
		t.Fatalf("pessoa sem nome deveria dar 400, deu %d", code)
	}
	if code := apiCall(t, srv.URL, token, "POST", "/api/v1/persons", map[string]any{"name": "X", "kind": "pet"}, nil); code != http.StatusBadRequest {
		t.Fatalf("tipo inválido deveria dar 400, deu %d", code)
	}

	var device store.DeviceCoverage
	if code := apiCall(t, srv.URL, token, "PATCH", dev, map[string]any{"name": "Tablet", "kind": "tablet"}, &device); code != http.StatusOK {
		t.Fatalf("editar aparelho: %d", code)
	}
	if device.Name != "Tablet" || device.Kind != "tablet" {
		t.Fatalf("aparelho = %+v", device)
	}
	if code := apiCall(t, srv.URL, token, "PATCH", dev, map[string]any{"kind": "geladeira"}, nil); code != http.StatusBadRequest {
		t.Fatalf("tipo de aparelho inválido deveria dar 400, deu %d", code)
	}

	if code := apiCall(t, srv.URL, token, "PUT", dev+"/person", map[string]any{"person_id": person.ID}, &device); code != http.StatusOK {
		t.Fatalf("vincular: %d", code)
	}
	if device.PersonID == nil || *device.PersonID != person.ID {
		t.Fatalf("dono = %v", device.PersonID)
	}
	if code := apiCall(t, srv.URL, token, "GET", "/api/v1/persons/"+strconv.FormatInt(person.ID, 10), nil, &person); code != http.StatusOK || len(person.DeviceIDs) != 1 {
		t.Fatalf("pessoa após vínculo: %d %+v", code, person)
	}
	if code := apiCall(t, srv.URL, token, "PATCH", "/api/v1/persons/"+strconv.FormatInt(person.ID, 10), map[string]any{"monitoring": false}, &person); code != http.StatusOK || person.Monitoring || person.Name != "Helena" {
		t.Fatalf("pausar monitoramento: %d %+v", code, person)
	}
	if code := apiCall(t, srv.URL, token, "PUT", dev+"/person", map[string]any{"person_id": nil}, &device); code != http.StatusOK || device.PersonID != nil {
		t.Fatalf("desvincular: %d %v", code, device.PersonID)
	}
	if code := apiCall(t, srv.URL, token, "PUT", dev+"/person", map[string]any{"person_id": 999}, nil); code != http.StatusNotFound {
		t.Fatalf("pessoa inexistente deveria dar 404, deu %d", code)
	}

	if code := apiCall(t, srv.URL, token, "PATCH", dev, map[string]any{"hidden": true}, &device); code != http.StatusOK || !device.Hidden {
		t.Fatalf("ocultar: %d", code)
	}
	var list []store.DeviceCoverage
	apiCall(t, srv.URL, token, "GET", "/api/v1/devices", nil, &list)
	if len(list) != 0 {
		t.Fatal("lista padrão não deveria trazer ocultos")
	}
	apiCall(t, srv.URL, token, "GET", "/api/v1/devices?filter=all", nil, &list)
	if len(list) != 1 {
		t.Fatal("filter=all deveria trazer os ocultos")
	}

	if code := apiCall(t, srv.URL, token, "DELETE", "/api/v1/persons/"+strconv.FormatInt(person.ID, 10), nil, nil); code != http.StatusNoContent {
		t.Fatalf("apagar pessoa: %d", code)
	}
	var persons []store.Person
	apiCall(t, srv.URL, token, "GET", "/api/v1/persons", nil, &persons)
	if len(persons) != 0 {
		t.Fatalf("pessoas = %+v", persons)
	}
}
