package repoproof

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type archiveWorkspace struct{ root string }

func (bed archiveWorkspace) Workspace() gittree.Workspace { return gittree.Workspace{Dir: bed.root} }
func (bed archiveWorkspace) Close() error                 { return nil }

// RunSection runs the contract's native section adapter in the extracted candidate.
// The archive is already isolated; the section runner must not open a Git worktree.
func RunSection(stdout, stderr io.Writer, id string) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runSection(stdout, stderr, root, id, os.Environ())
}

func runSection(stdout, stderr io.Writer, module, id string, environment []string) int {
	contract, err := testpolicy.Load(filepath.Join(module, "testing.json"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, group := range contract.Groups {
		if group.ID != id || group.Adapter != "section" {
			continue
		}
		logs, err := os.MkdirTemp("", "metasystem-full-section-")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		request := proofrun.TestRunRequest{ProjectRoot: filepath.Dir(module), InstallationPrefix: filepath.Base(module), Contract: contract, Environment: environment, LogRoot: logs, ControlRoot: logs}
		request.WithCandidateOpener(func(root, _ string) (proofrun.CandidateWorkspace, error) { return archiveWorkspace{root}, nil })
		result := proofrun.RunSectionGroup(context.Background(), request, group)
		if result.LogPath != "" {
			data, readErr := os.ReadFile(result.LogPath)
			if readErr != nil {
				fmt.Fprintln(stderr, readErr)
				return 1
			}
			fmt.Fprint(stderr, string(data))
		}
		var report struct {
			Data struct {
				Groups []proofrun.GroupResult `json:"groups"`
			} `json:"data"`
		}
		report.Data.Groups = []proofrun.GroupResult{result}
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if result.Status != "passed" {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stderr, "section group %s is not declared\n", id)
	return 1
}

// Custodian answers the native adapter's helper invocation before proof selection.
func Custodian(args []string, stderr io.Writer) (int, bool) {
	if len(args) < 2 || args[0] != "proof-run" {
		return 0, false
	}
	if args[1] == "custody-exec" {
		return custodyExec(args[2:], stderr), true
	}
	if args[1] != "watchdog" {
		return 0, false
	}
	flags := flag.NewFlagSet("proof-run watchdog", flag.ContinueOnError)
	flags.SetOutput(stderr)
	enabled := flags.Bool("resource-custody", false, "")
	var options proofrun.ResourceCustodyOptions
	var parent identity.Ref
	flags.Int64Var(&parent.Pid, "custody-parent-pid", 0, "")
	flags.Int64Var(&parent.StartedAtSec, "custody-parent-started-at", 0, "")
	flags.Int64Var(&parent.StartedAtUnixMicro, "custody-parent-start-micro", 0, "")
	flags.Int64Var(&parent.StartTicks, "custody-parent-start-ticks", 0, "")
	flags.StringVar(&parent.BootID, "custody-parent-boot-id", "", "")
	flags.IntVar(&options.ControlFD, "custody-control-fd", 0, "")
	flags.IntVar(&options.ReadyFD, "custody-ready-fd", 0, "")
	flags.IntVar(&options.MarkerFD, "custody-marker-fd", 0, "")
	flags.StringVar(&options.MarkerPath, "custody-marker-path", "", "")
	if flags.Parse(args[2:]) != nil || flags.NArg() != 0 || !*enabled {
		return 2, true
	}
	options.Launcher = parent
	if err := proofrun.RunResourceCustodian(options); err != nil {
		fmt.Fprintln(stderr, err)
		return 1, true
	}
	return 0, true
}

func custodyExec(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("proof-run custody-exec", flag.ContinueOnError)
	flags.SetOutput(stderr)
	readyFD := flags.Int("ready-fd", 0, "")
	releaseFD := flags.Int("release-fd", 0, "")
	path := flags.String("path", "", "")
	if flags.Parse(args) != nil || flags.NArg() < 1 || *readyFD < 3 || *releaseFD < 3 || *path == "" {
		return 2
	}
	ready := os.NewFile(uintptr(*readyFD), "custody-exec-ready")
	release := os.NewFile(uintptr(*releaseFD), "custody-exec-release")
	if _, err := ready.Write([]byte("ready\n")); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_ = ready.Close()
	var token [1]byte
	if count, err := release.Read(token[:]); err != nil || count != 1 || token[0] != 1 {
		return 1
	}
	_ = release.Close()
	if err := syscall.Exec(*path, flags.Args(), os.Environ()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
