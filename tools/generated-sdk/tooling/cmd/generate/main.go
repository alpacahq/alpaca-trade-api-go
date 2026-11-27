package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/generatortrust"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/gosandbox"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/overlay"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
)

type preflightCommand struct {
	name string
	args []string
}

type candidateResult struct {
	FileCount   int      `json:"file_count"`
	FinalSHA256 string   `json:"final_sha256"`
	Platform    string   `json:"platform"`
	RawSHA256   string   `json:"raw_sha256"`
	Toolchains  []string `json:"toolchains"`
}

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

func runMain() error {
	rootFlag := flag.String("repo-root", ".", "repository root")
	platformFlag := flag.String("platform", "", "generator platform: linux/amd64 or linux/arm64/v8")
	bundleFlag := flag.String("input-bundle", "", "external finalized proposal bundle")
	outputFlag := flag.String("output-dir", "", "external directory for the verified final candidate")
	reviewSpecChangeFlag := flag.Bool("review-spec-change", false, "generate a proposed specification for review without pinned compatibility assertions")
	jsonFlag := flag.Bool("json", false, "write the candidate result as JSON")
	flag.Parse()

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	if err := generatortrust.ValidateHostArchitecture(runtime.GOARCH); err != nil {
		return err
	}
	platform := *platformFlag
	if platform == "" {
		platform, err = generatortrust.DefaultPlatform(runtime.GOARCH)
		if err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var result candidateResult
	if *bundleFlag == "" {
		if *reviewSpecChangeFlag {
			return errors.New("-review-spec-change requires -input-bundle")
		}
		result, err = runCandidateTo(ctx, root, platform, *outputFlag)
	} else {
		result, err = runCandidateFromBundle(
			ctx, root, platform, *bundleFlag, *outputFlag, !*reviewSpecChangeFlag,
		)
	}
	if err != nil {
		return err
	}
	if *jsonFlag {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	fmt.Printf(
		"generated SDK candidate is deterministic and compiles: platform=%s raw_sha256=%s final_sha256=%s files=%d toolchains=%s\n",
		result.Platform,
		result.RawSHA256,
		result.FinalSHA256,
		result.FileCount,
		strings.Join(result.Toolchains, ","),
	)
	return nil
}

func runCandidateTo(ctx context.Context, root, requestedPlatform, outputDir string) (candidateResult, error) {
	return runCandidateWithOptions(ctx, root, requestedPlatform, outputDir, nil, nil, true, runGenerationPreflights)
}

func runCandidateWithPreflight(
	ctx context.Context,
	root, requestedPlatform string,
	preflight func(context.Context, string, string) error,
) (candidateResult, error) {
	return runCandidateWithOptions(ctx, root, requestedPlatform, "", nil, nil, true, preflight)
}

func runCandidateWithOptions(
	ctx context.Context,
	root, requestedPlatform, outputDir string,
	proposedTrust *generatortrust.TrustRoot,
	overrides map[string][]byte,
	enforcePinnedCompatibility bool,
	preflight func(context.Context, string, string) error,
) (candidateResult, error) {
	if outputDir != "" {
		if err := requireOutsideRepository(root, outputDir); err != nil {
			return candidateResult{}, fmt.Errorf("candidate output: %w", err)
		}
	}
	if err := preflight(ctx, root, requestedPlatform); err != nil {
		return candidateResult{}, err
	}
	trust := proposedTrust
	if trust == nil {
		var err error
		trust, err = generatortrust.Load(root)
		if err != nil {
			return candidateResult{}, err
		}
	}
	platform, err := generatortrust.SelectPlatform(trust, requestedPlatform)
	if err != nil {
		return candidateResult{}, err
	}
	if err := generatortrust.VerifyImageJDK(ctx, trust, platform); err != nil {
		return candidateResult{}, err
	}
	testPlatform, err := generatortrust.SelectTestPlatform(trust, requestedPlatform)
	if err != nil {
		return candidateResult{}, err
	}
	if err := generatortrust.VerifyTestImageGo(ctx, trust, testPlatform); err != nil {
		return candidateResult{}, err
	}
	return withTemporaryWorkspace("generated-sdk-candidate-", func(workspace string) (candidateResult, error) {
		snapshot := filepath.Join(workspace, "verified-inputs")
		if err := generatortrust.SnapshotInputsWithOverrides(root, snapshot, trust, overrides); err != nil {
			return candidateResult{}, err
		}
		return prepareCandidateTo(
			ctx, snapshot, workspace, outputDir, trust, platform, testPlatform,
			enforcePinnedCompatibility,
		)
	})
}

func runCandidateFromBundle(
	ctx context.Context,
	root, requestedPlatform, bundleRoot, outputDir string,
	enforcePinnedCompatibility bool,
) (candidateResult, error) {
	if err := requireOutsideRepository(root, bundleRoot); err != nil {
		return candidateResult{}, fmt.Errorf("proposal bundle: %w", err)
	}
	overrides, trust, err := loadBundleOverrides(bundleRoot)
	if err != nil {
		return candidateResult{}, err
	}
	return runCandidateWithOptions(
		ctx, root, requestedPlatform, outputDir, trust, overrides,
		enforcePinnedCompatibility, runProposalPreflights,
	)
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
	path, err := resolvePath(candidate)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(repository, path)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path must be outside the repository")
	}
	return nil
}

func resolvePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	var suffix []string
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return resolved, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("path has no existing ancestor")
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func loadBundleOverrides(bundleRoot string) (map[string][]byte, *generatortrust.TrustRoot, error) {
	root, err := filepath.Abs(bundleRoot)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, errors.New("proposal bundle is not a directory")
	}
	trustData, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(generatortrust.TrustRootPath)))
	if err != nil {
		return nil, nil, err
	}
	trust, err := generatortrust.Decode(trustData)
	if err != nil {
		return nil, nil, err
	}
	overrides := map[string][]byte{}
	for _, artifact := range []generatortrust.Artifact{
		trust.Specifications["trading"],
		trust.Specifications["market-data"],
		trust.TemplateArtifacts["overlays"],
	} {
		data, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return nil, nil, err
		}
		if digestBytes(data) != artifact.SHA256 {
			return nil, nil, fmt.Errorf("proposal artifact %s digest differs from its trust lock", artifact.Path)
		}
		overrides[artifact.Path] = data
	}
	for _, artifact := range []generatortrust.Artifact{
		trust.PolicyArtifacts["feature_allowlist"],
		trust.PolicyArtifacts["provenance"],
	} {
		data, err := safetree.ReadRegular(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return nil, nil, err
		}
		if digestBytes(data) != artifact.SHA256 {
			return nil, nil, fmt.Errorf("proposal artifact %s digest differs from its trust lock", artifact.Path)
		}
	}
	return overrides, trust, nil
}

