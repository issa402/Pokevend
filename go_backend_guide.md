# Go Backend Engineering Guide
## Everything You Need to Know to Be an Insane Backend Engineer

---

## 1. Go Fundamentals

### Variables & Types
```go
// Three ways to declare a variable:
var x int = 5          // explicit type + value
var y = "hello"        // inferred type (string)
z := 42                // short declaration (inside functions only)

// CONSTANTS (compile-time, immutable)
const MaxRetries = 3
const Pi = 3.14159

// MULTIPLE ASSIGNMENT (Go specialty)
a, b := 10, 20
a, b = b, a  // swap without temp variable

// ZERO VALUES (Go initializes everything)
var i int    // 0
var f float64 // 0.0
var s string  // ""
var b bool    // false
var p *int   // nil
```

### Types You Must Know
```go
// Primitives
int, int8, int16, int32, int64
uint, uint8, uint16, uint32, uint64
float32, float64
string     // immutable sequence of bytes (UTF-8)
bool       // true or false
byte       // alias for uint8 (often used for raw bytes)
rune       // alias for int32 (represents a Unicode code point)

// Composite
[]int             // slice (dynamic array)
map[string]int    // map (hash map)
[5]int            // array (fixed length — rarely used, prefer slices)
struct{}          // struct (group of fields)
interface{}       // any type (use sparingly — prefer specific types)

// Pointer
*int    // pointer to int — holds memory address
nil     // zero value for pointers, slices, maps, channels, functions, interfaces
```

### Pointers — The Key to Go
```go
x := 42
p := &x     // & = "address of" — p is *int pointing to x
*p = 100    // * = "dereference" — changes the value x points to
// x is now 100

// WHY POINTERS MATTER IN OUR CODE:
// func Register() (*models.User, string, error)
// Returns *models.User (pointer) instead of models.User (value)
// Pointer = can be nil (user not found), value = always has data (default zeros)
// Rule: return *T for things that might not exist, T for things always there
```

### Slices (Dynamic Arrays)
```go
// Create
s := []string{"a", "b", "c"}
s := make([]string, 0, 10)  // len=0, cap=10 (pre-allocated)

// Operations
s = append(s, "d")         // add element
s[0]                        // index (0-based)
s[1:3]                      // slice [1, 3) → ["b", "c"]
len(s)                      // length

// IMPORTANT: nil vs empty slice
var nilSlice []string       // nil (JSON: null)
emptySlice := []string{}   // empty (JSON: [])
// In APIs, always return []string{} not nil — "null" in JSON confuses frontend
```

### Maps (Hash Maps)
```go
m := map[string]int{"a": 1, "b": 2}
m := make(map[string]int)

// Read (always returns zero value if key missing — no panic)
v := m["key"]         // 0 if not found (not an error!)

// Safe read with "ok idiom"
v, ok := m["key"]    // ok=false if not found
if ok { /* key exists */ }

// Write
m["new"] = 42

// Delete
delete(m, "key")

// Iterate
for k, v := range m { fmt.Println(k, v) }

// We use maps in handlers for JSON responses:
pkg.JSON(w, 200, map[string]interface{}{
    "token": token,
    "user":  user,
})
```

### Structs — Your Domain Models
```go
// Define
type Card struct {
    CardID  string   `json:"cardId"`   // json tag for serialization
    Name    string   `json:"name"`
    Price   *float64 `json:"price"`    // *float64 = nullable
}

// Instantiate
c := Card{CardID: "ch-1", Name: "Charizard"}  // named fields (preferred)
c := Card{"ch-1", "Charizard", nil}             // positional (avoid — breaks on field adds)

// Access
c.Name  // "Charizard"

// Struct embedding (like inheritance but explicit)
type CardWithHistory struct {
    Card             // embed Card — all Card fields available directly
    History []PricePoint
}
cwh := CardWithHistory{}
cwh.Name = "Charizard"  // direct access to embedded Card.Name
```

