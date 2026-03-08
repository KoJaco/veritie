# Internal Documentation Index

This directory contains comprehensive technical documentation for the Schma.ai server's internal architecture, components, and systems.

## 📋 Documentation Status

| Component             | Domain | App | Infra | Transport | Status      | Notes                             |
| --------------------- | ------ | --- | ----- | --------- | ----------- | --------------------------------- |
| **Authentication**    | ✅     | ✅  | ✅    | ✅        | 📖 Complete | API keys, caching, rate limiting  |
| **Audio Processing**  | ✅     | -   | ✅    | ✅        | 📖 Complete | Ring buffer, silence service      |
| **Batch Processing**  | ✅     | ✅  | ✅    | ✅        | 📖 Complete | Job queues, workers, processing   |
| **Database**          | -      | -   | ✅    | -         | 📖 Complete | Schema, migrations, repositories  |
| **Health Monitoring** | -      | -   | ✅    | ✅        | 📖 Complete | Endpoints, Fly.io integration     |
| **LLM Integration**   | ✅     | ✅  | ✅    | -         | 📖 Complete | Gemini, session mgmt, functions   |
| **Pipeline**          | ✅     | ✅  | -     | -         | 📖 Complete | Core speech processing flow       |
| **Schema Management** | ✅     | -   | -     | -         | 📖 Complete | Dynamic schema watcher            |
| **STT Integration**   | ✅     | ✅  | ✅    | -         | 📖 Complete | Provider routing, swappable APIs  |
| **Usage Tracking**    | ✅     | ✅  | ✅    | -         | 📖 Complete | Real-time accumulation, analytics |
| **WebSocket Core**    | ✅     | -   | -     | ✅        | 📖 Complete | Message protocol & communication  |

**Legend:**

-   ✅ Component exists in codebase
-   📖 Complete documentation
-   ⚠️ Partial documentation
-   ❌ Missing documentation
-   `-` Component doesn't exist in this layer

## 📁 Directory Structure

### Core Architecture

-   [`overview.md`](./overview.md) - High-level system architecture and design principles
-   [`mvp/`](./mvp/) - MVP sprint planning and progress tracking

### Domain Layer (`internal/domain/`)

-   [`auth/`](./auth/) - Authentication and authorization domain models
-   `speech/` - Speech-to-text and audio processing domains ⚠️ _Needs docs_
-   `usage/` - Usage metering and cost tracking domains ⚠️ _Needs docs_
-   `batch/` - Batch processing domain models ⚠️ _Needs docs_
-   `session/` - Session lifecycle and state management ⚠️ _Needs docs_
-   `silence/` - Silence detection domain interfaces ✅ _In audio docs_

### Application Layer (`internal/app/`)

-   [`pipeline/`](./pipeline/) - Core speech processing pipeline
-   [`auth/`](./auth/) - Authentication service implementation
-   [`batch/`](./batch/) - Batch job orchestration
-   [`usage/`](./usage/) - Usage accumulation and aggregation
-   `silence/` - Silence detection service ✅ _In audio docs_
-   `prompts/` - LLM prompt generation ⚠️ _Needs docs_
-   `spelling/` - Speech spelling correction ⚠️ _Needs docs_
-   `draft/` - Draft function detection ⚠️ _Needs docs_

### Infrastructure Layer (`internal/infra/`)

-   [`database/`](./database/) - Database management and migrations
-   [`fastparser/`](./fastparser/) - ML-based function parsing
-   [`audio/`](./audio/) - Audio processing and buffering
-   [`stt/`](./stt/) - STT provider routing and implementations
-   [`llm/`](./llm/) - LLM integration and session management
-   `auth/` - Authentication infrastructure ⚠️ _Needs docs_

### Transport Layer (`internal/transport/`)

-   [`health/`](./health/) - Health and readiness endpoints
-   [`websocket/`](./websocket/) - WebSocket protocol and message types
-   `http/` - HTTP endpoints and middleware ⚠️ _Needs docs_

### Schema Management

-   [`schema/`](./schema/) - Dynamic schema watcher system

## 🏗️ System Components

### 1. Real-Time Speech Pipeline

```
Audio Input → STT → Fast Parser → LLM → Function Calls
     ↓           ↓         ↓         ↓         ↓
Ring Buffer  Transcripts Drafts  Structured  Usage
```

**Key Components:**

