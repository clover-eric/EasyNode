# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

EasyNode is a single-binary VPN node management panel written in Go. It manages sing-box proxy protocols (VLESS Reality, Shadowsocks, Hysteria2, Trojan-TLS, VLESS-WS-TLS, TUIC) with auto-detection, certificate automation, and chain proxy support. The frontend is a pre-built SPA embedded via `go:embed`.

## Build & Test Commands

```bash
# Run all tests
go test ./...

# Build (Windows dev binary, no frontend rebuild needed — static assets already in cmd/easynode/dist/)
go build -o dist/easynode.exe ./cmd/easynode

# Cross-compile release binaries
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/easynode-linux-amd64 ./cmd/easynode
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/easynode-linux-arm64 ./cmd/easynode

# Run locally (listens :8088 HTTP, :8443 HTTPS)
go run ./cmd/easynode --data data
```

## Architecture

**Entry point**: `cmd/easynode/main.go` — embeds `cmd/easynode/dist/` as static assets, starts HTTP + HTTPS servers.

**Persistence**: JSON file store (`internal/store/store.go`). All state lives in a single `model.AppState` struct serialized to `data/state.json`. No SQL database — the store uses a mutex + atomic file write pattern.

**API layer** (`internal/api/server.go`): Single-file HTTP server using `net/http` stdlib mux. Auth is cookie-based session token. Routes are registered in `Server.routes()`.

**Core packages** (`internal/core/`):
- `singbox/` — generates sing-box JSON config from node list, manages `systemctl restart easynode-singbox`
- `subscribe/` — generates V2rayN (base64) and Clash (YAML) subscription output from nodes
- `recommender/` — returns protocol recommendations based on detected environment
- `detector/` — probes network environment (public IP, UDP, latency, IP purity)
- `cert/` — ACME/Let's Encrypt certificate automation
- `chain/` — pairing code generation/validation for chain proxy (entry/exit node linking)
- `traffic/` — reads per-port traffic bytes from iptables

**Key design decisions**:
- No external dependencies beyond `github.com/skip2/go-qrcode`. Pure stdlib HTTP, no framework.
- sing-box config is regenerated and service restarted on every node state change.
- Chain proxy uses ENPAIR-prefixed base64 bundles containing endpoint + outbound link for cross-server pairing.
- "clash" protocol is a virtual node (no sing-box inbound) that generates a Clash subscription profile. It auto-adds a shadowsocks node as its backing protocol.

## Protocol Development Rule

All user-facing protocols must go through the protocol library. A new protocol must appear in `recommender.Recommend()`, support add/remove via the nodes API, generate a subscribe link in `subscribe.Link()`, produce a sing-box inbound in `singbox.inbound()`, and render in the Clash output if applicable. Virtual protocols (like "clash") that are subscription formats rather than sing-box inbounds are modeled as nodes with no inbound generation.

## CI

GitHub Actions workflow (`.github/workflows/release.yml`): runs `go test ./...` then builds linux/amd64 + linux/arm64 binaries on tag push or manual dispatch.
