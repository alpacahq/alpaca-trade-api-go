package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/fixturedeps"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/generatortrust"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/gosandbox"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/templatepolicy"
)

type mutant struct {
	name        string
	testName    string
	wantFailure string
}

func main() {
	var platform string
	var repoRoot string
	flag.StringVar(&platform, "platform", "", "generator platform: linux/amd64 or linux/arm64/v8")
	flag.StringVar(&repoRoot, "repo-root", "", "repository root")
	flag.Parse()
	if repoRoot == "" {
		repoRoot = findRepoRoot()
	}
	if err := generatortrust.ValidateHostArchitecture(runtimeGOARCH); err != nil {
		fmt.Fprintln(os.Stderr, "templatecheck:", err)
		os.Exit(1)
	}
	if platform == "" {
		var err error
		platform, err = generatortrust.DefaultPlatform(runtimeGOARCH)
		if err != nil {
			fmt.Fprintln(os.Stderr, "templatecheck:", err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, repoRoot, platform); err != nil {
		fmt.Fprintln(os.Stderr, "templatecheck:", err)
		os.Exit(1)
	}
}

var runtimeGOARCH = runtime.GOARCH

func run(ctx context.Context, repoRoot, requestedPlatform string) (resultErr error) {
	trust, err := generatortrust.Load(repoRoot)
	if err != nil {
		return err
	}
	platform, err := generatortrust.SelectPlatform(trust, requestedPlatform)
	if err != nil {
		return err
	}
	if err := generatortrust.VerifyImageJDK(ctx, trust, platform); err != nil {
		return err
	}
	image := generatortrust.ImageReference(trust, platform)
	testPlatform, err := generatortrust.SelectTestPlatform(trust, requestedPlatform)
	if err != nil {
		return err
	}
	if err := generatortrust.VerifyTestImageGo(ctx, trust, testPlatform); err != nil {
		return err
	}
	testImage := generatortrust.TestImageReference(trust, testPlatform)
	if err := templatepolicy.Verify(repoRoot); err != nil {
		return err
	}
	snapshot, err := os.MkdirTemp("", "generated-sdk-template-inputs-*")
	if err != nil {
		return err
	}
	defer removeTemporaryDirectory(snapshot, &resultErr)
	if err := generatortrust.SnapshotInputs(repoRoot, snapshot, trust); err != nil {
		return err
	}
	repoRoot = snapshot
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errorsForMissingTool("docker")
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		return errorsForMissingTool("go")
	}
	if output, err := generateAndTest(
		ctx, repoRoot, docker, goCommand, platform.Platform, image, testImage, "", "",
	); err != nil {
		return fmt.Errorf("custom generated fixture failed: %w\n%s", err, output)
	}
	fmt.Println("custom generated fixture: pass")
	mutants := []mutant{
		{name: "model_enum.mustache", testName: "TestEnumPolicy", wantFailure: "unknown enum value was rejected"},
		{name: "model_anyof.mustache", testName: "TestAnyOfPolicy", wantFailure: "RawJSON"},
		{name: "model_oneof.mustache", testName: "TestOneOfMultipleMatch", wantFailure: "multiple oneOf match was rejected"},
		{name: "model_simple.mustache", testName: "TestSimpleModelPolicy", wantFailure: "rejected missing required response fields"},
	}
	for _, candidate := range mutants {
		output, testErr := generateAndTest(
			ctx, repoRoot, docker, goCommand, platform.Platform, image, testImage, candidate.name, candidate.testName,
		)
		if testErr == nil {
			return fmt.Errorf("stock mutant %s unexpectedly passed %s", candidate.name, candidate.testName)
		}
		if !strings.Contains(output, candidate.wantFailure) {
			return fmt.Errorf("stock mutant %s failed for an unexpected reason: %w\n%s", candidate.name, testErr, output)
		}
		fmt.Printf("stock mutant %s: rejected by %s\n", candidate.name, candidate.testName)
	}
	if output, err := verifyPinnedOneOfAmbiguities(
		ctx, repoRoot, docker, goCommand, platform.Platform, image, testImage,
	); err != nil {
		return fmt.Errorf("stock oneOf template ambiguity check failed: %w\n%s", err, output)
	}
	fmt.Println("stock mutant model_oneof.mustache: reproduced both pinned oneOf ambiguities and was rejected")
	fmt.Println("generated SDK template check passed")
	return nil
}

func verifyPinnedOneOfAmbiguities(
	ctx context.Context,
	repoRoot, docker, goCommand, platform, image, testImage string,
) (output string, err error) {
	customDir := filepath.Join(repoRoot, filepath.FromSlash(templatepolicy.CustomTemplateDir))
	templateDir, err := os.MkdirTemp("", "generated-sdk-production-oneof-template-*")
	if err != nil {
		return "", err
	}
	defer removeTemporaryDirectory(templateDir, &err)
	entries, err := os.ReadDir(customDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		content, readErr := os.ReadFile(filepath.Join(customDir, entry.Name()))
		if readErr != nil {
			return "", readErr
		}
		if entry.Name() == "model_oneof.mustache" {
			content, readErr = templatepolicy.LoadStockTemplate(repoRoot, entry.Name())
			if readErr != nil {
				return "", readErr
			}
		}
		// #nosec G703 -- entry comes from the verified custom-template directory and templateDir is a fresh temporary directory.
		if writeErr := os.WriteFile(filepath.Join(templateDir, entry.Name()), content, 0o644); writeErr != nil {
			return "", writeErr
		}
	}
	outputDir, err := os.MkdirTemp("", "generated-sdk-production-oneof-generated-*")
	if err != nil {
		return "", err
	}
	defer removeTemporaryDirectory(outputDir, &err)
	jar := filepath.Join(repoRoot, "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar")
	specification := filepath.Join(repoRoot, "tools/generated-sdk/specs/trading.openapi.json")
	generationOutput, err := runGenerator(ctx, repoRoot, docker, jar, specification, outputDir, templateDir, platform, image,
		"--additional-properties=packageName=trading,enumClassPrefix=true,enumUnknownDefaultCase=false,disallowAdditionalPropertiesIfNotPresent=false,generateMarshalJSON=true,generateUnmarshalJSON=true,hideGenerationTimestamp=true,useDefaultValuesForRequiredVars=false,useOneOfDiscriminatorLookup=true,withGoMod=true",
		"--type-mappings=string+decimal=string",
		"--global-property=apiDocs=false,apiTests=false,modelDocs=false,modelTests=false",
	)
	if err != nil {
		return generationOutput, err
	}
	if err := safetree.Validate(outputDir); err != nil {
		return generationOutput, err
	}
	if err := fixturedeps.VerifyGenerated(repoRoot, outputDir); err != nil {
		return generationOutput, err
	}
	testSource := `package trading

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStockOneOfAmbiguities(t *testing.T) {
	position := []byte(` + "`" + `{"asset_class":"us_equity","asset_id":"00000000-0000-4000-8000-000000000001","canceled_at":null,"client_order_id":"00000000-0000-4000-8000-000000000002","created_at":"2020-01-02T03:04:05Z","expired_at":null,"extended_hours":false,"failed_at":null,"filled_at":null,"filled_avg_price":null,"filled_qty":"0","hwm":null,"id":"00000000-0000-4000-8000-000000000003","legs":null,"limit_price":null,"notional":null,"order_class":"","order_type":"market","qty":"2","replaced_at":null,"replaced_by":null,"replaces":null,"side":"sell","status":"accepted","stop_price":null,"submitted_at":"2020-01-02T03:04:05Z","symbol":"AAPL","time_in_force":"day","trail_percent":null,"trail_price":null,"type":"market","updated_at":"2020-01-02T03:04:05Z"}` + "`" + `)
	var positionValue PositionClosedResponseBody
	if err := json.Unmarshal(position, &positionValue); err == nil || !strings.Contains(err.Error(), "data matches more than one schema in oneOf(PositionClosedResponseBody)") {
		t.Fatalf("stock template did not reproduce the position response ambiguity: %v", err)
	}
	activity := []byte(` + "`" + `{"activity_type":"FILL","cum_qty":"2","id":"synthetic-fill-1","leaves_qty":"0","order_id":"00000000-0000-4000-8000-000000000004","order_status":"filled","price":"100.00","qty":"2","side":"buy","symbol":"AAPL","transaction_time":"2020-01-02T03:04:05Z","type":"fill"}` + "`" + `)
	var activityValue GetAccountActivitiesByActivityType200ResponseInner
	if err := json.Unmarshal(activity, &activityValue); err == nil || !strings.Contains(err.Error(), "data matches more than one schema in oneOf(GetAccountActivitiesByActivityType200ResponseInner)") {
		t.Fatalf("stock template did not reproduce the activity response ambiguity: %v", err)
	}
}
`
	if err := safetree.WriteNew(filepath.Join(outputDir, "oneof_stock_test.go"), []byte(testSource), 0o600); err != nil {
		return generationOutput, err
	}
	if err := gosandbox.Vendor(ctx, goCommand, outputDir); err != nil {
		return generationOutput, err
	}
	testOutput, err := gosandbox.RunTests(
		ctx, docker, platform, testImage, outputDir,
		"test", "-count=1", "-run", "^TestStockOneOfAmbiguities$", "./...",
	)
	if err != nil {
		return testOutput, err
	}
	return testOutput, nil
}

func generateAndTest(
	ctx context.Context,
	repoRoot, docker, goCommand, platform, image, testImage, stockMutant, testName string,
) (output string, err error) {
	templateDir := filepath.Join(repoRoot, filepath.FromSlash(templatepolicy.CustomTemplateDir))
	if stockMutant != "" {
		mutantDir, err := os.MkdirTemp("", "generated-sdk-template-mutant-*")
		if err != nil {
			return "", err
		}
		defer removeTemporaryDirectory(mutantDir, &err)
		entries, err := os.ReadDir(templateDir)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			content, err := os.ReadFile(filepath.Join(templateDir, entry.Name()))
			if err != nil {
				return "", err
			}
			if entry.Name() == stockMutant {
				content, err = templatepolicy.LoadStockTemplate(repoRoot, stockMutant)
				if err != nil {
					return "", err
				}
			}
			// #nosec G703 -- entry comes from the verified custom-template directory and mutantDir is a fresh temporary directory.
			if err := os.WriteFile(filepath.Join(mutantDir, entry.Name()), content, 0o644); err != nil {
				return "", err
			}
		}
		templateDir = mutantDir
	}
	outputDir, err := os.MkdirTemp("", "generated-sdk-template-generated-*")
	if err != nil {
		return "", err
	}
	defer removeTemporaryDirectory(outputDir, &err)
	jar := filepath.Join(repoRoot, "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar")
	fixture := filepath.Join(repoRoot, "tools/generated-sdk/generator/fixtures/template-policy.openapi.json")
	generationOutput, err := runGenerator(ctx, repoRoot, docker, jar, fixture, outputDir, templateDir, platform, image,
		"--additional-properties=packageName=templatefixture,enumClassPrefix=true,enumUnknownDefaultCase=false,disallowAdditionalPropertiesIfNotPresent=false,generateMarshalJSON=true,generateUnmarshalJSON=true,hideGenerationTimestamp=true,useOneOfDiscriminatorLookup=true,withGoMod=true",
		"--global-property=apiDocs=false,apiTests=false,modelDocs=false,modelTests=false",
	)
	if err != nil {
		return generationOutput, fmt.Errorf("generate fixture: %w", err)
	}
	if err := safetree.Validate(outputDir); err != nil {
		return generationOutput, err
	}
	if err := fixturedeps.VerifyGenerated(repoRoot, outputDir); err != nil {
		return generationOutput, err
	}
	testSource, err := os.ReadFile(filepath.Join(repoRoot, "tools/generated-sdk/generator/fixtures/template_policy_test.go.tmpl"))
	if err != nil {
		return generationOutput, err
	}
	if err := safetree.WriteNew(filepath.Join(outputDir, "template_policy_test.go"), testSource, 0o600); err != nil {
		return generationOutput, err
	}
	if err := gosandbox.Vendor(ctx, goCommand, outputDir); err != nil {
		return generationOutput, err
	}
	args := []string{"test", "-count=1", "./..."}
	if testName != "" {
		args = []string{"test", "-count=1", "-run", "^" + testName + "$", "./..."}
	}
	testOutput, testErr := gosandbox.RunTests(ctx, docker, platform, testImage, outputDir, args...)
	return testOutput, testErr
}

