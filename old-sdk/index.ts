/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unused-expressions */

import { v4 as uuid } from "uuid";
// TODO: Implement runtime val with Zod
// TODO: Implement Reconnection / back-off
// TODO: Add MediaRecorder WebM/Opus fallback
// TODO: Add in server-server auth, don't expose API key here in the client.

// ----------------- Primitives -----------------
type FunctionParamType =
    | "unspecified"
    | "string"
    | "number"
    | "integer"
    | "boolean"
    | "array"
    | "object";

/**
 * A subset of the OpenAPI 3.0 Schema Object.
 * This interface can be used for both output format schema and function calling schema. Google Gemini only accepts schemas of the following type.
 * -- DOCS: https://ai.google.dev/gemini-api/docs/structured-output?lang=rest#supply-schema-in-prompt
 *
 */
export type GenaiSchema = {
    // a genai.TypeObject
    // This is an int32 which map out to a specific type. 0 = "TypeUnspecified", 1 = "TypeString", 2 = "TypeNumber", 3 = "TypeInteger", 4 = "TypeBoolean", 5 = "TypeArray", 6 = "TypeObject"
    // note, included 'unspecified' so this must be included or listed as 'unspecified' in the client.
    type: FunctionParamType;

    // * OPTIONAL FIELDS
    description?: string;
    // format of the data, only used for primitives.. i.e., NUMBER type: float, double... INTEGER type: int32, int64... just handle on server
    format?: string;
    // indicates whether the value may be null.
    nullable?: boolean;
    // TODO: set enum format in parsing of schema on server
    // possible values of the element of Type.STRING with enum format.... for example, an Enum for the value 'headingLevel' as: {type: STRING, format: enum, enum: ["1", "2", "3"]}
    enum?: string[];
    // Schema of the elements of Type.ARRAY
    items?: GenaiSchema;
    // Object where keys represent the property name and values are a GenaiSchema.
    /**
     * {
     *  "level": {
     *      type: "string",
     *      format: enum,
     *      enum: ["1", "2", "3"],
     *      description: "Describes the hierarchy of the heading element. Level 1 will be H1, level 2 will be H2, level 3 will be H3."
     *  },
     *  {
     *  "content": {
     *      type: "string",
     *      description: "The actual text content of the heading."
     * },
     *  "order": {
     *      type: "number",
     *      description: "Where the heading lies within the overall document."
     * }
     *  }
     * }
     */
    properties?: { [key: string]: GenaiSchema };
    // Required properties of Type.OBJECT... use array here.
    // {"brightness", "temperature"}
    required?: string[];
};

// ---------------- config sent by client ---------------

export type FunctionCallingSchemaObject = {
    // The name of the function, i.e., 'updateHeadingElement'
    name: string;
    // The description of the function, i.e., 'Used to update an existing heading element using parameters to control the heading hierarchy (level 1, 2, 3) and setting the actual text content of the heading 'This is a heading'.
    description: string;
    // a genai.Schema -- expected in GoLang
    parameters: GenaiSchema;
};

export type StructuredOutputSchemaObject = {
    name: string;
    description: string;
    type: "object";
    additionalProperties?: boolean; // if true, the object can have additional properties not specified in the schema
    required?: string[];
    properties: { [key: string]: GenaiSchema };
};

export interface STTConfig {
    provider?: "deepgram" | "google"; // STT provider selection
    interimStabilityThreshold?: number; // => interim_stability_threshold
    encoding?: string;
    sampleHertz?: number; // => sample_hertz
    diarization?: {
        enableSpeakerDiarization: boolean;
        minSpeakerCount?: number;
        maxSpeakerCount?: number;
    };
}

export type ParsingStrategy =
    | "auto"
    | "update-ms"
    | "after-silence"
    | "end-of-session";

export type TranscriptMode = "full" | "window";
export type PrevOutputMode = "apply" | "ignore" | "keys-only" | "window";

export interface TranscriptInclusionPolicy {
    transcriptMode: TranscriptMode;
    windowTokenSize: number;
    tailSentences: number;
}

export interface PrevOutputInclusionPolicy {
    prevOutputMode: PrevOutputMode;
}

export interface ParsingConfig {
    parsingStrategy: ParsingStrategy;
    transcriptInclusionPolicy: TranscriptInclusionPolicy;
    prevOutputInclusionPolicy: PrevOutputInclusionPolicy;
}

export interface FunctionConfig {
    name: string;
    description: string;
    updateMs?: number; // => update_ms
    parsingGuide?: string; // => parsing_guide
    definitions: FunctionCallingSchemaObject[];
    parsingConfig: ParsingConfig;
}

export interface StructuredOutputConfig {
    parsingGuide?: string;
    updateMs?: number;
    schema: StructuredOutputSchemaObject;
    parsingConfig: ParsingConfig;
}

export type StructuredOutputReceived = {
    rev: number;
    delta: { [key: string]: any };
    final: { [key: string]: any };
};

export interface InputContext {
    currentRawTranscript?: string; // => current_raw_transcript
    currentFunctions?: FunctionCallReceived[]; // => current_functions
    currentStructured?: { [key: string]: any }; // => current_structured
}

export interface ClientConfig {
    apiUrl: string;
    isTest?: boolean;
    // apiKey: string; // API key for authentication
    wsSessionId?: string; // => ws_session_id
    language?: string;
    stt?: STTConfig;
    functionConfig?: FunctionConfig; // => function_config
    structuredOutputConfig?: StructuredOutputConfig; // => structured_output_config
    inputContext?: InputContext; // => input_context
    // Optional silence detection config (client-side)
    silenceDetection?: {
        threshold?: number; // dBFS threshold for silence (-38 default)
        checkInterval?: number; // ms between checks (300 default)
        keepAliveInterval?: number; // ms between keep-alive (5000 default)
        minSilenceMs?: number; // ms below threshold before entering silence (2000 default)
        minVoiceMs?: number; // ms above exit threshold before exiting silence (800 default)
        exitHysteresisDb?: number; // dB above threshold to exit silence (6 default)
    };
    // Optional audio mixing config (client-side)
    audioMixing?: {
        mixDevicePlayback?: boolean; // mix system/device playback audio with mic (default: true)
        micGain?: number; // linear gain for mic (default: 1.0)
        deviceGain?: number; // linear gain for device playback (default: 1.0)
    };
    // Optional batch REST API configuration
    batchApi?: {
        baseUrl?: string; // Base URL for REST endpoints (defaults to derived from apiUrl or window.origin)
        getAuthHeaders?: () => Promise<Record<string, string>>; // Return headers for API key auth
        apiKey?: string; // If provided (trusted env only), used when getAuthHeaders not provided
        headerName?: string; // Default: Authorization (Bearer <apiKey>); or e.g. "x-api-key"
        uploadPath?: string; // default: /action/schma-batch
        statusPath?: string; // default: /action/schma-batch-status
        jobsPath?: string; // default: /action/schma-batch-jobs
    };
    redactionConfig?: {
        disablePhi?: boolean;
    };

