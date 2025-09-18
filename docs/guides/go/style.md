# Go Coding Standards for LLMs

## Core Language Patterns

### Error Handling

**Always check errors immediately after the function call that returns them.**

```go
// GOOD
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// BAD - separating error check from call
result, err := doSomething()
doSomethingElse()
if err != nil {
    return err
}
```

**Wrap errors with context using `fmt.Errorf` and `%w` verb for error chains.**

```go
// GOOD
if err := db.Connect(); err != nil {
    return fmt.Errorf("database connection failed: %w", err)
}

// BAD - losing error context
if err := db.Connect(); err != nil {
    return err
}
```

**Return early on errors. Avoid deep nesting.**

```go
// GOOD
if err := validate(input); err != nil {
    return err
}
if err := process(input); err != nil {
    return err
}
return save(input)

// BAD - unnecessary nesting
if err := validate(input); err == nil {
    if err := process(input); err == nil {
        return save(input)
    } else {
        return err
    }
} else {
    return err
}
```

### Interface Design

**Define interfaces at the point of use, not at implementation.**

```go
// GOOD - interface defined where it's needed
package consumer

type DataStore interface {
    Get(id string) (*Item, error)
}

// BAD - interface defined with implementation
package database

type DataStore interface {
    Get(id string) (*Item, error)
    Save(item *Item) error
    Delete(id string) error
    // ... many more methods
}
```

**Keep interfaces small. Prefer many small interfaces over one large interface.**

```go
// GOOD
type Reader interface {
    Read([]byte) (int, error)
}

// BAD
type ReadWriter interface {
    Read([]byte) (int, error)
    Write([]byte) (int, error)
    Close() error
    Flush() error
}
```

**Accept interfaces, return concrete types.**

```go
// GOOD
func Process(r io.Reader) (*Result, error) {
    // returns concrete type
    return &Result{}, nil
}

// BAD
func Process(f *os.File) (io.Reader, error) {
    // accepts concrete, returns interface
    return f, nil
}
```

### Context Propagation

**Pass context as the first parameter. Never store context in structs.**

```go
// GOOD
func GetUser(ctx context.Context, id string) (*User, error) {
    return db.QueryContext(ctx, "SELECT * FROM users WHERE id = ?", id)
}

// BAD - context stored in struct
type Service struct {
    ctx context.Context // NEVER do this
}
```

**Create new contexts from request context, not background context.**

```go
// GOOD
func HandleRequest(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return process(ctx)
}

// BAD - ignoring parent context
func HandleRequest(ctx context.Context) error {
    newCtx := context.WithTimeout(context.Background(), 5*time.Second)
    return process(newCtx)
}
```

**Always call cancel functions returned by context.WithCancel/WithTimeout.**

```go
// GOOD
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
defer cancel() // Always defer immediately

// BAD - leaking context
ctx, _ := context.WithTimeout(parentCtx, 30*time.Second)
// Missing cancel call
```

## Code Organization

### Package Structure and Naming

**Use short, lowercase, single-word package names. No underscores or camelCase.**

```go
// GOOD
package user
package auth
package httputil

// BAD
package userManager
package user_auth
package HttpUtil
```

**Avoid stutter. Package contents should not repeat the package name.**

```go
// GOOD
package user

type User struct{}
func Get(id string) (*User, error)

// BAD
package user

type UserStruct struct{}
func GetUser(id string) (*UserStruct, error)
```

**Place main packages in `cmd/` directory. Use `internal/` for private packages, `pkg/` for public libraries.**

```
// GOOD
myapp/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/    // Private: not importable by other projects
│   └── auth/
└── pkg/         // Public: importable by other projects
    └── client/
```

### File Organization Within Packages

**Separate types, constructors, and methods logically. Group by feature, not by type.**

```go
// GOOD - user.go
type User struct {
    ID   string
    Name string
}

func NewUser(name string) *User {
    return &User{Name: name}
}

func (u *User) Validate() error {
    // validation logic
}

// BAD - types.go, constructors.go, methods.go (separated by kind)
```

