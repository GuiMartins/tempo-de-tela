//go:build !linux

package enrich

import "net/netip"

// defaultGateway só é lida no Linux (onde o app roda de verdade).
func defaultGateway() netip.Addr { return netip.Addr{} }
