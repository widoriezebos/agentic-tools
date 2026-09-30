// Package verbresult is the one result a MetaSystem verb prints under
// --json, and the one way a MetaSystem process reads another's result
// (design structured-output.md §3). A parent never decides anything from a
// child's human text: it passes --json, reads the child's stdout as exactly
// one envelope, and branches on Outcome and Code. A missing, truncated or
// unreadable envelope reads as Unknown, never as success.
package verbresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// SchemaVersion is the envelope's version.
const SchemaVersion = 1

// The outcomes a result can have. Unknown is never printed: it is what a
// reader answers when the child's result is missing or unreadable.
const (
	Confirmed  = "confirmed"
	Unchanged  = "unchanged"
	InProgress = "in-progress"
	Partial    = "partial"
	Refused    = "refused"
	Failed     = "failed"
	Unknown    = "unknown"
)

type Target struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Next struct {
	Argv   []string `json:"argv"`
	Reason string   `json:"reason"`
}

// Result is the envelope: the public verbs' intentResult fields, plus Code,
// the refusal-register code a parent branches on. Data holds what a parent
// reads beyond the outcome (a plan, a refusal's state).
type Result struct {
	SchemaVersion int             `json:"schemaVersion"`
	Verb          string          `json:"verb"`
	Targets       []Target        `json:"targets"`
	Outcome       string          `json:"outcome"`
	Code          string          `json:"code,omitempty"`
	Summary       string          `json:"summary"`
	Data          json.RawMessage `json:"data,omitempty"`
	Next          *Next           `json:"next,omitempty"`
	Decision      string          `json:"decision,omitempty"`
	Details       []string        `json:"details,omitempty"`
	// Exit is the child's exit status as the reader saw it; not printed.
	Exit int `json:"-"`
}

// DataCarrier is an error that carries the facts a parent branches on,
// printed as the envelope's data (for example a refused candidate's state).
type DataCarrier interface {
	ResultData() any
}

// OutcomeForExit is the outcome an exit status stands for, the one table
// both writer and reader hold (R3): 0 confirmed, 76 unchanged (a reusable
// success), 75 in progress (a live duplicate), 2, 77 and 78 refused, any
// other status failed.
func OutcomeForExit(status int) string {
	switch status {
	case 0:
		return Confirmed
	case 76:
		return Unchanged
	case 75:
		return InProgress
	case 2, 77, 78:
		return Refused
	}
	return Failed
}

// Permitted reports whether a child that exited with status may report
// outcome: the consistency check a reader applies before it trusts either.
func Permitted(outcome string, status int) bool {
	switch outcome {
	case Confirmed:
		return status == 0
	case Unchanged:
		return status == 0 || status == 76
	case InProgress:
		return status == 75 || status == 1
	case Partial:
		return status == 1
	case Refused:
		return status == 1 || status == 2 || status == 77 || status == 78
	case Failed:
		return status > 0 && status != 75 && status != 76
	}
	return false
}

// FromError is the result of verb ending with status and err: the outcome
// the status stands for (refused for a coded error ending with status 1),
// and, when err carries a register code, that code,
// its plain reason as the summary and its command as the next step. A plain
// error has no code.
func FromError(verb string, status int, err error, data any) Result {
	result := Result{SchemaVersion: SchemaVersion, Verb: verb, Targets: []Target{}, Outcome: OutcomeForExit(status), Exit: status}
	if err != nil {
		result.Summary = err.Error()
		var coded *refusal.Coded
		var coder refusal.Coder
		if errors.As(err, &coded) {
			result.Code = coded.Code
			if coded.Reason != nil {
				result.Summary = coded.Reason.Error()
			}
			if coded.Run != "" {
				result.Next = &Next{Argv: strings.Fields(coded.Run), Reason: "resolves it"}
			}
			result.Details = []string{coded.Detail()}
		} else if errors.As(err, &coder) {
			result.Code = coder.RefusalCode()
			result.Details = []string{coder.RefusalDetail()}
		}
		if result.Code != "" && result.Outcome == Failed && status == 1 {
			result.Outcome = Refused // a coded refusal ending with status 1
		}
		var carrier DataCarrier
		if data == nil && errors.As(err, &carrier) {
			data = carrier.ResultData()
		}
	}
	if data != nil {
		if encoded, marshalErr := json.Marshal(data); marshalErr == nil {
			result.Data = encoded
		}
	}
	return result
}

