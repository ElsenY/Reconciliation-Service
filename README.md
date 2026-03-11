# Reconciliation Service

A transaction reconciliation API service that identifies unmatched and discrepant transactions between internal system data and external bank statements.

## Prerequisites

- **Go 1.24.4+**

## How to Run

1. **Create the .env file** in the main directory (Can just copy .env.example file and rename it to .env to run default settings with sample csv data located in testdata/):

   ```bash
   cp .env.example .env
   ```

2. **Bootstrap the go app**
    ```bash
    go mod tidy
    ```
3. **Start the server:**

   ```bash
   go run cmd/main.go
   ```

   The server listens on the port set in `APP_PORT` (default `8001`). You should see:


4. **Call the reconcile API** with form-data (see [API](#api) and [cURL examples](#curl-examples) below).

## .env configuration

| Variable             | Default | Description                                                |
|----------------------|---------|------------------------------------------------------------|
| `APP_PORT`           | `8001`  | Port the HTTP server listens on                            |
| `DEFAULT_SYSTEM_CSV` | testdata/system_transactions.csv | Default system transactions CSV path (used if no upload)  |
| `DEFAULT_BANK_CSV`   | testdata/bank_bca.csv,testdata/bank_bni.csv,testdata/banka.csv,testdata/bankb.csv,testdata/bankc.csv | Comma-separated default bank statement CSV paths            |

Example `.env`:

```
APP_PORT=8001
DEFAULT_SYSTEM_CSV=testdata/system_transactions.csv
DEFAULT_BANK_CSV=testdata/bank_bca.csv,testdata/bank_bni.csv,testdata/banka.csv,testdata/bankb.csv,testdata/bankc.csv
```

> There are also provided shorter version of the csv files named ```system_transactions_short.csv``` and ```bank_short.csv``` you can replace the .env file with these files to see the discrepancies more clearly

## API

### `POST /api/reconcile`

Reconcile system transactions against bank statements.

**Content-Type:** `multipart/form-data`

| Field                  | Type     | Required | Description                                    |
|------------------------|----------|----------|------------------------------------------------|
| `start_date`           | string   | **Yes**  | Start date `YYYY-MM-DD`                        |
| `end_date`             | string   | **Yes**  | End date `YYYY-MM-DD`                          |
| `system_transactions`  | file     | No*      | System transactions CSV file                   |
| `bank_statements`      | file(s)  | No*      | One or more bank statement CSV files           |

\* If files are not sent, the server uses `DEFAULT_SYSTEM_CSV` and `DEFAULT_BANK_CSV`. If those are not set, the request returns `400`.

### cURL examples (run in the root repository folder)

**Using default files** (only date range; server reads from `DEFAULT_SYSTEM_CSV` and `DEFAULT_BANK_CSV`):

```bash
curl -X POST http://localhost:8001/api/reconcile \
  -F "start_date=2026-03-01" \
  -F "end_date=2026-03-31"
```

**With uploaded system transactions and one bank file:**

```bash
curl -X POST http://localhost:8001/api/reconcile \
  -F "start_date=2026-03-01" \
  -F "end_date=2026-03-31" \
  -F "system_transactions=@testdata/system_transactions.csv" \
  -F "bank_statements=@testdata/bank_bca.csv"
```

**With system transactions and multiple bank statement files:**

```bash
curl -X POST http://localhost:8001/api/reconcile \
  -F "start_date=2026-03-01" \
  -F "end_date=2026-03-31" \
  -F "system_transactions=@testdata/system_transactions.csv" \
  -F "bank_statements=@testdata/bank_bca.csv" \
  -F "bank_statements=@testdata/bank_bni.csv" \
  -F "bank_statements=@testdata/banka.csv"
```


### Response

Success response shape:

```json
{
  "code":200,
  "status": "success",
  "message": "Reconciliation data fetched",
  "data": {
    "total_statements_processed": 320,
    "total_system_transactions_processed": 250,
    "total_bank_statements_processed": 70,
    "total_matched_transactions": 200,
    "total_unmatched_transactions": 75,
    "missing_in_bank": [
      { "id": "TRX042", "amount": 318000, "date": "2026-03-18T18:05:06Z", "type": "CREDIT" }
    ],
    "missing_in_system": {
      "bank_bca": [
        { "id": "BCA0099", "amount": 80000, "date": "2026-03-19T00:00:00Z" }
      ]
    },
    "total_discrepancy_amount": 12500,
    "discrepancies": [
      {
        "system_trx_id": "TRX004",
        "bank_identifier": "BCA004",
        "system_amount": 730860,
        "bank_amount": 720000,
        "difference": 10860,
        "bank_source": "bank_bca"
      }
    ]
  }
}
```

### `GET /health`

Health check.

```bash
curl http://localhost:8001/health
# {"status":"ok"}
```

## Architecture

The project uses a **3-layered architecture** with Go Fiber:

| Layer          | Responsibility                                                     |
|----------------|--------------------------------------------------------------------|
| **Handler**    | Accepts multipart form-data, delegates to service, returns JSON    |
| **Service**    | Reconciliation logic: match by date + amount, report discrepancies |
| **Repository** | Reads CSV from file paths or `io.Reader` into domain models       |

```
cmd/main.go                    # Entry point, Fiber server
internal/handler/              # POST /api/reconcile
internal/service/              # Matching algorithm
internal/repository/           # CSV parsing
internal/model/                # Transaction, BankStatement
internal/dto/                  # Request/Response DTOs
internal/helper/               # Generic helper functions
testdata/                      # Sample CSV files
```

## CSV Formats

**System transactions** (`trxID,amount,type,transactionTime`). (amount in float matching precise up to 3 numbers behind decimal):

```csv
trxID,amount,type,transactionTime
TRX001,504230,CREDIT,2026-03-04T11:08:30
TRX002,99290,DEBIT,2026-03-09T08:08:43
```

**Bank statements** (`unique_identifier,amount,date`). (amount in float matching precise up to 3 numbers behind decimal),negative amount = debit

```csv
unique_identifier,amount,date
BANKA0001,504230,2026-03-04
BCA0002,-2530,2026-03-07
```

> Bank amounts are normalized to absolute values (3 numbers behind decimals) for matching if there's comma; negative or positive is used only to indicate debit/credit.

## Design Decisions

- **3-layered architecture**: using handler/service/repository architecture to enforce separate responsibility for each layer
- **Interface-driven layers**: Both `TransactionRepository` and `ReconciliationService` are defined as interfaces, enabling easy mocking in tests and swapping implementations.
- **Model vs DTO separation**: Domain models (`Transaction`, `BankStatement`) are kept separate from request/response DTOs with JSON tags for clean API responses.
- **Efficient records matching**: Instead of looping and matching record one by one O(N*M), we use hash map to store records and match it with O(1) time, resulting in overall time complexity to O(N+M) where N is the number of system transactions, and M is the number of combined bank statements