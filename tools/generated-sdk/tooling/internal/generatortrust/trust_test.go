package generatortrust

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

func TestTrustRootMutationsFail(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TrustRoot)
	}{
		{"jar-version", func(trust *TrustRoot) { trust.Generator.Version = "7.27.1" }},
		{"jar-hash", func(trust *TrustRoot) { trust.Generator.JAR.SHA256 = "bad" }},
		{"stock-template-path", func(trust *TrustRoot) { trust.StockTemplates[0].Path = "go/future.mustache" }},
		{"stock-template-hash", func(trust *TrustRoot) { trust.StockTemplates[0].SHA256 = "bad" }},
		{"tag-only-container", func(trust *TrustRoot) { trust.Container.ImageRepository = "docker.io/library/eclipse-temurin:17-jdk" }},
		{"platform-digest", func(trust *TrustRoot) { trust.Container.Platforms[0].Digest = "sha256:bad" }},
		{"test-container-digest", func(trust *TrustRoot) { trust.TestContainer.Platforms[0].Digest = "sha256:bad" }},
		{"test-container-version", func(trust *TrustRoot) { trust.TestContainer.GoVersion = "1.27.4" }},
		{"go-matrix", func(trust *TrustRoot) { trust.GoToolchains[1] = "1.27.4" }},
		{"environment", func(trust *TrustRoot) { trust.Environment.AllowedVariables["TZ"] = "America/New_York" }},
		{"wrapper-hash", func(trust *TrustRoot) { trust.Wrapper.SHA256 = "bad" }},
		{"config-hash", func(trust *TrustRoot) {
			artifact := trust.Configs["trading"]
			artifact.SHA256 = "bad"
			trust.Configs["trading"] = artifact
		}},
		{"custom-template-status", func(trust *TrustRoot) { trust.CustomTemplates.Status = "pending" }},
		{"fixture-mod-lock-hash", func(trust *TrustRoot) {
			artifact := trust.TemplateArtifacts["fixture_mod"]
			artifact.SHA256 = "bad"
			trust.TemplateArtifacts["fixture_mod"] = artifact
		}},
		{"fixture-sum-lock-hash", func(trust *TrustRoot) {
			artifact := trust.TemplateArtifacts["fixture_sum"]
			artifact.SHA256 = "bad"
			trust.TemplateArtifacts["fixture_sum"] = artifact
		}},
		{"feature-allowlist-hash", func(trust *TrustRoot) {
			artifact := trust.PolicyArtifacts["feature_allowlist"]
			artifact.SHA256 = "bad"
			trust.PolicyArtifacts["feature_allowlist"] = artifact
		}},
		{"provenance-hash", func(trust *TrustRoot) {
			artifact := trust.PolicyArtifacts["provenance"]
			artifact.SHA256 = "bad"
			trust.PolicyArtifacts["provenance"] = artifact
		}},
		{"overlay-hash", func(trust *TrustRoot) {
			artifact := trust.TemplateArtifacts["overlays"]
			artifact.SHA256 = "bad"
			trust.TemplateArtifacts["overlays"] = artifact
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trust := validTrust()
			test.mutate(trust)
			if err := ValidateManifest(trust); err == nil {
				t.Fatal("mutated trust root unexpectedly passed")
			}
		})
	}
}

func TestValidTrustManifest(t *testing.T) {
	if err := ValidateManifest(validTrust()); err != nil {
		t.Fatal(err)
	}
}

func TestRotatingSpecificationLocksAcceptReviewedDigests(t *testing.T) {
	trust := validTrust()
	trading := trust.Specifications["trading"]
	trading.SHA256 = string(bytes.Repeat([]byte{'a'}, 64))
	trust.Specifications["trading"] = trading
	allowlist := trust.PolicyArtifacts["feature_allowlist"]
	allowlist.SHA256 = string(bytes.Repeat([]byte{'b'}, 64))
	trust.PolicyArtifacts["feature_allowlist"] = allowlist
	overlays := trust.TemplateArtifacts["overlays"]
	overlays.SHA256 = string(bytes.Repeat([]byte{'c'}, 64))
	trust.TemplateArtifacts["overlays"] = overlays
	if err := ValidateManifest(trust); err != nil {
		t.Fatal(err)
	}
}

