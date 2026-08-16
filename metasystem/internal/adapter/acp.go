package adapter

import (
	"fmt"
	"sort"
)

// ACPDialect is one runtime's ACP translation facts: how the job
// envelope's tools grade maps onto the runtime's session modes.
// The mode IS the enforcement lever on this transport (the wire
// probe proved the permission-request machinery idle in every
// envelope-relevant mode), so the mapping is behavioral evidence,
// never guesswork, and it lives HERE — adapter-owned dialect —
// while internal/runtimes carries only the expected-ACP
// declaration (the registry stays data).
type ACPDialect struct {
	// ModeForTools maps each envelope tools grade to the session
	// mode the adapter sets before the prompt. Every grade on the
	// ordinal scale must be covered; a missing grade would turn
	// into a silent default at dispatch time.
	ModeForTools map[string]string
}

// toolsGrades is the envelope's full tools ordinal; conformance
// holds every dialect to covering all of it.
var toolsGrades = []string{"read-only", "runtime-default"}

// acpDialects is the adapter seam's dialect table. Registration
// happens from per-runtime seam files at init; core code only
// looks up.
var acpDialects = map[string]ACPDialect{}

// RegisterACPDialect registers a runtime's dialect. A duplicate
// runtime or an incomplete grade cover is a declaration bug and
// panics at init.
func RegisterACPDialect(runtime string, dialect ACPDialect) {
	if _, dup := acpDialects[runtime]; dup {
		panic(fmt.Sprintf("acp dialect for %s registered twice", runtime))
	}
	for _, grade := range toolsGrades {
		if dialect.ModeForTools[grade] == "" {
			panic(fmt.Sprintf("acp dialect for %s does not cover tools=%s", runtime, grade))
		}
	}
	acpDialects[runtime] = dialect
}

// ACPDialectFor resolves a runtime's dialect; an undeclared
// runtime is a refusal, never a fallback.
func ACPDialectFor(runtime string) (ACPDialect, error) {
	dialect, ok := acpDialects[runtime]
	if !ok {
		return ACPDialect{}, fmt.Errorf("no acp dialect declared for runtime %s", runtime)
	}
	return dialect, nil
}

// ACPDialectList is the read-only conformance view: runtime names,
// sorted.
func ACPDialectList() []string {
	var names []string
	for runtime := range acpDialects {
		names = append(names, runtime)
	}
	sort.Strings(names)
	return names
}
