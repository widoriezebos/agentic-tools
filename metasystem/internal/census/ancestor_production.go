package census

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// nativeProcTree reads a process's tree facts from the kernel: argv from the
// prober, the group from getpgid, and the parent from the proc_info parent
// query. A pid that is gone or unreadable stops the walk.
type nativeProcTree struct{}

func (nativeProcTree) Info(pid int64) (ProcInfo, bool) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return ProcInfo{}, false
	}
	command := strings.Join(exact.Argv, " ")
	if command == "" {
		return ProcInfo{}, false
	}
	pgid, _ := unix.Getpgid(int(pid))
	ppid, ok := identity.ParentPid(pid)
	if !ok {
		ppid = 1 // no distinct ancestor: classify this level, then stop
	}
	return ProcInfo{PPID: ppid, PGID: int64(pgid), Command: command}, true
}

// FindAncestorProduction walks the live process tree from pid and returns the
// first ancestor whose argv matches a runtime signature — the agent session a
// delegate or wrapper runs under. A fake-agent ancestor may be pinned by
// environment for the fake runtime.
func FindAncestorProduction(metasystemRoot string, pid int64, runtime string) (map[string]any, error) {
	if fake := os.Getenv("METASYSTEM_FAKE_AGENT_ANCESTOR_PID"); fake != "" && runtime == "fake" {
		if candidate, err := strconv.ParseInt(fake, 10, 64); err == nil {
			pgid, _ := unix.Getpgid(int(candidate))
			return map[string]any{
				"pid": candidate, "pidStartedAt": startedSeconds(candidate),
				"pgid": int64(pgid), "runtime": "fake", "argv": "metasystem-fake-agent",
			}, nil
		}
	}
	signatures, err := signaturesFor(metasystemRoot, runtime)
	if err != nil {
		return nil, err
	}
	ancestor, err := FindAncestor(nativeProcTree{}, pid, signatures)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"pid": ancestor.Pid, "pidStartedAt": startedSeconds(ancestor.Pid),
		"pgid": ancestor.PGID, "runtime": ancestor.Runtime, "argv": ancestor.Argv,
	}, nil
}

func startedSeconds(pid int64) int64 {
	seconds, _, _ := kernelProbe(pid)
	return seconds
}

// signaturesFor builds the runtime signatures to match against: just the named
// runtime when one is given, otherwise every configured runtime.
func signaturesFor(metasystemRoot, only string) ([]Signature, error) {
	var selected []string
	if only != "" {
		selected = []string{only}
	} else {
		confPath := filepath.Join(metasystemRoot, "metasystem.conf")
		selected = splitRuntimes(config.ConfValue(confPath, "metasystem.runtimes", ""))
		if len(selected) == 0 {
			return nil, fmt.Errorf("%s lists no metasystem.runtimes", confPath)
		}
	}
	var out []Signature
	for _, runtime := range selected {
		adapter := filepath.Join(metasystemRoot, "scripts", "agents", "adapters", runtime+".sh")
		info, err := os.Stat(adapter)
		if err != nil || info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("runtime %q signature adapter is missing or not executable: %s", runtime, adapter)
		}
		text, err := SignatureText(adapter)
		if err != nil {
			return nil, err
		}
		matches, excludes := ParseSignatureText(text)
		sig, err := CompileSignature(runtime, matches, excludes)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, nil
}
