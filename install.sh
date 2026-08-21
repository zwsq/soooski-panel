#!/bin/bash
# One-line installer. Puts the soooski CLI on PATH and opens the numbered menu.
#
#   curl -fsSL https://raw.githubusercontent.com/zwsq/soooski-panel/release/install.sh | sudo bash
#
# Optional flags after bash -s -- skip the menu and install unattended:
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

if [[ $# -eq 0 ]]; then
  exec /usr/local/bin/soooski
fi
if [[ "$1" == -* ]]; then
  exec /usr/local/bin/soooski install "$@"
fi
exec /usr/local/bin/soooski "$@"
