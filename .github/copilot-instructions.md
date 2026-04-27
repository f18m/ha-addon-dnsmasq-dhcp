# Copilot Instructions

## Repository Overview

This repository contains a **Home Assistant add-on** that provides flexible DNS and DHCP servers for a home LAN. It wraps the well-known [dnsmasq](https://thekelleys.org.uk/dnsmasq/doc.html) utility and adds a real-time web UI to monitor DHCP clients (current and past).

Key components:

| Path | Language | Purpose |
|---|---|---|
| `backend/` | Go | HTTP + WebSocket server that powers the web UI; reads dnsmasq lease files and an SQLite tracker DB |
| `frontend/` | HTML / SCSS / JS | Templated single-page app rendered server-side by the Go backend |
| `rootfs/` | Shell / s6-overlay / tempio | Files baked into the Docker image: init scripts, dnsmasq config templates, nginx reverse-proxy config |
| `Dockerfile` | Docker | Multi-stage build: Go binary + frontend assets + dnsmasq + s6-overlay |
| `config.yaml` | YAML | Home Assistant add-on manifest (version, schema, options) |
| `Makefile` | Make | Developer workflow targets (see below) |

## Architecture

- **Init system**: [s6-overlay](https://github.com/just-containers/s6-overlay) manages three services inside the container: `dnsmasq`, the Go web-UI backend, and `nginx`.
- **Config templating**: [tempio](https://github.com/home-assistant/tempio) renders Go-template-based config files (e.g. `rootfs/usr/share/tempio/dnsmasq.config`) at container startup, injecting values from the HA options JSON (`/data/options.json`).
- **DHCP event hook**: `rootfs/opt/bin/dnsmasq-dhcp-script.sh` is called by dnsmasq on every DHCP event and forwards the event to the Go backend via a Unix socket.
- **Real-time UI**: The Go backend pushes updates to all connected browsers over WebSocket whenever a lease file or DB change is detected.
- **Tracker DB**: SQLite database (`/data/trackerdb.sqlite3`) keeps a persistent history of every DHCP client ever seen.

## Building

### Backend (Go)

```bash
cd backend
go build -o bin/backend .
```

### Frontend (SCSS → CSS)

Install dart-sass once with `make install-dart-sass`, then:

```bash
make build-frontend
```

### Full Docker image

```bash
# build both amd64 + aarch64
make build-docker-image

# build amd64 only (faster)
make FAST=1 build-docker-image
```

## Running Tests

### Unit tests (Go)

```bash
cd backend
go test -v ./...
```

The tests require `dnsmasq` to be installed on the host (`sudo apt install -y dnsmasq` on Debian/Ubuntu). This mirrors the CI setup.

## Linting

### Go

```bash
cd backend
golangci-lint run          # check only
golangci-lint run --fix    # auto-fix where possible
go fmt ./...               # standard formatting
```

The linter configuration lives in `backend/.golangci.yaml`. The enabled linter set is strict: `errcheck`, `gosec`, `staticcheck`, `govet`, etc. Do not disable linters with `//nolint:` without an explanatory comment (`nolintlint` enforces this).

## Coding Conventions

- **Go version**: keep in sync with the `go-version` pinned in `.github/workflows/main.yaml` and the `golang` base image in `Dockerfile`.
- **Error handling**: always check errors; do not use `_` to discard them unless there is a clear reason.
- **Context propagation**: pass `context.Context` through call chains; do not use `noctx`-violating HTTP requests.
- **Formatting**: code must pass `gofmt` and `gofumpt` (extra rules enabled). Run `make fmt-backend` before committing.
- **Tests**: place tests in `_test.go` files next to the package under test, using the `github.com/stretchr/testify` assertion helpers already in `go.mod`.
- **Config changes**: when adding or changing add-on configuration options, update all four of: `config.yaml` (options + schema), `translations/en.yaml`, `DOCS.md` (user-facing documentation), and the relevant Go struct in `backend/pkg/uibackend/`.

## Release Process

1. Bump `version` in `config.yaml` to match the intended Git tag.
2. Update `CHANGELOG.md`.
3. Push a Git tag — the CI pipeline will build and publish Docker images for `amd64` and `aarch64` to GHCR.

The `beta` branch is managed via `make recreate-beta-branch` / `make adjust-config-for-beta-branch`.
