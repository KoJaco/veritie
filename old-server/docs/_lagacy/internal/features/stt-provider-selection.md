# STT Provider Selection

## Overview

This feature allows clients to specify which Speech-to-Text provider to use on a per-session basis, enabling optimization based on user location, provider performance, or specific use case requirements.

## Supported Providers

-   **Deepgram** (default): Fast, real-time optimized STT
-   **Google Cloud STT**: High accuracy, broad language support

## Client Configuration

### TypeScript SDK

```typescript
import { useSchma.ai } from "@/lib/sdk/useSchma.ai";

const config = {
    apiUrl: "ws://localhost:8080/ws",
    apiKey: "your-api-key",
    stt: {
        provider: "deepgram" | "google", // Choose STT provider
        sampleHertz: 16000,
        interimStabilityThreshold: 0.8,
    },
    functionConfig: {
        // ... your function definitions
    },
};

const { transcriptFinal, functions } = useSchma.ai({ config });
```

### Configuration Options

```typescript
interface STTConfig {
    provider?: "deepgram" | "google"; // STT provider selection
    interimStabilityThreshold?: number;
    encoding?: string;
    sampleHertz?: number;
}
```

## Server Implementation

### STT Factory Pattern

The server uses a factory pattern to create STT clients per session:

```go
// STT Factory creates clients based on provider configuration
type STTFactory interface {
    CreateSTTClient(ctx context.Context, provider string) (speech.STTClient, error)
}
```

### Per-Session Provider Selection

```go
// WebSocket handler creates STT client based on client config
sttProvider := cfgMsg.STT.Provider
if sttProvider == "" {
    sttProvider = "deepgram" // Default to Deepgram
}

sttClient, err := h.deps.STTFactory.CreateSTTClient(r.Context(), sttProvider)
```

### Fallback Mechanism

If the requested provider fails to initialize, the server falls back to the default STT client:

```go
if err != nil {
    log.Printf("❌ [WS] Failed to create STT client for provider %s: %v", sttProvider, err)
    log.Printf("🔄 [WS] Falling back to default STT client")
    sttClient = h.deps.STT // Use fallback STT client
}
```

## Performance Comparison

Based on latency analysis:

| Provider | First Transcript | Interim Frequency | Best Use Case               |
| -------- | ---------------- | ----------------- | --------------------------- |
| Deepgram | 1-3 seconds      | 500ms-2s          | Real-time, low latency      |
| Google   | 2-4 seconds      | 1-3s              | High accuracy, multilingual |

## Environment Configuration

### Required Environment Variables

```bash
# For Deepgram support
DEEPGRAM_API_KEY=your_deepgram_key

# For Google Cloud STT support
GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json

# Default provider (optional)
SCHMA_STT_PROVIDER=deepgram
```

### Server Configuration

```go
// main.go - STT Factory initialization
googleSttCfg := sttgoogle.Config{
    Encoding:        gstt.RecognitionConfig_WEBM_OPUS,
    SampleRateHertz: 16000,
    LanguageCode:    "en-US",
    Punctuate:       true,
}

sttFactory := sttfactory.NewFactory(googleSttCfg)

deps := pipeline.Deps{
    STT:        sttrouter.New(ctx, googleSttCfg), // Fallback
    STTFactory: sttFactory,                       // Per-session factory
    // ... other deps
}
```

## Usage Examples

### Basic Usage (Default Deepgram)

```typescript
const config = {
    apiUrl: "ws://localhost:8080/ws",
    apiKey: "your-api-key",
    // STT provider defaults to "deepgram"
};
```

### Explicit Provider Selection

```typescript
const config = {
    apiUrl: "ws://localhost:8080/ws",
    apiKey: "your-api-key",
    stt: {
        provider: "google", // Use Google Cloud STT
    },
};
```

### Provider-Specific Optimization

```typescript
// For high accuracy requirements
const accuracyConfig = {
    stt: { provider: "google" },
};

// For low latency requirements
const latencyConfig = {
    stt: { provider: "deepgram" },
};
```

## Testing

### Manual Testing

1. **Test Deepgram provider:**

```typescript
const config = { stt: { provider: "deepgram" } };
```

2. **Test Google provider:**

```typescript
const config = { stt: { provider: "google" } };
```

3. **Test fallback behavior:**

```typescript
const config = { stt: { provider: "invalid-provider" } };
// Should fallback to default STT client
```

### Expected Server Logs

```
🔊 [WS] Creating STT client for provider: deepgram
✅ [WS] Successfully created STT client for provider: deepgram
```

Or with fallback:

```
🔊 [WS] Creating STT client for provider: google
❌ [WS] Failed to create STT client for provider google: credentials not found
🔄 [WS] Falling back to default STT client
```

## Monitoring & Observability

The server logs provider selection and creation success/failure:

-   `🔊 [WS] Creating STT client for provider: {provider}` - Provider selection
-   `✅ [WS] Successfully created STT client for provider: {provider}` - Success
-   `❌ [WS] Failed to create STT client for provider {provider}: {error}` - Failure
-   `🔄 [WS] Falling back to default STT client` - Fallback used

## Benefits

1. **Performance Optimization**: Choose fastest provider per user location
2. **Accuracy Optimization**: Select most accurate provider per use case
3. **Cost Optimization**: Route to most cost-effective provider
4. **Reliability**: Automatic fallback prevents service disruption
5. **A/B Testing**: Compare provider performance with real users

## Migration Guide

### From Static Provider (Environment Variable)

**Before:**

```bash
SCHMA_STT_PROVIDER=deepgram
```

**After:**

```typescript
// Client-side control
const config = {
    stt: { provider: "deepgram" },
};
```

### Backward Compatibility

-   Existing clients without `stt.provider` default to Deepgram
-   Environment variable `SCHMA_STT_PROVIDER` still works as server fallback
-   No breaking changes to existing API

---

_This feature enables dynamic STT provider selection while maintaining full backward compatibility and providing robust fallback mechanisms._
