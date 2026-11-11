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

| key | action |
|---|---|
| type | filter the current list |
| `enter` | detail for the selected item |
| `ctrl+r` | roles that grant the selected permission |
| `ctrl+p` | permissions in the selected role |
| `ctrl+s` | permissions in the selected service (or the service of the selected permission) |
| `esc`, or `backspace` on an empty filter | back (pop the pivot stack) |
| `ctrl+o`, `home` | reset to the full index |
| `up` `down` `pgup` `pgdown` | move |
| `q`, `ctrl+c` | quit (`q` only on an empty filter, so you can type it) |

## Development

```sh
go test -race ./...
golangci-lint run
```

Tests use no live GCP calls. See `PLAN.md` for design and `AGENTS.md` for notes.

## License

MIT. See `LICENSE.md`.