    // how the browser obtains a short-lived WS token from the integrator's server route
    getToken: GetWsToken;
}

// ---------- Messages received back *from* the server -------------

export interface FunctionCallReceived {
    name: string;
    args: { [name: string]: any };
}

export type Word = {
    text: string;
    start: number;
    end: number;
    confidence?: number;
    punctuatedWord?: string;
    speaker?: string;
    speakerConfidence?: number;
};

export type PhraseDisplay = {
    start: number;
    end: number;
    confidence?: number;
    speaker?: string;
    textNorm?: string;
    textMasked?: string;
};

export type Turn = {
    id?: string;
    speaker: string;
    start: number;
    end: number;
    words?: Word[];
    confidence?: number;
    final?: boolean;
};

export interface FunctionDraftDataReceived {
    draftId: string; // <= draft_id
    name: string;
    args: { [key: string]: any };

    similarityScore: number; // <= similarity_score
    status:
        | "pending_confirmation"
        | "confirmed_by_llm"
        | "awaiting_potential_update";
    timestamp: string;
}

export interface TranscriptMsg {
    type: "transcript";
    text: string;
    final: boolean;
    confidence?: number;
    stability?: number;
    words?: Word[];
    // diarization
    turns?: Turn[];
    channel?: number;
    phrasesDisplay?: PhraseDisplay[];
}

export interface FunctionMsg {
    type: "functions";
    functions: FunctionCallReceived[];
}

export interface StructuredOutputMsg {
    type: "structured_output";
    rev: number;
    delta: { [key: string]: any };
    final: { [key: string]: any };
}

export interface DraftMsg {
    type: "function_draft_extracted";
    data: FunctionDraftDataReceived;
}

export interface AckMsg {
    type: "ack";
    wsSessionId: string; // <= ws_session_id
}

export interface SessionEndMsg {
    type: "session_end";
}

export interface ConfigUpdateAckMsg {
    type: "config_update_ack";
    success: boolean;
    message?: string;
}

export type ServerMsg =
    | TranscriptMsg
    | FunctionMsg
    | DraftMsg
    | AckMsg
    | SessionEndMsg
    | ConfigUpdateAckMsg
    | StructuredOutputMsg
    | { type: "error"; err: string };

// ----------- Helper types --------------------------
export interface GenaiObjectSchema<T extends Record<string, GenaiSchema>> {
    type: "object"; // This should be the value that corresponds to an object type.
    // Here, properties is explicitly typed as T, and required is an array of keys of T.
    properties: T;
    required?: Array<Extract<keyof T, string>>;
}

type GetWsTokenArgs = {
    wsSessionId?: string;
    schemaChecksum?: string;
};

type GetWsToken = (args: GetWsTokenArgs) => Promise<string>;

// Diarization: group words into turns (same-speaker contiguous)
export function groupTurns(
    words?: Word[],
    isFinal?: boolean
): Turn[] | undefined {
    if (!words || words.length === 0) return undefined;
    const turns: Turn[] = [];
    let currentSpeaker = words[0].speaker ?? "";
    let currentStart = words[0].start;
    let currentEnd = words[0].end;
    let currentWords: Word[] = [words[0]];
    const flush = () => {
        if (currentWords.length === 0) return;
        turns.push({
            speaker: currentSpeaker,
            start: currentStart,
            end: currentEnd,
            words: [...currentWords],
            final: isFinal,
        });
    };
    for (let i = 1; i < words.length; i++) {
        const w = words[i];
        const speaker = w.speaker ?? "";
        if (speaker !== currentSpeaker) {
            flush();
            currentSpeaker = speaker;
            currentStart = w.start;
            currentWords = [];
        }
        currentWords.push(w);
        currentEnd = w.end;
    }
    flush();
    return turns;
}

// Diarization: group phrases (single-speaker) into contiguous turns
export function groupTurnsFromPhrases(
    phrases?: PhraseDisplay[],
    isFinal?: boolean
): Turn[] | undefined {
    if (!phrases || phrases.length === 0) return undefined;

    const turns: Turn[] = [];
    let currentSpeaker = phrases[0].speaker ?? "";
    let currentStart = phrases[0].start;
    let currentEnd = phrases[0].end;

    const flush = () => {
        turns.push({
            speaker: currentSpeaker,
            start: currentStart,
            end: currentEnd,
            final: isFinal,
        });
    };

    for (let i = 1; i < phrases.length; i++) {
        const p = phrases[i];
        const speaker = p.speaker ?? "";
        if (speaker !== currentSpeaker) {
            flush();
            currentSpeaker = speaker;
            currentStart = p.start;
        }
        currentEnd = p.end;
    }
    flush();
    return turns;
}

// Helper: for correctly typing required to the keys of the GenaiSchema object.
export function defineFunction<T extends Record<string, GenaiSchema>>(fn: {
    name: string;
    description: string;
    parameters: GenaiObjectSchema<T>;
}): FunctionCallingSchemaObject {
    return fn;
}

function toSnakeCase(str: string): string {
    return str.replace(/([A-Z])/g, (letter) => `_${letter.toLowerCase()}`);
}

function convertRequiredArrayToSnakeCase(arr: string[]): string[] {
    return arr.map(toSnakeCase);
}

function toCamelCase(str: string): string {
    return str.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
}

function convertRequiredArrayToCamelCase(arr: string[]): string[] {
    return arr.map(toCamelCase);
}

export function convertKeysToSnakeCase(obj: any): any {
    if (Array.isArray(obj)) {
        return obj.map(convertKeysToSnakeCase);
        // null is an object type
    } else if (obj !== null && typeof obj === "object") {
        return Object.keys(obj).reduce((acc, key) => {
            const snakeKey = toSnakeCase(key);

            const value = obj[key];

            // special handling for 'required' as values refer to object keys (must be in appropriate casing for function calling schema.)
            if (snakeKey === "required" && Array.isArray(value)) {
                acc[snakeKey] = convertRequiredArrayToSnakeCase(value);
            } else {
                acc[snakeKey] = convertKeysToSnakeCase(obj[key]);
            }

            return acc;
        }, {} as any);
    }

    return obj;
}

