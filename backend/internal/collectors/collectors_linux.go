//go:build linux

package collectors

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

type lan struct {
	iface  *net.Interface
	ip     netip.Addr
	prefix netip.Prefix
}

func findLAN(name string) (lan, error) {
	if name == "" {
		routes, err := os.ReadFile("/proc/net/route")
		if err != nil {
			return lan{}, err
		}
		bestMetric := int(^uint(0) >> 1)
		for _, line := range strings.Split(string(routes), "\n") {
			f := strings.Fields(line)
			if len(f) < 8 || f[1] != "00000000" || f[7] != "00000000" {
				continue
			}
			flags, _ := strconv.ParseUint(f[3], 16, 32)
			metric, err := strconv.Atoi(f[6])
			if flags&1 != 0 && err == nil && metric < bestMetric {
				bestMetric, name = metric, f[0]
			}
		}
	}
	if name == "" {
		return lan{}, fmt.Errorf("interface LAN não encontrada na rota padrão")
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return lan{}, err
	}
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 || len(iface.HardwareAddr) != 6 {
		return lan{}, fmt.Errorf("interface %s não é uma interface Ethernet ativa", name)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return lan{}, err
	}
	for _, addr := range addrs {
		prefix, err := netip.ParsePrefix(addr.String())
		if err == nil && prefix.Addr().Is4() {
			return lan{iface, prefix.Addr(), prefix.Masked()}, nil
		}
	}
	return lan{}, fmt.Errorf("interface %s sem IPv4", name)
}

func (c *Collectors) run(ctx context.Context, st *store.Store, cfg func() settings.Settings, name string) error {
	if name == "" {
		name = cfg().Interface
	}
	network, err := findLAN(name)
	if err != nil {
		return err
	}
	c.update(func(s *Status) { s.Interface = network.iface.Name })
	save := func(o store.Observation) {
		if o.IP.IsValid() && (!network.prefix.Contains(o.IP) || o.IP == network.ip || o.IP == netip.AddrFrom4([4]byte{255, 255, 255, 255})) {
			return
		}
		if o.MAC.String() == network.iface.HardwareAddr.String() {
			return
		}
		writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := st.ObserveDevice(writeCtx, o, time.Now(), 3*scanInterval(cfg)); err != nil && ctx.Err() == nil {
			slog.Debug("observação LAN descartada", "erro", err)
		}
	}

	var workers sync.WaitGroup
	start := func(field func(*Status) *string, run func(func()) error) {
		workers.Go(func() {
			set := func(value string) { c.update(func(s *Status) { *field(s) = value }) }
			if err := run(func() { set("ativo") }); err != nil && ctx.Err() == nil {
				set(err.Error())
				slog.Warn("coletor LAN indisponível", "erro", err)
			}
		})
	}
	start(func(s *Status) *string { return &s.ARP }, func(ready func()) error {
		return network.collectARP(ctx, cfg, save, ready)
	})
	start(func(s *Status) *string { return &s.DHCP }, func(ready func()) error {
		return network.collectDHCP(ctx, save, ready)
	})
	start(func(s *Status) *string { return &s.MDNS }, func(ready func()) error {
		return network.collectMDNS(ctx, st, cfg, ready)
	})
	workers.Wait()
	return nil
}

