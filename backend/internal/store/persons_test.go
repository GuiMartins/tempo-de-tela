package store

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

func TestAssignDevice(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	first := time.Unix(1_800_000_000, 0)
	client := netip.MustParseAddr("192.168.1.50")
	if err := st.RecordQuery(ctx, first, client, "youtube.com", 1, 0); err != nil {
		t.Fatal(err)
	}
	devices, _ := st.DeviceCoverage(ctx, AllDevices)
	device := devices[0].ID

	ana, err := st.CreatePerson(ctx, Person{Name: "Ana", Kind: "child", Monitoring: true}, first)
	if err != nil {
		t.Fatal(err)
	}
	bia, _ := st.CreatePerson(ctx, Person{Name: "Bia", Kind: "child", Monitoring: true}, first)

	now := first.Add(time.Hour)
	if err := st.AssignDevice(ctx, device, &ana.ID, now); err != nil {
		t.Fatal(err)
	}
	var since int64
	st.DB.QueryRow(`SELECT since FROM assignments WHERE device_id = ?`, device).Scan(&since)
	if since != first.Unix() {
		t.Fatalf("primeiro vínculo deveria valer desde first_seen, since = %d", since)
	}
	if err := st.AssignDevice(ctx, device, &ana.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Hour)
	if err := st.AssignDevice(ctx, device, &bia.ID, later); err != nil {
		t.Fatal(err)
	}
	var count int
	st.DB.QueryRow(`SELECT count(*) FROM assignments WHERE device_id = ?`, device).Scan(&count)
	if count != 2 {
		t.Fatalf("revincular à mesma pessoa não deveria criar vínculo; vínculos = %d", count)
	}
	d, _ := st.Device(ctx, device)
	if d.PersonID == nil || *d.PersonID != bia.ID {
		t.Fatalf("dono atual = %v, esperado %d", d.PersonID, bia.ID)
	}
	st.DB.QueryRow(`SELECT since FROM assignments WHERE person_id = ?`, bia.ID).Scan(&since)
	if since != later.Unix() {
		t.Fatalf("troca de dono deveria valer a partir de agora, since = %d", since)
	}
	if p, _ := st.Person(ctx, bia.ID); len(p.DeviceIDs) != 1 || p.DeviceIDs[0] != device {
		t.Fatalf("aparelhos da Bia = %v", p.DeviceIDs)
	}
	if p, _ := st.Person(ctx, ana.ID); len(p.DeviceIDs) != 0 {
		t.Fatalf("Ana não deveria ter aparelhos, tem %v", p.DeviceIDs)
	}

	// Monitoramento pausado não grava consultas.
	bia.Monitoring = false
	if err := st.UpdatePerson(ctx, bia); err != nil {
		t.Fatal(err)
	}
	st.RecordQuery(ctx, later.Add(time.Minute), client, "tiktok.com", 1, 0)
	st.DB.QueryRow(`SELECT count(*) FROM queries`).Scan(&count)
	if count != 1 {
		t.Fatalf("pessoa pausada não deveria ter consultas gravadas; consultas = %d", count)
	}

	if err := st.AssignDevice(ctx, device, nil, later.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.Device(ctx, device); d.PersonID != nil {
		t.Fatal("desvincular deveria deixar o aparelho sem dono")
	}
	missing := int64(999)
	if err := st.AssignDevice(ctx, device, &missing, later); err != ErrNotFound {
		t.Fatalf("pessoa inexistente: %v", err)
	}
	if err := st.DeletePerson(ctx, ana.ID); err != nil {
		t.Fatal(err)
	}
	if persons, _ := st.Persons(ctx); len(persons) != 1 {
		t.Fatalf("pessoas = %d", len(persons))
	}
}

func TestActivity(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 11, 7, 20, 30, 0, 0, time.UTC)
	client := netip.MustParseAddr("192.168.1.50")
	record := func(at time.Time, name string, status uint8) {
		if err := st.RecordQuery(ctx, at, client, name, 1, status); err != nil {
			t.Fatal(err)
		}
	}
	record(now, "rr3---sn-abc.googlevideo.com", 0)
	record(now.Add(-time.Minute), "rr5---sn-def.googlevideo.com", 0)
	record(now.Add(-2*time.Hour), "www.youtube.com", 0)
	record(now.Add(-3*time.Hour), "dns.google", 1)
	record(now.Add(-25*time.Hour), "old.example.com", 0)

	act, err := st.Activity(ctx, now, 5)
	if err != nil {
		t.Fatal(err)
	}
	if act.Total != 4 || act.Blocked != 1 || act.Hourly[23] != 2 || act.Hourly[21] != 1 || act.Hourly[20] != 1 {
		t.Fatalf("atividade = %+v", act)
	}
	if len(act.Top) != 2 || act.Top[0] != (DomainCount{"googlevideo.com", 2}) || act.Top[1].Domain != "youtube.com" {
		t.Fatalf("mais consultados = %+v", act.Top)
	}
}
