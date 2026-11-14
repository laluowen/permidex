# permidex: agent notes

Terminal browser for GCP predefined IAM roles and permissions. One static Go binary. See `PLAN.md` for design.

## Layout

- `cmd/permidex`: entrypoint. `permidex <provider>` browses; `permidex <provider> --refresh|-r` refreshes that provider's cache.
- `internal/browse`: the shared Bubble Tea browser. Knows nothing about any provider. Everything goes through `browse.Source`.
- `internal/gcp`: the GCP provider. Cache load and refresh, and the `browse.Source` implementation.
- `internal/iam`: paged `roles.list` (`view=FULL`) over an authorized `*http.Client`; `NewADCClient` builds one from ADC.
- `internal/index`: GCP reverse lookups and service grouping, built once in memory.
- `internal/cache`: generic JSON cache files under `~/.cache/permidex` (or `$PERMIDEX_CACHE_DIR`, or `$XDG_CACHE_HOME/permidex`). Atomic write, read, path.
- `schema/`: the GCP cache schema, `gcp-roles.schema.json`, embedded as the `schema` package. Each refresh writes it beside the cache. Tests validate real output against it (`santhosh-tekuri/jsonschema`) and check its property names against the structs.
- `skills/permidex-gcp`: agent skill for querying the GCP cache.
- `testdata/roles.json`: real `roles.list` output (4 roles). Tests use it.

## Testing

- `go test -race ./...` and `golangci-lint run` after every non-trivial change. No live GCP calls in tests.
- `testdata/roles.json` is a fixture. Don't hand-edit it beyond trimming.

## Decisions

- Navigation is drill-down: `enter` opens an item; `esc` goes back (clears the search first). Arrow left/right and `backspace` do not navigate, so the search text stays editable. Bare letters and space are search input, so selection is `tab`. `ctrl+j`, `ctrl+k`, `ctrl+l` jump to the first three sections, the same for every provider. `ctrl+q` quits and prints the selection as YAML; `ctrl+c` quits with no output (bound, not shown in the footer). There is no help screen: every key is in the footer.
- Providers implement `browse.Source`. Keep provider knowledge (roles, permissions, services, policies, the names of kinds) out of `internal/browse`.
- GCP services are the first dot-segment of a permission (`compute`), shown and written to the YAML as-is. GCP exposes no permission-to-service mapping and the roles API has no service field, so there is no FQDN (`*.googleapis.com`): deriving it would guess. Don't add FQDN mapping or per-service exceptions.
- Auth: ADC only. No `gcloud` binary at runtime.

## Commits

Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`; `!` or a `BREAKING CHANGE:` footer for breaking changes).
