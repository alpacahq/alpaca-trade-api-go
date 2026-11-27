package specupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/generatortrust"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/overlay"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/specsource"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/treecompare"
)

const (
	ManifestPath         = "spec-update-manifest.json"
	ReportPath           = "spec-update-report.md"
	GeneratedDiffPath    = "generated-source.diff"
	StatusReviewRequired = "review-required"
	StatusAdoptable      = "adoptable"
)

var trackedPaths = []string{
	"tools/generated-sdk/specs/trading.openapi.json",
	"tools/generated-sdk/specs/market-data.openapi.json",
	"tools/generated-sdk/specs/provenance.json",
	"tools/generated-sdk/specs/feature-allowlist.json",
	"tools/generated-sdk/generator/overlays.json",
	generatortrust.TrustRootPath,
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type CandidateResult struct {
	FileCount   int      `json:"file_count"`
	FinalSHA256 string   `json:"final_sha256"`
	Platform    string   `json:"platform"`
	RawSHA256   string   `json:"raw_sha256"`
	Toolchains  []string `json:"toolchains"`
}

type CandidateRunner interface {
	Run(context.Context, string, string) (CandidateResult, error)
}

type Manifest struct {
	Artifacts         map[string]string   `json:"artifacts"`
	BaselineArtifacts map[string]string   `json:"baseline_artifacts"`
	Classifications   []spec.Comparison   `json:"classifications"`
	Generated         treecompare.Summary `json:"generated"`
	Issues            []string            `json:"issues"`
	PinnedCandidate   CandidateResult     `json:"pinned_candidate"`
	PlanDigest        string              `json:"plan_digest"`
	ProposedCandidate CandidateResult     `json:"proposed_candidate"`
	ReviewArtifacts   map[string]string   `json:"review_artifacts"`
	RetrievalDate     string              `json:"retrieval_date"`
	SchemaVersion     int                 `json:"schema_version"`
	Status            string              `json:"status"`
}

type AcquireOptions struct {
	Client        *http.Client
	OutputDir     string
	RepoRoot      string
	RetrievalDate time.Time
	Runner        CandidateRunner
}

type FinalizeOptions struct {
	AcquisitionDir string
	AcceptPlan     string
	OutputDir      string
	RepoRoot       string
	ReviewedDir    string
	Runner         CandidateRunner
}

type provenance struct {
	RetrievalDate      string               `json:"retrieval_date"`
	SchemaVersion      int                  `json:"schema_version"`
	SelectedSpecs      map[string]specEntry `json:"selected_specs"`
	SelectionRationale string               `json:"selection_rationale"`
}

type specEntry struct {
	CanonicalSHA256              string `json:"canonical_sha256"`
	ComponentSchemaCount         int    `json:"component_schema_count"`
	InfoVersion                  string `json:"info_version"`
	OpenAPI                      string `json:"openapi"`
	OperationCount               int    `json:"operation_count"`
	Path                         string `json:"path"`
	RedirectRawSHA256            string `json:"redirect_raw_sha256"`
	ReachableSchemaIdentityCount int    `json:"reachable_schema_identity_count"`
	RedirectEquivalentURL        string `json:"redirect_equivalent_url"`
	RetrievalAttestation         string `json:"retrieval_attestation"`
	RetrievalDate                string `json:"retrieval_date"`
	SourceRawSHA256              string `json:"source_raw_sha256"`
	SourceURL                    string `json:"source_url"`
}

type targetDefinition struct {
	Path        string
	PrimaryURL  string
	RedirectURL string
}

var targetDefinitions = map[string]targetDefinition{
	"trading": {
		Path:        "tools/generated-sdk/specs/trading.openapi.json",
		PrimaryURL:  "https://docs.alpaca.markets/us/openapi/trading-api.json",
		RedirectURL: "https://docs.alpaca.markets/openapi/trading-api.json",
	},
	"market-data": {
		Path:        "tools/generated-sdk/specs/market-data.openapi.json",
		PrimaryURL:  "https://docs.alpaca.markets/us/openapi/market-data-api.json",
		RedirectURL: "https://docs.alpaca.markets/openapi/market-data-api.json",
	},
}

func Acquire(ctx context.Context, options AcquireOptions) (result Manifest, resultErr error) {
	if options.Client == nil || options.Runner == nil {
		return Manifest{}, errors.New("spec update acquisition requires an HTTP client and candidate runner")
	}
	if err := prepareExternalOutput(options.RepoRoot, options.OutputDir); err != nil {
		return Manifest{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, os.RemoveAll(options.OutputDir))
		}
	}()
	baselineArtifacts, err := readArtifacts(options.RepoRoot, trackedPaths)
	if err != nil {
		return Manifest{}, err
	}
	currentProvenance, err := decodeProvenance(baselineArtifacts["tools/generated-sdk/specs/provenance.json"])
	if err != nil {
		return Manifest{}, err
	}
	trust, err := generatortrust.Decode(baselineArtifacts[generatortrust.TrustRootPath])
	if err != nil {
		return Manifest{}, err
	}
	retrievalDate := options.RetrievalDate.UTC().Format("2006-01-02")
	if retrievalDate == "0001-01-01" {
		return Manifest{}, errors.New("retrieval date is required")
	}
	proposedArtifacts := map[string][]byte{}
	var comparisons []spec.Comparison
	var inventories []*spec.Inventory
	targets := []string{"market-data", "trading"}
	for _, target := range targets {
		definition := targetDefinitions[target]
		pair, err := specsource.FetchPair(ctx, options.Client, definition.PrimaryURL, definition.RedirectURL)
		if err != nil {
			return Manifest{}, fmt.Errorf("%s: %w", target, err)
		}
		document, err := spec.Parse(pair.Primary, ".json")
		if err != nil {
			return Manifest{}, fmt.Errorf("%s: %w", target, err)
		}
		canonical, err := spec.Canonical(document.Root)
		if err != nil {
			return Manifest{}, err
		}
		inventory, err := spec.BuildInventory(document, target)
		if err != nil {
			return Manifest{}, fmt.Errorf("%s inventory: %w", target, err)
		}
		comparison, err := spec.Compare(target, baselineArtifacts[definition.Path], canonical)
		if err != nil {
			return Manifest{}, err
		}
		current := currentProvenance.SelectedSpecs[target]
		if comparison.Classification == spec.ClassificationNoChange &&
			(current.SourceRawSHA256 != spec.Digest(pair.Primary) ||
				current.RedirectRawSHA256 != spec.Digest(pair.Redirect)) {
			comparison.Classification = spec.ClassificationProvenanceOnly
			comparison.Changes = append(comparison.Changes, spec.Change{
				Classification: spec.ClassificationProvenanceOnly,
				Kind:           "raw-source-provenance",
				Subject:        target,
				Detail:         "raw public source bytes changed without changing canonical JSON",
			})
		}
		comparisons = append(comparisons, comparison)
		inventories = append(inventories, inventory)
		proposedArtifacts[definition.Path] = canonical
		current.CanonicalSHA256 = spec.Digest(canonical)
		current.ComponentSchemaCount = componentSchemaCount(document.Root)
		current.InfoVersion = inventory.InfoVersion
		current.OpenAPI = inventory.OpenAPI
		current.OperationCount = inventory.OperationCount
		current.Path = definition.Path
		current.RedirectEquivalentURL = definition.RedirectURL
		current.RedirectRawSHA256 = spec.Digest(pair.Redirect)
		current.ReachableSchemaIdentityCount = inventory.ReachableSchemaCount
		current.RetrievalAttestation = "recorded-source-and-redirect-bytes"
		current.RetrievalDate = retrievalDate
		current.SourceRawSHA256 = spec.Digest(pair.Primary)
		current.SourceURL = definition.PrimaryURL
		currentProvenance.SelectedSpecs[target] = current
	}
	currentProvenance.RetrievalDate = retrievalDate
	provenanceData, err := spec.Canonical(currentProvenance)
	if err != nil {
		return Manifest{}, err
	}
	proposedArtifacts["tools/generated-sdk/specs/provenance.json"] = provenanceData
	counts := map[string]int{}
	for _, inventory := range inventories {
		counts[inventory.Source] = inventory.OperationCount
	}
	allowlistData, err := spec.Canonical(spec.ReviewedAllowlist(counts, inventories...))
	if err != nil {
		return Manifest{}, err
	}
	proposedArtifacts["tools/generated-sdk/specs/feature-allowlist.json"] = allowlistData
	proposedArtifacts["tools/generated-sdk/generator/overlays.json"] = baselineArtifacts["tools/generated-sdk/generator/overlays.json"]
	updateTrustLocks(trust, proposedArtifacts)
	trustData, err := spec.Canonical(trust)
	if err != nil {
		return Manifest{}, err
	}
	proposedArtifacts[generatortrust.TrustRootPath] = trustData
	if err := writeArtifacts(options.OutputDir, proposedArtifacts); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Artifacts:         artifactDigests(proposedArtifacts),
		BaselineArtifacts: artifactDigests(baselineArtifacts),
		Classifications:   comparisons,
		Issues:            []string{},
		ReviewArtifacts:   map[string]string{},
		RetrievalDate:     retrievalDate,
		SchemaVersion:     1,
		Status:            StatusReviewRequired,
	}
	if err := validateBundleInputs(options.OutputDir, false); err != nil {
		return Manifest{}, err
	}
	if err := validateBundleOverlays(options.OutputDir); err != nil {
		manifest.Issues = []string{"reviewed overlays do not apply to the proposed specifications"}
		if err := sealManifest(options.OutputDir, &manifest); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}
	if err := os.Mkdir(filepath.Join(options.OutputDir, "generated"), 0o755); err != nil {
		return Manifest{}, err
	}
	pinnedOutput := filepath.Join(options.OutputDir, "generated", "pinned")
	proposedOutput := filepath.Join(options.OutputDir, "generated", "proposed")
	pinnedCandidate, err := options.Runner.Run(ctx, "", pinnedOutput)
	if err != nil {
		return Manifest{}, fmt.Errorf("generate pinned candidate: %w", err)
	}
	proposedCandidate, err := options.Runner.Run(ctx, options.OutputDir, proposedOutput)
	if err != nil {
		return Manifest{}, fmt.Errorf("generate proposed candidate: %w", err)
	}
	generated, err := treecompare.Compare(pinnedOutput, proposedOutput)
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(options.OutputDir, GeneratedDiffPath), generated.Diff, 0o444); err != nil {
		return Manifest{}, err
	}
	manifest.Generated = generated.Summary
	manifest.PinnedCandidate = pinnedCandidate
	manifest.ProposedCandidate = proposedCandidate
	manifest.ReviewArtifacts = map[string]string{GeneratedDiffPath: spec.Digest(generated.Diff)}
	if err := sealManifest(options.OutputDir, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Finalize(ctx context.Context, options FinalizeOptions) (result Manifest, resultErr error) {
	acquisition, err := LoadManifest(options.AcquisitionDir)
	if err != nil {
		return Manifest{}, err
	}
	if acquisition.Status != StatusReviewRequired || acquisition.PlanDigest != options.AcceptPlan {
		return Manifest{}, errors.New("acquisition plan digest was not explicitly accepted")
	}
	if err := verifyArtifacts(options.AcquisitionDir, acquisition.Artifacts); err != nil {
		return Manifest{}, err
	}
	if options.Runner == nil {
		return Manifest{}, errors.New("spec update finalization requires a candidate runner")
	}
	if err := verifyArtifacts(options.RepoRoot, acquisition.BaselineArtifacts); err != nil {
		return Manifest{}, fmt.Errorf("acquisition baseline no longer matches the repository: %w", err)
	}
	if err := prepareExternalOutput(options.RepoRoot, options.OutputDir); err != nil {
		return Manifest{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, os.RemoveAll(options.OutputDir))
		}
	}()
	proposedArtifacts, err := readArtifacts(options.AcquisitionDir, trackedPaths)
	if err != nil {
		return Manifest{}, err
	}
	reviewedPolicy := filepath.Join(options.ReviewedDir, "feature-allowlist.json")
	proposedArtifacts["tools/generated-sdk/specs/feature-allowlist.json"], err = safetree.ReadRegular(reviewedPolicy)
	if err != nil {
		return Manifest{}, fmt.Errorf("reviewed feature policy: %w", err)
	}
	reviewedOverlay := filepath.Join(options.ReviewedDir, "overlays.json")
	if _, statErr := os.Lstat(reviewedOverlay); statErr == nil {
		proposedArtifacts["tools/generated-sdk/generator/overlays.json"], err = safetree.ReadRegular(reviewedOverlay)
		if err != nil {
			return Manifest{}, fmt.Errorf("reviewed overlays: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return Manifest{}, statErr
	}
	trust, err := generatortrust.Decode(proposedArtifacts[generatortrust.TrustRootPath])
	if err != nil {
		return Manifest{}, err
	}
	updateTrustLocks(trust, proposedArtifacts)
	proposedArtifacts[generatortrust.TrustRootPath], err = spec.Canonical(trust)
	if err != nil {
		return Manifest{}, err
	}
	if err := writeArtifacts(options.OutputDir, proposedArtifacts); err != nil {
		return Manifest{}, err
	}
	if err := validateBundleInputs(options.OutputDir, true); err != nil {
		return Manifest{}, err
	}
	if err := os.Mkdir(filepath.Join(options.OutputDir, "generated"), 0o755); err != nil {
		return Manifest{}, err
	}
	pinnedOutput := filepath.Join(options.OutputDir, "generated", "pinned")
	proposedOutput := filepath.Join(options.OutputDir, "generated", "proposed")
	pinnedCandidate, err := options.Runner.Run(ctx, "", pinnedOutput)
	if err != nil {
		return Manifest{}, err
	}
	if len(acquisition.Issues) == 0 && !equalCandidateResult(pinnedCandidate, acquisition.PinnedCandidate) {
		return Manifest{}, errors.New("pinned candidate differs from the acquisition baseline")
	}
	proposedCandidate, err := options.Runner.Run(ctx, options.OutputDir, proposedOutput)
	if err != nil {
		return Manifest{}, err
	}
	generated, err := treecompare.Compare(pinnedOutput, proposedOutput)
	if err != nil {
		return Manifest{}, err
	}
	if len(acquisition.Issues) == 0 &&
		generated.Summary.BaselineTreeSHA256 != acquisition.Generated.BaselineTreeSHA256 {
		return Manifest{}, errors.New("pinned generated tree differs from the acquisition baseline")
	}
	if err := verifyArtifacts(options.RepoRoot, acquisition.BaselineArtifacts); err != nil {
		return Manifest{}, fmt.Errorf("acquisition baseline changed during finalization: %w", err)
	}
	if err := os.WriteFile(filepath.Join(options.OutputDir, GeneratedDiffPath), generated.Diff, 0o444); err != nil {
		return Manifest{}, err
	}
	manifest := acquisition
	manifest.Artifacts = artifactDigests(proposedArtifacts)
	manifest.Generated = generated.Summary
	manifest.Issues = []string{}
	manifest.PinnedCandidate = pinnedCandidate
	manifest.PlanDigest = ""
	manifest.ProposedCandidate = proposedCandidate
	manifest.ReviewArtifacts = map[string]string{GeneratedDiffPath: spec.Digest(generated.Diff)}
	manifest.Status = StatusAdoptable
	if err := sealManifest(options.OutputDir, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateBundleInputs(bundleRoot string, requireOverlays bool) error {
	artifacts, err := readArtifacts(bundleRoot, trackedPaths)
	if err != nil {
		return err
	}
	trust, err := generatortrust.Decode(artifacts[generatortrust.TrustRootPath])
	if err != nil {
		return err
	}
	var inventories []*spec.Inventory
	inventoryByTarget := map[string]*spec.Inventory{}
	documentByTarget := map[string]*spec.Document{}
	for target, definition := range targetDefinitions {
		specificationData := artifacts[definition.Path]
		document, err := spec.Parse(specificationData, ".json")
		if err != nil {
			return err
		}
		canonicalSpecification, err := spec.Canonical(document.Root)
		if err != nil {
			return err
		}
		if !bytes.Equal(specificationData, canonicalSpecification) {
			return fmt.Errorf("%s proposed specification is not canonical JSON", target)
		}
		inventory, err := spec.BuildInventory(document, target)
		if err != nil {
			return err
		}
		inventories = append(inventories, inventory)
		inventoryByTarget[target] = inventory
		documentByTarget[target] = document
	}
	allowlist, err := spec.DecodeAllowlist(artifacts["tools/generated-sdk/specs/feature-allowlist.json"])
	if err != nil {
		return err
	}
	if err := spec.CheckAllowlist(allowlist, inventories...); err != nil {
		return err
	}
	canonicalAllowlist, err := spec.Canonical(allowlist)
	if err != nil {
		return err
	}
	if !bytes.Equal(artifacts["tools/generated-sdk/specs/feature-allowlist.json"], canonicalAllowlist) {
		return errors.New("proposed feature policy is not canonical JSON")
	}
	provenanceData := artifacts["tools/generated-sdk/specs/provenance.json"]
	provenance, err := decodeProvenance(provenanceData)
	if err != nil {
		return err
	}
	canonicalProvenance, err := spec.Canonical(provenance)
	if err != nil {
		return err
	}
	if !bytes.Equal(provenanceData, canonicalProvenance) {
		return errors.New("proposed specification provenance is not canonical JSON")
	}
	for target, definition := range targetDefinitions {
		entry, ok := provenance.SelectedSpecs[target]
		inventory := inventoryByTarget[target]
		document := documentByTarget[target]
		if !ok ||
			entry.Path != definition.Path ||
			entry.SourceURL != definition.PrimaryURL ||
			entry.RedirectEquivalentURL != definition.RedirectURL ||
			entry.CanonicalSHA256 != spec.Digest(artifacts[definition.Path]) ||
			entry.OpenAPI != inventory.OpenAPI ||
			entry.InfoVersion != inventory.InfoVersion ||
			entry.OperationCount != inventory.OperationCount ||
			entry.ReachableSchemaIdentityCount != inventory.ReachableSchemaCount ||
			entry.ComponentSchemaCount != componentSchemaCount(document.Root) ||
			entry.RetrievalDate != provenance.RetrievalDate ||
			entry.RetrievalAttestation != "recorded-source-and-redirect-bytes" ||
			!sha256Pattern.MatchString(entry.SourceRawSHA256) ||
			!sha256Pattern.MatchString(entry.RedirectRawSHA256) {
			return fmt.Errorf("%s provenance differs from the proposed specification", target)
		}
	}
	if requireOverlays {
		if err := applyBundleOverlays(artifacts); err != nil {
			return err
		}
	}
	for _, artifact := range []generatortrust.Artifact{
		trust.Specifications["trading"],
		trust.Specifications["market-data"],
		trust.PolicyArtifacts["feature_allowlist"],
		trust.PolicyArtifacts["provenance"],
		trust.TemplateArtifacts["overlays"],
	} {
		if spec.Digest(artifacts[artifact.Path]) != artifact.SHA256 {
			return fmt.Errorf("%s digest differs from proposed trust root", artifact.Path)
		}
	}
	return nil
}

func validateBundleOverlays(bundleRoot string) error {
	artifacts, err := readArtifacts(bundleRoot, trackedPaths)
	if err != nil {
		return err
	}
	return applyBundleOverlays(artifacts)
}

func applyBundleOverlays(artifacts map[string][]byte) error {
	rules, err := overlay.Decode(artifacts["tools/generated-sdk/generator/overlays.json"])
	if err != nil {
		return err
	}
	for target, definition := range targetDefinitions {
		document, err := spec.Parse(artifacts[definition.Path], ".json")
		if err != nil {
			return err
		}
		for _, operation := range rules.Operations {
			if operation.Target == target {
				if err := overlay.Apply(document.Root, operation); err != nil {
					return fmt.Errorf("%s overlay %s: %w", target, operation.Pointer, err)
				}
			}
		}
	}
	return nil
}

func updateTrustLocks(trust *generatortrust.TrustRoot, artifacts map[string][]byte) {
	for target, definition := range targetDefinitions {
		artifact := trust.Specifications[target]
		artifact.SHA256 = spec.Digest(artifacts[definition.Path])
		trust.Specifications[target] = artifact
	}
	for name, path := range map[string]string{
		"feature_allowlist": "tools/generated-sdk/specs/feature-allowlist.json",
		"provenance":        "tools/generated-sdk/specs/provenance.json",
	} {
		artifact := trust.PolicyArtifacts[name]
		artifact.SHA256 = spec.Digest(artifacts[path])
		trust.PolicyArtifacts[name] = artifact
	}
	artifact := trust.TemplateArtifacts["overlays"]
	artifact.SHA256 = spec.Digest(artifacts["tools/generated-sdk/generator/overlays.json"])
	trust.TemplateArtifacts["overlays"] = artifact
}

func LoadManifest(root string) (Manifest, error) {
	data, err := safetree.ReadRegular(filepath.Join(root, ManifestPath))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := spec.DecodeStrict(data, &manifest); err != nil {
		return Manifest{}, err
	}
	canonical, err := spec.Canonical(manifest)
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Manifest{}, errors.New("spec update manifest is not canonical JSON")
	}
	if err := validateManifestStructure(manifest); err != nil {
		return Manifest{}, err
	}
	digest := manifest.PlanDigest
	manifest.PlanDigest = ""
	unsigned, err := spec.Canonical(manifest)
	if err != nil {
		return Manifest{}, err
	}
	if spec.Digest(unsigned) != digest {
		return Manifest{}, errors.New("spec update manifest digest is invalid")
	}
	manifest.PlanDigest = digest
	if err := verifyReviewArtifacts(root, manifest.ReviewArtifacts); err != nil {
		return Manifest{}, err
	}
	reportData, err := safetree.ReadRegular(filepath.Join(root, ReportPath))
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(reportData, report(manifest)) {
		return Manifest{}, errors.New("specification update report differs from its manifest")
	}
	return manifest, nil
}

func validateManifestStructure(manifest Manifest) error {
	if manifest.SchemaVersion != 1 ||
		manifest.Status != StatusReviewRequired && manifest.Status != StatusAdoptable ||
		!sha256Pattern.MatchString(manifest.PlanDigest) {
		return errors.New("invalid specification update manifest identity")
	}
	if _, err := time.Parse("2006-01-02", manifest.RetrievalDate); err != nil {
		return errors.New("invalid specification update retrieval date")
	}
	if !slices.Equal(sortedKeys(manifest.Artifacts), sortedTrackedPaths()) ||
		!slices.Equal(sortedKeys(manifest.BaselineArtifacts), sortedTrackedPaths()) {
		return errors.New("specification update artifact set differs from policy")
	}
	for _, digests := range []map[string]string{manifest.Artifacts, manifest.BaselineArtifacts} {
		for _, digest := range digests {
			if !sha256Pattern.MatchString(digest) {
				return errors.New("specification update artifact has an invalid digest")
			}
		}
	}
	if manifest.Status == StatusAdoptable && len(manifest.Issues) != 0 {
		return errors.New("adoptable specification update manifest contains unresolved issues")
	}
	if len(manifest.Issues) != 0 {
		if manifest.Status != StatusReviewRequired ||
			!slices.Equal(manifest.Issues, []string{"reviewed overlays do not apply to the proposed specifications"}) ||
			len(manifest.ReviewArtifacts) != 0 {
			return errors.New("specification update manifest has invalid review issues")
		}
	} else if len(manifest.ReviewArtifacts) != 1 ||
		!sha256Pattern.MatchString(manifest.ReviewArtifacts[GeneratedDiffPath]) {
		return errors.New("generated source review artifact is missing or invalid")
	}
	targets := map[string]bool{}
	for _, comparison := range manifest.Classifications {
		if comparison.Target != "trading" && comparison.Target != "market-data" || targets[comparison.Target] {
			return errors.New("specification classifications have invalid targets")
		}
		targets[comparison.Target] = true
		if !knownClassification(comparison.Classification) {
			return fmt.Errorf("%s has unknown classification %q", comparison.Target, comparison.Classification)
		}
		worst := spec.ClassificationNoChange
		for _, change := range comparison.Changes {
			if !knownClassification(change.Classification) {
				return fmt.Errorf("%s has an unknown change classification %q", comparison.Target, change.Classification)
			}
			if classificationRank(change.Classification) > classificationRank(worst) {
				worst = change.Classification
			}
		}
		if len(comparison.Changes) == 0 && comparison.Classification != spec.ClassificationNoChange ||
			len(comparison.Changes) != 0 && comparison.Classification != worst {
			return fmt.Errorf("%s classification is inconsistent with its detailed changes", comparison.Target)
		}
	}
	if len(targets) != 2 {
		return errors.New("specification classifications do not cover both targets")
	}
	if len(manifest.Issues) == 0 {
		for _, candidate := range []CandidateResult{manifest.PinnedCandidate, manifest.ProposedCandidate} {
			if candidate.FileCount <= 0 ||
				!sha256Pattern.MatchString(candidate.FinalSHA256) ||
				!sha256Pattern.MatchString(candidate.RawSHA256) ||
				candidate.Platform == "" ||
				len(candidate.Toolchains) == 0 {
				return errors.New("candidate attestation is incomplete")
			}
		}
		if !sha256Pattern.MatchString(manifest.Generated.BaselineTreeSHA256) ||
			!sha256Pattern.MatchString(manifest.Generated.ProposedTreeSHA256) {
			return errors.New("generated tree attestation is incomplete")
		}
	}
	return nil
}

func knownClassification(classification spec.Classification) bool {
	switch classification {
	case spec.ClassificationNoChange,
		spec.ClassificationProvenanceOnly,
		spec.ClassificationDocumentationOnly,
		spec.ClassificationAdditive,
		spec.ClassificationManualReview,
		spec.ClassificationBreaking:
		return true
	default:
		return false
	}
}

func classificationRank(classification spec.Classification) int {
	switch classification {
	case spec.ClassificationNoChange:
		return 0
	case spec.ClassificationProvenanceOnly:
		return 1
	case spec.ClassificationDocumentationOnly:
		return 2
	case spec.ClassificationAdditive:
		return 3
	case spec.ClassificationManualReview:
		return 4
	case spec.ClassificationBreaking:
		return 5
	default:
		return 6
	}
}

func verifyReviewArtifacts(root string, artifacts map[string]string) error {
	for path, digest := range artifacts {
		data, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if spec.Digest(data) != digest {
			return fmt.Errorf("%s differs from the reviewed manifest", path)
		}
	}
	return nil
}

func sealManifest(root string, manifest *Manifest) error {
	manifest.PlanDigest = ""
	unsigned, err := spec.Canonical(*manifest)
	if err != nil {
		return err
	}
	manifest.PlanDigest = spec.Digest(unsigned)
	data, err := spec.Canonical(*manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, ManifestPath), data, 0o444); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ReportPath), report(*manifest), 0o444)
}

