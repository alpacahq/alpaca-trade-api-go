package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/specsource"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/specupdate"
)

type commandRunner struct {
	platform         string
	repoRoot         string
	reviewSpecChange bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "specupdate:", err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "dry-run", "dry-run, finalize, adopt-additive, adopt-breaking, or rollback")
	repoRootFlag := flag.String("repo-root", ".", "repository root")
	proposalDir := flag.String("proposal-dir", "", "reviewed proposal directory")
	outputDir := flag.String("output-dir", "", "new external proposal directory")
	reviewedDir := flag.String("reviewed-dir", "", "directory containing reviewed feature-allowlist.json and optional overlays.json")
	acceptPlan := flag.String("accept-plan", "", "exact reviewed proposal digest")
	retrievalDate := flag.String("retrieval-date", "", "UTC retrieval date in YYYY-MM-DD")
	platform := flag.String("platform", "", "generator platform")
	timeout := flag.Duration("timeout", 30*time.Second, "public specification fetch timeout")
	flag.Parse()
	repoRoot, err := filepath.Abs(*repoRootFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runner := commandRunner{repoRoot: repoRoot, platform: *platform}
	switch *mode {
	case "dry-run":
		date, err := time.Parse("2006-01-02", *retrievalDate)
		if err != nil || date.Format("2006-01-02") != *retrievalDate {
			return errors.New("dry-run requires -retrieval-date in YYYY-MM-DD")
		}
		if *outputDir == "" {
			return errors.New("dry-run requires -output-dir")
		}
		reviewRunner := runner
		reviewRunner.reviewSpecChange = true
		manifest, err := specupdate.Acquire(ctx, specupdate.AcquireOptions{
			Client: specsource.NewClient(*timeout), OutputDir: *outputDir,
			RepoRoot: repoRoot, RetrievalDate: date, Runner: reviewRunner,
		})
		if err != nil {
			return err
		}
		return printManifest(manifest)
	case "finalize":
		if *proposalDir == "" || *outputDir == "" || *reviewedDir == "" || *acceptPlan == "" {
			return errors.New("finalize requires -proposal-dir, -output-dir, -reviewed-dir, and -accept-plan")
		}
		manifest, err := specupdate.Finalize(ctx, specupdate.FinalizeOptions{
			AcquisitionDir: *proposalDir, AcceptPlan: *acceptPlan, OutputDir: *outputDir,
			RepoRoot: repoRoot, ReviewedDir: *reviewedDir, Runner: runner,
		})
		if err != nil {
			return err
		}
		return printManifest(manifest)
	case "adopt-additive", "adopt-breaking":
		if *proposalDir == "" || *acceptPlan == "" {
			return errors.New("adoption requires -proposal-dir and -accept-plan")
		}
		if err := specupdate.Adopt(ctx, specupdate.AdoptOptions{
			AcceptPlan: *acceptPlan, AllowBreaking: *mode == "adopt-breaking",
			BundleDir: *proposalDir, RepoRoot: repoRoot, Validator: runner,
		}); err != nil {
			return err
		}
		fmt.Println("generated SDK specification inputs adopted and validated")
		return nil
	case "rollback":
		if err := specupdate.Rollback(repoRoot); err != nil {
			return err
		}
		fmt.Println("generated SDK specification adoption rolled back")
		return nil
	default:
		return fmt.Errorf("unsupported mode %q", *mode)
	}
}

func (runner commandRunner) Run(
	ctx context.Context,
	bundleRoot, outputDir string,
) (specupdate.CandidateResult, error) {
	return runner.executeCandidate(ctx, runner.candidateArgs(bundleRoot, outputDir))
}

func (runner commandRunner) candidateArgs(bundleRoot, outputDir string) []string {
	args := runner.generateArgs()
	args = append(args, "-output-dir", outputDir, "-json")
	if bundleRoot != "" {
		args = append(args, "-input-bundle", bundleRoot)
		if runner.reviewSpecChange {
			args = append(args, "-review-spec-change")
		}
	}
	return args
}

func (runner commandRunner) Validate(ctx context.Context, _ string) (specupdate.CandidateResult, error) {
	args := append(runner.generateArgs(), "-json")
	return runner.executeCandidate(ctx, args)
}

func (runner commandRunner) executeCandidate(
	ctx context.Context,
	args []string,
) (specupdate.CandidateResult, error) {
	// #nosec G204 -- The executable is the active Go toolchain and arguments are constructed by the fixed candidate runner.
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
	command.Env = offlineEnvironment()
	command.Stderr = os.Stderr
	var output bytes.Buffer
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return specupdate.CandidateResult{}, err
	}
	var result specupdate.CandidateResult
	if err := spec.DecodeStrict(output.Bytes(), &result); err != nil {
		return specupdate.CandidateResult{}, fmt.Errorf("decode candidate result: %w", err)
	}
	return result, nil
}

func (runner commandRunner) generateArgs() []string {
	args := []string{
		"-C", filepath.Join(runner.repoRoot, "tools/generated-sdk/tooling"),
		"run", "./cmd/generate", "-repo-root", runner.repoRoot,
	}
	if runner.platform != "" {
		args = append(args, "-platform", runner.platform)
	}
	return args
}

func offlineEnvironment() []string {
	replacements := map[string]string{
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
		"GOFLAGS":     "-mod=readonly",
		"GOPROXY":     "off",
		"GOSUMDB":     "off",
	}
	result := make([]string, 0, len(os.Environ())+len(replacements))
	for _, item := range os.Environ() {
		key, _, found := bytes.Cut([]byte(item), []byte("="))
		if found {
			if _, replace := replacements[string(key)]; replace {
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

func printManifest(manifest specupdate.Manifest) error {
	data, err := spec.Canonical(manifest)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
