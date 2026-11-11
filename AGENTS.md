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

- Navigation is drill-down: `enter` opens an item; `esc` goes back (clears the search first). Arrow left/right and `backspace` do not navigate, so the search text stays editable. Bare letters and space are search input, so selection is `tab`. `ctrl+q` quits and prints the selection as YAML; `ctrl+c` aborts with no output.
- Auth: ADC only. No `gcloud` binary at runtime.

## Commits

Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`; `!` or a `BREAKING CHANGE:` footer for breaking changes).
- Services are the first dot-segment of a permission (`compute`), shown and written to the YAML as-is. GCP exposes no permission-to-service mapping and the roles API has no service field, so there is no FQDN (`*.googleapis.com`): deriving it would guess. Don't add FQDN mapping or per-service exceptions.
