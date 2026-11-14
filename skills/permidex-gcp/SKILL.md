---
name: permidex-gcp
description: Look up GCP predefined IAM roles and their permissions offline, from the permidex GCP cache. Use when you need a role's ID, title, description or permissions, which roles grant a permission, or the permissions in a role. Triggers: IAM role, roles/..., which role grants, permission lookup, least privilege.
---

# permidex-gcp

`permidex gcp` caches GCP's predefined IAM roles in a JSON file. Query that file directly. It is the offline index, so you don't need the TUI or the network.

## Where the file is

`gcp/roles.json` in the cache directory. Ask permidex for that directory rather than working it out: `permidex --cache-dir` (or `-c`) prints it and exits 0. It honours `PERMIDEX_CACHE_DIR` and `XDG_CACHE_HOME`, falling back to `~/.cache/permidex`. An invalid `XDG_CACHE_HOME` is ignored, as the XDG spec says, with a warning on stderr. A relative `PERMIDEX_CACHE_DIR` makes the command fail with an error, so treat that as a configuration problem, not a missing cache. `permidex gcp -r` prints the exact path it wrote.

Its format is defined in `gcp-roles.schema.json`, which sits in the same directory. The same schema is in the permidex repository at `schema/gcp-roles.schema.json`. It has `version` (currently `1`), `fetchedAt`, and `roles[]`. Each role has `name` (such as `roles/compute.admin`), and may have `title`, `description`, `stage` and `includedPermissions`. Empty fields are omitted.

The file is about 10 MB. Query it with `jq`. Don't read it whole.

## Missing or stale file

If the file is missing, or its `version` isn't `1`, build it. This needs Application Default Credentials and read access to IAM:

```sh
gcloud auth application-default login
permidex gcp --refresh|-r
```

Check `fetchedAt` if the data might be out of date. Refresh when it matters.

## Queries

Set `F` to the path:

```sh
D=$(permidex --cache-dir) || exit 1
F="$D/gcp/roles.json"
```

Role by exact ID:

```sh
jq --arg n roles/compute.admin '.roles[] | select(.name == $n)' "$F"
```

Find roles by title or ID text (case-insensitive):

```sh
jq -r '.roles[] | select(((.title // "") + " " + .name) | test("storage viewer"; "i")) | .name' "$F"
```

Roles that grant a permission:

```sh
jq -r --arg p compute.instances.get '.roles[] | select((.includedPermissions // []) | index($p)) | .name' "$F"
```

Permissions in a role:

```sh
jq -r --arg n roles/compute.viewer '.roles[] | select(.name == $n) | (.includedPermissions // [])[]' "$F"
```

Permissions matching text, across all roles:

```sh
jq -r '[.roles[] | (.includedPermissions // [])[]] | unique[] | select(test("instances"))' "$F"
```

## Notes

- A permission's service is its first dot-segment (`compute` in `compute.instances.get`). The API has no permission-to-service mapping, so don't derive hostnames such as `compute.googleapis.com` from it.
- Use the full role ID (`roles/...`) when you write it down or pass it on. Titles are for reading, not identifiers.
- Basic roles (`roles/owner`, `roles/editor`, `roles/viewer`, `roles/browser`) are in the file too.