func withTemporaryWorkspace(
	prefix string,
	run func(string) (candidateResult, error),
) (result candidateResult, err error) {
	workspace, err := os.MkdirTemp("", prefix)
	if err != nil {
		return candidateResult{}, err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(workspace); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary workspace: %w", cleanupErr))
		}
	}()
	return run(workspace)
}

func generationPreflightCommands(root, platform string) []preflightCommand {
	tooling := filepath.Join(root, "tools/generated-sdk/tooling")
	return []preflightCommand{
		{
			name: "generator trust",
			args: []string{"-C", tooling, "run", "./cmd/generatortrust", "-mode", "check", "-repo-root", root},
		},
		{
			name: "pinned specification policy",
			args: []string{"-C", tooling, "run", "./cmd/speccheck", "-mode", "pinned", "-repo-root", root},
		},
		{
			name: "template policy",
			args: []string{"-C", tooling, "run", "./cmd/templatecheck", "-platform", platform, "-repo-root", root},
		},
	}
}

func runGenerationPreflights(ctx context.Context, root, platform string) error {
	return runPreflightCommands(ctx, generationPreflightCommands(root, platform))
}

func runProposalPreflights(ctx context.Context, root, platform string) error {
	commands := generationPreflightCommands(root, platform)
	return runPreflightCommands(ctx, []preflightCommand{commands[0], commands[2]})
}

