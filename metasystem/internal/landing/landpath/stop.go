package landpath

import (
	"fmt"
	"io"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// Stop is what a landing or a commit that stopped tells the person who ran
// it, in the two lines of "Messages a Person Reads"
// (docs/design/design-principles.md): Reason is what happened and why, in
// plain words; Run is the one command that resolves it, with every value the
// landing knows filled in; Then goes with it ("then repeat this command"), or
// says in words what to do first when no single command does. Codes,
// verdicts, paths lists and step logs go to the details stream instead.
type Stop struct {
	Reason string
	Run    []string
	Then   string
	// cause is the refusal in its own terms (code and words) where a record
	// keeps it: the why a carried landing's abandoned reservation names.
	cause string
}

// said records the first stop of a landing: the innermost cause wins, so a
// step that failed because its commit was refused reports the refusal, never
// the step.
func (s *Stop) said(reason string, run []string, then string) {
	if s == nil || s.Reason != "" {
		return
	}
	s.Reason, s.Run, s.Then = reason, run, then
}

// writeStop writes a stop's two lines to the person's stream: the reason,
// then "run: COMMAND  (then ...)" or "needed first: WORDS".
func writeStop(w io.Writer, stop Stop) {
	fmt.Fprintln(w, stop.Reason)
	switch {
	case len(stop.Run) > 0 && stop.Then != "":
		fmt.Fprintf(w, "run: %s  (%s)\n", commandLine(stop.Run), stop.Then)
	case len(stop.Run) > 0:
		fmt.Fprintf(w, "run: %s\n", commandLine(stop.Run))
	case stop.Then != "":
		fmt.Fprintf(w, "needed first: %s\n", stop.Then)
	}
}

// writeDetails writes background lines to the details stream, one per line.
func writeDetails(w io.Writer, lines ...string) {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintln(w, strings.TrimRight(line, "\n"))
		}
	}
}

// oneLine is text's first line, cut to about the length line 1 aims at.
func oneLine(text string) string {
	text = strings.TrimSpace(text)
	if cut := strings.IndexByte(text, '\n'); cut >= 0 {
		text = strings.TrimSpace(text[:cut])
	}
	if runes := []rune(text); len(runes) > 90 {
		text = string(runes[:89]) + "…"
	}
	return text
}

// commandLine is argv as a person pastes it.
func commandLine(argv []string) string {
	words := make([]string, len(argv))
	for index, arg := range argv {
		words[index] = shellquote.Word(arg)
	}
	return strings.Join(words, " ")
}

// repeat is the "then repeat this command" that goes with a run line.
const repeat = "then repeat this command"

// The reasons the boundary and the driver share.
const (
	// notHolderReason is a refusal by the checkout's holder check.
	notHolderReason = "another session holds this checkout, or this shell couldn't be identified, so nothing was committed"
	// notHolderThen is what to do about it.
	notHolderThen = "land from the session that holds this checkout; --verbose names it"
	// brainReason is the brain fence's refusal: the coordinator's checkout
	// never lands.
	brainReason = "this checkout is the coordinator's, which never lands changes, so nothing was committed"
	brainThen   = "land from the checkout of the machine doing the work"
)
