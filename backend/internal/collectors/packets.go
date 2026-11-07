package collectors

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/miekg/dns"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func arpObservation(p []byte, source net.HardwareAddr) (store.Observation, bool) {
	if len(p) < 28 || !bytes.Equal(p[:6], []byte{0, 1, 8, 0, 6, 4}) ||
		(p[7] != 1 && p[7] != 2) || p[6] != 0 || !bytes.Equal(p[8:14], source) {
		return store.Observation{}, false
	}
	ip := netip.AddrFrom4([4]byte(p[14:18]))
	if ip.IsUnspecified() || ip.IsMulticast() {
		return store.Observation{}, false
	}
	return store.Observation{MAC: source, IP: ip}, true
}

func dhcpObservation(p []byte, source net.HardwareAddr) (store.Observation, bool) {
	if len(p) < 28 || p[0]>>4 != 4 || p[9] != 17 || binary.BigEndian.Uint16(p[6:8])&0x3fff != 0 {
		return store.Observation{}, false
	}
	header := int(p[0]&15) * 4
	if header < 20 || len(p) < header+8 {
		return store.Observation{}, false
	}
	srcIP := netip.AddrFrom4([4]byte(p[12:16]))
	udp := p[header:]
	size := int(binary.BigEndian.Uint16(udp[4:6]))
	if size < 248 || size > len(udp) {
		return store.Observation{}, false
	}
	from, to := binary.BigEndian.Uint16(udp[:2]), binary.BigEndian.Uint16(udp[2:4])
	msg := udp[8:size]
	if msg[1] != 1 || msg[2] != 6 || !bytes.Equal(msg[236:240], []byte{99, 130, 83, 99}) {
		return store.Observation{}, false
	}
	mac := net.HardwareAddr(msg[28:34])
	o := store.Observation{MAC: mac}
	var messageType byte
	options := msg[240:]
	for len(options) > 0 {
		code := options[0]
		options = options[1:]
		if code == 255 {
			break
		}
		if code == 0 {
			continue
		}
		if len(options) == 0 || int(options[0])+1 > len(options) {
			return store.Observation{}, false
		}
		data := options[1 : int(options[0])+1]
		options = options[len(data)+1:]
		switch code {
		case 12:
			o.Hostname = cleanClue(data)
		case 53:
			if len(data) == 1 {
				messageType = data[0]
			}
		case 55:
			parts := make([]string, len(data))
			for i, option := range data {
				parts[i] = strconv.Itoa(int(option))
			}
			o.Fingerprint = strings.Join(parts, ",")
		case 60:
			o.DHCPVendor = cleanClue(data)
		}
	}

	switch {
	case msg[0] == 1 && from == 68 && to == 67 && bytes.Equal(mac, source):
		if messageType != 1 && messageType != 3 && messageType != 8 {
			return store.Observation{}, false
		}
		// O IP solicitado (opção 50) ainda não pertence ao cliente.
		if ciaddr := netip.AddrFrom4([4]byte(msg[12:16])); ciaddr == srcIP && !ciaddr.IsUnspecified() {
			o.IP = ciaddr
		}
	case msg[0] == 2 && from == 67 && to == 68 && messageType == 5:
		// ACK confirma a concessão de yiaddr, inclusive antes do primeiro ARP.
		o.IP = netip.AddrFrom4([4]byte(msg[16:20]))
	default:
		return store.Observation{}, false
	}
	return o, true
}

func cleanClue(data []byte) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, string(data)))
}

func mdnsName(wire []byte, source netip.Addr) string {
	msg := new(dns.Msg)
	if msg.Unpack(wire) != nil || !msg.Response {
		return ""
	}
	for _, section := range [][]dns.RR{msg.Answer, msg.Extra} {
		for _, record := range section {
			a, ok := record.(*dns.A)
			if !ok || a.Hdr.Ttl == 0 || a.Hdr.Class&0x7fff != dns.ClassINET {
				continue
			}
			ip, _ := netip.AddrFromSlice(a.A)
			host := strings.TrimSuffix(a.Hdr.Name, ".")
			if strings.HasSuffix(strings.ToLower(host), ".local") && ip.Unmap() == source {
				return cleanClue([]byte(host[:len(host)-len(".local")]))
			}
		}
	}
	return ""
}
