// Package store wires up the sqlite-backed ent client and provides the
// data-access helpers used by the rest of the service: loading the
// singleton AppConfig row, reading/writing the interface VLAN cache, and
// writing request log rows.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"go-mikrotik-vlan-switcher/ent"
	"go-mikrotik-vlan-switcher/ent/interfacevlanstate"

	_ "modernc.org/sqlite"
)

// Open creates an ent client backed by the sqlite file at dbPath and makes
// sure the InterfaceVlanState/RequestLog tables exist. The AppConfig table
// is expected to already exist and be populated externally.
func Open(ctx context.Context, dbPath string) (*ent.Client, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	if err := client.Schema.Create(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return client, nil
}

// LoadAppConfig reads the singleton AppConfig row. It errors if the table
// is empty since the config is expected to be prepopulated before the
// service starts.
func LoadAppConfig(ctx context.Context, client *ent.Client) (*ent.AppConfig, error) {
	cfg, err := client.AppConfig.Query().First(ctx)
	if err != nil {
		return nil, fmt.Errorf("load app_config (has it been prepopulated?): %w", err)
	}
	return cfg, nil
}

// AppConfigSeed is the full set of AppConfig column values used to
// create-or-replace the singleton row.
type AppConfigSeed struct {
	MikrotikAddress      string
	MikrotikUsername     string
	MikrotikPassword     string
	OauthIssuer          string
	OauthAudience        string
	VlanScope            string
	EnableAuthentication bool
}

// SeedAppConfig creates the singleton AppConfig row if none exists, or
// overwrites every column of the existing one otherwise. Since the table
// is a singleton, this is the ORM-side equivalent of scripts/seed_config.sql.
func SeedAppConfig(ctx context.Context, client *ent.Client, seed AppConfigSeed) error {
	existing, err := client.AppConfig.Query().First(ctx)
	if ent.IsNotFound(err) {
		_, err = client.AppConfig.Create().
			SetMikrotikAddress(seed.MikrotikAddress).
			SetMikrotikUsername(seed.MikrotikUsername).
			SetMikrotikPassword(seed.MikrotikPassword).
			SetOauthIssuer(seed.OauthIssuer).
			SetOauthAudience(seed.OauthAudience).
			SetVlanScope(seed.VlanScope).
			SetEnableAuthentication(seed.EnableAuthentication).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}

	_, err = existing.Update().
		SetMikrotikAddress(seed.MikrotikAddress).
		SetMikrotikUsername(seed.MikrotikUsername).
		SetMikrotikPassword(seed.MikrotikPassword).
		SetOauthIssuer(seed.OauthIssuer).
		SetOauthAudience(seed.OauthAudience).
		SetVlanScope(seed.VlanScope).
		SetEnableAuthentication(seed.EnableAuthentication).
		Save(ctx)
	return err
}

// GetCachedVlanState returns the last known list/vlan-id for an interface,
// or ent.IsNotFound(err) == true if nothing has been cached yet.
func GetCachedVlanState(ctx context.Context, client *ent.Client, iface string) (*ent.InterfaceVlanState, error) {
	return client.InterfaceVlanState.Query().
		Where(interfacevlanstate.InterfaceEQ(iface)).
		Only(ctx)
}

// SetCachedVlanState records the current list/vlan-id for an interface,
// creating the row if this is the first time it's seen.
func SetCachedVlanState(ctx context.Context, client *ent.Client, iface, list string, vlanID int) error {
	existing, err := GetCachedVlanState(ctx, client, iface)
	if ent.IsNotFound(err) {
		_, err = client.InterfaceVlanState.Create().
			SetInterface(iface).
			SetCurrentList(list).
			SetCurrentVlanID(vlanID).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}

	_, err = existing.Update().
		SetCurrentList(list).
		SetCurrentVlanID(vlanID).
		Save(ctx)
	return err
}

// DeleteCachedVlanState removes any cached row for iface. It is a no-op if
// nothing was cached.
func DeleteCachedVlanState(ctx context.Context, client *ent.Client, iface string) error {
	_, err := client.InterfaceVlanState.Delete().
		Where(interfacevlanstate.InterfaceEQ(iface)).
		Exec(ctx)
	return err
}

// RequestLogEntry is the set of fields recorded for a single handled request.
type RequestLogEntry struct {
	Method     string
	Path       string
	RemoteAddr string
	Subject    string
	Interface  string
	VlanID     int
	StatusCode int
	DurationMs int64
	Error      string
}

// WriteRequestLog persists one request log row.
func WriteRequestLog(ctx context.Context, client *ent.Client, e RequestLogEntry) error {
	q := client.RequestLog.Create().
		SetMethod(e.Method).
		SetPath(e.Path).
		SetRemoteAddr(e.RemoteAddr).
		SetStatusCode(e.StatusCode).
		SetDurationMs(e.DurationMs)

	if e.Subject != "" {
		q.SetSubject(e.Subject)
	}
	if e.Interface != "" {
		q.SetInterface(e.Interface)
	}
	if e.VlanID != 0 {
		q.SetVlanID(e.VlanID)
	}
	if e.Error != "" {
		q.SetError(e.Error)
	}

	_, err := q.Save(ctx)
	return err
}

// RequestCmdEntry is the set of fields recorded for a single RouterOS API
// command sent to the MikroTik device.
type RequestCmdEntry struct {
	Command    string
	Args       string
	Interface  string
	Success    bool
	Error      string
	DurationMs int64
}

// WriteRequestCmd persists one request_cmds row.
func WriteRequestCmd(ctx context.Context, client *ent.Client, e RequestCmdEntry) error {
	q := client.RequestCmd.Create().
		SetCommand(e.Command).
		SetSuccess(e.Success).
		SetDurationMs(e.DurationMs)

	if e.Args != "" {
		q.SetArgs(e.Args)
	}
	if e.Interface != "" {
		q.SetInterface(e.Interface)
	}
	if e.Error != "" {
		q.SetError(e.Error)
	}

	_, err := q.Save(ctx)
	return err
}
