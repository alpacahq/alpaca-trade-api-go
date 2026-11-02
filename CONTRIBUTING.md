# Contributing

Thanks for helping improve the Alpaca Go SDK. By participating, you agree to follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

## Choose the right channel

- **Library / SDK issues:** Bugs, feature requests, or questions specific to this SDK → [GitHub Issues](https://github.com/alpacahq/alpaca-trade-api-go/issues/new/choose).
- **Security vulnerabilities:** Do not report them publicly. Follow the private reporting process in our [Security Policy](./SECURITY.md).
- **Alpaca API bugs:** Bugs in the Alpaca API itself, its OpenAPI specs, or its documentation → [alpacahq/alpaca-api](https://github.com/alpacahq/alpaca-api/issues/new/choose).
- **Community:** API feature requests, general questions, and dashboard bugs → [Alpaca Community Forum](https://forum.alpaca.markets/) or [Slack](https://alpaca.markets/slack).
- **Account help:** Questions about your account, funding, or orders → [Support request](https://alpaca.markets/support/request).
- **Outages:** Service status and incident updates → [Status page](https://status.alpaca.markets/).

Please do not include credentials, account details, or other sensitive information in an issue, discussion, pull request, test, or example.

## Contribution policy

### Contributions we welcome

Focused contributions are welcome when they improve this SDK, including:

- bug fixes and compatibility improvements;
- support for documented Alpaca API capabilities;
- focused tests and reliability improvements;
- documentation and examples; and
- maintainability improvements that preserve public behavior.

### Discuss substantial changes first

Open an issue before investing in a substantial SDK design, new dependency, broad refactor, or third-party integration. Early discussion helps confirm that the proposal fits the SDK's scope and avoids duplicated or unusable work.

Bugs in the Alpaca API itself, its OpenAPI specs, or its documentation must be fixed upstream, not patched in this SDK. Report them in [alpacahq/alpaca-api](https://github.com/alpacahq/alpaca-api/issues/new/choose). API feature requests, general questions, and dashboard bugs belong on the [Alpaca Community Forum](https://forum.alpaca.markets/) or [Slack](https://alpaca.markets/slack). Contributions that add support for already documented API capabilities are welcome after the SDK design is agreed with maintainers.

### Out of scope

This repository does not accept:

- trading strategies, algorithm design, or investment advice;
- bugs in the Alpaca API, its OpenAPI specs, or docs.alpaca.markets;
- API feature requests, dashboard bugs, or outage reports;
- questions that aren't about this SDK, including questions about your account, funding, or orders;
- credentials, private account data, or other secrets;
- referral links or promotional content; or
- unapproved integrations with third-party products or services.

Contributions that add or promote third-party products or services, including provider-specific clients, API calls, hosted endpoints, referral links, or promotional content, require prior written approval from Alpaca before implementation. Open an issue describing the technical scope; repository maintainers will coordinate the necessary internal review and confirm whether the proposal may proceed. Contributors do not need to identify or contact internal approvers.

Using an ordinary open-source dependency or improving generic interoperability does not by itself make a contribution a provider-specific integration. Maintainers will explain when a proposal falls outside this policy. A pull request that proceeds without the required approval may be closed as out of scope, so open an issue first if you are unsure whether a proposal qualifies.

## License and contribution terms

The project is licensed under the [Apache License 2.0](./LICENSE). Subject to its full terms, the license permits use, reproduction, modification, and distribution of the software. It does not grant permission to use Alpaca trade names, trademarks, service marks, or product names except as described in the license.

Unless you explicitly state otherwise, a contribution intentionally submitted for inclusion in this project is provided under Apache-2.0 without additional terms, as described in Section 5 of the license. This project does not currently require a separate Contributor License Agreement (CLA) or Developer Certificate of Origin (DCO) sign-off.

## Contribution workflow

1. Search existing issues and pull requests.
2. Open an issue first for work described under [Discuss substantial changes first](#discuss-substantial-changes-first).
3. Fork the repository and create a focused branch.
4. Make the change in hand-written source files.
5. Add focused tests for new or changed behavior and update relevant documentation.
6. Regenerate easyjson output when entity definitions change.
7. Run the relevant checks listed below.
8. Open a focused pull request, explain why the change is needed, and respond to review feedback.

Use clear, descriptive commit messages and pull request titles. Conventional Commits are not required, and contributors do not need to squash commits before opening a pull request. Maintainers use GitHub's squash-merge workflow for the final merge.

## Prerequisites

- Go 1.24 or newer, as declared in [`go.mod`](./go.mod).
- `make` for the repository's documented convenience targets.

## Setup and checks

```bash
go build -v ./...
make test
make lint
make generate
```

The repository does not enforce a separate formatter command. Format changed Go files with `gofmt`, match the surrounding style, and run the linter. There is no numeric coverage threshold, but changed behavior must have focused tests.

After running `make generate`, inspect the diff and commit expected generated changes. CI verifies that committed generated files are current.

## Testing and financial safety

- Repository tests must never place live orders or depend on a live trading account.
- Tests must be deterministic and run without Alpaca credentials. Use mocks, fakes, `httptest`, or synthetic fixtures for API behavior.
- Runnable trading examples must use paper trading by default and require an explicit user choice before targeting live trading.
- Real credentials, account identifiers, account data, and production payloads must never appear in tests, fixtures, examples, or committed logs. Remove request and trace IDs from public or committed material; share them only through private Alpaca Support channels when needed.

## Generated and hand-written code

The SDK clients, requests, responses, streaming behavior, and public API are hand-written Go. This repository currently uses [easyjson](https://github.com/mailru/easyjson) only to generate JSON serialization code for selected entities.

Do not edit these generated files directly:

- `alpaca/entities_easyjson.go`
- `marketdata/entities_easyjson.go`
- `marketdata/stream/entities_easyjson.go`

Edit the corresponding `entities.go` source and regenerate the output:

```bash
make generate
```

Commit the regenerated files when they change. Generated output should be reproducible from the hand-written source.

## Documentation

Keep the [README](./README.md), package comments exposed through [pkg.go.dev](https://pkg.go.dev/github.com/alpacahq/alpaca-trade-api-go/v3), and runnable [examples](./examples) consistent with behavior changes. Public Go identifiers should have useful Go documentation comments.

## Pull request checklist

Before requesting review, confirm that:

- the change is focused, explained, and linked to prior discussion when needed;
- generated easyjson files were not edited by hand and are current;
- new or changed behavior has focused tests;
- tests are deterministic and credential-free and cannot place live orders;
- trading examples default to paper trading and fixtures use synthetic data;
- relevant documentation and examples are updated;
- applicable build, lint, test, and generation checks pass;
- no credentials, account data, request or trace IDs, or other secrets are included; and
- any provider-specific integration or promotional contribution has written Alpaca approval confirmed by repository maintainers.
