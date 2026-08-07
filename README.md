Do you use a laptop dock for your personal and work laptops?


Do you use vlans to isolate your network traffic?


Do you want to easily switch vlans on a port of your <a href="https://mikrotik.com/product/crs309_1g_8s_in">Mikrotik RouterOS device</a>?


Use this to expose an (optionally) oauth protected api you can use to perform this task! Everything is configured and logged in SQLite.


<img width="1566" height="1860" alt="Image" src="https://github.com/user-attachments/assets/0f35b12d-9e0c-4114-a3da-f8e9a2faac22" />



# Configuration

The app is intended to be deployed to docker and is configured via environment variables:

| Env var | Default | Description |
| --- | --- | --- |
| `DEBUG` | `false` | debug log level mode |
| `LISTEN_ADDR` | `:7071` | HTTP listen address |
| `DATA_PATH` | `data` | directory holding both the sqlite database file (`vlan-switcher.db`) and `ui.html`, resolved relative to the current directory |
| `ENABLE_UI` | `true` | serve the HTML UI at `GET /`; when `false` the root path 404s like any other unknown path |
| `SEED_CONFIG` | `false` | create or replace the singleton app_config row from the `SEED_*` vars below, then exit without starting the server |
| `SEED_MIKROTIK_ADDRESS` | `""` | seed: mikrotik_address |
| `SEED_MIKROTIK_USERNAME` | `""` | seed: mikrotik_username |
| `SEED_MIKROTIK_PASSWORD` | `""` | seed: mikrotik_password |
| `SEED_OAUTH_ISSUER` | `""` | seed: oauth_issuer |
| `SEED_OAUTH_AUDIENCE` | `""` | seed: oauth_audience |
| `SEED_VLAN_SCOPE` | `""` | seed: vlan_scope |
| `SEED_ENABLE_AUTHENTICATION` | `true` | seed: enable_authentication |



# Initial Setup & Config
Enable the RestAPI in your mikrotik device


First run the app with these env vars to create and seed the initial sqlite database with the necessary configs:

```
DATA_PATH=./data 
SEED_CONFIG=true
SEED_MIKROTIK_ADDRESS="192.168.88.1:8728"
SEED_MIKROTIK_USERNAME=mikrotikRestApiUserName
SEED_MIKROTIK_PASSWORD=mikrotikRestApiPassword
SEED_OAUTH_ISSUER="https://your-org.okta.com/oauth2/default"
SEED_OAUTH_AUDIENCE="api://mikrotik-vlan-switcher"
SEED_VLAN_SCOPE="vlan:write"
SEED_ENABLE_AUTHENTICATION=true
go run .
```


# Basic UI
Optionally set `ENABLE_UI=true` to serve the file ui.html (sample included) from the root "/" path. There are no additional file serving capabilites in this api. Review the ui.html file for library CDN requirements.

<img width="492" height="416" alt="Image" src="https://github.com/user-attachments/assets/1de2a810-84c5-44e0-9197-b5c696c12d16" />