func runPreflightCommands(ctx context.Context, commands []preflightCommand) error {
	goCommand := filepath.Join(runtime.GOROOT(), "bin", "go")
	if info, err := os.Stat(goCommand); err != nil || !info.Mode().IsRegular() {
		return errors.New("go is required for generated SDK preflights")
	}
	environment := replaceEnvironment(os.Environ(), map[string]string{
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
		"GOFLAGS":     "-mod=readonly",
		"GOPROXY":     "off",
		"GOSUMDB":     "off",
	})
	for _, preflight := range commands {
		command := exec.CommandContext(ctx, goCommand, preflight.args...)
		command.Env = environment
		command.Stdout = os.Stderr
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("%s preflight: %w", preflight.name, err)
		}
	}
	return nil
}

func prepareCandidateTo(
	ctx context.Context,
	root, workspace, outputDir string,
	trust *generatortrust.TrustRoot,
	platform generatortrust.Platform,
	testPlatform generatortrust.Platform,
	enforcePinnedCompatibility bool,
) (candidateResult, error) {
	input := filepath.Join(workspace, "input")
	if err := os.MkdirAll(input, 0o755); err != nil {
		return candidateResult{}, err
	}
	rules, err := overlay.Load(filepath.Join(root, "tools/generated-sdk/generator/overlays.json"))
	if err != nil {
		return candidateResult{}, err
	}
	if err := prepareInputs(root, input, rules); err != nil {
		return candidateResult{}, err
	}

	rawRoots := []string{filepath.Join(workspace, "raw-1"), filepath.Join(workspace, "raw-2")}
	finalRoots := []string{filepath.Join(workspace, "final-1"), filepath.Join(workspace, "final-2")}
	for index := range rawRoots {
		if err := generateRawTree(ctx, root, input, rawRoots[index], trust, platform); err != nil {
			return candidateResult{}, err
		}
		if err := buildFinalTree(rawRoots[index], finalRoots[index]); err != nil {
			return candidateResult{}, err
		}
	}
	firstRaw, err := treeDigest(rawRoots[0])
	if err != nil {
		return candidateResult{}, err
	}
	secondRaw, err := treeDigest(rawRoots[1])
	if err != nil {
		return candidateResult{}, err
	}
	if firstRaw != secondRaw {
		return candidateResult{}, errors.New("raw generation is not deterministic")
	}
	firstFinal, err := treeDigest(finalRoots[0])
	if err != nil {
		return candidateResult{}, err
	}
	secondFinal, err := treeDigest(finalRoots[1])
	if err != nil {
		return candidateResult{}, err
	}
	if firstFinal != secondFinal {
		return candidateResult{}, errors.New("post-processed candidate generation is not deterministic")
	}
	if enforcePinnedCompatibility {
		if err := validateCandidatePatterns(finalRoots[0]); err != nil {
			return candidateResult{}, err
		}
	}
	if err := compileCandidate(ctx, root, finalRoots[0], trust, testPlatform); err != nil {
		return candidateResult{}, err
	}
	files, err := fileDigests(finalRoots[0])
	if err != nil {
		return candidateResult{}, err
	}
	if outputDir != "" {
		if err := exportCandidate(finalRoots[0], outputDir); err != nil {
			return candidateResult{}, err
		}
	}
	return candidateResult{
		FileCount:   len(files),
		FinalSHA256: firstFinal,
		Platform:    platform.Platform,
		RawSHA256:   firstRaw,
		Toolchains:  append([]string(nil), trust.GoToolchains...),
	}, nil
}

func exportCandidate(source, destination string) error {
	if err := safetree.Validate(source); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("candidate output directory already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	exportErr := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			if relative == "." {
				return nil
			}
			return os.Mkdir(target, 0o755)
		}
		data, err := safetree.ReadRegular(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o444)
	})
	if exportErr != nil {
		_ = os.RemoveAll(destination)
		return exportErr
	}
	return nil
}

