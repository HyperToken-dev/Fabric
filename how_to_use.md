# Fabric Usage Guide

This document explains how to use Fabric in detail, including running the integrated gateway, configuring management APIs, calling the OpenAI-compatible proxy, and reusing Core / Business layers as libraries.

## 1. Run the Integrated Gateway

### 1.1 Requirements

- Docker and Docker Compose for the recommended startup path.
- Go 1.26.4 and PostgreSQL only when running locally without Docker.
- Access to an OpenAI-compatible upstream service

### 1.2 Start with Docker Compose

The recommended way to run Fabric from a fresh clone is Docker Compose:

```bash
git clone https://github.com/HyperToken-dev/Fabric.git
cd Fabric
docker compose up -d
```

Docker Compose builds the Fabric gateway image from `Dockerfile`, starts a PostgreSQL service, waits for PostgreSQL health checks, and then starts the gateway.

Default servers:

- Proxy Server: `http://localhost:3002`
- Admin Server: `http://localhost:9090`

Fabric runs migrations from `db/migrations/` during startup. The current migrations create and maintain these core tables:

- `channels`
- `api_keys`
- `models`
- `usage_logs`

The migrations also seed an OpenAI channel and a set of OpenAI-compatible models. In real deployments, configure or create a channel with a real `provider_key` through the management API.

### 1.3 Configuration Files

Fabric's binary reads a file named `config.yaml`. Which file you edit depends on how you start Fabric:

- **Docker Compose**: edit `configs/config.docker.yaml`. The tracked `compose.yaml` mounts it as `/app/configs/config.yaml` inside the Fabric container.
- **Local Go run**: edit `configs/config.yaml`. This is the default file read by `go run ./cmd/gateway` from the repository.

Docker users do not need to create a separate `configs/config.yaml` before starting Fabric.

The Docker configuration currently looks like this:

```yaml
proxyAddr: 3002
adminAddr: 9090
logLevel: info

sensitiveWordDetect: false
sensitiveWordDictionaries:
  - name: example_name
    effectModels: [gpt-5.5, gpt-5.4]
    keywordFileList: [st-01, st-02]

db:
  addr: postgres
  user: root
  port: 5432
  dbName: fabric
  password: "123456"
  maxIdle: 20
  maxOpen: 100
  maxLifeTime: 1h

log:
  maxSize: 100
  maxBackups: 10
  maxAge: 30
  compress: true
```

Configuration reference:

- `proxyAddr`: proxy server listen port. Clients call OpenAI-compatible APIs through this port. Docker exposes it as `3002`.
- `adminAddr`: admin server listen port. connect-go management APIs are exposed through this port. Docker exposes it as `9090`.
- `logLevel`: logging level, for example `info`.
- `sensitiveWordDetect`: enables or disables sensitive-word detection.
- `sensitiveWordDictionaries`: list of sensitive-word detection rules used when detection is enabled.
- `sensitiveWordDictionaries[].name`: rule name used in logs and match results.
- `sensitiveWordDictionaries[].effectModels`: models this rule applies to. Leave it empty to apply the rule to all models.
- `sensitiveWordDictionaries[].keywordFileList`: dictionary file base names under `configs/stwd/`. Do not include the `.txt` suffix.
- `db.addr`: PostgreSQL host. In Docker Compose this is the service name `postgres`.
- `db.user`: PostgreSQL user. In Docker Compose this must match `POSTGRES_USER` in `compose.yaml`.
- `db.port`: PostgreSQL port.
- `db.dbName`: PostgreSQL database name. In Docker Compose this must match `POSTGRES_DB` in `compose.yaml`.
- `db.password`: PostgreSQL password. In Docker Compose this must match `POSTGRES_PASSWORD` in `compose.yaml`.
- `db.maxIdle`: maximum idle database connections.
- `db.maxOpen`: maximum open database connections.
- `db.maxLifeTime`: maximum database connection lifetime.
- `log.maxSize`: maximum size of one rotated log file.
- `log.maxBackups`: maximum number of retained rotated log files.
- `log.maxAge`: maximum age of retained log files.
- `log.compress`: whether to compress rotated log files.

### 1.4 Local Go Run

Fabric reads `configs/config.yaml` by default. A typical configuration looks like this:

```yaml
proxyAddr: 3002
adminAddr: 9090
logLevel: info

sensitiveWordDetect: false

db:
  addr: 127.0.0.1
  user: postgres
  port: 5432
  dbName: dbexample
  password: "your-password"
  maxIdle: 20
  maxOpen: 100
  maxLifeTime: 1h

log:
  maxSize: 100
  maxBackups: 10
  maxAge: 30
  compress: true
```

