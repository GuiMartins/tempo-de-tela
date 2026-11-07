package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound indica que o registro pedido não existe.
var ErrNotFound = errors.New("registro não encontrado")

type Token struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used"`
}

func (s *Store) CreateToken(ctx context.Context, name, hash string, now time.Time) (int64, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO tokens(name, hash, created_at) VALUES(?, ?, ?) RETURNING id`,
		name, hash, now.Unix()).Scan(&id)
	return id, err
}

func (s *Store) Tokens(ctx context.Context) ([]Token, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, created_at, last_used FROM tokens ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []Token{}
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.LastUsed); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *Store) DeleteToken(ctx context.Context, id int64) error {
	return expectRow(s.DB.ExecContext(ctx, `DELETE FROM tokens WHERE id = ?`, id))
}

// UseToken confirma o token e atualiza last_used no máximo uma vez por minuto.
func (s *Store) UseToken(ctx context.Context, hash string, now time.Time) (bool, error) {
	var id, lastUsed int64
	err := s.DB.QueryRowContext(ctx, `SELECT id, last_used FROM tokens WHERE hash = ?`, hash).Scan(&id, &lastUsed)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if now.Unix()-lastUsed >= 60 {
		if _, err := s.DB.ExecContext(ctx, `UPDATE tokens SET last_used = ? WHERE id = ?`, now.Unix(), id); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (s *Store) CreateSession(ctx context.Context, hash string, now, expires time.Time) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions(hash, created_at, expires_at) VALUES(?, ?, ?)`,
		hash, now.Unix(), expires.Unix())
	return err
}

func (s *Store) ValidSession(ctx context.Context, hash string, now time.Time) (bool, error) {
	var ok bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sessions WHERE hash = ? AND expires_at > ?)`,
		hash, now.Unix()).Scan(&ok)
	return ok, err
}

func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE hash = ?`, hash)
	return err
}

func (s *Store) DeleteSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions`)
	return err
}

// SetSettingIfAbsent grava a chave só se ela ainda não existir e diz se gravou.
func (s *Store) SetSettingIfAbsent(ctx context.Context, key, value string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO NOTHING`, key, value)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}

func expectRow(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
