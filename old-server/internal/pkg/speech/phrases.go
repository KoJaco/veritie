package speech

import "schma.ai/internal/domain/speech"


func BuildPhraseFromFinalWords(words []speech.Word, norm, masked string) (speech.Phrase, speech.PhraseLight) {
    var speaker string
    var start, end float32
    if len(words) > 0 {
        speaker = words[0].Speaker
        start = words[0].Start
        end = words[len(words)-1].End
        for i := range words {
            if words[i].Start < start { start = words[i].Start }
            if words[i].End > end { end = words[i].End }
        }
    }
    ph := speech.Phrase{
        Speaker:    speaker,
        Start:      start,
        End:        end,
        TextNorm:   norm,
        TextRedacted: masked,
    }
    pl := speech.PhraseLight{
        Speaker: speaker,
        Start:   start,
        End:     end,
        TextRedacted: masked,
    }
    return ph, pl
}

// BuildPhrasesFromFinalWords splits final words into multiple phrases by speaker boundaries.
// Each phrase contains only a single speaker, ensuring accurate diarization.
// Returns arrays of phrases and phrase lights.
func BuildPhrasesFromFinalWords(words []speech.Word, norm, masked string) ([]speech.Phrase, []speech.PhraseLight) {
    if len(words) == 0 {
        return []speech.Phrase{}, []speech.PhraseLight{}
    }

    var phrases []speech.Phrase
    var phrasesLight []speech.PhraseLight
    
    var currentSpeaker string
    var currentStart, currentEnd float32
    var currentWords []speech.Word
    var haveCurrent bool

    for _, word := range words {
        // If this is the first word or speaker has changed, start a new phrase
        if !haveCurrent || (word.Speaker != "" && currentSpeaker != "" && word.Speaker != currentSpeaker) {
            // Save the previous phrase if we have one
            if haveCurrent && len(currentWords) > 0 {
                ph, pl := buildPhraseFromWords(currentWords, norm, masked)
                phrases = append(phrases, ph)
                phrasesLight = append(phrasesLight, pl)
            }
            
            // Start new phrase
            currentSpeaker = word.Speaker
            currentStart = word.Start
            currentEnd = word.End
            currentWords = []speech.Word{word}
            haveCurrent = true
        } else {
            // Continue current phrase
            if word.Start < currentStart {
                currentStart = word.Start
            }
            if word.End > currentEnd {
                currentEnd = word.End
            }
            currentWords = append(currentWords, word)
        }
    }
    
    // Don't forget the last phrase
    if haveCurrent && len(currentWords) > 0 {
        ph, pl := buildPhraseFromWords(currentWords, norm, masked)
        phrases = append(phrases, ph)
        phrasesLight = append(phrasesLight, pl)
    }
    
    return phrases, phrasesLight
}

// buildPhraseFromWords is a helper function to build a single phrase from a slice of words
func buildPhraseFromWords(words []speech.Word, norm, masked string) (speech.Phrase, speech.PhraseLight) {
    var speaker string
    var start, end float32
    
    if len(words) > 0 {
        speaker = words[0].Speaker
        start = words[0].Start
        end = words[len(words)-1].End
        
        // Find the actual start and end times across all words
        for _, word := range words {
            if word.Start < start {
                start = word.Start
            }
            if word.End > end {
                end = word.End
            }
        }
    }
    
    ph := speech.Phrase{
        Speaker:      speaker,
        Start:        start,
        End:          end,
        TextNorm:     norm,
        TextRedacted: masked,
    }
    
    pl := speech.PhraseLight{
        Speaker:      speaker,
        Start:        start,
        End:          end,
        TextRedacted: masked,
    }
    
    return ph, pl
}


func ToPhraseLight(ph speech.Phrase) speech.PhraseLight {
    return speech.PhraseLight{
        Start:     ph.Start,
        End:       ph.End,
        Speaker:   ph.Speaker,
        TextRedacted: ph.TextRedacted,
    }
}

func ToPhrasesLight(phs []speech.Phrase) []speech.PhraseLight {
    out := make([]speech.PhraseLight, 0, len(phs))
    for _, ph := range phs {
        out = append(out, ToPhraseLight(ph))
    }
    return out
}

// BuildPhrasesBySpeakerGroups consumes final transcript segments and groups them by single-speaker turns.
// It uses the provided diarization turns to segment words and assigns redacted text.
func BuildPhrasesBySpeakerGroups(words []speech.Word, turns []speech.Turn, normByTurn map[string]string, maskedByTurn map[string]string) ([]speech.Phrase, []speech.PhraseLight) {
    // Index words within turn spans (by time). If words carry speaker labels, we also filter by speaker.
    type agg struct{ start, end float32 }
    phOut := make([]speech.Phrase, 0, len(turns))
    plOut := make([]speech.PhraseLight, 0, len(turns))

    for _, t := range turns {
        // gather words that fall inside this turn window and match speaker when present
        var tStart, tEnd float32
        var have bool
        for i := range words {
            w := words[i]
            if w.Start >= t.Start && w.End <= t.End {
                if t.Speaker != "" && w.Speaker != "" && w.Speaker != t.Speaker { continue }
                if !have { tStart, tEnd, have = w.Start, w.End, true } else {
                    if w.Start < tStart { tStart = w.Start }
                    if w.End > tEnd { tEnd = w.End }
                }
            }
        }
        // even if no aligned words, still create a phrase for the turn bounds (optional):
        if !have { tStart, tEnd = t.Start, t.End }

        norm := normByTurn[t.ID]
        masked := maskedByTurn[t.ID]

        ph := speech.Phrase{
            Speaker: t.Speaker,
            Start:   tStart,
            End:     tEnd,
            TextNorm: norm,
            TextRedacted: masked,
        }
        pl := speech.PhraseLight{
            Speaker: t.Speaker,
            Start:   tStart,
            End:     tEnd,
            TextRedacted: masked,
        }
        phOut = append(phOut, ph)
        plOut = append(plOut, pl)
    }
    return phOut, plOut
}