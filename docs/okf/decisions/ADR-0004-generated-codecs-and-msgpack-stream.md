---
type: Architecture Decision
title: ADR-0004 Generate JSON codecs and decode streams with msgpack
description: Entity marshalling uses generated easyjson code checked into the repository, while the market data stream is decoded with msgpack.
status: accepted
date: unknown
deciders: []
supersedes: []
affects: [alpaca_trade_api_go.trading_client, alpaca_trade_api_go.marketdata_client, alpaca_trade_api_go.marketdata_stream]
allium: []
evidence: ["alpaca/entities.go:17-18", "marketdata/entities.go:13-14", "marketdata/stream/entities.go:9-10", "alpaca/entities_easyjson.go", "marketdata/stream/message.go", "marketdata/stream/conn_coder.go", "go.mod", "Makefile", ".github/workflows/go.yml"]
tags: [alpacahq, adr, generated]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---

# ADR-0004 Generate JSON codecs and decode streams with msgpack

## Context
The SDK decodes high-volume market data and trading payloads. Entity files carry
`//go:generate easyjson -all ...` directives that install easyjson `v0.7.7`
(`alpaca/entities.go:17-18`, `marketdata/entities.go:13-14`,
`marketdata/stream/entities.go:9-10`), and the streaming package imports
`github.com/vmihailenco/msgpack/v5` (`go.mod`, `marketdata/stream/message.go`).

## Decision
JSON (un)marshalling for entities is generated with easyjson and the generated
`*_easyjson.go` files are committed; the websocket stream is framed with
`github.com/coder/websocket` (`marketdata/stream/conn_coder.go`) and its messages decoded with
msgpack (`marketdata/stream/message.go`, `marketdata/stream/flow.go`). Naming differs per package
(`-snake_case` for `alpaca` and `marketdata/stream`, `-lower_camel_case` for `marketdata`), matching
each endpoint's wire format.

## Consequences
Generated files are part of the review surface and must be regenerated when entities change: CI
runs `make generate` and fails on any diff ("Make sure the generated files are up-to-date",
`.github/workflows/go.yml`). `Makefile` exposes `generate` for that purpose.

> Unverified: the generator pinned in the directives (`v0.7.7`) differs from the runtime
> dependency in `go.mod` (`v0.9.2` after commit `d0a9719`); no file records whether that is
> intentional — confidence: low.

## Alternatives considered
Not evidenced in the repository.
