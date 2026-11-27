package generatortrust

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/fixturedeps"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/templatepolicy"
)

const TrustRootPath = "tools/generated-sdk/generator/trust-root.json"

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type TrustRoot struct {
	Configs           map[string]Artifact `json:"configs"`
	Container         Container           `json:"container"`
	CustomTemplates   CustomTemplates     `json:"custom_templates"`
	Environment       Environment         `json:"environment"`
	Generator         Generator           `json:"generator"`
	GoToolchains      []string            `json:"go_toolchains"`
	PolicyArtifacts   map[string]Artifact `json:"policy_artifacts"`
	SchemaVersion     int                 `json:"schema_version"`
	Specifications    map[string]Artifact `json:"specifications"`
	StockTemplates    []Artifact          `json:"stock_templates"`
	TemplateArtifacts map[string]Artifact `json:"template_artifacts"`
	TestContainer     GoContainer         `json:"test_container"`
	Wrapper           Artifact            `json:"wrapper"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Container struct {
	ImageRepository string     `json:"image_repository"`
	JDKVersion      string     `json:"jdk_version"`
	Platforms       []Platform `json:"platforms"`
}

type GoContainer struct {
	GoVersion       string     `json:"go_version"`
	ImageRepository string     `json:"image_repository"`
	Platforms       []Platform `json:"platforms"`
}

type Platform struct {
	Digest   string `json:"digest"`
	Platform string `json:"platform"`
}

type CustomTemplates struct {
	Entries []CustomTemplate `json:"entries"`
	Status  string           `json:"status"`
}

type CustomTemplate struct {
	CustomPath   string `json:"custom_path"`
	CustomSHA256 string `json:"custom_sha256"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StockPath    string `json:"stock_path"`
	StockSHA256  string `json:"stock_sha256"`
}

type Environment struct {
	AllowedVariables     map[string]string `json:"allowed_variables"`
	ClearingPolicy       string            `json:"clearing_policy"`
	Network              string            `json:"network"`
	PassThroughVariables []string          `json:"pass_through_variables"`
}

type Generator struct {
	JAR     JAR    `json:"jar"`
	Version string `json:"version"`
}

type JAR struct {
	CachePath string `json:"cache_path"`
	SHA256    string `json:"sha256"`
	SourceURL string `json:"source_url"`
}

type Config struct {
	AdditionalProperties map[string]any    `json:"additionalProperties"`
	GeneratorName        string            `json:"generatorName"`
	GlobalProperties     map[string]string `json:"globalProperties"`
	InputSpec            string            `json:"inputSpec"`
	OutputDir            string            `json:"outputDir"`
	TemplateDir          string            `json:"templateDir"`
	TypeMappings         map[string]string `json:"typeMappings"`
}

type imageIdentity struct {
	Architecture string `json:"Architecture"`
	OS           string `json:"Os"`
	Variant      string `json:"Variant"`
}

type DigestReport struct {
	Artifacts      map[string]string `json:"artifacts"`
	StockTemplates map[string]string `json:"stock_templates"`
}

func Load(repoRoot string) (*TrustRoot, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(TrustRootPath)))
	if err != nil {
		return nil, err
	}
	return Decode(data)
}

func Decode(data []byte) (*TrustRoot, error) {
	var trust TrustRoot
	if err := spec.DecodeStrict(data, &trust); err != nil {
		return nil, fmt.Errorf("decode trust root: %w", err)
	}
	canonical, err := spec.Canonical(trust)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, errors.New("generator trust root is not canonical JSON")
	}
	if err := ValidateManifest(&trust); err != nil {
		return nil, err
	}
	return &trust, nil
}

func VerifyTracked(repoRoot string) error {
	_, err := verifyTracked(repoRoot)
	return err
}

func verifyTracked(repoRoot string) (*TrustRoot, error) {
	trust, err := Load(repoRoot)
	if err != nil {
		return nil, err
	}
	if err := verifyArtifacts(repoRoot, trust); err != nil {
		return nil, err
	}
	if _, err := validateContainedPath(repoRoot, trust.Generator.JAR.CachePath, true); err != nil {
		return nil, err
	}
	if err := templatepolicy.VerifyTracked(repoRoot); err != nil {
		return nil, err
	}
	if err := fixturedeps.VerifyLocks(repoRoot); err != nil {
		return nil, err
	}
	return trust, nil
}