func prepareInputs(root, input string, rules overlay.Manifest) error {
	for _, target := range []string{"trading", "market-data"} {
		specPath := filepath.Join(root, "tools/generated-sdk/specs", target+".openapi.json")
		specification, err := loadObject(specPath)
		if err != nil {
			return err
		}
		for _, rule := range rules.Operations {
			if rule.Target == target {
				if err := overlay.Apply(specification, rule); err != nil {
					return err
				}
			}
		}
		if err := writeJSON(filepath.Join(input, target+".openapi.json"), specification); err != nil {
			return err
		}
		configPath := filepath.Join(root, "tools/generated-sdk/generator/config", target+".json")
		config, err := loadObject(configPath)
		if err != nil {
			return err
		}
		config["inputSpec"] = "/generator-input/" + target + ".openapi.json"
		config["outputDir"] = "/output"
		config["templateDir"] = "/workspace/tools/generated-sdk/generator/templates/go"
		if err := writeJSON(filepath.Join(input, target+"-config.json"), config); err != nil {
			return err
		}
	}
	return nil
}

func generateRawTree(
	ctx context.Context,
	root, input, rawRoot string,
	trust *generatortrust.TrustRoot,
	platform generatortrust.Platform,
) error {
	for _, target := range []string{"trading", "market-data"} {
		output := filepath.Join(rawRoot, target)
		if err := os.MkdirAll(output, 0o755); err != nil {
			return err
		}
		if err := generate(ctx, root, input, output, target, trust, platform); err != nil {
			return err
		}
	}
	return nil
}

func generate(
	ctx context.Context,
	root, input, output, target string,
	trust *generatortrust.TrustRoot,
	platform generatortrust.Platform,
) error {
	// #nosec G204 -- The image, platform, environment, and mounts are constrained by verified trust and safe-tree policy.
	command := exec.CommandContext(
		ctx,
		"docker", "run", "--rm", "--pull=never", "--network=none", "--read-only",
		"--platform="+platform.Platform, fmt.Sprintf("--user=%d:%d", os.Getuid(), os.Getgid()),
		"--cap-drop=ALL", "--security-opt=no-new-privileges=true", "--pids-limit=256",
		"--memory=4g", "--cpus=2",
		"--workdir=/workspace",
		"--mount=type=bind,src="+root+",dst=/workspace,readonly",
		"--mount=type=bind,src="+input+",dst=/generator-input,readonly",
		"--mount=type=bind,src="+output+",dst=/output",
		"--tmpfs=/tmp:rw,noexec,nosuid,size=512m",
		"--entrypoint=/usr/bin/env", generatortrust.ImageReference(trust, platform),
		"-i", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TZ=UTC",
		"JAVA_HOME=/opt/java/openjdk", "PATH=/opt/java/openjdk/bin:/usr/bin:/bin",
		"/opt/java/openjdk/bin/java",
		"-jar", "/workspace/"+trust.Generator.JAR.CachePath,
		"generate", "--config", "/generator-input/"+target+"-config.json",
	)
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generate %s: %w: %s", target, err, strings.TrimSpace(string(outputBytes)))
	}
	return nil
}

func buildFinalTree(rawRoot, finalRoot string) error {
	for _, target := range []struct {
		output      string
		packageName string
	}{
		{output: "trading", packageName: "trading"},
		{output: "market-data", packageName: "marketdata"},
	} {
		if err := copyGoFiles(
			filepath.Join(rawRoot, target.output),
			filepath.Join(finalRoot, target.packageName),
		); err != nil {
			return err
		}
	}
	if err := applyGeneratedSourcePatches(finalRoot); err != nil {
		return err
	}
	return format(finalRoot)
}

