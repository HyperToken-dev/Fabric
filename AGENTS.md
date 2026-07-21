# AGENTS.md

## Project

Fabric is a modular AI gateway framework. The current implementation is an OpenAI-compatible gateway with:

- Transparent OpenAI API proxying.
- API key, channel, model, and usage-log management APIs.
- PostgreSQL-backed storage with startup migrations.
- Optional sensitive-word detection for OpenAI chat completion requests.

Future provider support is planned, but current proxy code is OpenAI-focused.

## Commands

```bash
go build ./...              # build all packages
go vet ./...                # vet all packages
go test ./...               # run tests
go run ./cmd/gateway        # start gateway locally; reads configs/config.yaml

docker compose up -d        # start Fabric and PostgreSQL; uses configs/config.docker.yaml
docker compose down         # stop Docker services

make generate               # sqlc generate, then buf generate
make build                  # generate, then go build ./...
make run                    # go run ./cmd/gateway
make lint                   # go vet ./...
make test                   # go test ./...
make sqlc-check             # sqlc compile
```

### Code generation (after schema/query/proto changes)

```bash
sqlc generate               # SQL queries -> Go (internal/repository/)
buf generate                 # .proto -> Go (gen/)
```

**Order matters**: run `sqlc generate` before `buf generate`. Run `go mod tidy` if code generation introduces or removes module dependencies.

## Architecture

Two ports in one binary:

- **Proxy** (`proxyAddr`) - OpenAI-compatible proxy routes. Clients send gateway API keys via `Authorization: Bearer <key>`; the gateway resolves the configured channel/model credentials and forwards to upstream.
- **Admin** (`adminAddr`) - connect-go management APIs for API keys, channels, models, and usage logs.

Configuration paths depend on startup mode:

- Local Go runs read `configs/config.yaml`.
- Docker Compose uses tracked `configs/config.docker.yaml`, mounted by `compose.yaml` as `/app/configs/config.yaml` inside the Fabric container.

Startup flow in `cmd/gateway/main.go`:

1. Load `configs/config.yaml` with Viper.
2. Initialize zap logging.
3. Optionally load sensitive-word dictionaries from `configs/stwd/`.
4. Run PostgreSQL migrations from `db/migrations/`.
5. Open the PostgreSQL connection and initialize sqlc repositories.
6. Register proxy routes and connect-go admin handlers.

## Working Principles

- Handle every returned error. Ignoring errors is prohibited.
- Keep changes small and localized. Prefer the existing package boundaries and patterns.
- Do not add source files under generated-code directories.
- Do not edit generated files directly; update schema/query/proto inputs and regenerate.

## Key concepts

- Module path: `fabric`.
- Go version: 1.26.4.
- Configuration is YAML-based via Viper. Local runs use `configs/config.yaml`; Docker Compose uses `configs/config.docker.yaml` mounted as container `configs/config.yaml`.
- The OpenAI proxy injects `stream_options.include_usage=true` for streaming chat completions so usage can be captured.
- Sensitive-word detection is controlled by `sensitiveWordDetect` in config.

## Database

- PostgreSQL via `lib/pq`. Migrations in `db/migrations/`, applied at startup via `golang-migrate`.
- Current tables include `channels`, `api_keys`, `models`, and `usage_logs`.
- sqlc queries in `db/queries/`, generates code into `internal/repository/`.

## Directory map

| Path                   | Purpose                                        |
| ---------------------- | ---------------------------------------------- |
| `compose.yaml`         | Docker Compose stack for Fabric + PostgreSQL   |
| `Dockerfile`           | Multi-stage Docker build for the gateway       |
| `cmd/gateway/`         | Binary entrypoint                              |
| `core/proxy/`          | Provider-neutral proxy primitives              |
| `core/providers/`      | Provider-specific core proxy implementations   |
| `business/usage/`      | Usage handling business logic                  |
| `business/sensitive/`  | Sensitive-word detection and policy logic      |
| `proto/`               | Protobuf definitions                           |
| `db/`                  | Migrations + sqlc queries                      |
| `configs/`             | Runtime config and sensitive-word dictionaries |
| `internal/auth/`       | API key generation, hashing, extraction        |
| `internal/repository/` | sqlc-generated DB code                         |
| `gen/`                 | buf generated code                             |
| `logger/`              | Logger construction                            |
| `internal/server/`     | connect-go management service implementation   |
| `internal/service/`    | Management service business logic              |
| `internal/router/`     | HTTP route registration and proxy client logic |
| `internal/storage/`    | Storage adapters used by gateway/proxy logic   |
| `internal/config/`     | Environment-based configuration                |

## Generated Code Rules

- Do not put handwritten source files in `gen/` or `internal/repository/`.
- Do not manually edit files in `gen/` or `internal/repository/`.
- For DB changes, edit `db/migrations/` and `db/queries/`, then run `sqlc generate`.
- For API changes, edit `proto/`, then run `buf generate`.
- If both DB and API generation are needed, run `sqlc generate` first, then `buf generate`.
- TypeScript protobuf and service descriptors are generated into `web/fabric/src/gen/`; do not edit files in that directory manually.