func Verify(repoRoot string) error {
	trust, err := verifyTracked(repoRoot)
	if err != nil {
		return err
	}
	cachePath := filepath.Join(repoRoot, filepath.FromSlash(trust.Generator.JAR.CachePath))
	if err := VerifyJAR(cachePath, trust); err != nil {
		return err
	}
	return templatepolicy.Verify(repoRoot)
}

func Bootstrap(ctx context.Context, repoRoot string) error {
	trust, err := Load(repoRoot)
	if err != nil {
		return err
	}
	if err := verifyArtifacts(repoRoot, trust); err != nil {
		return err
	}
	cachePath := filepath.Join(repoRoot, filepath.FromSlash(trust.Generator.JAR.CachePath))
	if _, err := validateContainedPath(repoRoot, trust.Generator.JAR.CachePath, true); err != nil {
		return err
	}
	if err := VerifyJAR(cachePath, trust); err != nil {
		if err := downloadJAR(ctx, trust.Generator.JAR, cachePath); err != nil {
			return err
		}
	}
	return Verify(repoRoot)
}

func SnapshotInputs(repoRoot, destination string, trust *TrustRoot) error {
	return SnapshotInputsWithOverrides(repoRoot, destination, trust, nil)
}

func SnapshotInputsWithOverrides(
	repoRoot, destination string,
	trust *TrustRoot,
	overrides map[string][]byte,
) error {
	sourceRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	snapshotRoot, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if pathInside(sourceRoot, snapshotRoot) {
		return errors.New("generator input snapshot must be outside the repository")
	}
	artifacts := map[string]Artifact{}
	add := func(artifact Artifact) error {
		if existing, ok := artifacts[artifact.Path]; ok && existing.SHA256 != artifact.SHA256 {
			return fmt.Errorf("conflicting trusted digests for %s", artifact.Path)
		}
		artifacts[artifact.Path] = artifact
		return nil
	}
	if err := add(Artifact{Path: trust.Generator.JAR.CachePath, SHA256: trust.Generator.JAR.SHA256}); err != nil {
		return err
	}
	for _, group := range []map[string]Artifact{
		trust.Configs,
		trust.Specifications,
		trust.TemplateArtifacts,
	} {
		for _, artifact := range group {
			if err := add(artifact); err != nil {
				return err
			}
		}
	}
	for _, template := range trust.CustomTemplates.Entries {
		if err := add(Artifact{Path: template.CustomPath, SHA256: template.CustomSHA256}); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(artifacts))
	for path := range artifacts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		artifact := artifacts[path]
		content, overridden := overrides[path]
		if !overridden {
			source, err := validateContainedPath(repoRoot, artifact.Path, false)
			if err != nil {
				return err
			}
			content, err = os.ReadFile(source)
			if err != nil {
				return err
			}
		}
		if digest := spec.Digest(content); digest != artifact.SHA256 {
			return fmt.Errorf("generator input %s SHA-256 is %s, want %s", artifact.Path, digest, artifact.SHA256)
		}
		target := filepath.Join(snapshotRoot, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		// #nosec G703 -- artifact paths passed repository containment checks and snapshotRoot is a fresh temporary directory.
		if err := os.WriteFile(target, content, 0o444); err != nil {
			return err
		}
	}
	for path := range overrides {
		if _, ok := artifacts[path]; !ok {
			return fmt.Errorf("unapproved generator input override %s", path)
		}
	}
	return nil
}

func ProposeDigests(repoRoot string) ([]byte, error) {
	trust, err := Load(repoRoot)
	if err != nil {
		return nil, err
	}
	artifacts := map[string]Artifact{}
	add := func(artifact Artifact) {
		artifacts[artifact.Path] = artifact
	}
	add(Artifact{Path: trust.Generator.JAR.CachePath})
	add(trust.Wrapper)
	for _, group := range []map[string]Artifact{
		trust.Configs,
		trust.PolicyArtifacts,
		trust.Specifications,
		trust.TemplateArtifacts,
	} {
		for _, artifact := range group {
			add(artifact)
		}
	}
	for _, template := range trust.CustomTemplates.Entries {
		add(Artifact{Path: template.CustomPath})
	}
	report := DigestReport{
		Artifacts:      make(map[string]string, len(artifacts)),
		StockTemplates: make(map[string]string, len(trust.StockTemplates)),
	}
	for path := range artifacts {
		contained, err := validateContainedPath(repoRoot, path, false)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(contained)
		if err != nil {
			return nil, err
		}
		report.Artifacts[path] = spec.Digest(content)
	}
	jarPath, err := validateContainedPath(repoRoot, trust.Generator.JAR.CachePath, false)
	if err != nil {
		return nil, err
	}
	jarData, err := os.ReadFile(jarPath)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(jarData), int64(len(jarData)))
	if err != nil {
		return nil, err
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	for _, template := range trust.StockTemplates {
		file := entries[template.Path]
		if file == nil {
			return nil, fmt.Errorf("stock template %s is missing from generator JAR", template.Path)
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		report.StockTemplates[template.Path] = spec.Digest(content)
	}
	return spec.Canonical(report)
}

func DefaultPlatform(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "linux/amd64", nil
	case "arm64":
		return "linux/arm64/v8", nil
	default:
		return "", fmt.Errorf("unsupported generator host architecture %q", goarch)
	}
}

