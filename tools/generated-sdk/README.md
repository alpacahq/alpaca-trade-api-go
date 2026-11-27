# Generated SDK generator platform

This directory contains pinned inputs and standalone tooling for reproducible Trading and Market Data client generation. It is the foundation for transitioning the internal Trading and Market Data REST implementations to OpenAPI-generated packages behind the existing v3 public facade. Landing the platform first lets maintainers review generator determinism, trust boundaries, and v3 compatibility rules separately from generated client source.

These assets are SDK-owned because stock OpenAPI Generator output against raw public specs cannot be used directly in Go without breaking v3 behavior: stock Go templates reject unknown future enum values, misdecode Alpaca union payloads, lose float64 precision, and log authentication headers in debug mode, while published documentation specs evolve without Go-level semver guarantees. Keeping the templates, compatibility overlays, and verification tooling in this repository makes generator changes reviewable beside the public contract they preserve.

The candidate check currently generates and compiles clients only in temporary directories. It does not install generated source or change public SDK routing.

## Contents

- `specs/`: canonical pinned OpenAPI documents, acquisition provenance, and the reviewed feature policy
- `generator/`: generator trust root, configurations, templates, overlays, template deltas, fixture locks, and the candidate wrapper
- `go.mod` and `tooling/`: a separate Go module for trust, specification, template, deterministic candidate, and reviewed specification-adoption checks

The trust root binds OpenAPI Generator 7.27.0, the generator JAR digest, digest-pinned Eclipse Temurin generator images and official Go behavior-test images for `linux/amd64` and `linux/arm64/v8`, Go toolchains, configurations, specifications, policy files, templates, fixture locks, overlays, and the candidate wrapper. Unsupported host architectures and requested platforms fail closed.

## Prerequisites and support

This maintainer-only platform is available from a repository checkout and is intentionally excluded from the published root Go module. It is not a separately published module.

Maintenance commands support Linux and macOS hosts on amd64 or arm64. They require Bash, Make, Go with toolchain selection enabled, Docker with a local Linux-container daemon, and sufficient cache space for both pinned Go toolchains, the generator JAR, and the selected generator and behavior-test images. Host Java is neither required nor executed.

Pinned OpenAPI inputs are canonical JSON. YAML specifications are intentionally unsupported.

Run `make generated-sdk-help` from the repository root to list the advanced specification proposal and adoption commands without adding them to the default Make help.

## Bootstrap

Bootstrap operations may access the network and populate local Go, JAR, and container caches:

```bash
make generated-sdk-dependency-bootstrap
make generated-sdk-generator-bootstrap
make generated-sdk-generator-image-bootstrap
```

Set `GENERATED_SDK_PLATFORM` explicitly when bootstrapping for a platform other than the supported native platform.

## Offline validation

For a quick check on any clean checkout without Docker or the downloaded generator JAR cache:

```bash
make generated-sdk-tooling-check
make generated-sdk-tooling-lint
```

`make generated-sdk-tooling-check` runs the standalone tooling tests, `go vet`, tracked trust-root and template-artifact verification (`generatortrust -mode check-tracked`), and pinned specification verification offline.

After bootstrap, these full checks run with module downloads disabled. Containerized generation also runs with networking disabled:

```bash
make generated-sdk-generator-trust-check
make generated-sdk-template-check
make generated-sdk-spec-check
make generated-sdk-generation-candidate-check
```

Candidate generation runs generator trust, pinned specification, and template policy preflights internally. It copies the verified generator inputs into an immutable temporary snapshot, mounts only that snapshot into the generator container, generates two complete temporary candidates, rejects symlinks and non-regular output entries, applies the reviewed post-processing independently to both, and requires byte-identical raw and final trees. Host validation compiles but never executes generated packages with both supported Go toolchains. Runtime policy tests execute only as an unprivileged user in the digest-pinned Go container with no network, a read-only filesystem, dropped capabilities, no-new-privileges, resource limits, a minimal environment, and only the temporary candidate mounted read-only. Temporary candidates are removed on success, failure, and graceful interruption.