func report(manifest Manifest) []byte {
	var output strings.Builder
	output.WriteString("# Generated SDK specification update\n\n")
	fmt.Fprintf(&output, "Status: `%s`\n\n", manifest.Status)
	fmt.Fprintf(&output, "Plan digest: `%s`\n\n", manifest.PlanDigest)
	fmt.Fprintf(&output, "Retrieval date: `%s`\n\n", manifest.RetrievalDate)
	output.WriteString("## Contract classification\n\n")
	for _, comparison := range manifest.Classifications {
		fmt.Fprintf(&output, "- `%s`: `%s` (%d detailed changes)\n", comparison.Target, comparison.Classification, len(comparison.Changes))
	}
	if len(manifest.Issues) != 0 {
		output.WriteString("\n## Required review\n\n")
		for _, issue := range manifest.Issues {
			fmt.Fprintf(&output, "- %s\n", issue)
		}
	}
	output.WriteString("\n## Generated Go candidate\n\n")
	fmt.Fprintf(&output, "- Added files: %d\n", len(manifest.Generated.AddedFiles))
	fmt.Fprintf(&output, "- Changed files: %d\n", len(manifest.Generated.ChangedFiles))
	fmt.Fprintf(&output, "- Removed files: %d\n", len(manifest.Generated.RemovedFiles))
	fmt.Fprintf(&output, "- Added public declarations: %d\n", len(manifest.Generated.PublicAdded))
	fmt.Fprintf(&output, "- Additive public declaration evolutions: %d\n", len(manifest.Generated.PublicAdditive))
	fmt.Fprintf(&output, "- Changed public declarations: %d\n", len(manifest.Generated.PublicChanged))
	fmt.Fprintf(&output, "- Removed public declarations: %d\n", len(manifest.Generated.PublicRemoved))
	output.WriteString("\nThis report describes generator inputs and an isolated candidate. It does not activate generated clients or approve an SDK release.\n")
	return []byte(output.String())
}

