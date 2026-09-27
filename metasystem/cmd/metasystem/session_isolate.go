package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
)

// runSessionIsolate creates an isolated writer worktree for a second session
// (launch.SecondSession): `session isolate [--root INSTALLATION] [NAME]`. It
// prints the command that enters the new checkout.
func runSessionIsolate(args []string) int {
	flags := flag.NewFlagSet("session isolate", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "installation whose checkout the session isolates from (default: this engine's)")
	if flags.Parse(args) != nil || flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal session isolate [--root INSTALLATION] [NAME]")
		return 2
	}
	harness := *root
	if harness == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "second-session:", err)
			return 1
		}
		harness = filepath.Dir(filepath.Dir(exe))
	}
	harness, err := canonicalPath(harness)
	if err != nil {
		fmt.Fprintln(os.Stderr, "second-session:", err)
		return 1
	}
	destination, err := launch.SecondSession(launch.SecondSessionOptions{
		HarnessRoot: harness, Name: flags.Arg(0), Pid: int64(os.Getpid()),
		Git: func(args ...string) (string, error) {
			command := exec.Command("git", args...)
			command.Env = gittree.ScrubbedEnviron()
			command.Stderr = os.Stderr
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
			command.Stdout, command.Stderr = io.Discard, os.Stderr
			return command.Run()
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var refusal *launch.SecondSessionError
		if errors.As(err, &refusal) {
			return refusal.Code
		}
		return 1
	}
	fmt.Printf("cd '%s'\n", destination)
	return 0
}
