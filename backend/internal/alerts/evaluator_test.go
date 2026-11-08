package alerts

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

func TestEvaluate(t *testing.T) {
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
	loc := time.Local
	day := time.Date(2026, 11, 9, 0, 0, 0, 0, loc) // segunda-feira
	at := func(h, m int) time.Time { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }
	ip := netip.MustParseAddr("192.168.1.30")
	mac, _ := net.ParseMAC("a4:c1:e8:be:f4:c9")

	// Antes de tudo: o aparelho já existia e já usava YouTube (não é "primeira vez").
	st.ObserveDevice(ctx, store.Observation{MAC: mac, IP: ip}, day.AddDate(0, 0, -2), 72*time.Hour)
	for m := 0; m < 10; m++ {
		st.RecordQuery(ctx, day.AddDate(0, 0, -2).Add(time.Duration(m)*time.Minute), ip, "rr1---sn.googlevideo.com", 1, 0)
	}
	devices, _ := st.DeviceCoverage(ctx, store.AllDevices)
	p, _ := st.CreatePerson(ctx, store.Person{Name: "Helena", Kind: "child", Monitoring: true}, day.AddDate(0, 0, -3))
	if err := st.AssignDevice(ctx, devices[0].ID, &p.ID, day.AddDate(0, 0, -2)); err != nil {
		t.Fatal(err)
	}
	rules := Rules{Limits: map[string]int{"games": 30, TotalLimit: 600}, Bedtime: &Window{Start: "22:00", End: "06:00"},
		School: &School{Days: []int{1, 2, 3, 4, 5}, Start: "07:00", End: "12:00"}}
	if err := rules.Validate(catalogs.Get()); err != nil {
		t.Fatal(err)
	}
	if err := Save(ctx, st, p.ID, rules); err != nil {
		t.Fatal(err)
	}
	// Primeira avaliação: marca o início e os apps antigos, sem alertar.
	if err := Evaluate(ctx, st, catalogs, day.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	if ev, _ := st.Events(ctx, 0, 50); len(ev) != 0 {
		t.Fatalf("bootstrap não deveria alertar: %+v", ev)
	}

	st.ObserveDevice(ctx, store.Observation{MAC: mac, IP: ip}, at(0, 0), 24*time.Hour)
	record := func(from time.Time, minutes int, domain string) {
		for m := 0; m <= minutes; m++ {
			st.RecordQuery(ctx, from.Add(time.Duration(m)*time.Minute), ip, domain, 1, 0)
		}
	}
	record(at(1, 10), 5, "rr1---sn.googlevideo.com")  // madrugada (hora de dormir)
	record(at(9, 0), 40, "roblox.com")                // aula + passa do limite de jogos (30 min)
	record(at(14, 0), 20, "rr1---sn.googlevideo.com") // YouTube à tarde, já conhecido
	newMAC, _ := net.ParseMAC("d4:43:8a:82:0a:d0")
	st.ObserveDevice(ctx, store.Observation{MAC: newMAC, IP: netip.MustParseAddr("192.168.1.31"), Hostname: "Galaxy-A54"}, at(15, 0), time.Hour)

	now := at(16, 0)
	for range 2 { // rodar de novo não duplica
		if err := Evaluate(ctx, st, catalogs, now); err != nil {
			t.Fatal(err)
		}
	}
	events, err := st.Events(ctx, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range events {
		got[e.Type] = e.Title + " — " + e.Message
	}
	for kind, want := range map[string]string{
		"limit":      "Helena passou do limite de jogos",
		"bedtime":    "Helena usou YouTube na hora de dormir",
		"school":     "Helena usou Roblox no horário de aula",
		"new_app":    "Helena usou Roblox pela primeira vez",
		"new_device": "Aparelho novo na rede: Galaxy A54",
	} {
		if !strings.Contains(got[kind], want) {
			t.Errorf("%s = %q, esperado conter %q", kind, got[kind], want)
		}
	}
	if len(events) != 5 {
		t.Errorf("alertas = %d, esperado 5: %v", len(events), got)
	}
	if !strings.Contains(got["bedtime"], "Às 01:10") || !strings.Contains(got["limit"], "o limite é 30 min") {
		t.Errorf("detalhes: %v", got)
	}
	latest, _ := st.Events(ctx, -1, 2)
	if len(latest) != 2 || latest[1].ID != events[len(events)-1].ID {
		t.Errorf("mais recentes = %+v", latest)
	}
}

func TestRulesValidate(t *testing.T) {
	ctx := context.Background()
	st, _ := store.OpenMemory()
	defer st.Close()
	catalogs, _ := usage.NewCatalogs(ctx, st)
	cat := catalogs.Get()
	bad := []Rules{
		{Limits: map[string]int{"inexistente": 10}},
		{Limits: map[string]int{"system": 10}},
		{Bedtime: &Window{Start: "25:00", End: "06:00"}},
		{School: &School{Days: []int{1}, Start: "12:00", End: "07:00"}},
		{School: &School{Days: []int{9}, Start: "07:00", End: "12:00"}},
	}
	for _, r := range bad {
		if r.Validate(cat) == nil {
			t.Errorf("deveria recusar %+v", r)
		}
	}
	ok := Rules{Limits: map[string]int{"video": 0, "games": 60}}
	if err := ok.Validate(cat); err != nil || len(ok.Limits) != 1 {
		t.Errorf("limite zero deveria sumir: %+v %v", ok, err)
	}
}
