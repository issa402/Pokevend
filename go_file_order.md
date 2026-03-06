# Go Backend: The Exact Order You Build Files
## And WHY That Order Is the FAANG Standard

The order matters because each layer DEPENDS on the layer below it.
You can't write a service before the store, and can't write a handler before the service.
Getting this order wrong = compile errors and confusion.

---

## The Mental Model: Bottom-Up, Dependencies First

```
ALWAYS build from lowest dependency to highest:

  Infrastructure (config, DB connection)
       ↓
  Domain Types (models)
       ↓
  Data Layer (store/repository interfaces + SQL)
       ↓
  Business Logic (services)
       ↓
  HTTP Layer (handlers, middleware)
       ↓
  URL Mapping (routes)
       ↓
  Background Jobs (worker)
       ↓
  Wiring (main.go — always LAST)
```

If you try to write `main.go` first, you'll have nothing to wire together.
If you write a handler before a service, you'll realize you don't know what to call.
The **bottom-up dependency order** is not optional — it's how software compiles.

---

## Step 1: `go.mod` — Module Declaration
**Created FIRST, before any Go files.**

```bash
go mod init pokemontool
```

```go
// go.mod
module pokemontool      // ← your module name (used in all imports)
go 1.22
```

**Why first?** Every Go file has `package X` and `import "pokemontool/config"`.
Those imports need the module name to resolve. Without `go.mod`, nothing compiles.

**What it contains:**
- Module name (used in all import paths)
- Go version
- Dependencies (added by `go get`)

**FAANG context:** `go.mod` is like `package.json` (Node) or `requirements.txt` (Python).
At Google, `go.mod` defines your service's identity in the monorepo.

---

## Step 2: `config/config.go` — Configuration Struct
**ALWAYS the first Go file you write.**

```go
package config

type Config struct {
    Port          string
    PostgresHost  string
    JWTSecret     string
    RedisAddr     string
    RabbitMQURL   string
}

func Load() *Config {
    return &Config{
        Port:         getEnv("PORT", "3001"),
        PostgresHost: getEnv("POSTGRES_HOST", "localhost"),
        // ...
    }
}

func getEnv(key, def string) string { ... }
```

**Why second?** Config has ZERO dependencies on your own code.
Everything else (DB connections, services, handlers) NEEDS config values.
You can't connect to PostgreSQL without knowing the host/port/credentials.

**The pattern:**
1. Read ALL env vars in ONE place
2. Put them in a typed struct
3. Pass the struct everywhere (never call `os.Getenv` twice for the same key)

**FAANG context:** Configuration centralization is required at FAANG scale.
When you have 200 microservices, you need to know: "what env vars does THIS service need?"
The answer must be in ONE file — `config/config.go`.

---

## Step 3: `config/db.go`, `config/redis.go`, `config/rabbitmq.go` — Connections
**Write immediately after config.go.**

```go
// config/db.go
func ConnectPostgres(cfg *Config) (*pgxpool.Pool, error) {
    dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
        cfg.PostgresUser, cfg.PostgresPassword,
        cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)
    return pgxpool.New(context.Background(), dsn)
}

// config/redis.go
func ConnectRedis(cfg *Config) *redis.Client { ... }

// config/rabbitmq.go
func ConnectRabbitMQ(cfg *Config) (*amqp.Connection, error) { ... }
```

**Why now?** These functions ONLY depend on `Config` (one file up).
They return the raw infrastructure clients that every other layer will use.

**The pattern:**
- One function per infrastructure dependency
- Accept `*Config`, return the client + error
- Call `Ping()` or health check immediately to verify connectivity
- `defer conn.Close()` is handled in `main.go`

**FAANG context:** Infrastructure connections are separated from config because
they have different failure modes. Config reading never fails. DB connections can fail.
Separating them lets `main.go` handle DB failures differently from config failures.

---

## Step 4: `models/*.go` — Domain Types
**Write before ANY stores, services, or handlers.**

