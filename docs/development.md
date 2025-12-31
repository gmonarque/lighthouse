# Development

Guide for contributing to Lighthouse development.

---

## Prerequisites

- **Go 1.22+**
- **Node.js 20+**
- **Make**
- **Git**

---

## Getting Started

### Clone Repository

```bash
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse
```

### Install Dependencies

```bash
make deps
```

This installs:
- Go dependencies
- Node.js packages
- Air (hot reload)

### Development Mode

Run backend with hot reload:

```bash
make dev
```

Run frontend dev server (separate terminal):

```bash
make dev-frontend
```

Access at:
- **Backend**: http://localhost:9999
- **Frontend Dev**: http://localhost:5173 (proxies to backend)

---

## Project Structure

```
lighthouse/
├── cmd/
│   └── lighthouse/
│       └── main.go          # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers/        # HTTP handlers
│   │   ├── middleware/      # Auth, logging
│   │   ├── router.go        # Route definitions
│   │   └── static/          # Embedded frontend
│   ├── comments/            # Comment system
│   ├── config/              # Configuration
│   ├── curator/             # Curation engine
│   ├── database/
│   │   ├── database.go      # SQLite connection
│   │   ├── migrations/      # SQL migrations
│   │   └── queries/         # SQL files
│   ├── decision/            # Verification decisions
│   ├── explorer/            # Relay exploration
│   ├── indexer/             # Core indexer
│   ├── models/              # Shared types
│   ├── moderation/          # Reports/appeals
│   ├── nostr/               # Nostr client
│   ├── relay/               # Relay server
│   ├── ruleset/             # Rule engine
│   ├── torznab/             # Torznab API
│   └── trust/               # Trust system
├── web/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/         # API client
│   │   │   ├── components/  # Svelte components
│   │   │   └── stores/      # State management
│   │   └── routes/          # SvelteKit pages
│   ├── static/              # Static assets
│   └── package.json
├── docs/                    # Documentation
├── Makefile
├── go.mod
└── config.yaml
```

---

## Make Commands

| Command | Description |
|---------|-------------|
| `make deps` | Install all dependencies |
| `make build` | Production build |
| `make dev` | Backend with hot reload |
| `make dev-frontend` | Frontend dev server |
| `make test` | Run all tests |
| `make lint` | Run linters |
| `make clean` | Clean build artifacts |
| `make docker` | Build Docker image |

---

## Backend Development

### Adding a Handler

1. Create handler in `internal/api/handlers/`:

```go
// internal/api/handlers/example.go
package handlers

import (
    "net/http"
)

func GetExample(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]string{
        "message": "Hello, World!",
    })
}
```

2. Register route in `internal/api/router.go`:

```go
r.Get("/example", handlers.GetExample)
```

### Adding a Database Migration

1. Create SQL file in `internal/database/migrations/`:

```sql
-- internal/database/migrations/003_example.sql
CREATE TABLE IF NOT EXISTS examples (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

2. Migrations run automatically on startup.

### Adding a Package

1. Create directory under `internal/`:

```bash
mkdir internal/myfeature
```

2. Create types:

```go
// internal/myfeature/types.go
package myfeature

type MyType struct {
    ID   string
    Name string
}
```

3. Create logic:

```go
// internal/myfeature/service.go
package myfeature

func DoSomething(input string) (*MyType, error) {
    // Implementation
}
```

4. Add tests:

```go
// internal/myfeature/types_test.go
package myfeature

import "testing"

func TestMyType(t *testing.T) {
    // Test implementation
}
```

---

## Frontend Development

### Adding a Page

1. Create route in `web/src/routes/`:

```svelte
<!-- web/src/routes/example/+page.svelte -->
<script lang="ts">
    import { onMount } from 'svelte';

    let data = [];

    onMount(async () => {
        // Fetch data
    });
</script>

<div class="page-header">
    <h1>Example Page</h1>
</div>

<div class="page-content">
    <!-- Content -->
