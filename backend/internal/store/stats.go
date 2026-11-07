package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// Activity resume as últimas 24 horas: consultas por hora (a última posição é a hora atual)
// e os sites mais consultados, agrupando subdomínios (rr3---sn.googlevideo.com → googlevideo.com).
type Activity struct {
	Hourly  []int
	Total   int
	Blocked int
	Top     []DomainCount
}

func (s *Store) Activity(ctx context.Context, now time.Time, topN int) (Activity, error) {
	act := Activity{Hourly: make([]int, 24)}
	currentHour := now.Unix() / 3600
	since := (currentHour - 23) * 3600
	rows, err := s.DB.QueryContext(ctx, `
		SELECT ts / 3600, count(*) FROM queries WHERE ts >= ? GROUP BY ts / 3600`, since)
	if err != nil {
		return act, err
	}
	for rows.Next() {
		var hour int64
		var n int
		if err := rows.Scan(&hour, &n); err != nil {
			rows.Close()
			return act, err
		}
		if i := int(hour - (currentHour - 23)); i >= 0 && i < 24 {
			act.Hourly[i] += n
			act.Total += n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return act, err
	}

	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM queries WHERE ts >= ? AND status = 1`, since).
		Scan(&act.Blocked); err != nil {
		return act, err
	}

	rows, err = s.DB.QueryContext(ctx, `
		SELECT d.name, count(*) FROM queries q JOIN domains d ON d.id = q.domain_id
		WHERE q.ts >= ? AND q.status = 0 GROUP BY q.domain_id`, since)
	if err != nil {
		return act, err
	}
	defer rows.Close()
	grouped := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return act, err
		}
		grouped[siteOf(name)] += n
	}
	if err := rows.Err(); err != nil {
		return act, err
	}
	for domain, n := range grouped {
		act.Top = append(act.Top, DomainCount{domain, n})
	}
	sort.Slice(act.Top, func(i, j int) bool {
		if act.Top[i].Count != act.Top[j].Count {
			return act.Top[i].Count > act.Top[j].Count
		}
		return act.Top[i].Domain < act.Top[j].Domain
	})
	if len(act.Top) > topN {
		act.Top = act.Top[:topN]
	}
	return act, nil
}

// siteOf reduz o nome ao domínio registrável; nomes locais (sem sufixo público) ficam como estão.
func siteOf(name string) string {
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if site, err := publicsuffix.EffectiveTLDPlusOne(name); err == nil {
		return site
	}
	return name
}