```go
// models/user.go
type User struct {
    ID          string     `json:"id"`
    Email       string     `json:"email"`
    DisplayName *string    `json:"displayName"`
    CreatedAt   *time.Time `json:"createdAt"`
}

// models/card.go
type Card struct {
    CardID        string   `json:"cardId"`
    Name          string   `json:"name"`
    TrendingScore int      `json:"trendingScore"`
    PriceEbay     *float64 `json:"priceEbay"`
    // note: no PasswordHash here — never serialize secrets
}

// models/alert.go
type Alert struct { ... }
// models/watchlist.go, models/inventory.go, models/deal.go, models/show.go
```

**Why now?** Models are the shared language of your system.
Every store, service, and handler deals in `models.Card` or `models.User`.
They have ZERO dependencies — perfect to write first.

**The pattern:** One file per domain entity.
Each file maps directly to a PostgreSQL table.

**What NOT to put in models:**
- ❌ Business logic (no `func (c *Card) BestPrice() float64` for complex logic)
- ❌ Database queries (no SQL in models)
- ❌ Sensitive fields exposed in JSON (no `PasswordHash string json:"passwordHash"`)

**FAANG context:** At Uber, each model file corresponds to a protobuf definition.
Models ARE the API contract — both internal and external. Getting them right first
means every team member writes code against the same types.

---

## Step 5: `store/*_store.go` — Repository Interfaces + SQL
**Write one store file per domain entity.**

**Order within each store file:**
1. Interface definition
2. Concrete struct (`postgresXxxStore`)
3. Constructor (`NewXxxStore`)
4. Each method implementation (SQL)

```go
// store/user_store.go

// 1. Define the INTERFACE first
type UserStore interface {
    Create(ctx context.Context, email, hash, name string) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, string, error)
    GetByID(ctx context.Context, id string) (*models.User, error)
}

// 2. Concrete type (unexported — only constructable via NewUserStore)
type postgresUserStore struct {
    db *pgxpool.Pool
}

// 3. Constructor returns the INTERFACE (not the concrete type)
func NewUserStore(db *pgxpool.Pool) UserStore {
    return &postgresUserStore{db: db}
}

// 4. Method implementations
func (s *postgresUserStore) Create(ctx context.Context, ...) (*models.User, error) {
    var u models.User
    err := s.db.QueryRow(ctx,
        `INSERT INTO users(email, password_hash, display_name)
         VALUES($1, $2, $3) RETURNING id, email, display_name, created_at`,
        email, hash, name,
    ).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
    return &u, err
}
```

**Order to write store files:**
```
user_store.go     ← auth depends on users → write first
card_store.go     ← everything queries cards → write second
alert_store.go    ← alerts reference users + cards
watchlist_store.go
inventory_store.go
deal_store.go
show_store.go
```

**Why write interface BEFORE implementation?**
The interface is the CONTRACT. It answers: "what operations do I need?"
Writing it first forces you to think about the API before the SQL.
Your services depend on the interface, not the SQL — this is what makes testing possible.

**FAANG context:** The interface-first pattern (program to interfaces, not implementations)
is one of the SOLID principles (Dependency Inversion). At Google SWE interviews,
they ask about this specifically. Your test can inject a `mockUserStore` that returns
fake users — no real database needed. Fast tests = fast CI = happy engineers.

---

## Step 6: `services/*_service.go` — Business Logic
**Write after ALL stores are done. Services depend on store interfaces.**

**Order within each service file:**
1. Error variables (named errors)
2. Service struct
3. Constructor
4. Methods (business logic)

