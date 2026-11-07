package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

type Person struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Emoji      string  `json:"emoji"`
	Kind       string  `json:"kind"` // child, adult ou shared (aparelhos da casa)
	Monitoring bool    `json:"monitoring"`
	CreatedAt  int64   `json:"created_at"`
	DeviceIDs  []int64 `json:"device_ids"`
}

func (s *Store) Persons(ctx context.Context) ([]Person, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, p.name, p.emoji, p.kind, p.monitoring, p.created_at,
			coalesce((SELECT group_concat(a.device_id) FROM assignments a WHERE a.person_id = p.id AND a.until IS NULL), '')
		FROM persons p ORDER BY CASE p.kind WHEN 'shared' THEN 1 ELSE 0 END, p.name COLLATE NOCASE, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	persons := []Person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		persons = append(persons, p)
	}
	return persons, rows.Err()
}

func (s *Store) Person(ctx context.Context, id int64) (Person, error) {
	p, err := scanPerson(s.DB.QueryRowContext(ctx, `
		SELECT p.id, p.name, p.emoji, p.kind, p.monitoring, p.created_at,
			coalesce((SELECT group_concat(a.device_id) FROM assignments a WHERE a.person_id = p.id AND a.until IS NULL), '')
		FROM persons p WHERE p.id = ?`, id))
	if err == sql.ErrNoRows {
		return p, ErrNotFound
	}
	return p, err
}

func scanPerson(row scanner) (Person, error) {
	var p Person
	var devices string
	if err := row.Scan(&p.ID, &p.Name, &p.Emoji, &p.Kind, &p.Monitoring, &p.CreatedAt, &devices); err != nil {
		return p, err
	}
	p.DeviceIDs = []int64{}
	for _, field := range strings.Split(devices, ",") {
		if id, err := strconv.ParseInt(field, 10, 64); err == nil {
			p.DeviceIDs = append(p.DeviceIDs, id)
		}
	}
	return p, nil
}

func (s *Store) CreatePerson(ctx context.Context, p Person, now time.Time) (Person, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO persons(name, emoji, kind, monitoring, created_at) VALUES(?, ?, ?, ?, ?) RETURNING id`,
		p.Name, p.Emoji, p.Kind, p.Monitoring, now.Unix()).Scan(&id)
	if err != nil {
		return p, err
	}
	return s.Person(ctx, id)
}

func (s *Store) UpdatePerson(ctx context.Context, p Person) error {
	return expectRow(s.DB.ExecContext(ctx, `UPDATE persons SET name = ?, emoji = ?, kind = ?, monitoring = ? WHERE id = ?`,
		p.Name, p.Emoji, p.Kind, p.Monitoring, p.ID))
}

// DeletePerson apaga a pessoa e os vínculos dela. As consultas continuam nos aparelhos.
func (s *Store) DeletePerson(ctx context.Context, id int64) error {
	return expectRow(s.DB.ExecContext(ctx, `DELETE FROM persons WHERE id = ?`, id))
}
