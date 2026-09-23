package partner

import (
	"fmt"
	"runtime"
	"strconv"
)

// Confinement: the half of read-only that the runtime cannot be trusted to
// supply itself.
//
// Each runtime below names the options that make its session read-only, and
// those options matter: they decide which tools exist at all, whether the
// network is reachable, whether an MCP server joins, and whether an approval a
// human once gave their own terminal still applies here. But two of the three
// were asked, live, to create a file in this checkout and did it, with no
// permission request reaching the client:
//
//   - Codex's adapter names a preset `read-only` whose seatbelt policy is of
//     kind workspaceWrite, which makes the session's working directory — this
//     checkout — writable with no approval at all.
//   - Devin under `--permission-mode auto`, documented as auto-approving
//     read-only tools, wrote the file and only asked about the command.
//
// A claim the runtime does not keep is worse than no claim, so the seat keeps
// it instead: every runtime is started inside the operating system's own
// sandbox with writes to the checkout denied. The write then fails below every
// protocol, which is where the design puts enforcement, and the attempt
// surfaces as the escalation the permission point refuses and the drawer
// shows.
//
// macOS has such a sandbox and it needs no privilege and no daemon. A platform
// without one admits only the runtimes that establish read-only by themselves;
// the rest are refused by name, because a Partner that says it is read-only
// and is not is the one outcome this design does not allow.

// confine wraps one command so that it cannot write anywhere inside root, and
// reports whether this platform could do it.
func confine(argv []string, root string) ([]string, bool) {
	if runtime.GOOS != "darwin" || len(argv) == 0 || root == "" {
		return argv, false
	}
	// The profile is the whole rule: everything a process may normally do,
	// minus every write beneath the checkout. The path is quoted as a Go
	// string, which is the same escaping the profile language takes for the
	// characters a path can hold.
	profile := "(version 1)(allow default)(deny file-write* (subpath " + strconv.Quote(root) + "))"
	return append([]string{sandboxExec, "-p", profile}, argv...), true
}

// sandboxExec is the platform's sandbox launcher. It is a variable so that a
// test can see what the seam builds without running it.
var sandboxExec = "/usr/bin/sandbox-exec"

// unconfined is the refusal a runtime gets when it cannot keep the contract
// itself and this platform cannot keep it for the runtime.
func unconfined(name string) error {
	return fmt.Errorf(
		"the %s runtime does not enforce read-only by itself — asked to, it writes in the checkout — and %s offers no sandbox this seat can start it in; set ui.partner.runtime to a runtime that enforces read-only itself",
		name, runtime.GOOS)
}

// confinement is the sentence each confined runtime adds to its own read-only
// line, so that what the seat actually did is readable rather than implied.
const confinement = "and the process is started inside the operating system's sandbox with every write to the checkout denied"