export function convertKeysToCamelCase(obj: any): any {
    if (Array.isArray(obj)) {
        return obj.map(convertKeysToCamelCase);
    } else if (obj !== null && typeof obj === "object") {
        return Object.keys(obj).reduce((acc, key) => {
            const camelKey = toCamelCase(key);

            const value = obj[key];

            if (camelKey === "required" && Array.isArray(value)) {
                acc[camelKey] = convertRequiredArrayToCamelCase(value);
            } else {
                acc[camelKey] = convertKeysToCamelCase(obj[key]);
            }

            return acc;
        }, {} as any);
    }
    return obj;
}

export type ConnStatus = "disconnected" | "connecting" | "connected" | "error";

// ----------------- Session Management Classes -----------------

class ConfigManager {
    private config = {
        // Match server WS_READ_TIMEOUT_SECONDS (default: 300s)
        readTimeout: 300000,
        // Match server WS_PING_INTERVAL_SECONDS (default: 30s)
        pingInterval: 30000,
        // Client-side silence detection interval
        silenceCheckInterval: 1000,
        // Keep-alive message interval during silence
        keepAliveInterval: 3000,
        // Session resumption window
        resumptionWindow: 60 * 60 * 1000, // 1 hour
        // Reconnection settings
        maxReconnectAttempts: 3,
        reconnectDelay: 1000, // Start with 1 second
        // Ping/pong timeout
        pingTimeout: 10000, // 10 seconds to respond to ping
    };

    updateConfig(newConfig: Partial<typeof this.config>) {
        this.config = { ...this.config, ...newConfig };
    }

    getConfig() {
        return { ...this.config };
    }
}

class SessionManager {
    private sessionId: string | null = null;
    private sessionState: any = null;

    setSessionId(id: string) {
        this.sessionId = id;
    }

    getSessionId(): string | null {
        return this.sessionId;
    }

    // Store session state for potential resumption
    saveSessionState(state: any) {
        this.sessionState = state;
        if (this.sessionId) {
            localStorage.setItem(
                `session_${this.sessionId}`,
                JSON.stringify({
                    timestamp: Date.now(),
                    state: this.sessionState,
                })
            );
        }
    }

    // Attempt to resume session after timeout
    async resumeSession(): Promise<{ canResume: boolean; state?: any }> {
        if (!this.sessionId) return { canResume: false };

        const savedState = localStorage.getItem(`session_${this.sessionId}`);
        if (savedState) {
            try {
                const { timestamp, state } = JSON.parse(savedState);
                const age = Date.now() - timestamp;

                // Only resume if session is less than 1 hour old
                if (age < 60 * 60 * 1000) {
                    this.sessionState = state;
                    return { canResume: true, state };
                }
            } catch (error) {
                console.warn("Failed to parse saved session state:", error);
            }
        }
        return { canResume: false };
    }

    clearSessionState() {
        if (this.sessionId) {
            localStorage.removeItem(`session_${this.sessionId}`);
        }
        this.sessionState = null;
    }
}

class MetricsManager {
    private sessionStartTime: number = 0;
    private metrics = {
        totalAudioTime: 0,
        silenceTime: 0,
        reconnectionAttempts: 0,
        errors: 0,
        pingsSent: 0,
        pongsReceived: 0,
        pingTimeouts: 0,
    };

    startSession() {
        this.sessionStartTime = Date.now();
        this.resetMetrics();
    }

    resetMetrics() {
        this.metrics = {
            totalAudioTime: 0,
            silenceTime: 0,
            reconnectionAttempts: 0,
            errors: 0,
            pingsSent: 0,
            pongsReceived: 0,
            pingTimeouts: 0,
        };
    }

    recordSilence(duration: number) {
        this.metrics.silenceTime += duration;
    }

    recordReconnection() {
        this.metrics.reconnectionAttempts++;
    }

    recordError() {
        this.metrics.errors++;
    }

    recordPing() {
        this.metrics.pingsSent++;
    }

    recordPong() {
        this.metrics.pongsReceived++;
    }

    recordPingTimeout() {
        this.metrics.pingTimeouts++;
    }

    getSessionMetrics() {
        return {
            ...this.metrics,
            sessionDuration: Date.now() - this.sessionStartTime,
        };
    }
}

class WebSocketManager {
    private reconnectAttempts = 0;
    private pingTimeoutTimer: number | null = null;
    private pingIntervalTimer: number | null = null;
    private lastPingTime: number = 0;

    constructor(
        private config: ConfigManager,
        private sessionManager: SessionManager,
        private metricsManager: MetricsManager,
        private onReconnect: () => Promise<void>,
        private onSessionFailure: () => void
    ) {}

    async handleConnectionError(error: any) {
        console.error("WebSocket error:", error);
        this.metricsManager.recordError();

        if (
            this.reconnectAttempts <
            this.config.getConfig().maxReconnectAttempts
        ) {
            this.reconnectAttempts++;
            const delay =
                this.config.getConfig().reconnectDelay *
                Math.pow(2, this.reconnectAttempts - 1);

            console.log(
                `Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`
            );

            setTimeout(() => {
                this.reconnect();
            }, delay);
        } else {
            console.error("Max reconnection attempts reached");
            this.onSessionFailure();
        }
    }

    private async reconnect() {
        try {
            await this.onReconnect();
            this.reconnectAttempts = 0; // Reset on successful connection
            this.metricsManager.recordReconnection();
        } catch (error) {
            this.handleConnectionError(error);
        }
    }

    startPingMonitoring(ws: WebSocket) {
        this.stopPingMonitoring();

        this.pingIntervalTimer = window.setInterval(() => {
            if (ws.readyState === WebSocket.OPEN) {
                this.sendPing(ws);
            }
        }, this.config.getConfig().pingInterval);
    }

    private sendPing(ws: WebSocket) {
        if (ws.readyState === WebSocket.OPEN) {
            ws.send("ping");
            this.lastPingTime = Date.now();
            this.metricsManager.recordPing();

            // Set timeout for pong response
            this.pingTimeoutTimer = window.setTimeout(() => {
                console.error("Ping timeout - no pong received");
                this.metricsManager.recordPingTimeout();
                ws.close(1000, "ping_timeout");
            }, this.config.getConfig().pingTimeout);
        }
    }