func copyGoFiles(source, destination string) error {
	if err := safetree.Validate(source); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := safetree.ReadRegular(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func applyGeneratedSourcePatches(root string) error {
	floatBefore := "value = strconv.FormatFloat(v.Float(), 'g', -1, 32)"
	floatAfter := "value = strconv.FormatFloat(v.Float(), 'g', -1, v.Type().Bits())"
	requestBefore := `	if c.cfg.Debug {
		dump, err := httputil.DumpRequestOut(request, true)
		if err != nil {
			return nil, err
		}
		log.Printf("\n%s\n", string(dump))
	}`
	requestAfter := `	if c.cfg.Debug {
		sanitizedRequest := request.Clone(request.Context())
		sanitizedRequest.Header = request.Header.Clone()
		for _, header := range []string{"APCA-API-KEY-ID", "APCA-API-SECRET-KEY", "Authorization"} {
			if sanitizedRequest.Header.Get(header) != "" {
				sanitizedRequest.Header.Set(header, "[REDACTED]")
			}
		}
		dump, err := httputil.DumpRequestOut(sanitizedRequest, false)
		if err != nil {
			return nil, err
		}
		log.Printf("\n%s\n", string(dump))
	}`
	responseBefore := `	if c.cfg.Debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return resp, err
		}
		log.Printf("\n%s\n", string(dump))
	}`
	responseAfter := `	if c.cfg.Debug {
		dump, err := httputil.DumpResponse(resp, false)
		if err != nil {
			return resp, err
		}
		log.Printf("\n%s\n", string(dump))
	}`
	for _, packageName := range []string{"trading", "marketdata"} {
		path := filepath.Join(root, packageName, "client.go")
		if err := replaceSourceExact(path, floatBefore, floatAfter, 1); err != nil {
			return err
		}
		if err := replaceSourceExact(path, requestBefore, requestAfter, 1); err != nil {
			return err
		}
		if err := replaceSourceExact(path, responseBefore, responseAfter, 1); err != nil {
			return err
		}
	}
	activityPath := filepath.Join(root, "trading", "model_get_account_activities_by_activity_type_200_response_inner.go")
	activityBefore := "\treturn fmt.Errorf(\"data failed to match schemas in oneOf(GetAccountActivitiesByActivityType200ResponseInner)\")"
	activityAfter := `	// Treat every unknown non-FILL activity as NonTradeActivities so future
	// activity codes remain forward compatible without weakening FILL decoding.
	if jsonDict["activity_type"] != "FILL" {
		err = json.Unmarshal(data, &dst.NonTradeActivities)
		if err == nil && GetAccountActivitiesByActivityType200ResponseInnerBranchMatchesRequiredFields(data, dst.NonTradeActivities) {
			dst.raw = append(dst.raw[:0], data...)
			dst.matchedSchemas = append(dst.matchedSchemas, "NonTradeActivities")
			return nil
		}
		dst.NonTradeActivities = nil
		if err != nil {
			return fmt.Errorf("failed to unmarshal unknown non-FILL activity as NonTradeActivities: %w", err)
		}
		return fmt.Errorf("failed to unmarshal unknown non-FILL activity as NonTradeActivities: required fields do not match")
	}
	return fmt.Errorf("data failed to match schemas in oneOf(GetAccountActivitiesByActivityType200ResponseInner)")`
	return replaceSourceExact(activityPath, activityBefore, activityAfter, 1)
}

func replaceSourceExact(path, before, after string, want int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if got := strings.Count(string(data), before); got != want {
		return fmt.Errorf("%s source patch matched %d times, want %d", path, got, want)
	}
	patched := strings.ReplaceAll(string(data), before, after)
	// #nosec G703 -- Callers pass fixed generated filenames rooted in a validated temporary candidate tree.
	return os.WriteFile(path, []byte(patched), 0o644)
}

func format(root string) error {
	if err := safetree.Validate(root); err != nil {
		return err
	}
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return walkErr
	}); err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("generated source tree contains no Go files")
	}
	slices.Sort(files)
	// #nosec G204 -- The executable is the active Go toolchain's gofmt and every argument passed safe-tree traversal.
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "gofmt"), append([]string{"-w"}, files...)...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("gofmt: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func validateCandidatePatterns(root string) error {
	patterns := []struct {
		path    string
		pattern string
	}{
		{"marketdata/model_stock_trade.go", `(?m)^\s*I\s+int64\s+` + "`json:\"i\"`"},
		{"trading/model_watchlist.go", `(?m)^\s*CreatedAt\s+string\s+` + "`json:\"created_at\"`"},
		{"trading/model_watchlist.go", `(?m)^\s*UpdatedAt\s+string\s+` + "`json:\"updated_at\"`"},
		{"marketdata/api_stock.go", `\*map\[string\]StockSnapshot,\s+\*http\.Response`},
		{"trading/api_clock.go", `LegacyClockExecute\(r ApiLegacyClockRequest\)`},
		{"marketdata/client.go", `strconv\.FormatFloat\(v\.Float\(\), 'g', -1, v\.Type\(\)\.Bits\(\)\)`},
		{"trading/client.go", `sanitizedRequest\.Header\.Set\(header, "\[REDACTED\]"\)`},
		{"trading/model_get_account_activities_by_activity_type_200_response_inner.go", `unknown non-FILL activity as NonTradeActivities`},
	}
	for _, check := range patterns {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(check.path)))
		if err != nil {
			return err
		}
		if !regexp.MustCompile(check.pattern).Match(content) {
			return fmt.Errorf("generated pattern %q is absent from %s", check.pattern, check.path)
		}
	}
	return nil
}