func ValidateHostArchitecture(goarch string) error {
	_, err := DefaultPlatform(goarch)
	return err
}

func SelectPlatform(trust *TrustRoot, requested string) (Platform, error) {
	return selectPlatform(trust.Container.Platforms, requested)
}

func SelectTestPlatform(trust *TrustRoot, requested string) (Platform, error) {
	return selectPlatform(trust.TestContainer.Platforms, requested)
}

func selectPlatform(platforms []Platform, requested string) (Platform, error) {
	for _, platform := range platforms {
		if platform.Platform == requested {
			return platform, nil
		}
	}
	return Platform{}, fmt.Errorf("unsupported generator platform %q", requested)
}

func ImageReference(trust *TrustRoot, platform Platform) string {
	return trust.Container.ImageRepository + "@" + platform.Digest
}

func TestImageReference(trust *TrustRoot, platform Platform) string {
	return trust.TestContainer.ImageRepository + "@" + platform.Digest
}

func VerifyImageJDK(ctx context.Context, trust *TrustRoot, platform Platform) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("docker is required to verify the generator image")
	}
	image := ImageReference(trust, platform)
	if err := verifyCachedImageIdentity(ctx, docker, image, platform.Platform); err != nil {
		return err
	}
	command := exec.CommandContext(
		ctx,
		docker,
		"run", "--rm", "--pull=never", "--network=none", "--read-only",
		"--platform="+platform.Platform,
		"--cap-drop=ALL", "--security-opt=no-new-privileges=true", "--pids-limit=64",
		"--user=65534:65534",
		"--entrypoint=/opt/java/openjdk/bin/java",
		image,
		"-version",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify generator JDK: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return ValidateJDKVersion(string(output), trust.Container.JDKVersion)
}

func VerifyTestImageGo(ctx context.Context, trust *TrustRoot, platform Platform) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("docker is required to verify the generated-code test image")
	}
	image := TestImageReference(trust, platform)
	if err := verifyCachedImageIdentity(ctx, docker, image, platform.Platform); err != nil {
		return err
	}
	command := exec.CommandContext(
		ctx,
		docker,
		"run", "--rm", "--pull=never", "--network=none", "--read-only",
		"--platform="+platform.Platform,
		"--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=64",
		"--user=65534:65534",
		"--entrypoint=/usr/local/go/bin/go",
		image,
		"version",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify generated-code test image: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return ValidateGoVersion(string(output), trust.TestContainer.GoVersion, platform.Platform)
}

func ValidateJDKVersion(output, expected string) error {
	parts := strings.Split(expected, "+")
	if len(parts) != 2 || !strings.Contains(output, parts[0]) || !strings.Contains(output, "+"+parts[1]) {
		return fmt.Errorf("generator JDK identity differs from %s: %s", expected, strings.TrimSpace(output))
	}
	return nil
}

func ValidateGoVersion(output, expected, platform string) error {
	goarch := strings.TrimPrefix(platform, "linux/")
	goarch = strings.TrimSuffix(goarch, "/v8")
	want := "go version go" + expected + " linux/" + goarch
	if strings.TrimSpace(output) != want {
		return fmt.Errorf("generated-code test Go identity differs: got %q, want %q", strings.TrimSpace(output), want)
	}
	return nil
}