Field reference:

- `proxyAddr`: proxy server listen port. Clients call OpenAI-compatible APIs through this port.
- `adminAddr`: admin server listen port. connect-go management APIs are exposed through this port.
- `logLevel`: logging level.
- `sensitiveWordDetect`: enables or disables sensitive-word detection.
- `sensitiveWordDictionaries`: dictionary configuration used when sensitive-word detection is enabled.
- `db.addr`: PostgreSQL host.
- `db.user`: PostgreSQL user.
- `db.port`: PostgreSQL port.
- `db.dbName`: database name.
- `db.password`: PostgreSQL password.
- `db.maxIdle`: maximum idle connections.
- `db.maxOpen`: maximum open connections.
- `db.maxLifeTime`: maximum connection lifetime.
- `log.maxSize`, `log.maxBackups`, `log.maxAge`, `log.compress`: log rotation settings.

If you have not prepared sensitive-word dictionaries, disable detection first:

```yaml
sensitiveWordDetect: false
```

If you want to enable sensitive-word detection for local runs, configure rule-based dictionaries and prepare the corresponding files:

```yaml
sensitiveWordDetect: true
sensitiveWordDictionaries:
  - name: example_name
    effectModels: [gpt-5.5]
    keywordFileList: [st-01, st-02]
```

Dictionary file paths:

```text
configs/stwd/st-01.txt
configs/stwd/st-02.txt
```

Each entry is a detection rule. `name` is required and is only used as a human-readable rule name in logs and match results. `effectModels` controls which models the rule applies to; leave it empty to apply the rule to all models. `keywordFileList` is required and lists keyword files under `configs/stwd/`, so `st-01` loads `configs/stwd/st-01.txt`. Each keyword file contains one keyword per line.

#### Local Database

Create the database:

```bash
createdb dbexample
```

Fabric runs migrations from `db/migrations/` during startup. The current migrations create and maintain these core tables:

- `channels`
- `api_keys`
- `models`
- `usage_logs`

The migrations also seed an OpenAI channel and a set of OpenAI-compatible models. In real deployments, configure or create a channel with a real `provider_key` through the management API.

#### Local Start

```bash
make generate
make run
```

Default servers:

- Proxy Server: `http://localhost:3002`
- Admin Server: `http://localhost:9090`

### 1.5 Basic Integration Flow

When running the integrated gateway directly, the recommended flow is:

1. Clone the repository.
2. Start Fabric with `docker compose up -d`.
3. Confirm Proxy Server `http://localhost:3002` and Admin Server `http://localhost:9090` are reachable.
4. Create or configure a Channel through the Admin API.
5. Create a Model through the Admin API.
6. Create a Gateway API Key through the Admin API.
7. Let clients call the Proxy Server with the Gateway API Key.

## 2. Management APIs

Management APIs are exposed with connect-go. Proto definitions are located at:

- `proto/channel.proto`
- `proto/model.proto`
- `proto/apiKey.proto`
- `proto/usage.proto`

The Admin Server listens on `:9090` by default. connect-go handler paths are registered from generated code and usually follow this shape:

```text
/proto.ChannelService/<Method>
/proto.ModelService/<Method>
/proto.ManageApiKeyService/<Method>
/proto.UsageService/<Method>
```

### 2.1 ChannelService

A Channel represents an upstream model service channel, including the upstream base URL, provider key, and API format.

Methods:

- `CreateChannel(CreateChannelRequest) returns (CreateChannelResponse)`
- `ListChannels(ListChannelsRequest) returns (ListChannelsResponse)`
- `ListActiveChannels(ListActiveChannelsRequest) returns (ListChannelsResponse)`

`CreateChannelRequest` fields:

- `channel_name`: channel name.
- `base_url`: upstream service URL, for example `https://api.openai.com`.
- `api_format`: API format. The current OpenAI-compatible format is `1`.
- `provider_key`: upstream provider API key.

Example:

```bash
curl http://localhost:9090/proto.ChannelService/CreateChannel \
  -H "Content-Type: application/json" \
  -d '{
    "channelName": "OpenAI",
    "baseUrl": "https://api.openai.com",
    "apiFormat": 1,
    "providerKey": "sk-your-provider-key"
  }'
```

### 2.2 ModelService

A Model represents an available model under a channel.

Methods:

- `GetModelInfo(GetModelInfoRequest) returns (GetModelInfoResponse)`
- `CreateModel(CreateModelRequest) returns (CreateModelResponse)`
- `ListModels(ListModelsRequest) returns (ListModelsResponse)`

`CreateModelRequest` fields:

