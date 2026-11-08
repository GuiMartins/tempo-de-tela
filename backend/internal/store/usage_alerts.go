package store

import (
	"context"
	"database/sql"
	"time"
)

// UsageAlert é um aviso cadastrado: "avise quando <app | categoria | qualquer uso> for usado em
// <aparelho | pessoa>". AppID e Category vazios = qualquer uso; só um de DeviceID e PersonID.
type UsageAlert struct {
	ID        int64  `json:"id"`
	AppID     string `json:"app_id"`
	Category  string `json:"category"`
	DeviceID  *int64 `json:"device_id"`
	PersonID  *int64 `json:"person_id"`
	CreatedAt int64  `json:"created_at"`
}

func (s *Store) UsageAlerts(ctx context.Context) ([]UsageAlert, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, app_id, category, device_id, person_id, created_at FROM usage_alerts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageAlert{}
	for rows.Next() {
		var a UsageAlert
		var device, person sql.NullInt64
		if err := rows.Scan(&a.ID, &a.AppID, &a.Category, &device, &person, &a.CreatedAt); err != nil {
			return nil, err
		}
		if device.Valid {
			a.DeviceID = &device.Int64
		}
		if person.Valid {
			a.PersonID = &person.Int64
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateUsageAlert(ctx context.Context, a UsageAlert, now time.Time) (UsageAlert, error) {
	a.CreatedAt = now.Unix()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO usage_alerts(app_id, category, device_id, person_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		a.AppID, a.Category, a.DeviceID, a.PersonID, a.CreatedAt)
	if err != nil {
		return a, err
	}
	a.ID, err = res.LastInsertId()
	return a, err
}

func (s *Store) DeleteUsageAlert(ctx context.Context, id int64) error {
	return expectRow(s.DB.ExecContext(ctx, `DELETE FROM usage_alerts WHERE id = ?`, id))
}

// LastEventTS é o instante do alerta mais recente cuja chave começa com prefix (0 se nenhum).
func (s *Store) LastEventTS(ctx context.Context, prefix string) (int64, error) {
	var ts sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT max(ts) FROM events WHERE substr(key, 1, length(?1)) = ?1`, prefix).Scan(&ts)
	return ts.Int64, err
}
