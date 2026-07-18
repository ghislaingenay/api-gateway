### General Responsibilities:

- Guide the development of idiomatic, maintainable, and high-performance Go code.
- Enforce modular design and separation of concerns through Clean Architecture.
- Promote test-driven development, robust observability, and scalable patterns across services.

### Architecture Patterns:

- Apply **Clean Architecture** by structuring code into handlers/controllers, services/use cases, repositories/data access, and domain models.
- Use **domain-driven design** principles where applicable.
- Prioritize **interface-driven development** with explicit dependency injection.
- Prefer **composition over inheritance**; favor small, purpose-specific interfaces.
- Ensure that all public functions interact with interfaces, not concrete types, to enhance flexibility and testability.

### Project Structure Guidelines:

- Use a consistent project layout:
  - cmd/: application entrypoints
  - internal/: core application logic (not exposed externally)
  - pkg/: shared utilities and packages
  - api/: gRPC/REST transport definitions and handlers
  - configs/: configuration schemas and loading
  - test/: test utilities, mocks, and integration tests
- Group code by feature when it improves clarity and cohesion.
- Keep logic decoupled from framework-specific code.

### Development Best Practices:

- Write **short, focused functions** with a single responsibility.
- Always **check and handle errors explicitly**, using wrapped errors for traceability ('fmt.Errorf("context: %w", err)').
- Avoid **global state**; use constructor functions to inject dependencies.
- Leverage **Go's context propagation** for request-scoped values, deadlines, and cancellations.
- Use **goroutines safely**; guard shared state with channels or sync primitives.
- **Defer closing resources** and handle them carefully to avoid leaks.

### Security and Resilience:

- Apply **input validation and sanitization** rigorously, especially on inputs from external sources.
- Use secure defaults for **JWT, cookies**, and configuration settings.
- Isolate sensitive operations with clear **permission boundaries**.
- Implement **retries, exponential backoff, and timeouts** on all external calls.
- Use **circuit breakers and rate limiting** for service protection.
- Consider implementing **distributed rate-limiting** to prevent abuse across services (e.g., using Redis).

### Testing:

- Write **unit tests** using table-driven patterns and parallel execution.
- **Mock external interfaces** cleanly using generated or handwritten mocks.
- Separate **fast unit tests** from slower integration and E2E tests.
- Ensure **test coverage** for every exported function, with behavioral checks.
- Use tools like 'go test -cover' to ensure adequate test coverage.

### Documentation and Standards:

- Document public functions and packages with **GoDoc-style comments**.
- Provide concise **READMEs** for services and libraries.
- Maintain a 'CONTRIBUTING.md' and 'ARCHITECTURE.md' to guide team practices.
- Enforce naming consistency and formatting with 'go fmt', 'goimports', and 'golangci-lint'.

### Observability with OpenTelemetry:

- Use **OpenTelemetry** for distributed tracing, metrics, and structured logging.
- Start and propagate tracing **spans** across all service boundaries (HTTP, gRPC, DB, external APIs).
- Always attach 'context.Context' to spans, logs, and metric exports.
- Use **otel.Tracer** for creating spans and **otel.Meter** for collecting metrics.
- Record important attributes like request parameters, user ID, and error messages in spans.
- Use **log correlation** by injecting trace IDs into structured logs.
- Export data to **OpenTelemetry Collector**, **Jaeger**, or **Prometheus**.

### Tracing and Monitoring Best Practices:

- Trace all **incoming requests** and propagate context through internal and external calls.
- Use **middleware** to instrument HTTP and gRPC endpoints automatically.
- Annotate slow, critical, or error-prone paths with **custom spans**.
- Monitor application health via key metrics: **request latency, throughput, error rate, resource usage**.
- Define **SLIs** (e.g., request latency < 300ms) and track them with **Prometheus/Grafana** dashboards.
- Alert on key conditions (e.g., high 5xx rates, DB errors, Redis timeouts) using a robust alerting pipeline.
- Avoid excessive **cardinality** in labels and traces; keep observability overhead minimal.
- Use **log levels** appropriately (info, warn, error) and emit **JSON-formatted logs** for ingestion by observability tools.
- Include unique **request IDs** and trace context in all logs for correlation.

### Performance:

- Use **benchmarks** to track performance regressions and identify bottlenecks.
- Minimize **allocations** and avoid premature optimization; profile before tuning.
- Instrument key areas (DB, external calls, heavy computation) to monitor runtime behavior.

### Concurrency and Goroutines:

- Ensure safe use of **goroutines**, and guard shared state with channels or sync primitives.
- Implement **goroutine cancellation** using context propagation to avoid leaks and deadlocks.

### Tooling and Dependencies:

- Rely on **stable, minimal third-party libraries**; prefer the standard library where feasible.
- Use **Go modules** for dependency management and reproducibility.
- Version-lock dependencies for deterministic builds.
- Integrate **linting, testing, and security checks** in CI pipelines.

### Key Conventions:

1. Prioritize **readability, simplicity, and maintainability**.
2. Design for **change**: isolate business logic and minimize framework lock-in.
3. Emphasize clear **boundaries** and **dependency inversion**.
4. Ensure all behavior is **observable, testable, and documented**.
5. **Automate workflows** for testing, building, and deployment.

Always use the latest stable version of Go (1.22 or newer) and be familiar with RESTful API design principles, best practices, and Go idioms.

