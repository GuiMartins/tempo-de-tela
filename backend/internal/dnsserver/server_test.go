package dnsserver

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
)

// fakeUpstream responde A 1.2.3.4 (TTL 300) pra qualquer nome e NXDOMAIN pra "nx.test.".
func fakeUpstream(t *testing.T) (addr string, hits *atomic.Int64) {
	t.Helper()
	hits = new(atomic.Int64)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		hits.Add(1)
		m := new(dns.Msg)
		m.SetReply(r)
		if r.Question[0].Name == "nx.test." {
			m.Rcode = dns.RcodeNameError
			m.Ns = []dns.RR{&dns.SOA{Hdr: dns.RR_Header{Name: "test.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 900}, Ns: "ns.test.", Mbox: "h.test.", Minttl: 120}}
		} else {
			m.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.IPv4(1, 2, 3, 4)}}
		}
		w.WriteMsg(m)
	})}
	go srv.ActivateAndServe()
	t.Cleanup(func() { srv.Shutdown() })
	return pc.LocalAddr().String(), hits
}

type logSink struct {
	mu      sync.Mutex
	entries []LogEntry
}

func (l *logSink) log(e LogEntry) {
	l.mu.Lock()
	l.entries = append(l.entries, e)
	l.mu.Unlock()
}

func (l *logSink) all() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LogEntry(nil), l.entries...)
}

func startServer(t *testing.T, upstreams ...string) (*Server, string, *logSink) {
	t.Helper()
	cfg := settings.Defaults()
	cfg.Upstreams = upstreams
	sink := &logSink{}
	s := New(func() settings.Settings { return cfg }, sink.log)
	if err := s.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Shutdown)
	// espera o servidor UDP ficar pronto
	time.Sleep(50 * time.Millisecond)
	return s, s.LocalAddr().String(), sink
}

func query(t *testing.T, addr, name string, qtype uint16) *dns.Msg {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	c := &dns.Client{Timeout: 3 * time.Second}
	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("consulta %s: %v", name, err)
	}
	return resp
}

func TestForwardCapsTTLAndCaches(t *testing.T) {
	up, hits := fakeUpstream(t)
	s, addr, sink := startServer(t, up)

	r1 := query(t, addr, "YouTube.com", dns.TypeA)
	if len(r1.Answer) != 1 || r1.Answer[0].(*dns.A).A.String() != "1.2.3.4" {
		t.Fatalf("resposta inesperada: %v", r1)
	}
	if ttl := r1.Answer[0].Header().Ttl; ttl != 60 {
		t.Fatalf("TTL devia ser limitado a 60, veio %d", ttl)
	}
	query(t, addr, "youtube.com", dns.TypeA)
	if got := hits.Load(); got != 1 {
		t.Fatalf("segunda consulta devia vir do cache; upstream recebeu %d", got)
	}
	if s.Stats.CacheHits.Load() != 1 {
		t.Fatalf("cache hits = %d", s.Stats.CacheHits.Load())
	}
	time.Sleep(20 * time.Millisecond)
	logs := sink.all()
	if len(logs) != 2 || logs[0].Name != "youtube.com" || logs[0].Status != StatusOK || !logs[0].Client.IsLoopback() {
		t.Fatalf("log inesperado: %+v", logs)
	}
}

func TestNegativeCached(t *testing.T) {
	up, hits := fakeUpstream(t)
	_, addr, _ := startServer(t, up)
	for i := 0; i < 2; i++ {
		if r := query(t, addr, "nx.test", dns.TypeA); r.Rcode != dns.RcodeNameError {
			t.Fatalf("rcode = %d", r.Rcode)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("NXDOMAIN devia ir pro cache; upstream recebeu %d", hits.Load())
	}
}

func TestBlocksBypassDomains(t *testing.T) {
	up, hits := fakeUpstream(t)
	_, addr, sink := startServer(t, up)
	for _, n := range []string{"dns.google", "chrome.cloudflare-dns.com", "use-application-dns.net", "mask.icloud.com"} {
		if r := query(t, addr, n, dns.TypeA); r.Rcode != dns.RcodeNameError {
			t.Fatalf("%s devia ser bloqueado, rcode %d", n, r.Rcode)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("bloqueado não devia ir pro upstream")
	}
	time.Sleep(20 * time.Millisecond)
	for _, e := range sink.all() {
		if e.Status != StatusBlocked {
			t.Fatalf("status devia ser bloqueado: %+v", e)
		}
	}
	if IsBypassDomain("google.com") || IsBypassDomain("notdns.google.com.br") {
		t.Fatal("falso positivo no anti-contorno")
	}
}

func TestFailoverAndServfail(t *testing.T) {
	up, _ := fakeUpstream(t)
	// primeiro upstream não responde (porta fechada), o segundo sim
	_, addr, _ := startServer(t, "127.0.0.1:1", up)
	if r := query(t, addr, "a.test", dns.TypeA); r.Rcode != dns.RcodeSuccess {
		t.Fatalf("failover falhou, rcode %d", r.Rcode)
	}
	s2, addr2, sink := startServer(t, "127.0.0.1:1")
	if r := query(t, addr2, "b.test", dns.TypeA); r.Rcode != dns.RcodeServerFailure {
		t.Fatalf("sem upstream devia dar SERVFAIL, rcode %d", r.Rcode)
	}
	time.Sleep(20 * time.Millisecond)
	if s2.Stats.Failures.Load() != 1 || sink.all()[0].Status != StatusFailed {
		t.Fatalf("falha não contabilizada")
	}
}

func TestOnlyLogsUsefulTypes(t *testing.T) {
	up, _ := fakeUpstream(t)
	_, addr, sink := startServer(t, up)
	query(t, addr, "x.test", dns.TypeTXT)
	query(t, addr, "x.test", dns.TypeAAAA)
	time.Sleep(20 * time.Millisecond)
	if l := sink.all(); len(l) != 1 || l[0].QType != dns.TypeAAAA {
		t.Fatalf("só A/AAAA/HTTPS deviam ir pro log: %+v", l)
	}
}
