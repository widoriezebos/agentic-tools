package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// messageTraceFixture is a small module holding one of each message the
// direct scan of TestAuditMessagesAPersonReads cannot see, and the internal
// strings the traced scan must leave alone.
const messageTraceFixture = `package x

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

const RangeCode = "X_RANGE_REFUSED"

const hintConst = "the helper saw the ledger tip move"

type OpError struct{ Code, Message string }

func (e *OpError) Error() string       { return e.Message }
func (e *OpError) RefusalCode() string { return e.Code }

func operationRefusal(code, format string, args ...any) error {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

type RangeError struct{ Code, Commit, Reason string }

func (e *RangeError) Error() string { return "range refused at commit " + e.Commit + ": " + e.Reason }

func refuse(commit, reason string) error { return &RangeError{RangeCode, commit, reason} }

func check(kind string) error {
	if kind == "" {
		return operationRefusal(RangeCode, "path %s has class plan, which kind unit does not allow", kind)
	}
	return refuse("abc123", "the range crosses the accepted tree")
}

func summaryLine(n int) string { return fmt.Sprintf("%d goals wait for the lineage check", n) }

func report(n int) {
	fmt.Fprintln(os.Stderr, summaryLine(n))
	msg := strings.Join([]string{"the digest moved under the run", "nothing was done"}, "; ")
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stderr, hintConst)
	var b strings.Builder
	b.WriteString("the builder holds the epoch line")
	fmt.Fprintln(os.Stderr, b.String())
	log.Printf("internal log line about the lineage %d", n)
	_ = map[string]string{"lineage key": "internal value about lineage"}
}

type request struct{}

func (request) noteStream() *os.File { return os.Stderr }

func (r request) prepare(ours string) {
	fmt.Fprintf(r.noteStream(), "the landing ref moved under the run (ours=%s)\n", ours)
}

func hook() string {
	block := ` + "`" + `{"systemMessage":"Stop blocked: the steward epoch is stale"}` + "`" + `
	_ = map[string]string{"systemMessage": "the supervision generation is gone"}
	return block
}

func serve(w http.ResponseWriter) { http.Error(w, "the proof lease is gone", 409) }
`

// TestAuditMessagesTracedSeesTheBlindSpots holds the traced scan to the
// messages the direct scan misses: a refusal built from a format string in a
// helper, a text a helper returns or a local assembles, a followed constant,
// a note to a stream the writer pattern did not name, a hook's systemMessage,
// a browser refusal and an error type's Error text; and to the internal
// strings it must not report: a code, a log line, a map key.
func TestAuditMessagesTracedSeesTheBlindSpots(t *testing.T) {
	t.Parallel()
	module := messageTraceFixtureModule(t)
	direct := messageScan(t, module)
	traced := messageTraceScan(t, module, direct)
	messageTraceFixtureChecks(t, direct, traced)
}

// TestAuditMessagesTracedEnforcesAGroupsPaths: a path a group enforces
// with enforceTracedMessages is judged in enforce mode. It runs before the
// parallel audits and removes its fixture key again.
func TestAuditMessagesTracedEnforcesAGroupsPaths(t *testing.T) {
	module := messageTraceFixtureModule(t)
	enforceTracedMessages("internal/x/x.go#summaryLine")
	defer delete(messageTracedModes, "internal/x/x.go#summaryLine")
	enforced := 0
	for _, source := range messageTraceScan(t, module, messageScan(t, module)) {
		want := auditReport
		if source.Function == "summaryLine" {
			want = auditEnforce
			enforced++
		}
		if source.Mode != want {
			t.Errorf("%s %q: mode %q, want %q", source.Function, source.Text, source.Mode, want)
		}
	}
	if enforced == 0 {
		t.Error("no source of the enforced function was found")
	}
}