### Interfaces — The Core of Go Design
```go
// Interface = a set of method signatures
// If a type has ALL the methods → it implements the interface (implicit)
type CardStore interface {
    Search(ctx context.Context, query string, limit int) ([]Card, error)
    GetByID(ctx context.Context, id string) (*Card, error)
}

// Concrete implementation
type postgresCardStore struct { db *pgxpool.Pool }

func (s *postgresCardStore) Search(ctx context.Context, q string, n int) ([]Card, error) {
    // ...SQL...
}
func (s *postgresCardStore) GetByID(ctx context.Context, id string) (*Card, error) {
    // ...SQL...
}
// postgresCardStore now implicitly implements CardStore!

// Using the interface
var store CardStore = NewCardStore(db)
// OR in a struct field:
type CardService struct {
    store CardStore  // interface → can be postgresCardStore OR mockCardStore in tests
}

// WHY INTERFACES = TESTABLE CODE:
// In tests, pass a mock:
type mockCardStore struct{}
func (m *mockCardStore) Search(...) ([]Card, error) { return fakeCards, nil }
// service.store = &mockCardStore{} → no real DB needed in unit tests!
```

### Methods & Method Receivers
```go
type AuthService struct { ... }

// Pointer receiver (*AuthService) — ALMOST ALWAYS USE THIS
// Allows method to modify the struct AND avoids copying
func (s *AuthService) Register(ctx context.Context, email, password string) (*User, string, error) {
    // s.users is a pointer to the real struct (not a copy)
}

// Value receiver (AuthService) — use for read-only methods on small structs
func (c Card) DisplayPrice() string {
    return fmt.Sprintf("$%.2f", *c.Price)
}

// RULE: if ANY method uses pointer receiver on a type → ALL methods should use pointer receiver
```

---

## 2. Error Handling — Go's Most Important Pattern

```go
// Go has NO exceptions. Errors are RETURN VALUES.
// Functions that can fail return (result, error)

// ── Pattern 1: Check and return ────────────────────────────
func DoSomething() (string, error) {
    result, err := risky()
    if err != nil {
        return "", err           // propagate up
        return "", fmt.Errorf("context: %w", err)  // wrap with context (%w preserves original)
    }
    return result, nil
}

// ── Pattern 2: Named errors ────────────────────────────────
var ErrEmailTaken = errors.New("email already registered")
var ErrNotFound   = errors.New("resource not found")

// compare with:
if errors.Is(err, ErrEmailTaken) { /* ... */ }    // type-safe check
// NOT: if err.Error() == "email already registered" { }  // fragile string compare

// ── Pattern 3: Fatal (only in main func startup) ──────────
db, err := pgxpool.New(ctx, dsn)
if err != nil {
    log.Fatalf("DB connection failed: %v", err)  // log + os.Exit(1)
}
// ONLY use log.Fatalf in main() startup — never in handlers or services

// ── Pattern 4: Panic/Recover (rare, mostly framework internals) ────
// NEVER panic in normal code. chi router's middleware.Recoverer catches panics
// from your handlers and converts them to 500 responses instead of crashing.
```

---

## 3. Goroutines & Concurrency

```go
// ── Goroutine: lightweight concurrent function ─────────────
// Like a thread but uses ~2KB instead of ~1MB
// Scheduled by Go runtime (M:N threading)
go someFunction()    // launch and forget

go func() {          // anonymous goroutine
    // runs concurrently
}()

// ── WaitGroup: wait for N goroutines to finish ─────────────
var wg sync.WaitGroup
wg.Add(2)            // "2 goroutines will run"
go func() {
    defer wg.Done()  // "I'm done" — ALWAYS use defer here
    fbScraper.Scrape()
}()
go func() {
    defer wg.Done()
    mercariScraper.Scrape()
}()
wg.Wait()            // BLOCK until both call Done()

// ── Channel: safe goroutine communication ─────────────────
ch := make(chan string, 16)  // buffered channel (capacity 16)

// Send (non-blocking if buffer has space)
ch <- "message"

// Receive (blocks until message available)
msg := <-ch

// Non-blocking send (select with default):
select {
case ch <- msg:  // send if possible
default:         // skip if channel full
}

// Range over channel (reads until channel closed):
for msg := range ch { process(msg) }

// ── Mutex: protect shared data ────────────────────────────
var mu sync.Mutex
mu.Lock()        // exclusive access
defer mu.Unlock()
sharedMap["key"] = value

// RWMutex: multiple readers OR one writer
var rmu sync.RWMutex
rmu.RLock()   // shared read lock — multiple goroutines can read simultaneously
defer rmu.RUnlock()
// vs
rmu.Lock()    // exclusive write lock — blocks all readers
defer rmu.Unlock()
```

