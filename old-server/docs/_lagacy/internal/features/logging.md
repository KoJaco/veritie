# Logging Strategy

## Overview

Schma now ships with a lightweight level-aware logger located at
`internal/pkg/logger`. It replaces raw `log.Printf` calls inside the
WebSocket handler, pipeline, and other high-traffic paths, allowing you
to choose how much information is printed at runtime.

```
import "schma.ai/internal/pkg/logger"

logger.SetLevel(logger.ParseLevel(os.Getenv("LOG_LEVEL")))
```

| Level | Env value | Typical use                               |
| ----- | --------- | ----------------------------------------- |
| DEBUG | `debug`   | Per-chunk traces, dev troubleshooting     |
| INFO  | `info`    | Session lifecycle, cost lines _(default)_ |
| WARN  | `warn`    | Non-fatal anomalies                       |
| ERROR | `error`   | Fatal errors, panics                      |

_All DEBUG logs are completely suppressed at INFO and above._

## Migration Notes

-   Replace `log.Printf` ➜ `logger.Debugf / Infof / Warnf / Errorf`.
-   Prefer **removing** messages that are no longer useful.
-   The legacy imports have already been migrated in `internal/transport/ws` and `internal/app/pipeline`.
