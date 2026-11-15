# PLAN-aws.md: `permidex aws`

Status: in progress on `feat/aws`. Done: service reference fetch, managed policy read, wildcard expansion, cache, browser source and `permidex aws`. Still to do: incremental refresh (reuse policy versions and service documents that haven't changed), schema for the AWS cache, the AWS skill, and a timing and size check on a full refresh.

Later: publish a canonical snapshot as a release asset, as noted under "Timing and shared data" below.

## Goal

The same browsing as `permidex gcp`, for AWS: find which managed policies grant an action, what a policy includes, and which actions a service has. Same keys, same YAML selection output, same drill-down.

## Sections

In display order, matching the GCP order (the things you'd usually start from, then the things beneath them, then services):

1. **Policies:** AWS managed policies. Opening one lists its actions.
2. **Actions:** IAM actions, such as `s3:GetObject`. Opening one lists the policies that grant it, then its service.
3. **Services:** service prefixes, such as `s3`. Opening one lists its actions, then its policies.

The service name shown for a service is its human-readable `ServiceName` (for example `S3`), with the prefix as the ID. This is the one place AWS gives us real names that GCP doesn't.

## Sources

**Actions and services (no auth):** the service reference, the public JSON AWS publishes for programmatic access.
- Index: `https://servicereference.us-east-1.amazonaws.com/` lists services with a `url` for each.
- Per service: `https://servicereference.us-east-1.amazonaws.com/v1/<service>/<service>.json`, with `Name`, `ServiceName`, `Actions` (each with `Name`, `Annotations.Properties` such as `IsWrite` and `IsDataPlane`, and `ActionConditionKeys`), and `Operations`.
- Not needed for v1: global condition keys, the SDK mapping.

**Managed policies (auth):** IAM.
- `ListPolicies` with `Scope=AWS` (AWS-managed policies only, as the built-in equivalent of GCP's predefined roles). Paginate with `Marker` / `IsTruncated`.
- `GetPolicy` for the description and `DefaultVersionId`.
- `GetPolicyVersion` for the document, then collect each statement's `Action` (and note `NotAction`, see open questions).

## Matching

A policy grants an action when one of its `Action` patterns matches. Patterns can contain wildcards: `s3:*`, `s3:Get*`, `*`. So the index has to expand patterns against the service reference's action list, not just compare strings. This is the main thing that differs from GCP's exact `includedPermissions` lookup. Expansion happens at refresh time, so browsing stays fast.

## Cache

One file, since the refresh does both sources: `$XDG_CACHE_HOME/permidex/aws/catalog.json` (or `~/.cache/...`).
- `version` (starts at 1), `fetchedAt`.
- `services[]`: `prefix`, `name` (ServiceName), `actions[]` with their annotations.
- `policies[]`: `name`, `arn`, `description`, `actions[]` as written in the document (patterns), and `grants[]` as expanded action IDs.

Schema: `schema/aws-catalog.schema.json`, checked against the structs by a test, as for GCP.

## Code

- `internal/aws` implements `browse.Source`, and holds the fetch, refresh and cache, in the same shape as `internal/gcp`.
- The shared `internal/browse` should need no changes. If it does, that's a sign the `Source` interface is missing something GCP also needs, and that's worth a decision first.
- `cmd/permidex`: add `aws` to the provider switch. `--refresh|-r` refreshes the catalogue.

## Auth and permissions

AWS default credential chain (environment, shared config, SSO, instance role). The refresh needs only `iam:ListPolicies`, `iam:GetPolicy` and `iam:GetPolicyVersion`. The service reference needs no credentials. Don't ask for broader IAM access without checking first.

## Progress

`--refresh|-r` prints a line per service reference fetch and per policy batch to stderr, the same as GCP's page progress. Speed isn't a priority: the output is cached.

## Skill

`skills/permidex-aws`, separate from `skills/permidex-gcp`. Same structure: where the cache is, how to refresh, `jq` recipes for actions, policies, and wildcard matches. Keep it free of personal paths.

## Open questions

1. **`NotAction`:** ignore in v1, or show policies that allow everything except an action? Ignoring it is simpler and under-reports, so the skill and the browser would need to say so.
2. **Customer-managed policies:** out of scope for v1 (`Scope=AWS` only). Confirm that's wanted.
3. **Refresh cost:** `GetPolicyVersion` per policy. Probably fine, since it's cached, but check the throttling behaviour once it's implemented.
4. **Wildcard display:** a policy that uses `s3:*` lists every expanded action. Show the pattern in the policy's actions, and the expanded set in the action lookup, or just the expansion?

## Timing and shared data

A full refresh is about 2,000 calls. Two things would cut that down:
- **Incremental refresh:** reuse a cached policy document when its `DefaultVersionId` hasn't changed, and refetch a service document only when its `modified` timestamp changes.
- **A shared snapshot:** CI publishes the catalogue as a versioned release asset, and `permidex aws --fetch` downloads it, opt-in. The public service reference needs no credentials, so CI can do that part. Policies need an IAM read identity in CI, which is an access decision, so it needs sign-off first.

## Tabled: pull the release artifact instead of a full refresh

A first run currently means a full refresh. A flag such as `permidex aws --pull` (and `permidex gcp --pull`) would download the latest release's artifact instead, then the normal delta refresh brings it up to date. It saves the first-run cost and needs no credentials for the public part.

It comes with trust: the artifact is whatever CI published, so the flag should verify a checksum from the same release and say where the data came from. Convenience costs, so it stays opt-in.

## Idea, not planned: interrupt tolerance and resumable refresh

A full refresh takes minutes, and an interrupted one currently starts over. Checkpointing the cache after each batch would let a refresh resume. It adds state and complexity, so it waits until a full refresh is interrupted often enough to matter. The delta refresh already makes most runs short.

## Steps

1. `internal/aws`: service reference fetch (public, no auth), with tests against recorded JSON in `testdata/`.
2. Policy fetch: `ListPolicies`, `GetPolicy`, `GetPolicyVersion`, with fake-client tests for pagination.
3. Wildcard expansion, with table tests (`s3:*`, `s3:Get*`, `*`, exact match, no match).
4. Cache, schema, drift test.
5. `aws` `Source` implementation, with tests for the three views.
6. CLI: `permidex aws` and `--refresh|-r`.
7. README, AGENTS.md, skill.
8. Release as a separate minor version, not part of the v2 change.
