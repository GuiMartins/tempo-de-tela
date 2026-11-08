// Package enrich dá nome aos aparelhos com o que a rede já sabe: o DNS reverso do roteador
// (hostname do DHCP), o MagicDNS da Tailscale e os domínios que cada aparelho consulta.
package enrich

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/naming"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/signature"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

var (
	tailscaleRange = netip.MustParsePrefix("100.64.0.0/10")
	magicDNS       = "100.100.100.100:53"
)

// Run roda até o contexto acabar: nomes reversos a cada 10 min, assinaturas e fusão de
// registros soltos a cada hora (as duas primeiras rodadas logo depois de subir).
func Run(ctx context.Context, st *store.Store) {
	names := time.NewTimer(20 * time.Second)
	hourly := time.NewTimer(time.Minute)
	defer names.Stop()
	defer hourly.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-names.C:
			reverseNames(ctx, st)
			names.Reset(10 * time.Minute)
		case <-hourly.C:
			if n, err := st.MergeOrphans(ctx); err != nil {
				slog.Warn("falha ao juntar registros soltos", "erro", err)
			} else if n > 0 {
				slog.Info("registros só com IP juntados aos aparelhos", "quantidade", n)
			}
			signatures(ctx, st)
			hourly.Reset(time.Hour)
		}
	}
}

func reverseNames(ctx context.Context, st *store.Store) {
	targets, err := st.NamingTargets(ctx)
	if err != nil {
		slog.Warn("falha ao listar aparelhos para nomear", "erro", err)
		return
	}
	gateway := defaultGateway()
	client := &dns.Client{Timeout: 2 * time.Second}
	for _, t := range targets {
		var server string
		switch {
		case t.OnLAN && gateway.IsValid() && t.IP.Is4():
			server = net.JoinHostPort(gateway.String(), "53")
		case !t.OnLAN && tailscaleRange.Contains(t.IP):
			server = magicDNS
		default:
			continue
		}
		name := lookupPTR(ctx, client, server, t.IP)
		if name == "" || !naming.Useful(name) || name == t.PTRName {
			continue
		}
		if err := st.SetPTRName(ctx, t.ID, name); err != nil && ctx.Err() == nil {
			slog.Debug("nome reverso descartado", "erro", err)
		}
	}
}

func lookupPTR(ctx context.Context, client *dns.Client, server string, ip netip.Addr) string {
	arpa, err := dns.ReverseAddr(ip.String())
	if err != nil {
		return ""
	}
	msg := new(dns.Msg)
	msg.SetQuestion(arpa, dns.TypePTR)
	msg.RecursionDesired = true
	resp, _, err := client.ExchangeContext(ctx, msg, server)
	if err != nil || resp.Rcode != dns.RcodeSuccess {
		return ""
	}
	for _, rr := range resp.Answer {
		if ptr, ok := rr.(*dns.PTR); ok {
			return naming.CleanPTR(ptr.Ptr)
		}
	}
	return ""
}

func signatures(ctx context.Context, st *store.Store) {
	counts, err := st.DomainCounts(ctx, time.Now().AddDate(0, 0, -7))
	if err != nil {
		slog.Warn("falha ao ler domínios por aparelho", "erro", err)
		return
	}
	for id, domains := range counts {
		r := signature.Classify(domains)
		if err := st.SetSignature(ctx, id, r.Label, r.Kind, r.OS); err != nil && ctx.Err() == nil {
			slog.Debug("assinatura descartada", "erro", err)
		}
	}
}

// SelfMatcher reconhece os IPs deste servidor: endereços das próprias interfaces e as redes
// dos containers Docker (docker0, br-…). Refaz a lista a cada 5 min (containers novos).
func SelfMatcher() func(netip.Addr) bool {
	var mu sync.Mutex
	var addrs map[netip.Addr]bool
	var nets []netip.Prefix
	var loaded time.Time
	refresh := func() {
		addrs, nets = map[netip.Addr]bool{}, nil
		ifaces, err := net.Interfaces()
		if err != nil {
			return
		}
		for _, iface := range ifaces {
			list, err := iface.Addrs()
			if err != nil {
				continue
			}
			bridge := strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-")
			for _, a := range list {
				p, err := netip.ParsePrefix(a.String())
				if err != nil {
					continue
				}
				addrs[p.Addr().Unmap()] = true
				if bridge && p.Addr().Is4() {
					nets = append(nets, p.Masked())
				}
			}
		}
	}
	return func(ip netip.Addr) bool {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(loaded) > 5*time.Minute {
			refresh()
			loaded = time.Now()
		}
		ip = ip.Unmap().WithZone("")
		if addrs[ip] {
			return true
		}
		for _, n := range nets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
}
