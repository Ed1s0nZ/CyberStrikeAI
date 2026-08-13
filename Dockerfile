# Build stage for Go binary
FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o cyberstrike-ai cmd/server/main.go

# Final runtime stage
FROM debian:bookworm-slim
WORKDIR /app

# Install runtime dependencies (Python3, Pip, Git, Curl, Nmap, Ca-certificates)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    nmap \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Copy python requirements and install them in a virtualenv
COPY requirements.txt .
RUN python3 -m venv /app/venv && \
    /app/venv/bin/pip install --upgrade pip && \
    /app/venv/bin/pip install -r requirements.txt

# Add venv to PATH so that python commands run in the environment
ENV PATH="/app/venv/bin:$PATH"

# Copy built application and resources
COPY --from=builder /app/cyberstrike-ai .
COPY config.yaml .
COPY web/ ./web/
COPY tools/ ./tools/
COPY agents/ ./agents/
COPY roles/ ./roles/
COPY plugins/ ./plugins/
COPY skills/ ./skills/
COPY knowledge_base/ ./knowledge_base/

# Ensure data directory exists
RUN mkdir -p data

# Expose port (default 8080)
EXPOSE 8080

# Command to run CyberStrikeAI
ENTRYPOINT ["./cyberstrike-ai", "-config", "config.yaml"]
