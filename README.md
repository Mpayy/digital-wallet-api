# Digital Wallet API

[![CI](https://github.com/Mpayy/digital-wallet-api/actions/workflows/ci.yml/badge.svg)](https://github.com/Mpayy/digital-wallet-api/actions/workflows/ci.yml)
[![Integration Test](https://github.com/Mpayy/digital-wallet-api/actions/workflows/integration.yml/badge.svg)](https://github.com/Mpayy/digital-wallet-api/actions/workflows/integration.yml)

A RESTful digital wallet API built with Go, simulating core features of mobile wallet applications like Dana or OVO — covering user registration, wallet top-up, peer-to-peer transfers, withdrawals, and transaction history.

> **Disclaimer:** This is a portfolio/simulation project. It is not a licensed payment system and is not intended for production financial use. The application is not yet deployed; it currently runs locally.

---

## Tech Stack

All dependencies listed below are verified from `go.mod`:

| Dependency | Version | Role |
|---|---|---|
| [Gin](https://github.com/gin-gonic/gin) | v1.12.0 | HTTP framework |
| [GORM](https://gorm.io) | v1.31.2 | ORM |
| [gorm/driver/postgres](https://github.com/go-gorm/postgres) | v1.6.2 | PostgreSQL driver |
| [go-redis/v9](https://github.com/redis/go-redis) | v9.21.0 | Redis client |
| [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | v5.3.1 | JWT token |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | v0.53.0 | bcrypt password hashing |
| [go-playground/validator/v10](https://github.com/go-playground/validator) | v10.30.3 | Request validation |
| [midtrans-go](https://github.com/midtrans/midtrans-go) | v1.3.8 | Midtrans Payment SDK (Top Up) |
| [xendit-go](https://github.com/xendit/xendit-go) | v7.0.0 | Xendit Payment SDK (Withdrawal) |
| [logrus](https://github.com/sirupsen/logrus) | v1.9.4 | Structured logging |
| [viper](https://github.com/spf13/viper) | v1.21.0 | Configuration / env |
| [google/wire](https://github.com/google/wire) | v0.7.0 | Compile-time dependency injection |
| [swaggo/swag](https://github.com/swaggo/swag) | v1.16.6 | OpenAPI/Swagger documentation |
| [mockery](https://github.com/vektra/mockery) | v2 (CLI) | Mock generation for unit tests |

**Infrastructure:** PostgreSQL 16 (Alpine), Redis 7 (Alpine)

---

## Architecture

The project follows **Clean Architecture** with three explicit layers:

```
Handler  →  Usecase  →  Repository
```

- **Handler** — Parses HTTP requests, validates input, reads JWT context, calls the appropriate usecase, and formats the response. No business logic.
- **Usecase** — Contains all business logic (idempotency, locking, balance mutation, ownership checks, payment gateway orchestrations). Each domain has its own usecase interface.
- **Repository** — Data access only. Abstracts GORM queries and Redis calls behind interfaces, making the usecase layer independent of persistence details.

Dependency wiring is handled at startup by **Google Wire** (`wire.go` + generated `wire_gen.go`).

### Domain Boundary & FK Design Decision

The project is split into two bounded contexts:

**`internal/auth`** — Identity & Auth domain (`users` table)
**`internal/wallet` & `internal/payment`** — Financial domains (`wallets`, `transfers`, `transactions`, `idempotency_keys`, `payment_transactions` tables)

**Why `wallets.user_id` has no FK to `users.id`:**
This is an intentional architectural decision. In real-world financial systems, identity/auth is frequently a separate service or an external Identity Provider (IDP). Enforcing a DB-level FK from `wallets` to `users` would create a hard coupling between two contexts that are designed to be independent. The wallet domain only needs to know a `user_id` exists — it does not own the user record. This boundary makes the auth domain replaceable without touching the financial schema.

---

## Payment Gateway Integration

To simulate a real-world fintech product, the API integrates with two different payment gateways.

**Why 2 different providers?**
- **Midtrans** is used for **Top Up (Collection)** because its Snap checkout product is highly mature for receiving payments in Indonesia.
- **Xendit** is used for **Withdrawals (Disbursement/Payout)** because its disbursement API is superior and more widely used for sending money out to user bank accounts.

### Top-Up Flow (Midtrans)
1. Client calls `POST /wallets/topup/checkout` with an `Idempotency-Key`.
2. The server calls Midtrans to generate a Snap checkout URL and saves a `PENDING` `payment_transactions` record.
3. User completes the payment on Midtrans.
4. Midtrans sends an asynchronous callback to `POST /webhooks/midtrans`.
5. The server verifies the signature, looks up the pending transaction, and credits the user's wallet via the internal `WalletUsecase.TopUp`, leveraging the same idempotency mechanism to prevent double-crediting.

### Withdrawal Flow (Xendit)
1. Client calls `POST /wallets/withdraw` with an `Idempotency-Key` and bank details.
2. The server instantly debits the user's wallet (locking the funds) and issues a Payout request to Xendit.
3. If Xendit immediately fails the request, the withdrawal is reversed (funds are returned to the wallet). Otherwise, it stays `PENDING`.
4. Xendit processes the disbursement and sends an asynchronous callback to `POST /webhooks/xendit`.
5. The server verifies the `X-CALLBACK-TOKEN`. If the payout succeeded, the withdrawal is finalized. If it failed, the wallet transaction is reversed, safely refunding the user.

---

## Key Features

All features listed below are verified to exist in the codebase.

### ✅ Auto-provisioned Wallet on Registration

When a user registers (`POST /auth/register`), `AuthUsecase.Register` calls `WalletUsecase.CreateWallet(userID)` immediately after the user record is persisted. Wallet provisioning is **best-effort**: if it fails, the registration still succeeds and returns a `200`. The wallet can be retrieved lazily on first access to `GET /wallets/me`.

### ✅ Session Management via Redis Token Store

On login, the JWT token is stored in Redis with a TTL. The JWT middleware validates both the token's cryptographic signature **and** its presence in Redis. On logout, the token is deleted from Redis, effectively invalidating the session server-side without waiting for the JWT to naturally expire.

### ✅ Pessimistic Locking for Balance Mutations

All balance mutations (top-up, transfer, withdraw) use `SELECT ... FOR UPDATE` via GORM's `clause.Locking{Strength: "UPDATE"}` inside a database transaction. This prevents lost-update race conditions under concurrent requests targeting the same wallet.

### ✅ Ordered Lock Acquisition to Prevent Deadlocks

In the transfer flow, when two wallets must be locked simultaneously, the wallet with the **smaller ID is always locked first**, regardless of who is the sender or recipient. This consistent ordering eliminates the circular-wait condition that causes deadlocks between two concurrent reverse-direction transfers.

See [Concurrency Safety Evidence](docs/deadlock-demo.md) for proof this was reproduced and verified.

### ✅ Idempotency Key Mechanism

Crucial endpoints (`POST /wallets/topup`, `POST /wallets/transfer`, `POST /wallets/withdraw`, `POST /wallets/topup/checkout`) require an `Idempotency-Key` request header. The mechanism:

1. Attempts to `INSERT` a new record with status `PROCESSING` — leveraging the DB `UNIQUE` constraint on `idem_key` as the concurrency-safe claim primitive.
2. On duplicate key: fetches the existing record and compares a SHA-256 hash of the request payload to detect mismatched reuse (returns `409 IDEMPOTENCY_KEY_CONFLICT`).
3. If the same key + same payload and status is `COMPLETED`: returns the cached response body directly, without re-executing the operation.
4. After successful execution: updates the record to `COMPLETED` with the serialized response body.
5. On failure: marks the record as `FAILED`.

### ✅ Monetary Values as Integer (BIGINT)

All balance and amount fields use `int64` in Go and `BIGINT` in PostgreSQL. No floating-point types are used anywhere in the money-handling path, avoiding float precision issues for currency representation.

---

## API Endpoints

Base path: `/api/v1`
All protected routes require `Authorization: Bearer <token>` header.

> **API Documentation**: A complete OpenAPI specification is automatically generated using Swagger. When running locally, visit `http://localhost:8080/swagger/index.html` to view and interact with the endpoints.

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/auth/register` | — | Register new user. Provisions wallet best-effort. |
| `POST` | `/auth/login` | — | Login. Returns a JWT token stored in Redis. |
| `POST` | `/auth/logout` | ✅ JWT | Invalidates the session token in Redis. |

### Wallet & Payment

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/wallets/me` | ✅ JWT | Returns the authenticated user's wallet. |
| `POST` | `/wallets/topup/checkout` | ✅ JWT | Initiate top-up via Midtrans. Returns a redirect URL. Requires `Idempotency-Key`. |
| `POST` | `/wallets/topup` | ✅ JWT | Internal top-up (bypass gateway). Requires `Idempotency-Key`. |
| `POST` | `/wallets/transfer` | ✅ JWT | Transfer to another user. Requires `Idempotency-Key`. |
| `POST` | `/wallets/withdraw` | ✅ JWT | Initiate withdrawal via Xendit. Requires `Idempotency-Key`. |

### Webhooks

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `POST` | `/webhooks/midtrans` | ❌ (unsigned) | Midtrans notification callback. Verified via payload signature. |
| `POST` | `/webhooks/xendit` | ❌ (header token) | Xendit payout callback. Verified via `X-CALLBACK-TOKEN` header. |

### Transactions

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/transactions` | ✅ JWT | List transaction history with pagination and filters. |
| `GET` | `/transactions/:id` | ✅ JWT | Get a single transaction detail. Ownership-checked. |

---

## CI/CD

Continuous Integration is enforced via GitHub Actions:

- **CI Workflow (`ci.yml`)**: Runs on every push/PR to `main`. Executes `go vet`, `golangci-lint`, standard unit tests (`go test`), and builds the application.
- **Integration Test Workflow (`integration.yml`)**: Runs on every push/PR to `main`. Spins up a PostgreSQL 16 container, runs migrations, and executes the `-tags=integration -race` test suite against the real database container to verify concurrency safety mechanisms.

---

## Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) & Docker Compose
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) — used as a CLI tool:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 1. Clone and configure environment

```bash
git clone https://github.com/Mpayy/digital-wallet-api.git
cd digital-wallet-api

cp .env.example .env
```

Edit `.env` and fill in all required values (including Midtrans and Xendit keys).

### 2. Start infrastructure services

```bash
docker compose up -d postgres redis
```

Wait a few seconds for PostgreSQL to finish initializing before running migrations.

### 3. Run database migrations

```bash
make migrate-up
# Or manually:
# migrate -path migrations -database "postgres://postgres:postgres@127.0.0.1:5432/digital_wallet_api?sslmode=disable&x-multi-statement=true" up
```

### 4. Run the application

**Option A — with Docker Compose (recommended):**

```bash
docker compose up --build
```

**Option B — run locally:**

```bash
go run ./cmd/api
```

Requires PostgreSQL and Redis to be running and accessible at the hosts/ports defined in `.env`.

---

## Database Schema Overview

| Table | Key Columns | Notes |
|---|---|---|
| `users` | `id`, `email` (UNIQUE), `password` | Auth domain. No FK to other tables. |
| `wallets` | `id`, `user_id` (UNIQUE), `balance BIGINT` | `user_id` has no FK to `users` — intentional (see Architecture). |
| `transfers` | `id`, `from_wallet_id`, `to_wallet_id`, `amount`, `note` | FK → `wallets` on both wallet columns. |
| `transactions` | `id`, `wallet_id`, `type`, `amount`, `balance_before`, `balance_after`, `transfer_id` (nullable), `status` | FK → `wallets`, FK → `transfers`. |
| `idempotency_keys` | `id`, `idem_key` (UNIQUE), `user_id`, `endpoint`, `request_hash CHAR(64)`, `status`, `response_body TEXT` | No FK to `users` — same boundary rationale. |
| `payment_transactions`| `id`, `provider`, `provider_ref_id`, `user_id`, `type`, `amount`, `status`, `wallet_transaction_id` | Gateway records tracking external payment states. |

Transaction types: `TOPUP`, `TRANSFER_IN`, `TRANSFER_OUT`, `WITHDRAWAL`
Transaction statuses: `SUCCESS`, `FAILED`
Idempotency statuses: `PROCESSING`, `COMPLETED`, `FAILED`
Payment statuses: `PENDING`, `SETTLED`, `FAILED`

---

## Testing

### Unit Tests

Run with the standard Go test command — no infrastructure required. All external dependencies (GORM, Redis, Gateways, etc.) are mocked using [Mockery](https://github.com/vektra/mockery)-generated mocks.

```bash
make test-unit
```

**Coverage (verified from file count):**

| Module | File | Test Cases |
|---|---|---|
| `wallet/usecase` | `wallet_usecase_test.go` | 18 |
| `wallet/usecase` | `transfer_usecase_test.go` | 25 |
| `wallet/usecase` | `idempotency_service_test.go` | 18 |
| `wallet/usecase` | `transaction_usecase_test.go` | 14 |
| `payment/usecase` | `payment_usecase_test.go` | 21 |
| `payment/usecase` | `withdrawal_usecase_test.go` | 20 |
| `auth/usecase` | `auth_usecase_test.go` | 19 |
| `auth/middleware` | `jwt_middleware_test.go` | 12 |
| **Total** | | **147** |

### Integration Tests

Require Docker. Uses a **dedicated PostgreSQL instance on port 5433** via `docker-compose.test.yml` (separate from the dev DB, using `tmpfs` for speed). Run with:

```bash
make test-integration
```

The `-race` flag is passed intentionally to catch Go-level data races in addition to the database-level correctness assertions.

**Integration test scenarios:**

| Test | File | What it proves |
|---|---|---|
| `TestConcurrentTopUp_NoLostUpdate` | `wallet_topup_test.go` | 20 goroutines top-up the same wallet concurrently with unique idempotency keys. Asserts final balance and transaction rows verify no write is lost under `SELECT FOR UPDATE`. |
| `TestConcurrentTransfer_OppositeDirection_NoDeadlock` | `wallet_transfer_test.go` | 50 iterations of A→B and B→A transfers fired simultaneously (100 goroutines). Asserts no deadlock (15s timeout), total money is conserved, and exactly 200 transaction rows exist. |
| `TestConcurrentTopUp_SameIdempotencyKey_OnlyAppliedOnce` | `wallet_idempotency_test.go` | 20 goroutines fire the same top-up with the **same idempotency key**. Asserts balance increases only once, 1 transaction row exists, and all successful responses share the same `transaction_id`. |

---

## Project Status

This project is **in active development**. The core API is functional and the critical financial consistency mechanisms (locking, idempotency, atomic transactions) are implemented and tested. The application logic is mature but it's currently awaiting full deployment.

### ✅ Completed

- User registration and login with bcrypt password hashing
- JWT-based authentication with Redis session store and logout invalidation
- Wallet top-up with pessimistic locking (`SELECT FOR UPDATE`) and idempotency
- Peer-to-peer transfer with ordered lock acquisition (deadlock prevention) and idempotency
- Transaction history with pagination and filtering by type/date range
- Transaction detail with ownership enforcement
- Structured logging (logrus) across all layers
- Dockerized with Docker Compose (3-service stack: app, postgres, redis)
- **147 unit test cases** using Mockery mocks
- **3 integration tests** covering concurrent operations against real PostgreSQL with `-race` flag
- Deadlock reproduced, documented, and fixed
- Integration with **Midtrans** for top-up collections
- Integration with **Xendit** for withdrawal payouts
- Automated CI and Integration Test pipelines via GitHub Actions
- Migration to PostgreSQL
- Swagger/OpenAPI documentation

### 🔧 In Progress / Planned

| Item | Status |
|---|---|
| Async webhook processing via message broker (RabbitMQ) | 📋 Planned |
| Retry mechanism for database deadlock errors | 📋 Planned (defensive) |
| TTL / reclaim mechanism for idempotency keys stuck in `PROCESSING` status | 📋 Planned |
