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
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/alerts"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/auth"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/collectors"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/dnsserver"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/enrich"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/querylog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/web"
)

var version = "dev"

func main() {
	dataDir := flag.String("data", env("TDT_DATA", "data"), "diretório de dados (TDT_DATA)")
	dnsAddr := flag.String("dns", env("TDT_DNS", "127.0.0.1:1053"), "endereço DNS, UDP e TCP (TDT_DNS)")
	webAddr := flag.String("http", env("TDT_HTTP", "127.0.0.1:8994"), "endereço do painel e da API (TDT_HTTP)")
	lanEnabled := flag.Bool("lan", env("TDT_LAN", "") == "1", "ativa ARP, DHCP passivo e mDNS na LAN, só Linux (TDT_LAN=1)")
	lanInterface := flag.String("lan-interface", env("TDT_LAN_INTERFACE", ""), "interface dos coletores LAN, padrão: rota IPv4 (TDT_LAN_INTERFACE)")
	resetPassword := flag.Bool("reset-password", false, "apaga a senha do painel e sai")
	healthcheck := flag.Bool("healthcheck", false, "confere se o painel responde e sai")
	showVersion := flag.Bool("version", false, "mostra a versão e sai")
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println(version)
		return
	case *healthcheck:
		if err := checkHealth(*webAddr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	case *resetPassword:
		if err := reset(*dataDir); err != nil {
			slog.Error("tempo-de-tela", "erro", err)
			os.Exit(1)
		}
		fmt.Println("Senha do painel apagada. Abra o painel para definir outra.")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dataDir, *dnsAddr, *webAddr, *lanEnabled, *lanInterface); err != nil {
		slog.Error("tempo-de-tela", "erro", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func reset(dataDir string) error {
	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	return auth.ResetPassword(context.Background(), st)
}

// checkHealth chama /healthz pelo loopback, na porta do painel.
func checkHealth(webAddr string) error {
	_, port, err := net.SplitHostPort(webAddr)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/healthz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz respondeu %s", resp.Status)
	}
	return nil
}

func run(ctx context.Context, dataDir, dnsAddr, webAddr string, lanEnabled bool, lanInterface string) error {
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
	// Consultas do próprio servidor e dos containers Docker viram um aparelho só.
	st.SetSelfMatcher(enrich.SelfMatcher())
	catalogs, err := usage.NewCatalogs(ctx, st)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", webAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

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

	enrichCtx, stopEnrich := context.WithCancel(ctx)
	enrichDone, alertsDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(enrichDone); enrich.Run(enrichCtx, st) }()
	go func() { defer close(alertsDone); alerts.Run(enrichCtx, st, catalogs) }()
	defer func() { stopEnrich(); <-enrichDone; <-alertsDone }()

	lanCtx, stopLAN := context.WithCancel(ctx)
	lan, lanDone := collectors.Start(lanCtx, st, cfg.Get, lanEnabled, lanInterface)
	defer func() { stopLAN(); <-lanDone }()
	httpServer := &http.Server{
		Handler: web.Handler(web.Deps{
			Version: version, Store: st, Settings: cfg, Auth: auth.New(st), DNS: dns,
			Dropped: log.Dropped, LAN: lan.Status,
			HTTPPort: strconv.Itoa(listener.Addr().(*net.TCPAddr).Port),
			Started:  time.Now(),
			Catalogs: catalogs,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	webDone := make(chan error, 1)
	go func() { webDone <- httpServer.Serve(listener) }()
	slog.Info("serviço iniciado", "versão", version, "dns", dns.LocalAddr(), "http", listener.Addr())

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
