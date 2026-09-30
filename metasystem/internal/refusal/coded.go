package refusal

import (
	"errors"
	"strings"
)

// Coded is a refusal as "Messages a Person Reads" (docs/design/
// design-principles.md) shapes it: Error is the plain reason a person reads
// by default and, when a command resolves it, that command on a second
// "run:" line. The register code, the key=value facts and any background
// are details, read by the register, refusal records, --verbose and --json
// (DetailOf), never printed by default.
type Coded struct {
	Code       string
	Facts      string // key=value words, space separated; may be empty
	Reason     error  // the plain reason; it may wrap the cause
	Run        string // the command that resolves it; may be empty
	Background string // more about the cause, for the detail only
}

// Error is the refusal's plain reason, then its command on a "run:" line.
func (e *Coded) Error() string {
	reason := ""
	if e.Reason != nil {
		reason = e.Reason.Error()
	}
	if e.Run == "" {
		return reason
	}
	return reason + "\nrun: " + e.Run
}

func (e *Coded) Unwrap() error { return e.Reason }

// RefusalCode and RefusalDetail make Coded a Coder.
func (e *Coded) RefusalCode() string   { return e.Code }
func (e *Coded) RefusalDetail() string { return e.Detail() }

// Coder is any error that carries a register code and a detail line; a
// package that may not import this one (the board) implements it itself.
type Coder interface {
	error
	RefusalCode() string
	RefusalDetail() string
}

// Detail is the code-first line records and --verbose keep:
// "CODE facts: reason", then ": background" when there is one.
func (e *Coded) Detail() string {
	reason := ""
	if e.Reason != nil {
		reason = e.Reason.Error()
	}
	detail := strings.TrimSpace(e.Code+" "+e.Facts) + ": " + reason
	if e.Background != "" {
		detail += ": " + e.Background
	}
	return detail
}

// New is a coded refusal whose reason is a plain sentence.
func New(code, facts string, reason error) error {
	return &Coded{Code: code, Facts: facts, Reason: reason}
}

// Detailed is an error that carries a detail beside its plain text.
type Detailed interface {
	error
	Detail() string
}

// Detail is the detail of the first detailed error in err's chain, or ""
// when it holds none.
func Detail(err error) string {
	var detailed Detailed
	if errors.As(err, &detailed) {
		return detailed.Detail()
	}
	return ""
}

// CodeOf is the refusal code err carries (a Coder in its chain), or "".
// The code is data beside the words; the words are never read for one.
func CodeOf(err error) string {
	var coded Coder
	if errors.As(err, &coded) {
		return coded.RefusalCode()
	}
	return ""
}

// DetailOf is err's code-first detail line when it carries a code, else its
// message: what --verbose and the refusal records show.
func DetailOf(err error) string {
	var coded Coder
	if errors.As(err, &coded) {
		return coded.RefusalDetail()
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
