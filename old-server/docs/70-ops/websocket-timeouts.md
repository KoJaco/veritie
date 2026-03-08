---
audience: engineers
status: draft
last_verified: 2024-08-29
---

# WebSocket Timeout Configuration

## Overview

This document describes the configurable timeout settings for WebSocket connections to support super long sessions (up to 2 hours).

## Environment Variables

| Variable                     | Default         | Description                                              |
| ---------------------------- | --------------- | -------------------------------------------------------- |
| `WS_SESSION_TIMEOUT_SECONDS` | 7200 (2 hours)  | Maximum time a WebSocket connection can stay open        |
| `WS_READ_TIMEOUT_SECONDS`    | 300 (5 minutes) | How long to wait for data from the client                |
| `WS_PING_INTERVAL_SECONDS`   | 30              | How often to send ping messages to keep connection alive |
| `WS_PING_TIMEOUT_SECONDS`    | 10              | How long to wait for pong response                       |

## Configuration Examples

### Development (Short Sessions)

```bash
export WS_SESSION_TIMEOUT_SECONDS=300    # 5 minutes
export WS_READ_TIMEOUT_SECONDS=60        # 1 minute
export WS_PING_INTERVAL_SECONDS=15       # 15 seconds
export WS_PING_TIMEOUT_SECONDS=5         # 5 seconds
```

### Production (Long Sessions)

```bash
export WS_SESSION_TIMEOUT_SECONDS=7200   # 2 hours (default)
export WS_READ_TIMEOUT_SECONDS=600       # 10 minutes
export WS_PING_INTERVAL_SECONDS=60       # 1 minute
export WS_PING_TIMEOUT_SECONDS=15        # 15 seconds
```

### Ultra Long Sessions (1+ Hours)

```bash
export WS_SESSION_TIMEOUT_SECONDS=14400  # 4 hours
export WS_READ_TIMEOUT_SECONDS=1800      # 30 minutes
export WS_PING_INTERVAL_SECONDS=120      # 2 minutes
export WS_PING_TIMEOUT_SECONDS=30        # 30 seconds
```

## Session Management

### Timeout Handling

-   **Graceful Timeouts**: WebSocket sends `connection_close` message before closing
-   **Session Resumption**: Timeout messages indicate session can be resumed
-   **Error Differentiation**: Distinguishes between timeouts and actual errors

### Best Practices

1. **Read Timeout**: Should be longer than expected silence periods
2. **Ping Interval**: Balance between connection health and overhead
3. **Session Timeout**: Should match your application's session requirements
4. **Ping Timeout**: Should be shorter than read timeout

### Monitoring

-   Logs show session duration when timeouts occur
-   Connection close messages indicate reason for closure
-   Session manager TTL (1 hour) is separate from WebSocket timeout

## Architecture Benefits

-   **Configurable**: Environment-based configuration for different environments
-   **Graceful**: Proper cleanup and client notification
-   **Resumable**: Sessions can be resumed after timeout
-   **Monitored**: Comprehensive logging for debugging
-   **Scalable**: Supports sessions from minutes to hours
