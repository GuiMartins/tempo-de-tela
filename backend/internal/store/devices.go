package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type DeviceCoverage struct {
	ID        int64  `json:"id"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	MDNSName  string `json:"mdns_name"`
	Vendor    string `json:"vendor"`
	OSGuess   string `json:"os_guess"`
	KindGuess string `json:"kind_guess"`
	RandomMAC bool   `json:"random_mac"`
	Name      string `json:"name"`
	// Kind é o tipo escolhido pelo usuário; vazio = usar KindGuess.
	Kind      string `json:"kind"`
	IPv4      string `json:"ipv4"`
	IPv6      string `json:"ipv6"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	LastDNS   int64  `json:"last_dns"`
	Hidden    bool   `json:"hidden"`
	// PersonID é o dono atual (vínculo sem data de fim); nil = sem dono.
	PersonID *int64 `json:"person_id"`
}

type DeviceFilter int

const (
	VisibleDevices DeviceFilter = iota
	HiddenDevices
	AllDevices
)

const deviceColumns = `d.id, d.name, d.kind, d.mac, d.hostname, d.mdns_name, d.vendor, d.os_guess, d.kind_guess, d.random_mac,
	d.ipv4, d.ipv6, d.first_seen, d.last_seen, d.last_dns, d.hidden, a.person_id
	FROM devices d LEFT JOIN assignments a ON a.device_id = d.id AND a.until IS NULL`

type scanner interface{ Scan(...any) error }

func scanDevice(row scanner) (DeviceCoverage, error) {
	var d DeviceCoverage
	var person sql.NullInt64
	err := row.Scan(&d.ID, &d.Name, &d.Kind, &d.MAC, &d.Hostname, &d.MDNSName, &d.Vendor, &d.OSGuess, &d.KindGuess, &d.RandomMAC,
		&d.IPv4, &d.IPv6, &d.FirstSeen, &d.LastSeen, &d.LastDNS, &d.Hidden, &person)
	if person.Valid {
		d.PersonID = &person.Int64
	}
	return d, err
}

// DeviceCoverage lista os aparelhos, dos que usaram o DNS mais recentemente para os outros.
func (s *Store) DeviceCoverage(ctx context.Context, filter DeviceFilter) ([]DeviceCoverage, error) {
	where := map[DeviceFilter]string{VisibleDevices: "WHERE d.hidden = 0", HiddenDevices: "WHERE d.hidden = 1", AllDevices: ""}[filter]
	rows, err := s.DB.QueryContext(ctx, `SELECT `+deviceColumns+` `+where+` ORDER BY d.last_dns DESC, d.last_seen DESC, d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := []DeviceCoverage{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) Device(ctx context.Context, id int64) (DeviceCoverage, error) {
	d, err := scanDevice(s.DB.QueryRowContext(ctx, `SELECT `+deviceColumns+` WHERE d.id = ?`, id))
	if err == sql.ErrNoRows {
		return d, ErrNotFound
	}
	return d, err
}

// DeviceUpdate muda só os campos não nulos.
type DeviceUpdate struct {
	Name   *string
	Kind   *string
	Hidden *bool
}

func (s *Store) UpdateDevice(ctx context.Context, id int64, u DeviceUpdate) error {
	var sets []string
	var args []any
	if u.Name != nil {
		sets, args = append(sets, "name = ?"), append(args, *u.Name)
	}
	if u.Kind != nil {
		sets, args = append(sets, "kind = ?"), append(args, *u.Kind)
	}
	if u.Hidden != nil {
		sets, args = append(sets, "hidden = ?"), append(args, *u.Hidden)
	}
	if len(sets) == 0 {
		_, err := s.Device(ctx, id)
		return err
	}
	return expectRow(s.DB.ExecContext(ctx, `UPDATE devices SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, id)...))
}

// AssignDevice troca o dono do aparelho a partir de agora (personID nil = sem dono).
// O primeiro vínculo de um aparelho vale desde que ele apareceu na rede, pra o
// histórico já coletado entrar na conta da pessoa.
func (s *Store) AssignDevice(ctx context.Context, deviceID int64, personID *int64, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var firstSeen int64
	if err := tx.QueryRowContext(ctx, `SELECT first_seen FROM devices WHERE id = ?`, deviceID).Scan(&firstSeen); err == sql.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if personID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM persons WHERE id = ?)`, *personID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}

	var currentID, currentPerson int64
	err = tx.QueryRowContext(ctx, `SELECT id, person_id FROM assignments WHERE device_id = ? AND until IS NULL`, deviceID).
		Scan(&currentID, &currentPerson)
	switch {
	case err == nil:
		if personID != nil && *personID == currentPerson {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE assignments SET until = max(since, ?) WHERE id = ?`, now.Unix(), currentID); err != nil {
			return err
		}
	case err != sql.ErrNoRows:
		return err
	}
	if personID != nil {
		var assignedBefore bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM assignments WHERE device_id = ?)`, deviceID).Scan(&assignedBefore); err != nil {
			return err
		}
		since := now.Unix()
		if !assignedBefore {
			since = min(firstSeen, since)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO assignments(device_id, person_id, since) VALUES(?, ?, ?)`,
			deviceID, *personID, since); err != nil {
			return err
		}
	}
	return tx.Commit()
}