## Public specification review

Run the networked drift check separately:

```bash
make generated-sdk-spec-drift-check
```

The check fetches both approved public URL forms, verifies their current byte equivalence, and reports raw or semantic differences from the pinned inputs without modifying repository files. Recorded raw digests and redirect equality in `specs/provenance.json` are acquisition attestations; canonical pinned digests, structure, versions, counts, and feature policy are validated offline.

Specification drift is a review signal, not an automatic update. Review upstream changes before replacing a pinned document or refreshing its provenance and feature policy.

## Reviewing and adopting newer specifications

Specification adoption follows an acquire, review, finalize, and adopt lifecycle. It is modeled after the explicit additive and breaking adoption workflow used by Alpaca's Java SDK and the temporary deterministic generation used by Alpaca's Python SDK. Go retains digest-pinned generator artifacts, network-disabled generation, complete two-run determinism, and sandboxed generated-code behavior tests.

The lifecycle remains candidate-only. It does not install generated source, change public SDK routing, stage or commit files, modify release policy, or approve a release.

### 1. Acquire and analyze

Choose an external directory and an explicit UTC retrieval date:

```bash
make generated-sdk-spec-update-dry-run \
  GENERATED_SDK_RETRIEVAL_DATE=2026-11-26 \
  GENERATED_SDK_SPEC_PROPOSAL_DIR=/tmp/alpaca-go-spec-proposal
```

The command fetches both approved URL forms for Trading and Market Data, requires byte equality, canonicalizes and validates both documents, classifies supported contract changes, and generates both the pinned and proposed candidates twice with identical trusted inputs. It compiles both candidates, runs sandboxed behavior tests, and writes a local generated-source comparison, canonical manifest, and public-neutral report to the external proposal directory.

Acquisition omits only pinned source-path and source-pattern compatibility assertions from the proposed review candidate so that an intentional contract removal can still produce a complete generated-source diff. Generator trust, input digests, overlay application, two-run determinism, compilation, and sandboxed behavior tests remain mandatory. Offline finalization restores every pinned compatibility assertion; a specification that requires generator-policy work cannot become adoptable until that work is reviewed separately.

If the existing overlays no longer apply, acquisition emits a non-adoptable review-required bundle before generation instead of discarding the fetched specifications or weakening overlay checks. Supply an explicitly reviewed overlay during finalization; full candidate generation and validation must then pass before the bundle can become adoptable.

Classification is deliberately bounded by the reviewed feature policy. Results are `no-change`, `provenance-only`, `documentation-only`, `additive`, `breaking`, or `manual-review`. Unknown OpenAPI shapes and generator-sensitive changes fail closed as `manual-review`; an `additive` result describes specification shape only and is not a claim of Go source compatibility or release approval. Review the generated declaration summary and `generated-source.diff` in addition to the OpenAPI classification.

Additive adoption also refuses removed generated files and changed or removed public Go declarations. Those outcomes require the breaking target even when the specification-shape classifier is additive.

The acquired bundle is never directly adoptable. Copy its proposed `feature-allowlist.json` into a separate review directory only after reviewing the complete policy. If the existing overlay is stale, place an explicitly reviewed `overlays.json` in the same directory. The tooling never synthesizes or silently approves policy or overlay changes.

### 2. Finalize reviewed inputs

Use the acquisition plan digest printed by the dry run:

```bash
make generated-sdk-spec-update-finalize \
  GENERATED_SDK_SPEC_PROPOSAL_DIR=/tmp/alpaca-go-spec-proposal \
  GENERATED_SDK_SPEC_REVIEWED_DIR=/tmp/alpaca-go-spec-reviewed \
  GENERATED_SDK_SPEC_FINAL_DIR=/tmp/alpaca-go-spec-final \
  GENERATED_SDK_SPEC_PLAN_DIGEST=<acquisition-plan-digest>
```

