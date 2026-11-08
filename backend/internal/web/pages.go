package web

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/collectors"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const onlineWindow = 5 * time.Minute

// base são os dados que toda página do painel usa (menu, mensagens, rodapé).
type base struct {
	Title, Active string
	Version       string
	Uptime        string
	OK, Err       string
}

func (h *handler) base(r *http.Request, active, title string) base {
	q := r.URL.Query()
	return base{
		Title: title, Active: active, Version: h.Version, OK: q.Get("ok"), Err: q.Get("err"),
		Uptime: since(h.Started, time.Now()),
	}
}

type deviceView struct {
	store.DeviceCoverage
	Label, Detected, Details string
	Icon, KindLabel          string
	Online, UsesDNS, OnLAN   bool
	Seen, LastQuery          string
	Owner                    *store.Person
	Search                   string
}

var kindInfo = map[string][2]string{
	"phone": {"phone", "Celular"}, "tablet": {"tablet", "Tablet"}, "computer": {"laptop", "Computador"},
	"tv": {"tv", "TV"}, "console": {"gamepad", "Videogame"}, "speaker": {"speaker", "Caixa de som"},
	"iot": {"sensor", "Casa inteligente"}, "router": {"router", "Roteador"}, "server": {"server", "Servidor"},
	"printer": {"printer", "Impressora"}, "other": {"devices", "Outro"},
}

func viewDevice(d store.DeviceCoverage, owners map[int64]*store.Person, now time.Time) deviceView {
	v := deviceView{DeviceCoverage: d, Detected: d.Label}
	v.Label = firstNonEmpty(d.Name, v.Detected, d.IPv4, d.IPv6, fmt.Sprintf("Aparelho %d", d.ID))
	kind := firstNonEmpty(d.Kind, d.KindGuess)
	v.Icon, v.KindLabel = "devices", ""
	if k, ok := kindInfo[kind]; ok {
		v.Icon, v.KindLabel = k[0], k[1]
	}
	var details []string
	if d.System {
		details = append(details, "consultas do próprio servidor e dos containers")
	}
	if d.Remote {
		details = append(details, "Tailscale")
	}
	for _, s := range []string{d.OSGuess, v.KindLabel, d.Brand} {
		if s != "" && s != v.Label && s != v.Detected && !contains(details, s) {
			details = append(details, s)
		}
	}
	if d.RandomMAC {
		details = append(details, "MAC aleatório")
	}
	v.Details = strings.Join(details, " · ")
	v.Online = d.LastSeen > 0 && now.Sub(time.Unix(d.LastSeen, 0)) < onlineWindow
	v.UsesDNS = d.LastDNS > 0
	v.OnLAN = d.MAC != ""
	v.Seen = ago(d.LastSeen, now)
	v.LastQuery = ago(d.LastDNS, now)
	if d.PersonID != nil {
		v.Owner = owners[*d.PersonID]
	}
	parts := []string{v.Label, v.Detected, d.Vendor, d.OSGuess, v.KindLabel, d.IPv4, d.IPv6, d.MAC, d.Hostname, d.PTRName, d.MDNSName, d.DNSLabel}
	if v.Owner != nil {
		parts = append(parts, v.Owner.Name)
	}
	v.Search = strings.ToLower(strings.Join(parts, " "))
	return v
}

func (h *handler) deviceViews(r *http.Request, filter store.DeviceFilter) ([]deviceView, error) {
	devices, err := h.Store.DeviceCoverage(r.Context(), filter)
	if err != nil {
		return nil, err
	}
	persons, err := h.Store.Persons(r.Context())
	if err != nil {
		return nil, err
	}
	owners := map[int64]*store.Person{}
	for i := range persons {
		owners[persons[i].ID] = &persons[i]
	}
	now := time.Now()
	views := make([]deviceView, len(devices))
	for i, d := range devices {
		views[i] = viewDevice(d, owners, now)
	}
	// Servidor e acesso remoto não são aparelhos da casa: vão para o fim da lista.
	sort.SliceStable(views, func(i, j int) bool {
		return !(views[i].System || views[i].Remote) && (views[j].System || views[j].Remote)
	})
	return views, nil
}

type bar struct {
	Height int
	Label  string
	Title  string
}

type topSite struct {
	Domain string
	Count  int
	Width  int
}

type collectorView struct {
	Name, Status, State string // State: ok, off, err
}

type overviewPage struct {
	base
	DNS                    dnsStats
	CacheRate              int
	PerHour, Blocked       int
	Devices, Online        int
	LANDevices, Covered    int
	Coverage               int
	NotCovered             []deviceView
	NotCoveredMore         int
	Bars                   []bar
	QueriesToday, PeakHour int
	Top                    []topSite
	Collectors             []collectorView
	Interface              string
	Upstreams              []string
	ServerIP               string
	Persons, Paired        int
}

