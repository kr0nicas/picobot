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

# ---- Runtime Stage ----
FROM alpine:3.20

# 1. Install system dependencies (Adding build-base for some Python C-extensions)
RUN apk add --no-cache \
    ca-certificates tzdata bash curl wget \
    jq git openssh-client python3 py3-pip py3-requests \
    bind-tools iputils iproute2 net-tools build-base python3-dev

# 2. Create non-root user FIRST
RUN addgroup -S picobot && adduser -S picobot -G picobot

# 3. Setup Python Environment and permissions
# We install supabase globally BUT ensure the site-packages are readable by everyone
RUN set -eux; \
    pip3 install --no-cache-dir --break-system-packages \
        supabase psutil postgrest \
        feedparser beautifulsoup4 requests lxml pandas; \
    # Ensure site-packages are readable by the picobot user
    find /usr/lib/python3* -type d -exec chmod 755 {} +; \
    find /usr/lib/python3* -type f -exec chmod 644 {} +

# 4. Picobot home directory (config + workspace live here)
ENV PICOBOT_HOME=/home/picobot/.picobot
# Adding local user bin to PATH so 'pip install --user' tools work
ENV PATH="/home/picobot/.picobot/bin:/home/picobot/.local/bin:${PATH}"

RUN mkdir -p ${PICOBOT_HOME}/bin && chown -R picobot:picobot /home/picobot

COPY --from=builder /picobot /usr/local/bin/picobot
RUN chmod +x /usr/local/bin/picobot

USER picobot
WORKDIR /home/picobot

# 5. Allow picobot user to install its own tools without sudo
RUN pip3 config --user set global.break-system-packages true

COPY --chown=picobot:picobot docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]
