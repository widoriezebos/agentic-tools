package snapshot

import "strings"

// intentLabel is what most goal records open their intent with. It names the
// field for whoever writes the record and says nothing to whoever reads a
// list of goals, so a title starts after it.
const intentLabel = "What:"

// GoalTitle is what a goal is called wherever a list names it by a line of
// its intent: the first sentence of the intent, without the label it opens
// with, in at most `most` runes. Every page that names a goal this way calls
// this, so one goal reads the same on all of them; how long the line may be
// is the caller's, because a chip and a row have different room.
func GoalTitle(intent string, most int) string {
	return Lede(IntentWords(intent), most)
}

// IntentWords is a goal's intent as a title is taken from it: without the
// label it opens with, and otherwise as it was written. A caller that cuts
// its own sentence cuts it from this.
func IntentWords(intent string) string {
	trimmed := strings.TrimSpace(intent)
	if len(trimmed) >= len(intentLabel) && strings.EqualFold(trimmed[:len(intentLabel)], intentLabel) {
		return strings.TrimSpace(trimmed[len(intentLabel):])
	}
	return trimmed
}

// Lede is the first sentence of a text on one line, cut at a sentence end and
// then at the last word boundary before `most` runes, with an ellipsis where
// it was cut. It is the cut a goal's title is made with, and what a row uses
// for any other text it has one line for.
func Lede(text string, most int) string {
	text = strings.Join(strings.Fields(text), " ")
	if at := strings.Index(text, ". "); at >= 0 {
		text = text[:at+1]
	}
	runes := []rune(text)
	if len(runes) <= most {
		return text
	}
	cut := most
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = most
	}
	return strings.TrimRight(string(runes[:cut]), " ,;:") + "…"
}
