---
type: Architecture Decision
title: ADR-0002 Pin the CI Go toolchains and lint with golangci-lint v2
description: CI runs the unit tests on exactly pinned Go 1.26.10 and 1.27.3 toolchains and lints with golangci-lint v2; the earlier 1.21-1.27 matrix was cut to 1.26/1.27 for a crypto/x509 security fix and then pinned to exact patches.
status: accepted
date: 2026-10-27
deciders: []
supersedes: []
affects: [alpaca_trade_api_go]
allium: []
evidence: [".github/workflows/go.yml", "go.mod", "Makefile", ".golangci.yml", "commit fe23999 ci: run tests across Go 1.21-1.27 (#375)", "commit 8827711 ci: migrate golangci-lint to v2 so lint can run on Go 1.25+ (#379)", "commit d0a9719 Bump the go-modules group across 1 directory with 6 updates (#378)", "commit ea2aedb Update Go version to 1.26.10 (#386)", "commit 6ee44e6 SRE-50: Patch CI language runtimes for eMudhra CVEs (#387)"]
tags: [alpacahq, adr, generated]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---

# ADR-0002 Pin the CI Go toolchains and lint with golangci-lint v2

## Context
Commit `fe23999` (2026-09-14, "ci: run tests across Go 1.21–1.27") widened the test matrix to
`[1.21.x .. 1.27.x]`, and commit `8827711` ("ci: migrate golangci-lint to v2 so lint can run on
Go 1.25+") records that the previous linter version could not run on recent Go releases.
Commit `ea2aedb` (2026-10-23, "Update Go version to 1.26.10") raised `go.mod` from `go 1.24.0` to
`go 1.26.10` and cut the matrix to `[1.26.x, 1.27.x]`; its message states the reason: picking up
the security fix for GO-2026-6809 / CVE-2026-111943 ("Distrust compromised intermediate
certificates in crypto/x509"). Commit `6ee44e6` (2026-10-27, "Patch CI language runtimes for
eMudhra CVEs") then pinned the floating minors to exact patches `[1.26.10, 1.27.3]` because
`1.26.x` resolved to the cached, unpatched `1.26.9` on hosted runners (commit body).

> Unverified: why the matrix was widened in `fe23999` in the first place (for example, a spread of
> consumer toolchains) is not recorded beyond the commit title — confidence: low.

## Decision
The `test` job runs `go test -cover -timeout 6s ./...` over the matrix `[1.26.10, 1.27.3]`,
and the `lint` job runs `golangci/golangci-lint-action` at `v2.13.2` on Go 1.27.3, then
`go build -v ./...`
(`.github/workflows/go.yml`). The `Makefile` derives the local linter version by parsing that same
workflow file so local and CI linting agree. `.golangci.yml` is the v2-format configuration
(`version: "2"`).

## Consequences
The linter configuration must stay v2-compatible (`.golangci.yml`). Bumping the linter means
editing one place, since the `Makefile` reads the version out of the workflow.

Pinning exact patch versions means a security fix in Go requires a deliberate edit to the
workflow rather than arriving through a floating minor pin (commit `6ee44e6`).

> Unverified: whether toolchains older than 1.26.10 are still supported for consumers. `go.mod`
> sets the floor at `go 1.26.10` and CI no longer exercises older versions. The only written
> statement, `.golangci.yml:82,85` ("we support go 1.21"), predates `ea2aedb` and is now stale —
> confidence: low.

## Alternatives considered
Not evidenced in the repository.
