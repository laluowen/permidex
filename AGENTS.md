# permidex: agent notes

Terminal browser for GCP predefined IAM roles and permissions. One static Go binary. See `PLAN.md` for design.

## Layout

- `cmd/permidex`: entrypoint. `refresh` fetches into the cache; no args opens the TUI.
- `internal/iam`: paged `roles.list` (`view=FULL`) over an authorized `*http.Client`; `NewADCClient` builds one from ADC.
- `internal/cache`: `roles.json` read/write (temp file, sync, rename).
- `internal/index`: reverse lookups and service grouping, built once in memory.
- `internal/tui`: Bubble Tea model. Depends on `index`, not `iam`.
- `testdata/roles.json`: real `roles.list` output (4 roles). Tests use it.

## Testing

- `go test -race ./...` and `golangci-lint run` after every non-trivial change. No live GCP calls in tests.
- `testdata/roles.json` is a fixture. Don't hand-edit it beyond trimming.

## Decisions

- Pivot keys are `ctrl+r`, `ctrl+p`, `ctrl+s`, not bare `r`/`p`/`s`, because bare letters are filter input. `backspace` and `q` act as back and quit only when the filter is empty.
- Auth: ADC only. No `gcloud` binary at runtime.

## Commits

Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`; `!` or a `BREAKING CHANGE:` footer for breaking changes).
