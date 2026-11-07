package dnsserver

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	maxCacheTTL      = time.Hour
	maxNegativeTTL   = 5 * time.Minute
	minCacheableTTL  = time.Second
)

type cacheEntry struct {
	msg     *dns.Msg
	stored  time.Time
	expires time.Time
}

type cache struct {
	mu  sync.Mutex
	m   map[string]cacheEntry
	max int
	now func() time.Time
}

func newCache(max int) *cache {
	return &cache{m: make(map[string]cacheEntry), max: max, now: time.Now}
}

// get devolve uma cópia com os TTLs descontados do tempo que ficou em cache.
func (c *cache) get(key string) (*dns.Msg, bool) {
	c.mu.Lock()
	e, ok := c.m[key]
	now := c.now()
	if ok && !now.Before(e.expires) {
		delete(c.m, key)
		ok = false
	}
	c.mu.Unlock()
	if !ok {
		return nil, false
	}
	m := e.msg.Copy()
	elapsed := uint32(now.Sub(e.stored) / time.Second)
	for _, sec := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range sec {
			h := rr.Header()
			if h.Rrtype == dns.TypeOPT {
				continue
			}
			if h.Ttl > elapsed {
				h.Ttl -= elapsed
			} else {
				h.Ttl = 1
			}
		}
	}
	return m, true
}

func (c *cache) set(key string, m *dns.Msg) {
	ttl, ok := cacheTTL(m)
	if !ok {
		return
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		for k, e := range c.m {
			if !now.Before(e.expires) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= c.max {
			c.m = make(map[string]cacheEntry)
		}
	}
	c.m[key] = cacheEntry{msg: m.Copy(), stored: now, expires: now.Add(ttl)}
}

func cacheTTL(m *dns.Msg) (time.Duration, bool) {
	switch m.Rcode {
	case dns.RcodeSuccess, dns.RcodeNameError:
	default:
		return 0, false
	}
	if m.Truncated {
		return 0, false
	}
	var ttl time.Duration
	if m.Rcode == dns.RcodeNameError || len(m.Answer) == 0 {
		// Resposta negativa: TTL vem do SOA da seção de autoridade.
		for _, rr := range m.Ns {
			if soa, ok := rr.(*dns.SOA); ok {
				t := soa.Minttl
				if soa.Hdr.Ttl < t {
					t = soa.Hdr.Ttl
				}
				ttl = time.Duration(t) * time.Second
			}
		}
		if ttl > maxNegativeTTL {
			ttl = maxNegativeTTL
		}
	} else {
		first := true
		for _, sec := range [][]dns.RR{m.Answer, m.Ns} {
			for _, rr := range sec {
				t := time.Duration(rr.Header().Ttl) * time.Second
				if first || t < ttl {
					ttl = t
					first = false
				}
			}
		}
		if ttl > maxCacheTTL {
			ttl = maxCacheTTL
		}
	}
	if ttl < minCacheableTTL {
		return 0, false
	}
	return ttl, true
}
