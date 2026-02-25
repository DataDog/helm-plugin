#!/usr/bin/env bash

set -eo pipefail

# define HELM_DEBUG when installing plugin for debugging the process
if [ "${HELM_DEBUG:-}" = "1" ] || [ "${HELM_DEBUG:-}" = "true" ]; then
    set -x
fi

if [ ! -z "${HELM_DD_PLUGIN_NO_INSTALL_HOOK}" ]; then
    echo "Development mode: not downloading versioned release."
    exit 0
fi

# Safe guard if running script directly for local testing
if [[ -z ${HELM_PLUGIN_DIR} ]]; then
    HELM_PLUGIN_DIR="."
fi

echo "HELM_PLUGIN_DIR: ${HELM_PLUGIN_DIR}"

PLUGIN_YAML=${HELM_PLUGIN_DIR}/plugin.yaml
version="$(grep "version" "${PLUGIN_YAML}" | head -n 1 | awk '{print $2}' | tr -d '"')"

pluginType=""
# type only present in Helm v4 plugins which we need to detect here
if grep -q "type" "${PLUGIN_YAML}"; then
    pluginType=$(grep "type" "${PLUGIN_YAML}")
fi

# Run installation only for Helm v3 or postrender v4
if [[ ${pluginType} != "" && ! ${pluginType} =~ "postrenderer/"* ]]; then
    echo "No need to install the binary"
    exit 0
fi

echo "Downloading and installing helm-datadog v${version} ..."

PROJECT_NAME="helm-plugin"
PROJECT_GH="DataDog/${PROJECT_NAME}"

init_arch() {
    ARCH=$(uname -m)
    case "${ARCH}" in
    aarch64 | arm64) ARCH="arm64" ;;
    x86) ARCH="386" ;;
    x86_64) ARCH="amd64" ;;
    *)
        echo "Unsupported datadog binary architecture: ${ARCH}"
        echo "If you suspect this is a bug please report it on Github"
        exit 1
        ;;
    esac
}

init_os() {
    OS=$(uname -s)
    case "${OS}" in
    Darwin) OS="darwin" ;;
    Linux) OS="linux" ;;
    *)
        echo "Unsupported OS: ${OS}"
        echo "If you suspect this is a bug please report it on Github"
        exit 1
        ;;
    esac
}

# verify_supported checks that the os/arch combination is supported for
# binary builds.
verify_supported() {
    supported="linux-amd64\nlinux-arm64\ndarwin-amd64\ndarwin-arm64\n"
    if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
        echo "No prebuild binary for ${OS}-${ARCH}."
        exit 1
    fi

    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        echo "Either curl or wget is required"
        exit 1
    fi
}

mk_temp_dir() {
    HELM_TMP="$(mktemp -d -t "${PROJECT_NAME}")"
}

rm_temp_dir() {
    if [[ -d "${HELM_TMP}" ]]; then
        rm -rf "${HELM_TMP}"
    fi
}

# downloadFiles downloads the latest binary package and also the checksum
# for that binary.
download_files() {
    DOWNLOAD_URL="https://github.com/${PROJECT_GH}/releases/download/v${version}/helm-plugin_${version}_${OS}_${ARCH}.tar.gz"
    CHECKSUM_URL="https://github.com/${PROJECT_GH}/releases/download/v${version}/helm-plugin_${version}_checksums.txt"

    DOWNLOAD_URL_FILE="${HELM_TMP}/$(basename "${DOWNLOAD_URL}")"
    CHECKSUM_URL_FILE="${HELM_TMP}/$(basename "${CHECKSUM_URL}")"

    echo "Downloading ${DOWNLOAD_URL}"
    if command -v curl >/dev/null 2>&1; then
        curl -sSf -L "${DOWNLOAD_URL}" -o "${DOWNLOAD_URL_FILE}"
        curl -sSf -L "${CHECKSUM_URL}" -o "${CHECKSUM_URL_FILE}"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "${DOWNLOAD_URL}" -O "${DOWNLOAD_URL_FILE}"
        wget -q "${CHECKSUM_URL}" -O "${CHECKSUM_URL_FILE}"
    fi

    echo "Verifying checksum..."
    csum=$(shasum -a 256 "${DOWNLOAD_URL_FILE}" | cut -d' ' -f 1)
    if ! grep -q "${csum}" -r "${CHECKSUM_URL_FILE}"; then
        echo "Wrong shasum 256 for ${DOWNLOAD_URL_FILE}"
        exit 2
    fi
    echo "OK"
}

# installFile verifies the SHA256 for the file, then unpacks and
# installs it.
install_file() {
    tar xzf "$DOWNLOAD_URL_FILE" -C "$HELM_TMP"
    HELM_TMP_BIN="$HELM_TMP/helm-plugin"
    echo "Preparing to install into ${HELM_PLUGIN_DIR}"
    mkdir -p "$HELM_PLUGIN_DIR/bin"
    cp "$HELM_TMP_BIN" "$HELM_PLUGIN_DIR/bin"

    echo "helm-datadog ${version} is installed."
    echo "Happy Helming!"
}

exit_trap() {
    result=$?
    # rm_temp_dir
    if [ "$result" != "0" ]; then
        echo "Failed to install $PROJECT_NAME"
        printf "\tFor support, go to https://%s.\n" ${PROJECT_GH}
    fi
    exit $result
}

#Stop execution on any error
trap "exit_trap" EXIT

init_arch
init_os
verify_supported
mk_temp_dir
download_files
install_file
