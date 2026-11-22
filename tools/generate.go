// Package tools holds the code generators' configuration and the
// `go generate` entry point. Run `go generate ./tools` from the module root
// after changing openapi.yaml, sqlc.yaml or the SQL migrations/queries.
//
// The generators themselves are pinned with the `tool` directive in go.mod.
package tools

// HTTP API server glue and wire types (internal/httpapi/apigen) from the
// API contract. The web client types are generated from the same spec by
// `npm run gen:api` in web/.
//go:generate go tool oapi-codegen -config oapi-codegen.yaml openapi.yaml

// Database access code (internal/store/db) from the migrations and queries.
//go:generate go tool sqlc generate -f sqlc.yaml
