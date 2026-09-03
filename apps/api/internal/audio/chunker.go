package audio

import (
	"strings"
	"unicode/utf8"
)

// ChunkText splits text into ordered chunks where each chunk has at most maxRunes,
// adhering strictly to the exact-narration invariant:
// strings.Join(ChunkText(text, maxRunes), "") == text
func ChunkText(text string, maxRunes int) []string {
	if text == "" {
		return nil
	}
	if maxRunes <= 0 {
		return []string{text}
	}
	if utf8.RuneCountInString(text) <= maxRunes {
		return []string{text}
	}

	// Break down into smallest indivisible units based on boundary hierarchy
	units := breakIntoUnits(text, maxRunes)

	// Greedily pack units into chunks up to maxRunes
	var chunks []string
	var current strings.Builder
	currentRunes := 0

	for _, unit := range units {
		unitRunes := utf8.RuneCountInString(unit)
		if currentRunes > 0 && currentRunes+unitRunes > maxRunes {
			chunks = append(chunks, current.String())
			current.Reset()
			currentRunes = 0
		}
		current.WriteString(unit)
		currentRunes += unitRunes
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func breakIntoUnits(text string, maxRunes int) []string {
	// 1. Paragraph boundary: split at \n\n or \r\n\r\n
	paras := splitKeepDelim(text, []string{"\r\n\r\n", "\n\n"})
	var result []string
	for _, para := range paras {
		if utf8.RuneCountInString(para) <= maxRunes {
			result = append(result, para)
			continue
		}
		// 2. Line boundary: split at \r\n or \n
		lines := splitKeepDelim(para, []string{"\r\n", "\n"})
		for _, line := range lines {
			if utf8.RuneCountInString(line) <= maxRunes {
				result = append(result, line)
				continue
			}
			// 3. Sentence boundary: split at ". ", "! ", "? ", "; ", ": "
			sentences := splitSentenceDelim(line)
			for _, sent := range sentences {
				if utf8.RuneCountInString(sent) <= maxRunes {
					result = append(result, sent)
					continue
				}
				// 4. Word boundary: split at spaces/tabs
				words := splitWordDelim(sent)
				for _, word := range words {
					if utf8.RuneCountInString(word) <= maxRunes {
						result = append(result, word)
						continue
					}
					// 5. Hard rune split
					runes := splitHardRunes(word, maxRunes)
					result = append(result, runes...)
				}
			}
		}
	}
	return result
}

func splitKeepDelim(s string, delims []string) []string {
	var res []string
	remaining := s
	for len(remaining) > 0 {
		earliestIdx := -1
		earliestDelimLen := 0
		for _, delim := range delims {
			idx := strings.Index(remaining, delim)
			if idx != -1 && (earliestIdx == -1 || idx < earliestIdx) {
				earliestIdx = idx
				earliestDelimLen = len(delim)
			}
		}
		if earliestIdx == -1 {
			res = append(res, remaining)
			break
		}
		end := earliestIdx + earliestDelimLen
		res = append(res, remaining[:end])
		remaining = remaining[end:]
	}
	return res
}

func splitSentenceDelim(s string) []string {
	delims := []string{". ", "! ", "? ", "; ", ": ", ".\n", "!\n", "?\n", ".\r\n", "!\r\n", "?\r\n"}
	return splitKeepDelim(s, delims)
}

func splitWordDelim(s string) []string {
	var res []string
	runes := []rune(s)
	n := len(runes)
	start := 0

	for i := 0; i < n; i++ {
		r := runes[i]
		if r == ' ' || r == '\t' {
			// consume any subsequent spaces in this unit
			j := i
			for j < n && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			res = append(res, string(runes[start:j]))
			start = j
			i = j - 1
		}
	}
	if start < n {
		res = append(res, string(runes[start:]))
	}
	return res
}

func splitHardRunes(s string, maxRunes int) []string {
	var res []string
	runes := []rune(s)
	for len(runes) > 0 {
		n := len(runes)
		if n > maxRunes {
			n = maxRunes
		}
		res = append(res, string(runes[:n]))
		runes = runes[n:]
	}
	return res
}
