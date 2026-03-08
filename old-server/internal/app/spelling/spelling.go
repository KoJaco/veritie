package spelling

import (
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// var spellRe = regexp.MustCompile(`(?i)(?:sp(?:e|a)ll(?:ed)?|spelt)\s+(([a-z]\s+){2,}[a-z])`)

// var spellRe = regexp.MustCompile(
// 	`(?i)\b(?:spell(?:ed)?|spelt)\s+([a-z](?:\s+[a-z]){2,})\b`,
// )

var spellRe = regexp.MustCompile(
	`(?i)\b(?:spell(?:ed)?|spelt)\s+((?:[a-z]|space)(?:\s+(?:[a-z]|space)){2,})\b`,
)

// func ExtractSpelledNames(chunk string) map[string]string {
// 	out := map[string]string{}

// 	for _, m := range spellRe.FindAllStringSubmatch(chunk, -1) {
// 		raw := m[1]
// 		letters := strings.Fields(raw)
// 		word := strings.Builder{}
// 		for _, l := range letters {
// 			word.WriteString(strings.ToUpper(l))
// 		}
// 		name := strings.Title(strings.ToLower(word.String()))
// 		out[strings.ToLower(name)] = name
// 	}
// 	return out
// }

func ExtractSpelledNames(chunk string) map[string]string {
	out := map[string]string{}

	for _, m := range spellRe.FindAllStringSubmatch(chunk, -1) {
		toks := strings.Fields(strings.ToLower(m[1]))
		var buf strings.Builder

		for _, t := range toks {
			if t == "space" {
				buf.WriteRune(' ')

			} else if len(t) == 1 && t[0] >= 'a' && t[0] <= 'z' {
				buf.WriteByte((t[0]))
			}
			// ignore anything else
		}

		// Collapse multiple spaces, trim:
		name := strings.Join(strings.Fields(buf.String()), " ")

		if name != "" {
			name = cases.Title(language.English).String(name)
			out[strings.ToLower(name)] = name
		}
	}
	return out
}
