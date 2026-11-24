.DEFAULT_GOAL := run

# Extra arguments passed to arrd, e.g. `make run ARGS="-log-level debug"`.
ARGS ?=

.PHONY: run web

# Headless/container builds (no tray icon): go build -tags notray ./cmd/arrd

# Build the web UI (embedded into the binary via web/embed.go), then run arrd.
run: web
	go run ./cmd/arrd $(ARGS)

web: web/node_modules/.package-lock.json
	cd web && npm run build

# npm writes this file on install; reinstall only when the lockfile changes.
web/node_modules/.package-lock.json: web/package-lock.json
	cd web && npm ci
