#!/usr/bin/env bash

set -euf

version() {
    grep version "${SCRIPT_DIR}/../plugin.yaml" | cut -d'"' -f2
}