func compileCandidate(
	ctx context.Context,
	root, candidate string,
	trust *generatortrust.TrustRoot,
	testPlatform generatortrust.Platform,
) error {
	return withCompileWorkspace(func(workspace string) error {
		for destination, source := range map[string]string{
			"go.mod": "tools/generated-sdk/generator/fixtures/template-fixture.mod",
			"go.sum": "tools/generated-sdk/generator/fixtures/template-fixture.sum",
		} {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source)))
			if err != nil {
				return err
			}
			// #nosec G703 -- workspace is a fresh temporary directory and destination is one of two fixed module filenames.
			if err := os.WriteFile(filepath.Join(workspace, destination), data, 0o644); err != nil {
				return err
			}
		}
		if err := copyTree(candidate, filepath.Join(workspace, "generated")); err != nil {
			return err
		}
		if err := writeGeneratedBehaviorTests(workspace); err != nil {
			return err
		}
		vendorGo, err := goBinaryForToolchain(ctx, trust.TestContainer.GoVersion)
		if err != nil {
			return err
		}
		if err := vendorCandidateDependencies(ctx, vendorGo, workspace); err != nil {
			return err
		}
		if err := compileCandidatePackages(ctx, workspace, trust.GoToolchains); err != nil {
			return err
		}
		docker, err := exec.LookPath("docker")
		if err != nil {
			return errors.New("docker is required to run generated-code behavior tests")
		}
		testOutput, err := runGeneratedBehaviorTests(
			ctx,
			docker,
			testPlatform.Platform,
			generatortrust.TestImageReference(trust, testPlatform),
			workspace,
		)
		if err != nil {
			return fmt.Errorf("generated-code behavior tests: %w: %s", err, strings.TrimSpace(testOutput))
		}
		return nil
	})
}

func compileCandidatePackages(ctx context.Context, workspace string, toolchains []string) error {
	compileDir := filepath.Join(workspace, ".compile")
	if err := os.MkdirAll(compileDir, 0o700); err != nil {
		return err
	}
	for _, toolchain := range toolchains {
		goCommand, err := goBinaryForToolchain(ctx, toolchain)
		if err != nil {
			return err
		}
		cache := filepath.Join(workspace, ".gocache", toolchain)
		if err := os.MkdirAll(cache, 0o700); err != nil {
			return err
		}
		for _, packageName := range []string{"trading", "marketdata"} {
			binary := filepath.Join(compileDir, toolchain+"-"+packageName+".test")
			command := exec.CommandContext(
				ctx,
				goCommand,
				"test", "-c", "-o", binary, "./generated/"+packageName,
			)
			command.Dir = workspace
			command.Env = replaceEnvironment(
				gosandbox.CompileEnvironment("-mod=vendor"),
				map[string]string{"GOCACHE": cache},
			)
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf(
					"compile candidate package %s with Go %s: %w: %s",
					packageName,
					toolchain,
					err,
					strings.TrimSpace(string(output)),
				)
			}
		}
	}
	return nil
}

