package prompts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"schma.ai/internal/domain/speech"
)

func BuildFunctionsSystemInstructionPrompt(parsingGuide string) string {
	// (1) Base instructions
	base := `You are an intelligent function-extraction engine.
You will receive **only** transcript snippets as user messages, a SpellingCorrections map to help guide output, 
and any previously extracted function calls that you must consider.
Your job is to return **only** a JSON array of function calls
that match the **pre-registered** function calling schema
(you know the tool names, parameters and types).

## CRITICAL RULES

1. **Only extract when explicitly stated**  
   - Never guess or fill missing values.  
   - Skip vague or incomplete info.

2. **Confidence ≥ 95 %** - if unsure, extract nothing.

3. **Explicit value checks**  
   • Names: must be said verbatim
   • Proper Nouns: may be spelled out within the transcript and MUST be adhered to intelligently
   • Dates: concrete ("tomorrow", "March 15" …)  
   • Times: concrete ("7 PM", "at noon" …)  
   • Numbers & selections: clearly stated or match valid options.

4. **Temporal conversions** (relative → absolute)  
   - "tonight" ⇒ today's date  
   - "tomorrow" ⇒ tomorrow's date
   - "in 2 hours" ⇒ now + 2h  
   - Apply only when relevant to a function parameter.

5. **Validation**  
   • Dates: YYYY-MM-DD | Times: HH:MM (24 h)  
   • Emails & phone numbers: valid format  
   • Selection fields: exact match to schema enum.

6. **Never output anything except valid function-call JSON.**

Remember: *better to output nothing than something wrong.*
`

    // (2) Server-provided context (time, date, etc.)
    now := time.Now()
    timeCtx := fmt.Sprintf(`
Current Context (server-provided):
• Date: %s
• Time: %s
• Weekday: %s
• Month: %s
• Year: %s
• ISO Timestamp: %s
`,
        now.Format("2006-01-02"),
        now.Format("15:04:05"),
        now.Format("Monday"),
        now.Format("January"),
        now.Format("2006"),
        now.UTC().Format(time.RFC3339),
    )

    // (3) Spelling behavior
	spelling := `
Spelling Corrections:
- A SpellingCorrections map is provided on each call.
- If transcript contains a spelled-out or corrected name,
  override previous values and use that exact spelling.
`

    // (4) Guide from client (if any)
	guideBlock := ""
	if parsingGuide != "" {
		guideBlock = fmt.Sprintf("\nClient's parsing guide:\n%s\n", parsingGuide)
	}

    return strings.TrimSpace(base + timeCtx + spelling + guideBlock)
}

func BuildStructuredSystemInstructionPrompt(parsingGuide string) string {
	// (1) Base instructions
	base := `You are an intelligent structured-data extraction engine.
You will receive transcript snippets as user messages, a SpellingCorrections map to help guide output, 
and any previously extracted structured data that you must consider.
Your job is to return **only** a JSON object that matches the **pre-registered** JSON Schema
(you know the required fields, types, and validation rules).

## CRITICAL RULES

1. **Confidence ≥ 95 %** - if unsure, leave field undefined.

2. **Explicit value checks**  
   • Names: must be said verbatim
   • Proper Nouns: may be spelled out within the transcript and MUST be adhered to intelligently
   • Dates: concrete ("tomorrow", "March 15" …)  
   • Times: concrete ("7 PM", "at noon" …)  
   • Numbers & selections: clearly stated or match valid options.

3. **Temporal conversions** (relative → absolute)  
   - "tonight" ⇒ today's date  
   - "tomorrow" ⇒ tomorrow's date
   - "in 2 hours" ⇒ now + 2h  
   - Apply only when relevant to a schema field.

4. **Validation**  
   • Dates: YYYY-MM-DD | Times: HH:MM (24 h)  
   • Emails & phone numbers: valid format  
   • Selection fields: exact match to schema enum.
   • Numbers: respect min/max constraints
   • Strings: respect length limits

5. **Schema compliance**
   • Follow the exact JSON Schema structure
   • Respect required vs optional fields
   • Use correct data types (string, number, boolean, array, object)
   • Handle nested objects and arrays properly

6. **Never output anything except valid JSON matching the schema.**

7. **Redaction handling**
   • The transcript may contain redacted placeholders like [TYPE#N] (e.g., [PATIENT#1], [DATE#2]).
   • ALWAYS reuse the exact placeholder value when it appears (do not invent or expand it).
   • Preserve existing placeholders across updates unless the transcript explicitly changes a field.

Remember: *better to output incomplete data than incorrect data.*
`

    // (2) Server-provided context (time, date, etc.)
    now := time.Now()
    timeCtx := fmt.Sprintf(`
Current Context (server-provided):
• Date: %s
• Time: %s
• Weekday: %s
• Month: %s
• Year: %s
• ISO Timestamp: %s
`,
        now.Format("2006-01-02"),
        now.Format("15:04:05"),
        now.Format("Monday"),
        now.Format("January"),
        now.Format("2006"),
        now.UTC().Format(time.RFC3339),
    )

    // (3) Spelling behavior
	spelling := `
Spelling Corrections:
- A SpellingCorrections map is provided on each call.
- If transcript contains a spelled-out or corrected name,
  override previous values and use that exact spelling.
`

    // (4) Guide from client (if any)
	guideBlock := ""
	if parsingGuide != "" {
		guideBlock = fmt.Sprintf("\nClient's parsing guide:\n%s\n", parsingGuide)
	}

    return strings.TrimSpace(base + timeCtx + spelling + guideBlock)
}

