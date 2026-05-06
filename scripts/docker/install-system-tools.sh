#!/usr/bin/env bash

set -Eeuo pipefail

export DEBIAN_FRONTEND=noninteractive

log() {
    printf '[docker-tools] %s\n' "$*"
}

warn() {
    printf '[docker-tools][warn] %s\n' "$*" >&2
}

apt_install_optional() {
    local package="$1"

    if apt-cache show "${package}" >/dev/null 2>&1; then
        if ! apt-get install -y --no-install-recommends "${package}"; then
            warn "failed to install optional apt package: ${package}"
        fi
    else
        warn "apt package not available on this platform: ${package}"
    fi
}

pip_install_optional() {
    if ! python3 -m pip install --break-system-packages --no-cache-dir "$@"; then
        warn "failed to install optional Python packages: $*"
    fi
}

gem_install_optional() {
    if ! gem install --no-document "$@"; then
        warn "failed to install optional Ruby gems: $*"
    fi
}

log "installing base runtime packages"
apt-get update
apt-get install -y --no-install-recommends \
    bash \
    build-essential \
    ca-certificates \
    curl \
    dnsutils \
    file \
    git \
    iputils-ping \
    jq \
    netcat-openbsd \
    python3 \
    python3-pip \
    python3-venv \
    ruby-full \
    sqlite3 \
    tini \
    unzip \
    wget \
    xz-utils

log "installing best-effort security packages from apt"
for package in \
    amass \
    binwalk \
    gdb \
    hashcat \
    john \
    libimage-exiftool-perl \
    masscan \
    nikto \
    nmap \
    radare2 \
    sqlmap \
    steghide
do
    apt_install_optional "${package}"
done

log "installing Python dependencies used by bundled tools"
python3 -m pip install --break-system-packages --no-cache-dir -r requirements.txt

log "installing extra best-effort Python tooling"
pip_install_optional \
    checkov \
    volatility3 \
    wafw00f

log "installing extra best-effort Ruby tooling"
gem_install_optional wpscan

apt-get clean
rm -rf /var/lib/apt/lists/*
