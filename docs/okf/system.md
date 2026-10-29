---
type: System
title: Alpaca Go SDK (alpaca-trade-api-go)
description: Go client library for the Alpaca trading and market data APIs, for developers building trading algorithms in Go.
resource: likec4://alpacahq/alpaca_trade_api_go
tags: [alpacahq, sdk, trading, market-data, generated]
status: active
owners: ["@alpacahq/api-dx", "@gnvk", "@said-saifi"]
timestamp: 2026-10-29T08:55:36+00:00
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
links:
  repository: https://github.com/alpacahq/alpaca-trade-api-go
  architecture: ../architecture/alpaca_trade_api_go.c4
---

# Alpaca Go SDK (alpaca-trade-api-go)

## Purpose
`alpaca-trade-api-go` is a Go library for the Alpaca trade and market data API; it is described as
allowing "rapid trading algo development easily, with support for the both REST and streaming
interfaces" (`README.md`). It is consumed as a Go module
(`module github.com/alpacahq/alpaca-trade-api-go/v3`, `go 1.26.10` in `go.mod`), installed with
`go get -u github.com/alpacahq/alpaca-trade-api-go/v3/alpaca` (`README.md`). It is a library, not a
deployed service: there is no Dockerfile, compose, k8s or terraform file in the repository
(`git ls-files`).

The audience is developers writing trading programs against Alpaca's APIs; `examples/README.md`
describes example algorithms "that connect to the paper-trading API" and notes that API behaviour
itself is documented in the online API documents (`README.md`).

