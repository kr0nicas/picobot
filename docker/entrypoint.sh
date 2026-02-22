#!/bin/bash
set -e

PICOBOT_HOME="${PICOBOT_HOME:-/home/picobot/.picobot}"

# If running as root, fix volume permissions and re-exec as picobot
if [ "$(id -u)" = "0" ]; then
  mkdir -p "${PICOBOT_HOME}/bin" "${PICOBOT_HOME}/workspace" /home/picobot/.cache/uv
  chown -R picobot:picobot /home/picobot
  exec gosu picobot "$0" "$@"
fi

# --- From here on we are picobot ---

# Auto-onboard if config doesn't exist yet
if [ ! -f "${PICOBOT_HOME}/config.json" ]; then
  echo "First run detected — running onboard..."
  picobot onboard
  echo "✅ Onboard complete. Config at ${PICOBOT_HOME}/config.json"
  echo ""
  echo "⚠️  You need to configure your API key and model."
  echo "   Mount a config file or set environment variables."
  echo ""
fi

# Allow overriding config values via environment variables
if [ -n "${OPENAI_API_KEY}" ]; then
  echo "Applying OPENAI_API_KEY from environment..."
  TMP=$(mktemp)
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|sk-or-v1-REPLACE_ME|${OPENAI_API_KEY}|g" > "$TMP" && \
    mv "$TMP" "${PICOBOT_HOME}/config.json"
fi

if [ -n "${OPENAI_API_BASE}" ]; then
  echo "Applying OPENAI_API_BASE from environment..."
  TMP=$(mktemp)
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|https://openrouter.ai/api/v1|${OPENAI_API_BASE}|g" > "$TMP" && \
    mv "$TMP" "${PICOBOT_HOME}/config.json"
fi

if [ -n "${TELEGRAM_BOT_TOKEN}" ]; then
  echo "Applying TELEGRAM_BOT_TOKEN from environment..."
  TMP=$(mktemp)
  cat "${PICOBOT_HOME}/config.json" | \
    sed 's|"enabled": false|"enabled": true|g' | \
    sed "s|\"token\": \"\"|\"token\": \"${TELEGRAM_BOT_TOKEN}\"|g" > "$TMP" && \
    mv "$TMP" "${PICOBOT_HOME}/config.json"
fi

if [ -n "${TELEGRAM_ALLOW_FROM}" ]; then
  echo "Applying TELEGRAM_ALLOW_FROM from environment..."
  TMP=$(mktemp)
  ALLOW_JSON=$(echo "${TELEGRAM_ALLOW_FROM}" | sed 's/,/","/g' | sed 's/^/["/' | sed 's/$/"]/')
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s/\"allowFrom\": null/\"allowFrom\": ${ALLOW_JSON}/g" | \
    sed "s/\"allowFrom\": \[\]/\"allowFrom\": ${ALLOW_JSON}/g" > "$TMP" && \
    mv "$TMP" "${PICOBOT_HOME}/config.json"
fi

if [ -n "${PICOBOT_MODEL}" ]; then
  echo "Applying PICOBOT_MODEL from environment..."
  TMP=$(mktemp)
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|\"model\": \"stub-model\"|\"model\": \"${PICOBOT_MODEL}\"|g" > "$TMP" && \
    mv "$TMP" "${PICOBOT_HOME}/config.json"
fi

echo "Starting picobot $@..."
exec picobot "$@"
