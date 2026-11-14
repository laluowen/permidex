# permidex

Browse GCP predefined IAM roles and permissions in the terminal. Find which roles grant a permission, what a role includes, or every permission under a service.

## Install

Build from source (Go 1.27+):

```sh
go install github.com/laluowen/permidex/v2/cmd/permidex@latest
```

Or download a release archive for darwin or linux (amd64, arm64) and put `permidex` on your `PATH`.

### mise

[mise](https://mise.jdx.dev) installs the binary straight from the GitHub release:

<!-- x-release-please-start-version -->
```sh
mise use -g github:laluowen/permidex@latest  # the newest release
mise use -g github:laluowen/permidex@2.1.0   # or pin this exact version
```
<!-- x-release-please-end -->

Then run `permidex gcp`. By default mise holds releases back for 24 hours (`minimum_release_age`), so `@latest` only picks a release up a day after it's published.

## Use

Authenticate with Application Default Credentials (no quota project is needed):

```sh
gcloud auth application-default login
```

Fetch the roles into the cache, then browse:

```sh
permidex gcp --refresh|-r   # refresh the cache from GCP
permidex gcp                # browse it
```

The cache is `permidex/gcp/roles.json` under `~/.cache`. To put it elsewhere, set `$XDG_CACHE_HOME` (the cache then lives in `$XDG_CACHE_HOME/permidex`) or `PERMIDEX_CACHE_DIR` (the cache files go straight into that directory). `PERMIDEX_CACHE_DIR` wins if both are set. `PERMIDEX_CACHE_DIR` must be an absolute path, or permidex stops with an error. A relative `XDG_CACHE_HOME` is ignored, as the XDG spec says. `permidex --cache-dir` (or `-c`) prints the directory it uses, which is handy in scripts. Each refresh also writes `gcp-roles.schema.json` into the same directory, so the cache describes itself. Browsing never fetches on its own.

Upgrading from 1.x: the command is now `permidex gcp`, and `permidex refresh` is `permidex gcp --refresh|-r`. The cache moved to `gcp/roles.json`, so run `permidex gcp --refresh|-r` once.

## Agents

The cache is plain JSON, so agents and scripts can query it directly. [`skills/permidex-gcp`](./skills/permidex-gcp/SKILL.md) is an agent skill with `jq` recipes. The format is defined in [`schema/gcp-roles.schema.json`](./schema/gcp-roles.schema.json).

## Keys

Start with a blank screen and type to search roles (by ID or title), permissions and services. Each result is selectable. Opening one shows its details and related items: a role lists its permissions; a permission shows its service and the roles that grant it; a service lists its permissions and roles.

| key | action |
|---|---|
| type | search the current list |
| `enter` | open the selected item |
| `esc` | back (clears the search first) |
| `backspace` | delete a character |
| `tab` | select or deselect the item |
| `ctrl+j`, `ctrl+k`, `ctrl+l` | jump to the first, second or third section (for GCP: roles, permissions, services) |
| `↑` `↓` `pgup` `pgdn` | move |
| `home` `end` | jump to the top or bottom of the list |
| `ctrl+o` | reset to the start screen |
| `ctrl+q` | quit and print the selection to stdout as YAML (the TUI draws on stderr) |

## Development

```sh
go test -race ./...
golangci-lint run
```

Tests use no live GCP calls. See `PLAN.md` for design and `AGENTS.md` for notes.

## License

MIT. See `LICENSE.md`.
