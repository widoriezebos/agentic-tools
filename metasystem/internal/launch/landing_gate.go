package launch

import "errors"

// LandingKind is the landing agent's launch kind (goal
// landing-lane-runtime-redesign, design section 3).
const LandingKind = "landing"

// LandingRuntimeKey is the roster key that names the landing agent's
// runtime (D2).
const LandingRuntimeKey = "launch.landing.runtime"

// landingRuntimeFix is the one command that puts the landing agent back on
// the runtime its tool gate holds on.
const landingRuntimeFix = "run: metasystem settings set " + LandingRuntimeKey + " claude"

// landingRuntimeRefusal refuses a landing lane on any runtime but claude.
// The landing agent's fail-closed tool gate is a Claude PreToolUse hook
// (internal/landing/agentgate); on codex or devin nothing would gate it, and
// auto may resolve to either.
func landingRuntimeRefusal(kind, runtime string) error {
	if kind != LandingKind || runtime == "claude" {
		return nil
	}
	shown := runtime
	if shown == "" {
		shown = "nothing"
	}
	return errors.New("the landing agent runs only on claude, where its tool gate holds, and " + LandingRuntimeKey + " resolves to " + shown + "\n" + landingRuntimeFix)
}

// refuseUngatedLanding is every non-Claude adapter's refusal of a landing
// record: a record that reached it anyway never becomes an ungated session.
func refuseUngatedLanding(record Record, adapter string) error {
	if record.Kind != LandingKind {
		return nil
	}
	return errors.New("the landing agent runs only on claude, where its tool gate holds, and this launch is on " + adapter + " (" + LandingRuntimeKey + ")\n" + landingRuntimeFix)
}
