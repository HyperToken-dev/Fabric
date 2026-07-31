# Fabric Usage Guide

This document explains how to use Fabric in detail, including running the integrated gateway, configuring management APIs, calling provider-routed proxy APIs, and reusing Core / Business layers as libraries.

## 1. Run the Integrated Gateway

### 1.1 Requirements

- Docker and Docker Compose for the recommended startup path.
- Go 1.26.4 and PostgreSQL only when running locally without Docker.
- Access to an OpenAI-compatible, Alibaba Bailian, or Seedance upstream service, depending on the provider channel you configure.

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
- `integral_logs`
- `provider_tasks`

The migrations do not seed default provider channels or models. In real deployments, configure or create a channel with a real `provider_key` and add required models through the management API or Admin Console.

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
- `db.addr`: PostgreSQL host.
- `db.user`: PostgreSQL user.
- `db.port`: PostgreSQL port.
- `db.dbName`: database name.
- `db.password`: PostgreSQL password.
- `db.maxIdle`: maximum idle connections.
- `db.maxOpen`: maximum open connections.
- `db.maxLifeTime`: maximum connection lifetime.
- `log.maxSize`, `log.maxBackups`, `log.maxAge`, `log.compress`: log rotation settings.

Fire Wall dictionaries are managed from the Web Console, not from `config.yaml`. For local runs, the runtime store is created under `configs/sensitive/` on first startup.

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
- `integral_logs`
- `provider_tasks`

The migrations do not seed default provider channels or models. In real deployments, configure or create a channel with a real `provider_key` and add required models through the management API or Admin Console.

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
- `UpdateChannelName(UpdateChannelNameRequest) returns (UpdateChannelResponse)`
- `UpdateChannelStatus(UpdateChannelStatusRequest) returns (UpdateChannelResponse)`
- `UpdateChannelBaseURL(UpdateChannelBaseURLRequest) returns (UpdateChannelResponse)`
- `UpdateChannelAPIFormat(UpdateChannelAPIFormatRequest) returns (UpdateChannelResponse)`
- `UpdateChannelProviderKey(UpdateChannelProviderKeyRequest) returns (UpdateChannelResponse)`

`CreateChannelRequest` fields:

- `channel_name`: channel name.
- `base_url`: upstream service URL, for example `https://api.openai.com` (OpenAI), `https://dashscope.aliyuncs.com` (Alibaba Bailian), or your Seedance upstream base URL.
- `api_format`: API format. `1` for OpenAI, `2` for Alibaba Bailian, `3` for Seedance.
- `provider_key`: upstream provider API key.

Update request fields:

- `UpdateChannelNameRequest`: `channel_id`, `channel_name`.
- `UpdateChannelStatusRequest`: `channel_id`, `status`. Current status values are `1` active, `2` banned, and `3` pending.
- `UpdateChannelBaseURLRequest`: `channel_id`, `base_url`.
- `UpdateChannelAPIFormatRequest`: `channel_id`, `api_format`.
- `UpdateChannelProviderKeyRequest`: `channel_id`, `provider_key`.

`UpdateChannelProviderKey` updates the stored upstream provider credential. `Channel` responses do not expose `provider_key`.

Example for OpenAI:

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

Example for Alibaba Bailian:

```bash
curl http://localhost:9090/proto.ChannelService/CreateChannel \
  -H "Content-Type: application/json" \
  -d '{
    "channelName": "Bailian",
    "baseUrl": "https://dashscope.aliyuncs.com",
    "apiFormat": 2,
    "providerKey": "sk-your-dashscope-key"
  }'
```

Example for Seedance:

```bash
curl http://localhost:9090/proto.ChannelService/CreateChannel \
  -H "Content-Type: application/json" \
  -d '{
    "channelName": "Seedance",
    "baseUrl": "https://your-seedance-upstream.example.com",
    "apiFormat": 3,
    "providerKey": "sk-your-seedance-key"
  }'
```

Use the upstream base URL assigned by your Seedance provider account. Fabric routes Seedance traffic through the configured channel `base_url`; it does not require a hardcoded Seedance base URL in `config.yaml`.

Update channel name:

```bash
curl http://localhost:9090/proto.ChannelService/UpdateChannelName \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": 1,
    "channelName": "openai-prod"
  }'
```

Update channel status:

```bash
curl http://localhost:9090/proto.ChannelService/UpdateChannelStatus \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": 1,
    "status": 1
  }'
```

