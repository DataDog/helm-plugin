# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Helm plugin that adds Datadog-specific annotations to Kubernetes manifests. It tracks the origin/location of deployed resources by injecting annotations that map resources to their source repository and chart locations.

The plugin supports:
- **Helm 3**: Single plugin installation
- **Helm 4**: Split into two plugins (CLI and post-renderer capabilities)

## Key Commands

### Build
```bash
make build              # Build both helm-datadog and kubectl-datadog binaries
```

### Testing
```bash
make test               # Run unit tests
make test-integration   # Run integration tests with helm-mock
make test-all           # Run both unit and integration tests
make vet                # Run go vet
make ci                 # Run full CI pipeline (build, test, vet, integration)

# Run specific test
go test -v ./pkg/helmdriver -run TestChartURL
go test -v ./pkg/git -run TestIsInsideRepo
```

### Coverage
```bash
make cover              # Generate coverage report and open in browser
```

### Installation (Local Development)
```bash
make install            # Build and install Helm v3 plugin in devmode (locally)
make uninstall          # Uninstall all plugin
```

### Helm 4 Packaging (Testing Only)
```bash
make package            # Package both plugins for Helm 4
make install4           # Install packaged Helm 4 plugins
make install4loc        # Install local Helm 4 plugins in devmode (locally)
```

## Architecture

### Two Deployment Modes

The plugin operates in two modes depending on the tool being used:

1. **Helm Mode** (`cmd/helm-datadog`): Acts as a Helm post-renderer
   - Invoked via `helm datadog <command>`
   - Wraps Helm commands and injects itself as `--post-renderer`
   - Processes rendered manifests through stdin/stdout
   - Adds Helm-specific location annotations (chart URL, values, etc.)

2. **Kubectl Mode** (`cmd/kubectl-datadog`): Standalone manifest processor
   - Invoked via `kubectl datadog` or directly
   - Reads manifests from stdin, outputs to stdout
   - Adds repository-based location annotations (git URL, commit SHA, path)
   - Example: `cat manifest.yaml | kubectl datadog | kubectl apply -f -`

### Core Components

#### `pkg/annotator/annotator.go`
The central annotation engine that processes YAML manifests. It:
- Decodes YAML streams (supports multi-document YAML)
- Injects location annotations into resource metadata
- Supports two location types via `LocationObject`:
  - `HelmLocation`: For Helm charts (chartURL, repoURL, valuesPath, etc.)
  - `RepoLocation`: For plain K8s manifests (git URL, revision, path)
- Encodes location data as JSON in annotation value
- Adds plugin version annotation when available

**Key annotation keys:**
- `origin.datadoghq.com/location`: Contains JSON location object
- `origin.datadoghq.com/plugin-version`: Plugin version (e.g., "v3/0.1.1")

#### `pkg/helmdriver/helmdriver.go`
Helm-specific driver that:
- Parses Helm CLI arguments to extract chart and values file paths
- Scans rendered manifests to find release name (from `app.kubernetes.io/instance` label)
- Detects chart source type (local path, URL, OCI registry, etc.)
- Auto-detects git repository information if chart is in a git repo
- Falls back to `DD_HELM_CHART_URL` env var if git detection fails
- Constructs `HelmLocation` object with all chart metadata

**Chart source detection logic:**
The driver handles all Helm installation patterns:
1. Chart reference: `helm install mymaria example/mariadb`
2. Packaged chart: `helm install mynginx ./nginx-1.2.3.tgz`
3. Unpacked directory: `helm install mynginx ./nginx`
4. Absolute URL: `helm install mynginx https://example.com/charts/nginx-1.2.3.tgz`
5. Chart + repo: `helm install --repo https://example.com/charts/ mynginx nginx`
6. OCI registry: `helm install mynginx --version 1.2.3 oci://example.com/charts/nginx`

#### `pkg/kubectldriver/kubectldriver.go`
Kubectl-specific driver that:
- Auto-detects git repository information from current directory
- Accepts manual overrides via CLI flags (`--repo-url`, `--target-revision`, `--path`)
- Constructs `RepoLocation` object
- Simpler than helmdriver since it doesn't need to parse Helm args

#### `pkg/git/git.go`
Git utility functions:
- `IsInsideRepo(dir)`: Check if directory is in a git repository
- `GetRepoURLAndSHA(dir)`: Extract remote URL and current commit SHA

### Helm Version Detection

The plugin detects Helm version by checking `plugin.yaml`:
- **Helm v3**: No `type:` field in plugin.yaml
- **Helm v4**: Contains `type: cli/v1` or `type: postrenderer/v1`

Version string format: `v{3,4}/{plugin-version}` (e.g., "v3/0.1.1" or "v4/0.1.1")

### Helm 3 vs Helm 4 Plugin Structure

**Helm 3** (single plugin):
- `plugin.yaml` at repo root
- Single binary: `bin/helm-datadog`
- Script wrapper: `scripts/main.sh`

**Helm 4** (split plugins):
- `plugins/datadog/` - CLI plugin (type: cli/v1)
  - Provides `helm datadog` command
  - Delegates to `datadog-post-renderer` plugin for actual rendering
- `plugins/datadog-post-renderer/` - Post-renderer plugin (type: postrenderer/v1)
  - Invoked by Helm as subprocess
  - Contains actual `bin/helm-datadog` binary

## Integration Tests

Located in `integration/test-helm.sh`:
- Uses mock Helm binary (`integration/helm-mock.sh`) to validate plugin behavior
- Tests both Helm v3 and v4 integration
- Validates:
  - Post-renderer binary path (absolute for v3, plugin name for v4)
  - Version detection and annotation
  - Helm command forwarding
  - Manifest output comparison against reference files
- Reference files in `integration/references/`:
  - `input/*.yaml` - Input manifests
  - `expected/v3/*.yaml` - Expected output for Helm v3
  - `expected/v4/*.yaml` - Expected output for Helm v4

**Important for test maintenance:**
- The mock expects `--post-renderer-args` to be passed multiple times (once per arg)
- `scripts/main.sh:67` uses `-pluginver` (single dash) as a flag to the post-renderer binary
- Expected files use `targetRevision` placeholders that get replaced with current git hash during test runs

## Development Workflow

1. Make code changes to `pkg/`, `cmd/`, or shell scripts
2. Run `make build` to compile binaries
3. Run `make test` for unit tests
4. Run `make test-integration` to validate plugin behavior end-to-end
5. If adding features, update reference files in `integration/references/expected/`

## Environment Variables

- `HELM_DEBUG=1`: Enable verbose shell script debugging
- `DD_HELM_CHART_URL`: Override chart URL when git detection unavailable
- `HELM_PLUGIN_DIR`: Set by Helm, points to plugin installation directory
- `HELM_BIN`: Override Helm binary path (used in tests)

## Logging

Both binaries write logs to local files:
- `ddhelm.log` - Helm plugin logs
- `kubectl-datadog.log` - Kubectl plugin logs

Logs use structured logging via `log/slog`.

## Shell Scripts

Key shell scripts in `scripts/`:
- `main.sh` - Main entry point for the plugin, handles version detection and command routing
- `install.sh` - Plugin installation script
- `help.sh` - Help text and usage information
- `version.sh` - Version display logic

Script linting:
```bash
make check-scripts  # Run shellcheck on all scripts
```
