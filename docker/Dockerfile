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
FROM python:3.12-slim

# 1. Herramientas de sistema + gosu para drop de privilegios
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata bash curl wget jq git \
    openssh-client dnsutils iputils-ping iproute2 net-tools \
    libffi-dev build-essential gosu \
    && rm -rf /var/lib/apt/lists/*

# 2. UV (fast Python package manager)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# 3. Usuario no-root
RUN groupadd picobot && useradd -g picobot -m picobot

ENV PICOBOT_HOME=/home/picobot/.picobot
ENV UV_CACHE_DIR=/home/picobot/.cache/uv
ENV PATH="/home/picobot/.picobot/bin:/home/picobot/.local/bin:${PATH}"

RUN mkdir -p ${PICOBOT_HOME}/bin && chown -R picobot:picobot /home/picobot

# 4. Binario de picobot
COPY --from=builder /picobot /usr/local/bin/picobot
RUN chmod +x /usr/local/bin/picobot

WORKDIR /home/picobot

# 5. Venv global activado por defecto
RUN uv venv /home/picobot/venv && chown -R picobot:picobot /home/picobot/venv
ENV VIRTUAL_ENV=/home/picobot/venv
ENV PATH="/home/picobot/venv/bin:$PATH"

# 6. Pre-instalar paquetes base
RUN uv pip install \
    supabase psutil postgrest \
    feedparser beautifulsoup4 requests lxml pandas \
    httpx selectolax

COPY --chown=picobot:picobot docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]
