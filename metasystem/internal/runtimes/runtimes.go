// Package runtimes is the ONE declaration of the agent-runtime universe.
// It is pure data and a dependency
// leaf: config, validate, audit, missionrunner, and cmd consume it;
// nothing here imports the rest of the tree, and no behavior lives here
// — behavioral capabilities register seam-locally in their owner
// packages (host, usage, adapter) and this package only declares which
// capabilities each runtime is EXPECTED to provide. Shell never parses
// this package's data directly; plumbing asks the `metasystem runtime`
// verbs. Adding a runtime is one Declaration here plus that runtime's
// seam files — the two declared exceptions are role-file waiver edits
// (a human security decision) and the handwritten conformance-evidence
// rows in docs/design/turn-verdict-delivery-contract.md.
package runtimes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Enforcement is the exact live snapshot enum (adapter/snapshot.go
// accepts precisely these two values).
type Enforcement string

const (
	Mapped      Enforcement = "mapped"
	NotEnforced Enforcement = "notEnforced"
)

// EnforcementFields is the exact envelope field set, in canonical order.
var EnforcementFields = []string{"writeRoots", "readRoots", "network"}

// LiveSelfCheck declares a runtime's live repository self-check: the
// vendored-entry marker `hooks check` requires inside the supervision
// hook commands. Paths stay explicit CLI arguments (the suite's
// nested-template case resolves live settings in the parent repo).
type LiveSelfCheck struct {
	VendoredMarker string
}

// Declaration is one runtime's complete registry entry.
type Declaration struct {
	Name            string
	HasAdapter      bool
	HasHostLauncher bool
	// Adoptable gates scripts/adopt.sh's population; fake is never
	// adoptable. AdoptionDefault marks the one default (claude).
	Adoptable       bool
	AdoptionDefault bool
	// TailoringPriority pins conf tailoring's default-runtime
	// precedence: the LOWEST selected priority wins (codex 1, devin 2,
	// claude 3, fake 4 — fake never outranks a real runtime).
	TailoringPriority int
	// SynthesizedModel is the fixed model name tailoring materializes
	// for a synthetic runtime ("fake-model"); empty for real runtimes.
	SynthesizedModel string
	// SessionEnv is the project-directory environment variable the
	// runtime exports into sessions ("" when it has none). The grammar
	// is pinned so shell consumers can expand it indirectly, never eval.
	SessionEnv string
	// InstructionFile is the runtime's instruction-bearing filename at
	// a repository root.
	InstructionFile string
	// RegistrationDirs is the adopted-repository directory view (the
	// `runtime dirs` verb; source of config validation's presence
	// checks). The registration rows are to become this field's source
	// of truth; until then it mirrors today's table.
	RegistrationDirs []string
	// ShippedEnforcementConfig is the scripts/enforcement filename this
	// runtime ships ("" when none). Independent of LiveSelfCheck.
	ShippedEnforcementConfig string
	// SelfCheck declares the live repository self-check (claude only
	// today); nil when the runtime has none.
	SelfCheck *LiveSelfCheck
	// ConfigIdentityFilter is the scripts/agents/adapters filter
	// filename config identity hashes ("" when none — fake).
	ConfigIdentityFilter string
	// ExpectedEnvelopeEnforcement is the static declaration the suite
	// asserts against the adapter's snapshot shape, over exactly
	// EnforcementFields. Nil for fake, whose declaration is
	// profile-driven fixture behavior.
	ExpectedEnvelopeEnforcement map[string]Enforcement
	// PermissionResiduals maps a permission FIELD to the globally
	// unique residual identifier a role file may waive under that same
	// field. Residuals govern live selection only (the unverified-field
	// path); they are NOT derived from ExpectedEnvelopeEnforcement.
	PermissionResiduals map[string]string
	// ExpectedCapabilities names the behavioral capabilities this
	// runtime's seam files must register (conformance joins these
	// against each owner table's list view, both ways).
	ExpectedCapabilities []string
	// SignatureVectors are the provider-owned conformance vectors the
	// S4-7 fixture consumes: the honest
	// positive process word and a lookalike that must stay out.
	SignatureVectors SignatureVectors
	// CommonLifecycleAdapter marks adapters sharing the common
	// initializer/writer source shape (runtime-common.sh); fake
	// deliberately does not. Independent of the static
	// enforcement map.
	CommonLifecycleAdapter bool
	// CollisionRoots are this runtime's contributed adoption collision
	// roots — scanned as the deduplicated FULL population regardless of
	// selection (the installer consumes them, the
	// verb transports them).
	CollisionRoots []string
	// ExpectedACP declares that this runtime is expected to support
	// the ACP transport, with the protocol version selection pins
	// pre-launch (this registry stays data
	// only). The dialect — mode mappings, launch argv — is
	// adapter-owned; conformance joins the two both ways.
	ExpectedACP *ACPExpectation
}

// ACPExpectation is the data half of a runtime's ACP support.
type ACPExpectation struct {
	ExpectedProtocolVersion int64
	// ExpectedCapabilities is the registry's expectation of the
	// NATIVE DRIVER's capability declaration for this runtime over
	// the acp transport — the seam's registration joins the driver's
	// claim against this exactly, both ways (a mismatch is an
	// init-time panic). Field names mirror the delegate seam's
	// Declaration one-for-one; the mirror is test-pinned from the
	// delegate side. Nil means no native driver is expected yet.
	// NOTE the surface: the adapter probe's snapshot describes the
	// SHELL adapter's offering and may honestly differ (the recorded
	// convergence residue on acp-adapter-seam).
	ExpectedCapabilities *ACPCapabilities
}

// ACPCapabilities mirrors delegate.Declaration as pure data (this
// package stays a dependency leaf and imports nothing of the seam).
type ACPCapabilities struct {
	Resume                   bool
	SessionEstablishedSignal bool
	NativeStructuredOutput   bool
	NativeEvents             bool
	NativeUsage              bool
	GracefulCancel           bool
	ProtocolServer           bool
	NativeBudget             bool
}

// SignatureVectors is one runtime's positive/lookalike pair.
type SignatureVectors struct {
	Positive  string
	Lookalike string
}

// Capability names (the declaration side of the seam-local tables).
const (
	CapDeliveryRecollection = "delivery-recollection"
	CapUsageRecovery        = "usage-recovery"
	CapSelfTestProbe        = "selftest-probe"
)

// nameRe is the shell-safe runtime-name grammar.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// envRe is the session-environment variable grammar (indirect
// expansion only, never eval).
var envRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// declarations is the universe, in tailoring-priority order.
var declarations = []Declaration{
	{
		Name: "codex", HasAdapter: true, HasHostLauncher: true,
		Adoptable: true, TailoringPriority: 1,
		SignatureVectors:         SignatureVectors{Positive: "codex", Lookalike: "metasystem-codex-lookalike"},
		CommonLifecycleAdapter:   true,
		CollisionRoots:           []string{".agents"},
		InstructionFile:          "AGENTS.md",
		RegistrationDirs:         []string{".agents/skills"},
		ShippedEnforcementConfig: "codex-hooks.json",
		ConfigIdentityFilter:     "codex-config-filter.v1.json",
		ExpectedEnvelopeEnforcement: map[string]Enforcement{
			"writeRoots": Mapped, "readRoots": NotEnforced, "network": Mapped,
		},
		ExpectedCapabilities: []string{CapUsageRecovery},
	},
	{
		Name: "devin", HasAdapter: true, HasHostLauncher: true,
		Adoptable: true, TailoringPriority: 2,
		// The lookalike IS the host CLI's internal ACP helper (issue
		// #12): the vector pins the exclusion that keeps a Devin-hosted
		// orchestrator classified MAIN.
		SignatureVectors:         SignatureVectors{Positive: "devin", Lookalike: "devin acp"},
		CommonLifecycleAdapter:   true,
		CollisionRoots:           []string{".agents", ".devin"},
		SessionEnv:               "DEVIN_PROJECT_DIR",
		InstructionFile:          "AGENTS.md",
		RegistrationDirs:         []string{".agents/skills", ".devin/skills", ".devin/agents"},
		ShippedEnforcementConfig: "devin-hooks.json",
		ConfigIdentityFilter:     "devin-config-filter.v1.json",
		// Measured truth, not a weakening: devin has been observed
		// writing and reading outside
		// the declared roots.
		ExpectedEnvelopeEnforcement: map[string]Enforcement{
			"writeRoots": NotEnforced, "readRoots": NotEnforced, "network": NotEnforced,
		},
		PermissionResiduals: map[string]string{
			"readRoots":  "devin-read-roots-unenforced",
			"writeRoots": "devin-write-roots-unenforced",
		},
		ExpectedCapabilities: []string{CapDeliveryRecollection, CapSelfTestProbe},
		// Protocol 1 is verified live at devin 3000.4.25. The
		// capability row is the native driver's declared surface
		// (plans/acp-seam-s2-design.md, "The declaration, earned").
		ExpectedACP: &ACPExpectation{
			ExpectedProtocolVersion: 1,
			ExpectedCapabilities: &ACPCapabilities{
				Resume:                   true,
				SessionEstablishedSignal: true,
				NativeEvents:             true,
				NativeUsage:              true,
				GracefulCancel:           true,
				ProtocolServer:           true,
			},
		},
	},
	{
		Name: "claude", HasAdapter: true, HasHostLauncher: true,
		Adoptable: true, AdoptionDefault: true, TailoringPriority: 3,
		SignatureVectors:         SignatureVectors{Positive: "claude", Lookalike: "metasystem-claude-lookalike"},
		CommonLifecycleAdapter:   true,
		CollisionRoots:           []string{".claude"},
		SessionEnv:               "CLAUDE_PROJECT_DIR",
		InstructionFile:          "CLAUDE.md",
		RegistrationDirs:         []string{".claude/skills", ".claude/agents"},
		ShippedEnforcementConfig: "claude-code-hooks.json",
		SelfCheck:                &LiveSelfCheck{VendoredMarker: "$CLAUDE_PROJECT_DIR/metasystem"},
		ConfigIdentityFilter:     "claude-config-filter.v1.json",
		ExpectedEnvelopeEnforcement: map[string]Enforcement{
			"writeRoots": Mapped, "readRoots": Mapped, "network": Mapped,
		},
		ExpectedCapabilities: []string{CapUsageRecovery},
	},
	{
		Name: "fake", HasAdapter: true, HasHostLauncher: true,
		TailoringPriority: 4, SynthesizedModel: "fake-model",
		InstructionFile:  "AGENTS.md",
		SignatureVectors: SignatureVectors{Positive: "metasystem-fake-agent", Lookalike: "metasystem-fake-lookalike"},
		// The fixture harness's own declared gap: the unverified-network
		// profile reports network unverified, and the waiver machinery's
		// fixtures exercise exactly this residual.
		PermissionResiduals: map[string]string{"network": "fake-network-unverified"},
	},
}

// All returns every declaration in tailoring-priority order.
func All() []Declaration {
	out := make([]Declaration, len(declarations))
	copy(out, declarations)
	return out
}

// Names returns every runtime name in tailoring-priority order.
func Names() []string {
	names := make([]string, 0, len(declarations))
	for _, d := range declarations {
		names = append(names, d.Name)
	}
	return names
}

// Lookup returns the declaration for name.
func Lookup(name string) (Declaration, bool) {
	for _, d := range declarations {
		if d.Name == name {
			return d, true
		}
	}
	return Declaration{}, false
}

// Supported reports whether name is a declared runtime. The adoption
// sentinel "none" is NOT a runtime and never appears here — its
// exclusivity and empty-roster semantics live at the adoption and
// tailoring boundaries.
func Supported(name string) bool {
	_, ok := Lookup(name)
	return ok
}

// Adoptable returns the adoptable runtime names in priority order.
func Adoptable() []string {
	var names []string
	for _, d := range declarations {
		if d.Adoptable {
			names = append(names, d.Name)
		}
	}
	return names
}

// AdoptionDefault returns the one default adoption runtime.
func AdoptionDefault() string {
	for _, d := range declarations {
		if d.AdoptionDefault {
			return d.Name
		}
	}
	return ""
}

// DefaultFor returns the strongest (lowest-priority-number) runtime in
// the selected set — conf tailoring's default-runtime rule.
func DefaultFor(selected map[string]bool) string {
	best, bestPriority := "", 1<<30
	for _, d := range declarations {
		if selected[d.Name] && d.TailoringPriority < bestPriority {
			best, bestPriority = d.Name, d.TailoringPriority
		}
	}
	return best
}

// InstructionFiles returns the deduplicated set of declared
// instruction filenames, sorted — every consumer (audit inventory,
// outside-reference scan roots, conformance protection, adoption
// collision and payload sets) builds from this one list.
func InstructionFiles() []string {
	set := map[string]bool{}
	for _, d := range declarations {
		if d.InstructionFile != "" {
			set[d.InstructionFile] = true
		}
	}
	files := make([]string, 0, len(set))
	for f := range set {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// WithAdapter returns the runtime names declaring an adapter, in
// priority order; WithHost and WithCommonLifecycle filter likewise.
func WithAdapter() []string { return filterNames(func(d Declaration) bool { return d.HasAdapter }) }

func WithHost() []string {
	return filterNames(func(d Declaration) bool { return d.HasHostLauncher })
}

func WithCommonLifecycle() []string {
	return filterNames(func(d Declaration) bool { return d.CommonLifecycleAdapter })
}

func filterNames(keep func(Declaration) bool) []string {
	var names []string
	for _, d := range declarations {
		if keep(d) {
			names = append(names, d.Name)
		}
	}
	return names
}

// CollisionRootsAll returns the deduplicated FULL population of
// contributed collision roots, sorted — the scan never narrows to the
// selected runtimes.
func CollisionRootsAll() []string {
	set := map[string]bool{}
	for _, d := range declarations {
		for _, root := range d.CollisionRoots {
			set[root] = true
		}
	}
	roots := make([]string, 0, len(set))
	for root := range set {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

// ResidualFor returns the declared residual identifier for a runtime's
// permission field ("" when the runtime declares no residual there —
// the fail-closed case).
func ResidualFor(runtime, field string) string {
	d, ok := Lookup(runtime)
	if !ok || d.PermissionResiduals == nil {
		return ""
	}
	return d.PermissionResiduals[field]
}

// Validate checks the whole universe's declaration invariants; the
// conformance test runs it, so a hostile or sloppy declaration cannot
// land. It returns every problem, not the first.
func Validate() []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	priorities := map[int]string{}
	residuals := map[string]string{}
	names := map[string]bool{}
	defaults := 0
	lastPriority := 0
	for _, d := range declarations {
		if !nameRe.MatchString(d.Name) {
			add("runtime name %q violates the shell-safe grammar", d.Name)
		}
		if names[d.Name] {
			add("runtime name %q declared twice", d.Name)
		}
		names[d.Name] = true
		if d.TailoringPriority <= 0 {
			add("%s: tailoring priority must be positive", d.Name)
		}
		if d.TailoringPriority <= lastPriority {
			add("%s: declarations must be in ascending priority order (Names promises it)", d.Name)
		}
		lastPriority = d.TailoringPriority
		if d.SelfCheck != nil && d.SelfCheck.VendoredMarker == "" {
			add("%s: a live self-check requires a nonblank vendored marker", d.Name)
		}
		if d.SessionEnv != "" && !envRe.MatchString(d.SessionEnv) {
			add("%s: session env %q violates the variable grammar", d.Name, d.SessionEnv)
		}
		if d.ExpectedACP != nil && d.ExpectedACP.ExpectedProtocolVersion < 1 {
			add("%s: an ACP expectation requires a positive protocol version", d.Name)
		}
		if previous, taken := priorities[d.TailoringPriority]; taken {
			add("%s: tailoring priority %d already belongs to %s", d.Name, d.TailoringPriority, previous)
		}
		priorities[d.TailoringPriority] = d.Name
		if d.AdoptionDefault {
			defaults++
			if !d.Adoptable {
				add("%s: adoption default must be adoptable", d.Name)
			}
		}
		if d.Adoptable && d.SynthesizedModel != "" {
			add("%s: a synthesized-model runtime is never adoptable", d.Name)
		}
		for _, relative := range d.RegistrationDirs {
			if !cleanRelative(relative) {
				add("%s: registration dir %q is not clean-relative", d.Name, relative)
			}
		}
		for _, emitted := range []string{d.ShippedEnforcementConfig, d.ConfigIdentityFilter, d.InstructionFile} {
			if emitted != "" && !cleanRelative(emitted) {
				add("%s: declared path %q is not clean-relative", d.Name, emitted)
			}
		}
		if d.HasAdapter && (d.SignatureVectors.Positive == "" || d.SignatureVectors.Lookalike == "") {
			add("%s: an adapter-bearing runtime requires signature vectors", d.Name)
		}
		for _, root := range d.CollisionRoots {
			if !strings.HasPrefix(root, ".") || !cleanRelative(root) {
				add("%s: collision root %q must be a clean dot-prefixed path", d.Name, root)
			}
		}
		if d.ExpectedEnvelopeEnforcement != nil {
			if len(d.ExpectedEnvelopeEnforcement) != len(EnforcementFields) {
				add("%s: envelope enforcement must cover exactly %v", d.Name, EnforcementFields)
			}
			for _, field := range EnforcementFields {
				value, present := d.ExpectedEnvelopeEnforcement[field]
				if !present {
					add("%s: envelope enforcement misses %s", d.Name, field)
				} else if value != Mapped && value != NotEnforced {
					add("%s: envelope enforcement %s=%q outside the live enum", d.Name, field, value)
				}
			}
		}
		for field, id := range d.PermissionResiduals {
			if id == "" {
				add("%s: empty residual identifier for %s", d.Name, field)
			}
			if owner, taken := residuals[id]; taken {
				add("residual identifier %q claimed by both %s and %s", id, owner, d.Name)
			}
			residuals[id] = d.Name
			found := false
			for _, known := range EnforcementFields {
				if field == known {
					found = true
				}
			}
			if !found {
				add("%s: residual field %q is not a permission field", d.Name, field)
			}
		}
	}
	if defaults != 1 {
		add("exactly one adoption default required, found %d", defaults)
	}
	return problems
}

func cleanRelative(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") {
		return false
	}
	if strings.ContainsAny(path, "\t\n\r ") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}

// OverrideForTest swaps the declaration universe and returns a restore
// function. TESTS ONLY — the conformance suites use it to prove a
// newly declared runtime's assets reach every consumer (audit
// inventory, scan roots, the no-waiver set) without editing core.
func OverrideForTest(replacement []Declaration) (restore func()) {
	saved := declarations
	declarations = replacement
	return func() { declarations = saved }
}