// TODO: just dynamic prompt builders to handle diarization. If we have diarization, group by speaker (use turns util).

func BuildFunctionParsingPrompt(
    latestTranscript string,
    spellingCache map[string]string,
    prevCalls []speech.FunctionCall,
) string {
	// 1) Turn spellingCache into an inline JSON snippet
	var sc strings.Builder
	if len(spellingCache) > 0 {
		sc.WriteString("SpellingCorrections:\n{\n")
		i := 0
		for key, val := range spellingCache {
			// keys should be lowercase lookup-terms
			sc.WriteString(fmt.Sprintf(`  "%s": "%s"`, key, val))
			if i < len(spellingCache)-1 {
				sc.WriteString(",\n")
			} else {
				sc.WriteString("\n")
			}
			i++
		}
		sc.WriteString("}\n\n")
	}

    // 2) Previous calls (accumulated during the session)
    var pc strings.Builder
    if len(prevCalls) > 0 {
        b, _ := json.MarshalIndent(prevCalls, "", "  ")
        pc.WriteString("PreviousCalls:\n")
        pc.Write(b)
        pc.WriteString("\n\n")
    }

    // 3) The core user prompt
    return fmt.Sprintf(`%s%sTranscript:
%s

Return **ONLY** a JSON array of function calls that exactly match the registered schema.
• Omit any function if you don't have all required arguments.
• Never invent values.
• When updating, reuse the existing "id" field.
`, sc.String(), pc.String(), latestTranscript)
}


func BuildFunctionParsingPromptWithRedaction(textRedacted string, spellingCache map[string]string, prevCallsRedacted []speech.FunctionCall) string {
    basePrompt := BuildFunctionParsingPrompt(textRedacted, spellingCache, prevCallsRedacted)

    return fmt.Sprintf(`
    %s
    CRITICAL REDACTION INSTRUCTIONS:
    The transcript contains redacted PHI values marked as [TYPE#N] (e.g., [PATIENT#1], [DATE#2], [ID#3], [EMAIL#4], [PHONE#5]).
    
    When extracting function arguments:
    1. **ALWAYS reuse the exact placeholder format [TYPE#N] when the corresponding information appears in the transcript**
    2. **NEVER generate "unknown", "N/A", or similar placeholder values**
    3. **NEVER attempt to reconstruct or guess the original values**
    4. **Maintain the exact placeholder structure for later reconstruction**
    5. **If you see [PATIENT#1] in the transcript, use [PATIENT#1] in your function arguments**
    6. **If you see [DATE#2] in the transcript, use [DATE#2] in your function arguments**
    7. **CRITICAL: When updating existing function calls, PRESERVE all existing placeholder values from previous calls**
    8. **CRITICAL: Only update fields that are explicitly mentioned in the new transcript**
    9. **CRITICAL: If a field was set to [PATIENT#1] in a previous call, keep it as [PATIENT#1] unless explicitly updated**
    10. **CRITICAL: If previous function calls contain placeholders like [PATIENT#1], [DATE#2], etc., you MUST reuse those exact placeholders**
    11. **CRITICAL: Do NOT replace existing placeholders with "unknown" - this is WRONG**
    12. **CRITICAL: The placeholders [PATIENT#1], [DATE#2], etc. are the CORRECT values - do not change them**
    
    Examples:
    - If transcript says "customer [PATIENT#1] has contact [EMAIL#2]", use {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]"}
    - If transcript says "appointment on [DATE#3]", use {"appointment_date": "[DATE#3]"}
    - If previous call has {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]"} and new transcript only mentions "[PHONE#4]", 
      then update to {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]", "phone": "[PHONE#4]"} (preserve existing placeholders)
    - If previous call has {"customer_name": "[PATIENT#1]"} and new transcript mentions "urgent request", 
      then update to {"customer_name": "[PATIENT#1]", "priority": "urgent request"} (keep [PATIENT#1], add priority)
    
    `, basePrompt)
}

