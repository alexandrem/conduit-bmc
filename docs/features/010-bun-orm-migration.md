---
rfd: "010"
title: "Migrate Database Layer to Bun ORM"
state: "implemented"
breaking_changes: false
testing_required: true
database_changes: true
api_changes: false
dependencies:
    - "github.com/uptrace/bun"
    - "github.com/uptrace/bun/dialect/sqlitedialect"
    - "github.com/uptrace/bun/driver/sqliteshim"
    - "github.com/uptrace/bun/extra/bundebug"
database_migrations:
    - "Convert raw SQL schema to Bun models"
    - "Add proper JSON column types"
    - "Add indexes for query optimization"
areas: [ "manager" ]
---

# RFD 010 - Migrate Database Layer to Bun ORM

**Status:** 🎉 Implemented

## Summary

Replace the current raw SQL database layer in the Manager service with Bun ORM
to improve code maintainability, type safety, and eliminate current limitations
such as improper JSON handling for arrays. This migration will convert ~1000
lines of manual SQL queries and scanning logic into declarative Go models with
automatic query generation, while maintaining full backwards compatibility with
the existing SQLite database.

## Problem

The current database implementation (`manager/pkg/database/database.go`, ~1040
lines) has several significant issues:

### 1. **Manual Query Construction and Error-Prone Scanning**

```go
// Current implementation - verbose and error-prone
err := db.conn.QueryRow(
"SELECT id, email, api_key, created_at FROM customers WHERE id = ?",
customerID,
).Scan(&customer.ID, &customer.Email, &customer.APIKey, &customer.CreatedAt)
```

- 50+ hand-written SQL queries
- Manual scanning of 10+ fields per query
- Easy to introduce bugs when adding/removing columns
- No compile-time validation of queries

### 2. **Broken JSON Handling**

```go
// Current: Features stored as comma-separated string
features := "power,console,vnc,sensors" // ❌ Wrong

// Expected: JSON array
features := ["power", "console", "vnc", "sensors"] // ✅ Correct
```

**Impact**:

- `GetServerLocation` returns `["power,console,vnc"]` instead of
  `["power", "console", "vnc"]`
- Tests had to be modified to work around this limitation
- CLI receives malformed feature data

### 3. **No Type Safety**

```go
// Current: String interpolation everywhere
query := "SELECT * FROM servers WHERE id = ?"
// What if 'id' column is renamed? No compile-time check!
```

### 4. **Complex JSON Serialization Logic**

```go
// 40+ lines of manual JSON marshaling/unmarshaling
solJSON, err := json.Marshal(server.SOLEndpoint)
// Then manually insert into TEXT column
// Then manually unmarshal on read
// Repeated for every endpoint type
```

### 5. **No Migration Framework**

- Single monolithic `migrate()` function
- No versioning of schema changes
- No rollback capability
- Difficult to track schema evolution

### 6. **Performance Issues**

- No query optimization
- Missing indexes on foreign keys
- N+1 query problems in list operations
- No query logging for debugging

## Solution