---

## 4. Context — Threading Request State

```go
// context.Context threads cancellation and values through call chains.
// Pass ctx as FIRST parameter to every function that does I/O.

// Create contexts:
ctx := context.Background()           // root context (in main, worker startup)
ctx := r.Context()                    // request context (in handlers — auto-cancelled if request ends)
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)  // timeout after 5s
defer cancel()                        // always cancel to free resources

// Check if context is cancelled:
select {
case <-ctx.Done():
    return ctx.Err()  // context.DeadlineExceeded or context.Canceled
default:
    // proceed
}

// Store values in context (use sparingly — only for request-scoped data like user):
ctx = context.WithValue(ctx, "userID", "uuid-123")
userID := ctx.Value("userID").(string)

// In our code:
// middleware/auth.go stores UserClaims in context
// handlers read it with middleware.GetUser(r)
```

---

## 5. HTTP Handling with chi

```go
// ── Router setup ───────────────────────────────────────────
r := chi.NewRouter()
r.Use(chimiddleware.Logger)   // log every request
r.Use(chimiddleware.Recoverer) // catch panics → 500

// ── Route registration ─────────────────────────────────────
r.Get("/path", handlerFunc)   // GET /path
r.Post("/path", handlerFunc)  // POST /path
r.Put("/path/{id}", handlerFunc) // PUT with URL param
r.Delete("/path/{id}", handlerFunc)
r.Route("/api", func(r chi.Router) { // route group
    r.Use(myMiddleware)              // applies to all routes in group
    r.Get("/cards", cardsHandler)
})

// ── Handler signature ──────────────────────────────────────
func MyHandler(w http.ResponseWriter, r *http.Request) {
    // Read request:
    chi.URLParam(r, "id")              // /path/{id}
    r.URL.Query().Get("q")            // ?q=value
    json.NewDecoder(r.Body).Decode(&body)  // JSON body

    // Write response:
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)       // MUST before writing body
    json.NewEncoder(w).Encode(result)

    // Our helpers (pkg/respond.go):
    pkg.JSON(w, 200, data)
    pkg.Error(w, 404, "not found")
}

// ── HTTP Status Codes You Must Memorize ───────────────────
// 200 OK              - successful GET, PUT
// 201 Created         - successful POST (resource created)
// 204 No Content      - successful DELETE (no body)
// 400 Bad Request     - client sent invalid data
// 401 Unauthorized    - not logged in (no or invalid JWT)
// 403 Forbidden       - logged in but no permission
// 404 Not Found       - resource doesn't exist
// 409 Conflict        - duplicate (email already registered)
// 422 Unprocessable   - valid JSON but failed business validation
// 429 Too Many Req    - rate limit exceeded
// 500 Internal Server - bug in our code
// 503 Service Unavail - our dependency is down
```

---

## 6. PostgreSQL with pgx