```go
// services/auth_service.go

// 1. Named errors — defined at package level
var (
    ErrEmailTaken   = errors.New("email already registered")
    ErrInvalidCreds = errors.New("invalid credentials")
    ErrWeakPassword = errors.New("password must be at least 6 characters")
)

// 2. Struct with injected dependencies
type AuthService struct {
    users     store.UserStore // INTERFACE, not concrete type
    jwtSecret string
}

// 3. Constructor
func NewAuthService(users store.UserStore, secret string) *AuthService {
    return &AuthService{users: users, jwtSecret: secret}
}

// 4. Business methods
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*models.User, string, error) {
    // business rule: check password length
    if len(password) < 6 { return nil, "", ErrWeakPassword }
    // business rule: bcrypt hash
    hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
    // delegate to store (no SQL in service)
    user, err := s.users.Create(ctx, email, string(hash), name)
    if err != nil { return nil, "", ErrEmailTaken }
    // business rule: issue JWT immediately
    token, _ := s.makeToken(user.ID, user.Email)
    return user, token, nil
}
```

**Order to write service files:**
```
auth_service.go    ← login/register. Depends on: UserStore
card_service.go    ← search, trending + Redis cache. Depends on: CardStore, *redis.Client
alert_service.go   ← list, mark read, delete. Depends on: AlertStore
watchlist_service.go
inventory_service.go
deal_service.go
show_service.go
```

**The rule:** Services have NO SQL. Services have NO HTTP code.
If you find SQL in a service → move it to the store.
If you find `http.Request` in a service → move it to the handler.

**FAANG context:** The service layer is where PRODUCT DECISIONS live.
"Passwords must be at least 8 characters" → product decision → service.
"SELECT * FROM users" → technical decision → store.
Product managers and engineers discuss the service layer.
Only engineers care about the store layer.

---

## Step 7: `middleware/*.go` — HTTP Cross-Cutting Concerns
**Write before handlers, because handlers use middleware output (user context).**

```go
// middleware/auth.go — validates JWT, injects user into context
func RequireAuth(cfg *config.Config) func(http.Handler) http.Handler { ... }
func GetUser(r *http.Request) UserClaims { ... }
func ValidateStreamToken(tokenStr, secret string) (UserClaims, bool) { ... }

// middleware/ratelimit.go — per-IP request throttling
func RateLimit(rps float64) func(http.Handler) http.Handler { ... }
```

**Why before handlers?** Handlers call `middleware.GetUser(r)`.
You need to write `GetUser` before any handler can use it.

**What goes in middleware:**
- Authentication (JWT validation)
- Rate limiting
- Request logging
- CORS headers
- Request ID injection
- Panic recovery

