package refusal

import (
	"errors"
	"strings"
)

// Coded is a refusal as "Messages a Person Reads" shapes it: Error is the
// plain reason a person reads by default; the register code and the
// key=value facts are details, read by the register, refusal records,
// --verbose and --json (DetailOf), never printed by default.
type Coded struct {
	Code   string
	Facts  string // key=value words, space separated; may be empty
	Reason error  // the plain reason; it may wrap the cause
}

func (e *Coded) Error() string { return e.Reason.Error() }
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
// "CODE facts: reason".
func (e *Coded) Detail() string {
	return strings.TrimSpace(e.Code+" "+e.Facts) + ": " + e.Reason.Error()
}

// New is a coded refusal whose reason is a plain sentence.
func New(code, facts string, reason error) error {
	return &Coded{Code: code, Facts: facts, Reason: reason}
}

// CodeOf is the refusal code err carries, or the code-first token of a
// legacy message ("CODE ..." or "CODE: ..."), or "".
func CodeOf(err error) string {
	var coded Coder
	if errors.As(err, &coded) {
		return coded.RefusalCode()
	}
	if err == nil {
		return ""
	}
	first := strings.Fields(strings.SplitN(err.Error(), "\n", 2)[0])
	if len(first) == 0 {
		return ""
	}
	token := strings.TrimSuffix(first[0], ":")
	if token == strings.ToUpper(token) && strings.Contains(token, "_") {
		return token
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
