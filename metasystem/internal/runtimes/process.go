package runtimes

import (
	"fmt"
	"strings"
)

// The runtime process definitions: the one Go source that the
// delegate-supervisor launcher and every process recognizer share (design
// verbs-object-action 6.6). A delegate or host turn used to be a script
// under scripts/agents/adapters or scripts/agents/hosts; it is now the
// engine's delegate-supervisor entry, and the recognizers (lease
// classification, the census, human-authority ancestry, the janitor's kill
// proof) read the signatures and argv shapes declared here instead of
// running or enumerating scripts.

// SupervisorEntry is the argv word of the delegate-supervisor process
// entrypoint: `ENGINE delegate-supervisor RUNTIME VERB [flags]`.
const SupervisorEntry = "delegate-supervisor"

// Supervisor verbs that own a long-lived process: a delegate round
// (dispatch, follow-up) and a mission host turn (start-turn). Each carries
// its instance tag as the value of --instance-tag.
const (
	SupervisorDispatch = "dispatch"
	SupervisorFollowUp = "follow-up"
	SupervisorHostTurn = "start-turn"
	SupervisorTagFlag  = "--instance-tag"
)

// SupervisorArgv is the argument vector of one supervisor process. The
// launcher and the recognizers both build from it, so the argv a launch
// produces is the argv a recognizer expects.
func SupervisorArgv(engine, runtime, verb string, flags ...string) []string {
	return append([]string{engine, SupervisorEntry, runtime, verb}, flags...)
}

// SupervisorShape is one long-lived supervisor argv shape: argv must carry
// every Includes word (exact, or as a path's base name) and the claim tag
// as the value of TagFlag.
type SupervisorShape struct {
	Name     string
	Includes []string
	TagFlag  string
}

// SupervisorShapes returns the supervisor shapes of every declared runtime:
// a dispatch and follow-up shape for each adapter-bearing runtime and a
// start-turn shape for each host-bearing one.
func SupervisorShapes() []SupervisorShape {
	var shapes []SupervisorShape
	for _, d := range declarations {
		if d.HasAdapter {
			for _, verb := range []string{SupervisorDispatch, SupervisorFollowUp} {
				shapes = append(shapes, SupervisorShape{
					Name:     "adapter-supervisor-" + d.Name + "-" + verb,
					Includes: []string{SupervisorEntry, d.Name, verb},
					TagFlag:  SupervisorTagFlag,
				})
			}
		}
		if d.HasHostLauncher {
			shapes = append(shapes, SupervisorShape{
				Name:     "host-" + d.Name + "-" + SupervisorHostTurn,
				Includes: []string{SupervisorEntry, d.Name, SupervisorHostTurn},
				TagFlag:  SupervisorTagFlag,
			})
		}
	}
	return shapes
}

// supervisorExclude keeps the supervisor process itself out of every
// runtime's delegate signature: it launches the CLI, it is not the CLI.
const supervisorExclude = `exclude (^|[[:space:]/])metasystem[[:space:]]+(internal[[:space:]]+)?delegate-supervisor([[:space:]]|$)`

// signatureLines are the census signature declarations, formerly printed
// by each adapter script's `signature` verb.
var signatureLines = map[string][]string{
	"claude": {
		`match ^([^[:space:]]*/)?claude([[:space:]]|$)`,
		`exclude claude-session-signal\.py`,
		`exclude supervision-hook\.sh`,
		supervisorExclude,
	},
	"codex": {
		`match ^([^[:space:]]*/)?codex([[:space:]]|$)`,
		`exclude supervision-hook\.sh`,
		supervisorExclude,
	},
	// The RAW `devin acp` helper is the HOST CLI's own internal child
	// (issue #12: `devin -p` spawns it between the announced main and every
	// tool shell, so the ancestry walk classified the orchestrator DELEGATE
	// and refused every dispatch). It is excluded; the DELEGATE ACP server
	// the supervisor launches carries the distinguishable argv0
	// devin-delegate-acp, matched by its own line, so delegate tool shells
	// still classify DELEGATE.
	"devin": {
		`match ^([^[:space:]]*/)?devin([[:space:]]|$)`,
		`match ^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)`,
		`exclude ^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)`,
		`exclude supervision-hook\.sh`,
		supervisorExclude,
	},
	"fake": {
		`match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)`,
		`exclude supervision-hook\.sh`,
		supervisorExclude,
	},
}

// SignatureText returns a runtime's normalized signature declaration: the
// `match`/`exclude` lines joined by newlines with a trailing newline. An
// undeclared runtime, or one without an adapter, has none.
func SignatureText(runtime string) (string, error) {
	d, ok := Lookup(runtime)
	if !ok || !d.HasAdapter {
		return "", fmt.Errorf("runtime %q declares no delegate signature", runtime)
	}
	lines, ok := signatureLines[runtime]
	if !ok || len(lines) == 0 {
		return "", fmt.Errorf("runtime %q declares no delegate signature", runtime)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// localConfigPaths are the checkout-relative runtime configuration files a
// runtime reads from the project, formerly printed by each adapter's
// `local-config-paths` verb.
var localConfigPaths = map[string][]string{
	"claude": {".claude/settings.json", ".claude/settings.local.json"},
	"codex":  {".codex/config.toml"},
	"devin":  {".devin/config.json", ".devin/config.local.json", ".devin/hooks.v1.json"},
	"fake":   {},
}

// LocalConfigPaths returns a runtime's checkout-relative configuration
// files. ok is false for a runtime without an adapter.
func LocalConfigPaths(runtime string) ([]string, bool) {
	d, found := Lookup(runtime)
	if !found || !d.HasAdapter {
		return nil, false
	}
	return append([]string(nil), localConfigPaths[runtime]...), true
}

// EnforcementMapJSON is the adapter's declared envelope-enforcement map in
// the canonical field order, the literal the capability snapshot and the
// contract emission carry. ok is false for a runtime that declares none
// (fake: its declaration is profile-driven fixture behavior).
func EnforcementMapJSON(runtime string) (string, bool) {
	d, found := Lookup(runtime)
	if !found || d.ExpectedEnvelopeEnforcement == nil {
		return "", false
	}
	parts := make([]string, 0, len(EnforcementFields))
	for _, field := range EnforcementFields {
		parts = append(parts, fmt.Sprintf("%q:%q", field, string(d.ExpectedEnvelopeEnforcement[field])))
	}
	return "{" + strings.Join(parts, ",") + "}", true
}