**Place methods immediately after their type definition. Order methods from public to private.**

```go
// GOOD
type Service struct {
    db Database
}

// Public methods first
func (s *Service) GetUser(id string) (*User, error) {
    return s.findByID(id)
}

func (s *Service) CreateUser(name string) (*User, error) {
    return s.insert(&User{Name: name})
}

// Private methods last
func (s *Service) findByID(id string) (*User, error) {
    // implementation
}

func (s *Service) insert(u *User) (*User, error) {
    // implementation
}
```

**Name test files with `_test.go` suffix. Keep tests next to the code they test.**

```
// GOOD
user/
├── user.go
├── user_test.go
├── store.go
└── store_test.go

// BAD
user/
├── user.go
├── store.go
tests/
├── user_test.go
└── store_test.go
```

### Dependency Injection Patterns

**Inject dependencies through constructors, not globals or init functions.**

```go
// GOOD
type Service struct {
    db Database
    logger Logger
}

func NewService(db Database, logger Logger) *Service {
    return &Service{
        db:     db,
        logger: logger,
    }
}

// BAD - global dependencies
var db *sql.DB

func init() {
    db = setupDatabase()
}

type Service struct{}
```

**Use interfaces for dependencies to enable testing and flexibility.**

```go
// GOOD
type Service struct {
    store UserStore
}

type UserStore interface {
    GetUser(id string) (*User, error)
}

func NewService(store UserStore) *Service {
    return &Service{store: store}
}

// BAD - concrete dependency
type Service struct {
    db *sql.DB
}
```

**Use functional options pattern when parameters have defaults or constructor has >4 parameters.**

```go
// GOOD - functional options for optional configuration
type Server struct {
    host    string
    port    int
    timeout int
}

type Option func(*Server)

func WithPort(p int) Option {
    return func(s *Server) { s.port = p }
}

func WithTimeout(t int) Option {
    return func(s *Server) { s.timeout = t }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        host:    "localhost",  // sensible defaults
        port:    8080,
        timeout: 30,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage: srv := NewServer(WithPort(9000), WithTimeout(60))

// BAD - too many constructor parameters
func NewServer(host string, port int, timeout int, maxConns int, debug bool) *Server {
    // difficult to use and maintain
}
```

**Pass dependencies explicitly. Avoid container or framework magic.**

```go
// GOOD
func main() {
    db := setupDB()
    cache := setupCache()
    service := NewService(db, cache)
    server := NewServer(service)
    server.Run()
}

// BAD - hidden dependencies
func main() {
    container.Register("db", setupDB())
    container.Register("cache", setupCache())
    service := container.Get("service").(*Service)
}
```

## Naming and Style

### Variable, Function, and Type Naming

**Use camelCase for local variables and unexported fields. Use PascalCase for exported identifiers.**

```go
// GOOD
var userCount int
var baseURL string

type User struct {
    Name      string  // exported
    email     string  // unexported
}

func processData() {}        // unexported
func GetUserByID() {}        // exported

// BAD
var user_count int
var base_url string
var UserCount int  // unexported should not use PascalCase

type User struct {
    name      string  // exported field should be PascalCase
    Email_Addr string  // no underscores
}
```

**Use short names for short-lived variables. Use descriptive names for package-level declarations.**

```go
// GOOD
for i, v := range values {
    process(v)
}

var defaultTimeout = 30 * time.Second
var maxRetryAttempts = 3

// BAD
for index, value := range values {
    process(value)
}

var t = 30 * time.Second  // unclear at package level
```

**Don't use generic names like `data`, `info`, `util`. Be specific.**

```go
// GOOD
type UserAccount struct{}
func ParseConfig() {}
func validateEmail() {}

// BAD
type Data struct{}
type Info struct{}
func ProcessStuff() {}
func handleData() {}
```

### Comment Formatting and Godoc

**Start comments with the name of the element being described. Use complete sentences.**

