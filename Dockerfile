# syntax=docker/dockerfile:1

# ============================================================
# AnalyzPRO — multi-stage Docker build
# Builds a static linux binary and ships it in a slim image.
# ============================================================

# ---- Build stage ----
# Плавающий тег golang:1 всегда указывает на свежайший стабильный Go 1.x,
# что гарантирует совместимость с директивой `go` в go.mod. Если нужна
# воспроизводимая сборка — зафиксируйте конкретный тег (например golang:1.25),
# соответствующий go.mod.
FROM golang:1 AS build

WORKDIR /src

# Cache Go module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy sources and build a static binary for linux.
# CGO_ENABLED=0 + GOOS=linux => fully static, no libc required.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/analyzpro ./cmd/bot

# ---- Final (runtime) stage ----
# debian:stable-slim keeps a minimal glibc base and easy curl access
# for the healthcheck. Switch to alpine if you want a smaller image.
FROM debian:stable-slim

# ca-certificates: required for HTTPS calls to Telegram / AI providers.
# curl: used by the docker-compose healthcheck on /healthz.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Create a dedicated non-root user to run the bot.
RUN groupadd -r analyzpro \
    && useradd -r -g analyzpro -d /app -s /usr/sbin/nologin analyzpro

# The Mini App / dashboard assets (cmd/bot/webapp, internal/monitoring/webapp_files)
# are embedded into the binary at build time via go:embed, so only the binary
# needs to be copied. No extra webapp_files copy is required.
COPY --from=build /src/bin/analyzpro /app/analyzpro

# Pre-create the runtime directories and hand them to the non-root user.
# They are also declared as VOLUME below so the host can mount persistent data.
RUN mkdir -p /app/data /app/uploads \
    && chown -R analyzpro:analyzpro /app

USER analyzpro

EXPOSE 8080

# Persisted state must outlive container restarts:
#   /app/data    -> analyzpro.db (SQLite, реальная БД: профили, диагнозы,
#                   курсы, предпочтения и история мониторинга), agreements.json,
#                   premium_users.json, states.json, analytics.jsonl
#   /app/uploads -> user-uploaded files (UPLOAD_DIR)
# Mount these as volumes (docker run -v or docker-compose)!
VOLUME ["/app/data", "/app/uploads"]

# IMPORTANT: the bot enforces a SINGLE instance via a flock on
# /tmp/analyzpro.lock. Never run more than one replica/container.
CMD ["/app/analyzpro"]
