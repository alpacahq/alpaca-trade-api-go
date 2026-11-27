#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <linux/amd64|linux/arm64/v8>" >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
platform=$1

case "$platform" in
  linux/amd64|linux/arm64/v8) ;;
  *) usage ;;
esac

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd "$script_dir/../../.." && pwd -P)
go_root=$(GOTOOLCHAIN=go1.27.3 GOWORK=off GOPROXY=off go env GOROOT)

tool() {
  PATH="$go_root/bin:$PATH" GOTOOLCHAIN=local GOWORK=off GOFLAGS=-mod=readonly GOPROXY=off GOSUMDB=off \
    "$go_root/bin/go" -C "$repo_root/tools/generated-sdk/tooling" "$@"
}

tool run ./cmd/generate -platform "$platform" -repo-root "$repo_root"