func BootstrapImage(ctx context.Context, repoRoot, requested string) (string, error) {
	trust, err := Load(repoRoot)
	if err != nil {
		return "", err
	}
	if err := verifyArtifacts(repoRoot, trust); err != nil {
		return "", err
	}
	platform, err := SelectPlatform(trust, requested)
	if err != nil {
		return "", err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return "", errors.New("docker is required to bootstrap the generator image")
	}
	image := ImageReference(trust, platform)
	pull := exec.CommandContext(ctx, docker, "pull", "--platform="+platform.Platform, image)
	if output, err := pull.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pull generator image: %w: %s", err, strings.TrimSpace(string(output)))
	}
	inspect := exec.CommandContext(ctx, docker, "image", "inspect", "--format", "{{json .}}", image)
	output, err := inspect.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect generator image: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var identity imageIdentity
	if err := json.Unmarshal(bytes.TrimSpace(output), &identity); err != nil {
		return "", fmt.Errorf("decode generator image identity: %w", err)
	}
	if err := validateImageIdentity(platform.Platform, identity); err != nil {
		return "", err
	}
	return image, nil
}

func BootstrapTestImage(ctx context.Context, repoRoot, requested string) (string, error) {
	trust, err := Load(repoRoot)
	if err != nil {
		return "", err
	}
	if err := verifyArtifacts(repoRoot, trust); err != nil {
		return "", err
	}
	platform, err := selectPlatform(trust.TestContainer.Platforms, requested)
	if err != nil {
		return "", err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return "", errors.New("docker is required to bootstrap the generated-code test image")
	}
	image := TestImageReference(trust, platform)
	pull := exec.CommandContext(ctx, docker, "pull", "--platform="+platform.Platform, image)
	if output, err := pull.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pull generated-code test image: %w: %s", err, strings.TrimSpace(string(output)))
	}
	inspect := exec.CommandContext(ctx, docker, "image", "inspect", "--format", "{{json .}}", image)
	output, err := inspect.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect generated-code test image: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var identity imageIdentity
	if err := json.Unmarshal(bytes.TrimSpace(output), &identity); err != nil {
		return "", fmt.Errorf("decode generated-code test image identity: %w", err)
	}
	if err := validateImageIdentity(platform.Platform, identity); err != nil {
		return "", err
	}
	if err := VerifyTestImageGo(ctx, trust, platform); err != nil {
		return "", err
	}
	return image, nil
}

func validateImageIdentity(platform string, identity imageIdentity) error {
	want := imageIdentity{OS: "linux"}
	switch platform {
	case "linux/amd64":
		want.Architecture = "amd64"
	case "linux/arm64/v8":
		want.Architecture = "arm64"
		want.Variant = "v8"
	default:
		return fmt.Errorf("unsupported generator platform %q", platform)
	}
	if identity != want {
		return fmt.Errorf(
			"generator image identity is %s/%s/%s, want %s/%s/%s",
			identity.OS, identity.Architecture, identity.Variant,
			want.OS, want.Architecture, want.Variant,
		)
	}
	return nil
}

func verifyCachedImageIdentity(
	ctx context.Context,
	docker, image, platform string,
) error {
	inspect := exec.CommandContext(ctx, docker, "image", "inspect", "--format", "{{json .}}", image)
	output, err := inspect.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect cached image %s: %w: %s", image, err, strings.TrimSpace(string(output)))
	}
	var identity imageIdentity
	if err := json.Unmarshal(bytes.TrimSpace(output), &identity); err != nil {
		return fmt.Errorf("decode cached image identity: %w", err)
	}
	return validateImageIdentity(platform, identity)
}

