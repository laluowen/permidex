# permidex

Browse GCP predefined IAM roles and permissions in the terminal. Find which roles grant a permission, what a role includes, or every permission under a service.

## Install

Build from source (Go 1.27+):

```sh
go install github.com/lalu-uk/permidex/cmd/permidex@latest
```

Or download a release archive for darwin or linux (amd64, arm64) and put `permidex` on your `PATH`.

## Use

Authenticate with Application Default Credentials (no quota project is needed):

```sh
gcloud auth application-default login
```

Fetch the roles into the cache, then browse:

```sh
permidex refresh
permidex
```

The cache is `$XDG_CACHE_HOME/permidex/roles.json`, or `~/.cache/permidex/roles.json`. `permidex` never fetches on its own.

## Keys

Start with a blank screen and type to search roles (by ID or title), permissions and services. Each result is selectable. Opening one shows its details and related items: a role lists its permissions; a permission shows its service and the roles that grant it; a service lists its permissions and roles.

| key | action |
|---|---|
| type | search the current list |
| `enter` | open the selected item |
| `esc` | back (clears the search first) |
| `backspace` | delete a character, or back when the search is empty |
| `↑` `↓` `pgup` `pgdn` | move |
| `ctrl+o`, `home` | reset to the start screen |
| `?` | key help (`?` or `esc` closes it) |
| `ctrl+c` | quit |

## Development

```sh
go test -race ./...
golangci-lint run
```

Tests use no live GCP calls. See `PLAN.md` for design and `AGENTS.md` for notes.

## License

MIT. See `LICENSE.md`.
