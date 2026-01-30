#!/usr/bin/env bash
set -euo pipefail

if [ "${HELM_DEBUG:-}" = "1" ] || [ "${HELM_DEBUG:-}" = "true" ]; then
    set -x
fi

HELM_BIN=${HELM_BIN:-$(helm env HELM_BIN)}

# Path to current directory
SCRIPT_DIR="${HELM_PLUGIN_DIR}/scripts"
PLUGIN_BIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POST_RENDERER_BIN="${PLUGIN_BIN_DIR}/../bin/helm-datadog"

PLUGIN_YAML=${HELM_PLUGIN_DIR}/plugin.yaml
PLUGIN_TYPE=""
HELM_VERSION="v3"
# type only present in Helm v4 plugins which we need to detect here
if grep -q "^type:" "${PLUGIN_YAML}"; then
    PLUGIN_TYPE=$(grep "^type:" "${PLUGIN_YAML}")
    HELM_VERSION="v4"
fi

# Extract plugin version from plugin.yaml
PLUGIN_VERSION=$(grep "^version:" "${PLUGIN_YAML}" | awk '{print $2}' | tr -d '"')
# Build full version string in format v{3,4}/x.y.z
FULL_VERSION="${HELM_VERSION}/${PLUGIN_VERSION}"

# This error is only valid for Helm v3 which is identified using PLUGIN_TYPE
if [[ ! -x "$POST_RENDERER_BIN" && ! ${PLUGIN_TYPE} =~ "cli/"* ]]; then
    echo "Error: post-renderer binary not found at $POST_RENDERER_BIN" >&2
    echo "Make sure installation was successful"
    exit 1
fi

# In Helm v4 case we need to override POST_RENDERER_BIN to plugin name instead
if [[ ${PLUGIN_TYPE} =~ "cli/"* ]]; then
    POST_RENDERER_BIN="datadog-post-renderer"
fi

is_help() {
    case "$1" in
    -h | --help | help)
        true
        ;;
    *)
        false
        ;;
    esac
}

helm_command() {
    if [ $# -lt 2 ] || is_help "$2"; then
        . "${SCRIPT_DIR}/help.sh"
        help_usage

        # helm_command_usage "${1:-"[helm command]"}"
        return
    fi

    # Forward all args to helm, and inject the post-renderer flags with args.
    # Pass --pluginver and version as separate args, then semicolon-packed helm args
    args=$(
        IFS=';'
        printf '%s' "$*"
    )
    ${HELM_BIN} "$@" --post-renderer "$POST_RENDERER_BIN" --post-renderer-args -pluginver --post-renderer-args "${FULL_VERSION}" --post-renderer-args "${args}"
}

while true; do
    case "${1:-}" in
    --version | -v)
        # shellcheck source=scripts/version.sh
        . "${SCRIPT_DIR}/version.sh"
        version
        break
        ;;
    --help | -h)
        # shellcheck source=scripts/help.sh
        . "${SCRIPT_DIR}/help.sh"
        help_usage
        break
        ;;
    "")
        # shellcheck source=scripts/help.sh
        . "${SCRIPT_DIR}/help.sh"
        help_usage
        exit 1
        ;;
    *)
        helm_command "$@"
        break
        ;;
    esac
    shift
done

exit 0
