# syntax=docker/dockerfile:1.7

FROM golang:1.24-bookworm AS builder

WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/cyberstrike-ai ./cmd/server/main.go

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/cyberstrike-ai /app/cyberstrike-ai
COPY web /app/web
COPY tools /app/tools
COPY skills /app/skills
COPY roles /app/roles
COPY knowledge_base /app/knowledge_base
COPY scripts /app/scripts
COPY requirements.txt /app/requirements.txt
COPY run_docker.sh /app/run_docker.sh
COPY config.docker.yaml /app/config.docker.yaml

RUN chmod +x /app/scripts/install-enabled-tools-container.sh \
    && chmod +x /app/run_docker.sh \
    && /app/scripts/install-enabled-tools-container.sh \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app/data /app/tmp \
    && chmod +x /app/cyberstrike-ai

EXPOSE 8080 8081

ENV CYBERSTRIKE_DOCKER=true

ENTRYPOINT ["/app/cyberstrike-ai", "--config", "/app/config.docker.yaml"]
