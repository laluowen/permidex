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

- Navigation is drill-down: `enter` opens an item; `esc`, or `backspace` on an empty search, goes back. Arrow left/right are not bound, so the search text stays editable. Bare letters are search input, so there are no single-letter commands. `ctrl+c` is the only quit key.
- Auth: ADC only. No `gcloud` binary at runtime.

## Commits

Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`; `!` or a `BREAKING CHANGE:` footer for breaking changes).
