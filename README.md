# Digital Wallet API

[![CI](https://github.com/Mpayy/digital-wallet-api/actions/workflows/ci.yml/badge.svg)](https://github.com/Mpayy/digital-wallet-api/actions/workflows/ci.yml)
[![Integration Test](https://github.com/Mpayy/digital-wallet-api/actions/workflows/integration.yml/badge.svg)](https://github.com/Mpayy/digital-wallet-api/actions/workflows/integration.yml)

A RESTful digital wallet API built with Go, simulating core features of mobile wallet applications like Dana or OVO — covering user registration, wallet top-up, peer-to-peer transfers, withdrawals, and transaction history.

> **Disclaimer:** This is a portfolio/simulation project. It is not a licensed payment system and is not intended for production financial use. The application is not yet deployed to a cloud environment; it currently runs locally or via Docker Compose.

---

## Tech Stack

All dependencies listed below are verified from `go.mod`:

| Dependency | Version | Role |
|---|---|---|
| [Gin](https://github.com/gin-gonic/gin) | v1.12.0 | HTTP framework |
| [GORM](https://gorm.io) | v1.31.2 | ORM |
| [gorm/driver/postgres](https://github.com/go-gorm/postgres) | v1.6.2 | PostgreSQL driver |
| [amqp091-go](github.com/rabbitmq/amqp091-go) | v1.13.0 | RabbitMQ Client |
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

**Infrastructure:** 
- **Database**: PostgreSQL 16 (Local/Docker) / Neon (Cloud)
- **Cache**: Redis 7 (Local/Docker) / Upstash (Cloud)
- **Message Broker**: RabbitMQ 3 (Local/Docker) / CloudAMQP (Cloud)

---

## Architecture

The project follows **Clean Architecture** with three explicit layers:

```
Handler  →  Usecase  →  Repository
```

- **Handler** — Parses HTTP requests, validates input, reads JWT context, calls the appropriate usecase, and formats the response. No business logic.
- **Usecase** — Contains all business logic (idempotency, locking, balance mutation, ownership checks, payment gateway orchestrations). Each domain has its own usecase interface.
- **Repository** — Data access only. Abstracts GORM queries and Redis calls behind interfaces, making the usecase layer independent of persistence details.

### Dual-Binary Design

The application is explicitly separated into two running services:
1. **`cmd/api`**: The synchronous HTTP server handling client requests.
2. **`cmd/worker`**: A background consumer that processes asynchronous tasks from RabbitMQ.

**Why are webhooks processed asynchronously?**
Payment gateways (like Midtrans and Xendit) require instant HTTP 200 responses to their webhooks. If the API were to process database-heavy operations (which include `SELECT FOR UPDATE` locks that could block or delay) synchronously, it might cause the gateway to timeout and repeatedly retry the webhook. By separating the concern, `cmd/api` only verifies the webhook's signature and immediately publishes the raw payload to RabbitMQ. The `cmd/worker` then consumes the payload and safely executes the heavy, lock-dependent state updates at its own pace.

### Domain Boundary & FK Design Decision

The project is split into two bounded contexts:

**`internal/auth`** — Identity & Auth domain (`users` table)
**`internal/wallet` & `internal/payment`** — Financial domains (`wallets`, `transfers`, `transactions`, `idempotency_keys`, `payment_transactions` tables)

**Why `wallets.user_id` has no FK to `users.id`:**
This is an intentional architectural decision. In real-world financial systems, identity/auth is frequently a separate service or an external Identity Provider (IDP). Enforcing a DB-level FK from `wallets` to `users` would create a hard coupling between two contexts that are designed to be independent. The wallet domain only needs to know a `user_id` exists — it does not own the user record. This boundary makes the auth domain replaceable without touching the financial schema.

---

## Async Processing & Reliability

Robust background processing is critical for financial consistency. The webhook pipeline handles external payment updates (Midtrans/Xendit) using RabbitMQ to guarantee processing delivery without impacting HTTP performance.

**Pipeline Flow:**
1. **Verify (API):** The webhook hits the API. The API validates the signature/token.
2. **Publish (API):** Once valid, the raw payload is published to a specific RabbitMQ queue (e.g., `midtrans.webhooks`).
3. **Consume (Worker):** The worker picks up the message using a `qos` configuration to process tasks safely.
4. **Process (Worker):** Database transactions and balance updates happen here. On success, the message is `Ack`ed.

**Dead-Letter Queue (DLQ):**
To ensure reliability without creating infinite retry loops, the RabbitMQ topology is configured with `x-delivery-limit` (set to 5 retries). 
If a message fails to process (e.g., database is down, or unexpected bug) and receives a `Nack` more than 5 times, it is automatically routed to a Dead Letter Exchange (DLX) and placed in a Dead Letter Queue (DLQ). 
This allows developers to inspect, debug, and manually redeliver permanently failing messages without clogging the main processing pipeline.

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
5. The API verifies the signature and publishes the payload to RabbitMQ.
6. The Worker processes the payload, crediting the user's wallet via `WalletUsecase.TopUp`, leveraging idempotency.

### Withdrawal Flow (Xendit)
1. Client calls `POST /wallets/withdraw` with an `Idempotency-Key` and bank details.
2. The server instantly debits the user's wallet (locking the funds) and issues a Payout request to Xendit.
3. If Xendit immediately fails the request, the withdrawal is reversed. Otherwise, it stays `PENDING`.
4. Xendit processes the disbursement and sends an asynchronous callback to `POST /webhooks/xendit`.
5. The API verifies the `X-CALLBACK-TOKEN` and publishes the payload to RabbitMQ.
6. The Worker processes the payload: if succeeded, the withdrawal is finalized. If failed, the wallet transaction is reversed, safely refunding the user.

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

### Webhooks (Async)

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `POST` | `/webhooks/midtrans` | ❌ (unsigned) | Midtrans notification callback. Verified via payload signature, then pushed to RabbitMQ. |
| `POST` | `/webhooks/xendit` | ❌ (header token) | Xendit payout callback. Verified via `X-CALLBACK-TOKEN` header, then pushed to RabbitMQ. |

### Transactions

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/transactions` | ✅ JWT | List transaction history with pagination and filters. |
| `GET` | `/transactions/:id` | ✅ JWT | Get a single transaction detail. Ownership-checked. |

---

## CI/CD

Continuous Integration is enforced via GitHub Actions:

- **CI Workflow (`ci.yml`)**: Runs on every push/PR to `main`. Executes `go vet`, `golangci-lint`, standard unit tests (`go test`), and builds both binaries (`api` and `worker`).
- **Integration Test Workflow (`integration.yml`)**: Runs on every push/PR to `main`. Spins up **PostgreSQL 16** and **RabbitMQ** containers, runs migrations, and executes the `-tags=integration -race` test suite to verify concurrency and message brokering mechanisms against real services.

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

### 2. Start all services using Docker Compose (Recommended)

The provided `docker-compose.yml` uses a multi-stage Dockerfile to build and run both the API and Worker containers alongside PostgreSQL, Redis, and RabbitMQ.

```bash
docker compose up --build -d
```
*Note: Wait a few seconds for PostgreSQL to finish initializing before running migrations.*

### 3. Run database migrations

```bash
make migrate-up
# Or manually:
# migrate -path migrations -database "postgres://postgres:postgres@127.0.0.1:5432/digital_wallet_api?sslmode=disable&x-multi-statement=true" up
```

### 4. Running locally without Docker Compose

If you prefer running the Go code on your host machine, you must run both binaries simultaneously. Requires PostgreSQL, Redis, and RabbitMQ to be accessible at the hosts/ports defined in `.env`.

**Terminal 1 (API Server):**
```bash
go run ./cmd/api
```

**Terminal 2 (Background Worker):**
```bash
go run ./cmd/worker
```
*Note: If you only run the API, webhooks will be accepted and queued, but the actual balance updates will never execute until the worker is started.*

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

Run with the standard Go test command — no infrastructure required. All external dependencies (GORM, Redis, RabbitMQ, Gateways, etc.) are mocked using [Mockery](https://github.com/vektra/mockery)-generated mocks.

```bash
make test-unit
```
*(Currently covering 147 test cases across Auth, Wallet, and Payment domains)*

### Integration Tests

Require Docker. Uses dedicated **PostgreSQL** and **RabbitMQ** instances on isolated ports via `integration.yml` logic. Run with:

```bash
make test-integration
```

The `-race` flag is passed intentionally to catch Go-level data races in addition to the database-level correctness assertions.

---

## Project Status

This project is **in active development**. The core API is functional, and the critical financial consistency mechanisms (locking, idempotency, atomic transactions, reliable asynchronous messaging) are implemented and tested.

### ✅ Completed

- User registration and login with bcrypt password hashing
- JWT-based authentication with Redis session store and logout invalidation
- Wallet top-up with pessimistic locking (`SELECT FOR UPDATE`) and idempotency
- Peer-to-peer transfer with ordered lock acquisition (deadlock prevention) and idempotency
- Transaction history with pagination and filtering by type/date range
- Transaction detail with ownership enforcement
- Integration with **Midtrans** for top-up collections
- Integration with **Xendit** for withdrawal payouts
- **Async webhook processing via RabbitMQ (Dual-Binary architecture)**
- Reliable dead-letter queue (DLQ) topology for failing background jobs
- Migration to PostgreSQL
- Swagger/OpenAPI documentation
- Dockerized with Docker Compose (5-service stack: api, worker, postgres, redis, rabbitmq)
- Automated CI and Integration Test pipelines via GitHub Actions
- 147 unit test cases and 3 heavy concurrent integration tests

### 🔧 In Progress / Planned

| Item | Status |
|---|---|
| Retry mechanism for database deadlock errors | 📋 Planned (defensive) |
| TTL / reclaim mechanism for idempotency keys stuck in `PROCESSING` status | 📋 Planned |