**What does NOT go in middleware:**
- Business logic (that's service)
- Database queries (that's store)
- Response formatting (that's handler)

---

## Step 8: `pkg/*.go` — Shared Utilities
**Write alongside middleware — utilities used by multiple packages.**

```go
// pkg/respond.go — HTTP response helpers used by ALL handlers
func JSON(w http.ResponseWriter, status int, v interface{}) { ... }
func Error(w http.ResponseWriter, status int, msg interface{}) { ... }

// pkg/crypto.go — AES encryption used by API key handler
func Encrypt(plaintext, keyHex string) (string, error) { ... }
func Decrypt(ciphertext, keyHex string) (string, error) { ... }
```

**Rule:** `pkg/` code has NO dependencies on your app's layers.
It can depend on the standard library and third-party libs, but NOT on config, models, store, or services.
If `pkg/respond.go` imported `services`, it would create a circular dependency.

---

## Step 9: `handlers/*_handler.go` — HTTP Request/Response Layer
**Write after: models, services, middleware, and pkg are complete.**

**Order within each handler file:**
1. Handler struct
2. Constructor
3. HTTP methods (one method per route)

```go
// handlers/auth_handler.go
type AuthHandler struct { svc *services.AuthService }

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
    return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
    var body struct { Email, Password, DisplayName string }
    json.NewDecoder(r.Body).Decode(&body)    // 1. parse
    user, token, err := h.svc.Register(...)  // 2. call service
    if err != nil { /* map error → HTTP status */ }
    pkg.JSON(w, 201, map...)                  // 3. respond
}
```

**Order to write handler files:**
```
auth_handler.go      ← Register + Login. Only public endpoints.
sse_handler.go       ← SSEManager struct + Stream() function
card_handler.go      ← Search, Trending, PriceHistory
alert_handler.go     ← List, MarkRead, MarkAllRead, Delete
watchlist_handler.go
inventory_handler.go
deal_handler.go
show_handler.go
apikey_handler.go
```

**Handler rules (strict):**
| Allowed in Handler | Not Allowed in Handler |
|---|---|
| Parse JSON body | SQL queries |
| Read URL params | Business rules |
| Read query params | bcrypt / JWT signing |
| Call ONE service | Redis/DB access |
| Write HTTP response | Multiple service calls (usually) |

**FAANG context:** Thin controllers/handlers = maintainable codebase.
At Stripe, handlers are typically 20-30 lines. If yours is 100+ lines,
you've put business logic in the handler. Extract it to a service.

---

## Step 10: `handlers/sse_handler.go` — Special: SSE Manager
**Same time as other handlers, but deserves its own note.**

```go
// The SSEManager must be created in main.go and passed to:
// 1. Stream handler (GET /api/stream)
// 2. notification_worker (to push alerts)
// It's infrastructure-level, not business-logic-level.

type SSEManager struct {
    mu      sync.RWMutex
    clients map[string][]*SSEClient
}
func NewSSEManager() *SSEManager { ... }
func (m *SSEManager) Add(c *SSEClient) { ... }
func (m *SSEManager) Remove(c *SSEClient) { ... }
func (m *SSEManager) SendToUser(userID, msg string) { ... }
func Stream(mgr *SSEManager, cfg *config.Config) http.HandlerFunc { ... }
```

---

## Step 11: `routes/routes.go` — URL → Handler Mapping
**Write after ALL handlers exist.**

```go
// routes/routes.go
func Register(r chi.Router, cfg *config.Config, sse *SSEManager,
              auth *AuthHandler, cards *CardHandler, ...) {
    r.Get("/health", ...)
    r.Route("/api", func(r chi.Router) {
        r.Post("/auth/register", auth.Register)
        r.Post("/auth/login", auth.Login)
        r.Get("/stream", Stream(sse, cfg))
        r.Group(func(r chi.Router) {
            r.Use(middleware.RequireAuth(cfg))
            r.Get("/cards/search", cards.Search)
            // ... all protected routes
        })
    })
}
```

**Why last among HTTP files?** Routes reference every handler.
Until all handlers exist, you can't write the complete routes file.

**FAANG context:** The routes file IS your API documentation (internal).
New engineer asks "what endpoints do we have?" → look at routes.go.
Never scatter routes across multiple files — one file, one truth.

---

## Step 12: `worker/*.go` — Background Goroutines
**Write after: stores, SSE manager, and models are complete.**

```go
// worker/notification_worker.go
func StartNotificationWorker(conn *amqp.Connection, db *pgxpool.Pool, mgr *handlers.SSEManager) {
    ch, _ := conn.Channel()
    defer ch.Close()
    q, _ := ch.QueueDeclare("listings", true, ...)
    msgs, _ := ch.Consume(q.Name, "", false, ...)
    for msg := range msgs {
        var listing Listing
        json.Unmarshal(msg.Body, &listing)
        go processListing(listing, wlStore, alertStore, mgr)
        msg.Ack(false)
    }
}
```

**Why now?** Workers use stores and SSE manager directly.
Workers don't go through HTTP — they bypass the handler layer entirely.
They're a direct pipeline: RabbitMQ → business logic → DB insert + SSE push.

---

## Step 13: `main.go` — Composition Root
**ALWAYS the last file you write.**

```go
func main() {
    // 1. Load config (nothing depends on anything yet)
    cfg := config.Load()

    // 2. Connect infrastructure (deps: config only)
    db, _  := config.ConnectPostgres(cfg)
    rdb    := config.ConnectRedis(cfg)
    rmq, _ := config.ConnectRabbitMQ(cfg)
    defer db.Close()
    defer rmq.Close()

    // 3. Create stores (deps: db)
    userStore    := store.NewUserStore(db)
    cardStore    := store.NewCardStore(db)
    alertStore   := store.NewAlertStore(db)
    // ...

    // 4. Create services (deps: stores + optional cache)
    authSvc  := services.NewAuthService(userStore, cfg.JWTSecret)
    cardSvc  := services.NewCardService(cardStore, rdb)
    alertSvc := services.NewAlertService(alertStore)
    // ...

    // 5. Create handlers (deps: services)
    authH  := handlers.NewAuthHandler(authSvc)
    cardH  := handlers.NewCardHandler(cardSvc)
    alertH := handlers.NewAlertHandler(alertSvc)
    // ...

    // 6. Create SSE manager + start background worker
    sseManager := handlers.NewSSEManager()
    go worker.StartNotificationWorker(rmq, db, sseManager)

    // 7. Register routes (deps: all handlers)
    r := chi.NewRouter()
    r.Use(chimiddleware.Logger)
    r.Use(chimiddleware.Recoverer)
    r.Use(middleware.RateLimit(100))
    routes.Register(r, cfg, sseManager, authH, cardH, alertH, ...)

    // 8. Start server
    log.Printf("Server listening on :%s", cfg.Port)
    http.ListenAndServe(":"+cfg.Port, r)
}
```

**Why last?** `main.go` ONLY wires things together.
It creates instances and connects them. Zero business logic. Zero SQL.
If `main.go` is long or complex, you've put logic in the wrong place.

---

## Complete File Creation Order (Cheat Sheet)

```
Phase 1: Foundation
  1.  go.mod
  2.  config/config.go
  3.  config/db.go
  4.  config/redis.go
  5.  config/rabbitmq.go

Phase 2: Domain
  6.  models/user.go
  7.  models/card.go
  8.  models/alert.go
  9.  models/watchlist.go
  10. models/inventory.go
  11. models/deal.go
  12. models/show.go

Phase 3: Data Layer
  13. store/user_store.go
  14. store/card_store.go
  15. store/alert_store.go
  16. store/watchlist_store.go
  17. store/inventory_store.go
  18. store/deal_store.go
  19. store/show_store.go

Phase 4: Business Logic
  20. services/auth_service.go
  21. services/card_service.go
  22. services/alert_service.go
  23. services/watchlist_service.go
  24. services/inventory_service.go
  25. services/deal_service.go
  26. services/show_service.go

Phase 5: HTTP Infrastructure
  27. pkg/respond.go
  28. pkg/crypto.go
  29. middleware/auth.go
  30. middleware/ratelimit.go

Phase 6: HTTP Handlers
  31. handlers/sse_handler.go    ← SSEManager first (worker needs it)
  32. handlers/auth_handler.go
  33. handlers/card_handler.go
  34. handlers/alert_handler.go
  35. handlers/watchlist_handler.go
  36. handlers/inventory_handler.go
  37. handlers/deal_handler.go
  38. handlers/show_handler.go
  39. handlers/apikey_handler.go

Phase 7: Routing + Workers
  40. routes/routes.go
  41. worker/notification_worker.go

Phase 8: Wiring
  42. main.go   ← ALWAYS LAST
```

---

## The One Rule That Explains All of This

> **A file can only import from files BELOW it in the dependency chain.**

```
main.go         → can import EVERYTHING below it
routes          → can import handlers, middleware, config
handlers        → can import services, pkg, middleware, models, config
services        → can import stores, models, config
stores          → can import models, config
models          → can import nothing (except standard library)
config          → can import nothing (except standard library + os.Getenv)
```

If you ever find yourself wanting to import "upward" (e.g., a store importing a service),
you have a circular dependency → your design is wrong. Restructure.

**This rule is why the creation order is non-negotiable.**
You naturally write bottom-up because you can't reference something that doesn't exist yet.
