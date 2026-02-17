# AGENTS.md

## Dev environment tips

- Build both helm-datadog and kubectl-datadog binaries: `make build`

- Make code changes to `pkg/`, `cmd/`, or shell scripts
- To compile binaries run `make build` 
- If adding features, update reference files in `integration/references/expected/`

## Testing instructions

- Run unit tests `make test`
- Run integration tests with helm-mock `make test-integration`
- Run both unit and integration tests `make test-all`
- Run go vet `make vet`
- Run full CI pipeline locally `make ci`
- Run specific test `go test -v <go package path> -run <Test name>`
- Run shellcheck on all scripts `make check-scripts`

## PR instructions

- To create a PR use `.github/PULL_REQUEST_TEMPLATE.md` file as a reference.
- Always run `make test-all` before committing.