- `model_name`: model name, for example `gpt-5.5`.
- `channel_id`: owning channel ID.
- `status`: model status. The current active status is `1`.
- `model_type`: model type. The current text model type is `1`.

Example:

```bash
curl http://localhost:9090/proto.ModelService/CreateModel \
  -H "Content-Type: application/json" \
  -d '{
    "modelName": "gpt-5.5",
    "channelId": 1,
    "status": 1,
    "modelType": 1
  }'
```

### 2.3 ManageApiKeyService

A Gateway API Key is the key downstream applications use when calling the Fabric Proxy Server. It is not the upstream provider key.

Methods:

- `CreateApiKey(CreateApiKeyRequest) returns (CreateApiKeyResponse)`
- `RevokeApiKey(RevokeApiKeyRequest) returns (RevokeApiKeyResponse)`
- `ListApiKeysByChannelID(ListApiKeysByChannelIDRequest) returns (ListApiKeysResponse)`
- `ListApiKeysByChannelName(ListApiKeysByChannelNameRequest) returns (ListApiKeysResponse)`

`CreateApiKeyRequest` fields:

- `key_name`: key name.
- `channel_id`: bound channel ID.

Example:

```bash
curl http://localhost:9090/proto.ManageApiKeyService/CreateApiKey \
  -H "Content-Type: application/json" \
  -d '{
    "keyName": "local-dev",
    "channelId": 1
  }'
```

The response `rawKey` is returned only when the key is created. Save it securely. Use this `rawKey` when calling the Proxy Server.

### 2.4 UsageService

UsageService queries and records usage data.

Methods:

- `GetUsageByKeyID(GetUsageByKeyIDRequest) returns (GetUsageResponse)`
- `GetUsageByKeyHash(GetUsageByKeyHashRequest) returns (GetUsageResponse)`
- `GetUsageByChannelID(GetUsageByChannelIDRequest) returns (GetUsageResponse)`
- `GetUsageByModelID(GetUsageByModelIDRequest) returns (GetUsageResponse)`
- `GetUsageByDeadlineAndKeyHash(GetUsageByDeadlineAndKeyHashRequest) returns (GetUsageResponse)`
- `GetUsageSummary(GetUsageSummaryRequest) returns (GetUsageResponse)`
- `LogUsage(LogUsageRequest) returns (LogUsageResponse)`

Query global usage summary:

```bash
curl http://localhost:9090/proto.UsageService/GetUsageSummary \
  -H "Content-Type: application/json" \
  -d '{}'
```

## 3. Proxy Examples

Clients call the Fabric Proxy Server with a Fabric-issued Gateway API Key. Fabric resolves the bound Channel and injects the provider key when forwarding the request upstream.

### 3.1 Chat Completions

```bash
curl http://localhost:3002/v1/chat/completions \
  -H "Authorization: Bearer hy_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }'
```

Notes:

- `hy_xxx` is a Gateway API Key created by Fabric.
- `hy_xxx` is not an OpenAI provider key.
- The provider key is stored in the Channel and injected by Fabric during forwarding.
- The requested `model` must exist under the bound Channel and must be enabled.

### 3.2 Streaming Chat Completions

```bash
curl http://localhost:3002/v1/chat/completions \
  -H "Authorization: Bearer hy_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.5",
    "stream": true,
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }'
```

For streaming chat completions, Fabric injects:

```json
{
  "stream_options": {
    "include_usage": true
  }
}
```

This allows Fabric to capture usage from the streaming response.

## 4. Usage Logging

Fabric currently supports OpenAI-compatible usage logging.

For non-streaming responses:

- Fabric reads the upstream response body.
- It extracts the `usage` field from the response JSON.
- It writes the usage log asynchronously.

For streaming responses:

- Fabric wraps the response stream with a tracking reader.
- It captures usage information from SSE events.
- It writes the usage log to `usage_logs`.

Usage logs are associated with:

- Gateway API Key
- Channel
- Model
- Prompt tokens
- Completion tokens
- Created time

## 5. Model-Scoped Sensitive-Word Detection

Sensitive-word detection is controlled by `sensitiveWordDetect`. It is disabled by default in `configs/config.docker.yaml`:

```yaml
sensitiveWordDetect: false
```

### 5.1 Create Dictionary Files

Dictionary files live under `configs/stwd/`. Create one `.txt` file per dictionary and put one keyword on each line:

```bash
cd configs/stwd
```

For example, create `blocked-words.txt`:

```text
badword1
badword2
badword3
```

Empty lines are ignored, and duplicate keywords are removed when the dictionary is loaded.

