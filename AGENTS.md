# AGENTS.md - Coding Guidelines for GSLB Project

## Project Overview
- **Backend**: Go 1.26+ (latest)
- **Frontend**: Vue 3 + Composition API + TypeScript
- **Database**: SQLite (pure Go, no CGO - use modernc.org/sqlite)
- **DNS Providers**: Cloudflare (Route53 removed)
- **Module**: `github.com/pheelee/gslb`
- **Deployment**: Single binary with embedded frontend

---

## Build Commands

### Go Backend
```bash
# Build (development)
go build -o gslb ./cmd/gslb

# Build (production - stripped)
go build -ldflags="-s -w" -o gslb ./cmd/gslb

# Run
go run ./cmd/gslb

# Download dependencies
go mod tidy
go mod download
```

### Frontend (Vue/TypeScript)
```bash
# Install dependencies
npm install

# Dev server
npm run dev

# Build for production
npm run build

# Type check
npm run type-check
```

---

## Test Commands

### Go Tests
```bash
# Run all tests
go test ./...

# Run single test (use this)
go test -run TestFunctionName ./pkg/path

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...

# Verbose output
go test -v ./...

# Race detection
go test -race ./...
```

### Frontend Tests
```bash
# Run all tests
npm test

# Run single test file
npm test -- src/components/MyComponent.spec.ts

# Run single test by name
npm test -- -t "test name pattern"

# Watch mode
npm test -- --watch
```

---

## Lint Commands

### Go
```bash
# Format code
go fmt ./...
gofumpt -w .  # preferred, stricter formatting

# Lint
golangci-lint run

# Vet (built-in static analysis)
go vet ./...
```

### Frontend
```bash
# ESLint
npm run lint

# Prettier format
npm run format

# Type check
npm run type-check
```

---

## Code Style Guidelines

### Go
- **Imports**: Group by stdlib, external, internal. Use `goimports`.
  ```go
  import (
      "context"
      "time"

      "github.com/gin-gonic/gin"
      "modernc.org/sqlite"

      "github.com/pheelee/gslb/internal/config"
  )
  ```
- **Naming**: PascalCase for exported, camelCase for unexported. No underscores.
- **Error Handling**: Always check errors. Wrap with context using `fmt.Errorf("context: %w", err)`.
- **Context**: Pass `context.Context` as first parameter to functions that need it.
- **Structs**: Use field tags for JSON. Pointer receivers for mutating methods.
- **Constants**: Use typed constants when possible: `type Status string`

### Vue 3 + TypeScript
- **API**: Use Composition API with `<script setup>` syntax.
- **Imports**: Group by Vue/core, libraries, composables, components.
- **Naming**: PascalCase for components, camelCase for composables (`useXxx`), ALL_CAPS for constants.
- **Types**: Explicit return types on exported functions. Use interfaces over type aliases for objects.
- **Props/Emits**: Define with TypeScript interfaces:
  ```ts
  interface Props {
    name: string
    count?: number
  }
  const props = defineProps<Props>()
  ```
- **Reactivity**: Use `ref()` for primitives, `reactive()` for objects. Prefer `computed()` for derived state.

---

## Embedded Frontend

The Vue frontend is embedded in the Go binary using `//go:embed`:

```go
//go:embed all:dist
var dist embed.FS
```

### Building for Production:
```bash
# 1. Build frontend (source maps are disabled by default)
cd web && npm run build

# 2. Build Go binary
go build -ldflags="-s -w" -o gslb ./cmd/gslb
```

### SPA Routing:
- All routes serve `index.html` (Vue Router handles client-side)
- API routes (`/api/*`) pass through to backend
- Static assets served from embedded filesystem

---

## Security Headers

The following security headers are automatically applied to all responses:

- `X-Content-Type-Options: nosniff` - Prevents MIME sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-XSS-Protection: 1; mode=block` - XSS protection
- `Referrer-Policy: strict-origin-when-cross-origin` - Limits referrer info
- `Content-Security-Policy` - CSP headers

### CORS Configuration:
- Same-origin requests allowed (no CORS needed when backend serves frontend)
- Development origins: `localhost:5173`, `localhost:3000`
- No wildcard `*` origins in production

---

## SQLite (No CGO)

Use `modernc.org/sqlite` driver:
```go
import _ "modernc.org/sqlite"

db, err := sql.Open("sqlite", "file:gslb.db?_pragma=foreign_keys(1)")
```

---

## File Organization

```
/
├── cmd/gslb/          # Main application entry point
├── internal/          # Private application code
│   ├── config/        # Configuration
│   ├── db/            # Database models and migrations
│   ├── dns/           # DNS provider implementations (Cloudflare, Mock)
│   ├── handlers/      # HTTP handlers
│   ├── health/        # Health check engine
│   ├── store/         # Database operations
│   └── types/         # Type definitions
├── web/               # Vue frontend
│   ├── src/
│   │   ├── components/    # Vue components
│   │   │   ├── DNSProviderForm.vue  # Dynamic DNS provider configuration
│   │   │   ├── HealthStatus.vue
│   │   │   ├── Layout.vue
│   │   │   ├── Modal.vue
│   │   │   ├── ThemeSelector.vue
│   │   │   └── Toast.vue
│   │   ├── composables/   # Vue composables
│   │   │   ├── useModal.ts
│   │   │   ├── useTheme.ts
│   │   │   └── useToast.ts
│   │   ├── views/         # Page views
│   │   │   ├── AuditLog.vue
│   │   │   ├── ConfigDetail.vue
│   │   │   ├── ConfigForm.vue
│   │   │   ├── ConfigList.vue
│   │   │   └── Dashboard.vue
│   │   ├── router/        # Vue Router configuration
│   │   ├── types/         # TypeScript types
│   │   └── embed.go       # Frontend embedding
│   └── package.json
├── go.mod
├── go.sum
├── AGENTS.md          # This file
└── SECURITY.md        # Security documentation
```

---

## Testing Conventions

### Go
- Table-driven tests with `t.Run()` for subtests
- Use `testify/assert` or `testify/require` for assertions
- Mock external dependencies via interfaces
- Test files: `*_test.go` in same package

### Frontend
- Use Vitest for unit tests
- Use Vue Test Utils for component tests
- Place tests next to source: `Component.vue` + `Component.spec.ts`

---

## Git Workflow
- Commit messages: Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`)
- Branch naming: `feature/description`, `fix/description`
- Run tests before committing
