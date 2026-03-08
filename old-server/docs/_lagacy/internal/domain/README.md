# Domain Layer Documentation

## Overview

The domain layer contains the pure business logic of the Schma system - the core rules, entities, and contracts that define how the application works. This layer is completely isolated from external concerns like databases, HTTP, or logging.

## Architecture Principles

### Clean Architecture

-   **No external dependencies**: The domain layer imports only standard library packages and other domain sub-packages
-   **Ports define contracts**: Interfaces define what the domain needs from the outside world
-   **Pure business logic**: No infrastructure concerns, logging, or external API calls
-   **Testable**: All logic can be tested with simple table-driven tests, no mocks needed

### Domain-Driven Design

-   **Entities**: Core business objects with identity and lifecycle
-   **Value Objects**: Immutable objects that represent concepts
-   **Aggregates**: Clusters of related entities with consistency boundaries
-   **Ports**: Interfaces that define external dependencies

## Directory Structure

```
internal/domain/
├── auth/           # Authentication and authorization
├── batch/          # Batch job processing
├── configwatcher/  # Dynamic configuration management
├── db/             # Database repository interfaces
├── eventlog/       # Event logging and auditing
├── session/        # Session lifecycle management
├── silence/        # Silence detection and monitoring
├── speech/         # Speech processing contracts
└── usage/          # Usage metering and billing
```

## Core Concepts

### Session Management

Sessions represent active voice processing connections. They have a lifecycle from creation to completion and track all associated data.

### Speech Processing

The speech domain defines contracts for:

-   **STT (Speech-to-Text)**: Converting audio to text
-   **LLM Integration**: Language model processing
-   **Parsing**: Extracting structured data from text
-   **Caching**: Optimizing LLM calls

### Usage Metering

Tracks resource consumption for billing and analytics:

-   Audio processing time
-   LLM token usage
-   CPU utilization
-   Cost calculations

### Authentication

Handles app-level authentication and session-level authorization.

## Testing Strategy

The domain layer uses table-driven tests with no external dependencies:

-   **Fast execution**: No network calls or database connections
-   **Deterministic**: Same inputs always produce same outputs
-   **Comprehensive**: All business rules are tested
-   **Maintainable**: Tests are easy to understand and modify

## Usage Examples

### Creating a Session

```go
session := session.New(sessionID, appID, sessionType)
session.Start()
```

### Processing Speech

```go
transcript := speech.Transcript{
    Text: "Hello world",
    IsFinal: true,
    Confidence: 0.95,
}
```

### Metering Usage

```go
meter := usage.NewMeter()
meter.AddSTT(10.5) // 10.5 seconds of audio
meter.AddLLM(100, 50) // 100 prompt + 50 completion tokens
```

## Next Steps

Each subdomain is documented in detail in its own file:

-   [Authentication](./auth/README.md)
-   [Batch Processing](./batch/README.md)
-   [Configuration Management](./configwatcher/README.md)
-   [Database Interfaces](./db/README.md)
-   [Event Logging](./eventlog/README.md)
-   [Session Management](./session/README.md)
-   [Silence Detection](./silence/README.md)
-   [Speech Processing](./speech/README.md)
-   [Usage Metering](./usage/README.md)
