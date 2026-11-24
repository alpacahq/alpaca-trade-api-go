# easyjson generation

The repository's existing easyjson directives use easyjson 0.7.7, while normal SDK builds use the current easyjson version from the root module. Pinning only the generator executable is insufficient because easyjson also loads its generation package through the SDK module graph.

Run `make generate-easyjson` to regenerate the SDK entity serializers. The target:

- copies the current root module files into a temporary directory
- overrides only easyjson to 0.7.7 in that temporary module graph
- propagates the temporary graph to easyjson's bootstrap process
- installs the easyjson executable into a temporary `GOBIN`
- discovers and runs every easyjson directive
- fails if no easyjson directives match
- removes the temporary generation environment when it finishes

The root `generate` target depends on `generate-easyjson`. Other generators can use separate targets and their current dependency graphs.

This keeps normal SDK builds and consumers on the current root module graph while reproducing the tracked serializers byte for byte. CI runs `make generate` and fails if generation leaves any tracked or untracked changes.

An easyjson version update requires regenerating all outputs, reviewing the complete generated diff, and running compatibility tests. In particular, verify null-decoding behavior because newer easyjson versions can change it.