Migrate to [Bun ORM](https://github.com/uptrace/bun), a lightweight,
Go-first SQL ORM with excellent
SQLite support.

### Why Bun?

| Feature        | Current (raw SQL)          | Bun ORM                         |
|----------------|----------------------------|---------------------------------|
| Type Safety    | ❌ Runtime errors           | ✅ Compile-time validation       |
| Query Building | ❌ String concatenation     | ✅ Fluent API                    |
| JSON Support   | ❌ Manual marshal/unmarshal | ✅ Automatic with `bun:",json"`  |
| Migrations     | ❌ Single file              | ✅ Versioned migrations          |
| Relations      | ❌ Manual joins             | ✅ Automatic with `rel:has-many` |
| Query Logging  | ❌ None                     | ✅ Built-in with bundebug        |
| Testing        | ❌ Hard to mock             | ✅ Interface-based               |
| Code Lines     | ~1040 lines                | ~400 lines (estimate)           |

### Architecture Changes

#### Before (Current):

```
manager/pkg/database/        ❌ Wrong location (exported package)
├── database.go              (~1040 lines of raw SQL)
└── (no models, no migrations)
```

#### After (Bun):

```
manager/internal/database/   ✅ Correct location (private to manager)
├── db.go                    (~100 lines - DB setup)
├── models.go                (~200 lines - Bun models)
├── repositories.go          (~200 lines - Repository pattern)
├── migrations/
│   ├── 001_initial_schema.go
│   ├── 002_add_vnc_endpoints.go
│   └── 003_add_indexes.go
└── migrations.go            (~50 lines - migration runner)
```

**Note**: The migration includes moving the database package from `pkg/` to
`internal/` to properly encapsulate the implementation details.

### Example Code Transformation

#### Before (Current):

```go
func (db *DB) GetServer(serverID string) (*models.Server, error) {
    var server models.Server
    var featuresStr, solEndpointJSON, vncEndpointJSON, controlEndpointJSON, metadataJSON sql.NullString

    err := db.conn.QueryRow(`
            SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint,
                   username, capabilities, features, status,
                   sol_endpoint, vnc_endpoint, control_endpoint, metadata,
                   created_at, updated_at
            FROM servers
            WHERE id = ?`, serverID).
        Scan(
            &server.ID, &server.CustomerID, &server.DatacenterID,
            &server.BMCType, &server.BMCEndpoint, &server.Username,
            &server.Capabilities, &featuresStr, &server.Status,
            &solEndpointJSON, &vncEndpointJSON, &controlEndpointJSON,
            &metadataJSON, &server.CreatedAt, &server.UpdatedAt,
        )

    // Manual JSON unmarshaling (20+ lines omitted)
    if solEndpointJSON.Valid {
        if err := json.Unmarshal([]byte(solEndpointJSON.String), &server.SOLEndpoint); err != nil {
            return nil, err
        }
    }
    // Repeat for vnc_endpoint, control_endpoint, metadata...

    return &server, nil
}
```

#### After (Bun):

```go
func (r *ServerRepository) GetServer(ctx context.Context, serverID string) (*models.Server, error) {
    server := new(models.Server)
    err := r.db.NewSelect().
        Model(server).
        Where("id = ?", serverID).
        Scan(ctx)
    return server, err
}
```

**Lines of Code**: 30+ lines → 6 lines (80% reduction)

### Model Definition Example

```go
type Server struct {
    bun.BaseModel `bun:"table:servers"`

    ID             string                `bun:"id,pk"`
    CustomerID     string                `bun:"customer_id,notnull"`
    DatacenterID   string                `bun:"datacenter_id,notnull"`
    BMCType        models.BMCType        `bun:"bmc_type,notnull"`
    BMCEndpoint    string                `bun:"bmc_endpoint,notnull"`
    Username       string                `bun:"username"`
    Capabilities   []string              `bun:"capabilities,type:jsonb"` // ✅ Proper JSON array
    Features       []string              `bun:"features,type:jsonb"` // ✅ Proper JSON array
    Status         string                `bun:"status,notnull,default:'active'"`
    SOLEndpoint    *models.SOLEndpoint   `bun:"sol_endpoint,type:jsonb"`
    VNCEndpoint    *models.VNCEndpoint   `bun:"vnc_endpoint,type:jsonb"`
    ControlEndpoint *models.BMCControlEndpoint `bun:"control_endpoint,type:jsonb"`
    Metadata       map[string]string     `bun:"metadata,type:jsonb"`
    CreatedAt      time.Time             `bun:"created_at,nullzero,notnull,default:current_timestamp"`
    UpdatedAt      time.Time             `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

    // Relations
    Customer       *Customer             `bun:"rel:belongs-to,join:customer_id=id"`
}
```

## Implementation Plan

### Phase 1: Setup and Model Definition

1. **Add Bun dependencies**
   ```bash
   go get github.com/uptrace/bun
   go get github.com/uptrace/bun/dialect/sqlitedialect
   go get github.com/uptrace/bun/driver/sqliteshim
   go get github.com/uptrace/bun/extra/bundebug
   ```

2. **Create new internal database package**

3. **Create Bun models** (`manager/internal/database/models.go`)
    - Convert `models.Server` → Bun model with tags
    - Convert `models.Customer` → Bun model
    - Convert `models.RegionalGateway` → Bun model
    - Convert `models.ServerLocation` → Bun model
    - Convert `models.Agent` → Bun model
    - Define relationships using `rel:` tags

4. **Create database wrapper** (`manager/pkg/database/db.go`)

5. **Write unit tests for models**
    - Test model serialization/deserialization
    - Test JSON field handling
    - Test relationship loading

### Phase 2: Repository Pattern

1. **Create repository interfaces** (`manager/pkg/database/repositories.go`)

2. **Implement all repositories**
    - `ServerRepository`
    - `CustomerRepository`
    - `GatewayRepository`
    - `ServerLocationRepository`
    - `AgentRepository`
    - `ProxySessionRepository`

3. **Write repository tests**
    - Use in-memory SQLite for fast tests
    - Test CRUD operations
    - Test relations
    - Test transactions

## API Changes

**None**. This is an internal refactoring with no external API changes.

## Benefits

### Immediate Benefits

1. **Fixed JSON Handling**: Features and capabilities properly stored as JSON
   arrays
2. **Type Safety**: Compile-time validation of queries
3. **Code Reduction**: ~1040 lines → ~400 lines (60% reduction)
4. **Better Tests**: Easier to mock and test with repository pattern
5. **Query Logging**: Built-in debugging with bundebug

### Long-term Benefits

1. **Easier Migrations**: Versioned migration system
2. **Better Performance**: Automatic query optimization, indexes
3. **Scalability**: Easier to migrate to PostgreSQL later (just change dialect)
4. **Maintainability**: Declarative models vs imperative SQL
5. **Developer Experience**: Fluent API, better IDE autocomplete

### Risk Mitigation

1. **Low Risk**: SQLite database format unchanged
2. **Gradual Rollout**: Feature flag allows instant rollback
3. **Test Coverage**: All existing tests must pass
4. **No API Changes**: Handler layer unaffected
5. **Bun Maturity**: Production-ready, used by major Go projects

## Appendix

### Bun ORM Feature Highlights

#### 1. Fluent Query API

```go
// Complex query made simple
servers, err := db.NewSelect().
    Model((*Server)(nil)).
    Relation("Customer").
    Where("customer_id = ?", customerID).
    Where("status = ?", "active").
    OrderExpr("created_at DESC").
    Limit(10).
    Scan(ctx)