func ValidateManifest(trust *TrustRoot) error {
	if trust.SchemaVersion != 1 {
		return fmt.Errorf("schema version is %d, want 1", trust.SchemaVersion)
	}
	if trust.Generator.Version != "7.27.0" {
		return fmt.Errorf("generator version is %q, want 7.27.0", trust.Generator.Version)
	}
	expectedJAR := JAR{
		CachePath: "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar",
		SHA256:    "db9c4b738727698748746144394cb7ee96512fe50166555baaeeca9311ee1dba",
		SourceURL: "https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/7.27.0/openapi-generator-cli-7.27.0.jar",
	}
	if trust.Generator.JAR != expectedJAR {
		return fmt.Errorf("generator JAR trust differs from reviewed value")
	}
	expectedContainer := Container{
		ImageRepository: "docker.io/library/eclipse-temurin",
		JDKVersion:      "17.0.21+9",
		Platforms: []Platform{
			{Digest: "sha256:101376794254cc58a1aa952a35ade1709cbc0f5942d5da35e226009171dde46e", Platform: "linux/amd64"},
			{Digest: "sha256:80fbefa720be4552d66b64b8d6b77a5e8daf3d75effe2a544c2acc8d7a4237d5", Platform: "linux/arm64/v8"},
		},
	}
	if !reflect.DeepEqual(trust.Container, expectedContainer) {
		return errors.New("container trust differs from reviewed platform or JDK identity")
	}
	expectedTestContainer := GoContainer{
		GoVersion:       "1.27.3",
		ImageRepository: "docker.io/library/golang",
		Platforms: []Platform{
			{Digest: "sha256:025eefc1e8dbe63201734a72aaa0a55d7a077f2d8410432f8ae2f3e939f32871", Platform: "linux/amd64"},
			{Digest: "sha256:bc8d7053145982971b2c25138ba67b259d9a2df739a7a715d45ca6368c25c1e0", Platform: "linux/arm64/v8"},
		},
	}
	if !reflect.DeepEqual(trust.TestContainer, expectedTestContainer) {
		return errors.New("generated-code test container differs from reviewed platform or Go identity")
	}
	if !reflect.DeepEqual(trust.GoToolchains, []string{"1.26.10", "1.27.3"}) {
		return fmt.Errorf("go toolchain matrix is %v, want [1.26.10 1.27.3]", trust.GoToolchains)
	}
	expectedEnvironment := Environment{
		AllowedVariables: map[string]string{
			"JAVA_HOME": "/opt/java/openjdk",
			"LANG":      "C.UTF-8",
			"LC_ALL":    "C.UTF-8",
			"PATH":      "/opt/java/openjdk/bin:/usr/bin:/bin",
			"TZ":        "UTC",
		},
		ClearingPolicy: "env-i", Network: "disabled", PassThroughVariables: []string{},
	}
	if !reflect.DeepEqual(trust.Environment, expectedEnvironment) {
		return errors.New("generation environment policy differs from reviewed values")
	}
	expectedTemplates := map[string]string{
		"go/model_enum.mustache":   "c7a5069f3c2896887abf53add1d1a83717a96dec4d25f79d8154c4227aeffda7",
		"go/model_anyof.mustache":  "29694d9c68575cbba5c8fd9af3de3633437a31836a4e17588a143951c8c15741",
		"go/model_oneof.mustache":  "98148bb5ed88123a26e1f0ec0c1f2b18744934732407858cd2359702b2c0ec5c",
		"go/model_simple.mustache": "80293db777ee6940cfc7f489e73d0c540990263a4e6d75a41aad5a711d7ff861",
	}
	if len(trust.StockTemplates) != len(expectedTemplates) {
		return fmt.Errorf("stock template count is %d, want %d", len(trust.StockTemplates), len(expectedTemplates))
	}
	seenTemplates := map[string]bool{}
	for _, template := range trust.StockTemplates {
		if expectedTemplates[template.Path] != template.SHA256 {
			return fmt.Errorf("stock template %s digest differs from reviewed value", template.Path)
		}
		if seenTemplates[template.Path] {
			return fmt.Errorf("duplicate stock template %s", template.Path)
		}
		seenTemplates[template.Path] = true
	}
	if err := validateCustomTemplates(trust.CustomTemplates); err != nil {
		return err
	}
	expectedConfigs := map[string]Artifact{
		"trading": {
			Path:   "tools/generated-sdk/generator/config/trading.json",
			SHA256: "acad28d0a2de371dcc49c6eebfce8c0f3158f58dba7a868769b6898dfd0c42b0",
		},
		"market-data": {
			Path:   "tools/generated-sdk/generator/config/market-data.json",
			SHA256: "93d0e0ad7ae59b0dd4a864dcadde5607a7eb95d9947385bb0ad37c573c48335b",
		},
	}
	if !reflect.DeepEqual(trust.Configs, expectedConfigs) {
		return errors.New("generator config trust differs from reviewed paths or digests")
	}
	if err := validateRotatingArtifacts(trust.Specifications, map[string]string{
		"trading":     "tools/generated-sdk/specs/trading.openapi.json",
		"market-data": "tools/generated-sdk/specs/market-data.openapi.json",
	}); err != nil {
		return fmt.Errorf("specification trust: %w", err)
	}
	if err := validateRotatingArtifacts(trust.PolicyArtifacts, map[string]string{
		"feature_allowlist": "tools/generated-sdk/specs/feature-allowlist.json",
		"provenance":        "tools/generated-sdk/specs/provenance.json",
	}); err != nil {
		return fmt.Errorf("specification policy trust: %w", err)
	}
	expectedWrapper := Artifact{
		Path:   "tools/generated-sdk/generator/run-pinned.sh",
		SHA256: "70cbf141e09748aac15a5b7734d749acc535b8768753fe4af6e7dba760fb8980",
	}
	if trust.Wrapper != expectedWrapper {
		return errors.New("generator wrapper differs from reviewed path or digest")
	}
	expectedTemplateArtifacts := map[string]Artifact{
		"fixture_mod": {
			Path:   "tools/generated-sdk/generator/fixtures/template-fixture.mod",
			SHA256: "d791a2e15e0cb120c3f3694ca0274bf24283a89060dc0692a256e3647cac5ca6",
		},
		"fixture_spec": {
			Path:   "tools/generated-sdk/generator/fixtures/template-policy.openapi.json",
			SHA256: "a95b8c220c94ed4e0375ec8e82aae873e801545e1a1b71e8b67a8f6de104716f",
		},
		"fixture_sum": {
			Path:   "tools/generated-sdk/generator/fixtures/template-fixture.sum",
			SHA256: "eca8ec9378c8637435ba9725cb7c1375b8dbb5ab404f8e128b18e9fe01d14d68",
		},
		"fixture_test": {
			Path:   "tools/generated-sdk/generator/fixtures/template_policy_test.go.tmpl",
			SHA256: "35b13985459f5df81be430eeb0a25553019ecb7ff4472e8e474919778e453344",
		},
		"stock_digests": {
			Path:   "tools/generated-sdk/generator/stock-template-digests.json",
			SHA256: "15bbf674cb0bfa3274fa2e6e5b7afee5d8594fd03975c08d14038b0aed5c9d22",
		},
		"template_deltas": {
			Path:   "tools/generated-sdk/generator/template-deltas.json",
			SHA256: "3c7fe4ab0faa1327c7cca0fd406dcb774bca571581432787f4673f6aea3f7434",
		},
	}
	overlay, ok := trust.TemplateArtifacts["overlays"]
	if !ok || overlay.Path != "tools/generated-sdk/generator/overlays.json" ||
		!sha256Pattern.MatchString(overlay.SHA256) {
		return errors.New("overlay trust differs from reviewed path or digest format")
	}
	staticTemplateArtifacts := make(map[string]Artifact, len(trust.TemplateArtifacts)-1)
	for name, artifact := range trust.TemplateArtifacts {
		if name != "overlays" {
			staticTemplateArtifacts[name] = artifact
		}
	}
	if !reflect.DeepEqual(staticTemplateArtifacts, expectedTemplateArtifacts) {
		return errors.New("template artifact trust differs from reviewed paths or digests")
	}
	for _, artifact := range append(append(append(append(mapArtifacts(trust.Configs), mapArtifacts(trust.Specifications)...), mapArtifacts(trust.PolicyArtifacts)...), mapArtifacts(trust.TemplateArtifacts)...), trust.Wrapper) {
		if err := validateRepoPath(artifact.Path); err != nil {
			return err
		}
	}
	if err := validateRepoPath(trust.Generator.JAR.CachePath); err != nil {
		return err
	}
	return nil
}

