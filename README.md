# EasyNode

EasyNode is an intelligent proxy node panel for regular users. It hides protocol complexity behind a simple setup flow: enter a domain, keep the recommended plans, then copy the subscription into a client.

## Features

- Single Go binary with embedded web UI
- Chinese / English interface
- First-run setup wizard
- Recommended node plans: VLESS Reality, Hysteria2, Trojan TLS, VLESS WS TLS, TUIC
- Human-readable protocol guidance: score, use case, compatible clients, tradeoffs
- Subscription link generation
- sing-box config generation
- Chain proxy pairing code flow
- Settings page for password, domain, IP direct mode, and panel path
- JSON persistence under the data directory

## One-Line Install

After publishing release binaries, users can install on a Linux server with one command:

```bash
curl -fsSL https://your-domain.com/install.sh | bash -s -- --yes --url https://your-domain.com/easynode-linux-amd64
```

If hosted on GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/scripts/install.sh | bash -s -- --yes --repo OWNER/REPO
```

Interactive install:

```bash
curl -fsSL https://your-domain.com/install.sh | bash
```

The installer can:

- upgrade system packages
- install common dependencies
- enable BBR acceleration
- create data directory with restrictive permissions
- install systemd service
- open the panel port when `ufw` or `firewalld` is active
- print the panel URL and next steps

Useful installer options:

```bash
bash scripts/install.sh --yes --url https://your-domain.com/easynode-linux-amd64
bash scripts/install.sh --port 8088 --skip-upgrade --skip-bbr
bash scripts/install.sh --data-dir /var/lib/easynode
```

## Local Run

```bash
go run ./cmd/easynode -addr :8088 -data data
```

Open:

```text
http://127.0.0.1:8088
```

## Build

```bash
make build
make build-linux-amd64
make build-linux-arm64
```

Windows local binary:

```text
dist/easynode.exe
```

Linux release binaries:

```text
dist/easynode-linux-amd64
dist/easynode-linux-arm64
```

## Production Gaps

Current version is a working MVP. Production still needs:

- ACME certificate automation
- real sing-box process management
- real DNS/IP/port/UDP probing
- SQLite storage
- cross-server pairing handshake
- signed release checksums