Update channel base URL:

```bash
curl http://localhost:9090/proto.ChannelService/UpdateChannelBaseURL \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": 1,
    "baseUrl": "https://api.openai.com"
  }'
```

Update channel API format:

```bash
curl http://localhost:9090/proto.ChannelService/UpdateChannelAPIFormat \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": 1,
    "apiFormat": 1
  }'
```

Update channel provider key:

```bash
curl http://localhost:9090/proto.ChannelService/UpdateChannelProviderKey \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": 1,
    "providerKey": "sk-new-provider-key"
  }'
```

### 2.2 ModelService

A Model represents an available model under a channel.

Methods:

- `GetModelInfo(GetModelInfoRequest) returns (GetModelInfoResponse)`
- `CreateModel(CreateModelRequest) returns (CreateModelResponse)`
- `ListModels(ListModelsRequest) returns (ListModelsResponse)`
- `ListCatalogModels(ListCatalogModelsRequest) returns (ListCatalogModelsResponse)`

`CreateModelRequest` fields:

- `model_name`: model name, for example `gpt-5.5` or `wan2.7-t2v-2026-06-12`.
- `channel_id`: owning channel ID.
- `status`: model status. The current active status is `1`.
- `model_type`: model type. Text is `1`, Video is `2`.

Example for a text model:

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

Example for a Seedance video model:

```bash
curl http://localhost:9090/proto.ModelService/CreateModel \
  -H "Content-Type: application/json" \
  -d '{
    "modelName": "doubao-seedance-2-0-260128",
    "channelId": 3,
    "status": 1,
    "modelType": 2
  }'
```

The `channelId` must point to a Seedance channel. Seedance catalog models are video models, so use `modelType: 2`.

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

### 3.1 OpenAI Chat Completions

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

### 3.2 OpenAI Streaming Chat Completions

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

### 3.3 Alibaba Bailian Text-to-Video Task Creation

```bash
curl http://localhost:3002/api/v1/services/aigc/video-generation/video-synthesis \
  -H "Authorization: Bearer hy_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wan2.7-t2v-2026-06-12",
    "input": {
      "prompt": "A cat running under moonlight"
    },
    "parameters": {
      "resolution": "720P",
      "ratio": "16:9",
      "duration": 5
    }
  }'
```

Notes:
- The `Authorization` header expects a Gateway API Key bound to an Alibaba Bailian API format channel.
- Fabric automatically injects `X-DashScope-Async: enable` and the provider key when forwarding.
- The `model` must be configured under the bound channel.

### 3.4 Alibaba Bailian Task Fetch

```bash
curl http://localhost:3002/api/v1/tasks/<task_id> \
  -H "Authorization: Bearer hy_xxx"
```

Notes:
- Replace `<task_id>` with the ID returned by the task creation request.
- The same Gateway API Key must be used.

### 3.5 Seedance Video Task Creation

```bash
curl http://localhost:3002/api/v3/contents/generations/tasks \
  -H "Authorization: Bearer hy_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "A cat running under moonlight"
      }
    ]
  }'
```

Notes:
- The `Authorization` header expects a Gateway API Key bound to a Seedance API format channel.
- The `model` must be configured under the bound Seedance channel and must be enabled.
- Fabric checks `content[]` entries where `type` is `text` before forwarding the request upstream.
- If the upstream creation response contains a task ID, Fabric stores a provider task record for later lifecycle and usage handling.

### 3.6 Seedance Task Query

```bash
curl http://localhost:3002/api/v3/contents/generations/tasks/<task_id> \
  -H "Authorization: Bearer hy_xxx"
```

Notes:
- Replace `<task_id>` with the ID returned by the Seedance task creation request.
- The same Gateway API Key must be used.
- Fabric updates usage only for Seedance tasks that were created through Fabric and are already tracked in `provider_tasks`.

## 4. Usage Logging

Fabric currently records usage for OpenAI-compatible requests and for tracked successful Seedance task responses. Alibaba Bailian text-to-video requests are proxied successfully, but structured Alibaba Bailian video usage is not currently recorded.

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

For Seedance asynchronous video tasks:

