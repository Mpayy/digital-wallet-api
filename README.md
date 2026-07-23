# Digital Wallet API

A RESTful digital wallet API built with Go, simulating core features of mobile wallet applications like Dana or OVO — covering user registration, wallet top-up, peer-to-peer transfers, and transaction history.

> **Disclaimer:** This is a portfolio/simulation project. It is not a licensed payment system and is not intended for production financial use.

---

## Tech Stack

All dependencies listed below are verified from `go.mod`:

| Dependency | Version | Role |
|---|---|---|
| [Gin](https://github.com/gin-gonic/gin) | v1.12.0 | HTTP framework |
| [GORM](https://gorm.io) | v1.31.2 | ORM |
| [gorm/driver/mysql](https://github.com/go-gorm/mysql) | v1.6.0 | MySQL driver |
| [go-redis/v9](https://github.com/redis/go-redis) | v9.21.0 | Redis client |
| [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | v5.3.1 | JWT token |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | v0.52.0 | bcrypt password hashing |
| [go-playground/validator/v10](https://github.com/go-playground/validator) | v10.30.3 | Request validation |
| [logrus](https://github.com/sirupsen/logrus) | v1.9.4 | Structured logging |
| [viper](https://github.com/spf13/viper) | v1.21.0 | Configuration / env |
| [google/wire](https://github.com/google/wire) | v0.7.0 | Compile-time dependency injection |
| [mockery](https://github.com/vektra/mockery) | v2 (CLI) | Mock generation for unit tests |

**Infrastructure:** MySQL 8.0, Redis 7 (Alpine)

---

## Architecture

The project follows **Clean Architecture** with three explicit layers:

```
Handler  →  Usecase  →  Repository
```

- **Handler** — Parses HTTP requests, validates input, reads JWT context, calls the appropriate usecase, and formats the response. No business logic.
- **Usecase** — Contains all business logic (idempotency, locking, balance mutation, ownership checks). Each domain has its own usecase interface.
- **Repository** — Data access only. Abstracts GORM queries and Redis calls behind interfaces, making the usecase layer independent of persistence details.

Dependency wiring is handled at startup by **Google Wire** (`wire.go` + generated `wire_gen.go`).

### Domain Boundary & FK Design Decision

The project is split into two bounded contexts:

**`internal/auth`** — Identity & Auth domain (`users` table)
**`internal/wallet`** — Financial domain (`wallets`, `transfers`, `transactions`, `idempotency_keys` tables)

**Why `wallets.user_id` has no FK to `users.id`:**
This is an intentional architectural decision. In real-world financial systems, identity/auth is frequently a separate service or an external Identity Provider (IDP). Enforcing a DB-level FK from `wallets` to `users` would create a hard coupling between two contexts that are designed to be independent. The wallet domain only needs to know a `user_id` exists — it does not own the user record. This boundary makes the auth domain replaceable without touching the financial schema.

**Why `transfers` and `transactions` do have FKs:**
`wallets`, `transfers`, and `transactions` all belong to the same financial bounded context and require strong referential consistency. A transfer record must always reference valid wallet IDs; a transaction record must always reference a valid wallet and, where applicable, a valid transfer. These FK constraints are enforced at the database level via InnoDB foreign keys.

```
Auth domain          Financial domain
─────────────        ──────────────────────────────────────
users                wallets ←── transfers (FK both sides)
  id ──(no FK)──▶    user_id    transactions ──FK──▶ wallets
                                transactions ──FK──▶ transfers
```

---

## Key Features

All features listed below are verified to exist in the codebase.

### ✅ Auto-provisioned Wallet on Registration

When a user registers (`POST /auth/register`), `AuthUsecase.Register` calls `WalletUsecase.CreateWallet(userID)` immediately after the user record is persisted. Wallet provisioning is **best-effort**: if it fails (e.g., duplicate), the registration still succeeds and returns a `200`. The wallet can be retrieved lazily on first access to `GET /wallets/me`.

### ✅ Session Management via Redis Token Store

On login, the JWT token is stored in Redis with a TTL. The JWT middleware validates both the token's cryptographic signature **and** its presence in Redis. On logout, the token is deleted from Redis, effectively invalidating the session server-side without waiting for the JWT to naturally expire.

### ✅ Pessimistic Locking for Balance Mutations

All balance mutations (top-up and transfer) use `SELECT ... FOR UPDATE` via GORM's `clause.Locking{Strength: "UPDATE"}` inside a database transaction. This prevents lost-update race conditions under concurrent requests targeting the same wallet.

### ✅ Ordered Lock Acquisition to Prevent Deadlocks

In the transfer flow, when two wallets must be locked simultaneously, the wallet with the **smaller ID is always locked first**, regardless of who is the sender or recipient. This consistent ordering eliminates the circular-wait condition that causes deadlocks between two concurrent reverse-direction transfers.

```go
// From transfer_usecase.go
firstID, secondID := fromWallet.ID, toWallet.ID
if firstID > secondID {
    firstID, secondID = secondID, firstID
}
// Lock firstID first, then secondID — always
```

See [Concurrency Safety Evidence](#concurrency-safety-evidence) for proof this was reproduced and verified.

### ✅ Idempotency Key Mechanism

`POST /wallets/top-up` and `POST /wallets/transfer` require an `Idempotency-Key` request header. The mechanism:

1. Attempts to `INSERT` a new record with status `PROCESSING` — leveraging the DB `UNIQUE` constraint on `idem_key` as the concurrency-safe claim primitive.
2. On duplicate key: fetches the existing record and compares a SHA-256 hash of the request payload to detect mismatched reuse (returns `409 IDEMPOTENCY_KEY_CONFLICT`).
3. If the same key + same payload and status is `COMPLETED`: returns the cached response body directly, without re-executing the operation.
4. After successful execution: updates the record to `COMPLETED` with the serialized response body.
5. On failure: marks the record as `FAILED`.

### ✅ Monetary Values as Integer (BIGINT)

All balance and amount fields use `int64` in Go and `BIGINT` in MySQL. No floating-point types are used anywhere in the money-handling path, avoiding float precision issues for currency representation.

### ✅ Transaction History with Pagination and Filtering

`GET /transactions` supports query parameters: `type` (`TOPUP`, `TRANSFER_IN`, `TRANSFER_OUT`), `start_date`, `end_date` (format: `YYYY-MM-DD`), `page`, and `limit` (max 100, default 10). Results are returned with a `meta` object containing `total` count and `total_pages`.

### ✅ Transaction Ownership Enforcement

`GET /transactions/:id` verifies that the retrieved transaction's `wallet_id` belongs to the authenticated user's wallet. Accessing another user's transaction ID returns `404`, not `403` — intentionally avoiding information leakage about the existence of other users' transactions.

---

## API Endpoints

Base path: `/api/v1`
All protected routes require `Authorization: Bearer <token>` header.

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/auth/register` | — | Register new user. Provisions wallet best-effort. |
| `POST` | `/auth/login` | — | Login. Returns a JWT token stored in Redis. |
| `POST` | `/auth/logout` | ✅ JWT | Invalidates the session token in Redis. |

### Wallet

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/wallets/me` | ✅ JWT | Returns the authenticated user's wallet. |
| `POST` | `/wallets/top-up` | ✅ JWT | Add funds. Requires `Idempotency-Key` header. |
| `POST` | `/wallets/transfer` | ✅ JWT | Transfer to another user. Requires `Idempotency-Key` header. |

### Transactions

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/transactions` | ✅ JWT | List transaction history with pagination and filters. |
| `GET` | `/transactions/:id` | ✅ JWT | Get a single transaction detail. Ownership-checked. |

---

## Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) & Docker Compose
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) — used as a CLI tool, not a Go library (not in `go.mod`):

```bash
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 1. Clone and configure environment

```bash
git clone https://github.com/Mpayy/digital-wallet-api.git
cd digital-wallet-api

cp .env.example .env
```

Edit `.env` and fill in all required values:

```env
APP_HOST=
APP_PORT=8080

DATABASE_HOST=localhost
DATABASE_PORT=3306
DATABASE_NAME=digital_wallet_api
DATABASE_USERNAME=root
DATABASE_PASSWORD=root

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0

LOG_LEVEL=info
JWT_SECRET_KEY=your-secret-key-here
```

### 2. Start infrastructure services

```bash
docker compose up -d mysql redis
```

Wait a few seconds for MySQL to finish initializing before running migrations.

### 3. Run database migrations

```bash
migrate -path migrations \
  -database "mysql://root:root@tcp(localhost:3306)/digital_wallet_api?multiStatements=true" \
  up
```

> Adjust the credentials to match your `.env` values.

### 4. Run the application

**Option A — with Docker Compose (recommended):**

```bash
docker compose up --build
```

The `app` service reads `.env` from the project root and connects to the `mysql` and `redis` services on the internal Docker network.

**Option B — run locally:**

```bash
go run ./cmd/api
```

Requires MySQL and Redis to be running and accessible at the hosts/ports defined in `.env`.

### 5. Verify

```bash
curl http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'
```

---

## Database Schema Overview

| Table | Key Columns | Notes |
|---|---|---|
| `users` | `id`, `email` (UNIQUE), `password` | Auth domain. No FK to other tables. |
| `wallets` | `id`, `user_id` (UNIQUE), `balance BIGINT` | `user_id` has no FK to `users` — intentional (see Architecture). |
| `transfers` | `id`, `from_wallet_id`, `to_wallet_id`, `amount`, `note` | FK → `wallets` on both wallet columns. |
| `transactions` | `id`, `wallet_id`, `type`, `amount`, `balance_before`, `balance_after`, `transfer_id` (nullable), `status` | FK → `wallets`, FK → `transfers`. |
| `idempotency_keys` | `id`, `idem_key` (UNIQUE), `user_id`, `endpoint`, `request_hash CHAR(64)`, `status`, `response_body TEXT` | No FK to `users` — same boundary rationale. |

Transaction types: `TOPUP`, `TRANSFER_IN`, `TRANSFER_OUT`
Transaction statuses: `SUCCESS`, `FAILED`
Idempotency statuses: `PROCESSING`, `COMPLETED`, `FAILED`

---

## Testing

### Unit Tests

Run with the standard Go test command — no infrastructure required. All external dependencies (GORM, Redis, etc.) are mocked using [Mockery](https://github.com/vektra/mockery)-generated mocks.

```bash
go test ./...
```

**Coverage (verified from file count):**

| File | Test Cases | What is covered |
|---|---|---|
| `wallet_usecase_test.go` | 18 | CreateWallet, GetWalletByUserID, TopUp (all success/failure paths including idempotency replay) |
| `transfer_usecase_test.go` | 25 | Transfer (invalid amount, wallet not found, self-transfer, ordered lock, insufficient balance, idempotency replay, failure paths) |
| `idempotency_service_test.go` | 18 | Claim (empty key, hash mismatch, replay COMPLETED/PROCESSING/FAILED), Complete, MarkFailed |
| `transaction_usecase_test.go` | 14 | GetTransactionHistory (pagination, filters, defaults), GetTransactionDetail (ownership check) |
| `auth_usecase_test.go` | 19 | Register, Login, Logout, GetUserByID — including bcrypt comparison and Redis session handling |
| `jwt_middleware_test.go` | 12 | Missing/invalid header, JWT validation, Redis session check, context injection |
| **Total** | **106** | |

### Integration Tests

Require Docker. Uses a **dedicated MySQL instance on port 3307** via `docker-compose.test.yml` (separate from the dev DB on port 3306, using `tmpfs` for speed). Run with:

```bash
make test-integration
```

Which executes:

```bash
docker-compose -f docker-compose.test.yml up -d
sleep 8
migrate -path migrations -database "mysql://root@tcp(localhost:3307)/digital_wallet_test?multiStatements=true" up
go test -tags=integration -race ./test/integration/... -v
docker-compose -f docker-compose.test.yml down
```

The `-race` flag is passed intentionally to catch Go-level data races in addition to the database-level correctness assertions.

**Integration test scenarios:**

| Test | File | What it proves |
|---|---|---|
| `TestConcurrentTopUp_NoLostUpdate` | `wallet_topup_test.go` | 20 goroutines top-up the same wallet concurrently with unique idempotency keys. Asserts final balance = `20 × 1000` and exactly 20 transaction rows — verifying no write is lost under `SELECT FOR UPDATE`. |
| `TestConcurrentTransfer_OppositeDirection_NoDeadlock` | `wallet_transfer_test.go` | 50 iterations of A→B and B→A transfers fired simultaneously (100 goroutines). Asserts no deadlock (15s timeout), total money is conserved (`finalA + finalB = 2,000,000`), and exactly 200 transaction rows exist. |
| `TestConcurrentTopUp_SameIdempotencyKey_OnlyAppliedOnce` | `wallet_idempotency_test.go` | 20 goroutines fire the same top-up with the **same idempotency key**. Asserts balance increases by exactly 1,000 (only once), only 1 transaction row exists, and all successful responses share the same `transaction_id`. |

---

## Concurrency Safety Evidence

The ordered-lock deadlock prevention was not only reasoned about — it was **reproduced and experimentally verified** against a real MySQL 8 instance.

### Method

The ordered lock logic in `transfer_usecase.go` was temporarily commented out, reverting to naive locking (lock sender first, always):

```go
// BEFORE fix (naive — causes deadlock):
// firstID, secondID := fromWallet.ID, toWallet.ID
// if firstID > secondID {
//     firstID, secondID = secondID, firstID   // <-- this line removed
// }

// AFTER fix (ordered — deadlock-free):
firstID, secondID := fromWallet.ID, toWallet.ID
if firstID > secondID {
    firstID, secondID = secondID, firstID
}
```

### Result Without Fix

Running `TestConcurrentTransfer_OppositeDirection_NoDeadlock` with naive locking produced **Error 1213** repeatedly from MySQL:

```
Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
SELECT * FROM `wallets` WHERE `wallets`.`id` = 2 ... FOR UPDATE
```

The test also failed its balance assertions:

```
expected: 1000000
actual  : 990000   ← balance corrupted by aborted transactions
```

And transaction count was wrong (`96` instead of `200`), confirming writes were lost.

### Result With Fix

After restoring the ordered lock, the same test passes cleanly across all 100 goroutines. No Error 1213, no balance drift, transaction count exact.

### Screenshot

![MySQL Error 1213 — deadlock reproduced without ordered locking](docs/images/deadlock-error.png)

For the full log output and more detail, see [docs/deadlock-demo.md](docs/deadlock-demo.md).

---

## Project Status

This project is **in active development**. The core API is functional and the critical financial consistency mechanisms (locking, idempotency, atomic transactions) are implemented and tested. The following is an honest breakdown.

### ✅ Completed

- User registration and login with bcrypt password hashing
- JWT-based authentication with Redis session store and logout invalidation
- Wallet top-up with pessimistic locking (`SELECT FOR UPDATE`) and idempotency
- Peer-to-peer transfer with ordered lock acquisition (deadlock prevention) and idempotency
- Transaction history with pagination and filtering by type/date range
- Transaction detail with ownership enforcement
- Structured logging (logrus) across all layers
- Graceful shutdown with proper DB and Redis connection cleanup
- Dockerized with Docker Compose (3-service stack: app, mysql, redis)
- **106 unit test cases** across 6 files (wallet, transfer, idempotency, transaction, auth, middleware) using Mockery mocks
- **3 integration tests** covering concurrent top-up (no lost update), concurrent reverse transfers (no deadlock), and concurrent idempotency key deduplication — all running against real MySQL with `-race` flag
- Deadlock reproduced, documented, and fixed (Error 1213 evidence captured)

### 🔧 In Progress / Planned

| Item | Status |
|---|---|
| Retry mechanism for MySQL deadlock error (error 1213) | 📋 Planned (defensive) |
| TTL / reclaim mechanism for idempotency keys stuck in `PROCESSING` status | 📋 Planned |
| Payment gateway sandbox integration (Midtrans / Xendit) | 💡 Optional, later phase |
| Kafka event-driven architecture enhancement | 💡 Optional, later phase |

> **Note:** There are currently **0 repository/handler-layer tests**. The tested scope covers usecase and middleware layers only.