func messageTraceFixtureModule(t *testing.T) string {
	t.Helper()
	module := t.TempDir()
	dir := filepath.Join(module, "internal", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(module, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(messageTraceFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return module
}

func messageTraceFixtureChecks(t *testing.T, direct []messageSource, traced []messageTraced) {
	t.Helper()
	found := map[string]string{}
	for _, source := range traced {
		found[source.Text] = source.Trace
	}
	for text, class := range map[string]string{
		"path … has class plan, which kind unit does not allow": messageTraceHelper,
		"the range crosses the accepted tree":                   messageTraceHelper,
		"… goals wait for the lineage check":                    messageTraceAssembled,
		"the digest moved under the run":                        messageTraceAssembled,
		"nothing was done":                                      messageTraceAssembled,
		"the builder holds the epoch line":                      messageTraceAssembled,
		"the helper saw the ledger tip move":                    messageTraceConstant,
		"the landing ref moved under the run (ours=…)\n":        messageTraceNotice,
		"Stop blocked: the steward epoch is stale":              messageTraceHook,
		"the supervision generation is gone":                    messageTraceHook,
		"the proof lease is gone":                               messageTraceUI,
		"range refused at commit …: …":                          messageTraceErrorMethod,
	} {
		got, ok := found[text]
		if !ok {
			t.Errorf("the traced scan misses %q; it found %v", text, found)
			continue
		}
		if got != class {
			t.Errorf("%q: class %q, want %q", text, got, class)
		}
		for _, source := range direct {
			if source.Text == text {
				t.Errorf("%q is already in the direct scan, so the fixture proves nothing", text)
			}
		}
	}
	// A refusal type with no remedy field carries line 2 in its words.
	for _, source := range traced {
		refusal := source.Text == "path … has class plan, which kind unit does not allow"
		if refusal != slices.Contains(source.Violations, "no-command") {
			t.Errorf("%q: violations %v; only the OpError refusal lacks its command", source.Text, source.Violations)
		}
		field := strings.HasPrefix(source.Text, "the landing ref moved")
		if field != slices.Contains(source.Violations, "field:ours") {
			t.Errorf("%q: violations %v; only the notice shows an internal key=value", source.Text, source.Violations)
		}
	}
	for text := range found {
		for _, internal := range []string{"X_RANGE_REFUSED", "internal log line", "lineage key", "internal value", "abc123"} {
			if strings.Contains(text, internal) {
				t.Errorf("the traced scan reports the internal string %q", text)
			}
		}
	}
	for _, source := range traced {
		if source.Mode != auditReport {
			t.Errorf("%s:%d %q: mode %q; a traced message is reported until its group enforces it", source.File, source.Line, source.Text, source.Mode)
		}
	}
}

// messageTraceRemedyFixture is a refusal type with a field for its remedy:
// a literal that fills it resolves, one that leaves it empty does not.
const messageTraceRemedyFixture = `package y

import "fmt"

type OpError struct{ Code, Message, Run string }

func (e *OpError) Error() string       { return e.Message }
func (e *OpError) RefusalCode() string { return e.Code }

func refuse(format string, args ...any) error {
	return &OpError{Code: "Y_REFUSED", Message: fmt.Sprintf(format, args...)}
}

func refuseBlank(format string, args ...any) error {
	return &OpError{Code: "Y_REFUSED", Message: fmt.Sprintf(format, args...), Run: ""}
}

func refuseRun(run, format string, args ...any) error {
	return &OpError{Code: "Y_REFUSED", Message: fmt.Sprintf(format, args...), Run: run}
}

func filled(id string) error {
	return refuseRun("metasystem landing status", "the batch %s is sealed, so nothing was joined", id)
}

func empty(id string) error { return refuse("the dispatch record %s was replaced while it was read", id) }

func blank(id string) error { return refuseBlank("the hazard ledger has an unknown entry %s", id) }
`

// TestAuditMessagesTracedJudgesAnEmptyRemedy: a refusal whose type has a
// remedy field is no-command when a literal leaves that field empty, and
// resolves when it fills it. Until those remedies are filled, the finding is
// reported even on an enforced path.
func TestAuditMessagesTracedJudgesAnEmptyRemedy(t *testing.T) {
	module := t.TempDir()
	dir := filepath.Join(module, "internal", "y")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(module, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "y.go"), []byte(messageTraceRemedyFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	enforceTracedMessages("internal/y/y.go")
	defer delete(messageTracedModes, "internal/y/y.go")
	seen := 0
	for _, source := range messageTraceScan(t, module, messageScan(t, module)) {
		noCommand := slices.Contains(source.Violations, "no-command")
		switch source.Text {
		case "the batch … is sealed, so nothing was joined":
			seen++
			if noCommand {
				t.Errorf("a filled Run resolves: %v", source.Violations)
			}
		case "the dispatch record … was replaced while it was read", "the hazard ledger has an unknown entry …":
			seen++
			if !noCommand || source.Mode != auditReport {
				t.Errorf("%q: violations %v mode %q; an empty Run is no-command, reported", source.Text, source.Violations, source.Mode)
			}
		}
	}
	if seen != 3 {
		t.Fatalf("the traced scan saw %d of the three refusals", seen)
	}
}
