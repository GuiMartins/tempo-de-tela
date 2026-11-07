package web

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/collectors"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

//go:embed index.html
var indexHTML string

var index = template.Must(template.New("index").Funcs(template.FuncMap{
	"formatTime": func(ts int64) string {
		if ts == 0 {
			return "—"
		}
		return time.Unix(ts, 0).Local().Format("02/01/2006 15:04:05")
	},
}).Parse(indexHTML))

func Handler(st *store.Store, dns *dnsserver.Server, dropped func() uint64, lanStatus func() collectors.Status) http.Handler {
	dnsAddr := dns.LocalAddr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		devices, err := st.DeviceCoverage(r.Context())
		if err != nil {
			http.Error(w, "Inventário indisponível", http.StatusServiceUnavailable)
			return
		}
		page := struct {
			DNS                                   string
			Queries, CacheHits, Failures, Dropped uint64
			LAN                                   collectors.Status
			Devices                               []store.DeviceCoverage
		}{
			DNS:     dnsAddr,
			Queries: dns.Stats.Queries.Load(), CacheHits: dns.Stats.CacheHits.Load(),
			Failures: dns.Stats.Failures.Load(), Dropped: dropped(), Devices: devices, LAN: lanStatus(),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := index.Execute(w, page); err != nil {
			slog.Warn("falha ao exibir painel", "erro", err)
		}
	})
	mux.HandleFunc("GET /api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		devices, err := st.DeviceCoverage(r.Context())
		if err != nil {
			http.Error(w, "Inventário indisponível", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(devices); err != nil {
			slog.Debug("falha ao enviar inventário", "erro", err)
		}
	})
	return mux
}
