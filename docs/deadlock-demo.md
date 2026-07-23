# Deadlock Reproduction & Fix — Transfer Usecase

This document provides a detailed account of how MySQL Error 1213 (deadlock) was reproduced intentionally, captured, and then eliminated by applying ordered lock acquisition in the transfer flow.

---

## Background

The transfer operation must lock two wallet rows simultaneously using `SELECT ... FOR UPDATE`. Without a consistent lock ordering, two concurrent reverse-direction transfers can enter a circular wait:

```
Goroutine 1 (A→B): locks wallet A, waits for wallet B
Goroutine 2 (B→A): locks wallet B, waits for wallet A
                   ↑ deadlock: each waits for the other forever
```

MySQL detects this and terminates one transaction with **Error 1213 (40001)**.

---

## How the Deadlock Was Reproduced

The fix in `transfer_usecase.go` is the ordered-lock block:

```go
firstID, secondID := fromWallet.ID, toWallet.ID
if firstID > secondID {
    firstID, secondID = secondID, firstID
}
```

To reproduce the deadlock, this swap was **temporarily commented out**, reverting to naive order (lock sender's wallet first, always):

```go
// Naive (broken) version — causes deadlock under concurrent reverse transfers
firstID, secondID := fromWallet.ID, toWallet.ID
// if firstID > secondID {
//     firstID, secondID = secondID, firstID   ← this line removed
// }
```

---

## Test Used

The integration test `TestConcurrentTransfer_OppositeDirection_NoDeadlock` in `test/integration/wallet_transfer_test.go` was the vehicle:

- **Setup:** Wallet A (ID=1, balance=1,000,000), Wallet B (ID=2, balance=1,000,000)
- **Load:** 50 iterations × 2 goroutines = 100 goroutines total, firing A→B and B→A transfers of 1,000 simultaneously
- **Timeout detector:** `time.After(15 * time.Second)` — if any goroutine hangs, the test fails immediately as a deadlock signal

**Command:**

```bash
make test-integration
# or directly:
go test -tags=integration -race ./test/integration/... -v -run TestConcurrentTransfer_OppositeDirection_NoDeadlock
```

---

## Output Without Fix (Deadlock Reproduced)

GORM's logger printed **Error 1213** directly to stdout for every transaction that MySQL aborted:

```
Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
[3.023ms] [rows:0] SELECT * FROM `wallets` WHERE `wallets`.`id` = 2 ... FOR UPDATE

Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
[0.690ms] [rows:0] SELECT * FROM `wallets` WHERE `wallets`.`id` = 2 ... FOR UPDATE

Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
[1.394ms] [rows:0] SELECT * FROM `wallets` WHERE `wallets`.`id` = 1 ... FOR UPDATE
```

The errors cascaded across both wallet IDs (1 and 2) as MySQL alternated which transaction it chose to roll back as the "deadlock victim."

The test also reported these assertion failures:

```
wallet_transfer_test.go:63: unexpected transfer error: lock recipient wallet:
    Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction

wallet_transfer_test.go:70:
    expected: 1000000
    actual  : 990000      ← Wallet A balance corrupted

wallet_transfer_test.go:71:
    expected: 1000000
    actual  : 1010000     ← Wallet B balance corrupted (money created from thin air)

wallet_transfer_test.go:76:
    expected: int(200)
    actual  : int64(96)   ← Only 96 of 200 expected transaction rows were written
--- FAIL: TestConcurrentTransfer_OppositeDirection_NoDeadlock (0.69s)
```

**Key observations from the failed run:**
- Balance totals were wrong: 990,000 + 1,010,000 = 2,000,000 (total is conserved, but individual balances are skewed — some transfers' debits committed while their credits rolled back, or vice versa)
- Only 96 transaction rows written instead of 200, confirming 104 transfer operations were aborted and never retried

The screenshot below was captured during this failed run:

![MySQL Error 1213 — deadlock reproduced without ordered locking](images/deadlock-error.png)

---

## The Fix

The ordered-lock swap was restored:

```go
firstID, secondID := fromWallet.ID, toWallet.ID
if firstID > secondID {
    firstID, secondID = secondID, firstID
}
// LockByID(tx, firstID) always runs before LockByID(tx, secondID)
// This guarantees a consistent global lock ordering regardless of transfer direction
```

**Why this works:** If every goroutine always acquires lock for the lower-ID wallet before the higher-ID wallet, no two goroutines can form a cycle. Goroutine processing A→B (ID 1 first, then ID 2) and goroutine processing B→A (also ID 1 first, then ID 2) — both contend on ID 1 first, so one waits instead of creating a circular dependency.

---

## Result With Fix

Running the same test with the fix in place:

- No `Error 1213` errors in the log
- Final balances: `walletA = 1,000,000`, `walletB = 1,000,000` ✅
- Total money conserved: `1,000,000 + 1,000,000 = 2,000,000` ✅
- Transaction row count: exactly `200` (50 iterations × 2 directions × 2 rows per transfer) ✅
- Test passes within timeout ✅

---

## Files Referenced

| File | Role |
|---|---|
| `internal/wallet/usecase/transfer_usecase.go` | Contains the ordered-lock fix |
| `internal/wallet/repository/wallet_repository.go` | `LockByID` — `SELECT ... FOR UPDATE` implementation |
| `test/integration/wallet_transfer_test.go` | Integration test that caught and verified the fix |
| `docs/images/deadlock-error.png` | Screenshot of GORM output showing Error 1213 |
