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
This is an intentional architectural decision. In real-world financial systems, identity/auth is frequently a separate service or an external Identity Provider (IDP). Enforcing a DB-level FK from `wallets` to `users` would create a hard coupling between two contexts that are designed to be independent. The wallet domain only needs to know a `user_id` exists — it does not own the user record. This boundary makes the auth domain replaceable (e.g., swap to an external IDP) without touching the financial schema.

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

When a user registers (`POST /auth/register`), the `AuthUsecase` immediately calls `WalletUsecase.CreateWallet(userID)` — creating a wallet with `balance = 0` atomically within the same request. No separate wallet-creation step is needed.

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
| `POST` | `/auth/register` | — | Register new user. Auto-creates a wallet. |
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

## Project Status

This project is **in active development**. The core API is functional and the critical financial consistency mechanisms (locking, idempotency, atomic transactions) are implemented. The following is an honest breakdown.

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

### 🔧 In Progress / Planned

| Item | Status |
|---|---|
| Unit tests for usecase layer (Wallet, Transfer, Idempotency) | 🔧 In progress |
| Integration tests for race condition scenarios (concurrent transfers, deadlock prevention) | 📋 Planned |
| Retry mechanism for MySQL deadlock error (error 1213) | 📋 Planned (defensive) |
| TTL / reclaim mechanism for idempotency keys stuck in `PROCESSING` status | 📋 Planned |
| Payment gateway sandbox integration (Midtrans / Xendit) | 💡 Optional, later phase |
| Kafka event-driven architecture enhancement | 💡 Optional, later phase |

> **Note:** There are currently **0 test files** (`*_test.go`) in the repository. Test coverage is the primary active development priority.