### 5.2 Configure Docker Detection Rules

Edit `configs/config.docker.yaml` and enable detection:

```yaml
sensitiveWordDetect: true
sensitiveWordDictionaries:
  - name: default-block-list
    effectModels: [gpt-5.5]
    keywordFileList: [blocked-words]
```

`keywordFileList` uses file base names without the `.txt` suffix. The example above loads:

```text
configs/stwd/blocked-words.txt
```

Each entry under `sensitiveWordDictionaries` is a detection rule:

- `name` is required and is used as a human-readable rule name in logs and match results.
- `effectModels` controls which models the rule applies to. Use `effectModels: []` to apply the rule to all models.
- `keywordFileList` is required and lists dictionary file base names under `configs/stwd/`.

After changing `configs/config.docker.yaml` or dictionary files, restart the service:

```bash
docker compose up -d
```

### 5.3 Detection Behavior

- Fabric checks input prompts before forwarding requests upstream.
- Fabric checks non-streaming model outputs before returning responses.
- If an input prompt matches, Fabric returns `403` with `prompt rejected`.
- If a non-streaming model output matches, Fabric returns `422` with `model output rejected, please change your prompt`.
- Streaming output sensitive-word detection is not currently applied to streamed response chunks.
- If `sensitiveWordDetect: true` and a configured dictionary file is missing or contains no usable words, startup fails.

## 6. Library Usage

One of Fabric's goals is to help downstream projects avoid rebuilding AI gateway primitives. Besides running the integrated gateway directly, you can reuse capabilities by layer.

### 6.1 Use Only the Core Layer

Use this when you already have a business system and only want provider proxying.

Relevant directories:

- `core/proxy/`
- `core/providers/`

Reusable capabilities:

- OpenAI-compatible reverse proxy.
- Provider request rewrite.
- Provider response hooks.
- Upstream base URL and provider key injection.

### 6.2 Use Only the Business Layer

Use this when you already have a proxy or gateway and only want business governance modules.

Relevant directories:

- `business/usage/`
- `business/sensitive/`

Reusable capabilities:

- OpenAI usage extraction.
- Streaming usage tracking.
- Sensitive-word detector.
- OpenAI prompt/output text extraction.
- Model-scoped dictionary policy.

### 6.3 Use the Integrated Gateway

Use this when you do not want to compose modules yourself.

Relevant directories:

- `cmd/gateway/`
- `internal/router/`
- `internal/server/`
- `internal/service/`
- `internal/storage/`

Usage flow:

1. Start with `docker compose up -d` or run locally with `go run ./cmd/gateway` after preparing PostgreSQL.
2. Configure Channel, Model, and Gateway API Key through the Admin API.
3. Let applications call the Fabric Proxy Server with OpenAI-compatible requests.

## 7. Troubleshooting

### Startup fails because a sensitive-word dictionary file is missing

If `sensitiveWordDetect: true`, Fabric loads each file listed by `keywordFileList` from `configs/stwd/` using the `.txt` suffix convention. For example, `keywordFileList: [st-01]` loads `configs/stwd/st-01.txt`. Startup fails if a configured keyword file does not exist.

Fix it by either:

- Creating the corresponding dictionary file.
- Setting `sensitiveWordDetect: false` first.

### Proxy returns `missing provider key`

The bound Channel does not have an upstream provider key. Provide `provider_key` when creating or updating the Channel through the Admin API.

### Proxy returns `unsupported model`

The requested model is not configured under the current Channel, or the model is disabled. Create or inspect model configuration through `ModelService`.

### Proxy returns `invalid api key`

The Gateway API Key does not exist or has been revoked. Create a new API Key through `ManageApiKeyService`.

### Database connection fails

For Docker Compose, check that PostgreSQL is healthy and that `configs/config.docker.yaml` matches the PostgreSQL environment in `compose.yaml`:

- `db.addr` should be `postgres`.
- `db.user` should match `POSTGRES_USER`.
- `db.password` should match `POSTGRES_PASSWORD`.
- `db.dbName` should match `POSTGRES_DB`.

```bash
docker compose ps
```

For local Go runs, check the `db` section in `configs/config.yaml`. Confirm PostgreSQL is running, the database exists, and the username/password are correct.

## 8. Development and Generation Commands

```bash
go build ./...
go vet ./...
go test ./...
go run ./cmd/gateway

docker compose up -d
docker compose down

make generate
make build
make run
make lint
make test
make sqlc-check
```

If you change database schema or queries, run:

```bash
sqlc generate
```

If you change proto files, run:

```bash
buf generate
```

If both change, run `sqlc generate` first, then `buf generate`.