    handlePong() {
        if (this.pingTimeoutTimer) {
            clearTimeout(this.pingTimeoutTimer);
            this.pingTimeoutTimer = null;
        }
        this.metricsManager.recordPong();
    }

    stopPingMonitoring() {
        if (this.pingIntervalTimer) {
            clearInterval(this.pingIntervalTimer);
            this.pingIntervalTimer = null;
        }
        if (this.pingTimeoutTimer) {
            clearTimeout(this.pingTimeoutTimer);
            this.pingTimeoutTimer = null;
        }
    }

    resetReconnectAttempts() {
        this.reconnectAttempts = 0;
    }
}

export class SchmaSDK {
    // Transport
    private ws: WebSocket | null = null;
    private wsSessionToken: string | null = null;

    // Session Management
    private configManager: ConfigManager;
    private sessionManager: SessionManager;
    private metricsManager: MetricsManager;
    private wsManager: WebSocketManager;

    // Audio
    private mediaRecorder: MediaRecorder | null = null;
    private mediaStream: MediaStream | null = null;
    private mediaStreamMixed: MediaStream | null = null; // final stream sent to recorder
    private micStream: MediaStream | null = null; // raw mic
    private systemAudioStream: MediaStream | null = null; // raw device/system playback
    private isRecording = false;
    private firstChunkSent = false;

    // Silence detection (Web Audio API)
    private audioContext: AudioContext | null = null;
    private analyser: AnalyserNode | null = null;
    private microphone: MediaStreamAudioSourceNode | null = null;
    private silenceCheckTimer: number | null = null;
    private keepAliveTimer: number | null = null;
    private isSilent = false;
    private silenceSinceMs: number | null = null;
    private lastBelowThresholdAtMs: number | null = null;
    private lastAboveExitThresholdAtMs: number | null = null;

    // State
    private status: ConnStatus = "disconnected";
    private readonly chunkMs: number;
    private currentRev?: number;

    // Callbacks
    onFinal?: (
        text: string,
        words?: Word[],
        confidence?: number,
        turns?: Turn[],
        phrasesDisplay?: PhraseDisplay[]
    ) => void;
    onInterim?: (
        text: string,
        words?: Word[],
        confidence?: number,
        stability?: number,
        turns?: Turn[]
    ) => void;
    onFuncs?: (f: FunctionCallReceived[]) => void;
    // TODO: update type to receive structured output properly. Guarantee normalization on server. We want a flat object.
    onStructuredOutput?: (s: StructuredOutputReceived) => void;
    onDraft?: (d: FunctionDraftDataReceived) => void;
    onAck?: (id: string) => void;
    onEnd?: () => void;
    onErr?: (e: Error) => void;
    onStatus?: (s: typeof this.status) => void;
    onConfigUpdateAck?: (
        success: boolean,
        message?: string,
        rev?: number,
        activeChecksum?: string
    ) => void;

    // Constructor
    constructor(
        private url: string,
        private cfg: ClientConfig,
        chunkMs = 250
    ) {
        this.chunkMs = chunkMs;
        if (!cfg.wsSessionId) cfg.wsSessionId = uuid();

        // Initialize session management
        this.configManager = new ConfigManager();
        this.sessionManager = new SessionManager();
        this.metricsManager = new MetricsManager();

        // Set session ID
        this.sessionManager.setSessionId(cfg.wsSessionId!);

        // Initialize WebSocket manager with callbacks
        this.wsManager = new WebSocketManager(
            this.configManager,
            this.sessionManager,
            this.metricsManager,
            () => this.reconnectWithBackoff(),
            () => this.handleSessionFailure()
        );
    }

    // Session management methods
    private handleSessionFailure(): void {
        console.error("Session failed - max reconnection attempts reached");
        this.setStatus("error");
        this.onErr?.(
            new Error("Session failed - max reconnection attempts reached")
        );
        this.destroy();
    }

    public getSessionMetrics() {
        return this.metricsManager.getSessionMetrics();
    }

    public updateSessionConfig(
        newConfig: Partial<ReturnType<ConfigManager["getConfig"]>>
    ) {
        this.configManager.updateConfig(newConfig);
    }

    // Connection

    async connect(): Promise<void> {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            return;
        }

        this.setStatus("connecting");

