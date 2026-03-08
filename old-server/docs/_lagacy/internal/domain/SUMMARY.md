# Domain Layer Documentation Summary

## Overview

This document provides a summary of the comprehensive documentation created for the Schma system's domain layer. The domain layer contains the pure business logic and core contracts that define how the application works.

## Documentation Status

### ✅ Completed Documentation

#### 1. [Domain Overview](./README.md)

-   **Purpose**: High-level overview of the domain layer architecture
-   **Content**: Architecture principles, directory structure, core concepts, testing strategy
-   **Key Sections**: Clean Architecture principles, Domain-Driven Design concepts, usage examples

#### 2. [Speech Domain](./speech/README.md)

-   **Purpose**: Core contracts for speech processing
-   **Content**: STT, LLM, parsing, caching, and audio buffering interfaces
-   **Key Sections**:
    -   Data structures (AudioChunk, Transcript, Word, Turn)
    -   Interfaces (STTClient, LLM, StructuredLLM, FastParser, LLMCache)
    -   Configuration types (FunctionConfig, StructuredConfig)
    -   Caching system and audio buffering
    -   Usage examples and error handling

#### 3. [Session Domain](./session/README.md)

-   **Purpose**: Session lifecycle and state management
-   **Content**: Session creation, state transitions, configuration tracking
-   **Key Sections**:
    -   Session hierarchy (WebSocket vs Database sessions)
    -   Session lifecycle and status transitions
    -   Configuration tracking and dynamic updates
    -   Manager interface and usage examples
    -   Error handling and testing strategies

#### 4. [Usage Domain](./usage/README.md)

-   **Purpose**: Resource consumption tracking and cost calculation
-   **Content**: Metering, pricing, cost calculation, draft aggregation
-   **Key Sections**:
    -   Resource metering (audio, LLM, CPU)
    -   Cost calculation with pricing models
    -   Usage event logging and analytics
    -   Draft function aggregation
    -   Performance monitoring and optimization

### 🔄 In Progress Documentation

#### 5. Authentication Domain

-   **Status**: Needs documentation
-   **Files**: `internal/domain/auth/`
-   **Key Components**:
    -   Authentication ports and interfaces
    -   WebSocket session management
    -   App-level authentication
    -   Principal management

#### 6. Database Interfaces

-   **Status**: Needs documentation
-   **Files**: `internal/domain/db/`
-   **Key Components**:
    -   Repository interfaces
    -   CRUD operations
    -   Database contracts

#### 7. Event Logging

-   **Status**: Needs documentation
-   **Files**: `internal/domain/eventlog/`
-   **Key Components**:
    -   Event logging interfaces
    -   Audit trail management
    -   Event persistence contracts

#### 8. Silence Detection

-   **Status**: Needs documentation
-   **Files**: `internal/domain/silence/`
-   **Key Components**:
    -   Silence detection interfaces
    -   Audio monitoring contracts
    -   Silence event handling

#### 9. Batch Processing

-   **Status**: Needs documentation
-   **Files**: `internal/domain/batch/`
-   **Key Components**:
    -   Batch job interfaces
    -   Job lifecycle management
    -   Batch processing contracts

#### 10. Configuration Watcher

-   **Status**: Needs documentation
-   **Files**: `internal/domain/configwatcher/`
-   **Key Components**:
    -   Dynamic configuration interfaces
    -   Configuration change detection
    -   Configuration persistence contracts

## Documentation Standards

### Structure

Each domain documentation follows a consistent structure:

1. **Overview**: Purpose and scope
2. **Core Concepts**: Key ideas and principles
3. **Data Structures**: Types, structs, and enums
4. **Interfaces**: Ports and contracts
5. **Usage Examples**: Practical code examples
6. **Error Handling**: Error scenarios and recovery
7. **Testing**: Testing strategies and examples
8. **Implementation Notes**: Performance, thread safety, monitoring
9. **Future Enhancements**: Planned features and extension points

### Content Guidelines

-   **Clear Purpose**: Each section explains why it exists
-   **Practical Examples**: Real code examples for all concepts
-   **Error Scenarios**: Comprehensive error handling documentation
-   **Testing Strategies**: How to test each component
-   **Performance Notes**: Performance considerations and optimizations

## Key Architectural Principles

### Clean Architecture

-   **Dependency Direction**: Domain layer has no external dependencies
-   **Ports and Adapters**: Clear interfaces define external contracts
-   **Pure Business Logic**: No infrastructure concerns in domain
-   **Testability**: All logic can be tested without external dependencies

### Domain-Driven Design

-   **Entities**: Core business objects with identity
-   **Value Objects**: Immutable objects representing concepts
-   **Aggregates**: Clusters of related entities
-   **Ports**: Interfaces defining external dependencies

### Hexagonal Architecture

-   **Domain Core**: Pure business logic at the center
-   **Ports**: Interfaces defining what the domain needs
-   **Adapters**: Infrastructure implementations of ports
-   **Dependency Inversion**: Domain doesn't depend on infrastructure

## Usage Patterns

### Session Management

```go
// Create and manage sessions
session, err := manager.StartSession(ctx, wsSessionID, principal)
err = manager.UpdateSessionStatus(ctx, session.ID, SessionRecording)
err = manager.CloseSession(ctx, session.ID)
```

### Speech Processing

```go
// Process audio through STT
transcripts, err := sttClient.Stream(ctx, audioChunks)
for transcript := range transcripts {
    // Process transcript
}

// Extract functions with LLM
calls, usage, err := llm.Enrich(ctx, prompt, transcript, config)
```

### Usage Tracking

```go
// Track resource consumption
meter := usage.NewMeter(usage.DefaultPricing)
meter.AddSTT(10.5)
meter.AddTokens(100, 50)
cost := meter.CostUSD()
```

## Testing Strategy

### Unit Testing

-   **Fast Execution**: No external dependencies
-   **Deterministic**: Same inputs produce same outputs
-   **Comprehensive**: All business rules tested
-   **Maintainable**: Easy to understand and modify

### Integration Testing

-   **Database Integration**: Test with real database connections
-   **Concurrency Testing**: Test multiple sessions and updates
-   **Configuration Testing**: Test dynamic configuration changes

## Next Steps

### Immediate Priorities

1. **Complete Remaining Domains**: Document auth, db, eventlog, silence, batch, and configwatcher domains
2. **Cross-Reference**: Add cross-references between related domains
3. **Examples**: Add more comprehensive usage examples
4. **Diagrams**: Add architectural diagrams for complex flows

### Future Enhancements

1. **API Documentation**: Generate API documentation from domain interfaces
2. **Code Examples**: Add more real-world usage examples
3. **Performance Guides**: Add performance optimization guides
4. **Migration Guides**: Document domain changes and migrations

## Contributing

### Documentation Updates

-   Update documentation when domain interfaces change
-   Add examples for new features
-   Review and update cross-references
-   Ensure consistency across all domain docs

### Quality Standards

-   **Accuracy**: All code examples must compile and run
-   **Completeness**: Cover all public interfaces and types
-   **Clarity**: Clear explanations for complex concepts
-   **Consistency**: Follow established patterns and structure

## Conclusion

The domain layer documentation provides a comprehensive guide to the core business logic of the Schma system. It serves as the foundation for understanding how the application works and how to extend it with new features.

The documentation follows clean architecture principles and provides practical examples for all major concepts. It's designed to help developers understand the system quickly and contribute effectively to its development.

As the system evolves, this documentation will be updated to reflect new features, improved patterns, and lessons learned from real-world usage.
