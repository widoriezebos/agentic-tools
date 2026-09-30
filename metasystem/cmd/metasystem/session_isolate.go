package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// runSessionIsolate creates an isolated writer worktree for a second session
// (launch.SecondSession): `session isolate [--root INSTALLATION] [NAME]`. It
// prints the command that enters the new checkout.
func runSessionIsolate(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("session isolate", stdout, stderr)
	root := pathFlag(flags, "root", "", "installation whose checkout the session isolates from (default: this engine's)")
	if flags.Parse(args) != nil || flags.NArg() > 1 {
		return refusePassthrough(stderr, 2, "session isolate takes at most one name; nothing was created",
			textui.Hint{Argv: []string{"metasystem", "session", "isolate"}, Reason: "with a name of its own, or none"})
	}
	failed := func(err error) int {
		return refusePassthrough(stderr, 1, "a second session's checkout cannot be created: "+err.Error(),
			textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
	}
	harness := *root
	if harness == "" {
		exe, err := os.Executable()
		if err != nil {
			return failed(err)
		}
		harness = filepath.Dir(filepath.Dir(exe))
	}
	harness, err := canonicalPath(harness)
	if err != nil {
		return failed(err)
	}
	// Git's own words stay off the page: the refusal says what failed.
	var gitErrors bytes.Buffer
	destination, err := launch.SecondSession(launch.SecondSessionOptions{
		HarnessRoot: harness, Name: flags.Arg(0), Pid: int64(os.Getpid()),
		Git: func(args ...string) (string, error) {
			command := exec.Command("git", args...)
			command.Env = gittree.ScrubbedEnviron()
			command.Stderr = &gitErrors
			output, err := command.Output()
			return string(output), err
		},
		StartedAt: func(pid int64) (int64, error) {
			exact, state, err := (identity.KernelProber{}).Probe(pid)
			if err != nil {
				return 0, err
			}
			if state != identity.Alive {
				return 0, fmt.Errorf("process %d is not alive", pid)
			}
			return exact.StartedAt.Unix(), nil
		},
		Token: func() (string, error) {
			word := make([]byte, 2)
			if _, err := rand.Read(word); err != nil {
				return "", err
			}
			return hex.EncodeToString(word), nil
		},
		Now: time.Now,
		ArmSupervision: func(newHarness string, upArgs []string) error {
			engine := os.Getenv("METASYSTEM_BIN")
			if engine == "" {
				engine = filepath.Join(newHarness, "bin", "metasystem")
			}
			command := exec.Command(engine, append([]string{"up", "--metasystem-root", newHarness}, upArgs...)...)
			command.Stdout, command.Stderr = io.Discard, stderr
			return command.Run()
		},
	})
	var isolated *launch.SecondSessionIsolated
	page := passthroughPage(stdout, "", false)
	if errors.As(err, &isolated) {
		// The named isolation already exists (R-129-ui).
		page.Headline("This session's checkout already exists; nothing to do", page.Env().Path(isolated.Path))
		page.Hint(textui.Hint{Argv: []string{"cd", isolated.Path}, Reason: "enters it"})
		printPage(stdout, page)
		return 0
	}
	if err != nil {
		code := 1
		var refusal *launch.SecondSessionError
		if errors.As(err, &refusal) {
			code = refusal.Code
		}
		return refusePassthrough(stderr, code, strings.TrimPrefix(err.Error(), "second-session: "), textui.Hint{Argv: []string{"metasystem", "session", "isolate", "--repo", "PATH"},
			Reason: "PATH is a checkout MetaSystem is set up in"})
	}
	page.Done("A checkout of its own is ready for a second session")
	page.Hint(textui.Hint{Argv: []string{"cd", destination}, Reason: "enters it"})
	printPage(stdout, page)
	return 0
}