- Fabric creates a `provider_tasks` row only when a Seedance generation request passes through Fabric and the upstream creation response includes a task ID.
- Later Seedance task query responses update only existing provider task records; querying a task ID that was not created through Fabric does not create a provider task or usage log.
- Usage is recorded only when a tracked task reaches a successful status and the response includes a positive `usage.completion_tokens` value.
- Seedance usage rows use `prompt_tokens = 0` and `completion_tokens = usage.completion_tokens`.
- `usage.total_tokens` alone is intentionally ignored.
- Repeated or concurrent polling for the same completed Seedance task records usage at most once.

## 5. Fire Wall

Fire Wall is managed from the Web Console. Use the `Fire Wall` page to enable detection, create dictionaries, set model scopes, and manage words. Docker Compose persists Fire Wall runtime data in the `fabric-sensitive` named volume. Local runs create the runtime store under `configs/sensitive/`.

### 5.1 Detection Behavior

- Fabric checks input prompts before forwarding requests upstream.
- Fabric checks non-streaming model outputs before returning responses.
- For Seedance generation requests, Fabric checks `content[]` entries where `type` is `text` and `text` is non-empty before forwarding upstream.
- Seedance image, video, and audio URL fields are not checked by this text-entry behavior.
- If an input prompt matches, Fabric returns `403` with `prompt rejected`.
- If a non-streaming model output matches, Fabric returns `422` with `model output rejected, please change your prompt`.
- Streaming output sensitive-word detection is not currently applied to streamed response chunks.
- If a runtime reload fails because the updated Fire Wall files are invalid, Fabric keeps using the previous rules and logs the reload error.

## 6. Library Usage

One of Fabric's goals is to help downstream projects avoid rebuilding AI gateway primitives. Besides running the integrated gateway directly, you can reuse capabilities by layer.

### 6.1 Use Only the Core Layer

Use this when you already have a business system and only want provider proxying.

Relevant directories:

- `core/proxy/`
- `core/providers/`

Reusable capabilities:

- OpenAI-compatible reverse proxy.
- Alibaba Bailian task API proxy.
- Seedance asynchronous task API proxy.
- Provider request rewrite hooks.
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

Business-layer packages can be imported directly from downstream Go services:

```go
import (
    openaiusage "github.com/HyperToken-dev/fabric/business/usage/openai"
    "github.com/HyperToken-dev/fabric/business/sensitive"
    sensitiveopenai "github.com/HyperToken-dev/fabric/business/sensitive/openai"
)
```

#### 6.2.1 Non-Streaming Usage Extraction

Use `business/usage/openai` when your service already has the upstream OpenAI-compatible response body and wants to extract token usage without running Fabric's integrated gateway:

```go
parsedUsage, err := openaiusage.ExtractNonStreaming(rawResponseBody, contentEncoding)
if err != nil {
    return err
}

// Persist, bill, or audit usage in your own system.
recordUsage(parsedUsage.PromptTokens, parsedUsage.CompletionTokens)
```

`rawResponseBody` is the complete upstream response body. `contentEncoding` should be the response `Content-Encoding` value, for example `""`, `"identity"`, `"gzip"`, or `"br"`.

#### 6.2.2 Streaming Usage Tracking

For streaming OpenAI-compatible responses, wrap the upstream response body before returning it to your caller:

```go
trackedBody := openaiusage.NewTrackingReader(upstreamBody, contentEncoding, func(parsedUsage *usage.Usage) {
    // This callback runs when stream usage is discovered.
    recordUsage(parsedUsage.PromptTokens, parsedUsage.CompletionTokens)
})

defer trackedBody.Close()
```

Your proxy still streams `trackedBody` to the client. The Business layer only parses usage and invokes the callback; your application decides where to store usage records and how to handle callback errors.

If you need the `usage.Usage` type in your callback signature, import it from:

```go
import "github.com/HyperToken-dev/fabric/business/usage"
```

#### 6.2.3 Sensitive-Word Detection

Use `business/sensitive` when you want Fabric's dictionary matching and model-scoped policy behavior inside your own request flow.

For static in-memory dictionaries, construct a detector directly:

```go
detector, err := sensitive.NewDetector(
    sensitive.Dictionary{
        Name:         "default-block-list",
        Words:        []string{"blocked phrase", "secret"},
        EffectModels: []string{"gpt-5.5"},
    },
)
if err != nil {
    return err
}

result := detector.Detect("gpt-5.5", "user prompt containing secret")
if result.Rejected() {
    rejectRequest(result.Matches)
}
```

`EffectModels` scopes a dictionary to specific model names. Leave it empty to apply that dictionary to every model passed to `Detect`. Dictionary names are included in match results so downstream systems can audit which rule matched.