func vendorCandidateDependencies(ctx context.Context, goCommand, workspace string) error {
	return gosandbox.Vendor(ctx, goCommand, workspace)
}

func runGeneratedBehaviorTests(
	ctx context.Context,
	docker, platform, image, workspace string,
) (string, error) {
	return gosandbox.RunTests(ctx, docker, platform, image, workspace, "test", "-count=1", "./generated/...")
}

func writeGeneratedBehaviorTests(workspace string) error {
	for _, fixture := range []struct {
		activityTest bool
		packageName  string
	}{
		{packageName: "marketdata"},
		{activityTest: true, packageName: "trading"},
	} {
		source := generatedClientBehaviorTest(fixture.packageName, fixture.activityTest)
		path := filepath.Join(workspace, "generated", fixture.packageName, "generated_policy_test.go")
		if err := safetree.WriteNew(path, []byte(source), 0o600); err != nil {
			return err
		}
	}
	return safetree.Validate(workspace)
}

func generatedClientBehaviorTest(packageName string, activityTest bool) string {
	activity := ""
	jsonImport := ""
	if activityTest {
		jsonImport = "\t\"encoding/json\"\n"
		activity = `
func TestUnknownNonFillActivityFallback(t *testing.T) {
	payload := []byte(` + "`" + `{"activity_type":"FUTURE_CODE","id":"synthetic-activity"}` + "`" + `)
	var value GetAccountActivitiesByActivityType200ResponseInner
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	if value.NonTradeActivities == nil || value.TradingActivities != nil {
		t.Fatalf("unknown non-FILL branch = non-trade:%v trading:%v", value.NonTradeActivities != nil, value.TradingActivities != nil)
	}
	if got := value.MatchedSchemas(); len(got) != 1 || got[0] != "NonTradeActivities" {
		t.Fatalf("matched schemas = %v, want [NonTradeActivities]", got)
	}
	if !bytes.Equal(value.RawJSON(), payload) {
		t.Fatalf("retained activity JSON = %s, want %s", value.RawJSON(), payload)
	}
}
`
	}
	return `package ` + packageName + `

import (
	"bytes"
` + jsonImport + `
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

type generatedPolicyRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip generatedPolicyRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestFloat64QueryFormattingPreservesPrecision(t *testing.T) {
	values := url.Values{}
	value := 1.2345678901234567
	parameterAddToHeaderOrQuery(values, "value", value, "", "")
	want := strconv.FormatFloat(value, 'g', -1, 64)
	if got := values.Get("value"); got != want {
		t.Fatalf("formatted float64 = %q, want %q", got, want)
	}
}

func TestDebugLoggingRedactsCredentialsAndBodies(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	previousPrefix := log.Prefix()
	log.SetOutput(&logs)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	config := NewConfiguration()
	config.Debug = true
	config.HTTPClient = &http.Client{Transport: generatedPolicyRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader("response-body-secret")),
		}, nil
	})}
	client := NewAPIClient(config)
	request, err := http.NewRequest(http.MethodPost, "https://example.invalid/test", strings.NewReader("request-body-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("APCA-API-KEY-ID", "key-id-secret")
	request.Header.Set("APCA-API-SECRET-KEY", "key-secret")
	request.Header.Set("Authorization", "Bearer authorization-secret")
	response, err := client.callAPI(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	got := logs.String()
	for _, secret := range []string{
		"key-id-secret",
		"key-secret",
		"authorization-secret",
		"request-body-secret",
		"response-body-secret",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("debug log exposed %q: %s", secret, got)
		}
	}
	if strings.Count(got, "[REDACTED]") != 3 {
		t.Fatalf("debug log redaction count = %d, want 3: %s", strings.Count(got, "[REDACTED]"), got)
	}
}
` + activity
}

