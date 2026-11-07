//go:build linux

package collectors

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

func (l lan) collectMDNS(ctx context.Context, st *store.Store, cfg func() settings.Settings, ready func()) error {
	conn, err := net.ListenMulticastUDP("udp4", l.iface, mdnsGroup)
	if err != nil {
		return fmt.Errorf("mDNS: %w", err)
	}
	defer conn.Close()
	packet := ipv4.NewPacketConn(conn)
	if err := packet.SetMulticastInterface(l.iface); err != nil {
		return err
	}
	if err := packet.SetMulticastTTL(255); err != nil {
		return err
	}
	if err := packet.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		return err
	}
	ready()

	// Dá tempo para a primeira varredura ARP confirmar os IPs.
	nextBrowse := time.Now().Add(2 * time.Second)
	buf := make([]byte, 9000)
	for ctx.Err() == nil {
		if now := time.Now(); !now.Before(nextBrowse) {
			if err := browseMDNS(conn); err != nil {
				slog.Warn("falha na descoberta mDNS", "erro", err)
			}
			nextBrowse = now.Add(scanInterval(cfg))
		}
		if err := conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return err
		}
		n, control, from, err := packet.ReadFrom(buf)
		if e, ok := err.(net.Error); ok && e.Timeout() {
			continue
		}
		if err != nil {
			return err
		}
		if control == nil || control.IfIndex != l.iface.Index {
			continue
		}
		src := from.(*net.UDPAddr)
		ip := src.AddrPort().Addr().Unmap()
		if src.Port != mdnsGroup.Port || ip == l.ip || !l.prefix.Contains(ip) {
			continue
		}
		if name := mdnsName(buf[:n], ip); name != "" {
			writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := st.ObserveMDNS(writeCtx, ip, name, time.Now())
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Debug("nome mDNS descartado", "erro", err)
			}
		}
	}
	return nil
}

func browseMDNS(conn *net.UDPConn) error {
	msg := new(dns.Msg)
	for _, service := range []string{"_workstation._tcp", "_airplay._tcp", "_googlecast._tcp", "_ipp._tcp", "_smb._tcp", "_companion-link._tcp"} {
		msg.Question = append(msg.Question, dns.Question{Name: service + ".local.", Qtype: dns.TypePTR, Qclass: dns.ClassINET})
	}
	wire, err := msg.Pack()
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(wire, mdnsGroup)
	return err
}
