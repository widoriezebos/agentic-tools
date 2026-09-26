package uitools

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// The one operation that reads nothing.
//
// A human writing in an editor — a sheet's field today — hands that editor over
// to the Partner with "Ask about this". When the Partner has words to offer for
// one of its fields, it calls this: the editor's name as the sheet's head says
// it, the field's name as the editor's own label says it, and the whole new
// value it proposes. Nothing is written and nothing is applied. The result says
// what it prepared, and the interface offers it to the human as a card with Use
// this beside it.
//
// This server cannot judge whether the field is one the human handed over: it
// runs in its own process, with the workspace's readers and no capture, so it
// has no way to know what is on the human's screen. That is the whole of why it
// validates its arguments and says "prepared" rather than "offered": the
// turn-owning service holds the capture and admits the suggestion against the
// draft the human actually handed over, and the human decides whether to use it.

// maxSuggestion is how much text one suggestion carries, in characters. A
// field of a sheet is a line or a paragraph. Past this the call is refused in
// words rather than cut, because half a sentence offered as a whole value is a
// suggestion a human cannot read as what the Partner meant.
const maxSuggestion = 4000

// The fixed form a prepared suggestion travels back in, written down once here
// and read once by the interface's host.
//
// It is textual because the result text is the channel this server has. The
// header names the editor and the field; the separator ends the framing; and
// everything after the separator is the suggestion, opaque to the end. A
// suggestion may itself hold a line that reads like a header — a Partner asked
// to improve an intent may quote one — and nothing after the separator is read
// as one.
const (
	SuggestionHeader    = "Suggestion-for: "
	SuggestionJoin      = " · "
	SuggestionSeparator = "--- the suggestion follows, whole and to the end ---"
)

// PreparedLine is what a prepared suggestion answers the model with.
//
// It says "prepared" and then says what preparing is not. A Partner that read
// "prepared as a suggestion for Intent" once told a human it had put a card on
// their sheet, and there was no card: the field was not one the human had handed
// over, and this server cannot know that. So the result says outright that
// preparing does not confirm showing, and names the one condition under which
// it is shown at all (g1-s52 D3).
const PreparedLine = "prepared; preparing does not confirm it was shown: " +
	"it is offered only for a field of a draft the human has handed over"

// suggest prepares one suggestion, or refuses the call in words.
func suggest(editor, field, text string) Result {
	editor = oneLine(editor)
	field = oneLine(field)
	// Only the framing's own newlines are the framing's. What the model wrote
	// travels as it wrote it, apart from the blank lines a transport may have
	// left at either end of it.
	text = strings.Trim(text, "\n")
	switch {
	case editor == "":
		return refusedCall("this tool needs the name of the editor the field is in, as the sheet's own head says it")
	case field == "":
		return refusedCall("this tool needs the name of the field, as the editor's own label says it")
	case strings.TrimSpace(text) == "":
		return refusedCall("this tool needs the text to offer for " + field +
			"; a suggestion is the field's whole new value")
	case utf8.RuneCountInString(text) > maxSuggestion:
		return refusedCall("a suggestion carries at most " + strconv.Itoa(maxSuggestion) +
			" characters and this one is " + strconv.Itoa(utf8.RuneCountInString(text)) +
			"; offer the field's whole new value, shorter")
	}
	return Result{Prepared: PreparedLine + "\n" +
		SuggestionHeader + editor + SuggestionJoin + field + "\n" +
		SuggestionSeparator + "\n" +
		text + "\n"}
}

// refusedCall is a call this server could read and would not answer. It is a
// failure the model is told in words, like a refused read, and it is worded for
// a call that read nothing.
func refusedCall(words string) Result {
	return Result{Problem: words, Prepared: "Outcome: this call was refused — " + words + "\n"}
}
