package store

import (
	"context"
	"net/netip"
	"time"
)

// NamingTarget é um aparelho que pode ganhar nome por DNS reverso.
type NamingTarget struct {
	ID      int64
	IP      netip.Addr
	OnLAN   bool // tem MAC (nome vem do roteador)
	PTRName string
}

// NamingTargets lista os aparelhos com IPv4 conhecido.
func (s *Store) NamingTargets(ctx context.Context) ([]NamingTarget, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, ipv4, mac <> '', ptr_name FROM devices WHERE ipv4 <> '' AND key <> ?`, SelfKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NamingTarget
	for rows.Next() {
		var t NamingTarget
		var ip string
		if err := rows.Scan(&t.ID, &ip, &t.OnLAN, &t.PTRName); err != nil {
			return nil, err
		}
		if t.IP, err = netip.ParseAddr(ip); err == nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func (s *Store) SetPTRName(ctx context.Context, id int64, name string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE devices SET ptr_name = ? WHERE id = ?`, name, id)
	return err
}

// DomainCounts devolve, por aparelho, quantas consultas respondidas cada domínio teve desde since.
func (s *Store) DomainCounts(ctx context.Context, since time.Time) (map[int64]map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT q.device_id, d.name, count(*) FROM queries q JOIN domains d ON d.id = q.domain_id
		WHERE q.ts >= ? AND q.status = 0 GROUP BY q.device_id, q.domain_id`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]int{}
	for rows.Next() {
		var id int64
		var name string
		var n int
		if err := rows.Scan(&id, &name, &n); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]int{}
		}
		out[id][name] = n
	}
	return out, rows.Err()
}

func (s *Store) SetSignature(ctx context.Context, id int64, label, kind, os string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE devices SET dns_label = ?, dns_kind = ?, dns_os = ? WHERE id = ?`, label, kind, os, id)
	return err
}

// MergeOrphans junta os registros só com IP aos aparelhos certos:
//   - IPs deste servidor (loopback, containers) vão para o registro do servidor;
//   - IPs da LAN que um MAC confirmou vão para esse MAC, consulta por consulta, pelo vínculo
//     vigente na hora (antes do primeiro vínculo, para o primeiro dono).
//
// Registros com pessoa vinculada ficam como estão. Devolve quantos registros sumiram.
func (s *Store) MergeOrphans(ctx context.Context) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, substr(key, 4) FROM devices d
		WHERE key LIKE 'ip:%' AND NOT EXISTS (SELECT 1 FROM assignments a WHERE a.device_id = d.id)`)
	if err != nil {
		return 0, err
	}
	type orphan struct {
		id int64
		ip string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		if err := rows.Scan(&o.id, &o.ip); err != nil {
			rows.Close()
			return 0, err
		}
		orphans = append(orphans, o)
	}
	rows.Close()

	merged := 0
	for _, o := range orphans {
		ip, err := netip.ParseAddr(o.ip)
		if err != nil {
			continue
		}
		ok, err := s.mergeOrphan(ctx, o.id, ip)
		if err != nil {
			return merged, err
		}
		if ok {
			merged++
		}
	}
	return merged, nil
}

func (s *Store) mergeOrphan(ctx context.Context, id int64, ip netip.Addr) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if s.selfIP(ip) {
		var selfID int64
		now := time.Now().Unix()
		err := tx.QueryRowContext(ctx, `
			INSERT INTO devices (key, first_seen, last_seen) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET key = key RETURNING id`, SelfKey, now, now).Scan(&selfID)
		if err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE queries SET device_id = ? WHERE device_id = ?`, selfID, id); err != nil {
			return false, err
		}
	} else {
		var bindings int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM device_addresses WHERE ip = ?`, ip.String()).Scan(&bindings); err != nil {
			return false, err
		}
		if bindings == 0 {
			return false, nil
		}
		// Cada consulta vai para o dono do IP naquele momento (ou o primeiro dono, se veio antes).
		_, err := tx.ExecContext(ctx, `
			UPDATE queries SET device_id = coalesce(
				(SELECT device_id FROM device_addresses a WHERE a.ip = ?1 AND a.since <= queries.ts * 1000000000 ORDER BY a.since DESC LIMIT 1),
				(SELECT device_id FROM device_addresses a WHERE a.ip = ?1 ORDER BY a.since LIMIT 1))
			WHERE device_id = ?2`, ip.String(), id)
		if err != nil {
			return false, err
		}
	}
	// Recalcula a última consulta e a primeira aparição dos aparelhos que receberam consultas.
	if _, err := tx.ExecContext(ctx, `
		UPDATE devices SET
			last_dns = max(last_dns, coalesce((SELECT max(ts) FROM queries WHERE device_id = devices.id), 0)),
			first_seen = min(first_seen, coalesce((SELECT min(ts) FROM queries WHERE device_id = devices.id), first_seen))
		WHERE id IN (SELECT DISTINCT device_id FROM queries WHERE device_id <> ?) AND id IN (
			SELECT device_id FROM device_addresses WHERE ip = ? UNION SELECT id FROM devices WHERE key = ?)`,
		id, ip.String(), SelfKey); err != nil {
		return false, err
	}
	var left int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM queries WHERE device_id = ?`, id).Scan(&left); err != nil {
		return false, err
	}
	if left > 0 {
		return false, tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
