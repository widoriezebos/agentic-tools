package partner

// Devin, natively: `devin acp` is an ACP server over stdio, so there is no
// adapter to install and no adapter to pin.
//
// Devin's read-only lever is its permission mode, which is a global flag and
// an environment variable rather than anything on the ACP wire:
//
//   - `--permission-mode auto` auto-approves read-only tools and asks for
//     everything else. `accept-edits` also auto-approves workspace edits,
//     `smart` auto-runs what a fast model judges safe, and `dangerous`
//     auto-approves everything: none of the three is admissible here, and auto
//     is the only mode under which an edit or a command becomes a question
//     this client answers.
//   - The question reaches the client as an ACP permission request, and the
//     permission point refuses it, so auto is read-only in practice: reads are
//     taken, everything else is asked for and denied.
//   - `DEVIN_PERMISSION_MODE=auto` says the same thing to the process the way
//     Devin documents it for a child that inherits the environment, so a
//     wrapper that drops the flag does not silently widen the session.
//   - The flag is placed before the `acp` verb, which is where Devin's global
//     options live.
//
// Devin is signed in or it is not, and this seat never signs it in: a server
// that needs authentication says so at initialize, and that sentence is what
// the drawer shows.

const devinReadOnly = "permission mode auto, so a command becomes a permission request the client refuses; " + confinement

func devinRuntime(argv []string, model string, checkout string) (Runtime, error) {
	// The permission mode is a global option, so it is inserted after the
	// command and before its verbs. A command the seat configured with its own
	// --permission-mode keeps it: the seat's argv is never rewritten, only
	// extended at the one place Devin reads global options from.
	extended := append([]string{}, argv...)
	if !namesFlag(argv, "--permission-mode") {
		extended = append([]string{extended[0], "--permission-mode", "auto"}, extended[1:]...)
	}
	env := []string{"DEVIN_PERMISSION_MODE=auto"}
	if model != "" {
		env = append(env, "DEVIN_MODEL="+model)
	}
	confined, kept := confine(extended, checkout)
	if !kept {
		// Devin under auto wrote the file when it was asked to, so the mode is
		// not the contract this seam needs; without the sandbox there is
		// nothing left to hold it.
		return Runtime{}, unconfined(RuntimeDevin)
	}
	return Runtime{
		Name:     RuntimeDevin,
		Model:    model,
		Argv:     confined,
		Env:      env,
		ReadOnly: devinReadOnly,
		Install:  "",
	}, nil
}

// namesFlag reports whether an argv already carries one flag, in either the
// separated or the joined spelling.
func namesFlag(argv []string, flag string) bool {
	for _, argument := range argv {
		if argument == flag || (len(argument) > len(flag) && argument[:len(flag)+1] == flag+"=") {
			return true
		}
	}
	return false
}
