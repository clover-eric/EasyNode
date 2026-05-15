#!/usr/bin/env bash
set -euo pipefail

APP_NAME="easynode"
APP_DIR="/opt/easynode"
DATA_DIR="/var/lib/easynode"
BIN="/usr/local/bin/easynode"
SERVICE="/etc/systemd/system/easynode.service"
PORT="8088"
ASSUME_YES="0"
DO_UPGRADE="ask"
DO_DEPS="ask"
DO_BBR="ask"
DO_FIREWALL="ask"
DOWNLOAD_URL="${EASYNODE_BIN_URL:-}"
GITHUB_REPO="${EASYNODE_GITHUB_REPO:-}"

export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1

usage() {
  cat <<EOF
EasyNode installer

Usage:
  curl -fsSL https://example.com/install.sh | bash
  curl -fsSL https://example.com/install.sh | bash -s -- --yes --url https://example.com/easynode-linux-amd64

Options:
  --yes                 Non-interactive install with recommended defaults
  --url URL             Download EasyNode binary from URL
  --repo OWNER/REPO     Download from GitHub latest release
  --port PORT           Panel listen port, default 8088
  --data-dir PATH       Data directory, default /var/lib/easynode
  --skip-upgrade        Do not upgrade system packages
  --skip-deps           Do not install common dependencies
  --skip-bbr            Do not enable BBR
  --skip-firewall       Do not open firewall port
  --help                Show help

Environment:
  EASYNODE_BIN_URL      Same as --url
  EASYNODE_GITHUB_REPO  Same as --repo
EOF
}

log() { printf '\033[1;32m[EasyNode]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[EasyNode]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[EasyNode]\033[0m %s\n' "$*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes) ASSUME_YES="1" ;;
    --url) DOWNLOAD_URL="${2:-}"; shift ;;
    --repo) GITHUB_REPO="${2:-}"; shift ;;
    --port) PORT="${2:-}"; shift ;;
    --data-dir) DATA_DIR="${2:-}"; shift ;;
    --skip-upgrade) DO_UPGRADE="no" ;;
    --skip-deps) DO_DEPS="no" ;;
    --skip-bbr) DO_BBR="no" ;;
    --skip-firewall) DO_FIREWALL="no" ;;
    --help) usage; exit 0 ;;
    *) die "Unknown option: $1" ;;
  esac
  shift
done

if [ "$(id -u)" -ne 0 ]; then
  die "Please run as root: sudo bash install.sh"
fi

if ! command -v systemctl >/dev/null 2>&1; then
  die "systemd is required"
fi

case "$PORT" in
  ''|*[!0-9]*) die "Invalid port: $PORT" ;;
esac

detect_os() {
  if [ -r /etc/os-release ]; then
    . /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_LIKE="${ID_LIKE:-}"
  else
    OS_ID="unknown"
    OS_LIKE=""
  fi
  ARCH="$(uname -m)"
  case "$ARCH" in
    x86_64|amd64) EASYNODE_ARCH="amd64" ;;
    aarch64|arm64) EASYNODE_ARCH="arm64" ;;
    *) die "Unsupported architecture: $ARCH" ;;
  esac
}

ask() {
  question="$1"
  default="$2"
  current="$3"
  if [ "$current" = "no" ]; then
    return 1
  fi
  if [ "$ASSUME_YES" = "1" ]; then
    return 0
  fi
  suffix="[Y/n]"
  [ "$default" = "no" ] && suffix="[y/N]"
  printf "%s %s " "$question" "$suffix"
  read -r answer
  answer="${answer:-$default}"
  case "$answer" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

pkg_update() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get upgrade -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"
  elif command -v dnf >/dev/null 2>&1; then
    dnf upgrade -y
  elif command -v yum >/dev/null 2>&1; then
    yum update -y
  else
    warn "No supported package manager found, skip system upgrade"
  fi
}

pkg_deps() {
  pkgs="curl ca-certificates tar gzip git make"
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" $pkgs
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y $pkgs
  elif command -v yum >/dev/null 2>&1; then
    yum install -y $pkgs
  else
    warn "No supported package manager found, skip dependencies"
  fi
}

enable_bbr() {
  modprobe tcp_bbr 2>/dev/null || true
  cat >/etc/sysctl.d/99-easynode-bbr.conf <<EOF
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
EOF
  sysctl --system >/dev/null || true
  if sysctl net.ipv4.tcp_congestion_control 2>/dev/null | grep -q bbr; then
    log "BBR enabled"
  else
    warn "BBR not available on this kernel"
  fi
}

