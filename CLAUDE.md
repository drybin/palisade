# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
make build          # Build binary in Docker (golang:1.25), outputs ./palisade

# Testing
make unit-test      # go test ./internal/... ./pkg/... with -race flag

# Code quality
make lint           # golangci-lint with 3-minute timeout

# Dependencies
make tidy           # go mod tidy
make upvendors      # go get -u ./...

# Code generation
make gensqlc        # Regenerate DB query code from sqlc/query.sql via Docker
make genmocks       # Generate mocks from mock.go files

# Run a command
go run ./cmd/cli/main.go <command>
# e.g.: go run ./cmd/cli/main.go get-coin-list
```

## Architecture

Clean (hexagonal) architecture with four layers:

```
Presentation (command/)  →  Application (usecase/)  →  Domain (service/, model/, repo/)  →  Adapter (webapi/, pg/)
```

- **`cmd/cli/main.go`** — entry point; loads `.env`, validates config, runs CLI
- **`internal/app/cli/registry/container.go`** — dependency injection; wires all adapters, services, and use cases
- **`internal/app/cli/usecase/`** — business logic orchestration; one file per CLI command
- **`internal/domain/service/`** — core trading logic (flat detection, level calculation, order placement)
- **`internal/adapter/webapi/`** — MEXC REST API clients (`mexc.go` = v1/v3, `mexcV2.go` = OTC endpoint, `telegram.go`)
- **`internal/adapter/pg/state.go`** — PostgreSQL repository implementation
- **`internal/domain/repo/`** — repository interfaces (depends on nothing concrete)
- **`sqlc/`** — schema + queries; run `make gensqlc` after changes; generated code goes to `sqlc/gen/`

## Database

PostgreSQL 16 via `docker-compose.yml`. Key tables:

- **`coins`** — MEXC coin metadata + palisade analysis results (`isPalisade`, `support`, `resistance`, `volatility`, etc.)
- **`trade_log`** — lifecycle of each trade: `open_date` (buy placed) → `deal_date` (buy filled) → `close_date` (sell filled) or `cancel_date`
- **`state`** / **`logs`** — current trading state and historical snapshots

DB timezone is set to `Asia/Bangkok` (GMT+7) in every session.

## Trading Strategy ("Palisade")

The bot detects **flat/sideways consolidation** on 15-minute klines (last ~4 hours):

1. Fetch klines → compute min, max, avg, volatility
2. Mark coin as "palisade" if `(max−min)/avg < MaxVolatilityPercent` (default 5%)
3. `support` = min price (buy target), `resistance` = max price (sell target)
4. Target coins: `isPalisade=true`, `volatility` between 0.2–0.5, USDT quote pair

Typical workflow:
```
get-coin-list            # Populate coins table from MEXC
check-palisade-coin-list # Scan coins for flat patterns
process / process-multi  # Place buy orders at support
process-sell             # Monitor open orders; place sell at resistance when buy fills
```

## Configuration

All config comes from `.env` (loaded by `godotenv`). Required fields validated in `internal/app/cli/config/config.go`:

| Variable | Purpose |
|---|---|
| `MEXC_API_KEY` / `MEXC_SECRET` | Trading credentials |
| `MEXC_API_URL` | `https://api.mexc.com` (v1/v3) |
| `MEXC_API_URL_V2` | `https://otc.mexc.com` (V2/OTC) |
| `POSTGREE_DSN` | Full PostgreSQL connection string |
| `TG_BOT_TOKEN` / `TG_CHAT_ID` | Telegram notifications (optional) |

## Key Patterns

- **Repository interfaces** live in `internal/domain/repo/`; implementations in `internal/adapter/`. Never import adapters from the domain layer.
- **sqlc-generated code** in `sqlc/gen/` is checked in — edit `sqlc/query.sql` and regenerate rather than editing generated files directly.
- The local `pkg/mexcsdk/` module is a thin wrapper around the MEXC SDK; its `README.md` documents the available API methods.
- Telegram notifications are sent from use cases via the `telegram.go` adapter for trade events and errors.
