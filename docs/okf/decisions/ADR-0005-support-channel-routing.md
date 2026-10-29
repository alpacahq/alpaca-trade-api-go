---
type: Architecture Decision
title: ADR-0005 Route support questions by channel
description: The README and issue template split library bugs (GitHub Issues) from API bugs (a separate API repository), community questions (forum, Slack), account help and outage status.
status: accepted
date: 2026-08-13
deciders: []
supersedes: []
affects: [alpaca_trade_api_go]
allium: []
evidence: ["README.md", ".github/ISSUE_TEMPLATE/config.yml", "commit 2164365 docs: add support channel routing to README and getting started (#373)", "commit 3d1f211 docs: route API, spec, and docs bugs to alpacahq/alpaca-api (#385)"]
tags: [alpacahq, adr, generated]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---

# ADR-0005 Route support questions by channel

## Context
Commit `2164365` (2026-08-13, "docs: add support channel routing to README and getting started")
is, by its title, where the routing first appeared in `README.md`; it is the grafted root of this
shallow clone (54 files, 34891 insertions), so its diff cannot show what it added. Commit `3d1f211`
(2026-10-15, "docs: route API, spec, and docs bugs to alpacahq/alpaca-api") refined the section
and added `.github/ISSUE_TEMPLATE/config.yml` with contact links so that API-side reports are
redirected before an issue is opened here; that diff is visible. The `date` above is the first
decision; the latest revision is 2026-10-15.

> Unverified: the motivation for the change is not recorded beyond the commit titles, and the
> original `2164365` change set cannot be inspected in the shallow clone — confidence: low.

## Decision
`README.md` directs library and SDK issues (bugs, feature requests, questions specific to the Go
library) to this repository's GitHub Issues; bugs in the Alpaca API, its OpenAPI specs or its
documentation to the `alpacahq/alpaca-api` repository; API feature requests, general questions and
dashboard bugs to the Alpaca Community Forum or Slack; account help to a support request form; and
outage information to the status page. `.github/ISSUE_TEMPLATE/config.yml` repeats the API, forum
and Slack links as contact links. The same section tells users to include `RequestID` from
`*alpaca.APIError` (the `X-Request-ID` header) for failed API calls.

## Consequences
Repository issues stay scoped to the library; API behaviour questions are deferred, consistent with
`README.md` stating that per-API behaviour is covered by the online API documents. Badges in
`README.md` link the forum and Slack destinations.

## Alternatives considered
Not evidenced in the repository.
