#!/usr/bin/env bash

set -euo pipefail

DEST_DIR="${1:-/out/bin}"

mkdir -p "${DEST_DIR}"

export GOBIN="${DEST_DIR}"

go install github.com/projectdiscovery/httpx/cmd/httpx@v1.9.0
go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@v3.8.0
go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@v2.14.0
go install github.com/ffuf/ffuf/v2@v2.1.0
go install github.com/OJ/gobuster/v3@v3.8.2
go install github.com/hahwul/dalfox/v2@v2.12.0
