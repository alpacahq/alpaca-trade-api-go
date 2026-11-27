package gosandbox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
)

func CompileEnvironment(goFlags string) []string {
	return replaceEnvironment(withoutEnvironment(os.Environ(), "GOROOT"), map[string]string{
		"CGO_ENABLED":  "0",
		"GOARCH":       runtime.GOARCH,
		"GOENV":        "off",
		"GOEXPERIMENT": "",
		"GOFLAGS":      goFlags,
		"GOOS":         runtime.GOOS,
		"GOPROXY":      "off",
		"GOSUMDB":      "off",
		"GOTOOLCHAIN":  "local",
		"GOWORK":       "off",
	})
}

func Vendor(ctx context.Context, goCommand, moduleDir string) (resultErr error) {
	vendorCache, err := os.MkdirTemp("", "generated-sdk-vendor-cache-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(vendorCache); cleanupErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove vendor cache: %w", cleanupErr))
		}
	}()
	command := exec.CommandContext(ctx, goCommand, "mod", "vendor")
	command.Dir = moduleDir
	command.Env = replaceEnvironment(
		CompileEnvironment("-mod=readonly"),
		map[string]string{"GOCACHE": vendorCache},
	)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("vendor generated dependencies: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return safetree.Validate(moduleDir)
}

func prepareReadOnlyMount(moduleDir string) error {
	if err := safetree.Validate(moduleDir); err != nil {
		return err
	}
	return filepath.WalkDir(moduleDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is forbidden in sandbox mount: %s", path)
		}
		if entry.IsDir() {
			// #nosec G122,G302 -- moduleDir is validated symlink-free inside a private 0700 host temp parent; container UID 65534 needs 0755 to traverse /workspace.
			return os.Chmod(path, 0o755)
		}
		// #nosec G122,G302 -- moduleDir is validated symlink-free inside a private 0700 host temp parent; container UID 65534 needs 0644 to read /workspace files.
		return os.Chmod(path, 0o644)
	})
}

func RunTests(
	ctx context.Context,
	docker, platform, image, moduleDir string,
	goArguments ...string,
) (string, error) {
	if err := prepareReadOnlyMount(moduleDir); err != nil {
		return "", err
	}
	arguments := []string{
		"run", "--rm", "--pull=never", "--network=none", "--read-only",
		"--platform=" + platform,
		"--cap-drop=ALL", "--security-opt=no-new-privileges=true", "--pids-limit=256",
		"--memory=4g", "--cpus=2",
		"--user=65534:65534",
		"--workdir=/workspace",
		"--mount=type=bind,src=" + moduleDir + ",dst=/workspace,readonly",
		"--tmpfs=/tmp:rw,exec,nosuid,nodev,size=1g,mode=1777",
		"--entrypoint=/usr/bin/env",
		image,
		"-i",
		"CGO_ENABLED=0",
		"GOCACHE=/tmp/go-build",
		"GOENV=off",
		"GOFLAGS=-mod=vendor",
		"GOPATH=/tmp/gopath",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
		"HOME=/tmp/home",
		"PATH=/usr/local/go/bin:/usr/bin:/bin",
		"/usr/local/go/bin/go",
	}
	arguments = append(arguments, goArguments...)
	command := exec.CommandContext(ctx, docker, arguments...)
	output, err := command.CombinedOutput()
	return string(output), err
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
