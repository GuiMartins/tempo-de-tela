package store

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestSelfDeviceAndMergeOrphans(t *testing.T) {
	ctx := context.Background()
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	start := time.Unix(1_800_000_000, 0)
	record := func(at time.Time, ip, name string) {
		if err := st.RecordQuery(ctx, at, netip.MustParseAddr(ip), name, 1, 0); err != nil {
			t.Fatal(err)
		}
	}
	// Antes de saber quais IPs são do servidor (como num banco antigo).
	record(start, "172.17.0.9", "radarr.test")
	record(start, "192.168.1.50", "early.test")

	docker := netip.MustParsePrefix("172.17.0.0/16")
	st.SetSelfMatcher(func(ip netip.Addr) bool { return docker.Contains(ip) || ip == netip.MustParseAddr("192.168.1.2") })
	record(start.Add(time.Second), "172.17.0.5", "sonarr.test")
	record(start.Add(time.Second), "127.0.0.1", "local.test")
	record(start.Add(time.Second), "192.168.1.2", "host.test")

	mac, _ := net.ParseMAC("10:a1:da:28:7f:44")
	if err := st.ObserveDevice(ctx, Observation{MAC: mac, IP: netip.MustParseAddr("192.168.1.50"), Hostname: "MacBook-de-Ana"},
		start.Add(time.Minute), time.Hour); err != nil {
		t.Fatal(err)
	}
	merged, err := st.MergeOrphans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if merged != 2 {
		t.Fatalf("registros fundidos = %d, esperado 2", merged)
	}
	devices, err := st.DeviceCoverage(ctx, AllDevices)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Fatalf("aparelhos = %+v", devices)
	}
	counts := map[string]int{}
	for _, d := range devices {
		var n int
		st.DB.QueryRow(`SELECT count(*) FROM queries WHERE device_id = ?`, d.ID).Scan(&n)
		counts[d.Label] = n
		if d.System && d.KindGuess != "server" {
			t.Errorf("servidor sem tipo server: %+v", d)
		}
	}
	if counts["Este servidor"] != 4 || counts["MacBook de Ana"] != 1 {
		t.Fatalf("consultas por aparelho = %v", counts)
	}
}
