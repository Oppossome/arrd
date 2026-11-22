# Project & Tooling

- **Stack**: [Language, framework, runtime, package manager]
<!-- - **Verify changes**: Run `<test cmd>`, `<lint cmd>`, and `<typecheck/build cmd>` before finishing. -->

# Project Layout

Follow the official Go module layout guidance (https://go.dev/doc/modules/layout), using these directories from the community "Standard Go Project Layout" (github.com/golang-standards/project-layout). That template is a convention, not an official standard.

- **`cmd/<name>/`**: One directory per executable, each with a thin `main` package that only parses config/flags and wires up code from `internal/`. Keep business logic out of `cmd/`.
- **`internal/`**: All application and library code. The Go compiler blocks imports of `internal/` from outside this module, so prefer it by default.
- **`tools/`**: Supporting dev tools and scripts for the project. Pin Go tool dependencies with the `tool` directive in `go.mod` (`go get -tool <pkg>`), not a `tools.go` blank-import file.
- **No `pkg/`** and no other template directories (`api/`, `configs/`, `build/`, etc.) unless there's a concrete need. Add structure only when the project requires it.

# Clean Code Rules

- **Scope**: Edit only files relevant to the task. Avoid drive-by refactors or unrequested edits to unrelated modules.
- **Modularity**: Keep functions small and single-purpose. Extract shared logic only when duplication is real and clear.
- **Clarity & Naming**: Use descriptive names for variables, functions, and types. Prefer early returns to deep nesting.
- **Error Handling**: Fail fast with explicit, contextual errors. Never swallow exceptions or use silent fallbacks unless specified.
- **Types & Boundaries**: Validate external inputs at boundaries; avoid loose/stringly-typed shortcuts where strong types exist.
- **Tests**: Add or update unit/integration tests for changed behavior. Do not weaken assertions or delete failing tests to pass.

# Boundaries (Do Not Touch)

- Never modify generated files, lockfiles (unless requested by a dependency task), secrets, or `.env` files.
- Ask before changing public APIs, database schemas, or infrastructure configs.