// promptsBuildStructured builds a simple prompt for structured extraction
func BuildStructuredParsingPrompt(latestTranscript string, spelling map[string]string, prev map[string]any) string {
	// 1) Turn spellingCache into an inline JSON snippet
	var sc strings.Builder
	if len(spelling) > 0 {
		sc.WriteString("SpellingCorrections:\n{\n")
		i := 0
		for key, val := range spelling {
			// keys should be lowercase lookup-terms
			sc.WriteString(fmt.Sprintf(`  "%s": "%s"`, key, val))
			if i < len(spelling)-1 {
				sc.WriteString(",\n")
			} else {
				sc.WriteString("\n")
			}
			i++
		}
		sc.WriteString("}\n\n")
	}

    // 2) Previous structured data (accumulated during the session)
    var pc strings.Builder
    if len(prev) > 0 {
        b, _ := json.MarshalIndent(prev, "", "  ")
        pc.WriteString("PreviousStructuredData:\n")
        pc.Write(b)
        pc.WriteString("\n\n")
    }

    // 3) The core user prompt
    return fmt.Sprintf(`%s%sTranscript:
%s

Return **ONLY** a JSON object that exactly matches the registered schema.
• Omit any field if you don't have all required information.
• Never invent values.
• Use null for missing optional fields.
• When updating, preserve existing valid data and only change what's explicitly mentioned.
`, sc.String(), pc.String(), latestTranscript)
}

func BuildStructuredOutputPromptWithRedaction(textRedacted string, spellingCache map[string]string, prevStructuredRedacted map[string]any) string {
    basePrompt := BuildStructuredParsingPrompt(textRedacted, spellingCache, prevStructuredRedacted)

    return fmt.Sprintf(`
    %s
    CRITICAL REDACTION INSTRUCTIONS:
    The transcript contains redacted PHI values marked as [TYPE#N] (e.g., [PATIENT#1], [DATE#2], [ID#3], [EMAIL#4], [PHONE#5]).
    
    When extracting structured output:
    1. **ALWAYS reuse the exact placeholder format [TYPE#N] when the corresponding information appears in the transcript**
    2. **NEVER generate "unknown", "N/A", or similar placeholder values**
    3. **NEVER attempt to reconstruct or guess the original values**
    4. **Maintain the exact placeholder structure for later reconstruction**
    5. **Preserve any existing masked values from previous structured data**
    6. **If you see [PATIENT#1] in the transcript, use [PATIENT#1] in your structured output**
    7. **If you see [DATE#2] in the transcript, use [DATE#2] in your structured output**
    8. **CRITICAL: When updating existing structured data, PRESERVE all existing placeholder values**
    9. **CRITICAL: Only update fields that are explicitly mentioned in the new transcript**
    10. **CRITICAL: If a field was set to [PATIENT#1] in previous data, keep it as [PATIENT#1] unless explicitly updated**
    
    Examples:
    - If transcript says "customer [PATIENT#1] has contact [EMAIL#2]", use {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]"}
    - If transcript says "appointment on [DATE#3]", use {"appointment_date": "[DATE#3]"}
    - If previous data has {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]"} and new transcript only mentions "[PHONE#4]", 
      then update to {"customer_name": "[PATIENT#1]", "email": "[EMAIL#2]", "phone": "[PHONE#4]"} (preserve existing placeholders)
    
    `, basePrompt)
}


// DiarizedParagraph represents a labeled paragraph for a single speaker
type DiarizedParagraph struct {
    Speaker string
    Text    string
}

// BuildDiarizedParagraphs groups phrase lights by speaker (first-seen order)
// and concatenates each speaker's redacted text into a single paragraph.
func BuildDiarizedParagraphs(pls []speech.PhraseLight) []DiarizedParagraph {
    if len(pls) == 0 {
        return nil
    }
    order := make([]string, 0, len(pls))
    seen := map[string]bool{}
    by := map[string][]string{}
    for _, p := range pls {
        spk := p.Speaker
        if spk == "" { spk = "Speaker" }
        if !seen[spk] { seen[spk] = true; order = append(order, spk) }
        t := strings.TrimSpace(p.TextRedacted)
        if t != "" { by[spk] = append(by[spk], t) }
    }
    out := make([]DiarizedParagraph, 0, len(order))
    for _, spk := range order {
        out = append(out, DiarizedParagraph{Speaker: spk, Text: strings.Join(by[spk], " ")})
    }
    return out
}

