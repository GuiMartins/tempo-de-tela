// Package store guarda tudo em um SQLite (modo WAL) dentro do diretório de dados.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/netip"
	"path/filepath"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	DB *sql.DB
	// isSelf diz se o IP é deste servidor (loopback, IPs do host, containers Docker).
	isSelf atomic.Pointer[func(netip.Addr) bool]
}

// SelfKey é a chave do aparelho que representa este servidor e seus containers.
const SelfKey = "self"

// SetSelfMatcher define quais IPs são deste servidor; as consultas deles vão para um registro só.
func (s *Store) SetSelfMatcher(match func(netip.Addr) bool) { s.isSelf.Store(&match) }

func (s *Store) selfIP(ip netip.Addr) bool {
	if ip.IsLoopback() {
		return true
	}
	if m := s.isSelf.Load(); m != nil {
		return (*m)(ip)
	}
	return false
}

func Open(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "tempo-de-tela.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", path)
	return open(dsn)
}

// OpenMemory abre um banco em memória (testes).
func OpenMemory() (*Store, error) {
	return open("file::memory:?_pragma=foreign_keys(1)")
}

func open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Uma conexão só: SQLite serializa escrita de qualquer jeito, e assim o
	// banco em memória dos testes não some entre conexões.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migração: %w", err)
	}
	if err := addColumns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migração: %w", err)
	}
	return &Store{DB: db}, nil
}

// addColumns acrescenta as colunas criadas depois da primeira versão (bancos antigos).
func addColumns(db *sql.DB) error {
	columns := []struct{ table, name, ddl string }{
		{"devices", "ptr_name", "TEXT NOT NULL DEFAULT ''"},
		{"devices", "dns_label", "TEXT NOT NULL DEFAULT ''"},
		{"devices", "dns_kind", "TEXT NOT NULL DEFAULT ''"},
		{"devices", "dns_os", "TEXT NOT NULL DEFAULT ''"},
		{"events", "key", "TEXT NOT NULL DEFAULT ''"},
		{"events", "title", "TEXT NOT NULL DEFAULT ''"},
		{"events", "message", "TEXT NOT NULL DEFAULT ''"},
	}
	defer func() {
		// Versões até 0.1.2 liam a opção 12 do ACK do roteador Xiaomi como hostname do cliente.
		db.Exec(`UPDATE devices SET hostname = '' WHERE hostname LIKE 'MiWiFi-%-srv'`)
	}()
	for _, c := range columns {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pragma_table_info(?) WHERE name = ?)`, c.table, c.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.Exec(`ALTER TABLE ` + c.table + ` ADD COLUMN ` + c.name + ` ` + c.ddl); err != nil {
				return err
			}
		}
	}
	// Um alerta por chave (ex.: "limite:3:2026-11-08:games"): o avaliador pode rodar de novo à vontade.
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_events_key ON events(key) WHERE key <> ''`); err != nil {
		return err
	}
	return nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Prune apaga consultas e eventos mais velhos que a retenção.
func (s *Store) Prune(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cut := time.Now().AddDate(0, 0, -retentionDays)
	res, err := s.DB.ExecContext(ctx, `DELETE FROM queries WHERE ts < ?`, cut.Unix())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM events WHERE ts < ?`, cut.Unix()); err != nil {
		return n, err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM device_addresses WHERE valid_until < ?`, cut.UnixNano())
	return n, err
}
