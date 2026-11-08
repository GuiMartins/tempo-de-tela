// Package web serve o painel de configuração e a API REST.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/auth"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/collectors"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/policy"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

//go:embed templates/*.html
var templateFS embed.FS

var pages = template.Must(template.New("").Funcs(template.FuncMap{
	"formatTime": formatTime,
	"icon":       icon,
	"num":        num,
}).ParseFS(templateFS, "templates/*.html"))

const sessionCookie = "tdt_session"

type Deps struct {
	Version  string
	Store    *store.Store
	Settings *settings.Manager
	Auth     *auth.Auth
	DNS      *dnsserver.Server
	Dropped  func() uint64
	LAN      func() collectors.Status
	// HTTPPort entra nos endereços do QR de pareamento quando o Host da requisição não traz porta.
	HTTPPort string
	// Started é a hora em que o serviço subiu (mostrada como "no ar há…").
	Started time.Time
	// Catalogs é o catálogo domínio → app → categoria (embutido + regras do usuário).
	Catalogs *usage.Catalogs
	// Policy aplica os bloqueios no DNS; nil nos testes que não precisam.
	Policy *policy.Engine
}

// changed avisa o motor de bloqueio que pessoas, regras ou vínculos mudaram.
func (h *handler) changed() {
	if h.Policy != nil {
		h.Policy.Changed()
	}
}

type handler struct{ Deps }

func Handler(d Deps) http.Handler {
	h := &handler{d}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /setup", h.setupPage)
	mux.HandleFunc("POST /setup", h.setup)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.login)

	mux.HandleFunc("GET /{$}", h.panel(h.overview))
	mux.HandleFunc("GET /devices", h.panel(h.devicesPage))
	mux.HandleFunc("GET /settings", h.panel(h.settingsPage))
	mux.HandleFunc("GET /pairing", h.panel(h.pairingPage))
	mux.HandleFunc("GET /security", h.panel(h.securityPage))
	mux.HandleFunc("POST /logout", h.panel(h.logout))
	mux.HandleFunc("POST /settings", h.panel(h.saveSettings))
	mux.HandleFunc("POST /password", h.panel(h.changePassword))
	mux.HandleFunc("POST /devices/{id}", h.panel(h.updateDevice))
	mux.HandleFunc("POST /pair", h.panel(h.pair))
	mux.HandleFunc("POST /tokens/{id}/delete", h.panel(h.deleteToken))

	mux.HandleFunc("GET /api/v1/status", h.api(h.status))
	h.apiRoutes(mux)
	h.usageRoutes(mux)
	return http.NewCrossOriginProtection().Handler(mux)
}

// panel exige sessão; sem senha definida, manda para a configuração inicial.
func (h *handler) panel(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, err := h.hasSession(r)
		if err != nil {
			http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
			return
		}
		if ok {
			next(w, r)
			return
		}
		has, err := h.Auth.HasPassword(r.Context())
		if err != nil {
			http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
			return
		}
		if !has {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// api aceita o token de pareamento (Authorization: Bearer) ou a sessão do painel.
func (h *handler) api(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		var err error
		if token, bearer := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); bearer {
			ok, err = h.Auth.APIToken(r.Context(), strings.TrimSpace(token))
		} else {
			ok, err = h.hasSession(r)
		}
		if err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "banco de dados indisponível")
			return
		}
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tempo-de-tela"`)
			writeJSONError(w, http.StatusUnauthorized, "token inválido ou ausente")
			return
		}
		next(w, r)
	}
}

func (h *handler) hasSession(r *http.Request) (bool, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false, nil
	}
	return h.Auth.Session(r.Context(), c.Value)
}

func setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(auth.SessionTTL / time.Second),
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

type authPage struct {
	Setup bool
	Err   string
}

