---
type: Architecture Decision
title: ADR-0001 Surface the API request id on typed errors
description: API failures are returned as a typed APIError that carries the X-Request-ID response header so support can trace the call.
status: accepted
date: 2026-09-24
deciders: []
supersedes: []
affects: [alpaca_trade_api_go.trading_client]
allium: []
evidence: ["alpaca/entities.go:355-388", "alpaca/rest.go:380", "README.md", "commit e08acc3 Add X-Request-ID information to Go SDK (#382)"]
tags: [alpacahq, adr, generated]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---

# ADR-0001 Surface the API request id on typed errors

## Context
Callers who hit a failing API call need something the service side can correlate. The trading API
returns an `X-Request-ID` header, and `README.md` instructs users: "For failed API calls, include
`RequestID` from `*alpaca.APIError` when contacting Alpaca support". Commit `e08acc3`
("Add X-Request-ID information to Go SDK") introduced the field.

## Decision
Non-2xx responses are converted into a typed `APIError` whose `RequestID` field is populated from
the `X-Request-ID` response header, including when the body cannot be read or parsed as an API
error (`alpaca/entities.go:355-388`); the `CloseAllPositions` per-symbol error path sets the same field (`alpaca/rest.go:341,375-381`; commit `e08acc3` body).
The field is excluded from JSON (`json:"-"`) because it comes from a header, not the body.

## Consequences
Error handling examples in `README.md` can tell users exactly what to quote in a support request,
and the id is available even for malformed error bodies. The field must be filled at every place a
response becomes an error, so new request paths have to remember it
(`alpaca/entities.go`, `alpaca/rest.go`).

## Alternatives considered
Not evidenced in the repository.
