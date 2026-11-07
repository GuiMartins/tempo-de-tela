package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/querylog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/web"
)

func main() {
	dataDir := flag.String("data", "data", "diretório de dados")
	dnsAddr := flag.String("dns", "127.0.0.1:1053", "endereço DNS (UDP e TCP)")
	webAddr := flag.String("http", "127.0.0.1:8994", "endereço HTTP de loopback")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dataDir, *dnsAddr, *webAddr); err != nil {
		slog.Error("tempo-de-tela", "erro", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dataDir, dnsAddr, webAddr string) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", webAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	if !listener.Addr().(*net.TCPAddr).IP.IsLoopback() {
		return fmt.Errorf("o painel HTTP precisa usar um endereço de loopback")
	}

	log := querylog.New()
	dns := dnsserver.New(cfg.Get, log.Enqueue)
	if err := dns.Listen(dnsAddr); err != nil {
		return err
	}
	logCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		log.Run(logCtx, st, func() int { return cfg.Get().RetentionDays })
	}()

	httpServer := &http.Server{
		Handler:           web.Handler(st, dns, log.Dropped),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	webDone := make(chan error, 1)
	go func() { webDone <- httpServer.Serve(listener) }()
	slog.Info("serviço iniciado", "dns", dns.LocalAddr(), "http", listener.Addr())

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-webDone:
	}

	shutdownCtx, stopShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopShutdown()
	httpServer.Shutdown(shutdownCtx)
	dns.Shutdown()
	log.Close()
	select {
	case <-logDone:
	case <-shutdownCtx.Done():
		cancel()
		<-logDone
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}