func validateRotatingArtifacts(actual map[string]Artifact, expectedPaths map[string]string) error {
	if len(actual) != len(expectedPaths) {
		return fmt.Errorf("artifact count is %d, want %d", len(actual), len(expectedPaths))
	}
	for name, path := range expectedPaths {
		artifact, ok := actual[name]
		if !ok {
			return fmt.Errorf("missing artifact %q", name)
		}
		if artifact.Path != path {
			return fmt.Errorf("%s path is %q, want %q", name, artifact.Path, path)
		}
		if !sha256Pattern.MatchString(artifact.SHA256) {
			return fmt.Errorf("%s SHA-256 is not 64 lowercase hexadecimal characters", name)
		}
	}
	return nil
}

func VerifyJAR(path string, trust *TrustRoot) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("verified generator JAR cache is missing at %s; run make generated-sdk-generator-bootstrap", path)
		}
		return err
	}
	if digest := spec.Digest(data); digest != trust.Generator.JAR.SHA256 {
		return fmt.Errorf("generator JAR SHA-256 is %s, want %s", digest, trust.Generator.JAR.SHA256)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open generator JAR: %w", err)
	}
	entries := map[string]*zip.File{}
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	for _, template := range trust.StockTemplates {
		file := entries[template.Path]
		if file == nil {
			return fmt.Errorf("stock template %s is missing from generator JAR", template.Path)
		}
		stream, err := file.Open()
		if err != nil {
			return err
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if digest := spec.Digest(content); digest != template.SHA256 {
			return fmt.Errorf("stock template %s SHA-256 is %s, want %s", template.Path, digest, template.SHA256)
		}
	}
	return nil
}

