package ws

import (
	"schma.ai/internal/domain/speech"
)

// TODO: We're not using input context yet, implement
// Client -> server. Config message represents initial configuration sent from the client SDK.
type ConfigMessage struct {
	Type         string         `json:"type"` // "config"
	IsTest       bool           `json:"is_test,omitempty"`
	WSSessionID  string         `json:"ws_session_id,omitempty"`
	Language     string         `json:"language,omitempty"`
	STT          STTConfig      `json:"stt"`
	Functions    *FunctionConfig `json:"function_config,omitempty"`
	Structured   *StructuredOutputConfig `json:"structured_output_config,omitempty"`
	InputContext *InputContext  `json:"input_context,omitempty"`
	Redaction    *RedactionConfig `json:"redaction_config,omitempty"`
}

// TODO: add support for input context. If a user stops audio but would then like to effectively add to the output from the LLM, we need to allow them to provide a previous transcript and either functions or structured output

// TODO: implement prompt injection guard after MVP.
// TODO: this is to be extended to envapsulate provider config, nice and accessibile for devs.
type STTConfig struct {
	Provider    string `json:"provider,omitempty"` // "google" or "deepgram"
	SampleHertz int    `json:"sample_hertz,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Diarization speech.DiarizationConfig `json:"diarization,omitempty"`	// toggle flag for the feature
}

// Function-specific config supplied by the SDK.
type FunctionConfig struct {
	Name     string                      `json:"name,omitempty"`     
	Description string                      `json:"description,omitempty"`    
	Definitions  []speech.FunctionDefinition `json:"definitions,omitempty"`   // list of tools
	ParsingGuide string                      `json:"parsing_guide,omitempty"` // free-text hints
	UpdateMS     int                         `json:"update_ms,omitempty"`     // throttle window
	ParsingConfig speech.ParsingConfig `json:"parsing_config,omitempty"`
}

type StructuredOutputConfig struct {
	Schema speech.StructuredOutputSchema `json:"schema,omitempty"`
	ParsingGuide string      `json:"parsing_guide,omitempty"`
	UpdateMS     int         `json:"update_ms,omitempty"`
	ParsingConfig speech.ParsingConfig `json:"parsing_config,omitempty"`
}

type RedactionConfig struct {
    DisablePHI bool `json:"disable_phi,omitempty"`
}

type InputContext struct {
	CurrentRawTranscript string                `json:"current_raw_transcript,omitempty"`
	CurrentFunctions     []speech.FunctionCall `json:"current_functions,omitempty"`
    CurrentStructured    map[string]any        `json:"current_structured,omitempty"`
}

// Dynamic config update message for hot-swapping function configs
// using pointers so the client can update just one of them at a time.
type DynamicConfigUpdateMessage struct {
	Type           string         `json:"type"` // "dynamic_config_update"
	FunctionConfig *FunctionConfig `json:"function_config,omitempty"`
	StructuredConfig *StructuredOutputConfig `json:"structured_output_config,omitempty"`
	PreserveContext bool `json:"preserve_context,omitempty"`
	Redaction       *RedactionConfig `json:"redaction_config,omitempty"`
}

// received when parsing strategy is set to "manual"
type ManualGenerationMessage struct {
	Type string `json:"parse_content_manually"`
}

// Config update acknowledgment message
type ConfigUpdateAckMessage struct {
	Type    string `json:"type"` // "config_update_ack"
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// Binary frames are raw opus/pcm
// server -> client
// simple ack after config accepted
type AckMessage struct {
	Type      string `json:"type"` // "ack"
	SessionID string `json:"session_id"`
}

// incremental or final transcript
type TranscriptMessage struct {
	Type       string        `json:"type"` // "transcript"
	Text       string        `json:"text"`
	Final      bool          `json:"final"`
	Confidence float32       `json:"confidence"`
	Words      []speech.Word `json:"words,omitempty"` // @deprecated
	// Diarization (optional)
	Turns      []speech.Turn `json:"turns,omitempty"` // @deprecated
	Channel    int           `json:"channel,omitempty"`
	// new management for normalization and redaction purposes
	PhrasesDisplay []speech.Phrase `json:"phrases_display,omitempty"`
}

// returned when the LLM emits or updates function calls
type FunctionMessage struct {
	Type      string                `json:"type"` // "functions"
	Functions []speech.FunctionCall `json:"functions"`
}

type StructuredOutputMessage struct {
	Type string `json:"type"` // "structured_output"
	Rev int64 `json:"rev"`
	Delta map[string]any `json:"delta,omitempty"`
	Final map[string]any `json:"final,omitempty"`
}

type DraftFunctionMessage struct {
	Type  string              `json:"type"`
	Draft speech.FunctionCall `json:"draft_function"`
}

// generic error envelope
type ErrorMessage struct {
	Type string `json:"type"` // "error"
	Err  string `json:"error"`
}

type ConnectionCloseMessage struct {
	Type string `json:"type"` // "connection_close"
	Message string `json:"message,omitempty"`
	Status string `json:"status,omitempty"` // "success", "error", "timeout"
}

// silence status message (client -> server)
type SilenceMessage struct {
	Type      string `json:"type"` // "silence_status"
	InSilence bool   `json:"in_silence"` // true if client detected silence
	Duration  string `json:"duration,omitempty"` // how long in silence
}

// Security event: PCI detected (server -> client)
type PCIDetectedMessage struct {
    Type    string `json:"type"` // "pci_detected"
    Message string `json:"message"`
}

// TODO: Add in new session message types, session_start, session_stop, session_usage *probs just include usage data in session_stop.
