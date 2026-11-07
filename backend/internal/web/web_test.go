package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/auth"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/collectors"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func newServer(t *testing.T) (*httptest.Server, *store.Store, *settings.Manager) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfg, err := settings.Load(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	dns := dnsserver.New(cfg.Get, nil)
	if err := dns.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dns.Shutdown)
	srv := httptest.NewServer(Handler(Deps{
		Version: "teste", Store: st, Settings: cfg, Auth: auth.New(st), DNS: dns,
		Dropped: func() uint64 { return 0 }, LAN: func() collectors.Status { return collectors.Status{} },
		HTTPPort: "8994",
	}))
	t.Cleanup(srv.Close)
	return srv, st, cfg
}

func newClient(t *testing.T) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func post(t *testing.T, c *http.Client, u string, form url.Values) *http.Response {
	t.Helper()
	resp, err := c.PostForm(u, form)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPanelFlow(t *testing.T) {
	srv, st, cfg := newServer(t)
	c := newClient(t)

	resp, err := c.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Request.URL.Path != "/setup" {
		t.Fatalf("sem senha deveria ir para /setup, foi para %s", resp.Request.URL.Path)
	}
	resp.Body.Close()

	resp = post(t, c, srv.URL+"/setup", url.Values{"password": {"senha-boa-123"}, "confirm": {"senha-boa-123"}})
	if resp.Request.URL.Path != "/" || !strings.Contains(body(t, resp), "Configurações") {
		t.Fatalf("setup deveria abrir o painel, status %d em %s", resp.StatusCode, resp.Request.URL.Path)
	}

	// Outro navegador sem sessão cai no login.
	anon := newClient(t)
	resp, _ = anon.Get(srv.URL + "/")
	resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("sem sessão deveria ir para /login, foi para %s", resp.Request.URL.Path)
	}
	resp = post(t, anon, srv.URL+"/settings", url.Values{"upstreams": {"9.9.9.9"}})
	if resp.Request.URL.Path != "/login" || cfg.Get().Upstreams[0] == "9.9.9.9" {
		t.Fatal("POST sem sessão não pode mudar configuração")
	}

	resp = post(t, c, srv.URL+"/settings", url.Values{
		"upstreams": {"9.9.9.9\n1.1.1.1"}, "max_client_ttl": {"30"}, "scan_interval_sec": {"60"},
		"retention_days": {"30"},
	})
	if !strings.Contains(body(t, resp), "Configurações salvas.") {
		t.Fatal("configuração deveria ser salva")
	}
	got := cfg.Get()
	if got.Upstreams[0] != "9.9.9.9" || got.MaxClientTTL != 30 || got.BlockBypass || got.RetentionDays != 30 {
		t.Fatalf("configuração = %+v", got)
	}
	resp = post(t, c, srv.URL+"/settings", url.Values{
		"upstreams": {"dns.google"}, "max_client_ttl": {"30"}, "scan_interval_sec": {"60"}, "retention_days": {"30"},
	})
	if !strings.Contains(body(t, resp), "Upstream inválido") {
		t.Fatal("upstream inválido deveria aparecer como erro")
	}

	ctx := context.Background()
	var id int64
	if err := st.DB.QueryRowContext(ctx, `INSERT INTO devices(key, first_seen, last_seen) VALUES('ip:192.168.1.9', 1, 1) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	resp = post(t, c, srv.URL+"/devices/"+itoa(id), url.Values{"action": {"save"}, "name": {"TV da sala"}})
	if !strings.Contains(body(t, resp), `value="TV da sala"`) {
		t.Fatal("nome do aparelho deveria aparecer no painel")
	}
	post(t, c, srv.URL+"/devices/"+itoa(id), url.Values{"action": {"hide"}})
	if devices, _ := st.DeviceCoverage(ctx, store.VisibleDevices); len(devices) != 0 {
		t.Fatal("aparelho ocultado não deveria aparecer na lista")
	}

	// Cross-origin POST com a sessão válida é recusado.
	req, _ := http.NewRequest("POST", srv.URL+"/devices/"+itoa(id), strings.NewReader("action=show"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST cross-site deveria dar 403, deu %d", resp.StatusCode)
	}

	resp = post(t, c, srv.URL+"/logout", nil)
	resp.Body.Close()
	resp, _ = c.Get(srv.URL + "/")
	resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatal("logout deveria encerrar a sessão")
	}
	resp = post(t, c, srv.URL+"/login", url.Values{"password": {"errada-123"}})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("senha errada deveria dar 401, deu %d", resp.StatusCode)
	}
}

func TestPairingAndAPI(t *testing.T) {
	srv, _, _ := newServer(t)
	c := newClient(t)
	post(t, c, srv.URL+"/setup", url.Values{"password": {"senha-boa-123"}, "confirm": {"senha-boa-123"}})

	resp, err := http.Get(srv.URL + "/api/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("API sem token deveria dar 401, deu %d", resp.StatusCode)
	}

	resp = post(t, c, srv.URL+"/pair", url.Values{"name": {"Celular"}})
	page := body(t, resp)
	if !strings.Contains(page, "<svg") {
		t.Fatal("pareamento deveria mostrar o QR code")
	}
	start := strings.Index(page, auth.TokenPrefix)
	if start < 0 {
		t.Fatal("token não apareceu na página de pareamento")
	}
	token := page[start : start+len(auth.TokenPrefix)+52]

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var status struct {
		Version string `json:"version"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&status) != nil || status.Version != "teste" {
		t.Fatalf("status com token: %d %+v", resp.StatusCode, status)
	}

	resp, err = c.Get(srv.URL + "/api/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API com sessão do painel deveria responder, deu %d", resp.StatusCode)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
