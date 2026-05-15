#!/usr/bin/env bash
set -euo pipefail

APP_NAME="easynode"
BIN="/usr/local/bin/easynode"
DATA_DIR="/var/lib/easynode"
BACKUP_DIR="/var/lib/easynode-uninstall-$(date +%Y%m%d-%H%M%S)"

if [ "$(id -u)" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

KEEP_DATA="yes"
if [ "${1:-}" = "--purge" ]; then
  KEEP_DATA="no"
fi

systemctl stop easynode 2>/dev/null || true
systemctl disable easynode 2>/dev/null || true
systemctl stop easynode-singbox 2>/dev/null || true
systemctl disable easynode-singbox 2>/dev/null || true

rm -f /etc/systemd/system/easynode.service
rm -f /etc/systemd/system/easynode-singbox.service
systemctl daemon-reload
rm -f "$BIN"

if [ -d "$DATA_DIR" ]; then
  if [ "$KEEP_DATA" = "yes" ]; then
    mv "$DATA_DIR" "$BACKUP_DIR"
    echo "Data backed up to: $BACKUP_DIR"
  else
    rm -rf "$DATA_DIR"
    echo "Data removed"
  fi
fi

echo "EasyNode uninstalled"
