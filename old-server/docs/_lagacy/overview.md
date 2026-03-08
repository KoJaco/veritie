# 🧠 Schma – System Architecture Overview

## 📌 Purpose

Schma is a real-time voice-to-function platform that parses structured data from audio in domain-agnostic applications. It is designed to be embedded in other apps via an SDK, with streaming STT, dynamic function extraction, and structured output.

This document provides a high-level overview of the system's architecture, design principles, and runtime flow.

---

## 🏗️ Layered Architecture

Schma is structured using a clean, layered hexagonal architecture with clear separation between core logic, infrastructure, and delivery mechanisms.

internal/
├── app/ # Use cases & orchestration (services)
├── domain/ # Pure business logic (entities, ports)
├── infra/ # External adapters (DB, STT, LLM, etc.)
├── transport/ # HTTP/WebSocket layer
├── config/ # Runtime configuration (env, CLI, etc.)
├── pkg/ # External SDK-style packages
└── model/ # Shared data structs (optional, evolving)

---

## 🔁 Data Flow – Real-Time Session

A typical real-time session follows this flow:

```txt
[Client SDK]
    │
    ▼
[Transport Layer]
    WebSocket upgrade
    Auth token → domain.Principal
    Session init
    │
    ▼
[App Layer]
    Compose services (Session, Auth, STT, Parser, LLM)
    → Pull in config for app
    → Hold session state
    → Route audio to STT client
    → Trigger parser → function → optional LLM formatting
    │
    ▼
[Domain Layer]
    Core contracts and logic:
    - Session lifecycle state machine
    - Metering
    - Auth principal
    - Transcript & function data models
    │
    ▼
[Infra Layer]
    Adapters:
    - STT: Deepgram / Google
    - FastParser (ML local inference)
    - LLM: Gemini API
    - DB: Postgres (sqlc, repos)
    - Auth: JWT or API keys
    │
    ▼
[DB + External APIs]
```

---

## 🧩 Layer Responsibilities

### `internal/app/`

-   Entry point to all business logic.
-   Composes ports into services.
-   Manages session lifecycle.
-   Handles use cases like:

    -   `StartSession()`
    -   `StreamAudioChunk()`
    -   `ApplyFunctionSchema()`

> Depends on: `domain`, injected `infra` implementations, concurrency utils.

---

### `internal/domain/`

-   Pure, import-free business logic.
-   Durable business concepts:

    -   `Session`, `Transcript`, `FunctionCall`, `Meter`

-   Ports define what the domain _needs_, not _how_ it gets it.
-   No logging, DB, or external APIs.

> Imports only other sub-packages in `domain`.

---

### `internal/infra/`

-   External adapter implementations.
-   Maps `domain.XPort` → concrete logic.
-   Includes:

    -   `infra/db` (Postgres/sqlc)
    -   `infra/sttdeepgram`, `sttgoogle`
    -   `infra/llmgemini`
    -   `infra/authjwt`
    -   `infra/fastparser`

> Never imports from `app` or `transport`.

---

### `internal/transport/`

-   Thin shell: WebSocket handler, HTTP endpoints.
-   Converts HTTP/WebSocket details → app calls.
-   Includes:

    -   WS upgrade
    -   Health + metrics endpoints
    -   Context injection (auth Principal)

---

### `internal/config/`

-   Bootstraps app configuration from env/flags.
-   Central place to manage:

    -   STT provider keys
    -   Database DSN
    -   JWT secrets
    -   Feature flags

---

### `internal/pkg/`

-   Public SDK-style packages.
-   May expose:

    -   `pkg/sdk` for WebSocket interaction
    -   Type-safe client interfaces

---

### `internal/model/` _(optional)_

-   Simple, behaviorless struct definitions.
-   Used when:

    -   Shared between layers
    -   Needed in multiple domain packages
    -   Acts as decoupling between DB and business logic

---

## 🗃️ Database Structure

-   Managed via [Atlas](https://atlasgo.io/)
-   SQLC for type-safe Go code
-   `infra/db/` includes:

    -   `migrations/`
    -   `repos/` (implementation of domain ports)
    -   `queries/` (raw SQL)
    -   `generated/` (sqlc output)
    -   `postgres.go` (DB connection setup)

---

## 🔐 Authentication Strategy

-   Each **App** is registered under an **Account**.
-   App connections are authenticated via:

    -   Static API keys or
    -   JWT-signed API tokens (future)

-   Each connection is authenticated _per session_.
-   After validation:

    -   App config is cached
    -   Usage is metered for the associated account

---

## 📊 Metering and Usage

-   Session-wide metering:

    -   Audio seconds
    -   LLM tokens
    -   CPU seconds (Fly.io runtime)

-   `domain/usage.Meter` tracks totals
-   Used for:

    -   Usage-based billing
    -   Internal analytics
    -   Feature gating (free tier)

---

## 🪵 Logging and Event Logs

-   Domain emits events → stored in event log
-   Useful for:

    -   Session tracing
    -   Auditing
    -   LLM debugging

---

## 🔁 Dynamic Schemas

Two core features:

-   **Function Schema**: defines callable functions
-   **Structured Schema**: defines response layout

Both can:

-   Be updated at runtime
-   Be scoped to a session
-   Guide parsing and formatting engines dynamically

---

## ✅ Design Goals

-   🔌 Pluggable backends (STT, LLM, Auth)
-   🔐 Secure per-session isolation
-   💡 Real-time speech-to-structured-data
-   💾 Cache-first architecture (app config, prompt)
-   ⚙️ Fine-grained cost control + observability

---

## 🧪 Testing Strategy

-   `domain/`: fast table-driven tests
-   `app/`: uses mocks generated for domain ports
-   `infra/`: integration-style tests with fakes or network hits
-   `transport/`: route-level and handler tests with stubbed app layer

---

## 🛠️ Future

-   Multiple transport options (gRPC, CLI)
-   Self-hosted licensing mode
-   Event log viewer + replay
-   Multi-LLM router support
-   App config dashboard (separate service)

---