-   **WebSocket Handler**: Manages real-time audio streaming
-   **STT Router**: Deepgram/Google speech-to-text routing
-   **Fast Parser**: ML-based draft function detection
-   **LLM Integration**: Gemini-based function call generation
-   **Pipeline Orchestrator**: Coordinates entire flow

**Documentation Status:** 📖 Complete

### 2. Authentication & Authorization

```
API Key → Cache Lookup → Principal → Rate Limiting → Session
```

**Key Components:**

-   **Auth Middleware**: API key validation and context injection
-   **Settings Cache**: LRU cache for app configurations
-   **Rate Limiter**: Per-app request throttling
-   **Principal System**: Authenticated identity management

**Documentation Status:** 📖 Complete

### 3. Usage Tracking & Analytics

```
Audio/LLM Events → Usage Accumulator → Database → Analytics
```

**Key Components:**

-   **Usage Meter**: Real-time usage accumulation
-   **Draft Aggregator**: Function detection analytics
-   **Event Logger**: Detailed usage event tracking
-   **Cost Calculator**: Usage-based billing calculations

**Documentation Status:** ⚠️ Partial - Need aggregation system docs

### 4. Batch Processing

```
File Upload → Job Queue → Background Worker → Result Storage
```

**Key Components:**

-   **Job Queue Manager**: Event-driven job dispatch
-   **Batch Processor**: Audio file processing pipeline
-   **Status Tracker**: Job status and progress monitoring
-   **Result Handler**: Processed data storage

**Documentation Status:** 📖 Complete

### 5. Health Monitoring

```
Component Checks → Status Aggregation → HTTP Endpoints → Fly.io
```

**Key Components:**

-   **Health Checker**: Database, API, filesystem validation
-   **Readiness Probe**: External dependency monitoring
-   **Metrics Collection**: Performance and error tracking
-   **Auto-Recovery**: Fly.io integration for restarts

**Documentation Status:** 📖 Complete

### 6. Dynamic Schema Management

```
Schema Updates → Change Detection → Session Notification → Hot Swap
```

**Key Components:**

-   **Schema Watcher**: Database polling for schema changes
-   **Version Manager**: Schema versioning and validation
-   **Session Notifier**: Real-time update propagation
-   **Hot-Swap Engine**: Atomic LLM/parser updates

**Documentation Status:** 📖 Complete

## 📝 Documentation Priorities

### High Priority (Core System Understanding)

1. ~~**Pipeline Architecture**~~ - ✅ Complete
2. ~~**WebSocket Protocol**~~ - ✅ Complete
3. **STT Integration** - Provider routing and configuration
4. **LLM Integration** - Gemini session management and tool configuration

### Medium Priority (Operational Knowledge)

5. **Usage Aggregation** - How usage metrics are collected and calculated
6. **Session Management** - Session lifecycle and state tracking
7. **Error Handling** - System-wide error handling patterns
8. **Testing Strategy** - How to test different components

### Low Priority (Implementation Details)

9. **Spelling Correction** - Speech text post-processing
10. **Draft Detection** - ML-based function extraction
11. **Prompt Generation** - LLM prompt construction
12. **Configuration Management** - Environment and feature flags

## 🔗 Cross-References

### Related External Documentation

-   [`README.md`](../../README.md) - Project overview and setup
-   [`Dockerfile`](../../Dockerfile) - Container configuration
-   [`fly.toml`](../../fly.toml) - Deployment configuration
-   [`Makefile`](../../Makefile) - Development workflow

### Database Documentation

-   [`atlas.hcl`](../../atlas.hcl) - Schema management configuration
-   [`internal/infra/db/schema.hcl`](../../internal/infra/db/schema.hcl) - Database schema

### Testing Documentation

-   `internal/testutil/` - Testing utilities and helpers
-   `test/` - Integration and end-to-end tests

## 🚀 Next Steps

To complete the documentation, we need to create:

1. ~~**`docs/internal/pipeline/`**~~ - ✅ Complete
2. ~~**`docs/internal/websocket/`**~~ - ✅ Complete
3. **`docs/internal/stt/`** - Speech-to-text integration and routing
4. **`docs/internal/llm/`** - LLM integration and session management
5. **`docs/internal/usage/`** - Usage tracking and aggregation systems
6. **`docs/internal/session/`** - Session lifecycle and state management

Each documentation section should include:

-   **Architecture overview** with diagrams
-   **Key components** and their responsibilities
-   **Integration patterns** and dependencies
-   **Configuration options** and environment variables
-   **Error handling** and edge cases
-   **Testing approach** and examples
-   **Performance considerations** and monitoring
