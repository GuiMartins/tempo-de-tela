package store

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestQueriesFollowConfirmedAddressIntervals(t *testing.T) {
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	start := time.Now().Truncate(time.Second)
	ip := netip.MustParseAddr("192.168.1.25")
	first := net.HardwareAddr{2, 0, 0, 0, 0, 1}
	second := net.HardwareAddr{2, 0, 0, 0, 0, 2}
	record := func(at time.Time, name string) {
		t.Helper()
		if err := st.RecordQuery(ctx, at, ip, name, 1, 0); err != nil {
			t.Fatal(err)
		}
	}
	observe := func(at time.Time, mac net.HardwareAddr, hostname string) {
		t.Helper()
		o := Observation{MAC: mac, IP: ip, Hostname: hostname}
		if err := st.ObserveDevice(ctx, o, at, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	record(start, "before.test")
	observe(start.Add(time.Millisecond), first, "iPhone")
	observe(start.Add(2*time.Millisecond), second, "")
	record(start.Add(time.Millisecond), "queued.test")
	record(start.Add(3*time.Millisecond), "reassigned.test")
	if err := st.ObserveMDNS(ctx, ip, "Apple-TV", start.Add(4*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	record(start.Add(2*time.Minute), "expired.test")
	record(start.Add(25*time.Hour), "stale.test")

	if _, err := st.DB.Exec(`INSERT INTO persons(id, name, monitoring, created_at) VALUES (1, 'Casa', 0, ?)`, start.Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO assignments(device_id, person_id, since)
		SELECT id, 1, ? FROM devices WHERE mac = ?`, start.Unix(), second.String()); err != nil {
		t.Fatal(err)
	}
	record(start.Add(5*time.Millisecond), "paused.test")
	var pausedCount int
	if err := st.DB.QueryRow(`SELECT count(*) FROM domains WHERE name = 'paused.test'`).Scan(&pausedCount); err != nil || pausedCount != 0 {
		t.Fatalf("consulta de pessoa pausada: domínios = %d, erro = %v", pausedCount, err)
	}

	for name, want := range map[string]string{
		"before.test":     "ip:" + ip.String(),
		"queued.test":     "mac:" + first.String(),
		"reassigned.test": "mac:" + second.String(),
		// Vínculo vencido há pouco continua valendo (o aparelho só não respondeu à varredura).
		"expired.test": "mac:" + second.String(),
		"stale.test":   "ip:" + ip.String(),
	} {
		var key string
		err := st.DB.QueryRow(`
			SELECT d.key FROM devices d JOIN queries q ON q.device_id = d.id
			JOIN domains n ON n.id = q.domain_id WHERE n.name = ?`, name).Scan(&key)
		if err != nil || key != want {
			t.Errorf("%s: chave = %q, erro = %v, esperado %q", name, key, err, want)
		}
	}
	devices, err := st.DeviceCoverage(ctx, VisibleDevices)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		switch d.MAC {
		case first.String():
			if d.IPv4 != "" || d.OSGuess != "iOS" || !d.RandomMAC {
				t.Errorf("primeiro aparelho: %+v", d)
			}
		case second.String():
			if d.IPv4 != ip.String() || d.MDNSName != "Apple-TV" || d.OSGuess != "tvOS" {
				t.Errorf("segundo aparelho: %+v", d)
			}
		}
	}
}
