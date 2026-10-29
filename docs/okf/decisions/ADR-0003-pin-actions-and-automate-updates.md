---
type: Architecture Decision
title: ADR-0003 Pin GitHub Actions to SHAs and automate dependency updates
description: CI actions are pinned to commit SHAs and Dependabot raises grouped weekly updates for Actions and Go modules.
status: accepted
date: 2026-09-14
deciders: []
supersedes: []
affects: [alpaca_trade_api_go]
allium: []
evidence: [".github/workflows/go.yml", ".github/dependabot.yml", "commit 87b3134 ci: pin Actions to SHAs and add Dependabot for Actions and Go modules (#376)", "commit 1b208b2 Bump the github-actions group across 1 directory with 3 updates (#377)", "commit d0a9719 Bump the go-modules group across 1 directory with 6 updates (#378)"]
tags: [alpacahq, adr, generated]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---

# ADR-0003 Pin GitHub Actions to SHAs and automate dependency updates

## Context
Commit `87b3134` is titled "ci: pin Actions to SHAs and add Dependabot for Actions and Go modules",
which states both the supply-chain concern (mutable action tags) and the mitigation.

## Decision
Every action in `.github/workflows/go.yml` is referenced by full commit SHA with the human tag in a
trailing comment (for example `actions/checkout@3d3c42e... # v7.0.1`), and
`.github/dependabot.yml` configures weekly updates for the `github-actions` and `gomod`
ecosystems, each with a single catch-all group.

## Consequences
Action and module updates arrive as reviewable grouped pull requests (commit `1b208b2`, "Bump the
github-actions group across 1 directory with 3 updates"; commit `d0a9719`, "Bump the go-modules
group across 1 directory with 6 updates") rather than silently through tag movement. The trade-off
is routine update traffic that must be merged to stay current, and a grouped module bump can move
the `go` directive in `go.mod` (to 1.24.0 in `d0a9719`, later raised by hand to 1.26.10 in `ea2aedb`) alongside library versions.

## Alternatives considered
Not evidenced in the repository.
