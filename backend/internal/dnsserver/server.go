// Package dnsserver é o DNS da casa: repassa as consultas pro upstream (com cache),
// limita o TTL entregue aos aparelhos e avisa o logger de cada consulta.
package dnsserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
)

// Status de uma consulta, igual à coluna queries.status.
const (
	StatusOK      uint8 = 0
	StatusBlocked uint8 = 1
	StatusFailed  uint8 = 2
	// StatusPolicy: bloqueada por regra da pessoa (pausa, hora de dormir, categoria, limite).
	StatusPolicy uint8 = 3
)

// Policy decide se a consulta de um aparelho deve ser bloqueada. Precisa ser rápida: roda em
// toda consulta, antes do cache.
type Policy func(client netip.Addr, name string) bool

type LogEntry struct {
	TS     time.Time
	Client netip.Addr
	Name   string // minúsculo, sem ponto final
	QType  uint16
	Status uint8
}

type Stats struct {
	Queries atomic.Uint64
	Blocked atomic.Uint64
	// PolicyBlocked conta as consultas barradas pelas regras das pessoas.
	PolicyBlocked atomic.Uint64
	CacheHits     atomic.Uint64
	Failures      atomic.Uint64
	lastErr       atomic.Pointer[string]
}

func (s *Stats) LastError() string {
	if p := s.lastErr.Load(); p != nil {
		return *p
	}
	return ""
}

type Server struct {
	cfg   func() settings.Settings
	log   func(LogEntry)
	cache *cache
	sf    singleflight.Group
	httpc *http.Client
	Stats Stats

	mu      sync.Mutex
	servers []*dns.Server
	policy  atomic.Pointer[Policy]
}

// SetPolicy liga o bloqueio por regra (nil desliga).
func (s *Server) SetPolicy(p Policy) {
	if p == nil {
		s.policy.Store(nil)
		return
	}
	s.policy.Store(&p)
}

// New cria o servidor. cfg é lido a cada consulta (configuração muda sem reiniciar);
// log precisa ser não bloqueante.
func New(cfg func() settings.Settings, log func(LogEntry)) *Server {
	if log == nil {
		log = func(LogEntry) {}
	}
	return &Server{
		cfg:   cfg,
		log:   log,
		cache: newCache(20000),
		httpc: &http.Client{Timeout: 3 * time.Second},
	}
}

// Listen abre UDP e TCP no endereço (":53" escuta IPv4 e IPv6) e começa a responder.
// Erro de bind volta na hora.
func (s *Server) Listen(addr string) error {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("DNS UDP %s: %w", addr, err)
	}
	l, err := net.Listen("tcp", pc.LocalAddr().String())
	if err != nil {
		pc.Close()
		return fmt.Errorf("DNS TCP %s: %w", addr, err)
	}
	udpReady := make(chan struct{})
	tcpReady := make(chan struct{})
	udp := &dns.Server{PacketConn: pc, Handler: s, NotifyStartedFunc: func() { close(udpReady) }}
	tcp := &dns.Server{
		Listener: l, Handler: s, NotifyStartedFunc: func() { close(tcpReady) },
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
	}
	s.mu.Lock()
	s.servers = append(s.servers, udp, tcp)
	defer s.mu.Unlock()
	go udp.ActivateAndServe()
	go tcp.ActivateAndServe()
	<-udpReady
	<-tcpReady
	return nil
}

// LocalAddr devolve o endereço UDP real (útil quando a porta é 0, nos testes).
func (s *Server) LocalAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, srv := range s.servers {
		if srv.PacketConn != nil {
			return srv.PacketConn.LocalAddr()
		}
	}
	return nil
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, srv := range s.servers {
		srv.Shutdown()
	}
	s.servers = nil
}

var loggedTypes = map[uint16]bool{dns.TypeA: true, dns.TypeAAAA: true, dns.TypeHTTPS: true}