func (h *handler) setupPage(w http.ResponseWriter, r *http.Request) {
	has, err := h.Auth.HasPassword(r.Context())
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	if has {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	render(w, http.StatusOK, "login.html", authPage{Setup: true})
}

func (h *handler) setup(w http.ResponseWriter, r *http.Request) {
	password := r.PostFormValue("password")
	if password != r.PostFormValue("confirm") {
		render(w, http.StatusBadRequest, "login.html", authPage{Setup: true, Err: "As senhas não conferem."})
		return
	}
	token, err := h.Auth.Setup(r.Context(), password)
	switch {
	case errors.Is(err, auth.ErrPasswordSet):
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	case errors.Is(err, auth.ErrWeakPassword):
		render(w, http.StatusBadRequest, "login.html", authPage{Setup: true, Err: capitalize(err.Error()) + "."})
		return
	case err != nil:
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	setSession(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if ok, _ := h.hasSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if has, err := h.Auth.HasPassword(r.Context()); err == nil && !has {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	render(w, http.StatusOK, "login.html", authPage{})
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	client := clientAddr(r)
	token, err := h.Auth.Login(r.Context(), client, r.PostFormValue("password"))
	var locked auth.LockedError
	switch {
	case errors.Is(err, auth.ErrBadPassword):
		render(w, http.StatusUnauthorized, "login.html", authPage{Err: "Senha incorreta."})
		return
	case errors.As(err, &locked):
		render(w, http.StatusTooManyRequests, "login.html", authPage{Err: capitalize(err.Error()) + "."})
		return
	case err != nil:
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	setSession(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h.Auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type dnsStats struct {
	Addr      string `json:"addr"`
	Queries   uint64 `json:"queries"`
	CacheHits uint64 `json:"cache_hits"`
	Blocked   uint64 `json:"blocked"`
	// PolicyBlocked: consultas barradas pelas regras das pessoas (pausa, hora de dormir…).
	PolicyBlocked uint64 `json:"policy_blocked"`
	Failures      uint64 `json:"failures"`
	LastError     string `json:"last_error"`
}

func (h *handler) dnsStats() dnsStats {
	s := &h.DNS.Stats
	return dnsStats{
		Addr: h.DNS.LocalAddr().String(), Queries: s.Queries.Load(), CacheHits: s.CacheHits.Load(),
		Blocked: s.Blocked.Load(), PolicyBlocked: s.PolicyBlocked.Load(), Failures: s.Failures.Load(), LastError: s.LastError(),
	}
}

func (h *handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	s := h.Settings.Get()
	s.Upstreams = strings.FieldsFunc(r.PostFormValue("upstreams"), func(c rune) bool {
		return c == '\n' || c == '\r' || c == ',' || c == ' ' || c == '\t'
	})
	ints := []struct {
		field, label string
		dst          *int
	}{
		{"max_client_ttl", "TTL máximo", &s.MaxClientTTL},
		{"scan_interval_sec", "intervalo da varredura", &s.ScanIntervalSec},
		{"retention_days", "retenção", &s.RetentionDays},
	}
	for _, f := range ints {
		n, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(f.field)))
		if err != nil {
			back(w, r, "settings", "", "Valor inválido em "+f.label+".")
			return
		}
		*f.dst = n
	}
	s.BlockBypass = r.PostFormValue("block_bypass") != ""
	s.Interface = r.PostFormValue("interface")
	if err := h.Settings.Save(r.Context(), s); err != nil {
		back(w, r, "settings", "", capitalize(err.Error())+".")
		return
	}
	back(w, r, "settings", "Configurações salvas.", "")
}

func (h *handler) changePassword(w http.ResponseWriter, r *http.Request) {
	next := r.PostFormValue("new")
	if next != r.PostFormValue("confirm") {
		back(w, r, "security", "", "As senhas novas não conferem.")
		return
	}
	token, err := h.Auth.ChangePassword(r.Context(), r.PostFormValue("current"), next)
	switch {
	case errors.Is(err, auth.ErrBadPassword):
		back(w, r, "security", "", "Senha atual incorreta.")
		return
	case errors.Is(err, auth.ErrWeakPassword):
		back(w, r, "security", "", capitalize(err.Error())+".")
		return
	case err != nil:
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	setSession(w, token)
	back(w, r, "security", "Senha trocada. As outras sessões do painel foram encerradas.", "")
}

func (h *handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var msg string
	yes, no := true, false
	switch r.PostFormValue("action") {
	case "hide":
		err, msg = h.Store.UpdateDevice(ctx, id, store.DeviceUpdate{Hidden: &yes}), "Aparelho ocultado."
	case "show":
		err, msg = h.Store.UpdateDevice(ctx, id, store.DeviceUpdate{Hidden: &no}), "Aparelho visível de novo."
	default:
		name := strings.TrimSpace(r.PostFormValue("name"))
		if utf8.RuneCountInString(name) > maxNameLen {
			back(w, r, "devices", "", "Nome muito longo (máximo 64 caracteres).")
			return
		}
		err, msg = h.Store.UpdateDevice(ctx, id, store.DeviceUpdate{Name: &name}), "Nome salvo."
	}
	if errors.Is(err, store.ErrNotFound) {
		back(w, r, "devices", "", "Aparelho não encontrado.")
		return
	}
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	back(w, r, "devices", msg, "")
}

type pairPayload struct {
	V     int      `json:"v"`
	URLs  []string `json:"urls"`
	Token string   `json:"token"`
}

type pairedPage struct {
	Name  string
	Token string
	URLs  []string
	QR    template.HTML
}

func (h *handler) pair(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	token, err := h.Auth.NewAPIToken(r.Context(), name)
	if err != nil {
		back(w, r, "pairing", "", capitalize(err.Error())+".")
		return
	}
	if name == "" {
		name = "App Android"
	}
	urls := serverURLs(r, h.HTTPPort)
	payload, _ := json.Marshal(pairPayload{V: 1, URLs: urls, Token: token})
	svg, err := qrSVG(string(payload))
	if err != nil {
		http.Error(w, "Falha ao gerar o QR code", http.StatusInternalServerError)
		return
	}
	render(w, http.StatusOK, "paired.html", pairedPage{Name: name, Token: token, URLs: urls, QR: svg})
}

func (h *handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = h.Store.DeleteToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		back(w, r, "pairing", "", "Pareamento não encontrado.")
		return
	}
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	back(w, r, "pairing", "Pareamento revogado.", "")
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, struct {
		Version     string            `json:"version"`
		DNS         dnsStats          `json:"dns"`
		DroppedLogs uint64            `json:"dropped_logs"`
		LAN         collectors.Status `json:"lan"`
	}{h.Version, h.dnsStats(), h.Dropped(), h.LAN()})
}

func render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	// same-origin, não no-referrer: com no-referrer o navegador manda "Origin: null" no POST
	// e, em HTTP pela LAN (sem Sec-Fetch-Site), a proteção cross-origin recusa o formulário.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.WriteHeader(status)
	if err := pages.ExecuteTemplate(w, name, data); err != nil {
		slog.Warn("falha ao exibir página", "página", name, "erro", err)
	}
}

// back volta para a página do formulário enviado, com uma mensagem.
func back(w http.ResponseWriter, r *http.Request, page, ok, errMsg string) {
	q := url.Values{}
	if ok != "" {
		q.Set("ok", ok)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	http.Redirect(w, r, "/"+page+"?"+q.Encode(), http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("falha ao enviar JSON", "erro", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func clientAddr(r *http.Request) netip.Addr {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[n:]
}

func formatTime(ts int64) string {
	if ts == 0 {
		return "—"
	}
	return time.Unix(ts, 0).Local().Format("02/01/2006 15:04")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// outboundIP descobre o IP local usado na rota padrão (UDP "connect" não envia pacote).
func outboundIP() netip.Addr {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return netip.Addr{}
	}
	defer conn.Close()
	ap, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr()
}

var tailscaleRange = netip.MustParsePrefix("100.64.0.0/10")

// serverURLs lista os endereços do painel que entram no QR: o que o navegador usou,
// o IP da LAN e o IP da Tailscale (para o app funcionar fora de casa).
func serverURLs(r *http.Request, defaultPort string) []string {
	port := defaultPort
	var urls []string
	add := func(host string) {
		u := "http://" + host
		if !contains(urls, u) {
			urls = append(urls, u)
		}
	}
	if host, p, err := net.SplitHostPort(r.Host); err == nil {
		port = p
		if ip, err := netip.ParseAddr(host); err != nil || !ip.IsLoopback() {
			add(r.Host)
		}
	} else if r.Host != "" {
		if ip, err := netip.ParseAddr(r.Host); err != nil || !ip.IsLoopback() {
			add(net.JoinHostPort(r.Host, port))
		}
	}
	if ip := outboundIP(); ip.IsValid() {
		add(net.JoinHostPort(ip.String(), port))
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if p, err := netip.ParsePrefix(a.String()); err == nil && tailscaleRange.Contains(p.Addr()) {
				add(net.JoinHostPort(p.Addr().String(), port))
			}
		}
	}
	return urls
}
