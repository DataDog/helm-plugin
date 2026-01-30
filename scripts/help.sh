#!/usr/bin/env bash

set -euf

help_usage() {
    cat <<'EOF'

helm-plugin is a helm plugin for annotating helm charts with location information.

For more information, see the README.md at https://github.com/datadog/helm-plugin

Example:
  $ helm datadog [<PLUGIN OPTIONS>] install <HELM INTALL OPTIONS>
  $ helm datadog [<PLUGIN OPTIONS>] upgrade <HELM UPGRADE OPTIONS>
  $ helm datadog [<PLUGIN OPTIONS>] template <HELM template OPTIONS>

Available Commands:
  <cmd>   wrapper runs helm <cmd> and post-renderer to add annotations

Available Options:
  --help                                           -h  Show help
  --version                                        -v  Display version of helm-plugin
EOF
}
