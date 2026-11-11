# PLAN.md: permidex, a GCP IAM role and permission browser

`permidex` (permission index).

## 1. What it is

A single static Go binary: a terminal UI to browse GCP predefined roles and permissions, with linked pivots:

- find a permission, see which roles grant it
- find a role, see its permissions
- find a service prefix (`compute`), see its permissions

Deliberately small. Non-goals for the MVP: custom roles, org/project policy lookups (who has what), writing or granting anything, a web UI, a database.

## 2. Data

### Source

One call, no per-role fetches:

`GET https://iam.googleapis.com/v1/roles?view=FULL&pageSize=1000[&pageToken=…]`

- `view=FULL` returns `includedPermissions` on each role. Without it the permission lists are missing.
- Paginate with `nextPageToken`. Default page size is 300, max 1000.
- Leave `parent` empty. That returns predefined roles only.
- `showDeleted` stays false.

### Auth (verified in Google's roles.list reference)

The call needs an OAuth token with scope `https://www.googleapis.com/auth/cloud-platform` (or `iam`). It is not anonymous. The reference gives no anonymous path. Use Application Default Credentials (`golang.org/x/oauth2/google`, `google.FindDefaultCredentials`). No `gcloud` binary is needed at runtime. ADC can come from a gcloud login file, a service account key, or workload identity, but gcloud is still an easy way to create the ADC file.

Assumed: no quota project is needed. Confirm in step 2.

### Cache

- Path: `$XDG_CACHE_HOME/permidex/roles.json`, falling back to `~/.cache/permidex/roles.json`.
- Written by `permidex refresh`, read by the TUI.
- Write to a temp file, then rename (atomic).
- Store the fetch time. The TUI shows "data from <date>" in the header.
- Missing cache: the TUI exits with "run `permidex refresh`". It does not fetch on its own.

### Derived in memory

Built on load, not stored:

- `perm → []role` (reverse index)
- `svc → []perm` (prefix before the first `.`)
- flat search list of roles, permissions and services

## 3. Embedded snapshot: deferred

Not in the MVP. A `go:embed` snapshot plus a nightly CI refresh would make the binary work offline, but it needs a GCP credential in CI. That is an access decision, so ask first (AGENTS: no widening access without asking). Add it when there are real users.

## 4. Architecture

Module: `github.com/lalu-uk/permidex`.

```
cmd/permidex/        main: subcommands `refresh` and default TUI
internal/iam/       fetch (paged roles.list, ADC token), types
internal/cache/     read/write roles.json atomically, staleness
internal/index/     reverse index, service grouping, search list
internal/tui/       bubbletea model: list, search, pivot stack, detail view
```

- `internal/iam` depends on an interface for the HTTP client so tests use a fake server (`httptest`). No live GCP calls in tests.
- `internal/tui` depends on `index`, not on `iam`.
- Keep the Bubble Tea model in one file to start. Split only when it hurts.

## 5. TUI

Built with Bubble Tea, `bubbles/list` or `bubbles/textinput` for search, lipgloss for styling. Keep the styling plain.

### Keys

| key | action |
|---|---|
| type | fuzzy filter over the current list |
| `enter` | detail for the selected item |
| `enter` | open the selected item (drill-down view) |
| `esc` | back (clears the search first) |
| `backspace` | delete, or back when the search is empty |
| `ctrl+o` or `home` | reset to the start screen |
| `?` | key help |
| `ctrl+c` | quit |

Pivot keys (`ctrl+r`, `ctrl+p`, `ctrl+s`) were replaced by drill-down: every result is selectable, and opening it shows its related items.

### Pivot stack

Every pivot pushes the previous list and filter onto a stack. `esc` pops. This fixes the "no back stack" gap in the old draft.

### Detail view

- role: title, description, stage, permission count, full permission list
- permission: roles that grant it
- service: permission count, role count

## 6. Testing

- `go test ./...` after every non-trivial change. `golangci-lint run` too (same config as Propsy: gci, gofumpt, errcheck, gosec, revive, staticcheck, unparam).
- `internal/index`: table tests for reverse index and service grouping, using a small fixture `roles.json` in `testdata/`.
- `internal/iam`: fake server with two pages, checks pagination and that `view=FULL` is sent.
- `internal/cache`: atomic write, missing file, stale date.
- TUI: a few model-update tests on key sequences (pivot and pop). No snapshot tests.

## 7. Tooling

`mise.toml` pins everything (per AGENTS: exact versions, no `latest` in project config):

- `core:go`: current stable, pinned exact
- `github:golangci/golangci-lint`: pinned exact
- `github:goreleaser/goreleaser` or `aqua:` only if no `github:` source works

Releases: GoReleaser, draft then publish (lalu-uk org rules: immutable releases, actions pinned by SHA). Ship darwin and linux, amd64 and arm64.

## 8. Open questions

1. Confirm roles.list works with a plain ADC user token and no quota project (step 2). Confirmed: works with a user ADC token, no quota project, 2429 roles.
2. Is `showDeleted` ever wanted? Default no.

## 9. Steps

1. `go mod init`, `mise.toml`, `.golangci.yaml`, `.gitignore`.
2. Manual check: one `curl` with a token from `gcloud auth application-default print-access-token`. Confirm the response shape and `includedPermissions`. Save a small `testdata/roles.json`.
3. `internal/iam` fetch, paged, with fake-server tests.
4. `internal/cache` and `permidex refresh`.
5. `internal/index` with tests.
6. `internal/tui`: list, search, detail, pivots, back stack.
7. README: install, `permidex refresh`, keys.

## 10. v1.1.0: fuzzy search

Goal: type loosely (`cmpadm` → `roles/compute.admin`) and still get sensible results, with exact substring matches always ranked first.

### Behavior

1. Exact substring matches first, shortest name first.
2. Then subsequence matches, scored by tight gaps and word-boundary hits (`.`, `/`).
3. Within a tier, keep the current stable order.
4. Match on name only. Title matching is out of scope for v1.1.0.

### Steps

1. Add a scoring function (new `internal/match` package, or a small file in `internal/tui`). No dependency; `github.com/sahilm/fuzzy` is the fallback if the scoring gets hard to own.
2. Wire it into `refresh()` in `internal/tui/tui.go`, replacing the substring filter.
3. Tests: table of query → expected top results; substring beats subsequence; empty query keeps the original order.
4. Measure per-keystroke filter time on the real cache (2429 roles plus their permissions and services). Only add an index if it is slow.
5. Tune the ranking on real queries, then update the README keys section if the behavior needs describing.

### Done when

- `go test -race ./...` and `golangci-lint run` pass.
- Real-cache queries feel right on a manual run.
- Tagged v1.1.0 via the release flow.

### Services (decision)

GCP's roles API has no service field and no permission-to-service mapping, so a service is only the first dot-segment of a permission name (`compute`). It is browsable and selectable as a convenience and written to the YAML as-is. It is not an API hostname, so no `.googleapis.com` FQDN is derived.
