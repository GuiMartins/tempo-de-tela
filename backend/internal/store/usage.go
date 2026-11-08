package store

import (
	"context"
	"time"
)

// Hit é uma consulta respondida, com o domínio, para montar as sessões de uso.
type Hit struct {
	TS       int64
	DeviceID int64
	Domain   string
}

// PersonHits devolve as consultas dos aparelhos da pessoa no período, respeitando o vínculo
// vigente em cada momento (aparelho que trocou de dono conta só no tempo dele).
func (s *Store) PersonHits(ctx context.Context, personID int64, from, to time.Time) ([]Hit, error) {
	return s.hits(ctx, `
		SELECT q.ts, q.device_id, d.name FROM queries q
		JOIN assignments a ON a.device_id = q.device_id AND a.person_id = ?1
			AND q.ts >= a.since AND (a.until IS NULL OR q.ts < a.until)
		JOIN domains d ON d.id = q.domain_id
		WHERE q.ts >= ?2 AND q.ts < ?3 AND q.status = 0
		ORDER BY q.ts`, personID, from.Unix(), to.Unix())
}

// DeviceHits devolve as consultas de um aparelho no período.
func (s *Store) DeviceHits(ctx context.Context, deviceID int64, from, to time.Time) ([]Hit, error) {
	return s.hits(ctx, `
		SELECT q.ts, q.device_id, d.name FROM queries q JOIN domains d ON d.id = q.domain_id
		WHERE q.device_id = ?1 AND q.ts >= ?2 AND q.ts < ?3 AND q.status = 0
		ORDER BY q.ts`, deviceID, from.Unix(), to.Unix())
}

func (s *Store) hits(ctx context.Context, query string, args ...any) ([]Hit, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.TS, &h.DeviceID, &h.Domain); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// DomainUse é quanto um domínio foi consultado no período e por quantos aparelhos.
type DomainUse struct {
	Domain  string
	Count   int
	Devices int
}

// DomainUses lista os domínios consultados pelos aparelhos da casa (sem este servidor).
func (s *Store) DomainUses(ctx context.Context, since time.Time) ([]DomainUse, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT d.name, count(*), count(DISTINCT q.device_id) FROM queries q
		JOIN domains d ON d.id = q.domain_id
		WHERE q.ts >= ? AND q.status = 0 AND q.device_id NOT IN (SELECT id FROM devices WHERE key = ?)
		GROUP BY q.domain_id`, since.Unix(), SelfKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DomainUse
	for rows.Next() {
		var u DomainUse
		if err := rows.Scan(&u.Domain, &u.Count, &u.Devices); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type CatalogApp struct {
	ID, Name, Category string
}

type CatalogRule struct {
	Domain, AppID string
}

func (s *Store) CatalogApps(ctx context.Context) ([]CatalogApp, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, category FROM catalog_apps ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogApp
	for rows.Next() {
		var a CatalogApp
		if err := rows.Scan(&a.ID, &a.Name, &a.Category); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CatalogRules(ctx context.Context) ([]CatalogRule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT domain, app_id FROM catalog_rules ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogRule
	for rows.Next() {
		var r CatalogRule
		if err := rows.Scan(&r.Domain, &r.AppID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateCatalogApp(ctx context.Context, a CatalogApp, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO catalog_apps(id, name, category, created_at) VALUES(?, ?, ?, ?)`,
		a.ID, a.Name, a.Category, now.Unix())
	return err
}

func (s *Store) SetCatalogRule(ctx context.Context, r CatalogRule, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO catalog_rules(domain, app_id, created_at) VALUES(?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET app_id = excluded.app_id, created_at = excluded.created_at`,
		r.Domain, r.AppID, now.Unix())
	return err
}

func (s *Store) DeleteCatalogRule(ctx context.Context, domain string) error {
	return expectRow(s.DB.ExecContext(ctx, `DELETE FROM catalog_rules WHERE domain = ?`, domain))
}
