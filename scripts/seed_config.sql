-- Creates (if missing) and populates the singleton app_configs row that
-- go-mikrotik-vlan-switcher reads at startup. The CREATE TABLE matches
-- exactly what `ent`'s auto-migration would create, so this script is safe
-- to run standalone before the app has ever started, or the app can create
-- the table itself on first run and this script just re-seeds the row.
--
-- Usage:
--   sqlite3 vlan-switcher.db < scripts/seed_config.sql
--
-- Edit the VALUES below before running.

CREATE TABLE IF NOT EXISTS `app_configs` (
    `id` integer NOT NULL PRIMARY KEY AUTOINCREMENT,
    `mikrotik_address` text NOT NULL,
    `mikrotik_username` text NOT NULL,
    `mikrotik_password` text NOT NULL,
    `oauth_issuer` text NOT NULL,
    `oauth_audience` text NOT NULL,
    `vlan_scope` text NOT NULL,
    `enable_authentication` bool NOT NULL DEFAULT (true)
);

-- app_configs is a singleton table: wipe any existing row before inserting
-- so re-running this script re-seeds cleanly instead of adding a second row.
DELETE FROM `app_configs`;

INSERT INTO `app_configs` (
    `mikrotik_address`,
    `mikrotik_username`,
    `mikrotik_password`,
    `oauth_issuer`,
    `oauth_audience`,
    `vlan_scope`,
    `enable_authentication`
) VALUES (
    '10.88.0.1:8728',                          -- mikrotik_address
    'apiuser',                                 -- mikrotik_username
    'apiuserpassword',                         -- mikrotik_password
    'https://your-org.okta.com/oauth2/default',-- oauth_issuer
    'api://vlan-switcher',                     -- oauth_audience
    'vlan:write',                               -- vlan_scope
    1                                           -- enable_authentication (1 = on, 0 = off)
);
