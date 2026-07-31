<div align="center">

![Fabric Logo](web/fabric/public/logo.png)

<h1 align="center" style="margin: 0;">Fabric</h1>

<p align="center" style="margin: 0;">
  A modular AI gateway framework for proxying, key management, model routing, and usage governance.
</p>

Fabric is a modular AI gateway framework. You can run as full AI gateway service or selectively import the Core and Business layers as libraries and compose them into your own services.

Fabric is not intended to be only a proxy service. Its goal is to provide reusable AI gateway primitives: transparent proxying, API key management, downstream-to-upstream key mapping, channel/model mapping, usage tracking, sensitive-word detection, and future governance modules such as quota and limit controls.

</div>

<p align="center">
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="LICENSE"></img>
    <img src="https://img.shields.io/badge/Go-1.26-aqua?logo=go&logoColor=white" alt="go 1.26"></img>
    <img src="https://img.shields.io/badge/React-19.2.7-aqua?logo=react&logoColor=blue" alit="React 19.2.7"></img>
</p>

<p align="center">
    English | <a href="./README.zh-CN.md">简体中文</a>
</p>

## 💡 Why This Project Exists

Many AI gateway or AI application projects, such as new_api, axonhub, and similar systems, repeatedly need to maintain the same infrastructure:

- Their own API key system.
- Their own mapping between downstream keys and upstream provider keys.
- Their own channel, model, and provider mappings.
- Their own usage logging, aggregation, and query logic.
- Their own sensitive-word, quota, limit, audit, and policy modules.
- Their own glue between proxy, business logic, and a complete gateway product.

Fabric exists to split these common capabilities into independent, composable, reusable layers. You can run the top-level integrated gateway directly, or you can extract only the Core or Business capabilities you need and embed them into your own service instead of rebuilding the same gateway foundation again.

## 🧭 Project Positioning

| Feature                                  | Description                                                                                                                                          |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 🧩 Layered modular gateway               | A pluggable, layered modular gateway.                                                                                                                |
| 🌐 Multi-provider and multimodal         | A multi-provider, multimodal AI gateway foundation.                                                                                                  |
| 🖥️ Modern Web Console                    | A modern gateway product with an easy-to-operate Web Console.                                                                                        |
| 📊 Dynamic data dashboard                | A dynamic data dashboard.                                                                                                                            |
| ⚡ High concurrency and availability     | A high-concurrency, high-availability gateway architecture.                                                                                          |
| 🔁 Transparent proxying                  | Callers use Fabric Gateway API Keys while Fabric resolves channels, models, upstream base URLs, and provider credentials before forwarding requests. |
| 📈 Usage logging                         | Records key, channel, model, prompt-token, and completion-token usage                                                                                |
| 🛡️ Fire Wall                             | Detects sensitive text before forwarding supported inputs, supports model-scoped dictionaries and hot updating                                       |
| 🎞️ Multi-provider and multimodal support | OpenAI、Google、Anthropic、Seedance and more                                                                                                         |

## 🏗️ Architecture

Fabric is designed as reusable layers, not as a single fixed deployment shape. You can run the complete gateway, combine Business with Core, or reuse only the Business or Core layer inside your own system.

```mermaid
flowchart TB
    subgraph Deployments["Deployable Forms"]
        GBC["Gateway + Business + Core<br/>Integrated AI gateway"]
        BC["Business + Core<br/>Governance modules with provider proxying"]
        B["Business Only<br/>Usage, sensitive-word detection, quota, limit, audit"]
        C["Core Only<br/>Provider proxying, protocol adaptation, I/O primitives"]
    end

    subgraph Layers["Reusable Layers"]
        Gateway["Gateway Layer<br/>Admin APIs, routing, assembled product"]
        Business["Business Layer<br/>Usage / Sensitive Words / Quota / Limit / Audit"]
        Core["Core Layer<br/>Provider proxying / protocol adaptation / I/O"]
    end

    GBC --> Gateway
    GBC --> Business
    GBC --> Core

    BC --> Business
    BC --> Core

    B --> Business
    C --> Core

    Gateway -. depends on .-> Business
    Business -. depends on .-> Core
```

Supported deployment and reuse forms include:

