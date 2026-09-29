package runtimes

import (
	"fmt"
	"strings"
)

// The registration rows are the canonical tagged-union declaration of every
// runtime's installed artifacts. Host setup executes them; registration/v1,
// the derived dirs view, config validation, and drift checks read the same
// declarations.

// RowOperation is the closed operation vocabulary consumed by host setup.
type RowOperation string

const (
	OpTree          RowOperation = "tree"
	OpCopyFile      RowOperation = "copy-file"
	OpJSONStripKey  RowOperation = "json-strip-key"
	OpSkillProfiles RowOperation = "skill-profiles"
)

// Requiredness is the context-indexed product:
// the template and adopted contexts are independent.
type Requiredness struct {
	TemplateSource     string // required | optional
	AdoptedDestination string // required | source-conditioned | optional
}

// ValidationPolicy is the row's drift judgment.
type ValidationPolicy string

const (
	PolicyExactBytes       ValidationPolicy = "exact-bytes"
	PolicyTransformedBytes ValidationPolicy = "transformed-canonical-bytes"
	PolicyNonDanglingLink  ValidationPolicy = "non-dangling-link"
	PolicyPresenceOnly     ValidationPolicy = "presence-only"
	PolicyInPlaceSource    ValidationPolicy = "in-place-source"
)

// RegistrationRow is one declared artifact.
type RegistrationRow struct {
	ID                 string // stable artifact role
	Operation          RowOperation
	Requiredness       Requiredness
	Destination        string
	Policy             ValidationPolicy
	InstructionBearing bool
	// UncoveredException marks the ONE sanctioned instruction-bearing
	// destination outside every collision root: codex's .codex/hooks.json
	// (any addition is a human-reserved change).
	UncoveredException bool
	Source             string
	Mode               string // link|copy (user-selectable trees), copy|in-place (profiles), "" otherwise
	Key                string // json-strip-key only
}

// registrationRows are each runtime's registrations, written at adoption and
// by runtime setup.
var registrationRows = map[string][]RegistrationRow{
	"claude": {
		{ID: "skill-tree", Operation: OpTree,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".claude/skills", Policy: PolicyNonDanglingLink,
			InstructionBearing: true, Source: "skills", Mode: "link"},
		{ID: "skill-profiles", Operation: OpSkillProfiles,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "source-conditioned"},
			Destination:  ".claude/agents/{skill}.md", Policy: PolicyExactBytes,
			InstructionBearing: true, Source: "skills/{skill}/agents/claude-profile.md", Mode: "copy"},
		{ID: "enforcement-config", Operation: OpJSONStripKey,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".claude/settings.json", Policy: PolicyTransformedBytes,
			InstructionBearing: true, Source: enforcementSourceDir + "/claude-code-hooks.json", Key: "_comment"},
	},
	// Devin discovers project skills under .agents/skills (its skills
	// documentation; the devin selftest probe proves symlinked discovery
	// there), and lists a skill found in two places twice, so the shared
	// tree is its one skill registration.
	"devin": {
		{ID: "skill-tree", Operation: OpTree,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".agents/skills", Policy: PolicyNonDanglingLink,
			InstructionBearing: true, Source: "skills", Mode: "link"},
		{ID: "skill-profiles", Operation: OpSkillProfiles,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "source-conditioned"},
			Destination:  ".devin/agents/{skill}/AGENT.md", Policy: PolicyExactBytes,
			InstructionBearing: true, Source: "skills/{skill}/agents/devin/AGENT.md", Mode: "copy"},
		{ID: "enforcement-config", Operation: OpCopyFile,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".devin/config.json", Policy: PolicyPresenceOnly,
			InstructionBearing: true, Source: enforcementSourceDir + "/devin-hooks.json"},
	},
	"codex": {
		{ID: "skill-tree", Operation: OpTree,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".agents/skills", Policy: PolicyNonDanglingLink,
			InstructionBearing: true, Source: "skills", Mode: "link"},
		{ID: "skill-profiles", Operation: OpSkillProfiles,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "source-conditioned"},
			Destination:  "skills/{skill}/agents/openai.yaml", Policy: PolicyInPlaceSource,
			InstructionBearing: false, Source: "skills/{skill}/agents/openai.yaml", Mode: "in-place"},
		{ID: "enforcement-config", Operation: OpCopyFile,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".codex/hooks.json", Policy: PolicyPresenceOnly,
			InstructionBearing: true, UncoveredException: true,
			Source: enforcementSourceDir + "/codex-hooks.json"},
	},
}

// RegistrationRows returns a runtime's declared rows (nil for
// runtimes with none — fake installs nothing).
func RegistrationRows(runtime string) []RegistrationRow {
	rows := registrationRows[runtime]
	out := make([]RegistrationRow, len(rows))
	copy(out, rows)
	return out
}

// ValidateRegistration checks the row invariants: unique
// (runtime, id), clean fields with no tabs/newlines, legal
// operation/policy combinations, and the collision-root proof — every
// instruction-bearing destination lies beneath a contributed collision
// root unless it carries the one sanctioned exception.
func ValidateRegistration() []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	legalPolicy := map[RowOperation]map[ValidationPolicy]bool{
		OpTree:          {PolicyNonDanglingLink: true, PolicyExactBytes: true},
		OpCopyFile:      {PolicyExactBytes: true, PolicyPresenceOnly: true},
		OpJSONStripKey:  {PolicyTransformedBytes: true},
		OpSkillProfiles: {PolicyExactBytes: true, PolicyInPlaceSource: true},
	}
	roots := CollisionRootsAll()
	for runtime, rows := range registrationRows {
		if !Supported(runtime) {
			add("registration rows declared for unknown runtime %q", runtime)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			if seen[row.ID] {
				add("%s: duplicate artifact role %q", runtime, row.ID)
			}
			seen[row.ID] = true
			for _, field := range []string{row.ID, string(row.Operation), row.Destination, row.Source, row.Mode, row.Key} {
				if strings.ContainsAny(field, "\t\n\r") {
					add("%s/%s: field carries framing bytes", runtime, row.ID)
				}
			}
			if !legalPolicy[row.Operation][row.Policy] {
				add("%s/%s: policy %s is not legal for operation %s", runtime, row.ID, row.Policy, row.Operation)
			}
			if row.UncoveredException && row.Destination != ".codex/hooks.json" {
				add("%s/%s: the uncovered exception is sanctioned only for .codex/hooks.json", runtime, row.ID)
			}
			if row.InstructionBearing && !row.UncoveredException && row.Policy != PolicyInPlaceSource {
				covered := false
				for _, root := range roots {
					if row.Destination == root || strings.HasPrefix(row.Destination, root+"/") {
						covered = true
					}
				}
				if !covered {
					add("%s/%s: instruction-bearing destination %s lies under no contributed collision root", runtime, row.ID, row.Destination)
				}
			}
		}
	}
	return problems
}
