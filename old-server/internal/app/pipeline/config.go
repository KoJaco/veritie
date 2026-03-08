package pipeline

import (
	"schma.ai/internal/app/draft"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/domain/usage"
)

// type Config { Model string; SilenceMS int...}

 

type ConfigFunctions struct {
	DBSessionID string
	AccountID   string
	AppID       string
	Pricing     usage.Pricing

	// Built by prompts.BuildFunctionParsingPrompt
	Prompt speech.Prompt

	// Nil if session does not ask for func-calling
	FuncCfg *speech.FunctionConfig

	InputGuide        string
	PrevFunctionsJSON string

	// Getting back draft functions
	DraftIndex *draft.Index

	// Redaction controls
	DisablePHI bool
}


type ConfigStructured struct {
	DBSessionID string
	AccountID   string
	AppID       string
	Pricing     usage.Pricing

	// Built by prompts.BuildFunctionParsingPrompt
	Prompt speech.Prompt

	// Nil if session does not ask for func-calling
	StructuredCfg *speech.StructuredOutputConfig

	InputGuide        string
	PrevStructuredJSON string

	// Redaction controls
	DisablePHI bool
}