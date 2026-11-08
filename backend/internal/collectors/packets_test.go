package collectors

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

func TestARPObservationRejectsProbesAndMismatchedMAC(t *testing.T) {
	mac := net.HardwareAddr{2, 1, 2, 3, 4, 5}
	wire := make([]byte, 28)
	copy(wire, []byte{0, 1, 8, 0, 6, 4, 0, 2})
	copy(wire[8:], mac)
	if _, ok := arpObservation(wire, mac); ok {
		t.Fatal("probe ARP sem endereço de origem")
	}
	copy(wire[14:], []byte{192, 168, 1, 25})
	if _, ok := arpObservation(wire, net.HardwareAddr{2, 0, 0, 0, 0, 1}); ok {
		t.Fatal("MAC de origem inconsistente")
	}
	if got, ok := arpObservation(wire, mac); !ok || got.IP.String() != "192.168.1.25" {
		t.Fatalf("ARP = %+v, %t", got, ok)
	}
}

func TestDHCPRequestDoesNotClaimRequestedIP(t *testing.T) {
	mac := net.HardwareAddr{2, 1, 2, 3, 4, 5}
	ip := netip.MustParseAddr("192.168.1.25")
	addr := ip.As4()
	options := []byte{53, 1, 3, 50, 4, 192, 168, 1, 25, 12, 4, 'P', 'i', 'x', 'l', 55, 3, 1, 3, 6, 255}
	wire := make([]byte, 20+8+240+len(options))
	wire[0], wire[9] = 0x45, 17
	binary.BigEndian.PutUint16(wire[20:], 68)
	binary.BigEndian.PutUint16(wire[22:], 67)
	binary.BigEndian.PutUint16(wire[24:], uint16(len(wire)-20))
	msg := wire[28:]
	msg[0], msg[1], msg[2] = 1, 1, 6
	copy(msg[28:], mac)
	copy(msg[236:], []byte{99, 130, 83, 99})
	copy(msg[240:], options)

	got, ok := dhcpObservation(wire, mac)
	if !ok || got.IP.IsValid() || got.Hostname != "Pixl" || got.Fingerprint != "1,3,6" {
		t.Fatalf("DHCPREQUEST = %+v, %t", got, ok)
	}

	binary.BigEndian.PutUint16(wire[20:], 67)
	binary.BigEndian.PutUint16(wire[22:], 68)
	msg[0], msg[242] = 2, 5
	copy(msg[16:], addr[:])
	got, ok = dhcpObservation(wire, net.HardwareAddr{2, 0, 0, 0, 0, 1})
	if !ok || got.IP != ip || got.MAC.String() != mac.String() {
		t.Fatalf("DHCPACK = %+v, %t", got, ok)
	}
	// As opções do ACK (aqui a opção 12 do pacote reaproveitado) são do servidor.
	if got.Hostname != "" || got.Fingerprint != "" {
		t.Fatalf("ACK não pode trazer hostname/fingerprint do cliente: %+v", got)
	}
}

func TestMDNSNameUsesOnlySourceAddress(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.25")
	msg := &dns.Msg{MsgHdr: dns.MsgHdr{Response: true}}
	for _, record := range []string{
		"proxy.local. 120 IN A 192.168.1.26",
		"expired.local. 0 IN A 192.168.1.25",
		"Apple-TV.LOCAL. 120 IN A 192.168.1.25",
	} {
		rr, err := dns.NewRR(record)
		if err != nil {
			t.Fatal(err)
		}
		msg.Extra = append(msg.Extra, rr)
	}
	wire, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}
	if name := mdnsName(wire, ip); name != "Apple-TV" {
		t.Fatalf("nome mDNS = %q", name)
	}
}