- Follow the user's requirements carefully & to the letter.
- First think step-by-step - describe your plan for the API structure, endpoints, and data flow in pseudocode, written out in great detail.
- Confirm the plan, then write code!
- Write correct, up-to-date, bug-free, fully functional, secure, and efficient Go code for APIs.
- Use the standard library's net/http package for API development:
  - Utilize the new ServeMux introduced in Go 1.22 for routing
  - Implement proper handling of different HTTP methods (GET, POST, PUT, DELETE, etc.)
  - Use method handlers with appropriate signatures (e.g., func(w http.ResponseWriter, r \*http.Request))
  - Leverage new features like wildcard matching and regex support in routes
- Implement proper error handling, including custom error types when beneficial.
- Use appropriate status codes and format JSON responses correctly.
- Implement input validation for API endpoints.
- Utilize Go's built-in concurrency features when beneficial for API performance.
- Follow RESTful API design principles and best practices.
- Include necessary imports, package declarations, and any required setup code.
- Implement proper logging using the standard library's log package or a simple custom logger.
- Consider implementing middleware for cross-cutting concerns (e.g., logging, authentication).
- Implement rate limiting and authentication/authorization when appropriate, using standard library features or simple custom implementations.
- Leave NO todos, placeholders, or missing pieces in the API implementation.
- Be concise in explanations, but provide brief comments for complex logic or Go-specific idioms.
- If unsure about a best practice or implementation detail, say so instead of guessing.
- Offer suggestions for testing the API endpoints using Go's testing package.

Always prioritize security, scalability, and maintainability in your API designs and implementations. Leverage the power and simplicity of Go's standard library to create efficient and idiomatic APIs.

## Package Organization and Domain Ownership

### Package Design

- Organize code primarily by business domain/feature, not by technical layer.
- Prefer:

```text
internal/
├── tenant/
├── apikey/
├── auth/
├── ratelimit/
└── gateway/
```

over:

```text
internal/
├── handlers/
├── services/
├── repositories/
└── models/
```

- Keep code that changes together in the same package.
- Avoid large, generic packages such as `models`, `utils`, `helpers`, or `common`.
- Create new packages only when they represent:
  - A distinct business domain.
  - Shared infrastructure.
  - A clear architectural boundary.

### Domain Package Structure

```text
internal/
└── tenant/
    ├── model.go
    ├── dto.go
    ├── repository.go
    ├── service.go
    ├── handler.go
    └── errors.go
```

### Infrastructure Package Structure

```text
internal/
├── database/
├── validation/
├── config/
├── cache/
├── logger/
└── response/
```

---

## Models and DTOs

### Domain Models

- Domain models belong to the package that owns the domain.
- Avoid centralized `models` packages unless strongly justified.

```go
package tenant

type Tenant struct {
    ID   uuid.UUID
    Name string
}
```

### DTOs

- Separate API request/response objects from domain models.
- Place DTOs in `dto.go` within the owning package.
- Never expose database entities directly through API responses.

```go
type CreateTenantRequest struct {
    Name string `json:"name"`
}
```

---

## Validation Standards

### Validation Rules

- Validation tags remain on DTOs and domain models.

```go
type CreateTenantRequest struct {
    Slug string `validate:"required,slug"`
}
```

### Custom Validators

- Custom validator implementations must live in a dedicated validation package.

```text
internal/
└── validation/
    ├── validator.go
    ├── slug.go
    └── timezone.go
```

- Register validators centrally during startup.
- Handlers validate requests before invoking services.

---

## Error Handling Standards

### Error Propagation

- Return errors rather than panic whenever possible.
- Wrap errors with context.

```go
return fmt.Errorf("fetch tenant: %w", err)
```

### Sentinel Errors

Use sentinel errors when callers need branching behavior.

```go
var ErrTenantNotFound = errors.New("tenant not found")
```

```go
if errors.Is(err, ErrTenantNotFound) {
    // handle not found
}
```

### Custom Error Types

Use custom error types only when structured information is required.

```go
type ValidationError struct {
    Field string
    Rule  string
}
```

### Error Ownership

- Define errors in the package that owns the domain.

```text
internal/
├── tenant/errors.go
├── apikey/errors.go
└── auth/errors.go
```

- Avoid a single global error package containing all application errors.

---

## Startup and Initialization

### Constructors

Prefer returning errors:

```go
func NewValidator() (*validator.Validate, error)
```

Avoid panics during initialization unless failure is unrecoverable.

### Panic Usage

`panic` is acceptable only for:

- Unrecoverable startup failures.
- Invalid application state.
- Programmer errors.

Functions that intentionally panic should use the `Must` prefix.

```go
MustLoadConfig()
MustConnectDatabase()
```

---

## Database Migrations

### Migration Execution

- Do not automatically run database migrations during production server startup.
- Treat migrations as a separate deployment concern.

Preferred flow:

```text
Deploy Migration Job
        ↓
Run Migrations
        ↓
Deploy Application
```

### Application Startup

Avoid:

```go
database.Migrate(db)
startServer()
```

Prefer:

```bash
go run cmd/migrate/main.go
go run cmd/server/main.go
```

### Local Development

- Automatic migrations may be enabled only for local development.

---

## Dependency Injection

### Interfaces

- Define interfaces near the consumer, not the implementation.

```go
type TenantRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
}
```

### Dependency Construction

Use constructor injection:

```go
func NewService(
    repo TenantRepository,
    logger Logger,
) *Service
```

Avoid service locators and global dependency containers.

---

## API Layer Responsibilities

### Handlers

Handlers should only:

- Parse requests.
- Validate input.
- Call services.
- Return responses.

Handlers must not contain:

- Business logic.
- Database access.
- External service calls.

### Services

Services should:

- Contain business rules.
- Orchestrate repositories.
- Coordinate external dependencies.

### Repositories

Repositories should:

- Encapsulate persistence.
- Contain database-specific logic.
- Return domain entities.

Repositories should not contain business rules.
