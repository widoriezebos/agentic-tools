package diskstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// CensusProcess is one live process of the user with what it has open.
type CensusProcess struct {
	Pid        int64    `json:"pid"`
	UID        uint32   `json:"uid"`
	Command    string   `json:"command,omitempty"`
	Cwd        string   `json:"cwd"`
	Executable string   `json:"executable"`
	Files      []string `json:"files,omitempty"`
}

// CensusGap is a live process of the user the census could not read; any gap
// makes the census incomplete, which keeps every worktree store.
type CensusGap struct {
	Pid     int64  `json:"pid"`
	UID     uint32 `json:"uid"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason"`
}

// UseCensus is the machine-wide use census of 3.1, taken once per pass:
// for every live process of the user, its cwd, executable and open files.
type UseCensus struct {
	Taken      bool            `json:"taken"`
	NotTaken   string          `json:"notTaken,omitempty"`
	Processes  []CensusProcess `json:"-"`
	Count      int             `json:"processes"`
	Unreadable []CensusGap     `json:"unreadable,omitempty"`
	seen       map[int64]bool
}

// Complete reports a census that was taken and read every live process.
func (c *UseCensus) Complete() bool { return c != nil && c.Taken && len(c.Unreadable) == 0 }

// CensusReader is the kernel seam of the census; tests fake it.
type CensusReader struct {
	UID  uint32
	Pids func() ([]int64, error)
	// ProcessUID answers false for a process that is gone.
	ProcessUID func(pid int64) (uint32, bool)
	Use        func(pid int64) (identity.ProcessUse, error)
	Command    func(pid int64) string
}

// KernelCensusReader reads this host's processes for the user uid.
func KernelCensusReader(uid uint32) CensusReader {
	prober := identity.KernelProber{}
	return CensusReader{
		UID: uid, Pids: identity.AllPids, ProcessUID: identity.ProcessUID, Use: identity.ReadProcessUse,
		Command: func(pid int64) string {
			argv, known := prober.ReadArgv(pid)
			if !known {
				return ""
			}
			command := strings.Join(argv, " ")
			if len(command) > 200 {
				command = command[:200] + "…"
			}
			return command
		},
	}
}

// TakeUseCensus reads every live process of the reader's user. A process
// that is gone by the time it is read is skipped; one that is alive and
// cannot be read is a gap. The context bounds the walk: a cut-short census
// is not taken.
func TakeUseCensus(ctx context.Context, reader CensusReader) UseCensus {
	census := UseCensus{seen: map[int64]bool{}}
	pids, err := reader.Pids()
	if err != nil {
		return UseCensus{NotTaken: "process table unreadable: " + err.Error()}
	}
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return UseCensus{NotTaken: "use census did not finish within the pass budget"}
		}
		census.read(reader, pid)
	}
	census.Taken = true
	census.Count = len(census.Processes)
	return census
}

func (c *UseCensus) read(reader CensusReader, pid int64) {
	c.seen[pid] = true
	uid, alive := reader.ProcessUID(pid)
	if !alive || uid != reader.UID {
		return
	}
	use, err := reader.Use(pid)
	if err != nil {
		// A process that ended while it was read is not a gap.
		if _, still := reader.ProcessUID(pid); !still {
			return
		}
		c.Unreadable = append(c.Unreadable, CensusGap{Pid: pid, UID: uid, Command: reader.Command(pid), Reason: err.Error()})
		return
	}
	c.Processes = append(c.Processes, CensusProcess{Pid: pid, UID: uid, Command: reader.Command(pid),
		Cwd: use.Cwd, Executable: use.Executable, Files: use.Files})
}

// ReadNew reads every live process the census did not see: the processes
// started after it, read fresh inside a store's critical section (3.1 step
// 3).
func (c *UseCensus) ReadNew(ctx context.Context, reader CensusReader) error {
	pids, err := reader.Pids()
	if err != nil {
		return fmt.Errorf("process table unreadable: %w", err)
	}
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !c.seen[pid] {
			c.read(reader, pid)
		}
	}
	c.Count = len(c.Processes)
	return nil
}

// Holders are the processes whose cwd, executable or an open file lies at
// or under path.
func (c *UseCensus) Holders(path string) []CensusProcess {
	var holders []CensusProcess
	for _, process := range c.Processes {
		for _, used := range append([]string{process.Cwd, process.Executable}, process.Files...) {
			if under(used, path) {
				holders = append(holders, process)
				break
			}
		}
	}
	sort.Slice(holders, func(i, j int) bool { return holders[i].Pid < holders[j].Pid })
	return holders
}

// under reports whether candidate is path or lies inside it, comparing
// cleaned absolute paths (and their resolved forms, so /private/var and
// /var name one tree).
func under(candidate, path string) bool {
	if candidate == "" || path == "" {
		return false
	}
	for _, left := range spellings(candidate) {
		for _, right := range spellings(path) {
			if left == right || strings.HasPrefix(left, right+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

func spellings(path string) []string {
	clean := filepath.Clean(path)
	forms := []string{clean}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != clean {
		forms = append(forms, resolved)
	}
	if trimmed, ok := strings.CutPrefix(clean, "/private"); ok && strings.HasPrefix(trimmed, "/") {
		forms = append(forms, trimmed)
	} else if strings.HasPrefix(clean, "/var/") || strings.HasPrefix(clean, "/tmp/") || clean == "/tmp" {
		forms = append(forms, "/private"+clean)
	}
	return forms
}

// GapLines names the unreadable processes for the report and a kept item.
func (c *UseCensus) GapLines() []string {
	var lines []string
	for _, gap := range c.Unreadable {
		line := fmt.Sprintf("pid %d (uid %d", gap.Pid, gap.UID)
		if gap.Command != "" {
			line += ", " + gap.Command
		}
		lines = append(lines, line+"): "+gap.Reason)
	}
	return lines
}
