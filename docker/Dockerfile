# syntax=docker/dockerfile:1.7

# ---- Build Stage ----
ARG BUILDPLATFORM=linux/amd64
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN set -eux; \
    GOOS="${TARGETOS:-linux}"; \
    GOARCH="${TARGETARCH:-amd64}"; \
    if [ "$GOARCH" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then \
        GOARM="${TARGETVARIANT#v}"; \
    fi; \
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" ${GOARM:+GOARM=$GOARM} \
    go build -ldflags="-s -w" -o /picobot ./cmd/picobot

# ---- Runtime Stage (Debian Slim — glibc, compatible con wheels Python) ----
FROM python:3.12-slim-bookworm

# 1. Herramientas de sistema + gosu para drop de privilegios
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata bash curl wget jq git openssh-client \
    dnsutils iputils-ping iproute2 net-tools procps \
    build-essential libffi-dev gosu \
    && rm -rf /var/lib/apt/lists/*

# 2. UV
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# 3. Usuario no-root (sin -r para que -m cree el home)
RUN groupadd picobot && useradd -g picobot -m -s /bin/bash picobot

# 4. Configuración de entorno
ENV PICOBOT_HOME=/home/picobot/.picobot
ENV UV_CACHE_DIR=/home/picobot/.cache/uv
ENV VIRTUAL_ENV=/home/picobot/venv
ENV PATH="/home/picobot/venv/bin:/home/picobot/.local/bin:${PICOBOT_HOME}/bin:${PATH}"

WORKDIR /home/picobot

# 5. Venv global + dependencias core
RUN uv venv /home/picobot/venv && \
    uv pip install --no-cache \
    supabase psutil postgrest \
    feedparser beautifulsoup4 requests lxml pandas \
    httpx selectolax

# 6. Permisos
RUN mkdir -p ${PICOBOT_HOME}/bin && chown -R picobot:picobot /home/picobot

# 7. Binarios y entrypoint
COPY --from=builder /picobot /usr/local/bin/picobot
RUN chmod +x /usr/local/bin/picobot

COPY --chown=picobot:picobot docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]
