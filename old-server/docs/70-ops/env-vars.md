---
audience: engineers
status: draft
last_verified: 2024-03-21
---

# Environment Variables

## You'll learn

-   Required and optional environment variables
-   Configuration types and defaults
-   Development vs production settings

## Where this lives in hex

Operations configuration, used across all layers.

## Variables

| NAME                   | Type   | Default | Required | Description                  |
| ---------------------- | ------ | ------- | -------- | ---------------------------- |
| DATABASE_URL           | string | -       | yes      | PostgreSQL connection string |
| SCHMA_API_PORT         | int    | 8080    | yes      | WebSocket server port        |
| DEEPGRAM_API_KEY       | string | -       | yes      | STT provider API key         |
| GOOGLE_STT_CREDENTIALS | json   | -       | no       | Fallback STT credentials     |

## Development Settings

-   [ ] TODO: Document local development defaults
-   [ ] TODO: Document recommended development overrides

## Production Requirements

-   [ ] TODO: Document required production settings
-   [ ] TODO: Document recommended security practices

## Configuration Loading

-   [ ] TODO: Document how env vars are loaded
-   [ ] TODO: Document any validation or transformation