func packetSocket(iface int, protocol uint16) (int, error) {
	// sockaddr_ll recebe EtherType em ordem de rede.
	protocol = htons(protocol)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(protocol))
	if err != nil {
		return -1, err
	}
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: protocol, Ifindex: iface}); err != nil {
		unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func receive(ctx context.Context, fd int, handle func([]byte, net.HardwareAddr)) error {
	buf := make([]byte, 4096)
	poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for ctx.Err() == nil {
		n, err := unix.Poll(poll, 250)
		if err == unix.EINTR || (err == nil && n == 0) {
			continue
		}
		if err != nil {
			return err
		}
		n, from, err := unix.Recvfrom(fd, buf, 0)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if addr, ok := from.(*unix.SockaddrLinklayer); ok && addr.Halen == 6 && addr.Pkttype != unix.PACKET_OUTGOING {
			handle(buf[:n], net.HardwareAddr(addr.Addr[:6]))
		}
	}
	return nil
}

func (l lan) collectDHCP(ctx context.Context, save func(store.Observation), ready func()) error {
	fd, err := packetSocket(l.iface.Index, unix.ETH_P_IP)
	if err != nil {
		return fmt.Errorf("DHCP passivo (CAP_NET_RAW): %w", err)
	}
	defer unix.Close(fd)
	// Filtra no kernel para evitar que todo o tráfego IPv4 chegue ao processo.
	filter, err := bpf.Assemble([]bpf.Instruction{
		bpf.LoadAbsolute{Off: 9, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 17, SkipFalse: 5},
		bpf.LoadMemShift{Off: 0},
		bpf.LoadIndirect{Off: 2, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 67, SkipTrue: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 68, SkipFalse: 1},
		bpf.RetConstant{Val: 4096},
		bpf.RetConstant{Val: 0},
	})
	if err != nil {
		return err
	}
	program := make([]unix.SockFilter, len(filter))
	for i, ins := range filter {
		program[i] = unix.SockFilter{Code: ins.Op, Jt: ins.Jt, Jf: ins.Jf, K: ins.K}
	}
	if err := unix.SetsockoptSockFprog(fd, unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, &unix.SockFprog{Len: uint16(len(program)), Filter: &program[0]}); err != nil {
		return err
	}
	ready()
	return receive(ctx, fd, func(p []byte, source net.HardwareAddr) {
		if o, ok := dhcpObservation(p, source); ok {
			save(o)
		}
	})
}

func (l lan) collectARP(ctx context.Context, cfg func() settings.Settings, save func(store.Observation), ready func()) error {
	fd, err := packetSocket(l.iface.Index, unix.ETH_P_ARP)
	if err != nil {
		return fmt.Errorf("ARP (CAP_NET_RAW): %w", err)
	}
	defer unix.Close(fd)
	scanCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanCtx.Err() == nil {
			if err := l.scanARP(scanCtx, fd); err != nil && scanCtx.Err() == nil {
				slog.Warn("falha na varredura ARP", "erro", err)
			}
			if !wait(scanCtx, scanInterval(cfg)) {
				return
			}
		}
	}()
	defer func() { cancel(); <-done }()
	ready()
	return receive(ctx, fd, func(p []byte, source net.HardwareAddr) {
		if o, ok := arpObservation(p, source); ok {
			save(o)
		}
	})
}

func (l lan) scanARP(ctx context.Context, fd int) error {
	prefix := l.prefix
	if prefix.Bits() < 24 {
		prefix = netip.PrefixFrom(l.ip, 24).Masked()
	}
	target := unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP), Ifindex: l.iface.Index, Halen: 6,
		Addr: [8]byte{255, 255, 255, 255, 255, 255},
	}
	var p [28]byte
	copy(p[:8], []byte{0, 1, 8, 0, 6, 4, 0, 1})
	copy(p[8:14], l.iface.HardwareAddr)
	local := l.ip.As4()
	copy(p[14:18], local[:])
	for ip := prefix.Addr(); prefix.Contains(ip); ip = ip.Next() {
		if ip == l.ip || (prefix.Bits() < 31 && (ip == prefix.Addr() || !prefix.Contains(ip.Next()))) {
			continue
		}
		addr := ip.As4()
		copy(p[24:28], addr[:])
		if err := unix.Sendto(fd, p[:], 0, &target); err != nil {
			return err
		}
		if !wait(ctx, 5*time.Millisecond) {
			break
		}
	}
	return nil
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func htons(value uint16) uint16 {
	var wire [2]byte
	binary.BigEndian.PutUint16(wire[:], value)
	return binary.NativeEndian.Uint16(wire[:])
}
