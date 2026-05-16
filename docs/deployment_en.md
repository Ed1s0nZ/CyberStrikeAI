# Docker Deployment

## Quick Start

Build from source:

```bash
docker build -t cyberstrikeai:local .
```

Run with local config and data mounts:

```bash
docker run -d \
  --name cyberstrikeai \
  -p 8080:8080 \
  -p 8081:8081 \
  -v "$(pwd)/.docker-runtime:/app/runtime-config" \
  -v "$(pwd)/data:/app/data" \
  -v "$(pwd)/tmp:/app/tmp" \
  -v "$(pwd)/knowledge_base:/app/knowledge_base" \
  cyberstrikeai:local
```

Run with Compose:

```bash
docker compose up -d --build
```

The bundled `docker-compose.yml` builds from the checked-out source tree, which is the best fit for source-based deployments and local customization.

## GHCR Image

GitHub Actions publishes images to GHCR:

```bash
docker pull ghcr.io/ed1s0nz/cyberstrikeai:latest
```

If you prefer the published image, replace `cyberstrikeai:local` in the `docker run` example above with a GHCR tag, or create your own compose file that uses `image:` instead of `build:`.

## Persistent Paths

- `/app/runtime-config/config.yaml`: Docker runtime config file; auto-generated on first start from the bundled `config.docker.yaml` template
- `/app/data`: SQLite databases
- `/app/tmp`: large result files and temp output
- `/app/knowledge_base`: knowledge-base files

The app still reads `/app/config.yaml` inside the container, but the Docker image links that path to `/app/runtime-config/config.yaml` so Docker deployments do not write secrets back into the repository root `config.yaml`.

## Privileges and Capabilities

The image runs as `root` by default to keep more tools usable. Most features do not need `--privileged`, but some raw-socket or advanced scan modes need extra capabilities, for example:

```bash
docker run ... --cap-add NET_ADMIN --cap-add NET_RAW cyberstrikeai:local
```

The bundled `docker-compose.yml` includes commented `cap_add` lines you can enable when needed.

## Preinstalled Tools

The image tries to preinstall a high-frequency tool set, including:

- Go tools: `httpx`, `nuclei`, `subfinder`, `ffuf`, `gobuster`, `dalfox`
- APT tools: `nmap`, `sqlmap`, `nikto`, `masscan`, `john`, `gdb`, `binwalk`, `steghide`
- Python / Ruby tools: `checkov`, `volatility3`, `wafw00f`, `wpscan`, plus everything from `requirements.txt`

Architecture support is best effort. If a tool cannot be installed reliably on the current distro or on `arm64`, the build skips it and continues.

## Upgrades

Container deployments do not use `run.sh` or `upgrade.sh`.

For source + Compose deployments:

```bash
git pull
docker compose up -d --build
```

For GHCR image deployments:

```bash
docker pull ghcr.io/ed1s0nz/cyberstrikeai:latest
docker rm -f cyberstrikeai
docker run -d --name cyberstrikeai -p 8080:8080 -p 8081:8081 -v "$(pwd)/.docker-runtime:/app/runtime-config" -v "$(pwd)/data:/app/data" -v "$(pwd)/tmp:/app/tmp" -v "$(pwd)/knowledge_base:/app/knowledge_base" ghcr.io/ed1s0nz/cyberstrikeai:latest
```