func TestPlatformSelectionFailsClosed(t *testing.T) {
	trust := validTrust()
	for architecture, want := range map[string]string{
		"amd64": "linux/amd64",
		"arm64": "linux/arm64/v8",
	} {
		got, err := DefaultPlatform(architecture)
		if err != nil || got != want {
			t.Fatalf("DefaultPlatform(%q) = %q, %v; want %q", architecture, got, err, want)
		}
	}
	if _, err := DefaultPlatform("riscv64"); err == nil {
		t.Fatal("unsupported host architecture unexpectedly passed")
	}
	if _, err := SelectPlatform(trust, "linux/riscv64"); err == nil {
		t.Fatal("unsupported generator platform unexpectedly passed")
	}
}

func TestImageIdentityIncludesArmVariant(t *testing.T) {
	if err := validateImageIdentity("linux/amd64", imageIdentity{OS: "linux", Architecture: "amd64"}); err != nil {
		t.Fatal(err)
	}
	if err := validateImageIdentity("linux/arm64/v8", imageIdentity{OS: "linux", Architecture: "arm64", Variant: "v8"}); err != nil {
		t.Fatal(err)
	}
	if err := validateImageIdentity("linux/arm64/v8", imageIdentity{OS: "linux", Architecture: "arm64"}); err == nil {
		t.Fatal("arm64 image without v8 variant unexpectedly passed")
	}
}

