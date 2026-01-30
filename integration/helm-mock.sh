#!/usr/bin/env bash
# Mock helm that captures arguments and validates post-renderer setup

set -eu
# set -x

# TEST_MANIFEST should point to a file helm needs to output to stdout
TEST_MANIFEST=${TEST_MANIFEST:?"should point to valid manifest file"}
OUTPUT_FILE=${OUTPUT_FILE:?"must be set to a valid name"}

# Save all arguments
echo "HELM_ARGS: $*" >>"${OUTPUT_FILE}"

if [[ -z ${TEST_MANIFEST} ]]; then
    echo "VALIDATION: FAIL - TEST_MANIFEST should pointing to existing file" >>"${OUTPUT_FILE}"
    exit 1
fi

if [[ ! -f ${TEST_MANIFEST} ]]; then
    echo "VALIDATION: FAIL - TEST_MANIFEST points to non-existing file" >>"${OUTPUT_FILE}"
    exit 1
fi

# Check for post-renderer flags
has_post_renderer=false
has_post_renderer_args=false
post_renderer_bin=""
post_renderer_args=()

while [ $# -gt 0 ]; do
    case "$1" in
    --post-renderer)
        has_post_renderer=true
        post_renderer_bin="$2"
        shift 2
        ;;
    --post-renderer-args)
        has_post_renderer_args=true
        # Collect the next argument as a value for --post-renderer-args
        if [ $# -gt 1 ]; then
            post_renderer_args+=("$2")
            shift 2
        else
            shift 1
        fi
        ;;
    *)
        shift
        ;;
    esac
done

echo "POST_RENDERER_BIN: ${post_renderer_bin}" >>"${OUTPUT_FILE}"
echo "POST_RENDERER_ARGS: ${post_renderer_args[*]}" >>"${OUTPUT_FILE}"

if [ "$has_post_renderer" = true ] && [ "$has_post_renderer_args" = true ]; then
    echo "VALIDATION: PASS" >>"${OUTPUT_FILE}"
else
    echo "VALIDATION: FAIL - Missing post-renderer flags" >>"${OUTPUT_FILE}"
    exit 1
fi

# Simulate what real helm does: pipe manifest through post-renderer with collected args
cat "${TEST_MANIFEST}" | "${post_renderer_bin}" "${post_renderer_args[@]}"

exit 0