- **Gateway + Business + Core**: run Fabric as a complete OpenAI-compatible gateway with admin APIs, proxy routes, usage logging, model/channel/key management, and policy modules.
- **Business + Core**: use governance capabilities together with provider proxying without adopting the full integrated gateway product.
- **Business only**: embed usage logging, sensitive-word detection, quota, limit, audit, or policy modules into an existing gateway.
- **Core only**: embed provider proxying, protocol adaptation, and low-level I/O primitives into your own service.

### Core Layer

Core is the protocol and transparent proxying layer. It handles provider request forwarding, response handling, protocol adaptation, and low-level proxy capabilities.

It is useful for:

- Projects that only need transparent OpenAI/provider proxying.
- Projects that want to embed proxying into their own service.
- Gateways that want to reuse provider adapters.

### Business Layer

Business contains cross-cutting governance capabilities such as usage tracking, sensitive-word detection, quota, limit, and audit modules.

It is useful for:

- Existing proxy services that only need usage logging.
- Existing business gateways that only need sensitive-word detection.
- Systems that need different policies by model, key, or channel.

### Integrated Gateway Layer

Integrated Gateway combines Core and Business capabilities into a complete gateway product.

It is useful for:

- Users who want to deploy a complete AI gateway directly.
- Users who do not want to manually compose Core and Business modules.
- Scenarios where writing a configuration file and running the gateway is enough.

## 🌐 Supported and Planned Providers / Models

Planned provider/model vendors include:

- OpenAI
- Google
- Anthropic
- xAI
- DeepSeek
- Meta
- Mistral
- Perplexity
- TII UAE
- Amazon
- Liquid AI
- Upstage
- MiniMax (Hailuo)
- Kimi (Moonshot AI)
- IBM
- Reka
- Tencent
- Baidu
- Z AI (Zhipu)
- ByteDance Seed
- Xiaomi
- Cohere
- Alibaba
- Kunlun Tech
- Kling AI
- iFLYTEK Spark
- Microsoft
- ...

**More than 300+ providers would be supported.**

## 🚀 Quick Start

### 1. Clone and Start with Docker Compose

```bash
git clone https://github.com/HyperToken-dev/Fabric.git
cd Fabric
docker compose up -d
```

Docker Compose builds the gateway image from `Dockerfile`, starts PostgreSQL, and exposes:

- Proxy Server: `http://localhost:3002`
- Admin Server and Web Console: `http://localhost:9090`

The tracked `configs/config.docker.yaml` is used by Docker Compose. It is mounted into the container as `/app/configs/config.yaml`, so Docker users do not need to create a separate `configs/config.yaml` before starting Fabric.

The gateway runs database migrations from `db/migrations/` during startup. After startup, create a channel for your desired provider (e.g. OpenAI, Alibaba Bailian, or Seedance), add a model, and configure a real upstream provider key through the Admin API or Web Console before proxying production traffic.

### 2. Optional Fire Wall

Fire Wall is managed from the Web Console. Use the `Fire Wall` page to enable detection, create dictionaries, set model scopes, and manage words. Docker Compose persists this runtime data in the `fabric-sensitive` named volume.

### 3. Local Development Run

For local development without Docker, prepare PostgreSQL, edit `configs/config.yaml`, then run:

```bash
make generate
make run
```

## 📖 Detailed Usage

See [how_to_use.md](./how_to_use.md) for detailed configuration, management APIs, proxy examples, and library integration guidance.

## 🛠️ Development Commands

```bash
go build ./...              # build all packages
go vet ./...                # vet all packages
go test ./...               # run tests
go run ./cmd/gateway        # start gateway; reads configs/config.yaml

docker compose up -d        # start Fabric and PostgreSQL with Docker Compose
docker compose down         # stop Docker services

make generate               # sqlc generate, then buf generate
make build                  # generate, then go build ./...
make run                    # go run ./cmd/gateway
make lint                   # go vet ./...
make test                   # go test ./...
make sqlc-check             # sqlc compile
```

### Frontend Development

```bash
cd web/fabric
pnpm install
pnpm dev                    # start the Vite development server
pnpm build                  # type-check and build the frontend
pnpm lint                   # run oxlint
pnpm format                 # format handwritten frontend source
```

## 🗺️ Roadmap

- Add more provider adapters.
- Support more model types and protocol forms.
- Add quota management.
- Add limit and rate-limiting capabilities.
- Add audit and policy workflows.
- Add a more complete management console.
- Continue strengthening independent deployment and library usage for Core, Business, and Integrated Gateway layers.
