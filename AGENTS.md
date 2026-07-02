# AGENTS.md

## Project

API metering gateway that counts LLM token usage. Transparent proxy for OpenAI, Google Gemini, and Anthropic APIs, etc.

## Commands

```bash
go build ./...              # build all packages
go vet ./...                # vet all packages
go run ./cmd/gateway        # start gateway (needs DATABASE_URL + provider keys)
```

### Code generation (after schema/query/proto changes)

```bash
sqlc generate               # SQL queries -> Go (internal/repository/)
buf generate                 # .proto -> Go (proto/hypertoken/v1/)
```

**Order matters**: `sqlc generate` then `buf generate`. Run `go mod tidy` if buf introduces new dependencies.

## Architecture

Two ports in one binary:

- **Proxy** (`PROXY_ADDR`, default `:8080`) — transparent reverse proxy. Client sends gateway API key via `Authorization: Bearer ht_...`, gateway swaps in the real provider key and forwards to upstream.
- **Admin** (`ADMIN_ADDR`, default `:9090`) — connect-go management API (gRPC + REST) for API key CRUD and usage queries.

## Key concepts

- Module path: `hyper-token` (not `HyperToken`).
- Go version: 1.26.4.
- Token counting works for both streaming (SSE tee reader) and non-streaming responses.

## Database

- PostgreSQL via `lib/pq`. Migrations in `db/migrations/`, applied at startup via `golang-migrate`.
- Three tables: `api_keys`, `token_usages`, `token_aggregations`. Aggregations use upsert (daily rollup).
- sqlc queries in `db/queries/`, generates code into `internal/repository/`.

## Directory map

| Path                   | Purpose                                               |
| ---------------------- | ----------------------------------------------------- |
| `cmd/gateway/`         | Binary entrypoint                                     |
| `proto/`               | Protobuf + generated code (both pb.go and connect.go) |
| `db/`                  | Migrations + sqlc queries                             |
| `internal/proxy/`      | Per-provider reverse proxy handlers                   |
| `internal/tokenizer/`  | Response parsing for usage extraction                 |
| `internal/auth/`       | API key generation, hashing, extraction               |
| `internal/repository/` | sqlc-generated DB code + business logic wrapper       |
| `internal/server/`     | connect-go management service implementation          |
| `internal/config/`     | Environment-based configuration                       |
