// Package querylog tira as gravações SQLite do caminho das respostas DNS.
package querylog

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

type Logger struct {
	queue   chan dnsserver.LogEntry
	dropped atomic.Uint64
}

func New() *Logger {
	return &Logger{queue: make(chan dnsserver.LogEntry, 4096)}
}

func (l *Logger) Enqueue(entry dnsserver.LogEntry) {
	if !entry.Client.IsValid() || entry.Name == "" {
		return
	}
	select {
	case l.queue <- entry:
	default:
		l.dropped.Add(1)
	}
}

func (l *Logger) Dropped() uint64 { return l.dropped.Load() }

// Close sinaliza que o DNS já parou e que não chegarão mais consultas.
func (l *Logger) Close() { close(l.queue) }

func (l *Logger) Run(ctx context.Context, st *store.Store, retentionDays func() int) {
	retention := time.NewTicker(time.Hour)
	defer retention.Stop()
	prune := func() {
		pruneCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := st.Prune(pruneCtx, retentionDays()); err != nil {
			slog.Warn("falha ao aplicar retenção", "erro", err)
		}
	}
	prune()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retention.C:
			prune()
		case entry, ok := <-l.queue:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := st.RecordQuery(writeCtx, entry.TS, entry.Client, entry.Name, entry.QType, entry.Status)
			cancel()
			if err != nil {
				// O painel contabiliza as perdas sem inundar o log durante uma falha no banco.
				if l.dropped.Add(1) == 1 {
					slog.Warn("falha ao gravar consulta", "erro", err)
				}
			}
		}
	}
}
