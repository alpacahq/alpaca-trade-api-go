package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/generatortrust"
)

func main() {
	var mode string
	var platform string
	var repoRoot string
	flag.StringVar(&mode, "mode", "check", "check, check-tracked, bootstrap, image-bootstrap, or digest-proposal")
	flag.StringVar(&platform, "platform", "", "generator platform: linux/amd64 or linux/arm64/v8")
	flag.StringVar(&repoRoot, "repo-root", "", "repository root")
	flag.Parse()
	if repoRoot == "" {
		repoRoot = findRepoRoot()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	successMessage := "Generated SDK generator trust check passed"
	switch mode {
	case "check":
		err = generatortrust.Verify(repoRoot)
	case "check-tracked":
		err = generatortrust.VerifyTracked(repoRoot)
		successMessage = "Generated SDK tracked generator trust check passed"
	case "digest-proposal":
		var proposal []byte
		proposal, err = generatortrust.ProposeDigests(repoRoot)
		if err == nil {
			if _, err = os.Stdout.Write(proposal); err == nil {
				return
			}
		}
	case "bootstrap":
		err = generatortrust.Bootstrap(ctx, repoRoot)
	case "image-bootstrap":
		if err = generatortrust.ValidateHostArchitecture(runtime.GOARCH); err != nil {
			break
		}
		if platform == "" {
			platform, err = generatortrust.DefaultPlatform(runtime.GOARCH)
			if err != nil {
				break
			}
		}
		var generatorImage, testImage string
		generatorImage, err = generatortrust.BootstrapImage(ctx, repoRoot, platform)
		if err == nil {
			testImage, err = generatortrust.BootstrapTestImage(ctx, repoRoot, platform)
		}
		if err == nil {
			fmt.Printf(
				"Generated SDK image bootstrap passed: platform=%s generator_image=%s test_image=%s\n",
				platform,
				generatorImage,
				testImage,
			)
			return
		}
	default:
		err = fmt.Errorf("unknown mode %q", mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "generatortrust:", err)
		os.Exit(1)
	}
	fmt.Println(successMessage)
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