        try {
            // 1) Obtain a short-lived JWT from caller's server route
            const sid = this.cfg.wsSessionId!;

            this.wsSessionToken = await this.cfg.getToken({
                wsSessionId: sid,
            });

            if (!this.wsSessionToken)
                throw new Error(
                    "Missing WS token, could not retrieve token using getToken. Please update your Server route appropriately."
                );

            // Step 2: WebSocket Connection with session token
            await this.connectWebSocket();
        } catch (error) {
            console.error("🔌 [SDK] Connection error:", error);
            this.emitErr(
                error instanceof Error ? error : new Error("Connection failed")
            );
        }
    }

    private async connectWebSocket(): Promise<void> {
        return new Promise((resolve, reject) => {
            if (!this.wsSessionToken) {
                console.error(
                    "❌ [SDK] No session token available for WebSocket connection"
                );
                reject(new Error("No session token available"));
                return;
            }

            this.ws = new WebSocket(this.url, [
                "schma.ws.v1",
                `auth.${this.wsSessionToken}`,
            ]);

            this.ws.onopen = () => {
                this.onWsOpen();
                resolve();
            };

            this.ws.onmessage = (ev) => this.onWsMsg(ev);

            this.ws.onerror = (event) => {
                console.error("❌ [SDK] WebSocket error:", event);
                const error = new Error("WebSocket connection error");
                this.emitErr(error);
                reject(error);
            };

            this.ws.onclose = async (event) => {
                // auth-related close? Re-mint token + retry a few times
                if (
                    event.code === 4401 ||
                    event.code === 4403 ||
                    event.code === 1008
                ) {
                    try {
                        await this.reconnectWithBackoff();
                        return; // success, do not fall through
                    } catch (e) {
                        // fall through to end if retries are exhausted
                        console.warn(
                            "❌ [SDK] Failed to reconnect with backoff, falling through to end"
                        );
                    }
                }

                this.setStatus("disconnected");
                this.onEnd?.();
            };
        });
    }

    private async reconnectWithBackoff(retries = 3, base = 250): Promise<void> {
        for (let i = 0; i < retries; i++) {
            try {
                this.wsSessionToken = await this.cfg.getToken({
                    wsSessionId: this.cfg.wsSessionId!,
                });

                if (!this.wsSessionToken) throw new Error("Missing WS Token");

                await this.connectWebSocket();
                return;
            } catch (err) {
                if (i === retries) throw err;
                await new Promise((r) => setTimeout(r, base * Math.pow(2, i)));
            }
        }
    }

    disconnect(): void {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.stopRecording();
            this.ws.close(1000, "client disconnect");
        }

        this.setStatus("disconnected");
    }

    private setStatus(s: typeof this.status) {
        this.status = s;
        this.onStatus?.(s);
    }

    // only send safe fields from cfg (avoid leaking getToken/appId)
    private sendInitialConfigSafely() {
        if (this.ws?.readyState !== WebSocket.OPEN) return;

        const payload = convertKeysToSnakeCase({
            type: "config",
            // wsSessionId: this.cfg.wsSessionId,
            language: this.cfg.language,
            stt: this.cfg.stt,
            functionConfig: this.cfg.functionConfig,
            structuredOutputConfig: this.cfg.structuredOutputConfig,
            inputContext: this.cfg.inputContext,
            redactionConfig: this.cfg.redactionConfig,
        });

        try {
            // Debug: ensure parsing_config with transcript/prev policies are present
            const so = (payload as any)?.structured_output_config;
            if (so?.parsing_config) {
                console.log(
                    "🔌 [SDK] sending structured_output_config.parsing_config:",
                    so.parsing_config
                );
            } else {
                console.warn(
                    "🔌 [SDK] structured_output_config.parsing_config missing in initial payload"
                );
            }
        } catch {}

        this.ws.send(JSON.stringify(payload));
    }

    private onWsOpen() {
        this.setStatus("connected");
        this.sendInitialConfigSafely();
        this.setStatus("connected");
    }

    private emitErr(e: Error) {
        console.error("🔌 [SDK] Emitting error:", e.message);
        this.setStatus("error");
        this.onErr?.(e);
    }

    private onWsMsg(ev: MessageEvent) {
        let msg: any;
        try {
            msg = convertKeysToCamelCase(JSON.parse(ev.data));
        } catch {
            return this.emitErr(new Error("Bad json from server"));
        }

        switch (msg.type) {
            case "ack":
                this.onAck?.(msg.wsSessionId ?? msg.sessionId);
                break;
            case "transcript":
                {
                    const turns: Turn[] | undefined =
                        msg.turns ??
                        groupTurnsFromPhrases(msg.phrasesDisplay, msg.final) ??
                        groupTurns(msg.words, msg.final);
                    if (msg.final) {
                        this.onFinal?.(
                            msg.text,
                            msg.words,
                            msg.confidence,
                            turns,
                            msg.phrasesDisplay
                        );
                    } else {
                        this.onInterim?.(
                            msg.text,
                            msg.words,
                            msg.confidence,
                            msg.stability,
                            turns
                        );
                    }
                    break;
                }
                break;
            case "functions":
                this.onFuncs?.(msg.functions);
                break;
            case "structured_output":
                this.onStructuredOutput?.({
                    rev: msg.rev,
                    delta: msg.delta ?? {},
                    final: msg.final ?? {},
                });
                break;
            case "function_draft_extracted":
                this.onDraft?.(msg.draftFunction);
                break;
            case "config_update_ack":
                this.currentRev = msg.rev ?? this.currentRev;
                this.onConfigUpdateAck?.(
                    msg.success,
                    msg.message,
                    msg.rev,
                    msg.activeChecksum
                );
                break;
            case "connection_close":
                if (msg.status === "success" || msg.status === "timeout") {
                    this.setStatus("disconnected");
                    this.destroy();
                } else if (msg.status === "error") {
                    this.emitErr(new Error(msg.message));
                    this.destroy();
                }
                break;
            case "session_end":
                this.onEnd?.();
                break;
            case "error":
                this.emitErr(new Error(msg.err));
                break;
        }
    }

    // Silence detection methods
    private getSilenceCfg() {
        const threshold = this.cfg.silenceDetection?.threshold ?? 0.05; // RMS threshold (0.05 default)
        const checkInterval = this.cfg.silenceDetection?.checkInterval ?? 300; // ms between checks (300 default)
        const keepAliveInterval =
            this.cfg.silenceDetection?.keepAliveInterval ?? 5000; // ms between keep-alive (5000 default)
        const minSilenceMs = this.cfg.silenceDetection?.minSilenceMs ?? 2000; // ms below threshold before entering silence (2000 default)
        const minVoiceMs = this.cfg.silenceDetection?.minVoiceMs ?? 800; // ms above exit threshold before exiting silence (800 default)
        const exitHysteresisDb =
            this.cfg.silenceDetection?.exitHysteresisDb ?? 6; // dB above threshold to exit silence (6 default)
        return {
            threshold,
            checkInterval,
            keepAliveInterval,
            minSilenceMs,
            minVoiceMs,
            exitHysteresisDb,
        };
    }

    // Build a mixed stream from mic and (optionally) system playback
    private async buildMixedStream(
        micStream: MediaStream
    ): Promise<MediaStream> {
        const wantMix = this.cfg.audioMixing?.mixDevicePlayback ?? true;
        const micGainValue = this.cfg.audioMixing?.micGain ?? 1.0;
        const deviceGainValue = this.cfg.audioMixing?.deviceGain ?? 1.0;

        // If mixing disabled, just passthrough mic
        if (!wantMix) {
            return micStream;
        }

        // Try to capture system/device audio using getDisplayMedia
        let systemStream: MediaStream | null = null;
        try {
            // Prefer audio-only capture; if not supported, fall back to including video
            if (navigator.mediaDevices.getDisplayMedia) {
                try {
                    systemStream = await navigator.mediaDevices.getDisplayMedia(
                        { audio: true, video: false } as MediaStreamConstraints
                    );
                } catch {
                    // Some browsers require video track to allow system audio capture
                    systemStream = await navigator.mediaDevices.getDisplayMedia(
                        { audio: true, video: true } as MediaStreamConstraints
                    );
                }
            }
        } catch {
            systemStream = null;
        }

        this.systemAudioStream = systemStream;

        // If no system stream, just return mic
        if (!systemStream) {
            return micStream;
        }

        // Create (or reuse) AudioContext
        if (!this.audioContext) {
            // In browsers only
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            this.audioContext = new (window.AudioContext ||
                (window as any).webkitAudioContext)();
        }

        const context = this.audioContext;

        // Create source nodes
        const micSource = context.createMediaStreamSource(micStream);
        const sysSource = context.createMediaStreamSource(systemStream);

        // Gains for level control
        const micGainNode = context.createGain();
        micGainNode.gain.value = micGainValue;
        const sysGainNode = context.createGain();
        sysGainNode.gain.value = deviceGainValue;

        // Destination node that exposes a MediaStream
        const destination = context.createMediaStreamDestination();

        // Connect graph: mic -> gain -> dest, system -> gain -> dest
        micSource.connect(micGainNode).connect(destination);
        sysSource.connect(sysGainNode).connect(destination);

        return destination.stream;
    }

    private setupSilenceDetection(stream: MediaStream): void {
        try {
            // In browsers only
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            if (typeof window === "undefined") return;

            this.audioContext = new (window.AudioContext ||
                (window as any).webkitAudioContext)();

            this.analyser = this.audioContext.createAnalyser();

            this.microphone = this.audioContext.createMediaStreamSource(stream);

            this.analyser.fftSize = 2048;
            this.analyser.smoothingTimeConstant = 0.8;
            this.microphone.connect(this.analyser);

            // Ensure context is running so analyser receives data
            if (this.audioContext.state === "suspended") {
                this.audioContext.resume().catch((err) => {
                    console.error(
                        "🔇 [SDK] Failed to resume AudioContext:",
                        err
                    );
                });
            }

            this.startSilenceMonitoring();
        } catch (error) {
            console.warn("🔇 [SDK] Could not setup silence detection:", error);
            // TODO: Notify server, server -> fallback to gracefully closing STT
            console.error("🔇 [SDK] Setup error details:", error);
        }
    }

    private startSilenceMonitoring(): void {
        if (!this.analyser) return;
        const { threshold, checkInterval, minSilenceMs, minVoiceMs } =
            this.getSilenceCfg();

        const bufferLength = this.analyser.fftSize;
        const dataArray = new Uint8Array(bufferLength);

        // Only stop the timer, don't clear the analyser
        if (this.silenceCheckTimer) {
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            window.clearInterval(this.silenceCheckTimer);
            this.silenceCheckTimer = null;
        }

        // Calculate RMS of audio samples (normalized to [-1, 1])
        const calculateRMS = (data: Uint8Array) => {
            let sum = 0;
            for (let i = 0; i < data.length; i++) {
                const normalized = data[i] / 128 - 1;
                sum += normalized * normalized;
            }
            return Math.sqrt(sum / data.length);
        };

        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-ignore
        this.silenceCheckTimer = window.setInterval(() => {
            if (!this.analyser || !this.isRecording) {
                return;
            }

            // Use byte time-domain data and normalize
            this.analyser.getByteTimeDomainData(dataArray);
            const rms = calculateRMS(dataArray);

            const previouslySilent = this.isSilent;
            const now = Date.now();

            if (rms < threshold) {
                if (this.lastBelowThresholdAtMs == null) {
                    this.lastBelowThresholdAtMs = now;
                }
            } else {
                if (this.lastBelowThresholdAtMs != null) {
                    this.lastBelowThresholdAtMs = null;
                }
            }

            if (!previouslySilent) {
                if (
                    this.lastBelowThresholdAtMs != null &&
                    now - this.lastBelowThresholdAtMs >= minSilenceMs
                ) {
                    this.isSilent = true;
                    this.silenceSinceMs = now;
                    this.sendSilenceStatus(true);
                    this.startKeepAlive();
                }
            } else {
                if (
                    this.lastBelowThresholdAtMs == null &&
                    now - (this.silenceSinceMs ?? now) >= minVoiceMs
                ) {
                    this.isSilent = false;
                    this.sendSilenceStatus(false);
                    this.stopKeepAlive();
                    this.silenceSinceMs = null;
                }
            }
        }, checkInterval);
    }

    private startKeepAlive(): void {
        const { keepAliveInterval } = this.getSilenceCfg();
        if (this.keepAliveTimer) return;

        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-ignore
        this.keepAliveTimer = window.setInterval(() => {
            if (
                !this.ws ||
                this.ws.readyState !== WebSocket.OPEN ||
                !this.isRecording
            ) {
                this.stopKeepAlive();
                return;
            }
            this.sendSilenceStatus(true);
        }, keepAliveInterval);
    }

    private stopKeepAlive(): void {
        if (this.keepAliveTimer) {
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            window.clearInterval(this.keepAliveTimer);
            this.keepAliveTimer = null;
        }
    }

    private stopSilenceMonitoring(): void {
        this.stopKeepAlive();
        if (this.isSilent) {
            this.sendSilenceStatus(false);
        }
        if (this.silenceCheckTimer) {
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            window.clearInterval(this.silenceCheckTimer);
            this.silenceCheckTimer = null;
        }
        if (this.audioContext) {
            try {
                this.audioContext.close();
            } catch {}
            this.audioContext = null;
        }
        this.analyser = null;
        this.microphone = null;
        this.isSilent = false;
    }

    private formatDurationMs(ms: number): string | undefined {
        if (ms <= 0) return undefined;
        if (ms < 1000) return `${Math.round(ms)}ms`;
        const seconds = (ms / 1000).toFixed(3);
        const trimmed = seconds.replace(/\.0{1,3}$/u, "");
        return `${trimmed}s`;
    }

    private sendSilenceStatus(inSilence: boolean): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        const since = this.silenceSinceMs ?? Date.now();
        const durationStr = this.formatDurationMs(Date.now() - since);
        const payload: Record<string, any> = {
            type: "silence_status",
            in_silence: inSilence,
        };
        if (durationStr) payload.duration = durationStr;
        this.ws.send(JSON.stringify(payload));
    }

    // -------- Batch REST helpers --------
    private deriveHttpBaseUrl(): string {
        const cfgBase = this.cfg.batchApi?.baseUrl;
        if (cfgBase) return cfgBase.replace(/\/$/, "");
        try {
            const u = new URL(this.url);
            const protocol =
                u.protocol === "wss:"
                    ? "https:"
                    : u.protocol === "ws:"
                      ? "http:"
                      : u.protocol;
            return `${protocol}//${u.host}`;
        } catch {
            if (typeof window !== "undefined" && window.location?.origin) {
                return window.location.origin;
            }
            return "";
        }
    }

    private async buildBatchHeaders(): Promise<Record<string, string>> {
        const headers: Record<string, string> = {};
        if (this.cfg.batchApi?.getAuthHeaders) {
            const custom = await this.cfg.batchApi.getAuthHeaders();
            Object.assign(headers, custom);
        } else if (this.cfg.batchApi?.apiKey) {
            const headerName = this.cfg.batchApi.headerName || "Authorization";
            if (headerName.toLowerCase() === "authorization") {
                headers[headerName] = `Bearer ${this.cfg.batchApi.apiKey}`;
            } else {
                headers[headerName] = this.cfg.batchApi.apiKey;
            }
        }
        return headers;
    }

    private getBatchPaths() {
        return {
            upload: this.cfg.batchApi?.uploadPath || "/action/schma-batch",
            status:
                this.cfg.batchApi?.statusPath || "/action/schma-batch-status",
            jobs: this.cfg.batchApi?.jobsPath || "/action/schma-batch-jobs",
        };
    }

    public async verifyBatchProxy(): Promise<void> {
        const base = this.deriveHttpBaseUrl();
        const { status, jobs } = this.getBatchPaths();
        const headers = await this.buildBatchHeaders();
        // Verify status route
        const statusUrl = `${base}/${status.replace(/^\//, "")}`;
        const res1 = await fetch(statusUrl, { method: "GET", headers }).catch(
            () => null
        );
        if (!res1 || (res1.status !== 200 && res1.status !== 204)) {
            throw new Error(
                `Batch status route not available at ${status}. Ensure a Remix loader exists (see app/routes/action.schma-batch-status.tsx) and is registered in app/routes.ts.`
            );
        }
        // Verify jobs route
        const jobsUrl = `${base}/${jobs.replace(/^\//, "")}`;
        const res2 = await fetch(jobsUrl, { method: "GET", headers }).catch(
            () => null
        );
        if (!res2 || (res2.status !== 200 && res2.status !== 204)) {
            throw new Error(
                `Batch jobs route not available at ${jobs}. Ensure a Remix loader exists (see app/routes/action.schma-batch-jobs.tsx) and is registered in app/routes.ts.`
            );
        }
    }

    public async uploadBatch(args: {
        files: File | Blob | Array<File | Blob>;
        fields?: Record<string, string | number | boolean>;
        endpoint?: string; // default: /action/schma-batch
    }): Promise<any> {
        const base = this.deriveHttpBaseUrl();
        const defaults = this.getBatchPaths();
        const endpoint = (args.endpoint || defaults.upload).replace(/^\//, "");
        const url = `${base}/${endpoint}`;

        const form = new FormData();
        const appendFile = (f: File | Blob) => form.append("file", f);

        if (Array.isArray(args.files)) {
            args.files.forEach((f) => appendFile(f));
        } else {
            appendFile(args.files);
        }
        console.log("fields in uploadBatch", args.fields);

        if (args.fields) {
            Object.entries(args.fields).forEach(([k, v]) => {
                const val = JSON.stringify(v);
                form.append(k, val);
                console.log(`Appending to form: ${k} = ${val}`);
            });
        }
        console.log("form after appending", form);
        // Debug: show FormData entries
        for (const [key, value] of form.entries()) {
            console.log(`FormData entry: ${key} = ${value}`);
        }

        const headers = await this.buildBatchHeaders();
        const res = await fetch(url, {
            method: "POST",
            headers,
            body: form,
        });
        if (!res.ok) {
            if (res.status === 404) {
                throw new Error(
                    `Batch upload route not found at /${endpoint}. Ensure action route app/routes/action.schma-batch.tsx exists and is registered in app/routes.ts.`
                );
            }
            const text = await res.text().catch(() => "");
            throw new Error(`Batch upload failed (${res.status}): ${text}`);
        }
        const data = await res.json().catch(() => ({}));
        return data;
    }

    public async getBatchStatus(
        params: Record<string, string | number> = {},
        endpoint?: string
    ): Promise<any> {
        const base = this.deriveHttpBaseUrl();
        const defaults = this.getBatchPaths();
        const ep = (endpoint || defaults.status).replace(/^\//, "");
        const qs = new URLSearchParams();
        Object.entries(params).forEach(([k, v]) => qs.set(k, String(v)));
        const url = `${base}/${ep}${qs.toString() ? `?${qs.toString()}` : ""}`;

        const headers = await this.buildBatchHeaders();
        const res = await fetch(url, {
            method: "GET",
            headers,
        });
        if (!res.ok) {
            if (res.status === 404) {
                throw new Error(
                    `Batch status route not found at /${ep}. Ensure loader route app/routes/action.schma-batch-status.tsx exists and is registered in app/routes.ts.`
                );
            }
            const text = await res.text().catch(() => "");
            throw new Error(`Batch status failed (${res.status}): ${text}`);
        }
        const data = await res.json().catch(() => ({}));
        return data;
    }

    public async listBatchJobs(
        params: Record<string, string | number> = {},
        endpoint?: string
    ): Promise<any> {
        const base = this.deriveHttpBaseUrl();
        const defaults = this.getBatchPaths();
        const ep = (endpoint || defaults.jobs).replace(/^\//, "");
        const qs = new URLSearchParams();
        Object.entries(params).forEach(([k, v]) => qs.set(k, String(v)));
        const url = `${base}/${ep}${qs.toString() ? `?${qs.toString()}` : ""}`;

        const headers = await this.buildBatchHeaders();
        const res = await fetch(url, {
            method: "GET",
            headers,
        });
        if (!res.ok) {
            if (res.status === 404) {
                throw new Error(
                    `Batch jobs route not found at /${ep}. Ensure loader route app/routes/action.schma-batch-jobs.tsx exists and is registered in app/routes.ts.`
                );
            }
            const text = await res.text().catch(() => "");
            throw new Error(`Batch list failed (${res.status}): ${text}`);
        }
        const data = await res.json().catch(() => ({}));
        return data;
    }
    // -------- End Batch REST helpers --------

    // Mic streaming (lazy-start)

    public startRecording() {
        if (this.isRecording) return;

        if (!navigator.mediaDevices) {
            throw new Error("MediaDevices not supported");
        }

        navigator.mediaDevices
            .getUserMedia({ audio: true })
            // Webclient - (stream) -> Server socket
            .then(async (micStream) => {
                const mimeType = MediaRecorder.isTypeSupported(
                    "audio/webm;codecs=opus"
                )
                    ? "audio/webm;codecs=opus"
                    : undefined;

                this.micStream = micStream;

                // Build mixed stream based on config
                const finalStream = await this.buildMixedStream(
                    micStream
                ).catch(() => micStream);
                this.mediaStreamMixed = finalStream;

                // Setup silence detection on the mixed stream
                this.setupSilenceDetection(finalStream);

                this.mediaRecorder = new MediaRecorder(
                    finalStream,
                    mimeType ? { mimeType } : undefined
                );
                this.isRecording = true; // mark recording flag to start recording recognition
                this.firstChunkSent = false; // reset flag at the start of a recording session
                this.mediaRecorder.ondataavailable = (e: BlobEvent) => {
                    if (!this.ws || this.ws.readyState !== WebSocket.OPEN)
                        return;

                    if (!this.isRecording) return;

                    if (!this.firstChunkSent) {
                        this.ws.send(JSON.stringify({ type: "audio_start" }));
                        this.firstChunkSent = true;
                    }

                    this.ws.send(e.data);
                };
                // startup
                this.mediaRecorder.start(this.chunkMs);
            })
            // error logging during streaming.
            .catch((err) => {
                this.emitErr(err);
            });
    }

    public stopRecording() {
        if (!this.isRecording) return;
        this.isRecording = false;

        this.ws?.send(JSON.stringify({ type: "audio_stop" }));

        // Stop silence monitoring resources
        this.stopSilenceMonitoring();

        if (this.mediaRecorder) {
            this.mediaRecorder.stop();
            this.mediaRecorder = null;
        }
        if (this.mediaStreamMixed) {
            this.mediaStreamMixed.getTracks().forEach((t) => t.stop());
            this.mediaStreamMixed = null;
        }
        if (this.micStream) {
            this.micStream.getTracks().forEach((t) => t.stop());
            this.micStream = null;
        }
        if (this.systemAudioStream) {
            this.systemAudioStream.getTracks().forEach((t) => t.stop());
            this.systemAudioStream = null;
        }

        this.firstChunkSent = false;
    }

    public destroy() {
        this.disconnect();
        this.stopSilenceMonitoring();
        this.ws = null;
        this.wsSessionToken = null;
        this.currentRev = undefined;
        this.onFinal = undefined;
        this.onInterim = undefined;
        this.onFuncs = undefined;
    }

    // Dynamic config updates
    public updateFunctionConfig(
        functionConfig: FunctionConfig,
        preserveContext: boolean = false
    ): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            // console.warn(
            //     "🔄 [SDK] Cannot update config: WebSocket not connected"
            // );
            return;
        }

        // TODO: make sure this doesn't send first time round, only if we have sent the first initial function config.

        const payload = convertKeysToSnakeCase({
            type: "dynamic_config_update",
            functionConfig: functionConfig,
            preserveExisting: preserveContext,
        });

        this.ws.send(JSON.stringify(payload));
    }

    public updateStructuredOutputConfig(
        structuredOutputConfig: StructuredOutputConfig,
        preserveContext: boolean = false
    ): void {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            console.warn(
                "🔄 [SDK] Cannot update config: WebSocket not connected"
            );
            return;
        }

        const payload = convertKeysToSnakeCase({
            type: "dynamic_structured_update",
            structuredOutputConfig,
            preserveExisting: preserveContext,
        });

        console.log(
            "🔌 [SDK] Sending dynamic structured config update:",
            payload
        );

        this.ws.send(JSON.stringify(payload));
    }

    getConnectionStatus() {
        return this.status;
    }
}