```go
// GOOD
// User represents a system user account.
type User struct{}

// GetByID retrieves a user by their unique identifier.
// It returns ErrNotFound if the user does not exist.
func GetByID(id string) (*User, error) {}

// BAD
// This struct is for users
type User struct{}

// gets user
func GetByID(id string) (*User, error) {}
```

**Document exported package elements. Package comments precede the package clause.**

```go
// GOOD
// Package auth provides authentication and authorization primitives.
package auth

// Token represents an authentication token.
type Token struct {
    Value   string
    Expires time.Time
}

// BAD - missing package and type documentation
package auth

type Token struct {
    Value   string
    Expires time.Time
}
```

**Use inline comments only for complex code. Describe what the code does, not how.**

```go
// GOOD
func process(data []byte) error {
    // Decode varint header to determine message size
    size, n := binary.Uvarint(data)
    if n <= 0 {
        return errors.New("invalid varint")
    }

    // Pre-allocate buffer to avoid reallocation during decompression
    buf := make([]byte, size)
    return decompress(data[n:], buf)
}

// BAD - obvious comments
func getUser(id string) (*User, error) {
    // Check if id is empty
    if id == "" {
        return nil, ErrEmptyID
    }

    // Query the database
    return db.QueryUser(id)
}
```

### Receiver Naming Conventions

**Use single letter receivers based on the type name. Be consistent across all methods.**

```go
// GOOD
type Server struct{}

func (s *Server) Start() error {}
func (s *Server) Stop() error {}

type User struct{}

func (u *User) IsValid() bool {}
func (u *User) Save() error {}

// BAD
func (server *Server) Start() error {}
func (srv *Server) Stop() error {}  // inconsistent

func (this *User) IsValid() bool {} // never use 'this' or 'self'
func (me *User) Save() error {}     // never use 'me'
```

**Use pointer receivers when modifying state or for large structs. Use value receivers for small, immutable types.**

```go
// GOOD
func (s *Server) SetPort(p int) {  // modifies state
    s.port = p
}

func (p Point) Distance() float64 {  // small, read-only
    return math.Sqrt(p.x*p.x + p.y*p.y)
}

// BAD
func (s Server) SetPort(p int) {  // won't modify original
    s.port = p
}
```

### Acronym Handling

**Keep acronyms in consistent case. All caps in exported names, all lowercase in unexported names.**

```go
// GOOD
var HTTPClient *http.Client
var apiURL string
type HTTPSConnection struct{}
type userID string

func GetHTTPResponse() {}
func parseJSON() {}

// BAD
var HttpClient *http.Client
var ApiUrl string
type HttpsConnection struct{}
type UserId string

func GetHttpResponse() {}
func parseJson() {}
```

## Testing Standards

### Test File Organization

**Name test files with `_test.go` suffix. Use same package for white-box testing, `_test` suffix for black-box testing.**

```go
// GOOD - white-box testing (same package)
package user

func TestUser_Validate(t *testing.T) {
    u := &User{name: "test"}  // can access private fields
    // test implementation
}

// GOOD - black-box testing (separate package)
package user_test

import "myapp/user"

func TestUser_PublicAPI(t *testing.T) {
    u := user.New("test")  // only public API
    // test implementation
}

// BAD - inconsistent naming
func Test_user_validate(t *testing.T) {}
func TestValidateUser(t *testing.T) {}
```

**Follow naming convention: `Test<Type>_<Method>` or `Test<Function>`.**

```go
// GOOD
func TestServer_Start(t *testing.T) {}
func TestServer_Stop(t *testing.T) {}
func TestParseConfig(t *testing.T) {}

// BAD
func TestServerCanStart(t *testing.T) {}
func TestStartingServer(t *testing.T) {}
```

### Table-Driven Test Patterns

**Use table-driven tests for multiple test cases. Define test cases as a slice of structs.**

