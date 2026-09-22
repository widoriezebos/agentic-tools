package project

import (
	"io"
	"os"
	"strings"
	"unicode"
)

// A record's summary is a convention, not a key: the first prose paragraph
// after the head is what the record says it is about, and the pane shows that
// rather than a filename. Nothing is written to make one, and a record whose
// body opens with a table, a list or a heading has none until a human writes
// one.
//
// The paragraph is carried as plain text — emphasis dropped, a link reduced to
// the words it reads as — because it is shown as one line beside a title, not
// as a document. It is clipped at a sentence end so that what the human reads
// is a whole thought and never a half word.

// summaryLimit is how many characters a summary may carry. It is a measure,
// not a truncation: the clip below stops at the last sentence that fits.
const summaryLimit = 320

// summaryHead is how far into a bound document its first paragraph is looked
// for — the same bounded read the title takes, with room for a head and a
// heading above the prose.
const summaryHead = 8 << 10

// summaryOf is the first prose paragraph of a body, as plain text, or "".
func summaryOf(body []string) string {
	return clip(plainText(strings.Join(firstParagraph(body), " ")))
}

// summaryOfFile is the same rule applied to a document that is not a record:
// one bounded read, and the first paragraph after whatever opens the file. A
// file that cannot be opened has no summary rather than a reason, because a
// listing that could not read a chapter still lists it.
func summaryOfFile(absolute string) string {
	file, err := os.Open(absolute)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	head, err := io.ReadAll(io.LimitReader(file, summaryHead+1))
	if err != nil {
		return ""
	}
	lines := splitLines(string(head))
	// The read may have stopped inside a line. That line is dropped rather
	// than summarised, so no summary ends in the middle of a word because of
	// where the limit fell.
	if len(head) > summaryHead && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return summaryOf(lines)
}

// firstParagraph is the first run of lines that is prose. Everything that is
// not prose — a heading, a list, a table, a quote, a fence, a rule, a block of
// HTML — is stepped over rather than read, so a record whose body opens with a
// `## Vision` heading is summarised by what that section says.
func firstParagraph(lines []string) []string {
	for index := 0; index < len(lines); {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			index++
			continue
		}
		if fence := fenceOf(trimmed); fence != "" {
			index = skipFence(lines, index, fence)
			continue
		}
		if !opensProse(lines[index]) {
			index = skipBlock(lines, index)
			continue
		}
		start := index
		for index < len(lines) {
			if strings.TrimSpace(lines[index]) == "" {
				break
			}
			if index > start && !opensProse(lines[index]) {
				break
			}
			index++
		}
		return lines[start:index]
	}
	return nil
}

// opensProse reports whether a line begins a paragraph rather than one of the
// block shapes Markdown spells with a leading character.
func opensProse(line string) bool {
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '#', '>', '|', '<', '=':
		return false
	}
	for _, bullet := range []string{"- ", "* ", "+ ", "-\t", "*\t", "+\t"} {
		if strings.HasPrefix(trimmed, bullet) {
			return false
		}
	}
	return !isThematicBreak(trimmed) && !isOrderedItem(trimmed)
}

// skipBlock steps over the block beginning at this line. A heading and a rule
// are one line; everything else runs to the first blank line.
func skipBlock(lines []string, index int) int {
	trimmed := strings.TrimSpace(lines[index])
	if strings.HasPrefix(trimmed, "#") || isThematicBreak(trimmed) {
		return index + 1
	}
	for index < len(lines) && strings.TrimSpace(lines[index]) != "" {
		index++
	}
	return index
}

func fenceOf(trimmed string) string {
	for _, fence := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, fence) {
			return fence
		}
	}
	return ""
}

func skipFence(lines []string, index int, fence string) int {
	for index++; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), fence) {
			return index + 1
		}
	}
	return index
}

// isThematicBreak is three or more of one rule character and nothing else.
func isThematicBreak(trimmed string) bool {
	for _, character := range []string{"-", "*", "_"} {
		stripped := strings.ReplaceAll(strings.ReplaceAll(trimmed, character, ""), " ", "")
		if stripped == "" && strings.Count(trimmed, character) >= 3 {
			return true
		}
	}
	return false
}

// isOrderedItem is a number, a dot or a bracket, and a space.
func isOrderedItem(trimmed string) bool {
	digits := 0
	for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits+1 >= len(trimmed) {
		return false
	}
	if trimmed[digits] != '.' && trimmed[digits] != ')' {
		return false
	}
	return trimmed[digits+1] == ' ' || trimmed[digits+1] == '\t'
}

