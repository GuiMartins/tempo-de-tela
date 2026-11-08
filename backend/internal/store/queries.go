package store

import (
	"context"
	"database/sql"
	"net/netip"
	"time"
)

func (s *Store) RecordQuery(ctx context.Context, seen time.Time, client netip.Addr, name string, qtype uint16, status uint8) error {
	client = client.Unmap().WithZone("")
	ip := client.String()
	var ipv4, ipv6 string
	if client.Is4() {
		ipv4 = ip
	} else {
		ipv6 = ip
	}
	ts := seen.Unix()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := "ip:" + ip
	var deviceID int64
	if s.selfIP(client) {
		key, err = SelfKey, sql.ErrNoRows
	} else {
		deviceID, err = deviceAt(ctx, tx, ip, seen.UnixNano())
	}
	switch err {
	case nil:
		_, err = tx.ExecContext(ctx, `UPDATE devices SET last_seen = max(last_seen, ?), last_dns = max(last_dns, ?) WHERE id = ?`,
			ts, ts, deviceID)
	case sql.ErrNoRows:
		err = tx.QueryRowContext(ctx, `
		INSERT INTO devices (key, ipv4, ipv6, first_seen, last_seen, last_dns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			first_seen = min(first_seen, excluded.first_seen),
			last_seen = max(last_seen, excluded.last_seen),
			last_dns = max(last_dns, excluded.last_dns)
		RETURNING id`, key, ipv4, ipv6, ts, ts, ts).Scan(&deviceID)
	}
	if err != nil {
		return err
	}

	var paused bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assignments a JOIN persons p ON p.id = a.person_id
			WHERE a.device_id = ? AND a.since <= ? AND (a.until IS NULL OR a.until > ?)
				AND p.monitoring = 0
		)`, deviceID, ts, ts).Scan(&paused)
	if err != nil {
		return err
	}
	if !paused {
		if _, err := tx.ExecContext(ctx, `INSERT INTO domains(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO queries (ts, device_id, domain_id, qtype, status)
			SELECT ?, ?, id, ?, ? FROM domains WHERE name = ?`, ts, deviceID, qtype, status, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}