```

#### 2. Automatic JSON Handling

```go
type Server struct {
    Features []string `bun:"features,type:jsonb"`
}

// Bun automatically:
// - Marshals to JSON on INSERT
// - Unmarshals from JSON on SELECT
// - No manual json.Marshal/Unmarshal needed!
```

#### 3. Relations

```go
// Load server with all relations in one query
server := new(Server)
err := db.NewSelect().
    Model(server).
    Relation("Customer").
    Relation("SOLEndpoint").
    Relation("VNCEndpoint").
    Where("id = ?", serverID).
    Scan(ctx)

// server.Customer is automatically populated!
```

#### 4. Transactions

```go
err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
    // All operations in transaction
    if err := tx.NewInsert().Model(&server).Exec(ctx); err != nil {
        return err // Automatic rollback
    }
    return nil // Automatic commit
})
```

#### 5. Query Hooks (Logging)

```go
db.AddQueryHook(bundebug.NewQueryHook(
    bundebug.WithVerbose(true),
    bundebug.FromEnv("BUNDEBUG"),
))

// Output:
// [bun]  0.234ms  SELECT "id", "email" FROM "customers" WHERE id = 'customer-123'
```

### Alternative ORMs Considered

| ORM     | Pros                                    | Cons                                                        | Decision                 |
|---------|-----------------------------------------|-------------------------------------------------------------|--------------------------|
| **Bun** | ✅ Lightweight, Go-first, excellent docs | ❌ Smaller community than GORM                               | ✅ **Selected**           |
| GORM    | ✅ Popular, large community              | ❌ Heavy, magic behavior, poor performance                   | ❌ Too heavy              |
| sqlx    | ✅ Minimal abstraction                   | ❌ Still requires manual SQL, no migrations                  | ❌ Not enough abstraction |
| sqlc    | ✅ Type-safe, generated                  | ❌ Requires separate SQL files, limited flexibility          | ❌ Too rigid              |
| ent     | ✅ Graph-based, powerful                 | ❌ Complex, steep learning curve, Facebook-specific patterns | ❌ Too complex            |

**Winner**: Bun - Best balance of simplicity, type safety, and features

### Database Schema Evolution

#### Current Issues

```sql
-- Problem: Features as TEXT (comma-separated)
CREATE TABLE servers
(
    features TEXT NOT NULL -- "power,console,vnc" ❌
);
```

#### After Bun Migration

```sql
-- Solution: Features as JSONB array
CREATE TABLE servers
(
    features JSONB NOT NULL -- ["power","console","vnc"] ✅
);
```

### References

- [Bun Documentation](https://bun.uptrace.dev/)
- [Bun GitHub](https://github.com/uptrace/bun)
- [Bun Examples](https://github.com/uptrace/bun/tree/master/example)
- [Bun SQLite Guide](https://bun.uptrace.dev/guide/drivers.html#sqlite)
- [Bun Migrations](https://bun.uptrace.dev/guide/migrations.html)