// TODO: Address bugs with the draft manager... add another utility for managing config changes within an audio session.

/**
 * Utility
 *
 * Manages a list of function drafts based on new incoming drafts and confirmed function calls by the LLM.
 *
 */
/* -------------------------------------------
   FunctionDraftManager (replacement)
-------------------------------------------- */
export class FunctionDraftManager {
    private drafts: FunctionDraftDataReceived[] = [];

    /* ---------- add / update draft ---------- */
    public newDraft(
        data: Omit<FunctionDraftDataReceived, "status">
    ): FunctionDraftDataReceived[] {
        const idx = this.drafts.findIndex(
            (d) => d.draftId === data.draftId || d.name === data.name
        );

        // 1️⃣ already confirmed? --> set status to awaiting_potential_update
        if (idx !== -1 && this.drafts[idx].status === "confirmed_by_llm") {
            const incoming: FunctionDraftDataReceived = {
                ...data,
                status: "awaiting_potential_update",
            };
            this.drafts[idx] = incoming;
            this.sortAndClean();
            return [...this.drafts];
        }

        const incoming: FunctionDraftDataReceived = {
            ...data,
            status: "pending_confirmation",
        };

        if (idx === -1) {
            this.drafts.push(incoming);
        } else if (
            incoming.similarityScore >= this.drafts[idx].similarityScore
        ) {
            // supersede weaker draft of the *same* name
            this.drafts[idx] = incoming;
        }

        this.sortAndClean();
        return [...this.drafts];
    }