```go
// GOOD
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive", 2, 3, 5},
        {"negative", -1, -2, -3},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}

// BAD - repetitive individual tests
func TestAddPositive(t *testing.T) {
    if got := Add(2, 3); got != 5 {
        t.Errorf("Add(2, 3) = %d, want 5", got)
    }
}

func TestAddNegative(t *testing.T) {
    if got := Add(-1, -2); got != -3 {
        t.Errorf("Add(-1, -2) = %d, want -3", got)
    }
}
```

**Use descriptive test names. Run subtests with `t.Run()`.**

```go
// GOOD
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"valid email", "user@example.com", false},
    {"missing @", "userexample.com", true},
    {"empty string", "", true},
}
```

**Use optional `validate` function for complex assertions. Define result struct for multiple outputs.**

```go
// GOOD - complex validation with result struct
func TestProcess(t *testing.T) {
    type result struct {
        output  *Output
        metrics *Metrics
        err     error
    }

    tests := []struct {
        name     string
        input    string
        validate func(t *testing.T, r result)
    }{
        {
            name:  "with metrics",
            input: "data",
            validate: func(t *testing.T, r result) {
                require.NoError(t, r.err)
                assert.Equal(t, "processed", r.output.Value)
                assert.Greater(t, r.metrics.Duration, 0)
                assert.Equal(t, 100, r.metrics.BytesProcessed)
            },
        },
        {
            name:  "error case",
            input: "",
            validate: func(t *testing.T, r result) {
                assert.Error(t, r.err)
                assert.Nil(t, r.output)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            out, metrics, err := Process(tt.input)
            tt.validate(t, result{out, metrics, err})
        })
    }
}
```

### Mock and Stub Conventions

**Use `testify/assert` and `testify/require` for assertions. Use `require` for setup, `assert` for verifying results.**

```go
// GOOD
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestService_Process(t *testing.T) {
    // Use require for test setup - fail fast if setup fails
    cfg, err := LoadConfig("test.yaml")
    require.NoError(t, err)
    require.NotNil(t, cfg)

    svc := NewService(cfg)

    // Use assert for testing function behavior
    result, err := svc.Process("input")
    assert.NoError(t, err)
    assert.Equal(t, "expected", result.Value)
    assert.Len(t, result.Items, 3)
}

// BAD - using assert for setup, require for results
func TestService_Process(t *testing.T) {
    cfg, err := LoadConfig("test.yaml")
    assert.NoError(t, err)  // should use require

    result, err := svc.Process("input")
    require.NoError(t, err)  // should use assert
}
```

**Generate mocks using `moq` with `go:generate`. Place mocks in `/mocks` subdirectory.**

```go
// GOOD - user/interfaces.go
package user

//go:generate go run github.com/matryer/moq@latest -pkg mocks -out mocks/store.go . UserStore
type UserStore interface {
    GetUser(id string) (*User, error)
    SaveUser(user *User) error
}

// Directory structure:
// user/
// ├── interfaces.go
// ├── user.go
// └── mocks/
//     └── store.go  // generated mock

// GOOD - using the generated mock
import "myapp/user/mocks"

func TestService(t *testing.T) {
    mock := &mocks.UserStoreMock{
        GetUserFunc: func(id string) (*User, error) {
            return &User{ID: id, Name: "Test"}, nil
        },
    }
    svc := NewService(mock)
    // test implementation
}
```

**Import existing mocks from packages. Never write custom mocks when generated ones exist.**

```go
// GOOD - reuse existing mock
import "myapp/database/mocks"

func TestHandler(t *testing.T) {
    dbMock := &mocks.DatabaseMock{
        QueryFunc: func(q string) (*Result, error) {
            return &Result{}, nil
        },
    }
    handler := NewHandler(dbMock)
}

// BAD - writing custom mock when one exists
type myCustomDBMock struct{}

func (m *myCustomDBMock) Query(q string) (*Result, error) {
    return &Result{}, nil
}
```

**Prefer dependency injection over global mocks or monkey patching.**

