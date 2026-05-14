# Repository Guidelines

## Project Structure & Module Organization

This repository currently contains `prompt.md`, which describes the intended EasyNode architecture. Treat it as source of truth until implementation files are added.

Planned layout:

- `cmd/easynode/`: Go application entry point.
- `internal/api/`: HTTP routes, middleware, handlers, DTOs.
- `internal/core/`: sing-box, detection, recommendation, certificate, chain proxy, subscription logic.
- `internal/model/` and `internal/store/`: SQLite models and persistence.
- `internal/util/`: shared helpers for crypto, network, and system tasks.
- `web/`: React 18 + TypeScript frontend using Vite, Tailwind CSS, and shadcn/ui.
- `scripts/`: deployment and install scripts.
- `dist/`: build outputs; avoid committing generated binaries.

## Build, Test, and Development Commands

Use these commands once planned Go/web structure exists:

- `make build-web`: install frontend dependencies and build React app.
- `make build`: build frontend, embed assets, and produce `dist/easynode`.
- `make build-linux-amd64`: cross-compile Linux amd64 release binary.
- `make build-linux-arm64`: cross-compile Linux arm64 release binary.
- `make dev`: run frontend dev server plus Go hot reload.
- `go test ./...`: run backend unit tests.
- `cd web && pnpm test`: run frontend tests when configured.

## Coding Style & Naming Conventions

Use `gofmt`/`go fmt ./...` for Go. Keep packages lowercase and focused: `api`, `store`, `singbox`, `recommender`. Use clear handler names such as `GetNodes`, `ToggleNode`, and `GenerateSubscribeLink`.

Frontend code should use TypeScript, functional React components, and PascalCase component files such as `NodeCard.tsx`. Hooks belong in `web/src/hooks/` and use `useXxx` naming. Keep API types explicit.

## Testing Guidelines

Backend tests should live beside source files as `*_test.go`. Prefer table-driven tests for recommendation logic, config generation, and store behavior. Frontend tests should use `*.test.ts` or `*.test.tsx`.

Prioritize coverage for security-sensitive flows: login lockout, JWT validation, subscription tokens, ACME/certificate paths, sing-box config generation, and chain pairing.

## Commit & Pull Request Guidelines

No Git history is present, so no existing commit convention can be inferred. Use concise conventional commits, for example `feat: add node subscription generator` or `fix: validate pairing code expiry`.

Pull requests should include purpose, key changes, test commands run, and screenshots for UI changes. Link issues when available. Call out config, migration, port, certificate, or security-impacting changes.

## Security & Configuration Tips

Never commit generated secrets, JWT keys, ACME private keys, SQLite runtime data, or sing-box credentials. Keep deployment defaults conservative: random panel path, HTTPS enabled, and restrictive file permissions for data under `/var/lib/easynode/`.