Finalization is offline. It binds the reviewed feature policy and optional overlay to the exact fetched bytes, reruns pinned-versus-proposed deterministic generation, compilation, and behavior tests, and emits a new immutable adoptable bundle and digest. Proposal manifests and reports contain repository-relative public paths only. Generated trees and diffs remain local external review material and must not be copied into the repository.

### 3. Adopt explicitly

For a proposal containing only `no-change`, `provenance-only`, `documentation-only`, or `additive` classifications:

```bash
make generated-sdk-spec-update-adopt-additive \
  GENERATED_SDK_SPEC_FINAL_DIR=/tmp/alpaca-go-spec-final \
  GENERATED_SDK_SPEC_PLAN_DIGEST=<final-plan-digest>
```

Breaking or manually reviewed changes require the distinct target and the exact finalized digest:

```bash
make generated-sdk-spec-update-adopt-breaking \
  GENERATED_SDK_SPEC_FINAL_DIR=/tmp/alpaca-go-spec-final \
  GENERATED_SDK_SPEC_PLAN_DIGEST=<final-plan-digest>
```

Adoption never refetches upstream content. It may replace only the two pinned specifications, provenance, feature policy, trust root, and explicitly reviewed overlay. Baseline and bundle digests must match exactly. A versioned journal, same-filesystem staged files, synchronized backups, and recorded rename phases make the update recoverable rather than claiming unsupported cross-file atomicity. Post-write trust, specification, template, and candidate checks run offline, and the resulting candidate attestation must match the finalized proposal exactly. Any ordinary failure restores the original bytes.

If a process is interrupted, new adoption attempts refuse to proceed until recovery:

```bash
make generated-sdk-spec-update-rollback
```

The adoption command does not run Git commands. Inspect and stage resulting input changes separately.

## Other reviewed input updates

Two read-only proposal commands print canonical review inputs to standard output:

```bash
make generated-sdk-spec-allowlist-proposal > /tmp/generated-sdk-feature-allowlist.json
make generated-sdk-generator-digest-proposal > /tmp/generated-sdk-digests.json
```

Review these proposals before copying any value into a tracked file. For template changes, extract each pinned stock template to a temporary location and inspect its complete diff against the custom template:

```bash
unzip -p tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar \
  go/model_oneof.mustache > /tmp/model_oneof.stock.mustache
git diff --no-index -- /tmp/model_oneof.stock.mustache \
  tools/generated-sdk/generator/templates/go/model_oneof.mustache
```

Updates must be separate reviewed changes:

1. Use the specification adoption workflow above for public specification changes.
2. For generator upgrades, review the JAR and container identities, stock templates, custom template deltas, fixture dependency graph, configurations, and post-processing matches together.
3. Keep generator, container, toolchain, configuration, template, fixture, wrapper, path, and schema policy anchored in code. Only specification, provenance, feature-policy, and explicitly reviewed overlay digests rotate through the adoption workflow.
4. Bootstrap the reviewed dependencies, run every offline check, run the candidate check twice, and confirm that the repository worktree is unchanged.

The custom templates deliberately preserve unknown enum values and exact union JSON while applying required-field and discriminator policies. `generator/template-deltas.json` records each reviewed deviation from the pinned stock templates. Exact source post-processing fails closed when an upstream generator change no longer matches. Containerized generated-code tests exercise credential and body redaction, float64 formatting, unknown non-FILL activity fallback, and the template response-decoding policies.

`generator/fixtures/template-fixture.mod` and `.sum` are inactive locks for the exact module files emitted by the pinned generator during focused template tests. They are not SDK runtime dependencies, are not discovered by root-module dependency automation, and must be reviewed manually with generator upgrades. Their nonstandard filenames prevent the fixture from becoming an active nested module in the checkout.

Until dedicated generated-SDK CI is introduced, changes under this directory require `make generated-sdk-tooling-check`, `make generated-sdk-tooling-lint`, and the full bootstrapped offline validation sequence above. Ordinary `make lint`, root tests, root builds, and the installed SDK release lifecycle remain independent of this standalone module.
