package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"go-mikrotik-vlan-switcher/internal/api"
	"go-mikrotik-vlan-switcher/internal/auth"
	"go-mikrotik-vlan-switcher/internal/mikrotik"
	"go-mikrotik-vlan-switcher/internal/store"
)

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

var (
	debug      = getEnvBool("DEBUG", false)
	listenAddr = getEnv("LISTEN_ADDR", ":7071")
	// dataPath is a directory holding both the sqlite database file and ui.html.
	dataPath = getEnv("DATA_PATH", "data")
	enableUI = getEnvBool("ENABLE_UI", false)

	seedConfig               = getEnvBool("SEED_CONFIG", false)
	seedMikrotikAddress      = getEnv("SEED_MIKROTIK_ADDRESS", "")
	seedMikrotikUsername     = getEnv("SEED_MIKROTIK_USERNAME", "")
	seedMikrotikPassword     = getEnv("SEED_MIKROTIK_PASSWORD", "")
	seedOauthIssuer          = getEnv("SEED_OAUTH_ISSUER", "")
	seedOauthAudience        = getEnv("SEED_OAUTH_AUDIENCE", "")
	seedVlanScope            = getEnv("SEED_VLAN_SCOPE", "")
	seedEnableAuthentication = getEnvBool("SEED_ENABLE_AUTHENTICATION", true)
)

func fatal(log *slog.Logger, message string, err error) {
	log.Error(message, slog.Any("error", err))
	os.Exit(2)
}

func main() {
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}))

	ctx := context.Background()

	dbFile := filepath.Join(dataPath, "vlan-switcher.db")
	uiHTMLPath := filepath.Join(dataPath, "ui.html")

	entClient, err := store.Open(ctx, dbFile)
	if err != nil {
		fatal(log, "could not open database", err)
	}
	defer entClient.Close()

	if seedConfig {
		err := store.SeedAppConfig(ctx, entClient, store.AppConfigSeed{
			MikrotikAddress:      seedMikrotikAddress,
			MikrotikUsername:     seedMikrotikUsername,
			MikrotikPassword:     seedMikrotikPassword,
			OauthIssuer:          seedOauthIssuer,
			OauthAudience:        seedOauthAudience,
			VlanScope:            seedVlanScope,
			EnableAuthentication: seedEnableAuthentication,
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

	mikClient := mikrotik.NewClient(cfg.MikrotikAddress, cfg.MikrotikUsername, cfg.MikrotikPassword, entClient)
	if err := mikClient.TestConnection(); err != nil {
		fatal(log, "could not reach mikrotik", err)
	}
	log.Info("mikrotik connection test succeeded")

	verifier := auth.NewVerifier(cfg.OauthIssuer, cfg.OauthAudience)

	handler := api.NewRouter(log, entClient, mikClient, verifier, cfg.VlanScope, cfg.EnableAuthentication, uiHTMLPath, enableUI)

	log.Info("listening", slog.String("address", listenAddr))
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		fatal(log, "server stopped", err)
	}
}