For runtime updates, use the reloadable policy from `business/sensitive` and provide your own source callback:

```go
policy := sensitive.NewReloadablePolicy(sensitive.Snapshot{})

source := func(ctx context.Context) (sensitive.SourceState, error) {
	 detector, err := sensitive.NewDetector(sensitive.Dictionary{
	     Name:         "default-block-list",
	     Words:        []string{"blocked phrase", "secret"},
	     EffectModels: []string{"gpt-5.5"},
	 })
	 if err != nil {
	     return sensitive.SourceState{}, err
    }
    return sensitive.SourceState{Enabled: true, Detector: detector, DictionaryCount: 1}, nil
}

_, err := policy.Reload(ctx, source)
if err != nil {
    return err
}
```

`Reload` publishes a new snapshot only after the source callback succeeds. If a later reload fails, callers can keep the existing policy active and log the returned error.

#### 6.2.4 OpenAI Prompt and Output Extraction

Use `business/sensitive/openai` when your proxy receives OpenAI-compatible requests and responses but wants to apply its own rejection policy:

```go
promptReq, err := sensitiveopenai.ExtractPromptRequest(req)
if err != nil {
    return err
}

for _, prompt := range promptReq.Prompts {
    if detector.Detect(promptReq.Model, prompt).Rejected() {
        rejectRequest(nil)
    }
}
```

`ExtractPromptRequest` reads and restores `req.Body`, so the request can still be forwarded upstream after inspection.

For non-streaming upstream responses, extract output text before returning the response to your caller:

```go
texts, err := sensitiveopenai.ExtractOutputTexts(req, rawResponseBody)
if err != nil {
    return err
}

for _, text := range texts {
    if detector.Detect(promptReq.Model, text).Rejected() {
        rejectResponse(nil)
    }
}
```

Streaming output sensitive-word detection is not provided by these helpers today; downstream callers must implement chunk-level inspection themselves if they need it.

#### 6.2.5 Business-Layer Boundaries

When used as a library, the Business layer provides parsing, extraction, detection, and callback hooks only. Your application remains responsible for:

- HTTP routing and request lifecycle.
- Forwarding requests to upstream providers.
- Returning or streaming responses to clients.
- Persisting usage records.
- Billing, quota, audit, or reporting side effects.
- Choosing whether matched sensitive text rejects, warns, logs, or triggers another policy.
- Logging and error handling around your own storage or policy systems.

Fabric's integrated gateway wires these pieces to `internal/router/`, `internal/storage/`, and management APIs. Downstream projects that import only `business/...` packages do not need to use Fabric's PostgreSQL schema, Admin Server, or integrated gateway process.

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
3. Let applications call the Fabric Proxy Server with OpenAI-compatible, Alibaba Bailian, or Seedance requests through the Gateway API Key bound to the appropriate channel.

## 7. Troubleshooting

### Proxy returns `missing provider key`

The bound Channel does not have an upstream provider key. Provide `provider_key` when creating or updating the Channel through the Admin API.

### Proxy returns `unsupported model`

The requested model is not configured under the current Channel, or the model is disabled. Create or inspect model configuration through `ModelService`. Official catalog models are only candidates; you must explicitly add a model to your channel before clients can request it.

### Proxy returns `unsupported alibaba bailian path`

The requested path is not `video-synthesis` or `tasks/`. Fabric restricts the Alibaba Bailian proxy surface to explicitly supported task paths.

### Proxy returns `unsupported seedance path`

The requested path is not under `/api/v3/contents/generations/tasks`, or the method is not supported for that Seedance task route. Fabric supports Seedance task creation with `POST /api/v3/contents/generations/tasks` and task query/delete paths under that prefix.

### Bailian Video Usage is not shown

Alibaba Bailian text-to-video requests are proxied successfully, but Fabric does not currently record video usage. This is a known limitation.

### Seedance Usage is not shown

Seedance usage is recorded only from tracked Fabric-created tasks. If no usage appears, check whether:

- The original Seedance task creation request passed through Fabric and returned a provider task ID.
- The task query response reached a successful status.
- The successful response included a positive `usage.completion_tokens` value.
- The response only included `usage.total_tokens`, which Fabric intentionally ignores for Seedance usage accounting.
- The task ID was created outside Fabric; unknown task queries produce audit logs but do not create provider task or usage records.

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