// plainText reduces one paragraph to the words a human reads in it: code spans
// keep their contents, a link keeps its text, emphasis is dropped, and an
// escape yields the character it escaped. Nothing here renders; a summary is a
// line of text beside a title, and markup in it would be noise.
func plainText(text string) string {
	runes := []rune(text)
	var out strings.Builder
	for index := 0; index < len(runes); {
		switch character := runes[index]; {
		case character == '\\' && index+1 < len(runes) && isMarkup(runes[index+1]):
			out.WriteRune(runes[index+1])
			index += 2
		case character == '`':
			contents, after := codeSpan(runes, index)
			out.WriteString(contents)
			index = after
		case character == '!' && index+1 < len(runes) && runes[index+1] == '[':
			index++
		case character == '[':
			contents, after, ok := linkText(runes, index)
			if !ok {
				out.WriteRune(character)
				index++
				continue
			}
			out.WriteString(plainText(contents))
			index = after
		case isEmphasis(runes, index):
			for index < len(runes) && runes[index] == character {
				index++
			}
		default:
			out.WriteRune(character)
			index++
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// codeSpan answers the contents of the span opening at index and the offset
// after it. A run of backticks with no closing run is not a span, and the
// backticks are kept as the characters they are.
func codeSpan(runes []rune, index int) (string, int) {
	opening := index
	for index < len(runes) && runes[index] == '`' {
		index++
	}
	width := index - opening
	start := index
	for index < len(runes) {
		if runes[index] != '`' {
			index++
			continue
		}
		closing := index
		for index < len(runes) && runes[index] == '`' {
			index++
		}
		if index-closing == width {
			return strings.TrimSpace(string(runes[start:closing])), index
		}
	}
	return string(runes[opening:]), len(runes)
}

// linkText answers the text of the link opening at index and the offset after
// the whole of it. A bracket that opens nothing is not a link.
func linkText(runes []rune, index int) (string, int, bool) {
	depth := 0
	for cursor := index; cursor < len(runes); cursor++ {
		switch runes[cursor] {
		case '\\':
			cursor++
		case '[':
			depth++
		case ']':
			depth--
			if depth > 0 {
				continue
			}
			text := string(runes[index+1 : cursor])
			return text, skipTarget(runes, cursor+1), true
		}
	}
	return "", index, false
}

// skipTarget steps over a link's destination, inline or by reference.
func skipTarget(runes []rune, index int) int {
	if index >= len(runes) {
		return index
	}
	closing := map[rune]rune{'(': ')', '[': ']'}[runes[index]]
	if closing == 0 {
		return index
	}
	for cursor := index + 1; cursor < len(runes); cursor++ {
		if runes[cursor] == '\\' {
			cursor++
			continue
		}
		if runes[cursor] == closing {
			return cursor + 1
		}
	}
	return index
}

// isEmphasis reports whether the run beginning here is a marker rather than a
// character of the prose. An underscore inside a word is part of the word, as
// every identifier written in a design document is.
func isEmphasis(runes []rune, index int) bool {
	character := runes[index]
	if character != '*' && character != '_' && character != '~' {
		return false
	}
	if character != '_' {
		return true
	}
	end := index
	for end < len(runes) && runes[end] == '_' {
		end++
	}
	before := index > 0 && isWord(runes[index-1])
	after := end < len(runes) && isWord(runes[end])
	return !(before && after)
}

func isWord(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character)
}

func isMarkup(character rune) bool {
	return strings.ContainsRune("\\`*_{}[]()#+-.!|~<>", character)
}

// clip keeps a summary to its measure and ends it where a sentence ends. When
// no sentence ends within reach the last whole word is kept and an ellipsis
// says that there is more, so nothing is ever cut through a word.
func clip(text string) string {
	runes := []rune(text)
	if len(runes) <= summaryLimit {
		return text
	}
	cut := 0
	for index := 0; index < summaryLimit; index++ {
		if !isSentenceEnd(runes[index]) {
			continue
		}
		if index+1 >= len(runes) || runes[index+1] == ' ' {
			cut = index + 1
		}
	}
	if cut > 0 {
		return strings.TrimSpace(string(runes[:cut]))
	}
	kept := string(runes[:summaryLimit])
	if space := strings.LastIndex(kept, " "); space > 0 {
		kept = kept[:space]
	}
	return strings.TrimRight(kept, " ,;:") + "…"
}

func isSentenceEnd(character rune) bool {
	return character == '.' || character == '!' || character == '?'
}

func splitLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}
