package web

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/auth"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

func TestUsageAndCatalogAPI(t *testing.T) {
	srv, st, _ := newServer(t)
	c := newClient(t)
	post(t, c, srv.URL+"/setup", url.Values{"password": {"senha-boa-123"}, "confirm": {"senha-boa-123"}})
	page := body(t, post(t, c, srv.URL+"/pair", url.Values{"name": {"Teste"}}))
	i := strings.Index(page, auth.TokenPrefix)
	token := page[i : i+len(auth.TokenPrefix)+52]
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	base := today.Add(10 * time.Hour)
	if now.Before(base.Add(time.Hour)) { // os testes rodam a qualquer hora: usa ontem se ainda é cedo
		today = today.AddDate(0, 0, -1)
		base = today.Add(10 * time.Hour)
	}
	ip := netip.MustParseAddr("192.168.1.40")
	for m := 0; m <= 20; m++ {
		st.RecordQuery(ctx, base.Add(time.Duration(m)*time.Minute), ip, "rr1---sn.googlevideo.com", 1, 0)
	}
	for m := 0; m < 3; m++ {
		st.RecordQuery(ctx, base.Add(time.Duration(30+m)*time.Minute), ip, "api.escola.exemplo.com.br", 1, 0)
	}
	devices, _ := st.DeviceCoverage(ctx, store.AllDevices)
	person, _ := st.CreatePerson(ctx, store.Person{Name: "Bia", Kind: "child", Monitoring: true}, now)
	if err := st.AssignDevice(ctx, devices[0].ID, &person.ID, now); err != nil {
		t.Fatal(err)
	}

	date := today.Format("2006-01-02")
	var sum usage.Summary
	if code := apiCall(t, srv.URL, token, "GET", "/api/v1/persons/"+strconv.FormatInt(person.ID, 10)+"/usage?date="+date, nil, &sum); code != http.StatusOK {
		t.Fatalf("uso da pessoa: %d", code)
	}
	if len(sum.Apps) != 1 || sum.Apps[0].ID != "youtube" || sum.TotalSec != 21*60 {
		t.Fatalf("resumo = %+v", sum)
	}

	var unclassified []usage.Unclassified
	apiCall(t, srv.URL, token, "GET", "/api/v1/catalog/unclassified", nil, &unclassified)
	if len(unclassified) != 1 || unclassified[0].Site != "exemplo.com.br" {
		t.Fatalf("não classificados = %+v", unclassified)
	}
	var created map[string]string
	code := apiCall(t, srv.URL, token, "POST", "/api/v1/catalog/rules",
		map[string]any{"domain": "exemplo.com.br", "new_app": map[string]string{"name": "Portal da Escola", "category": "study"}}, &created)
	if code != http.StatusCreated || created["app_id"] != "custom-portal-da-escola" {
		t.Fatalf("criar regra: %d %v", code, created)
	}
	apiCall(t, srv.URL, token, "GET", "/api/v1/catalog/unclassified", nil, &unclassified)
	if len(unclassified) != 0 {
		t.Fatalf("depois de classificar, não classificados = %+v", unclassified)
	}
	apiCall(t, srv.URL, token, "GET", "/api/v1/persons/"+strconv.FormatInt(person.ID, 10)+"/usage?date="+date, nil, &sum)
	if len(sum.Apps) != 2 || sum.Categories[0].ID != "video" {
		t.Fatalf("resumo com o app novo = %+v", sum)
	}
	if code := apiCall(t, srv.URL, token, "POST", "/api/v1/catalog/rules", map[string]any{"domain": "x.com", "app_id": "nao-existe"}, nil); code != http.StatusBadRequest {
		t.Fatalf("app inexistente deveria dar 400, deu %d", code)
	}
	var week usage.Summary
	apiCall(t, srv.URL, token, "GET", "/api/v1/devices/"+strconv.FormatInt(devices[0].ID, 10)+"/usage?days=7&date="+date, nil, &week)
	if len(week.Daily) != 7 || len(week.Heatmap) != 7 {
		t.Fatalf("semana do aparelho = %+v", week)
	}
	var todayList []map[string]any
	apiCall(t, srv.URL, token, "GET", "/api/v1/usage/today?date="+date, nil, &todayList)
	if len(todayList) != 1 || todayList[0]["total_sec"].(float64) < 21*60 {
		t.Fatalf("hoje = %+v", todayList)
	}
}