// RenderDiarizedParagraphs renders paragraphs as:
// Speaker:\n<text>\n
func RenderDiarizedParagraphs(paras []DiarizedParagraph) string {
    if len(paras) == 0 { return "" }
    var b strings.Builder
    for i, p := range paras {
        if i > 0 { b.WriteString("\n") }
        b.WriteString("Speaker: " + p.Speaker)
        b.WriteString(":\n")
        b.WriteString(strings.TrimSpace(p.Text))
        b.WriteString("\n")
    }
    return strings.TrimSpace(b.String())
}

// countTokens returns whitespace-delimited token count (approx)
func countTokens(s string) int { return len(strings.Fields(s)) }

// WindowDiarizedParagraphsTokens returns the last n tokens across the diarized
// paragraphs, preserving the speaker header above the first included text.
// If the cut happens mid-paragraph, only the tail of that paragraph is included
// but its Speaker header is kept.
func WindowDiarizedParagraphsTokens(paras []DiarizedParagraph, n int) string {
    if n <= 0 || len(paras) == 0 { return "" }
    // Walk from end, accumulate tokens until n
    type part struct{ speaker, text string }
    acc := make([]part, 0, len(paras))
    remaining := n
    for i := len(paras)-1; i >= 0 && remaining > 0; i-- {
        p := paras[i]
        toks := strings.Fields(p.Text)
        if len(toks) == 0 { continue }
        if len(toks) <= remaining {
            // take whole paragraph
            acc = append(acc, part{speaker: p.Speaker, text: strings.Join(toks, " ")})
            remaining -= len(toks)
        } else {
            // take tail of paragraph
            tail := strings.Join(toks[len(toks)-remaining:], " ")
            acc = append(acc, part{speaker: p.Speaker, text: tail})
            remaining = 0
        }
    }
    // reverse to restore forward order
    for i, j := 0, len(acc)-1; i < j; i, j = i+1, j-1 { acc[i], acc[j] = acc[j], acc[i] }
    // render
    out := make([]DiarizedParagraph, 0, len(acc))
    for _, a := range acc { out = append(out, DiarizedParagraph{Speaker: a.speaker, Text: a.text}) }
    return RenderDiarizedParagraphs(out)
}

func BuildFunctionParsingPromptBatch(
	transcript string,

	// schemaJSON string,
) string {

	var b strings.Builder

	// 0️⃣  Optionally include the schema so the model sees arguments/types.
	// if schemaJSON != "" {
	// 	b.WriteString("RegisteredFunctions:\n")
	// 	b.WriteString(schemaJSON)
	// 	b.WriteString("\n\n")
	// }

	// 1️⃣  Header makes it obvious we’re dealing with a *final* transcript
	//     (Gemini sometimes tries to “wait for more” if we don’t clarify).
	b.WriteString(fmt.Sprintf("Transcript (length: %d chars):\n", len(transcript)))
	b.WriteString(transcript)
	b.WriteString("\n\n")

	// 2️⃣  The same strict instructions as streaming.
	b.WriteString(`Return **ONLY** a JSON array of function calls that exactly match the registered schema.
• Omit any function if you don't have all required arguments.
• Never invent values.
• When updating, reuse the existing "id" field.
`)

	return b.String()
}


// BuildBatchFunctionPrompt combines system instructions and a final-transcript prompt for batch function parsing
func BuildBatchFunctionPrompt(transcript string, parsingGuide string) string {
	sys := BuildFunctionsSystemInstructionPrompt(parsingGuide)
	body := BuildFunctionParsingPromptBatch(transcript)
	return strings.TrimSpace(sys + "\n\n" + body)
}

// BuildBatchStructuredPrompt combines system instructions and a final-transcript prompt for batch structured output
func BuildBatchStructuredPrompt(transcript string, parsingGuide string) string {
	sys := BuildStructuredSystemInstructionPrompt(parsingGuide)
	// Reuse structured parsing builder with no prior state for batch
	body := BuildStructuredParsingPrompt(transcript, map[string]string{}, map[string]any{})
	return strings.TrimSpace(sys + "\n\n" + body)
}