    /* ---------- confirm / reconcile ---------- */
    public reconcileWithConfirmed(
        confirmed: FunctionCallReceived[]
    ): FunctionDraftDataReceived[] {
        const byName = new Map(confirmed.map((c) => [c.name, c]));

        // mark existing drafts
        this.drafts.forEach((d) => {
            const fn = byName.get(d.name);
            if (fn) {
                d.status = "confirmed_by_llm";
                d.args = fn.args;
                byName.delete(d.name);
            }
        });

        // 2️⃣ any brand-new confirmations create implicit drafts
        byName.forEach((fn) => {
            this.drafts.push({
                draftId: crypto.randomUUID(),
                name: fn.name,
                args: fn.args,
                similarityScore: 1,
                status: "confirmed_by_llm",
                timestamp: new Date().toISOString(),
            });
        });

        this.sortAndClean();
        return [...this.drafts];
    }

    /* ---------- housekeeping ---------- */
    private sortAndClean() {
        // keep one entry per draftId & highest score first
        this.drafts = Array.from(
            new Map(this.drafts.map((d) => [d.draftId, d])).values()
        ).sort((a, b) => b.similarityScore - a.similarityScore);
    }

    public clearDrafts(
        filter?: (d: FunctionDraftDataReceived) => boolean
    ): FunctionDraftDataReceived[] {
        this.drafts = filter ? this.drafts.filter((d) => !filter(d)) : [];
        return [...this.drafts];
    }

    public getDrafts() {
        return [...this.drafts];
    }
}
