package usage

import (
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/catalog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func TestSessionsAndSummary(t *testing.T) {
	cat, err := catalog.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("BRT", -3*3600)
	day := time.Date(2026, 11, 8, 0, 0, 0, 0, loc)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	var hits []store.Hit
	// YouTube das 14:00 às 14:30 no tablet (1 consulta por minuto, como faz um app com TTL de 60 s).
	for m := 0; m <= 30; m++ {
		hits = append(hits, store.Hit{TS: at(14, m), DeviceID: 1, Domain: "rr1---sn.googlevideo.com"})
	}
	// WhatsApp no celular das 14:10 às 14:20 (sobrepõe o YouTube: total não soma em dobro).
	for m := 10; m <= 20; m += 2 {
		hits = append(hits, store.Hit{TS: at(14, m), DeviceID: 2, Domain: "g.whatsapp.net"})
	}
	// Consulta isolada de madrugada: sincronização em segundo plano, não é sessão.
	hits = append(hits, store.Hit{TS: at(3, 0), DeviceID: 2, Domain: "g.whatsapp.net"})
	// Telemetria e domínio desconhecido não contam.
	hits = append(hits, store.Hit{TS: at(15, 0), DeviceID: 1, Domain: "www.googleapis.com"},
		store.Hit{TS: at(15, 1), DeviceID: 1, Domain: "www.googleapis.com"},
		store.Hit{TS: at(15, 2), DeviceID: 1, Domain: "exemplo.com.br"})
	// Roblox à noite: duas rodadas separadas por mais que o gap de jogos (10 min).
	hits = append(hits, store.Hit{TS: at(20, 0), DeviceID: 1, Domain: "roblox.com"}, store.Hit{TS: at(20, 9), DeviceID: 1, Domain: "rbxcdn.com"},
		store.Hit{TS: at(20, 30), DeviceID: 1, Domain: "roblox.com"}, store.Hit{TS: at(20, 35), DeviceID: 1, Domain: "roblox.com"})
	sortHits(hits)

	sessions := Sessionize(hits, cat)
	if len(sessions) != 4 {
		t.Fatalf("sessões = %+v", sessions)
	}
	sum := Summarize(sessions, cat, day, 1)
	want := map[string]int64{"youtube": 31 * 60, "whatsapp": 11 * 60, "roblox": 10*60 + 6*60}
	for _, a := range sum.Apps {
		if a.Sec != want[a.ID] {
			t.Errorf("%s = %d s, esperado %d", a.ID, a.Sec, want[a.ID])
		}
	}
	if sum.Apps[0].ID != "youtube" || sum.Apps[2].Sessions != 2 && sum.Apps[1].Sessions != 2 {
		t.Errorf("ordem/sessões dos apps: %+v", sum.Apps)
	}
	if sum.TotalSec != 31*60+16*60 {
		t.Errorf("total = %d s (a união deveria evitar contar 14:10–14:21 em dobro)", sum.TotalSec)
	}
	if sum.FirstActivity != at(14, 0) || sum.LastActivity != at(20, 36) {
		t.Errorf("primeira/última atividade = %d/%d", sum.FirstActivity, sum.LastActivity)
	}
	if len(sum.Daily) != 1 || sum.Daily[0].Sec != sum.TotalSec || len(sum.Sessions) != 4 {
		t.Errorf("diário/sessões = %+v / %d", sum.Daily, len(sum.Sessions))
	}

	week := Summarize(sessions, cat, day.AddDate(0, 0, -6), 7)
	if len(week.Daily) != 7 || week.Daily[6].Sec != sum.TotalSec || week.Sessions != nil {
		t.Fatalf("semana = %+v", week.Daily)
	}
	// 08/11/2026 é domingo (índice 6): 14h tem 60 min (YouTube + WhatsApp), 20h tem 16.
	if week.Heatmap[6][14] != 31 || week.Heatmap[6][20] != 16 {
		t.Errorf("mapa de calor domingo = %v", week.Heatmap[6])
	}
}

func sortHits(h []store.Hit) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].TS < h[j-1].TS; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}

func TestUnclassifiedSites(t *testing.T) {
	cat, _ := catalog.Builtin()
	got := UnclassifiedSites([]store.DomainUse{
		{Domain: "api.escola.exemplo.com.br", Count: 10, Devices: 1},
		{Domain: "cdn.escola.exemplo.com.br", Count: 5, Devices: 2},
		{Domain: "rr1---sn.googlevideo.com", Count: 500, Devices: 3},
		{Domain: "1.31.168.192.in-addr.arpa", Count: 9, Devices: 1},
	}, cat, 10)
	if len(got) != 1 || got[0].Site != "exemplo.com.br" || got[0].Count != 15 || got[0].Devices != 2 || len(got[0].Examples) != 2 {
		t.Fatalf("não classificados = %+v", got)
	}
}
