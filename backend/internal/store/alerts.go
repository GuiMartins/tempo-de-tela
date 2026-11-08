package store

import (
	"context"
	"database/sql"
	"time"
)

// Event é um alerta para o app (vira notificação no celular).
type Event struct {
	ID       int64  `json:"id"`
	TS       int64  `json:"ts"`
	Type     string `json:"type"`
	PersonID *int64 `json:"person_id"`
	DeviceID *int64 `json:"device_id"`
	Key      string `json:"-"`
	Title    string `json:"title"`
	Message  string `json:"message"`
}

// AddEvent grava o alerta se ainda não existir outro com a mesma chave; diz se gravou.
func (s *Store) AddEvent(ctx context.Context, e Event) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT OR IGNORE INTO events(ts, type, person_id, device_id, key, title, message) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.TS, e.Type, e.PersonID, e.DeviceID, e.Key, e.Title, e.Message)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Events lista os alertas com id maior que afterID (afterID < 0: os mais recentes), em ordem.
func (s *Store) Events(ctx context.Context, afterID int64, limit int) ([]Event, error) {
	query := `SELECT id, ts, type, person_id, device_id, title, message FROM events WHERE id > ? AND title <> '' ORDER BY id LIMIT ?`
	args := []any{afterID, limit}
	if afterID < 0 {
		query = `SELECT * FROM (SELECT id, ts, type, person_id, device_id, title, message FROM events WHERE title <> '' ORDER BY id DESC LIMIT ?) ORDER BY id`
		args = []any{limit}
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var person, device sql.NullInt64
		if err := rows.Scan(&e.ID, &e.TS, &e.Type, &person, &device, &e.Title, &e.Message); err != nil {
			return nil, err
		}
		if person.Valid {
			e.PersonID = &person.Int64
		}
		if device.Valid {
			e.DeviceID = &device.Int64
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) PersonRules(ctx context.Context, personID int64) (string, bool, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT rules FROM person_rules WHERE person_id = ?`, personID).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return raw, err == nil, err
}

func (s *Store) SetPersonRules(ctx context.Context, personID int64, rules string, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO person_rules(person_id, rules, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(person_id) DO UPDATE SET rules = excluded.rules, updated_at = excluded.updated_at`,
		personID, rules, now.Unix())
	return err
}

// AddKnownApp registra que o app já apareceu para a pessoa; diz se é a primeira vez.
func (s *Store) AddKnownApp(ctx context.Context, personID int64, appID string, seen time.Time) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO person_apps(person_id, app_id, first_seen) VALUES (?, ?, ?)`,
		personID, appID, seen.Unix())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// NewDevicesSince lista os aparelhos com MAC que apareceram depois de since.
func (s *Store) NewDevicesSince(ctx context.Context, since time.Time) ([]DeviceCoverage, error) {
	all, err := s.DeviceCoverage(ctx, AllDevices)
	if err != nil {
		return nil, err
	}
	var out []DeviceCoverage
	for _, d := range all {
		if d.MAC != "" && d.FirstSeen > since.Unix() {
			out = append(out, d)
		}
	}
	return out, nil
}
