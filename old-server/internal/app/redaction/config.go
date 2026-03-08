package redaction


type RedactionMode string

const (
	ModeA_HIPAA RedactionMode = "vertex_hipaa" 
	ModeB_Public RedactionMode = "public"
)

type RedactionPolicyConfig struct {
	Mode RedactionMode // defaults to "public"
	RedactPHI bool // default true in ModeB
	RedactPIIStrict bool // org-level toggle per app
	RedactPCI bool // always true
	StorePHI bool // false (mvp)
	StoreRawTranscripts bool // false (mvp)
	Locale string // "AU" default
}

type ModelPaths struct {
	PHI_ONNX string
	PII_ONNX string

}

type RedactionConfig struct {
	Redaction RedactionPolicyConfig
	Models ModelPaths
}

func (c *RedactionConfig) ShouldRedactBeforeLLM() bool {
	return c.Redaction.Mode == ModeB_Public
}
	