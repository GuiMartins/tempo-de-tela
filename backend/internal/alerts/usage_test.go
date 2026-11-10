package alerts

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

func TestEvaluateUsage(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	catalogs, err := usage.NewCatalogs(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)
	at := func(h, m int) time.Time { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }

	tvIP, tabletIP := netip.MustParseAddr("192.168.1.40"), netip.MustParseAddr("192.168.1.41")
	tvMAC, _ := net.ParseMAC("a4:c1:e8:be:f4:01")
	tabletMAC, _ := net.ParseMAC("a4:c1:e8:be:f4:02")
	st.ObserveDevice(ctx, store.Observation{MAC: tvMAC, IP: tvIP}, day, 24*time.Hour)
	st.ObserveDevice(ctx, store.Observation{MAC: tabletMAC, IP: tabletIP}, day, 24*time.Hour)
	devices, _ := st.DeviceCoverage(ctx, store.AllDevices)
	var tv, tablet int64
	for _, d := range devices {
		switch d.IPv4 {
		case tvIP.String():
			tv = d.ID
		case tabletIP.String():
			tablet = d.ID
		}
	}
	tvName, tabletName := "TV da sala", "Tablet da Helena"
	st.UpdateDevice(ctx, tv, store.DeviceUpdate{Name: &tvName})
	st.UpdateDevice(ctx, tablet, store.DeviceUpdate{Name: &tabletName})
	helena, _ := st.CreatePerson(ctx, store.Person{Name: "Helena", Kind: "child", Monitoring: true}, day)
	if err := st.AssignDevice(ctx, tablet, &helena.ID, day); err != nil {
		t.Fatal(err)
	}

	record := func(ip netip.Addr, from time.Time, minutes int, domain string) {
		for m := 0; m <= minutes; m++ {
			st.RecordQuery(ctx, from.Add(time.Duration(m)*time.Minute), ip, domain, 1, 0)
		}
	}
	netflixTV, _ := st.CreateUsageAlert(ctx, store.UsageAlert{AppID: "netflix", DeviceID: &tv}, at(8, 0))
	anyTV, _ := st.CreateUsageAlert(ctx, store.UsageAlert{DeviceID: &tv}, at(8, 0))
	gamesHelena, _ := st.CreateUsageAlert(ctx, store.UsageAlert{Category: "games", PersonID: &helena.ID}, at(8, 0))

	count := map[int64]int{}
	titles := map[int64][]string{}
	eval := func(now time.Time) {
		t.Helper()
		if err := EvaluateUsage(ctx, st, catalogs, now); err != nil {
			t.Fatal(err)
		}
		events, _ := st.Events(ctx, 0, 100)
		count, titles = map[int64]int{}, map[int64][]string{}
		for _, e := range events {
			for _, a := range []store.UsageAlert{netflixTV, anyTV, gamesHelena} {
				if e.Type == "usage" && hasPrefix(st, e.ID, a.ID) {
					count[a.ID]++
					titles[a.ID] = append(titles[a.ID], e.Title+" — "+e.Message)
				}
			}
		}
	}

	// Uso de antes do cadastro não avisa.
	record(tvIP, at(7, 0), 10, "nflxvideo.net")
	eval(at(8, 1))
	if count[netflixTV.ID] != 0 || count[anyTV.ID] != 0 {
		t.Fatalf("uso de antes do cadastro avisou: %v", titles)
	}

	// Netflix na TV: avisa uma vez, mesmo rodando de novo e com uma pausa curta no meio.
	record(tvIP, at(9, 0), 10, "nflxvideo.net")
	eval(at(9, 5))
	eval(at(9, 8))
	record(tvIP, at(9, 14), 4, "nflxvideo.net")
	eval(at(9, 20))
	if count[netflixTV.ID] != 1 {
		t.Fatalf("netflix = %d avisos, esperado 1: %v", count[netflixTV.ID], titles[netflixTV.ID])
	}
	if got, want := titles[netflixTV.ID][0], "TV da sala: Netflix em uso — Desde 09:00."; got != want {
		t.Errorf("texto = %q, esperado %q", got, want)
	}
	// Trocar pra YouTube logo depois continua o mesmo bloco de "qualquer uso".
	record(tvIP, at(9, 20), 5, "rr1---sn.googlevideo.com")
	eval(at(9, 25))
	if count[anyTV.ID] != 1 || count[netflixTV.ID] != 1 {
		t.Fatalf("troca de app não deveria avisar de novo: %v", titles)
	}

	// Pausa longa e volta: avisa de novo.
	record(tvIP, at(10, 0), 5, "nflxvideo.net")
	eval(at(10, 3))
	if count[netflixTV.ID] != 2 || count[anyTV.ID] != 2 {
		t.Fatalf("novo uso depois de pausa deveria avisar: %v", titles)
	}

	// Uso que acabou há muito tempo (servidor parado) não avisa atrasado.
	record(tvIP, at(11, 0), 3, "nflxvideo.net")
	eval(at(11, 40))
	if count[netflixTV.ID] != 2 {
		t.Fatalf("aviso velho: %v", titles[netflixTV.ID])
	}

	// App de sistema (pesquisa no Google no navegador): avisa, mas não conta como "qualquer uso".
	searchTV, _ := st.CreateUsageAlert(ctx, store.UsageAlert{AppID: "google-search", DeviceID: &tv}, at(8, 0))
	st.RecordQuery(ctx, at(11, 50), tvIP, "www.google.com.br", 1, 0)
	st.RecordQuery(ctx, at(11, 50).Add(10*time.Second), tvIP, "ogs.google.com", 1, 0)
	st.RecordQuery(ctx, at(11, 51), tvIP, "encrypted-tbn0.gstatic.com", 1, 0)
	eval(at(11, 52))
	events, _ := st.Events(ctx, 0, 100)
	var search []string
	for _, e := range events {
		if hasPrefix(st, e.ID, searchTV.ID) {
			search = append(search, e.Title)
		}
	}
	if len(search) != 1 || search[0] != "TV da sala: Navegador (pesquisa no Google) em uso" {
		t.Fatalf("pesquisa no Google = %v", search)
	}
	if count[anyTV.ID] != 2 {
		t.Fatalf("sistema não deveria contar como qualquer uso: %v", titles[anyTV.ID])
	}
	// YouTube da TV também consulta www.google.com: isso não é o navegador.
	record(tvIP, at(11, 55), 3, "www.google.com")
	eval(at(11, 59))
	if n, _ := st.LastEventTS(ctx, fmt.Sprintf("usage:%d:", searchTV.ID)); n != at(11, 52).Unix() {
		t.Fatalf("www.google.com não deveria avisar navegador")
	}

	// Categoria por pessoa: Roblox no tablet da Helena.
	record(tabletIP, at(12, 0), 5, "roblox.com")
	eval(at(12, 4))
	if count[gamesHelena.ID] != 1 {
		t.Fatalf("jogos da Helena = %d: %v", count[gamesHelena.ID], titles)
	}
	if got, want := titles[gamesHelena.ID][0], "Helena está usando Roblox — Desde 12:00 · Tablet da Helena."; got != want {
		t.Errorf("texto = %q, esperado %q", got, want)
	}
	// YouTube não é jogo.
	record(tabletIP, at(13, 0), 5, "rr1---sn.googlevideo.com")
	eval(at(13, 4))
	if count[gamesHelena.ID] != 1 {
		t.Fatalf("vídeo não deveria avisar jogos: %v", titles[gamesHelena.ID])
	}

	// Monitoramento pausado: nada.
	helena.Monitoring = false
	st.UpdatePerson(ctx, helena)
	record(tabletIP, at(14, 0), 5, "roblox.com")
	eval(at(14, 4))
	if count[gamesHelena.ID] != 1 {
		t.Fatalf("pessoa pausada avisou: %v", titles[gamesHelena.ID])
	}
}

// hasPrefix diz se o alerta eventID veio do aviso alertID (pela chave "usage:<id>:").
func hasPrefix(st *store.Store, eventID, alertID int64) bool {
	var key string
	st.DB.QueryRow(`SELECT key FROM events WHERE id = ?`, eventID).Scan(&key)
	prefix := fmt.Sprintf("usage:%d:", alertID)
	return len(key) >= len(prefix) && key[:len(prefix)] == prefix
}