func TestJDKVersionValidation(t *testing.T) {
	output := `openjdk version "17.0.21" 2025-10-21 LTS
OpenJDK Runtime Environment Temurin-17.0.21+9 (build 17.0.21+9-LTS)`
	if err := ValidateJDKVersion(output, "17.0.21+9"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJDKVersion(`openjdk version "17.0.20"`, "17.0.21+9"); err == nil {
		t.Fatal("unexpected JDK version passed")
	}
}

func TestGoVersionValidation(t *testing.T) {
	if err := ValidateGoVersion("go version go1.27.3 linux/amd64\n", "1.27.3", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGoVersion("go version go1.27.3 linux/arm64\n", "1.27.3", "linux/arm64/v8"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGoVersion("go version go1.27.4 linux/amd64\n", "1.27.3", "linux/amd64"); err == nil {
		t.Fatal("unexpected generated-code test Go version passed")
	}
}

func TestGoToolchainPolicyMatchesRepositoryLifecycle(t *testing.T) {
	root := testRepositoryRoot(t)
	trust, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(trust.GoToolchains) != 2 {
		t.Fatalf("Go toolchain count = %d, want root and current toolchains", len(trust.GoToolchains))
	}
	rootVersion := trust.GoToolchains[0]
	currentVersion := trust.GoToolchains[1]
	if trust.TestContainer.GoVersion != currentVersion {
		t.Fatalf("test container Go version = %q, want %q", trust.TestContainer.GoVersion, currentVersion)
	}
	checks := map[string][]string{
		"go.mod": {
			"\ngo " + rootVersion + "\n",
		},
		".github/workflows/go.yml": {
			"go-version: [" + strings.Join(trust.GoToolchains, ", ") + "]",
			"go-version: " + currentVersion,
		},
		"Makefile": {
			"GENERATED_SDK_GO_TOOLCHAIN := go" + currentVersion,
			"GOTOOLCHAIN=go" + rootVersion + " go version",
		},
		"tools/generated-sdk/generator/run-pinned.sh": {
			"GOTOOLCHAIN=go" + currentVersion,
		},
	}
	for path, expected := range checks {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range expected {
			if !strings.Contains(string(data), value) {
				t.Errorf("%s does not contain synchronized toolchain policy %q", path, value)
			}
		}
	}
}

func TestConfigMutationsFail(t *testing.T) {
	config := validConfig("trading")
	if err := ValidateConfig("trading", config); err != nil {
		t.Fatal(err)
	}
	config.AdditionalProperties["enumClassPrefix"] = false
	if err := ValidateConfig("trading", config); err == nil {
		t.Fatal("mutated config option unexpectedly passed")
	}
}

func TestConfigDigestMutationFails(t *testing.T) {
	root := t.TempDir()
	path := "config.json"
	if err := os.WriteFile(filepath.Join(root, path), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifact(root, Artifact{Path: path, SHA256: "bad"}, "config"); err == nil {
		t.Fatal("mutated config digest unexpectedly passed")
	}
}

func TestVerifyArtifactRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "artifact.json")); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifact(
		root,
		Artifact{Path: "artifact.json", SHA256: spec.Digest([]byte("{}\n"))},
		"artifact",
	); err == nil {
		t.Fatal("symlinked trusted artifact unexpectedly passed")
	}
}

func TestSnapshotInputsCopiesOnlyVerifiedBytes(t *testing.T) {
	root := t.TempDir()
	inputs := map[string][]byte{
		"cache/generator.jar":      []byte("jar"),
		"config/trading.json":      []byte("config"),
		"specs/trading.json":       []byte("specification"),
		"fixtures/overlays.json":   []byte("overlays"),
		"templates/model.mustache": []byte("template"),
	}
	for path, content := range inputs {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	trust := &TrustRoot{
		Generator: Generator{JAR: JAR{
			CachePath: "cache/generator.jar",
			SHA256:    spec.Digest(inputs["cache/generator.jar"]),
		}},
		Configs: map[string]Artifact{
			"trading": {Path: "config/trading.json", SHA256: spec.Digest(inputs["config/trading.json"])},
		},
		Specifications: map[string]Artifact{
			"trading": {Path: "specs/trading.json", SHA256: spec.Digest(inputs["specs/trading.json"])},
		},
		TemplateArtifacts: map[string]Artifact{
			"overlays": {Path: "fixtures/overlays.json", SHA256: spec.Digest(inputs["fixtures/overlays.json"])},
		},
		CustomTemplates: CustomTemplates{Entries: []CustomTemplate{{
			CustomPath:   "templates/model.mustache",
			CustomSHA256: spec.Digest(inputs["templates/model.mustache"]),
		}}},
	}
	snapshot := t.TempDir()
	if err := SnapshotInputs(root, snapshot, trust); err != nil {
		t.Fatal(err)
	}
	for path, want := range inputs {
		got, err := os.ReadFile(filepath.Join(snapshot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("snapshot %s = %q, want %q", path, got, want)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "config", "trading.json"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotInputs(root, t.TempDir(), trust); err == nil {
		t.Fatal("changed input unexpectedly passed snapshot verification")
	}
	got, err := os.ReadFile(filepath.Join(snapshot, "config", "trading.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "config" {
		t.Fatalf("existing snapshot changed with source: %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "trading.json"), inputs["config/trading.json"], 0o644); err != nil {
		t.Fatal(err)
	}
	proposed := []byte("proposed specification")
	specification := trust.Specifications["trading"]
	specification.SHA256 = spec.Digest(proposed)
	trust.Specifications["trading"] = specification
	overrideSnapshot := t.TempDir()
	if err := SnapshotInputsWithOverrides(root, overrideSnapshot, trust, map[string][]byte{
		specification.Path: proposed,
	}); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(overrideSnapshot, specification.Path))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(proposed) {
		t.Fatalf("proposed snapshot = %q, want %q", got, proposed)
	}
	if err := SnapshotInputsWithOverrides(root, t.TempDir(), trust, map[string][]byte{
		specification.Path:   proposed,
		"unknown/input.json": []byte("unknown"),
	}); err == nil {
		t.Fatal("unknown override unexpectedly passed")
	}
}

func TestProposeDigestsReportsBytesWithoutMutatingTrust(t *testing.T) {
	root := t.TempDir()
	trust := validTrust()
	trustBytes, err := spec.Canonical(trust)
	if err != nil {
		t.Fatal(err)
	}
	trustPath := filepath.Join(root, filepath.FromSlash(TrustRootPath))
	if err := os.MkdirAll(filepath.Dir(trustPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(trustPath, trustBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	var artifacts []Artifact
	artifacts = append(artifacts, trust.Wrapper)
	artifacts = append(artifacts, mapArtifacts(trust.Configs)...)
	artifacts = append(artifacts, mapArtifacts(trust.PolicyArtifacts)...)
	artifacts = append(artifacts, mapArtifacts(trust.Specifications)...)
	artifacts = append(artifacts, mapArtifacts(trust.TemplateArtifacts)...)
	for _, template := range trust.CustomTemplates.Entries {
		artifacts = append(artifacts, Artifact{Path: template.CustomPath})
	}
	for _, artifact := range artifacts {
		path := filepath.Join(root, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("review me\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var jar bytes.Buffer
	writer := zip.NewWriter(&jar)
	for _, template := range trust.StockTemplates {
		file, err := writer.Create(template.Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(template.Path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	jarPath := filepath.Join(root, filepath.FromSlash(trust.Generator.JAR.CachePath))
	if err := os.MkdirAll(filepath.Dir(jarPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jarPath, jar.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	proposal, err := ProposeDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	var report DigestReport
	if err := json.Unmarshal(proposal, &report); err != nil {
		t.Fatal(err)
	}
	if got := report.Artifacts[trust.Wrapper.Path]; got != spec.Digest([]byte("review me\n")) {
		t.Fatalf("wrapper proposal digest = %q", got)
	}
	for _, template := range trust.StockTemplates {
		if got := report.StockTemplates[template.Path]; got != spec.Digest([]byte(template.Path)) {
			t.Fatalf("stock template %s proposal digest = %q", template.Path, got)
		}
	}
}

func TestCorruptCacheFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generator.jar")
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyJAR(path, validTrust()); err == nil {
		t.Fatal("corrupt JAR cache unexpectedly passed")
	}
}

func TestVerifyTrackedWithoutJARCache(t *testing.T) {
	srcRoot := testRepositoryRoot(t)
	trust, err := Load(srcRoot)
	if err != nil {
		t.Fatal(err)
	}
	tempRoot := t.TempDir()
	copyFile := func(rel string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(tempRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(TrustRootPath)
	copyFile(trust.Wrapper.Path)
	for _, group := range []map[string]Artifact{
		trust.Configs,
		trust.PolicyArtifacts,
		trust.Specifications,
		trust.TemplateArtifacts,
	} {
		for _, artifact := range group {
			copyFile(artifact.Path)
		}
	}
	for _, entry := range trust.CustomTemplates.Entries {
		copyFile(entry.CustomPath)
	}
	if err := VerifyTracked(tempRoot); err != nil {
		t.Fatalf("VerifyTracked without JAR cache failed: %v", err)
	}
	if err := Verify(tempRoot); err == nil {
		t.Fatal("Verify without JAR cache unexpectedly passed")
	}
	wrapperPath := filepath.Join(tempRoot, filepath.FromSlash(trust.Wrapper.Path))
	if err := os.WriteFile(wrapperPath, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTracked(tempRoot); err == nil {
		t.Fatal("VerifyTracked with mutated tracked wrapper unexpectedly passed")
	}
}

func TestStrictTrustRootDecode(t *testing.T) {
	trust := validTrust()
	data, err := spec.Canonical(trust)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TrustRoot
	if err := spec.DecodeStrict(data, &decoded); err != nil {
		t.Fatal(err)
	}
	unknown := []byte(`{"configs":{},"container":{},"custom_templates":{},"environment":{},"future":true,"generator":{},"go_toolchains":[],"schema_version":1,"specifications":{},"stock_templates":[],"wrapper":{}}` + "\n")
	if err := spec.DecodeStrict(unknown, &decoded); err == nil {
		t.Fatal("unknown trust-root field unexpectedly passed")
	}
	if err := spec.DecodeStrict(append(data, []byte("{}")...), &decoded); err == nil {
		t.Fatal("trailing trust-root value unexpectedly passed")
	}
}

func testRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve generator trust test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func validTrust() *TrustRoot {
	return &TrustRoot{
		SchemaVersion: 1,
		Generator: Generator{
			Version: "7.27.0",
			JAR: JAR{
				CachePath: "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar",
				SHA256:    "db9c4b738727698748746144394cb7ee96512fe50166555baaeeca9311ee1dba",
				SourceURL: "https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/7.27.0/openapi-generator-cli-7.27.0.jar",
			},
		},
		Container: Container{
			ImageRepository: "docker.io/library/eclipse-temurin",
			JDKVersion:      "17.0.21+9",
			Platforms: []Platform{
				{Digest: "sha256:101376794254cc58a1aa952a35ade1709cbc0f5942d5da35e226009171dde46e", Platform: "linux/amd64"},
				{Digest: "sha256:80fbefa720be4552d66b64b8d6b77a5e8daf3d75effe2a544c2acc8d7a4237d5", Platform: "linux/arm64/v8"},
			},
		},
		TestContainer: GoContainer{
			GoVersion:       "1.27.3",
			ImageRepository: "docker.io/library/golang",
			Platforms: []Platform{
				{Digest: "sha256:025eefc1e8dbe63201734a72aaa0a55d7a077f2d8410432f8ae2f3e939f32871", Platform: "linux/amd64"},
				{Digest: "sha256:bc8d7053145982971b2c25138ba67b259d9a2df739a7a715d45ca6368c25c1e0", Platform: "linux/arm64/v8"},
			},
		},
		GoToolchains: []string{"1.26.10", "1.27.3"},
		Environment: Environment{
			AllowedVariables: map[string]string{
				"JAVA_HOME": "/opt/java/openjdk", "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8",
				"PATH": "/opt/java/openjdk/bin:/usr/bin:/bin", "TZ": "UTC",
			},
			ClearingPolicy: "env-i", Network: "disabled", PassThroughVariables: []string{},
		},
		StockTemplates: []Artifact{
			{Path: "go/model_anyof.mustache", SHA256: "29694d9c68575cbba5c8fd9af3de3633437a31836a4e17588a143951c8c15741"},
			{Path: "go/model_enum.mustache", SHA256: "c7a5069f3c2896887abf53add1d1a83717a96dec4d25f79d8154c4227aeffda7"},
			{Path: "go/model_oneof.mustache", SHA256: "98148bb5ed88123a26e1f0ec0c1f2b18744934732407858cd2359702b2c0ec5c"},
			{Path: "go/model_simple.mustache", SHA256: "80293db777ee6940cfc7f489e73d0c540990263a4e6d75a41aad5a711d7ff861"},
		},
		CustomTemplates: CustomTemplates{
			Status: "reviewed",
			Entries: []CustomTemplate{
				{
					Name: "model_enum.mustache", Status: "reviewed",
					StockPath: "go/model_enum.mustache", StockSHA256: "c7a5069f3c2896887abf53add1d1a83717a96dec4d25f79d8154c4227aeffda7",
					CustomPath:   "tools/generated-sdk/generator/templates/go/model_enum.mustache",
					CustomSHA256: "153e3a586982664544edb2849c8c7fd517a7cea5a635825f5219f755bdb2b2d3",
				},
				{
					Name: "model_anyof.mustache", Status: "reviewed",
					StockPath: "go/model_anyof.mustache", StockSHA256: "29694d9c68575cbba5c8fd9af3de3633437a31836a4e17588a143951c8c15741",
					CustomPath:   "tools/generated-sdk/generator/templates/go/model_anyof.mustache",
					CustomSHA256: "0e4d10580c8801b9de78c1a1904d9e27ad75041934416fd7d495aa88445f20b6",
				},
				{
					Name: "model_oneof.mustache", Status: "reviewed",
					StockPath: "go/model_oneof.mustache", StockSHA256: "98148bb5ed88123a26e1f0ec0c1f2b18744934732407858cd2359702b2c0ec5c",
					CustomPath:   "tools/generated-sdk/generator/templates/go/model_oneof.mustache",
					CustomSHA256: "1c0058a441cf6121b22e675206ec8ce2058394e6baa85b0248248c63d561a59f",
				},
				{
					Name: "model_simple.mustache", Status: "reviewed",
					StockPath: "go/model_simple.mustache", StockSHA256: "80293db777ee6940cfc7f489e73d0c540990263a4e6d75a41aad5a711d7ff861",
					CustomPath:   "tools/generated-sdk/generator/templates/go/model_simple.mustache",
					CustomSHA256: "e0ea8a1b093106f9bb8a2b283779523e38bc15674a20136f3c80e7eb227de70d",
				},
			},
		},
		Configs: map[string]Artifact{
			"trading": {
				Path:   "tools/generated-sdk/generator/config/trading.json",
				SHA256: "acad28d0a2de371dcc49c6eebfce8c0f3158f58dba7a868769b6898dfd0c42b0",
			},
			"market-data": {
				Path:   "tools/generated-sdk/generator/config/market-data.json",
				SHA256: "93d0e0ad7ae59b0dd4a864dcadde5607a7eb95d9947385bb0ad37c573c48335b",
			},
		},
		Specifications: map[string]Artifact{
			"trading": {
				Path:   "tools/generated-sdk/specs/trading.openapi.json",
				SHA256: "6114a39affa96fceb9a03bcb458f1ba721b8acd00c5a04fca61c2bd76deb1d59",
			},
			"market-data": {
				Path:   "tools/generated-sdk/specs/market-data.openapi.json",
				SHA256: "28aaaed7e89c1c7358edecb2fa334da3b40d72d169cdec36be8502fa9807a110",
			},
		},
		PolicyArtifacts: map[string]Artifact{
			"feature_allowlist": {
				Path:   "tools/generated-sdk/specs/feature-allowlist.json",
				SHA256: "6f4725c4848e5e20f3635fcd7244eb950178fe87f44b6be1b5d39cc77e7db56d",
			},
			"provenance": {
				Path:   "tools/generated-sdk/specs/provenance.json",
				SHA256: "4966932671989ac5660f12a88e404e4205a4965f85c92a659af382d5e668322f",
			},
		},
		TemplateArtifacts: map[string]Artifact{
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
			"overlays": {
				Path:   "tools/generated-sdk/generator/overlays.json",
				SHA256: "198a6b242f27a3fb58b3db89c3a3fddbf0a03ac40af0fdfbdf4b52e7fddd697c",
			},
			"stock_digests": {
				Path:   "tools/generated-sdk/generator/stock-template-digests.json",
				SHA256: "15bbf674cb0bfa3274fa2e6e5b7afee5d8594fd03975c08d14038b0aed5c9d22",
			},
			"template_deltas": {
				Path:   "tools/generated-sdk/generator/template-deltas.json",
				SHA256: "3c7fe4ab0faa1327c7cca0fd406dcb774bca571581432787f4673f6aea3f7434",
			},
		},
		Wrapper: Artifact{
			Path:   "tools/generated-sdk/generator/run-pinned.sh",
			SHA256: "70cbf141e09748aac15a5b7734d749acc535b8768753fe4af6e7dba760fb8980",
		},
	}
}

func validConfig(target string) *Config {
	packageName := "trading"
	inputSpec := "/workspace/tools/generated-sdk/specs/trading.openapi.json"
	outputDir := "/output/trading"
	if target == "market-data" {
		packageName = "marketdata"
		inputSpec = "/workspace/tools/generated-sdk/specs/market-data.openapi.json"
		outputDir = "/output/market-data"
	}
	return &Config{
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
}