func prepareExternalOutput(repoRoot, output string) error {
	if err := requireOutsideRepository(repoRoot, output); err != nil {
		return errors.New("proposal directory must be outside the repository")
	}
	destination, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("proposal directory already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(destination, 0o755)
}

func requireOutsideRepository(repoRoot, candidate string) error {
	repository, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("proposal parent directory: %w", err)
	}
	destination = filepath.Join(resolvedParent, filepath.Base(destination))
	if _, err := os.Lstat(destination); err == nil {
		destination, err = filepath.EvalSymlinks(destination)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	relative, err := filepath.Rel(repository, destination)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path is inside the repository")
	}
	return nil
}

func readArtifacts(root string, paths []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		result[path] = data
	}
	return result, nil
}

func writeArtifacts(root string, artifacts map[string][]byte) error {
	paths := make([]string, 0, len(artifacts))
	for path := range artifacts {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, artifacts[path], 0o444); err != nil {
			return err
		}
	}
	return nil
}

func artifactDigests(artifacts map[string][]byte) map[string]string {
	result := make(map[string]string, len(artifacts))
	for path, data := range artifacts {
		result[path] = spec.Digest(data)
	}
	return result
}

func verifyArtifacts(root string, digests map[string]string) error {
	for path, digest := range digests {
		data, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if spec.Digest(data) != digest {
			return fmt.Errorf("%s differs from the reviewed proposal", path)
		}
	}
	return nil
}

func decodeProvenance(data []byte) (provenance, error) {
	var result provenance
	if err := spec.DecodeStrict(data, &result); err != nil {
		return provenance{}, err
	}
	if result.SchemaVersion != 1 || len(result.SelectedSpecs) != 2 {
		return provenance{}, errors.New("invalid pinned specification provenance")
	}
	return result, nil
}

func componentSchemaCount(root map[string]any) int {
	components, _ := root["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	return len(schemas)
}
