//go:build linux

package enrich

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// defaultGateway lê o roteador da rota IPv4 padrão em /proc/net/route.
func defaultGateway() netip.Addr {
	routes, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return netip.Addr{}
	}
	best, bestMetric := netip.Addr{}, int(^uint(0)>>1)
	for _, line := range strings.Split(string(routes), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 || f[1] != "00000000" || f[7] != "00000000" {
			continue
		}
		flags, _ := strconv.ParseUint(f[3], 16, 32)
		metric, err := strconv.Atoi(f[6])
		if flags&0x3 != 0x3 || err != nil || metric >= bestMetric { // RTF_UP | RTF_GATEWAY
			continue
		}
		raw, err := hex.DecodeString(f[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], binary.BigEndian.Uint32(raw))
		best, bestMetric = netip.AddrFrom4(b), metric
	}
	return best
}