open_firewall() {
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qi active; then
    ufw allow "${PORT}/tcp" || true
  elif command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
    firewall-cmd --permanent --add-port="${PORT}/tcp" || true
    firewall-cmd --reload || true
  else
    warn "No active ufw/firewalld detected, skip firewall rule"
  fi
}

install_binary() {
  install -d -m 755 "$APP_DIR"
  install -d -m 700 "$DATA_DIR"

  if [ -z "$DOWNLOAD_URL" ] && [ -n "$GITHUB_REPO" ]; then
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/easynode-linux-${EASYNODE_ARCH}"
  fi

  if [ -n "$DOWNLOAD_URL" ]; then
    log "Downloading EasyNode binary"
    tmp="$(mktemp)"
    if curl -fL "$DOWNLOAD_URL" -o "$tmp"; then
      install -m 755 "$tmp" "$BIN"
      rm -f "$tmp"
      return
    fi
    rm -f "$tmp"
    if [ -z "$GITHUB_REPO" ]; then
      die "Binary download failed: $DOWNLOAD_URL"
    fi
    warn "Release binary not found, falling back to source build"
  elif [ -f ./dist/easynode ]; then
    install -m 755 ./dist/easynode "$BIN"
    return
  elif [ -f ./dist/easynode-linux-"$EASYNODE_ARCH" ]; then
    install -m 755 ./dist/easynode-linux-"$EASYNODE_ARCH" "$BIN"
    return
  else
    if [ -z "$GITHUB_REPO" ]; then
      die "No binary found. Pass --url URL, --repo OWNER/REPO, or run make build-linux-$EASYNODE_ARCH first."
    fi
  fi

  build_from_source
}

install_go_if_missing() {
  if command -v go >/dev/null 2>&1; then
    return
  fi
  log "Installing Go toolchain"
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" golang-go
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y golang
  elif command -v yum >/dev/null 2>&1; then
    yum install -y golang
  else
    die "Go is not installed and no supported package manager found"
  fi
}

build_from_source() {
  install_go_if_missing
  command -v git >/dev/null 2>&1 || die "git is required for source build"
  command -v make >/dev/null 2>&1 || die "make is required for source build"

  src="$(mktemp -d)"
  log "Cloning source from https://github.com/${GITHUB_REPO}.git"
  git clone --depth 1 "https://github.com/${GITHUB_REPO}.git" "$src"
  log "Building EasyNode from source"
  (cd "$src" && go build -o "$src/easynode" ./cmd/easynode)
  install -m 755 "$src/easynode" "$BIN"
  rm -rf "$src"
}

write_service() {
  cat >"$SERVICE" <<EOF
[Unit]
Description=EasyNode intelligent proxy panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN -addr :$PORT -data $DATA_DIR
Restart=always
RestartSec=3
User=root
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=$DATA_DIR
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now "$APP_NAME"
}

wait_for_panel() {
  for _ in $(seq 1 20); do
    if curl -fsS --max-time 2 "http://127.0.0.1:${PORT}/api/v1/setup/status" >/tmp/easynode-setup-status.json 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  warn "EasyNode service did not become ready"
  systemctl status easynode --no-pager -l || true
  journalctl -u easynode -n 80 --no-pager || true
  return 1
}

public_ip() {
  curl -fsS --max-time 3 https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || echo "SERVER_IP"
}

detect_os

cat <<EOF
============================================================
 EasyNode one-line installer
 OS: ${OS_ID} ${OS_LIKE}
 Arch: ${EASYNODE_ARCH}
 Port: ${PORT}
 Data: ${DATA_DIR}
============================================================
EOF

if ask "Upgrade system packages?" "yes" "$DO_UPGRADE"; then
  log "Upgrading system"
  pkg_update
fi

if ask "Install common dependencies?" "yes" "$DO_DEPS"; then
  log "Installing dependencies"
  pkg_deps
fi

if ask "Enable BBR network acceleration?" "yes" "$DO_BBR"; then
  enable_bbr
fi

install_binary
write_service
wait_for_panel

if ask "Open firewall port ${PORT}/tcp if firewall is active?" "yes" "$DO_FIREWALL"; then
  open_firewall
fi

IP="$(public_ip)"

cat <<EOF

EasyNode installed.

Open this in your browser:
  http://${IP}:${PORT}

Useful commands:
  systemctl status easynode
  journalctl -u easynode -f
  systemctl restart easynode

Next:
  1. Open the panel URL.
  2. Set admin password.
  3. Enter domain or choose IP direct mode.
  4. Keep recommended node plans unless you know what to change.

If the browser cannot open it, check your cloud firewall/security group allows TCP ${PORT}.
EOF