func verifyArtifacts(repoRoot string, trust *TrustRoot) error {
	for target, artifact := range trust.Configs {
		if err := verifyArtifact(repoRoot, artifact, "config "+target); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return err
		}
		var config Config
		if err := spec.DecodeStrict(data, &config); err != nil {
			return fmt.Errorf("decode config %s: %w", target, err)
		}
		canonical, err := spec.Canonical(config)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, canonical) {
			return fmt.Errorf("config %s is not canonical JSON", target)
		}
		if err := ValidateConfig(target, &config); err != nil {
			return err
		}
	}
	for target, artifact := range trust.Specifications {
		if err := verifyArtifact(repoRoot, artifact, "specification "+target); err != nil {
			return err
		}
	}
	for name, artifact := range trust.PolicyArtifacts {
		if err := verifyArtifact(repoRoot, artifact, "policy artifact "+name); err != nil {
			return err
		}
	}
	for name, artifact := range trust.TemplateArtifacts {
		if err := verifyArtifact(repoRoot, artifact, "template artifact "+name); err != nil {
			return err
		}
	}
	return verifyArtifact(repoRoot, trust.Wrapper, "wrapper")
}

func ValidateConfig(target string, config *Config) error {
	var packageName, inputSpec, outputDir string
	switch target {
	case "trading":
		packageName = "trading"
		inputSpec = "/workspace/tools/generated-sdk/specs/trading.openapi.json"
		outputDir = "/output/trading"
	case "market-data":
		packageName = "marketdata"
		inputSpec = "/workspace/tools/generated-sdk/specs/market-data.openapi.json"
		outputDir = "/output/market-data"
	default:
		return fmt.Errorf("unknown generator config target %q", target)
	}
	expected := Config{
		GeneratorName: "go", InputSpec: inputSpec, OutputDir: outputDir,
		TemplateDir: "/workspace/tools/generated-sdk/generator/templates/go",
		AdditionalProperties: map[string]any{
			"disallowAdditionalPropertiesIfNotPresent": false,
			"enumClassPrefix":                          true,
			"enumUnknownDefaultCase":                   false,
			"generateInterfaces":                       false,
			"generateMarshalJSON":                      true,
			"generateUnmarshalJSON":                    true,
			"hideGenerationTimestamp":                  true,
			"isGoSubmodule":                            false,
			"packageName":                              packageName,
			"packageVersion":                           "0.0.0-internal",
			"prependFormOrBodyParameters":              false,
			"structPrefix":                             false,
			"useDefaultValuesForRequiredVars":          false,
			"useHttpHeaderSet":                         false,
			"useOneOfDiscriminatorLookup":              true,
			"withAWSV4Signature":                       false,
			"withGoMod":                                false,
			"withXml":                                  false,
		},
		GlobalProperties: map[string]string{
			"apiDocs": "false", "apiTests": "false", "modelDocs": "false", "modelTests": "false",
		},
		TypeMappings: map[string]string{"string+decimal": "string"},
	}
	if !reflect.DeepEqual(config, &expected) {
		return fmt.Errorf("generator config %s differs from reviewed explicit policy", target)
	}
	return nil
}

func downloadJAR(ctx context.Context, jar JAR, cachePath string) (resultErr error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, jar.SourceURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download generator JAR: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, response.Body.Close())
	}()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download generator JAR: %s", response.Status)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 128<<20))
	if err != nil {
		return err
	}
	if digest := spec.Digest(content); digest != jar.SHA256 {
		return fmt.Errorf("downloaded generator JAR SHA-256 is %s, want %s", digest, jar.SHA256)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(cachePath), ".openapi-generator-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		removeErr := os.Remove(tempPath)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, removeErr)
		}
	}()
	if _, err := temp.Write(content); err != nil {
		return errors.Join(err, temp.Close())
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, cachePath)
}

