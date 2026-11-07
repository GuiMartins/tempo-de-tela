package store

import (
	"context"
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

	var deviceID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO devices (key, ipv4, ipv6, first_seen, last_seen, last_dns)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			first_seen = min(first_seen, excluded.first_seen),
			last_seen = max(last_seen, excluded.last_seen),
			last_dns = max(last_dns, excluded.last_dns)
		RETURNING id`, "ip:"+ip, ipv4, ipv6, ts, ts, ts).Scan(&deviceID)
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

type DeviceCoverage struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IPv4     string `json:"ipv4"`
	IPv6     string `json:"ipv6"`
	LastSeen int64  `json:"last_seen"`
	LastDNS  int64  `json:"last_dns"`
}

func (s *Store) DeviceCoverage(ctx context.Context) ([]DeviceCoverage, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, ipv4, ipv6, last_seen, last_dns FROM devices
		WHERE hidden = 0 ORDER BY last_dns DESC, last_seen DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := []DeviceCoverage{}
	for rows.Next() {
		var device DeviceCoverage
		if err := rows.Scan(&device.ID, &device.Name, &device.IPv4, &device.IPv6, &device.LastSeen, &device.LastDNS); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}
