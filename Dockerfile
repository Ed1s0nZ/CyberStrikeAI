# syntax=docker/dockerfile:1.7

FROM golang:1.24-bookworm AS app-builder

WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /out/cyberstrike-ai ./cmd/server/main.go

FROM golang:1.25-bookworm AS tools-builder

ARG TARGETARCH

WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && rm -rf /var/lib/apt/lists/*

COPY scripts/docker/install-go-tools.sh /usr/local/bin/install-go-tools.sh

RUN chmod +x /usr/local/bin/install-go-tools.sh \
    && /usr/local/bin/install-go-tools.sh /out/bin

FROM debian:bookworm-slim AS runtime

ENV APP_HOME=/app \
    PATH=/opt/cyberstrike/bin:$PATH \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONUNBUFFERED=1

WORKDIR ${APP_HOME}

COPY requirements.txt ./requirements.txt
COPY scripts/docker/install-system-tools.sh /usr/local/bin/install-system-tools.sh
COPY scripts/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/install-system-tools.sh /usr/local/bin/docker-entrypoint.sh \
    && /usr/local/bin/install-system-tools.sh \
    && rm -f /usr/local/bin/install-system-tools.sh

COPY --from=app-builder /out/cyberstrike-ai ./cyberstrike-ai
COPY --from=tools-builder /out/bin/ /opt/cyberstrike/bin/

COPY web ./web
COPY tools ./tools
COPY roles ./roles
COPY skills ./skills
COPY agents ./agents
COPY knowledge_base ./knowledge_base
COPY config.docker.yaml ./config.example.yaml

RUN mkdir -p runtime-config data tmp \
    && ln -s /app/runtime-config/config.yaml /app/config.yaml

EXPOSE 8080 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=5 \
    CMD curl -fsS http://127.0.0.1:8080/ >/dev/null || exit 1

ENTRYPOINT ["tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
CMD ["./cyberstrike-ai"]