func verifyArtifact(repoRoot string, artifact Artifact, description string) error {
	path, err := validateContainedPath(repoRoot, artifact.Path, false)
	if err != nil {
		return fmt.Errorf("%s: %w", description, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if digest := spec.Digest(data); digest != artifact.SHA256 {
		return fmt.Errorf("%s SHA-256 is %s, want %s", description, digest, artifact.SHA256)
	}
	return nil
}

func validateCustomTemplates(custom CustomTemplates) error {
	if custom.Status != "reviewed" || len(custom.Entries) != 4 {
		return errors.New("custom templates must be reviewed")
	}
	expected := map[string]CustomTemplate{
		"model_enum.mustache": {
			Name: "model_enum.mustache", Status: "reviewed",
			StockPath: "go/model_enum.mustache", StockSHA256: "c7a5069f3c2896887abf53add1d1a83717a96dec4d25f79d8154c4227aeffda7",
			CustomPath:   "tools/generated-sdk/generator/templates/go/model_enum.mustache",
			CustomSHA256: "153e3a586982664544edb2849c8c7fd517a7cea5a635825f5219f755bdb2b2d3",
		},
		"model_anyof.mustache": {
			Name: "model_anyof.mustache", Status: "reviewed",
			StockPath: "go/model_anyof.mustache", StockSHA256: "29694d9c68575cbba5c8fd9af3de3633437a31836a4e17588a143951c8c15741",
			CustomPath:   "tools/generated-sdk/generator/templates/go/model_anyof.mustache",
			CustomSHA256: "0e4d10580c8801b9de78c1a1904d9e27ad75041934416fd7d495aa88445f20b6",
		},
		"model_oneof.mustache": {
			Name: "model_oneof.mustache", Status: "reviewed",
			StockPath: "go/model_oneof.mustache", StockSHA256: "98148bb5ed88123a26e1f0ec0c1f2b18744934732407858cd2359702b2c0ec5c",
			CustomPath:   "tools/generated-sdk/generator/templates/go/model_oneof.mustache",
			CustomSHA256: "1c0058a441cf6121b22e675206ec8ce2058394e6baa85b0248248c63d561a59f",
		},
		"model_simple.mustache": {
			Name: "model_simple.mustache", Status: "reviewed",
			StockPath: "go/model_simple.mustache", StockSHA256: "80293db777ee6940cfc7f489e73d0c540990263a4e6d75a41aad5a711d7ff861",
			CustomPath:   "tools/generated-sdk/generator/templates/go/model_simple.mustache",
			CustomSHA256: "e0ea8a1b093106f9bb8a2b283779523e38bc15674a20136f3c80e7eb227de70d",
		},
	}
	actual := make(map[string]CustomTemplate, len(custom.Entries))
	for _, entry := range custom.Entries {
		if _, duplicate := actual[entry.Name]; duplicate {
			return fmt.Errorf("duplicate custom template %s", entry.Name)
		}
		actual[entry.Name] = entry
	}
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("custom template entries differ from reviewed values")
	}
	return nil
}

func mapArtifacts(values map[string]Artifact) []Artifact {
	result := make([]Artifact, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func validateRepoPath(path string) error {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != filepath.FromSlash(path) || strings.HasPrefix(path, "..") {
		return fmt.Errorf("%q is not an unambiguous repository-relative path", path)
	}
	return nil
}

func validateContainedPath(repoRoot, relative string, allowMissing bool) (string, error) {
	if err := validateRepoPath(relative); err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", err
	}
	path := filepath.Join(repoRoot, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("%q is not a regular non-symlink file", relative)
		}
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return "", resolveErr
		}
		if !pathInside(root, resolved) {
			return "", fmt.Errorf("%q resolves outside the repository", relative)
		}
		return resolved, nil
	}
	if !os.IsNotExist(err) || !allowMissing {
		return "", err
	}
	parent := filepath.Dir(path)
	for {
		if _, statErr := os.Lstat(parent); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("no existing parent for %q", relative)
		}
		parent = next
	}
	parent, resolveErr := filepath.EvalSymlinks(parent)
	if resolveErr != nil {
		return "", resolveErr
	}
	if !pathInside(root, parent) {
		return "", fmt.Errorf("%q has a parent outside the repository", relative)
	}
	return path, nil
}

func pathInside(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