func removeTemporaryDirectory(path string, result *error) {
	if err := os.RemoveAll(path); err != nil {
		*result = errors.Join(*result, fmt.Errorf("remove temporary directory %s: %w", path, err))
	}
}

func runGenerator(
	ctx context.Context,
	repoRoot, docker, jar, specification, outputDir, templateDir, platform, image string,
	generatorArguments ...string,
) (string, error) {
	jarRelative, err := filepath.Rel(repoRoot, jar)
	if err != nil || strings.HasPrefix(jarRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generator JAR is outside the repository")
	}
	specRelative, err := filepath.Rel(repoRoot, specification)
	if err != nil || strings.HasPrefix(specRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generator specification is outside the repository")
	}
	arguments := []string{
		"run", "--rm", "--pull=never", "--network=none", "--read-only",
		"--platform=" + platform,
		fmt.Sprintf("--user=%d:%d", os.Getuid(), os.Getgid()),
		"--cap-drop=ALL", "--security-opt=no-new-privileges=true", "--pids-limit=256",
		"--memory=4g", "--cpus=2",
		"--workdir=/workspace",
		"--mount=type=bind,src=" + repoRoot + ",dst=/workspace,readonly",
		"--mount=type=bind,src=" + templateDir + ",dst=/generator-templates,readonly",
		"--mount=type=bind,src=" + outputDir + ",dst=/output",
		"--tmpfs=/tmp:rw,noexec,nosuid,size=512m",
		"--entrypoint=/usr/bin/env",
		image,
		"-i",
	}
	arguments = append(arguments, focusedGenerationEnvironment()...)
	arguments = append(arguments,
		"/opt/java/openjdk/bin/java",
		"-jar", "/workspace/"+filepath.ToSlash(jarRelative),
		"generate",
		"-g", "go",
		"-i", "/workspace/"+filepath.ToSlash(specRelative),
		"-o", "/output",
		"-t", "/generator-templates",
	)
	arguments = append(arguments, generatorArguments...)
	command := exec.CommandContext(ctx, docker, arguments...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func focusedGenerationEnvironment() []string {
	return []string{
		"JAVA_HOME=/opt/java/openjdk",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PATH=/opt/java/openjdk/bin:/usr/bin:/bin",
		"TZ=UTC",
	}
}

func errorsForMissingTool(name string) error {
	return fmt.Errorf("%s is required for focused generated fixture tests", name)
}

func findRepoRoot() string {
	current, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "tools", "generated-sdk", "generator", "trust-root.json")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "."
		}
		current = parent
	}
}
