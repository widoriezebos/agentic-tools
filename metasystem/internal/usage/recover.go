package usage

import (
	"fmt"
	"sort"
)

// Dead-round usage recovery: the fence
// asks by provider; each per-runtime seam file registers its
// recoverer; a provider without one is honestly unsupported. The
// consumer keeps the measured-count rule: a Recovered outcome
// whose fields are all null normalizes to unavailable
// (fence.go's field count IS the contract), so State alone never
// claims spend.
type RecoveryState string

const (
	Recovered   RecoveryState = "recovered"
	Unavailable RecoveryState = "unavailable"
	Unsupported RecoveryState = "unsupported"
)

// RecoveryContext is the runtime-neutral recovery input: the round's
// artifacts, never a runtime name.
type RecoveryContext struct {
	Repo       string
	RoundDir   string
	EventsPath string
}

// RecoveryOutcome carries the state, the typed fields for the generic
// aggregator, and provenance.
type RecoveryOutcome struct {
	State  RecoveryState
	Fields map[string]any
	Source string
	Detail string
}

// RecoverFn is one runtime's registered dead-round recovery.
type RecoverFn func(RecoveryContext) RecoveryOutcome

var recoverers = map[string]RecoverFn{}

// RegisterRecoverer registers a runtime's recovery; a duplicate key is
// a declaration bug and panics at init.
func RegisterRecoverer(runtime string, fn RecoverFn) {
	if fn == nil {
		panic(fmt.Sprintf("nil usage recoverer for %s", runtime))
	}
	if _, dup := recoverers[runtime]; dup {
		panic(fmt.Sprintf("usage recoverer for %s registered twice", runtime))
	}
	recoverers[runtime] = fn
}

// Recover runs the provider's registered recovery, or reports
// Unsupported when none is declared — honest unavailability, stated
// (devin and every future runtime without a recoverer).
func Recover(provider string, ctx RecoveryContext) RecoveryOutcome {
	fn, ok := recoverers[provider]
	if !ok {
		return RecoveryOutcome{State: Unsupported,
			Detail: "usage recovery is not declared for " + provider}
	}
	return fn(ctx)
}

// RecovererList is the read-only conformance view.
func RecovererList() []string {
	var names []string
	for runtime := range recoverers {
		names = append(names, runtime)
	}
	sort.Strings(names)
	return names
}
