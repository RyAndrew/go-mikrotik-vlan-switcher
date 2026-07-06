package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"go-mikrotik-vlan-switcher/internal/api"
	"go-mikrotik-vlan-switcher/internal/auth"
	"go-mikrotik-vlan-switcher/internal/mikrotik"
	"go-mikrotik-vlan-switcher/internal/store"
)

var (
	debug      = flag.Bool("debug", false, "debug log level mode")
	listenAddr = flag.String("listen-addr", ":8080", "HTTP listen address")
	dbFile     = flag.String("db-file", "vlan-switcher.db", "sqlite file name, resolved relative to the current directory")
)

func fatal(log *slog.Logger, message string, err error) {
	log.Error(message, slog.Any("error", err))
	os.Exit(2)
}

func main() {
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}))

	ctx := context.Background()

	entClient, err := store.Open(ctx, *dbFile)
	if err != nil {
		fatal(log, "could not open database", err)
	}
	defer entClient.Close()

	cfg, err := store.LoadAppConfig(ctx, entClient)
	if err != nil {
		fatal(log, "could not load app config", err)
	}

	mikClient, err := mikrotik.Dial(cfg.MikrotikAddress, cfg.MikrotikUsername, cfg.MikrotikPassword)
	if err != nil {
		fatal(log, "could not dial mikrotik", err)
	}
	defer mikClient.Close()

	verifier := auth.NewVerifier(cfg.OauthIssuer, cfg.OauthAudience)

	handler := api.NewRouter(log, entClient, mikClient, verifier, cfg.VlanScope, cfg.EnableAuthentication)

	log.Info("listening", slog.String("address", *listenAddr))
	if err := http.ListenAndServe(*listenAddr, handler); err != nil {
		fatal(log, "server stopped", err)
	}
}
