#!/usr/bin/env bash
set -euo pipefail

if [ "${HELM_DEBUG:-}" = "1" ] || [ "${HELM_DEBUG:-}" = "true" ]; then
    set -x
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

# Get script directory and repo root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Test directories
TEST_TMP_DIR="${SCRIPT_DIR}/tmp"
# TEST_CHARTS_DIR="${TEST_TMP_DIR}/charts"
TEST_REFS_DIR="${SCRIPT_DIR}/references"

# Get current git hash for substitution in expected files
CURRENT_GIT_HASH=$(cd "${REPO_ROOT}" && git rev-parse HEAD)

# Stop execution on any error
trap "cleanup" EXIT

# Cleanup function
cleanup() {
    if [ -d "${TEST_TMP_DIR}" ]; then
        rm -rf "${TEST_TMP_DIR}"
    fi
}

# Setup test environment
setup() {
    cleanup
    mkdir -p "${TEST_TMP_DIR}"
}

# Print test result
print_result() {
    local test_name=$1
    local result=$2
    local message=$3

    if [ "$result" = "pass" ]; then
        echo -e "${GREEN}✓${NC} ${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗${NC} ${test_name}"
        echo -e "  ${RED}Error:${NC} ${message}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Prepare expected file by substituting current git hash and plugin version
# $1 - source expected file path
# $2 - destination file path
# $3 - plugin version
prepare_expected_file() {
    local source_file=$1
    local dest_file=$2
    local plugin_version=$3

    # Replace targetRevision with current git hash, then replace plugin version
    sed "s/\"targetRevision\":\"[^\"]*\"/\"targetRevision\":\"${CURRENT_GIT_HASH}\"/" "${source_file}" |
        sed "s/v3\/[0-9.][0-9.]*/v3\/${plugin_version}/g; s/v4\/[0-9.][0-9.]*/v4\/${plugin_version}/g" >"${dest_file}"
}

# Test Helm v3 integration for one file
# $1 - input file for the post-renderer
# $2 - expected file after post-renderer
test_helm_v3_integration() {
    # Create test chart
    local test_dir="${TEST_TMP_DIR}/v3"
    mkdir -p "${test_dir}"
    local chart_dir="chart/dir/dummy" # chart dir is only used as dummy
    local helm_output_file="${test_dir}/helm-output.txt"
    local input_file="${TEST_REFS_DIR}/input/$1"
    local expected_file_source="${TEST_REFS_DIR}/expected/$2"
    local expected_file="${test_dir}/expected-$2"
    local output_manifest="${test_dir}/output.yaml"

    echo -e "\n${YELLOW}Running Helm v3 Integration Tests using $1${NC}"

    # Set up environment for Helm v3
    export HELM_PLUGIN_DIR="${REPO_ROOT}"

    # Get plugin version from plugin.yaml
    local plugin_version=$(grep '^version:' "${HELM_PLUGIN_DIR}/plugin.yaml" | awk '{print $2}')

    # Prepare expected file with current git hash and plugin version
    prepare_expected_file "${expected_file_source}" "${expected_file}" "${plugin_version}"

    # Test 1: Verify helm command is called with correct arguments
    OUTPUT_FILE=${helm_output_file} \
        HELM_BIN=${SCRIPT_DIR}/helm-mock.sh \
        TEST_MANIFEST=${input_file} \
        bash "${HELM_PLUGIN_DIR}/scripts/main.sh" template test-release "${chart_dir}" >"${output_manifest}" 2>&1 || true

    if [[ -f "${helm_output_file}" ]]; then
        # Check that post-renderer binary path is correct (should be full path to bin/helm-plugin)
        local post_renderer_bin=$(grep "POST_RENDERER_BIN:" "${helm_output_file}" | cut -d' ' -f2-)

        if [[ "${post_renderer_bin}" == *"/bin/helm-plugin" ]]; then
            print_result "Helm v3: post-renderer binary path" "pass" ""
        else
            print_result "Helm v3: post-renderer binary path" "fail" "Expected path ending with /bin/helm-plugin, got: ${post_renderer_bin}"
        fi

        # Check that post-renderer-args contains version
        local post_renderer_args=$(grep "POST_RENDERER_ARGS:" "${helm_output_file}" | cut -d' ' -f2-)

        if [[ "${post_renderer_args}" == *"-pluginver v3/"* ]]; then
            print_result "Helm v3: version detection (v3)" "pass" ""
        else
            print_result "Helm v3: version detection (v3)" "fail" "Expected args to contain '-pluginver v3/', got: ${post_renderer_args}"
        fi

        # Check that helm template command is preserved
        if grep -q "HELM_ARGS:.*template.*test-release" "${helm_output_file}"; then
            print_result "Helm v3: helm command forwarding" "pass" ""
        else
            print_result "Helm v3: helm command forwarding" "fail" "Expected helm template command to be forwarded"
        fi

        # Check validation passed
        if grep -q "VALIDATION: PASS" "${helm_output_file}"; then
            print_result "Helm v3: post-renderer setup" "pass" ""
        else
            print_result "Helm v3: post-renderer setup" "fail" "Post-renderer flags validation failed"
        fi

        if [[ ! -f "${output_manifest}" ]]; then
            print_result "Helm v3: post-renderer output" "fail" "Expected output file at path: ${output_manifest}"
        fi

        if cmp -s "${expected_file}" "${output_manifest}"; then
            print_result "Helm v3: post-renderer compare expected with output" "pass" ""
        else
            print_result "Helm v3: post-renderer compare expected with output" "fail" "Expected (left) differs from output (right)"
            diff --color=always -u "${expected_file}" "${output_manifest}" || true
        fi
    else
        print_result "Helm v3: command execution" "fail" "Output file not created"
    fi

    rm "${helm_output_file}"
}

# Test Helm v4 integration for one file
# $1 - input file for the post-renderer
# $2 - expected file after post-renderer
test_helm_v4_integration() {
    # Create test chart
    local test_dir="${TEST_TMP_DIR}/v4"
    mkdir -p "${test_dir}"
    local chart_dir="chart/dir/dummy" # chart dir is only used as dummy
    local helm_output_file="${test_dir}/helm-output.txt"
    local input_file="${TEST_REFS_DIR}/input/$1"
    local expected_file_source="${TEST_REFS_DIR}/expected/$2"
    local expected_file="${test_dir}/expected-$2"
    local output_manifest="${test_dir}/output.yaml"

    echo -e "\n${YELLOW}Running Helm v4 Integration Tests using $1${NC}"

    # Set up environment for Helm v4 (uses plugins/datadog)
    export HELM_PLUGIN_DIR="${REPO_ROOT}/plugins/datadog"

    # Get plugin version from plugin.yaml
    local plugin_version=$(grep '^version:' "${HELM_PLUGIN_DIR}/plugin.yaml" | awk '{print $2}')

    # Prepare expected file with current git hash and plugin version
    prepare_expected_file "${expected_file_source}" "${expected_file}" "${plugin_version}"

    # For Helm v4, create a mock datadog-post-renderer in PATH that redirects to actual binary
    local mock_post_renderer="${test_dir}/datadog-post-renderer"
    cat >"${mock_post_renderer}" <<'MOCKEOF'
#!/usr/bin/env bash
exec REPO_ROOT/bin/helm-plugin "$@"
MOCKEOF
    sed -i.bak "s|REPO_ROOT|${REPO_ROOT}|g" "${mock_post_renderer}"
    rm "${mock_post_renderer}.bak"
    chmod +x "${mock_post_renderer}"
    export PATH="${test_dir}:${PATH}"

    # Test 1: Verify helm command is called with correct arguments
    OUTPUT_FILE=${helm_output_file} \
        HELM_BIN=${SCRIPT_DIR}/helm-mock.sh \
        TEST_MANIFEST=${input_file} \
        bash "${HELM_PLUGIN_DIR}/scripts/main.sh" template test-release "${chart_dir}" >"${output_manifest}" 2>&1 || true

    if [[ -f "${helm_output_file}" ]]; then
        # Check that post-renderer binary is plugin name (not path) for Helm v4
        local post_renderer_bin=$(grep "POST_RENDERER_BIN:" "${helm_output_file}" | cut -d' ' -f2-)

        if [[ "${post_renderer_bin}" == "datadog-post-renderer" ]]; then
            print_result "Helm v4: post-renderer plugin name" "pass" ""
        else
            print_result "Helm v4: post-renderer plugin name" "fail" "Expected 'datadog-post-renderer', got: ${post_renderer_bin}"
        fi

        # Check that post-renderer-args contains version
        local post_renderer_args=$(grep "POST_RENDERER_ARGS:" "${helm_output_file}" | cut -d' ' -f2-)

        if [[ "${post_renderer_args}" == *"-pluginver v4/"* ]]; then
            print_result "Helm v4: version detection (v4)" "pass" ""
        else
            print_result "Helm v4: version detection (v4)" "fail" "Expected args to contain '-pluginver v4/', got: ${post_renderer_args}"
        fi

        # Check that helm template command is preserved
        if grep -q "HELM_ARGS:.*template.*test-release" "${helm_output_file}"; then
            print_result "Helm v4: helm command forwarding" "pass" ""
        else
            print_result "Helm v4: helm command forwarding" "fail" "Expected helm template command to be forwarded"
        fi

        # Check validation passed
        if grep -q "VALIDATION: PASS" "${helm_output_file}"; then
            print_result "Helm v4: post-renderer setup" "pass" ""
        else
            print_result "Helm v4: post-renderer setup" "fail" "Post-renderer flags validation failed"
        fi

        if [[ ! -f "${output_manifest}" ]]; then
            print_result "Helm v4: post-renderer output" "fail" "Expected output file at path: ${output_manifest}"
        fi

        if cmp -s "${expected_file}" "${output_manifest}"; then
            print_result "Helm v4: post-renderer compare expected with output" "pass" ""
        else
            print_result "Helm v4: post-renderer compare expected with output" "fail" "Expected (left) differs from output (right)"
            diff --color=always -u "${expected_file}" "${output_manifest}" || true
        fi
    else
        print_result "Helm v4: command execution" "fail" "Output file not created"
    fi

    rm "${helm_output_file}"
}

# Test rendered chart output against reference
test_chart_rendering() {
    echo -e "\n${YELLOW}Running Chart Rendering Tests${NC}"

    # This test would require the actual helm-plugin binary to run
    # For now, we'll create a placeholder test

    if [ ! -x "${REPO_ROOT}/bin/helm-plugin" ]; then
        make build
    fi

    print_result "helm-plugin binary exists" "pass" ""

    # Test that binary can be executed
    if "${REPO_ROOT}/bin/helm-plugin" --help >/dev/null 2>&1; then
        print_result "helm-plugin binary executable" "pass" ""
    else
        print_result "helm-plugin binary executable" "fail" "Binary failed to execute"
    fi
}

# Main test execution
main() {
    echo -e "${YELLOW}=== Helm Datadog Integration Tests ===${NC}\n"

    setup

    test_helm_v3_integration "deployment.yaml" "deployment-v3.yaml"
    test_helm_v3_integration "deployment-a.yaml" "deployment-v3-a.yaml"

    test_helm_v4_integration "deployment.yaml" "deployment-v4.yaml"
    test_helm_v4_integration "deployment-a.yaml" "deployment-v4-a.yaml"

    test_chart_rendering

    # Print summary
    echo -e "\n${YELLOW}=== Test Summary ===${NC}"
    echo -e "Tests passed: ${GREEN}${TESTS_PASSED}${NC}"
    echo -e "Tests failed: ${RED}${TESTS_FAILED}${NC}"

    if [ ${TESTS_FAILED} -gt 0 ]; then
        exit 1
    fi

    exit 0
}

# Run tests
main "$@"