```go
// ── Connection pool ────────────────────────────────────────
pool, _ := pgxpool.New(ctx, "postgres://user:pass@host:5432/db")

// ── Query (multiple rows) ──────────────────────────────────
rows, err := pool.Query(ctx, "SELECT id, name FROM cards WHERE name ILIKE $1", "%char%")
defer rows.Close()   // ALWAYS close rows
for rows.Next() {
    var c Card
    rows.Scan(&c.ID, &c.Name)  // order must match SELECT
}

// ── QueryRow (exactly one row) ───────────────────────────
var c Card
pool.QueryRow(ctx, "SELECT id, name FROM cards WHERE id=$1", id).Scan(&c.ID, &c.Name)

// ── Exec (no rows returned) ──────────────────────────────
result, err := pool.Exec(ctx, "UPDATE cards SET name=$1 WHERE id=$2", name, id)
result.RowsAffected()  // check if anything was updated

// ── PARAMETERIZED QUERIES — MANDATORY ─────────────────────
// SAFE:   pool.Query(ctx, "SELECT ... WHERE id=$1", id)
// UNSAFE: pool.Query(ctx, fmt.Sprintf("SELECT ... WHERE id=%s", id))  // SQL INJECTION!

// ── RETURNING — get generated data ──────────────────────
var newID string
pool.QueryRow(ctx, 
    "INSERT INTO users(email) VALUES($1) RETURNING id", email,
).Scan(&newID)
```

---

## 7. Redis with go-redis

```go
// ── Setup ─────────────────────────────────────────────────
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

// ── Basic operations ──────────────────────────────────────
rdb.Set(ctx, "key", "value", 5*time.Minute)  // SET with TTL
rdb.Get(ctx, "key").Result()                  // GET → (string, error)
rdb.Del(ctx, "key1", "key2")                  // DELETE

// ── JSON caching pattern ──────────────────────────────────
// Cache:
bytes, _ := json.Marshal(cards)
rdb.Set(ctx, "search:charizard", bytes, 5*time.Minute)

// Read:
cached, err := rdb.Get(ctx, "search:charizard").Bytes()
if err == nil {  // cache HIT
    json.Unmarshal(cached, &cards)
    return cards, nil
}
// err == redis.Nil means key not found (cache MISS)
// any other error = Redis problem — fall through to DB

// ── Rate limiting ─────────────────────────────────────────
count, _ := rdb.Incr(ctx, "ratelimit:ip:1.2.3.4").Result()
rdb.Expire(ctx, "ratelimit:ip:1.2.3.4", 1*time.Second)
if count > 100 { /* rate limited */ }
```

---

## 8. RabbitMQ with amqp091-go

```go
// ── Connect ───────────────────────────────────────────────
conn, _ := amqp.Dial("amqp://guest:guest@localhost:5672")
defer conn.Close()
ch, _ := conn.Channel()
defer ch.Close()

// ── Declare queue (idempotent) ───────────────────────────
q, _ := ch.QueueDeclare("listings", true, false, false, false, nil)
// durable=true: queue survives broker restart

// ── Publish ───────────────────────────────────────────────
ch.Publish("", "listings", false, false, amqp.Publishing{
    ContentType:  "application/json",
    DeliveryMode: amqp.Persistent,  // survive broker restart
    Body:         []byte(`{"cardName":"Charizard","price":89.99}`),
})

// ── Consume ───────────────────────────────────────────────
msgs, _ := ch.Consume("listings", "", false, false, false, false, nil)
for msg := range msgs {
    var listing Listing
    json.Unmarshal(msg.Body, &listing)
    processListing(listing)
    msg.Ack(false)   // acknowledge (remove from queue)
    // msg.Nack(false, false) to reject without requeue
}
```

---

## 9. JWT with golang-jwt

```go
// ── Create (sign) a JWT ───────────────────────────────────
claims := jwt.MapClaims{
    "id":    userID,
    "email": email,
    "exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
}
token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))

// ── Validate a JWT ────────────────────────────────────────
token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{},
    func(t *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    },
)
if err != nil || !token.Valid { /* invalid */ }

claims := token.Claims.(*jwt.MapClaims)
userID := (*claims)["id"].(string)   // type assertion

// ── JWT Structure (decoded) ───────────────────────────────
// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9  ← base64(header)
// .eyJpZCI6InV1aWQiLCJlbWFpbCI6Ii4uLiJ9  ← base64(payload)
// .HMAC_SHA256(header.payload, secret)    ← signature (cannot forge without secret)
```

