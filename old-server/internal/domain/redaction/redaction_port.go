package redactor


type Kind string

const (
	KindPHI Kind = "phi"
	KindPII Kind = "pii"
	KindPCI Kind = "pci"
)

type Span struct {
	Start, End int // (start, end) in NORMALIZED coords
	Kind Kind
	Confidence float32
	RuleID string
}

type Placeholder struct {
	Kind Kind
	Ordinal int // per-session counter
	Text string // formatted token, e.g., "[PHI:NAME#3]"
}

type RedactionResult struct {
	Spans []Span // normalized spans
	Placeholders []Placeholder // created placeholders
	RedactedRaw string // final rewritten RAW string
}

type EffectivePolicy struct {
	RedactPHI bool
	RedactPIIStrict bool
	RedactPCI bool
	StorePHI bool
	StoreRawTranscripts bool
	Locale string
	PriorityOrder []Kind // PCI > PHI > PII
}

type Vault interface {
	Put(kind Kind, rawValue string) (ph Placeholder, err error) // creates placeholder & stores encrypted raw
	Get (ph Placeholder) (rawValue string, ok bool)
	Clear() // on session end, clear in-memory vault
}

type Redactor interface {
	RedactTranscript(text string) (spans []Span, err error)
	RedactFunctionArgs(args map[string]interface{}) (spans []Span, err error)
	RedactStructuredOutput(output map[string]interface{}) (spans []Span, err error)
}

type RedactionOrchestrator interface {
	// Merge spans from multiple redactors (PCI > PHI > PII), then apply rewrite to text.
	// Since normalization happens pre-redaction, spans are already in the correct coordinate system.
	Apply(text string, spanSets ...[]Span) (RedactionResult, error)
}

type PolicyResolver interface {
	Resolve(sessionID string) (EffectivePolicy, error)
}

type PersistenceGuard interface {
	AllowStorePHI() bool
	AllowStoreRawTranscripts() bool

	// validates and strips sensitve fields before persitence per EffectivePolicy
	FilterBeforeSave(kind Kind, data any) (any, error)
}

