package policy

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/alerts"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

func TestDecide(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	catalogs, _ := usage.NewCatalogs(ctx, st)
	loc := time.Local
	day := time.Date(2026, 11, 9, 0, 0, 0, 0, loc) // segunda
	at := func(h, m int) time.Time { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }
	ip := netip.MustParseAddr("192.168.1.40")
	other := netip.MustParseAddr("192.168.1.41")
	mac, _ := net.ParseMAC("a4:c1:e8:be:f4:c9")
	st.ObserveDevice(ctx, store.Observation{MAC: mac, IP: ip}, day, 48*time.Hour)
	devices, _ := st.DeviceCoverage(ctx, store.AllDevices)
	p, _ := st.CreatePerson(ctx, store.Person{Name: "Helena", Kind: "child", Monitoring: true}, day)
	st.AssignDevice(ctx, devices[0].ID, &p.ID, day)

	e := New(st, catalogs)
	check := func(now time.Time, client netip.Addr, domain string, wantBlocked bool, wantReason string) {
		t.Helper()
		if err := e.Refresh(ctx, now); err != nil {
			t.Fatal(err)
		}
		reason, blocked := e.Decide(client, domain, now)
		if blocked != wantBlocked || reason != wantReason {
			t.Errorf("%s %s às %s: bloqueado=%v (%q), esperado %v (%q)", client, domain, now.Format("15:04"), blocked, reason, wantBlocked, wantReason)
		}
	}

	// Sem regras: nada bloqueado.
	check(at(10, 0), ip, "roblox.com", false, "")

	rules := alerts.Rules{
		Limits:          map[string]int{"video": 20},
		BlockOnLimit:    true,
		BlockCategories: []string{"social"},
		Bedtime:         &alerts.Window{Start: "22:00", End: "06:00", Block: true},
		School:          &alerts.School{Days: []int{1, 2, 3, 4, 5}, Start: "07:00", End: "12:00", Block: true},
	}
	if err := rules.Validate(catalogs.Get()); err != nil {
		t.Fatal(err)
	}
	alerts.Save(ctx, st, p.ID, rules)
	e.Changed()

	check(at(14, 0), ip, "www.tiktok.com", true, ReasonCategory)      // sempre bloqueada
	check(at(14, 0), other, "www.tiktok.com", false, "")              // outro aparelho, sem dono
	check(at(9, 0), ip, "roblox.com", true, ReasonSchool)             // jogo na aula
	check(at(9, 0), ip, "classroom.google.com", false, "")            // estudo na aula
	check(at(23, 0), ip, "classroom.google.com", true, ReasonBedtime) // hora de dormir bloqueia tudo
	check(at(23, 0), ip, "mtalk.google.com", false, "")               // menos o sistema
	check(at(14, 0), ip, "rr1---sn.googlevideo.com", false, "")       // vídeo antes do limite

	// 25 min de YouTube à tarde: passa do limite de 20 min e o vídeo fica bloqueado.
	for m := 0; m <= 25; m++ {
		st.RecordQuery(ctx, at(14, m), ip, "rr1---sn.googlevideo.com", 1, 0)
	}
	e.Changed()
	check(at(14, 30), ip, "rr1---sn.googlevideo.com", true, ReasonLimit)
	check(at(14, 30), ip, "roblox.com", false, "")
	if s := e.Status(p.ID, at(14, 30)); s != "Bloqueado: Redes sociais, Vídeo (limite)" {
		t.Errorf("status = %q", s)
	}

	// Pausa por 30 min bloqueia tudo (menos o sistema) e some sozinha.
	st.SetPausedUntil(ctx, p.ID, at(15, 30).Unix())
	e.Changed()
	check(at(15, 0), ip, "roblox.com", true, ReasonPause)
	if s := e.Status(p.ID, at(15, 0)); s != "Internet pausada até 15:30" {
		t.Errorf("status = %q", s)
	}
	check(at(15, 31), ip, "roblox.com", false, "")
}
