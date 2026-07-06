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

	seedConfig               = flag.Bool("seed-config", false, "create or replace the singleton app_config row from the seed-* flags below, then exit without starting the server")
	seedMikrotikAddress      = flag.String("seed-mikrotik-address", "", "seed: mikrotik_address")
	seedMikrotikUsername     = flag.String("seed-mikrotik-username", "", "seed: mikrotik_username")
	seedMikrotikPassword     = flag.String("seed-mikrotik-password", "", "seed: mikrotik_password")
	seedOauthIssuer          = flag.String("seed-oauth-issuer", "", "seed: oauth_issuer")
	seedOauthAudience        = flag.String("seed-oauth-audience", "", "seed: oauth_audience")
	seedVlanScope            = flag.String("seed-vlan-scope", "", "seed: vlan_scope")
	seedEnableAuthentication = flag.Bool("seed-enable-authentication", true, "seed: enable_authentication")
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

	if *seedConfig {
		err := store.SeedAppConfig(ctx, entClient, store.AppConfigSeed{
			MikrotikAddress:      *seedMikrotikAddress,
			MikrotikUsername:     *seedMikrotikUsername,
			MikrotikPassword:     *seedMikrotikPassword,
			OauthIssuer:          *seedOauthIssuer,
			OauthAudience:        *seedOauthAudience,
			VlanScope:            *seedVlanScope,
			EnableAuthentication: *seedEnableAuthentication,
		})
		if err != nil {
			fatal(log, "could not seed app config", err)
		}
		log.Info("app_config seeded")
		return
	}

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
