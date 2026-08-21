#!/bin/bash
# One-line installer. Downloads the soooski CLI and runs `soooski install`.
#
#   curl -fsSL https://raw.githubusercontent.com/zwsq/soooski-panel/release/install.sh | sudo bash
#
# Optional flags after bash -s -- :
#   --host vpn.example.com --email you@example.com --user admin --password '...'
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  echo "soooski: re-run as root: curl -fsSL https://raw.githubusercontent.com/zwsq/soooski-panel/release/install.sh | sudo bash" >&2
  exit 1
fi

SOOOSKI_REPO="${SOOOSKI_REPO:-https://raw.githubusercontent.com/zwsq/soooski-panel}"
SOOOSKI_REF="${SOOOSKI_REF:-release}"
RAW="${SOOOSKI_REPO}/${SOOOSKI_REF}"

if ! command -v curl >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y curl ca-certificates
  elif command -v yum >/dev/null 2>&1; then
    yum install -y curl ca-certificates
  else
    echo "soooski: install curl first" >&2
    exit 1
  fi
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
curl -fsSL "$RAW/scripts/soooski" -o "$tmp"
chmod 0755 "$tmp"
install -m 0755 "$tmp" /usr/local/bin/soooski
exec /usr/local/bin/soooski install "$@"