// Write prints result as the one envelope on w.
func Write(w io.Writer, result Result) error {
	result.SchemaVersion = SchemaVersion
	if result.Targets == nil {
		result.Targets = []Target{}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", encoded)
	return err
}

// stderrQuote is how much of a child's stderr a reader keeps to quote.
const stderrQuote = 2048

// Run runs cmd, whose argv already carries --json, and reads its stdout as
// exactly one envelope of verb. Stdout goes to a buffer and never mixes
// with stderr; stderr goes where the caller pointed it and its tail is kept
// to quote. The result's outcome is Unknown, with an error that quotes the
// stderr tail, when the child printed nothing, more than one object, invalid
// JSON, another schema or verb, or an outcome its exit status does not
// permit. No path turns Unknown into success. A child that ran and reported
// is no error: its Outcome and Code say what happened.
func Run(cmd *exec.Cmd, verb string) (Result, error) {
	var stdout bytes.Buffer
	tail := &tailBuffer{limit: stderrQuote}
	cmd.Stdout = &stdout
	if cmd.Stderr == nil {
		cmd.Stderr = tail
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, tail)
	}
	runErr := cmd.Run()
	status := -1
	if cmd.ProcessState != nil {
		status = cmd.ProcessState.ExitCode()
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return unknown(verb, status, fmt.Errorf("%s did not run: %w", verb, runErr), tail)
	}
	return Read(stdout.Bytes(), verb, status, tail.String())
}

// Read judges a child's stdout and exit status as Run does; stderr is only
// quoted.
func Read(stdout []byte, verb string, status int, stderr string) (Result, error) {
	tail := &tailBuffer{limit: stderrQuote}
	_, _ = tail.Write([]byte(stderr))
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	decoder.DisallowUnknownFields()
	var result Result
	if err := decoder.Decode(&result); err != nil {
		return unknown(verb, status, fmt.Errorf("%s printed no readable result (exit %d): %v", verb, status, err), tail)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return unknown(verb, status, fmt.Errorf("%s printed more than its one result (exit %d)", verb, status), tail)
	}
	switch {
	case result.SchemaVersion != SchemaVersion:
		return unknown(verb, status, fmt.Errorf("%s printed a result of schema %d, not %d", verb, result.SchemaVersion, SchemaVersion), tail)
	case result.Verb != verb:
		return unknown(verb, status, fmt.Errorf("%s printed the result of %q", verb, result.Verb), tail)
	case !Permitted(result.Outcome, status):
		return unknown(verb, status, fmt.Errorf("%s reported %q but exited %d", verb, result.Outcome, status), tail)
	}
	result.Exit = status
	return result, nil
}

func unknown(verb string, status int, err error, tail *tailBuffer) (Result, error) {
	if quoted := strings.TrimSpace(tail.String()); quoted != "" {
		err = fmt.Errorf("%w; it said: %s", err, quoted)
	}
	return Result{SchemaVersion: SchemaVersion, Verb: verb, Targets: []Target{}, Outcome: Unknown, Exit: status}, err
}

// Err is a result that did not succeed, as an error a person reads (its
// summary, then its command) that keeps its register code for errors.As.
func (result Result) Err() error {
	reason := strings.TrimSpace(result.Summary)
	if reason == "" {
		reason = result.Verb + " ended " + result.Outcome
	}
	run := ""
	if result.Next != nil {
		run = strings.Join(result.Next.Argv, " ")
	}
	return &refusal.Coded{Code: result.Code, Reason: errors.New(reason), Run: run}
}

// DecodeData reads the result's data into value, refusing unknown fields.
func (result Result) DecodeData(value any) error {
	if len(result.Data) == 0 {
		return fmt.Errorf("%s printed no data", result.Verb)
	}
	decoder := json.NewDecoder(bytes.NewReader(result.Data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	limit int
	data  []byte
}

func (buffer *tailBuffer) Write(p []byte) (int, error) {
	buffer.data = append(buffer.data, p...)
	if extra := len(buffer.data) - buffer.limit; extra > 0 {
		buffer.data = buffer.data[extra:]
	}
	return len(p), nil
}

func (buffer *tailBuffer) String() string { return string(buffer.data) }
