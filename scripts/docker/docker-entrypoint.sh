#!/usr/bin/env bash

set -Eeuo pipefail

template_config="${CYBERSTRIKE_TEMPLATE_CONFIG:-/app/config.example.yaml}"
runtime_config_dir="${CYBERSTRIKE_RUNTIME_CONFIG_DIR:-/app/runtime-config}"
runtime_config_path="${CYBERSTRIKE_RUNTIME_CONFIG_PATH:-${runtime_config_dir}/config.yaml}"

mkdir -p "${runtime_config_dir}"

if [[ ! -f "${runtime_config_path}" ]]; then
    cp "${template_config}" "${runtime_config_path}"
    printf '[docker-entrypoint] initialized runtime config at %s\n' "${runtime_config_path}"
fi

exec "$@"
