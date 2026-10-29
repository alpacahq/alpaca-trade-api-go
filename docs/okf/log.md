---
type: log
title: Alpaca Go SDK (alpaca-trade-api-go) — OKF change log
generated_by: claude-fable-5 / layered-docs 2026-09
source_commit: 6ee44e6acbfe27101f84ff4949ff217bc3e84ba0
source_branch: docs/layered-2026-09
generated_at: 2026-10-29T08:55:36+00:00
confidence: medium
review_status: draft-needs-review
---
# Change log

- 2026-10-09 — initial generation of OKF context + LikeC4 architecture from `docs/layered-2026-09` @ `d0a9719d1e133ea72ff9bc829904b4709028b4d5` (claude-opus-5, layered-docs 2026-09). Draft, review required.
- 2026-10-29 — regenerated against `docs/layered-2026-09` @ `6ee44e6acbfe27101f84ff4949ff217bc3e84ba0` (claude-fable-5, layered-docs 2026-09): Go 1.26.10 in `go.mod`, CI matrix pinned to 1.26.10/1.27.3, support routing and issue template updates (ADR-0002, ADR-0005 revised). Draft, review required.
- 2026-10-29 — refined after verification: ADR-0002 context corrected to the commit history (ea2aedb cut the matrix for CVE-2026-111943, 6ee44e6 pinned patches); ADR-0005 date set to the first decision; module-distribution edge reworded and marked low confidence; HTTP edges use `calls`; examples -> trade updates stream added; stream env-override and retry wording corrected; OKF now points to the LikeC4 model for hosts. Draft, review required.