</div>
```

2. Add to navigation in `web/src/routes/+layout.svelte`:

```typescript
const navItems = [
    // ... existing items
    { href: '/example', label: 'Example', icon: SomeIcon },
];
```

### Adding an API Method

1. Add to `web/src/lib/api/client.ts`:

```typescript
export interface ExampleResponse {
    id: string;
    name: string;
}

class APIClient {
    // ... existing methods

    async getExample(): Promise<ExampleResponse> {
        return this.request<ExampleResponse>('/example');
    }

    async createExample(data: { name: string }): Promise<ExampleResponse> {
        return this.request<ExampleResponse>('/example', {
            method: 'POST',
            body: JSON.stringify(data)
        });
    }
}
```

### Styling

Use Tailwind CSS classes. Custom styles in `web/src/app.css`.

Common patterns:
- `card` - Card container
- `btn-primary`, `btn-secondary` - Buttons
- `input` - Form inputs
- `modal`, `modal-backdrop` - Modals
- `page-header`, `page-content` - Page layout

---

## Testing

### Running Tests

```bash
# All tests
make test

# Specific package
go test ./internal/ruleset/...

# With coverage
go test -cover ./...

# Verbose
go test -v ./...
```

### Writing Tests

```go
// internal/example/example_test.go
package example

import (
    "testing"
)

func TestSomething(t *testing.T) {
    // Arrange
    input := "test"

    // Act
    result := DoSomething(input)

    // Assert
    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}

func TestTableDriven(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case 1", "input1", "expected1"},
        {"case 2", "input2", "expected2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := DoSomething(tt.input)
            if result != tt.expected {
                t.Errorf("expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### Frontend Tests

```bash
cd web
npm run test
```

---

## Code Style

### Go

- Follow standard Go conventions
- Use `gofmt` for formatting
- Use `golint` for linting
- Prefer explicit over implicit
- Handle errors properly

### TypeScript/Svelte

- Use TypeScript strict mode
- Follow Svelte 5 patterns (runes)
- Use Tailwind for styling
- Keep components small and focused

### Commits

Follow conventional commits:

```
feat: add new feature
fix: fix bug
docs: update documentation
refactor: refactor code
test: add tests
chore: maintenance tasks
```

---

## Pull Request Process

1. **Fork** the repository
2. **Create branch**: `git checkout -b feature/my-feature`
3. **Make changes**
4. **Add tests**
5. **Run tests**: `make test`
6. **Run lints**: `make lint`
7. **Commit** with conventional commit message
8. **Push**: `git push origin feature/my-feature`
9. **Create PR** with description

### PR Checklist

- [ ] Tests pass
- [ ] Lints pass
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
- [ ] Follows code style

---

## Debugging

### Backend

```go
import "log"

log.Printf("Debug: %v", value)
```

Or use Delve:

```bash
dlv debug ./cmd/lighthouse
```

### Frontend

```typescript
console.log('Debug:', value);
```

Or use browser DevTools.

### Database

```bash
sqlite3 ./data/lighthouse.db

# View tables
.tables

# Query
SELECT * FROM torrents LIMIT 10;
```

---

## Architecture Guidelines

### Separation of Concerns

- **Handlers** - HTTP request/response only
- **Services** - Business logic
- **Storage** - Database operations
- **Types** - Data structures

### Error Handling

```go
// Return errors, don't panic
func DoSomething() error {
    if err := operation(); err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}
```

### Context Usage

```go
// Pass context for cancellation
func DoSomething(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue
    }
}
```

---

## Documentation

### Code Comments

```go
// DoSomething performs an important operation.
// It takes an input string and returns a result.
//
// Example:
//
//     result, err := DoSomething("input")
//     if err != nil {
//         // handle error
//     }
func DoSomething(input string) (string, error) {
    // Implementation
}
```

### API Documentation

Update `docs/api-reference.md` for new endpoints.

### User Documentation

Update relevant docs in `docs/` folder.

---

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create git tag: `git tag v1.0.0`
4. Push tag: `git push origin v1.0.0`
5. GitHub Actions builds releases

---

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/gmonarque/lighthouse/issues)
- **Discussions**: GitHub Discussions
- **Nostr**: Follow project npub

---

## License

MIT License - contributions are welcome under the same license.
