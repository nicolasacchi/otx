# otx — OneTrust Explorer

Go CLI for the OneTrust REST API. Mirrors the architecture of `ddx`/`jx`/`mbx`:
single binary, OAuth 2.0 client-credentials with token cache+refresh, JSON-first
output, TTY-aware tables via go-pretty, gjson `--jq` filter, structured error
envelope with deterministic exit codes, agent-mode row caps under `CLAUDECODE=1`,
read-only by default with writes gated behind `--confirm` + write credentials.

**v1.0.0 — all six phases shipped.** 25 top-level command groups covering every
OneTrust REST endpoint group: auth, config, consent (CMP), ucpm, dsar,
assessment, workflow, user (SCIM 2.0), role, credential, org, webhook, attach,
export, auditlog, tprm, datamap, discovery, risk, audit, incident, policy, esg,
ethics, coi, plus a parallel `overview` fan-out across modules. Ethics-hotline
and CoI surfaces are partially documented at the API level — those commands
probe the inferred endpoints and emit `kind:not_publicly_documented` on 404
rather than failing silently.

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

## Commands

### `otx overview`

```bash
otx overview --window 24h
```

Parallel fan-out across modules — single-call snapshot. Each section runs
concurrently with its own error envelope on failure (so a partial outage
doesn't abort the call). Sections: `consent_receipts`, `dsar_open`,
`assessment_under_review`, `incidents_recent`, `vendors`, `webhooks`, `auth`
(token expires_in), `scopes` (count granted).



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
otx auth token get            # masked bearer preview + expires_at
otx auth token get --reveal   # print the full bearer (default is masked)
otx auth token refresh        # invalidate cache and re-fetch (masked)
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
otx ucpm subject create --file body.json --confirm
otx ucpm subject list
otx ucpm consent-group list
```

### `otx dsar`

```bash
otx dsar request list --status IN_PROGRESS
otx dsar request get <id>
otx dsar request create --file body.json --confirm
otx dsar request cancel <id> --reason "duplicate" --confirm
otx dsar stage update <id> --stage COMPLETE --confirm
otx dsar subtask list <id>
otx dsar subtask complete <id> <subtask-id> --confirm
otx dsar poll <id> --interval 5s --timeout 10m
```

### `otx assessment` / `otx tprm` / `otx workflow`

```bash
otx assessment list --stage UNDER_REVIEW
otx assessment launch --file body.json --confirm
otx assessment workflow submit <id> --confirm
otx assessment workflow complete <id> --confirm
otx assessment result list <id>
otx assessment risk create <id> --file body.json --confirm
otx assessment attachment upload <id> --file evidence.pdf --confirm

otx tprm vendor list --all
otx tprm vendor get <id>
otx tprm vendor link-child <parent> <child> --confirm
otx tprm assessment list --vendor <id>
otx tprm questionnaire send --file body.json --confirm
otx tprm score get <vendor-id>

otx workflow workflow {list,get,create,export,import}
otx workflow task {list,create,complete}
otx workflow approval {list,approve,reject}
```

### `otx datamap` / `otx discovery` / `otx risk`

```bash
otx datamap inventory list --type system
otx datamap inventory upsert-by-ref <ext-id> --type vendor --file body.json --confirm
otx datamap link {list,add,remove} --type system <inv-id>
otx datamap ropa generate --confirm
otx datamap ropa export
otx datamap classification {list,create}
otx datamap schema get --type processingactivity

otx discovery scan create --file scan.json --confirm
otx discovery scan list
otx discovery poll <job-id> --interval 10s --timeout 60m
otx discovery classify submit <job-id> --file data.json --confirm
otx discovery detector {list,create}

otx risk it-risk {list,get,create,update}
```

### `otx audit` / `otx incident` / `otx policy`

```bash
otx audit workpaper {list,get,create}
otx audit finding {list,get,create,update}
otx audit plan {list,get}

otx incident list --stage IN_PROGRESS
otx incident create --file body.json --confirm
otx incident workflow-advance <id> --to RESOLVED --confirm

otx policy policy {list,get,create,update}
otx policy notice {list,get}
otx policy template list
```

### `otx user` (SCIM 2.0) / `otx role` / `otx credential` / `otx org` / `otx webhook`

```bash
otx user list --filter 'userName eq "alice"'
otx user create --file scim-user.json --confirm
otx user update <id> --file patch.json --confirm
otx user group add-user <group-id> --user-ids <id1>,<id2> --confirm
otx user schema list

otx role list
otx role scope grant <role-id> CONSENTMANAGER_WRITE --confirm
otx role scope revoke <role-id> AUDIT_READ --confirm

otx credential client {list,create,delete}
otx credential api-key {list,create,delete}     # legacy; warn-on-use

otx org {list,get,create,delete}

otx webhook subscribe --file body.json --confirm
otx webhook event {list,replay}
```

### `otx attach` / `otx export` / `otx auditlog`

```bash
otx attach upload --file evidence.pdf --confirm     # max 64MB
otx attach download <id> --out file.bin
otx attach subject-zip <data-subject-id>            # downloads .zip

otx export bulk create --type CONSENT_RECEIPTS --from 2026-05-01 --to 2026-05-19 \
                       --await --out receipts.csv --confirm
otx export bulk status <id>
otx export bulk download <id> --out file.csv
otx export poll <id> --timeout 30m

otx auditlog login-history --from 24h
otx auditlog activity search --user <id> --action modified
```

### `otx esg` / `otx ethics` / `otx coi`

```bash
otx esg metric list --from 2026-Q1 --to 2026-Q2     # ⚠ deprecated June 2025
otx esg framework list
otx esg report generate --file body.json --confirm

otx ethics case {list,get,update}                   # 404 → kind:not_publicly_documented
otx ethics hotline-config get

otx coi disclosure {submit,list,get}
otx coi approval {list,approve}
```

### `otx config` / `otx auth` (Phase 1 surfaces, unchanged)

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
