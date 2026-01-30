.SILENT: $(CI)
.DEFAULT_GOAL := help

SHELL = /bin/bash -e

# Helm plugin
HELM_PLUGIN_MAIN := ./cmd/helm-plugin/main.go
HELM_PLUGIN_BIN := ./bin/helm-plugin

# Kubectl datadog plugin
KUBECTL_DATADOG_MAIN := ./cmd/kubectl-datadog/main.go
KUBECTL_DATADOG_BIN := ./bin/kubectl-datadog

BINDIR ?= $(HOME)/.local/bin

HELM4_PLUGIN_PATH = ./plugins/
VERSION := $(shell grep "version" plugin.yaml | head -n 1 | cut -d ':' -f 2 | tr -d '" ')

.PHONY: ci
ci: build test vet test-integration ## Runs build test vet and integration tests in CI
	@echo "CI done"

.PHONY: build
build: ## Build helm post renderer and kubectl plugin
	CGO_ENABLED=0 go build -o $(HELM_PLUGIN_BIN) $(HELM_PLUGIN_MAIN)
	CGO_ENABLED=0 go build -o $(KUBECTL_DATADOG_BIN) $(KUBECTL_DATADOG_MAIN)

.PHONY: kube_install
kube_install:
	install -m 0755 $(KUBECTL_DATADOG_BIN) $(BINDIR)

.PHONY: kube_uninstall
kube_uninstall:
	rm -f $(KUBECTL_DATADOG_BIN)

.PHONY: install
install: build ## Install Helm v3 in devmode
	helm plugin install .

.PHONY: uninstall
uninstall:
	helm plugin uninstall datadog
	helm plugin uninstall datadog-post-renderer

# This target is for testing
.PHONY: package
package: clean
	rm -rf $(HELM4_PLUGIN_PATH)/datadog/scripts
	rm $(HELM4_PLUGIN_PATH)/datadog/datadog-$(VERSION).tgz || true
	cp -a scripts $(HELM4_PLUGIN_PATH)/datadog
	rm -rf $(HELM4_PLUGIN_PATH)/datadog-post-renderer/scripts
	rm $(HELM4_PLUGIN_PATH)/datadog-post-renderer/datadog-post-renderer-$(VERSION).tgz || true
	cp -a scripts $(HELM4_PLUGIN_PATH)/datadog-post-renderer
	helm plugin package $(HELM4_PLUGIN_PATH)/datadog --sign=false -d $(HELM4_PLUGIN_PATH)/datadog
	helm plugin package $(HELM4_PLUGIN_PATH)/datadog-post-renderer --sign=false -d $(HELM4_PLUGIN_PATH)/datadog-post-renderer

# This target is for testing
.PHONY: install4
install4:
	helm plugin install $(HELM4_PLUGIN_PATH)/datadog/datadog-$(VERSION).tgz --verify=false
	helm plugin install $(HELM4_PLUGIN_PATH)/datadog-post-renderer/datadog-post-renderer-$(VERSION).tgz --verify=false

# This target is for testing
.PHONY: install4loc
install4loc: build
	helm plugin install $(HELM4_PLUGIN_PATH)/datadog --verify=false
	helm plugin install $(HELM4_PLUGIN_PATH)/datadog-post-renderer --verify=false
	cp $(HELM_PLUGIN_BIN) ./plugins/datadog-post-renderer/bin/helm-plugin

.PHONY: vet
vet: ## Runs go vet
	go vet ./...

.PHONY: test
test: ## Runs go test
	go test  ./...

.PHONY: test-integration
test-integration: build ## Runs integration tests
	./integration/test-helm.sh

.PHONY: test-all
test-all: test test-integration ## Runs all tests (unit and integration)
	@echo "All tests completed"

check-scripts:
	# Fail if any of these files have warnings
	shellcheck scripts/*.sh

cover: ## Generates test coverage report for local analisys
	rm cover.out
	go test -coverprofile cover.out ./...
	go tool cover -html=cover.out

.PHONY: clean
clean: ## Clean local binaries
	@rm -rf ./bin/
	@rm -rf ./plugins/datadog-post-renderer/bin

help:
	@perl -nle'print $& if m{^[a-zA-Z_-]+:.*?## .*$$}' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'
