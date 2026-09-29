// Command signedinlaunchgit stands in for git inside the signed-in launch
// end-to-end test's detached chain (cmd/metasystem
// signed_in_launch_e2e_test.go), so the real `steward arm` verb runs with real
// Git unavailable.
//
// It answers the two questions the arm asks of Git that decide the ruling —
// the repository top that binds the record's destination, and the configured
// notify command — from the bed's environment, and fails every other call as
// git fails outside a repository. It never runs real Git. Each call is logged
// to SIGNED_IN_LAUNCH_GIT_LOG, one line of arguments per call.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if log := os.Getenv("SIGNED_IN_LAUNCH_GIT_LOG"); log != "" {
		if file, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(file, strings.Join(args, " "))
			_ = file.Close()
		}
	}
	dir := ""
	if len(args) >= 2 && args[0] == "-C" {
		dir, args = args[1], args[2:]
	}
	top := os.Getenv("SIGNED_IN_LAUNCH_GIT_TOP")
	inside := top != "" && dir != "" && (dir == top || strings.HasPrefix(filepath.Clean(dir), top+string(filepath.Separator)))
	switch {
	case inside && len(args) == 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel":
		fmt.Println(top)
		return
	case inside && len(args) == 3 && args[0] == "config" && args[1] == "--get" && args[2] == "metasystem.steward.notify-command":
		if notify := os.Getenv("SIGNED_IN_LAUNCH_GIT_NOTIFY"); notify != "" {
			fmt.Println(notify)
			return
		}
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "fatal: not answered by the signed-in launch stand-in")
	os.Exit(128)
}
