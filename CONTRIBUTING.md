# Contributing to oc-tnf

## Prerequisites

- Go 1.25+
- Access to an OpenShift TNF (DualReplica) cluster for E2E testing

## Development workflow

1. Fork and clone the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run `make test` and `make golangci-lint`
5. Open a PR against `openshift/oc-tnf`

## CI

PRs are gated by Prow (OpenShift CI). The following jobs run on every PR:

- **unit** — `make test`
- **golint** — `make golangci-lint`
- **modtidy** — `go mod tidy && git diff --exit-code`
- **verify-deps** — dependency verification

## Merge process

PRs require the following Prow labels to merge via Tide:

- `lgtm` — added by a reviewer with `/lgtm`
- `approved` — added by an approver (listed in `OWNERS`) with `/approve`

## Adding a new subcommand

1. Create `pkg/cmd/<name>/<name>.go` with `NewCmd<Name>(streams)` following the Complete/Validate/Run pattern
2. Add `cmd.AddCommand(...)` in `pkg/cmd/root.go`

## Commit conventions

- Use imperative mood in commit messages
- Reference Jira tickets when applicable (e.g., `OCPEDGE-XXXX: ...`)