---

## 10. The 8-Layer Architecture

```
HTTP Request
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  routes/routes.go    URL → Handler mapping          │  Layer 8
│  GET /api/cards →  handlers.CardHandler.Search()    │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  middleware/auth.go   Validate JWT, inject user      │  Layer 7
│  middleware/ratelimit.go  Rate limiting              │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  handlers/*_handler.go  Parse req → call service    │  Layer 6
│  pkg/respond.go          Write JSON response         │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  services/*_service.go  Business logic              │  Layer 5
│  Named errors, bcrypt, JWT, caching decisions        │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  store/*_store.go   ALL SQL queries                 │  Layer 4
│  Interface → concrete implementation (PostgreSQL)    │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  models/*.go   Pure data structs                    │  Layer 3
│  No logic — just typed containers                    │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  config/*.go   Infrastructure connections           │  Layer 2
│  PostgreSQL pool, Redis client, RabbitMQ conn        │
└─────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────┐
│  main.go    Composition root — DI wiring            │  Layer 1
│  config → stores → services → handlers → router     │
└─────────────────────────────────────────────────────┘

Background:
  worker/notification_worker.go  RabbitMQ consumer → SSE push
  handlers/sse_handler.go        Long-running stream handler
```

---

## 11. Dependency Injection Pattern

```go
// main.go (the Composition Root) — builds everything bottom-up
cfg := config.Load()
db, _     := config.ConnectPostgres(cfg)
rdb       := config.ConnectRedis(cfg)
rmq, _    := config.ConnectRabbitMQ(cfg)

// STORES (Layer 4 — depends on db)
cardStore    := store.NewCardStore(db)
userStore    := store.NewUserStore(db)
alertStore   := store.NewAlertStore(db)

// SERVICES (Layer 5 — depends on stores + cache)
authSvc  := services.NewAuthService(userStore, cfg.JWTSecret)
cardSvc  := services.NewCardService(cardStore, rdb)
alertSvc := services.NewAlertService(alertStore)

// HANDLERS (Layer 6 — depends on services)
authH    := handlers.NewAuthHandler(authSvc)
cardH    := handlers.NewCardHandler(cardSvc)
alertH   := handlers.NewAlertHandler(alertSvc)

// WORKER (Background — depends on raw storage + SSE)
sseManager := handlers.NewSSEManager()
go worker.StartNotificationWorker(rmq, db, sseManager)

// ROUTES wires everything
routes.Register(r, cfg, sseManager, authH, cardH, alertH, ...)
```

---

## 12. Production Readiness Checklist

```
Security:
  ✅ Passwords → bcrypt (cost 12)
  ✅ API keys → AES-256-GCM encrypted
  ✅ JWTs → HMAC-SHA256 signed (7-day expiry)
  ✅ SQL → parameterized queries ($1, $2)
  ✅ All writes → user_id in WHERE clause (IDOR prevention)
  ✅ Rate limiting → 100 req/s per IP
  ✅ CORS → only allows frontend origin

Performance:
  ✅ PostgreSQL connection pool (pgxpool)
  ✅ Redis caching (5-30 min TTL per resource)
  ✅ Non-blocking SSE (buffered channels, select/default)
  ✅ Concurrent scrapers (sync.WaitGroup + goroutines)

Reliability:
  ✅ defer Close/Stop/Cleanup on all resources
  ✅ Graceful error handling (no raw panics in handlers)
  ✅ RabbitMQ Ack/Nack (no message loss on crash)
  ✅ Context propagation (cancellation on request end)

Still To Do (your practice TODOs):
  [ ] Graceful shutdown (os.Signal handling)
  [ ] Structured logging (JSON logs with request_id)
  [ ] Metrics endpoint (Prometheus format)
  [ ] Unit tests for services and stores
  [ ] Integration tests for handlers
  [ ] Distributed rate limiting (Redis-based)
```
