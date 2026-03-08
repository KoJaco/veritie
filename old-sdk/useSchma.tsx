import { useEffect, useRef, useState, useCallback, useMemo } from "react";
import {
    SchmaSDK,
    type ClientConfig,
    type FunctionCallReceived,
    type FunctionConfig,
    type FunctionDraftDataReceived,
    FunctionDraftManager,
    type Word,
    type PhraseDisplay,
    type StructuredOutputConfig,
    type StructuredOutputReceived,
    type Turn,
} from "./index";

type ConnStatus = "disconnected" | "connecting" | "connected" | "error";

interface UseSchmaOpts {
    /** Mandatory server / WS details etc. */
    config: ClientConfig;
    /** Share a manager across hooks – otherwise a fresh one is created */
    manager?: FunctionDraftManager;
}

/**
 * Real-time speech -> Memo functions hook
 *
 */

export function useSchma({ config, manager }: UseSchmaOpts) {
    // stable refs
    const sdkRef = useRef<SchmaSDK | null>(null);
    const mgrRef = useRef<FunctionDraftManager>(
        manager ?? new FunctionDraftManager()
    );

    // React state
    // const [finalTxt, setFinalTxt] = useState<{
    //     text: string;
    //     words?: Word[];
    //     confidence?: number;
    // }>({
    //     text: "",
    //     words: [],
    //     confidence: 0,
    // });
    const [finalTranscriptPieces, setFinalTranscriptPieces] = useState<
        {
            text: string;
            words?: Word[];
            confidence?: number;
            turns?: Turn[];
            phrasesDisplay?: PhraseDisplay[];
        }[]
    >([]);
    const [interim, setInterim] = useState<{
        text: string;
        words?: Word[];
        confidence?: number;
        stability?: number;
    }>({
        text: "",
        words: [],
        confidence: 0,
        stability: 0,
    });
    const [funcs, setFuncs] = useState<FunctionCallReceived[]>([]);
    const [structuredOutput, setStructuredOutput] =
        useState<StructuredOutputReceived | null>(null);
    const [drafts, setDrafts] = useState<FunctionDraftDataReceived[]>([]);
    const [status, setStatus] = useState<ConnStatus>("disconnected");
    const [error, setError] = useState<Error | null>(null);

    // derived value using final transcript pieces
    const finalTxt = useMemo(
        () => ({
            text: finalTranscriptPieces.map((p) => p.text).join(" "),
            words: finalTranscriptPieces.flatMap((p) => p.words ?? []),
            confidence: finalTranscriptPieces.reduce(
                (c, p) => p.confidence ?? c,
                0
            ),
            turns: finalTranscriptPieces.flatMap((p) => p.turns ?? []),
        }),
        [finalTranscriptPieces]
    );

    // SDK init / teardown
    // Intentional: only recreate SDK when connection primitives change.
    // Function/structured configs are hot-swapped via update* APIs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => {
        // console.log("🔌 [useSchma] Initializing SDK with config:", config);
        // Only re-initialize the SDK when connection primitives change
        // Dynamic schema/guide updates should use update* APIs instead of recreating the SDK
        const sdk = new SchmaSDK(config.apiUrl, config);
        sdkRef.current = sdk;

        /* ---------- wire events -------------------------------------- */
        sdk.onFinal = (txt, words, confidence, turns, phrasesDisplay) => {
            // React 18 batches updates? Gone with derived state instead
            setFinalTranscriptPieces((prev) => [
                ...prev,
                { text: txt, words, confidence, turns, phrasesDisplay },
            ]);
            // setFinalTxt((prev) => ({
            //     text: prev.text ? `${prev.text.trimEnd()} ${txt}`.trim() : txt,
            //     words: words ? [...(prev.words ?? []), ...words] : prev.words,
            //     // TODO: set average for google stt hmmm
            //     confidence: confidence ?? prev.confidence,
            // }));
        };

        sdk.onInterim = (txt, words, confidence, stability) =>
            setInterim({ text: txt, words, confidence, stability });
        sdk.onFuncs = (calls) => {
            setFuncs(calls);
            const upd = mgrRef.current.reconcileWithConfirmed(calls);
            setDrafts(upd);
        };
        sdk.onStructuredOutput = (s) => {
            setStructuredOutput(s);
        };
        sdk.onDraft = (d) => {
            const upd = mgrRef.current.newDraft(d);
            setDrafts(upd);
        };
        sdk.onStatus = (status) => {
            // console.log("🔌 [useSchma] Status changed:", status);
            setStatus(status);
        };
        sdk.onErr = (error) => {
            // console.error("🔌 [useSchma] Error received:", error);
            setError(error);
        };
        sdk.onEnd = () => {
            // console.log("🔌 [useSchma] Connection ended");
            setStatus("disconnected");
        };

        return () => {
            sdk.disconnect();
            sdkRef.current = null;
        };
    }, [config, mgrRef]);

    // Manager helpers (memoised)
    const addManualDraft = useCallback(
        (d: Omit<FunctionDraftDataReceived, "status">) => {
            setDrafts(mgrRef.current.newDraft(d));
        },
        [mgrRef]
    );

    const confirmWithLLM = useCallback(
        (calls: FunctionCallReceived[]) => {
            setDrafts(mgrRef.current.reconcileWithConfirmed(calls));
        },
        [mgrRef]
    );

    const clearDrafts = useCallback(() => {
        setDrafts(mgrRef.current.clearDrafts());
    }, [mgrRef]);

    const getDrafts = useCallback(() => mgrRef.current.getDrafts(), [mgrRef]);

    // Public SDK actions
    const connect = async () => {
        // console.log("🔌 [useSchma] Connect called");
        try {
            await sdkRef.current?.connect();
        } catch (error) {
            console.error("🔌 [useSchma] Connection failed:", error);
            setError(
                error instanceof Error ? error : new Error("Connection failed")
            );
        }
    };
    const disconnect = () => sdkRef.current?.disconnect();
    const destroy = () => sdkRef.current?.destroy();
    const startRecording = () => sdkRef.current?.startRecording();
    const stopRecording = () => sdkRef.current?.stopRecording();
    const updateFunctionConfig = (
        functionConfig: FunctionConfig,
        preserveContext: boolean = false
    ) => sdkRef.current?.updateFunctionConfig(functionConfig);
    const updateStructuredOutputConfig = (
        structuredOutputConfig: StructuredOutputConfig,
        preserveContext: boolean = false
    ) => sdkRef.current?.updateStructuredOutputConfig(structuredOutputConfig);

    // Batch helpers passthrough
    const uploadBatch = (args: Parameters<SchmaSDK["uploadBatch"]>[0]) =>
        sdkRef.current?.uploadBatch(args);
    const getBatchStatus = (
        params?: Parameters<SchmaSDK["getBatchStatus"]>[0],
        endpoint?: Parameters<SchmaSDK["getBatchStatus"]>[1]
    ) => sdkRef.current?.getBatchStatus(params ?? {}, endpoint);
    const listBatchJobs = (
        params?: Parameters<SchmaSDK["listBatchJobs"]>[0],
        endpoint?: Parameters<SchmaSDK["listBatchJobs"]>[1]
    ) => sdkRef.current?.listBatchJobs(params ?? {}, endpoint);
    const verifyBatchProxy = () => sdkRef.current?.verifyBatchProxy();

    const clearSession = () => {
        setFinalTranscriptPieces([]);
        setInterim({ text: "", confidence: 0 });
        setFuncs([]);
        setStructuredOutput(null);
        clearDrafts();
        setError(null);
    };

    // Hook return out

    return {
        /* Transcript */
        transcriptFinalPieces: finalTranscriptPieces,
        transcriptFinal: finalTxt,
        transcriptInterim: interim,

        /* Functions & drafts */
        functions: funcs,
        structuredOutput: structuredOutput,
        drafts,
        addManualDraft,
        confirmWithLLM,
        clearDrafts,
        getDrafts,

        /* Connection */
        connectionStatus: status,
        connect,
        disconnect,
        startRecording,
        stopRecording,
        updateFunctionConfig,
        updateStructuredOutputConfig,
        destroy,

        /* Batch */
        uploadBatch,
        getBatchStatus,
        listBatchJobs,
        verifyBatchProxy,

        /* Misc */
        error,
        clearSession,
    };
}
