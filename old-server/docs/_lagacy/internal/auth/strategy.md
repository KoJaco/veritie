# Auth Strategy

Schma is a backend WebSocket server that other apps connect to via SDK. Schma DOES NOT manage end users, thus the authentication model focuses entirely on validating the calling app and tracking usage/session data per app.

## App-level Auth Only

-   Every app created in Schma has a unique API key (long random uuid/token)
-   We do NOT authenticate the end users of the integrating app.
-   All actions (sessions, function calls, usage, etc) are logged against the app ID which belongs to an account.

## The client connection

1. Still maintain the client config, allow clients to insert their api key in the config object using environment variables

```typescript
const config = {

    apiUrl: "wss://schma-api.fly.dev/ws",
    apiKey: process.env("MEMONIC_API_KEY"),
    sttConfig: {
        ...
    },
    functionParsingConfig: {
        ...
    },
    ... // omitted for brevity
}
```

2. inside SDK: Secure WebSocket Header Auth approach.

```typescript
new WebSocket("wss://schma-api.fly.dev/ws", {
    headers: {
        "x-api-key": "<app_api_key>",
    },
});
```

3. (OPTIONAL) Fallback, if headers cannot be sent (some clients block it?)... require the first JSON message to be of `auth` type, until that message is received reject all traffic.

This may need to be implemented, but has not been done yet

```json
{ "type": "auth", "apiKey": "<app_api_key>" }
```

## Verify API Key on Connect

1.  Check that the API key exists in the `apps` table.
2.  Check if the app is still associated with an active account (`apps.account_id` -> `accounts.subscription_status`).
3.  Cache this app config and necessary account info during the session lifecycle

## Store session data with app context

1.  When a session is started, log it under `app.app_id`
2.  (OPTIONAL) snapshot the app config used (`config_snapshot`) locally
3.  All downstream data (transcripts, functions, usage) gets tied to the `session.session_id`, which maps to `app.app_id`

## JWT Keys (Post MVP Most Likely)

(NOT YET IMPLEMENTED)

-   JWT API Keys (signed by Schma backend)

## Rate Limiting & Billing (End MVP Sprint)

-   Enforce quotas (per app or account plan)
-   Block/limit request if `account.subscription_status = 'past_due'`
