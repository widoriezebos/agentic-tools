package runtimes

import (
	"fmt"
	"strings"
)

// The registration rows: the canonical tagged-union declaration of every
// runtime's installed artifacts, transcribed verbatim from adopt.sh's
// arms. Today
// the rows are VALIDATION DATA — the registration/v1 wire, the derived
// dirs view, config validation's presence checks, and the declared
// drift policies all read them; a future installer
// will execute them.

// RowOperation is the closed operation vocabulary; the installer union
// arm lands with the installer.
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

// registrationRows transcribes adopt.sh:292-345 per runtime.
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
			InstructionBearing: true, Source: "scripts/enforcement/claude-code-hooks.json", Key: "_comment"},
	},
	"devin": {
		{ID: "skill-tree", Operation: OpTree,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".agents/skills", Policy: PolicyNonDanglingLink,
			InstructionBearing: true, Source: "skills", Mode: "link"},
		{ID: "skill-tree-devin", Operation: OpTree,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".devin/skills", Policy: PolicyNonDanglingLink,
			InstructionBearing: true, Source: "skills", Mode: "link"},
		{ID: "skill-profiles", Operation: OpSkillProfiles,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "source-conditioned"},
			Destination:  ".devin/agents/{skill}/AGENT.md", Policy: PolicyExactBytes,
			InstructionBearing: true, Source: "skills/{skill}/agents/devin/AGENT.md", Mode: "copy"},
		{ID: "enforcement-config", Operation: OpCopyFile,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".devin/config.json", Policy: PolicyPresenceOnly,
			InstructionBearing: true, Source: "scripts/enforcement/devin-hooks.json"},
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
			Source: "scripts/enforcement/codex-hooks.json"},
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

// RegistrationV1 encodes a runtime's rows in the pinned wire format:
// header line `registration/v1`, one row per line, twelve
// tab-separated columns [id, operation, templateSource,
// adoptedDestination, destination, policy, collisionClass,
// uncoveredException, source, mode, key, handlerId], `-` for unused,
// trailing newline; zero rows = header only.
func RegistrationV1(runtime string) string {
	var b strings.Builder
	b.WriteString("registration/v1\n")
	for _, row := range registrationRows[runtime] {
		class := "plain"
		if row.InstructionBearing {
			class = "instruction-bearing"
		}
		exception := "false"
		if row.UncoveredException {
			exception = "true"
		}
		columns := []string{
			row.ID, string(row.Operation),
			row.Requiredness.TemplateSource, row.Requiredness.AdoptedDestination,
			row.Destination, string(row.Policy), class, exception,
			dash(row.Source), dash(row.Mode), dash(row.Key), "-",
		}
		b.WriteString(strings.Join(columns, "\t"))
		b.WriteString("\n")
	}
	return b.String()
}

func dash(value string) string {
	if value == "" {
		return "-"
	}
	return value
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
