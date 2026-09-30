package humanauthority

import (
	"errors"
	"fmt"
)

// A proof's refusal carries its outcome as data (goal
// error-checks-use-typed-errors): PlainReason, RemedyFor and every caller
// that tells an agent from a missing terminal decide on OutcomeOf, never
// on the words. The words stay the record's text, outcome code first;
// PlainReason is what a person reads.

// OutcomeError is a refusal with its proof outcome and, for an agent in the
// chain, the agent's runtime.
type OutcomeError struct {
	Outcome string
	Runtime string
	text    string
	cause   error
}

func (e *OutcomeError) Error() string { return e.text }
func (e *OutcomeError) Unwrap() error { return e.cause }

// ProofOutcome is the refusal's outcome and agent runtime.
func (e *OutcomeError) ProofOutcome() (string, string) { return e.Outcome, e.Runtime }

// Refused is a refusal with outcome; its words are the outcome, then the
// cause's when there is one, which it wraps.
func Refused(outcome string, cause error) error {
	if cause == nil {
		return &OutcomeError{Outcome: outcome, text: outcome}
	}
	return &OutcomeError{Outcome: outcome, text: outcome + ": " + cause.Error(), cause: cause}
}

// Refusedf is Refused with a cause formatted as fmt.Errorf does (%w wraps).
func Refusedf(outcome, format string, args ...any) error {
	return Refused(outcome, fmt.Errorf(format, args...))
}

// AgentRefused is the refusal of a shell an agent of runtime started.
func AgentRefused(runtime string) error {
	return &OutcomeError{Outcome: OutcomeAgent, Runtime: runtime, text: OutcomeAgent + ": " + runtime}
}

// outcomeCarrier is any refusal that knows its proof outcome.
type outcomeCarrier interface {
	error
	ProofOutcome() (string, string)
}

// OutcomeOf is the proof outcome and agent runtime of the outermost refusal
// in err's chain that carries one, or "" when none does.
func OutcomeOf(err error) (outcome, runtime string) {
	var carrier outcomeCarrier
	if errors.As(err, &carrier) {
		return carrier.ProofOutcome()
	}
	return "", ""
}

// Plain is err in PlainReason's words, still carrying err's outcome for
// errors.As: a caller that shows the plain words and hands the error on
// keeps the decision typed.
func Plain(err error) error {
	if err == nil {
		return nil
	}
	return &plainWords{err: err}
}

type plainWords struct{ err error }

func (p *plainWords) Error() string { return PlainReason(p.err) }
func (p *plainWords) Unwrap() error { return p.err }

// ErrEnrollmentUnnamed refuses a person's act when the enrolled terminal
// has no recorded name.
var ErrEnrollmentUnnamed = errors.New("the enrolled terminal has no recorded name")