## Capabilities
- Trading REST client (account, orders, positions, assets, option contracts, corporate-action announcements, watchlists, clock, calendar) — evidence: `alpaca/rest.go` (endpoint builders `url.Parse(fmt.Sprintf("%s/%s/..."))`), `alpaca/entities.go`
- Limit-price rounding helper — evidence: `alpaca/trading.go` (`RoundLimitPrice`, the file's only function)
- Trade-updates stream over `/v2/events/trades`, including a background variant with reconnect — evidence: `alpaca/stream.go`, `README.md` ("Trade updates stream example")
- Historical and latest market data for stocks, crypto, options and news — evidence: `marketdata/rest.go`, `marketdata/options.go`
- Real-time market data streaming (stocks, crypto, crypto perps, options, news) with subscription management and reconnection — evidence: `marketdata/stream/client.go`, `marketdata/stream/options.go`, `marketdata/stream/subscription.go`
- Bounded retry on HTTP 429: up to 3 retries (default) with a 1s delay in the trading client; the market data client defaults to 10 retries — evidence: `alpaca/rest.go:29-30,61-65,103-115` (loop breaks when `i >= RetryLimit`), `marketdata/rest.go:73-74`
- Typed API errors carrying the `X-Request-ID` response header for support escalation — evidence: `alpaca/entities.go:355-388`, `alpaca/rest.go:380` (per-symbol `CloseAllPositions` errors), `README.md` ("Support")
- Runnable example strategies — evidence: `examples/README.md`, `examples/mean-reversion/mean-reversion.go`, `examples/long-short/long-short.go`

## Interfaces
**Inbound:** Go package API only — `alpaca.NewClient(alpaca.ClientOpts{...})`,
`marketdata.NewClient(...)`, `marketdata/stream` clients (`alpaca/rest.go`, `marketdata/rest.go`,
`marketdata/stream/client.go`). Credentials and hosts can also come from `APCA_API_*` and `DATA_*PROXY_WS` environment
variables (`alpaca/rest.go:44-60`, `marketdata/rest.go:58-71`, `marketdata/stream/options.go`,
`README.md`). No HTTP server, CLI or UI is present.

**Outbound:** HTTPS to the Alpaca trading API (live, paper and broker hosts, plus the trade-updates
event stream) and HTTPS/WebSocket to the Alpaca market data API. The concrete hosts, defaults and
environment overrides are modelled on `ext_alpaca_trading_api` and `ext_alpaca_market_data_api` in
`../architecture/alpaca_trade_api_go.c4` and are not repeated here.

## Dependencies
- `github.com/coder/websocket` v1.8.15 — WebSocket transport for the streaming client — evidence `go.mod`, `marketdata/stream/conn_coder.go`
- `github.com/vmihailenco/msgpack/v5` v5.4.1 — msgpack decoding of stream messages — evidence `go.mod`, `marketdata/stream/message.go`, `marketdata/stream/flow.go`
- `github.com/mailru/easyjson` v0.9.2 — generated JSON codecs for entities — evidence `go.mod`, `alpaca/entities_easyjson.go`, `marketdata/entities_easyjson.go`
- `github.com/shopspring/decimal` v1.4.0 — decimal money/quantity values — evidence `go.mod`, `README.md`
- `cloud.google.com/go` v0.123.0 (`civil` package only) — civil date types in request and entity structs — evidence `go.mod`, `marketdata/rest.go:15`, `alpaca/entities.go:11`
- `github.com/RobinUS2/golang-moving-average` — used by an example strategy — evidence `go.mod`, `examples/`
- `github.com/stretchr/testify` — tests only — evidence `go.mod`, `alpaca/rest_test.go`
- Alpaca Trading, Market Data and Broker API endpoints — the services the clients call — evidence `alpaca/rest.go`, `marketdata/rest.go`, `README.md`

## Data & storage
No datastore, queue, schema or migration is present in the repository (`git ls-files`). The library
holds no persistent state; the only data file is the streaming test fixture
`marketdata/testdata/trades.json.gz`. Credentials are supplied by the caller through client options
or `APCA_API_*` environment variables (`alpaca/rest.go:44-60`) and are not stored in the repository.

## Operations
Not deployed: the artefact is a Go module that consumers fetch with `go get` (`README.md:15`,
`go.mod`); how it is released or tagged is not evidenced (see Open questions). Local workflow is
driven by `Makefile` targets `test`, `lint` and `generate`. CI concerns, each with its rationale in an
ADR rather than restated here:
- Go toolchain pins and golangci-lint v2 — `.github/workflows/go.yml`; see [ADR-0002](decisions/ADR-0002-go-version-matrix-and-golangci-v2.md)
- Action SHA pinning and Dependabot updates — `.github/workflows/go.yml`, `.github/dependabot.yml`; see [ADR-0003](decisions/ADR-0003-pin-actions-and-automate-updates.md)
- Generated-code drift check (`make generate` must leave no diff) — `.github/workflows/go.yml`; see [ADR-0004](decisions/ADR-0004-generated-codecs-and-msgpack-stream.md)

Code ownership: `* @alpacahq/api-dx @gnvk @said-saifi` (`.github/CODEOWNERS`).

## Behaviour (Allium)
- none — no Allium specification exists in this repository and no accepted spec was supplied for this generation (`git ls-files` shows no `.allium` files or `specs/` directory).

## Decisions
- [ADR-0001 Surface the API request id on typed errors](decisions/ADR-0001-surface-request-id-on-api-errors.md)
- [ADR-0002 Pin the CI Go toolchains and lint with golangci-lint v2](decisions/ADR-0002-go-version-matrix-and-golangci-v2.md)
- [ADR-0003 Pin GitHub Actions to SHAs and automate dependency updates](decisions/ADR-0003-pin-actions-and-automate-updates.md)
- [ADR-0004 Generate JSON codecs and decode streams with msgpack](decisions/ADR-0004-generated-codecs-and-msgpack-stream.md)
- [ADR-0005 Route support questions by channel](decisions/ADR-0005-support-channel-routing.md)

## Evidence
- `README.md` — purpose, installation, authentication, endpoints, support routing
- `go.mod`, `go.sum` — module path, Go version, dependencies
- `Makefile` — test, lint, generate targets
- `.github/workflows/go.yml`, `.github/dependabot.yml`, `.github/CODEOWNERS` — CI, updates, ownership
- `alpaca/rest.go` (endpoints, options, retry), `alpaca/trading.go` (limit-price rounding), `alpaca/stream.go`, `alpaca/entities.go`, `alpaca/version.go` — trading client
- `marketdata/rest.go`, `marketdata/options.go` — market data REST client
- `marketdata/stream/client.go`, `marketdata/stream/conn_coder.go`, `marketdata/stream/options.go`, `marketdata/stream/subscription.go` — streaming client
- `examples/README.md` and `examples/*` — example strategies
- `.github/ISSUE_TEMPLATE/config.yml` — issue routing to the API repository, forum and Slack
- `git log` on `docs/layered-2026-09` @ `6ee44e6` (shallow clone, 11 commits) — recent change rationale

## Open questions
- > Unverified: minimum supported Go version. `go.mod` declares `go 1.26.10` (commit `ea2aedb`) and CI tests only 1.26.10 and 1.27.3 (commit `6ee44e6`). The only written support statement is stale: `.golangci.yml:82,85` disables `copyloopvar` and `intrange` with the comment "we support go 1.21", which contradicts the `go 1.26.10` floor in `go.mod` — confidence: low.
- > Unverified: the `//go:generate` directives pin easyjson `v0.7.7` (`alpaca/entities.go:17`, `marketdata/entities.go:13`, `marketdata/stream/entities.go:9`) while `go.mod` requires `v0.9.2`; the CI drift check would surface any output difference — confidence: low.
- > Unverified: release/versioning process. The module is `/v3` (`go.mod`) but no release workflow (`.github/workflows/` holds only `go.yml` with test and lint jobs), changelog or tagging policy file is present, and the shallow clone carries no tags (`git tag`) — confidence: low.
- > Unverified: `status: active`. Inferred from recent commits on the default branch (`git log`), not from a stated support policy — confidence: medium.
- > Unverified: deprecation state of individual endpoints or feeds; the repository defers API behaviour to the online API documents (`README.md`) — confidence: low.
- > Unverified: whether `owners` from `.github/CODEOWNERS` are the maintaining team in an organisational sense; CODEOWNERS only states review ownership.
- > Unverified: `ext_alpaca_trading_api`, `ext_alpaca_market_data_api` and `ext_go_module_proxy` in `../architecture/alpaca_trade_api_go.c4` are not yet listed among the organisation's shared external ids; the first two stand for the hosts named in `alpaca/rest.go:58`, `marketdata/rest.go:70` and `marketdata/stream/options.go`. The repository alone does not evidence which organisation systems serve those hosts, so the mapping is left to the org consistency pass, which may mark both sides `review_status: conflict`. The only Google dependency is `cloud.google.com/go/civil` (`alpaca/entities.go:11`, `marketdata/rest.go:15`), a date type, not a cloud service — confidence: low.
- > Unverified: the SDK packages are modelled as containers although they ship as one Go module (`go.mod`) — confidence: low on the level assignment.
