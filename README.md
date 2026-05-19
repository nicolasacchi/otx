# otx — OneTrust Explorer

Go CLI for the OneTrust REST API. Mirrors the architecture of `ddx`/`jx`/`mbx`:
single binary, OAuth 2.0 client-credentials with token cache+refresh, JSON-first
output, TTY-aware tables via go-pretty, gjson `--jq` filter, structured error
envelope with deterministic exit codes, agent-mode row caps under `CLAUDECODE=1`,
read-only by default with writes gated behind `--confirm` + write credentials.

**Phase 1 (v0.1.x) ships:** auth + config + consent (CMP) + UCPM. Subsequent
phases add DSAR, data mapping, TPRM, GRC (risk/audit/incident/policy), workflows,
SCIM users, webhooks, attachments, exports, ESG, ethics, CoI, and `overview`.
See the design plan for the full roadmap.

## Install

```bash
go install github.com/nicolasacchi/otx/cmd/otx@latest
# or
git clone https://github.com/nicolasacchi/otx ~/progetti/otx
cd ~/progetti/otx && make install
```

## Authentication

OneTrust uses OAuth 2.0 client-credentials. Get a credential pair from
**Global Settings → Access Management → Client Credentials** in the OneTrust UI.

Resolution order (flag > env > config file):

1. `--client-id`, `--client-secret`, `--base-url` flags
2. `OTX_CLIENT_ID`, `OTX_CLIENT_SECRET`, `OTX_BASE_URL` env vars
3. Named project from `~/.config/otx/config.toml`

```bash
otx config add production-eu \
  --client-id     "..." \
  --client-secret "..." \
  --base-url      "https://app-eu.onetrust.com"

otx config doctor   # token exchange + scope discovery probe
```

### Write safety

`POST` / `PUT` / `PATCH` / `DELETE` commands refuse to run without `--confirm`
**and** a write-scoped credential pair (`write_client_id`/`write_client_secret`
in the config profile, or `OTX_WRITE_CLIENT_ID`/`OTX_WRITE_CLIENT_SECRET` env).
Without `--confirm` you get `kind:write_locked` (exit 6). With `--confirm` but
read-only credentials you get `kind:forbidden_scope` (exit 2).

## Global flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--client-id` | — | OAuth client_id |
| `--client-secret` | — | OAuth client_secret |
| `--base-url` | `https://app-eu.onetrust.com` | Tenant origin |
| `--project` | (default profile) | Named profile from config |
| `--json` | auto on pipe | Force JSON output |
| `--jq` | — | gjson path filter (NOT real jq) |
| `--from`, `--to` | — | Time-range bounds (`1h`, `7d`, `now-2h`, RFC3339, epoch) |
| `--limit` | 50 | Max results (capped at 100 under `CLAUDECODE=1`) |
| `--rows` | — | Override `CLAUDECODE` cap (0 = unlimited) |
| `--confirm` | false | Required for any write |
| `--verbose` | false | Log requests/responses to stderr |
| `--timing` | false | Print per-request timing |

## Commands (v0.1)

### `otx config`

```bash
otx config add <name> --client-id … --client-secret … [--base-url …] \
                      [--write-client-id …] [--write-client-secret …]
otx config remove <name>
otx config use <name>            # set as default profile
otx config list                  # all profiles, secrets masked
otx config current               # active profile + config path
otx config doctor                # OAuth token + scope discovery
```

### `otx auth`

```bash
otx auth token get            # cached or fresh bearer + expires_at
otx auth token refresh        # invalidate cache and re-fetch
otx auth token validate       # probe a token-gated endpoint
otx auth scopes list          # OAuth scopes granted to current credentials
otx auth scopes check <scope> # boolean check for one scope
```

### `otx consent`

```bash
otx consent receipt list --from 24h
otx consent receipt list --data-subject-id <id>
otx consent receipt get <data-subject-id>
otx consent receipt create --file body.json --confirm     # POST /request/v1/consentreceipts/identified
otx consent receipt bulk   --file body.json --confirm     # POST /request/v1/consentreceipts/bulk

otx consent purpose list
otx consent purpose get <purpose-id>

otx consent group list
otx consent group get <group-id>
otx consent group settings

otx consent collection-point list

otx consent subject list
otx consent subject get <data-subject-id>            # v4
```

### `otx ucpm`

```bash
otx ucpm preference get <data-subject-id>            # v4
otx ucpm preference update --file body.json --confirm
otx ucpm subject get <data-subject-id>
otx ucpm subject list
otx ucpm subject create --file body.json --confirm
otx ucpm consent-group list
```

## Output

- **TTY** → go-pretty table (where a column definition exists).
- **Pipe / `--json` / `--jq`** → indented JSON.
- **`--jq`** is gjson syntax. Array projection: `#.{id:Id, name:Name}`. Index:
  `0.Id`. Filter: `#(Status=="Active")#`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic / network |
| 2 | Auth failed / forbidden scope |
| 3 | Validation error (400) |
| 4 | Not found (404) |
| 5 | Rate limited (429) |
| 6 | Deprecated endpoint / not publicly documented / write-locked |
| 7 | Async-job timeout |

## Architecture

```
cmd/otx/main.go
internal/client/        HTTP client, OAuth token provider, retries, pagination
internal/commands/      One file per command group (Cobra)
internal/config/        TOML multi-profile loader with read/write credential split
internal/output/        TTY detect, go-pretty tables, gjson filter, JSON envelope
```

## License

MIT