func goBinaryForToolchain(ctx context.Context, version string) (string, error) {
	offlineEnvironment := replaceEnvironment(withoutEnvironment(os.Environ(), "GOROOT"), map[string]string{
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
		"GOPROXY":     "off",
		"GOSUMDB":     "off",
	})
	expected := "go version go" + version + " " + runtime.GOOS + "/" + runtime.GOARCH
	localVersion := exec.CommandContext(ctx, "go", "version")
	localVersion.Env = offlineEnvironment
	localVersionOutput, err := localVersion.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("identify local Go toolchain: %w: %s", err, strings.TrimSpace(string(localVersionOutput)))
	}

	var goCommand string
	if strings.TrimSpace(string(localVersionOutput)) == expected {
		command := exec.CommandContext(ctx, "go", "env", "GOROOT")
		command.Env = offlineEnvironment
		output, err := command.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("resolve local Go %s GOROOT: %w: %s", version, err, strings.TrimSpace(string(output)))
		}
		goRoot := strings.TrimSpace(string(output))
		if !filepath.IsAbs(goRoot) {
			return "", fmt.Errorf("go %s GOROOT is not absolute: %q", version, goRoot)
		}
		goCommand = filepath.Join(goRoot, "bin", "go")
	} else if pathCommand, pathErr := exec.LookPath("go" + version); pathErr == nil {
		goCommand = pathCommand
	} else {
		command := exec.CommandContext(ctx, "go", "env", "GOMODCACHE")
		command.Env = offlineEnvironment
		output, err := command.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("resolve Go module cache for %s toolchain: %w: %s", version, err, strings.TrimSpace(string(output)))
		}
		moduleCache := strings.TrimSpace(string(output))
		if !filepath.IsAbs(moduleCache) {
			return "", fmt.Errorf("go module cache is not absolute: %q", moduleCache)
		}
		goCommand = filepath.Join(
			moduleCache,
			"golang.org",
			"toolchain@v0.0.1-go"+version+"."+runtime.GOOS+"-"+runtime.GOARCH,
			"bin",
			"go",
		)
	}
	if runtime.GOOS == "windows" {
		goCommand += ".exe"
	}
	info, err := os.Stat(goCommand)
	if err != nil {
		return "", fmt.Errorf("resolve Go %s binary: %w", version, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("go %s binary is not a regular file", version)
	}
	verify := exec.CommandContext(ctx, goCommand, "version")
	verify.Env = offlineEnvironment
	versionOutput, err := verify.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("verify Go %s binary: %w: %s", version, err, strings.TrimSpace(string(versionOutput)))
	}
	if strings.TrimSpace(string(versionOutput)) != expected {
		return "", fmt.Errorf("go toolchain identity mismatch: got %q, want %q", strings.TrimSpace(string(versionOutput)), expected)
	}
	return goCommand, nil
}

func withoutEnvironment(environment []string, removed ...string) []string {
	keys := make(map[string]struct{}, len(removed))
	for _, key := range removed {
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, remove := keys[key]; remove {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

func withCompileWorkspace(run func(string) error) (err error) {
	workspace, err := os.MkdirTemp("", "generated-sdk-compile-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(workspace); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove compile workspace: %w", cleanupErr))
		}
	}()
	return run(workspace)
}

func copyTree(source, destination string) error {
	if err := safetree.Validate(source); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := safetree.ReadRegular(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func fileDigests(root string) (map[string]string, error) {
	if err := safetree.Validate(root); err != nil {
		return nil, err
	}
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := safetree.ReadRegular(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = digestBytes(data)
		return nil
	})
	return result, err
}

func treeDigest(root string) (string, error) {
	files, err := fileDigests(root)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	hash := sha256.New()
	for _, key := range keys {
		_, _ = hash.Write([]byte(key))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(files[key]))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func loadObject(path string) (map[string]any, error) {
	var result map[string]any
	if err := loadStrict(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func loadStrict(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := decodeStrict(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing content")
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func replaceEnvironment(environment []string, replacements map[string]string) []string {
	result := make([]string, 0, len(environment)+len(replacements))
	for _, item := range environment {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, replace := replacements[key]; replace {
				continue
			}
		}
		result = append(result, item)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}