func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if r.Opcode != dns.OpcodeQuery || len(r.Question) != 1 {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeNotImplemented)
		w.WriteMsg(m)
		return
	}
	s.Stats.Queries.Add(1)
	cfg := s.cfg()
	q := r.Question[0]
	name := strings.ToLower(strings.TrimSuffix(q.Name, "."))
	status := StatusOK
	var client netip.Addr
	if ap, err := netip.ParseAddrPort(w.RemoteAddr().String()); err == nil {
		client = ap.Addr().Unmap()
	}

	var resp *dns.Msg
	if cfg.BlockBypass && IsBypassDomain(name) {
		resp = new(dns.Msg)
		resp.SetRcode(r, dns.RcodeNameError)
		resp.RecursionAvailable = true
		status = StatusBlocked
		s.Stats.Blocked.Add(1)
	} else if p := s.policy.Load(); p != nil && client.IsValid() && (*p)(client, name) {
		resp = new(dns.Msg)
		resp.SetRcode(r, dns.RcodeNameError)
		resp.RecursionAvailable = true
		status = StatusPolicy
		s.Stats.PolicyBlocked.Add(1)
	} else {
		var err error
		resp, err = s.resolve(r, cfg)
		if err != nil {
			s.Stats.Failures.Add(1)
			e := err.Error()
			s.Stats.lastErr.Store(&e)
			resp = new(dns.Msg)
			resp.SetRcode(r, dns.RcodeServerFailure)
			resp.RecursionAvailable = true
			status = StatusFailed
		}
	}

	resp.Id = r.Id
	resp.Question = r.Question // mantém a caixa original (0x20)
	capTTL(resp, uint32(cfg.MaxClientTTL))
	if w.LocalAddr().Network() == "udp" {
		size := dns.MinMsgSize
		if opt := r.IsEdns0(); opt != nil && int(opt.UDPSize()) > size {
			size = int(opt.UDPSize())
		}
		resp.Truncate(size)
	}
	w.WriteMsg(resp)

	if loggedTypes[q.Qtype] {
		s.log(LogEntry{TS: time.Now(), Client: client, Name: name, QType: q.Qtype, Status: status})
	}
}

func (s *Server) resolve(r *dns.Msg, cfg settings.Settings) (*dns.Msg, error) {
	q := r.Question[0]
	do := false
	if opt := r.IsEdns0(); opt != nil {
		do = opt.Do()
	}
	key := fmt.Sprintf("%s|%d|%d|%t|%t", strings.ToLower(q.Name), q.Qtype, q.Qclass, do, r.CheckingDisabled)
	if m, ok := s.cache.get(key); ok {
		s.Stats.CacheHits.Add(1)
		return m, nil
	}
	v, err, _ := s.sf.Do(key, func() (any, error) {
		m, err := s.forward(r, cfg.Upstreams)
		if err != nil {
			return nil, err
		}
		s.cache.set(key, m)
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*dns.Msg).Copy(), nil
}

func (s *Server) forward(r *dns.Msg, upstreams []string) (*dns.Msg, error) {
	var lastResp *dns.Msg
	var errs []error
	for _, u := range upstreams {
		m := r.Copy()
		m.Id = dns.Id()
		var resp *dns.Msg
		var err error
		if strings.HasPrefix(u, "https://") {
			resp, err = s.doh(m, u)
		} else {
			resp, err = exchange(m, hostPort(u))
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", u, err))
			continue
		}
		if resp.Rcode == dns.RcodeServerFailure || resp.Rcode == dns.RcodeRefused {
			lastResp = resp
			continue
		}
		return resp, nil
	}
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, errors.Join(errs...)
}

func exchange(m *dns.Msg, addr string) (*dns.Msg, error) {
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	resp, _, err := c.Exchange(m, addr)
	if err == nil && resp.Truncated {
		c = &dns.Client{Net: "tcp", Timeout: 3 * time.Second}
		resp, _, err = c.Exchange(m, addr)
	}
	return resp, err
}

func (s *Server) doh(m *dns.Msg, url string) (*dns.Msg, error) {
	id := m.Id
	m.Id = 0
	b, err := m.Pack()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	res, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 65535))
	if err != nil {
		return nil, err
	}
	resp := new(dns.Msg)
	if err := resp.Unpack(body); err != nil {
		return nil, err
	}
	resp.Id = id
	return resp, nil
}

func hostPort(u string) string {
	if ap, err := netip.ParseAddrPort(u); err == nil {
		return ap.String()
	}
	if a, err := netip.ParseAddr(u); err == nil {
		return netip.AddrPortFrom(a, 53).String()
	}
	return u
}

func capTTL(m *dns.Msg, max uint32) {
	if max == 0 {
		return
	}
	for _, sec := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range sec {
			if rr.Header().Rrtype == dns.TypeOPT {
				continue
			}
			if rr.Header().Ttl > max {
				rr.Header().Ttl = max
			}
		}
	}
}