func (h *handler) overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	devices, err := h.deviceViews(r, store.VisibleDevices)
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	now := time.Now()
	act, err := h.Store.Activity(ctx, now, 8)
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	persons, _ := h.Store.Persons(ctx)
	tokens, _ := h.Store.Tokens(ctx)

	p := overviewPage{base: h.base(r, "overview", "Visão geral"), DNS: h.dnsStats(), Devices: len(devices)}
	if p.DNS.Queries > 0 {
		p.CacheRate = int(p.DNS.CacheHits * 100 / p.DNS.Queries)
	}
	for _, d := range devices {
		if d.Online {
			p.Online++
		}
		if !d.OnLAN {
			continue
		}
		p.LANDevices++
		if d.UsesDNS {
			p.Covered++
		} else if len(p.NotCovered) < 6 {
			p.NotCovered = append(p.NotCovered, d)
		} else {
			p.NotCoveredMore++
		}
	}
	if p.LANDevices > 0 {
		p.Coverage = p.Covered * 100 / p.LANDevices
	}

	peak := 1
	for _, n := range act.Hourly {
		peak = max(peak, n)
	}
	firstHour := now.Truncate(time.Hour).Add(-23 * time.Hour)
	for i, n := range act.Hourly {
		at := firstHour.Add(time.Duration(i) * time.Hour)
		label := ""
		if at.Hour()%6 == 0 {
			label = at.Format("15h")
		}
		p.Bars = append(p.Bars, bar{
			Height: max(n*100/peak, map[bool]int{true: 2, false: 0}[n > 0]),
			Label:  label,
			Title:  fmt.Sprintf("%s: %d consultas", at.Format("15h"), n),
		})
		if n == peak && n > 0 {
			p.PeakHour = at.Hour()
		}
	}
	p.QueriesToday = act.Total
	p.PerHour = act.Total / 24
	p.Blocked = act.Blocked
	if len(act.Top) > 0 {
		for _, t := range act.Top {
			p.Top = append(p.Top, topSite{t.Domain, t.Count, max(t.Count*100/act.Top[0].Count, 3)})
		}
	}

	lan := h.LAN()
	p.Interface = lan.Interface
	p.Collectors = []collectorView{
		collector("Varredura ARP", lan.ARP), collector("DHCP passivo", lan.DHCP), collector("mDNS", lan.MDNS),
	}
	p.Upstreams = h.Settings.Get().Upstreams
	p.ServerIP = "o IP deste servidor"
	if ip := outboundIP(); ip.IsValid() {
		p.ServerIP = ip.String()
	}
	p.Persons, p.Paired = len(persons), len(tokens)
	render(w, http.StatusOK, "overview.html", p)
}

func collector(name, status string) collectorView {
	state := "err"
	switch status {
	case "ativo":
		state = "ok"
	case "desativado", "iniciando":
		state = "off"
	}
	return collectorView{name, status, state}
}

type devicesPage struct {
	base
	Devices                              []deviceView
	All, Online, NoDNS, NumHidden, Infra int
}

func (h *handler) devicesPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.deviceViews(r, store.AllDevices)
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	p := devicesPage{base: h.base(r, "devices", "Aparelhos"), Devices: devices}
	for _, d := range devices {
		switch {
		case d.Hidden:
			p.NumHidden++
		case d.System || d.Remote:
			p.Infra++
		default:
			p.All++
			if d.Online {
				p.Online++
			}
			if !d.UsesDNS {
				p.NoDNS++
			}
		}
	}
	render(w, http.StatusOK, "devices.html", p)
}

type settingsPage struct {
	base
	Settings      settings.Settings
	UpstreamsText string
	LAN           collectors.Status
	NtfyURL       string
}

func (h *handler) settingsPage(w http.ResponseWriter, r *http.Request) {
	cfg := h.Settings.Get()
	render(w, http.StatusOK, "settings.html", settingsPage{
		base: h.base(r, "settings", "Configurações"), Settings: cfg,
		UpstreamsText: strings.Join(cfg.Upstreams, "\n"), LAN: h.LAN(),
		NtfyURL: h.Ntfy.SubscribeURL(),
	})
}

type pairingPage struct {
	base
	Tokens []store.Token
}

func (h *handler) pairingPage(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.Store.Tokens(r.Context())
	if err != nil {
		http.Error(w, "Banco de dados indisponível", http.StatusServiceUnavailable)
		return
	}
	render(w, http.StatusOK, "pairing.html", pairingPage{base: h.base(r, "pairing", "App Android"), Tokens: tokens})
}

func (h *handler) securityPage(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "security.html", struct{ base }{h.base(r, "security", "Segurança")})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ago: "agora", "há 5 min", "há 3 h", "ontem 14:20", "02/11 09:10".
func ago(ts int64, now time.Time) string {
	if ts <= 0 {
		return "nunca"
	}
	t := time.Unix(ts, 0).Local()
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("há %d min", int(d/time.Minute))
	case d < 6*time.Hour:
		return fmt.Sprintf("há %d h", int(d/time.Hour))
	}
	y, m, day := now.Local().Date()
	today := time.Date(y, m, day, 0, 0, 0, 0, time.Local)
	switch {
	case !t.Before(today):
		return "hoje " + t.Format("15:04")
	case !t.Before(today.AddDate(0, 0, -1)):
		return "ontem " + t.Format("15:04")
	}
	return t.Format("02/01 15:04")
}

// since: "há 3 min", "há 5 h", "há 2 dias".
func since(start, now time.Time) string {
	if start.IsZero() {
		return ""
	}
	d := now.Sub(start)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("há %d min", max(1, int(d/time.Minute)))
	case d < 48*time.Hour:
		return fmt.Sprintf("há %d h", int(d/time.Hour))
	}
	return fmt.Sprintf("há %d dias", int(d/(24*time.Hour)))
}

func icon(name string) template.HTML {
	return template.HTML(`<svg class="i" aria-hidden="true"><use href="#i-` + template.HTMLEscapeString(name) + `"/></svg>`)
}

// num formata inteiros com separador de milhar: 12345 → "12.345".
func num(v any) string {
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	case uint64:
		n = int64(x)
	}
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}