```go
// GOOD - inject mock via interface
type Service struct {
    store UserStore
}

func TestService(t *testing.T) {
    mock := &mocks.UserStoreMock{}
    svc := &Service{store: mock}
}

// BAD - replacing global
var globalDB *sql.DB

func TestService(t *testing.T) {
    originalDB := globalDB
    globalDB = mockDB  // dangerous
    defer func() { globalDB = originalDB }()
}
```

### Benchmark Structure

**Name benchmarks with `Benchmark` prefix. Always use `b.N` for loop iterations.**

```go
// GOOD
func BenchmarkEncode(b *testing.B) {
    data := []byte("test data")
    b.ResetTimer()  // exclude setup time

    for i := 0; i < b.N; i++ {
        Encode(data)
    }
}

func BenchmarkDecode(b *testing.B) {
    encoded := Encode([]byte("test"))
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        Decode(encoded)
    }
}

// BAD - not using b.N
func BenchmarkEncode(b *testing.B) {
    for i := 0; i < 1000; i++ {  // wrong - use b.N
        Encode(data)
    }
}
```

**Use `b.ResetTimer()` to exclude setup time. Report allocations with `b.ReportAllocs()`.**

```go
// GOOD
func BenchmarkParse(b *testing.B) {
    input := generateLargeInput()
    b.ResetTimer()
    b.ReportAllocs()  // include allocation stats

    for i := 0; i < b.N; i++ {
        Parse(input)
    }
}
```

## Project-Specific Conventions

### Internal Utility Packages

**Place shared utilities in `internal/pkg/`. Keep utilities focused on single responsibilities.**

```go
// GOOD - internal/pkg/retry/retry.go
package retry

func WithBackoff(fn func() error, maxAttempts int) error {
    // focused retry logic
}

// GOOD - internal/pkg/validate/validate.go
package validate

func Email(s string) error {
    // email validation only
}

// BAD - internal/utils/utils.go
package utils

func Retry() {}
func ValidateEmail() {}
func ParseJSON() {}
func HashPassword() {}
// too many unrelated functions
```

**Never create generic `utils` or `helpers` packages. Name packages by their function.**

```go
// GOOD
internal/pkg/
├── retry/
├── validate/
├── hash/
└── encode/

// BAD
internal/
├── utils/
├── helpers/
└── common/
```

### Logging Standards

**Use structured logging with fields. Pass loggers explicitly through context or struct fields.**

```go
// GOOD
import "log/slog"

type Service struct {
    logger *slog.Logger
    db     Database
}

func (s *Service) Process(ctx context.Context, id string) error {
    s.logger.Info("processing request",
        slog.String("id", id),
        slog.String("user", userFromContext(ctx)),
    )

    if err := s.db.Update(id); err != nil {
        s.logger.Error("update failed",
            slog.String("id", id),
            slog.String("error", err.Error()),
        )
        return err
    }
    return nil
}

// BAD - unstructured logs
func Process(id string) error {
    log.Printf("Processing %s", id)

    if err := db.Update(id); err != nil {
        log.Printf("Error: %v", err)
    }
}
```

**Use consistent log levels: Debug, Info, Warn, Error. Never use Fatal or Panic in libraries.**

```go
// GOOD - in library code
func Connect() error {
    if err := dial(); err != nil {
        logger.Error("connection failed", slog.String("error", err.Error()))
        return fmt.Errorf("dial failed: %w", err)
    }
    return nil
}

// BAD - in library code
func Connect() {
    if err := dial(); err != nil {
        log.Fatal("connection failed:", err)  // crashes entire program
    }
}
```

### Metrics and Observability

**Define metrics at package level. Use consistent naming: `<namespace>_<subsystem>_<name>`.**

```go
// GOOD
package server

import "github.com/prometheus/client_golang/prometheus"

var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "myapp",
            Subsystem: "http",
            Name:      "request_duration_seconds",
            Help:      "HTTP request duration in seconds",
        },
        []string{"method", "endpoint", "status"},
    )

    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "myapp",
            Subsystem: "http",
            Name:      "requests_total",
            Help:      "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
)

func init() {
    prometheus.MustRegister(requestDuration, requestsTotal)
}
```