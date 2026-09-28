package receipt

import (
	"os"
	"strings"
	"testing"
)

// Every free-text field is sanitized by one shared path: a CRLF in the note,
// the skills list, a delegate, or the retro summary each leaves exactly one
// log line and no carriage return (ported from the retired
// validate-metasystem.sh workflow-tooling section).
func TestEveryFreeTextFieldKeepsOneLineWithoutCarriageReturns(t *testing.T) {
	t.Parallel()
	crlf := "a\r\nb"
	opts := baseOptions(t)
	for index, edit := range []func(*Options){
		func(o *Options) { o.Note = crlf },
		func(o *Options) { o.Skills = crlf },
		func(o *Options) { o.Delegates = []string{crlf} },
	} {
		add := opts
		add.Type, add.Outcome = "implement", "shipped"
		edit(&add)
		if result := Add(add); result.Code != 0 {
			t.Fatalf("add %d refused: %+v", index, result)
		}
	}
	retro := opts
	retro.Summary = crlf
	if result := Retro(retro); result.Code != 0 {
		t.Fatalf("retro refused: %+v", result)
	}
	data, err := os.ReadFile(opts.File)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(data), "\n"); lines != 4 {
		t.Fatalf("four CRLF-bearing entries wrote %d lines:\n%q", lines, data)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("receipt sanitizer left a carriage return in the log: %q", data)
	}
}
