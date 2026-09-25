package launch

// The host as this verb really finds it, and the runner that really spawns
// the steps.
//
// Everything here is the concrete side of the seams the sequencer is written
// against. None of it decides anything: it opens files, resolves paths, reads
// the stamp out of an engine, asks the steward what it last recorded, and
// runs one bounded subprocess on an owned process group with the launching
// process's METASYSTEM_* variables removed.

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// OSHost answers the sequencer's questions from this host.
type OSHost struct{}

// Exists reports whether a path is there. An error that is not "absent" is
// carried: a directory that cannot be read is not a directory that is free.
func (OSHost) Exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Canonical resolves a path's symlinks.
func (OSHost) Canonical(path string) (string, error) { return filepath.EvalSymlinks(path) }

// MakeDir creates a directory and says whether this call created it, which is
// what the evidence root's own refusal turns on.
func (OSHost) MakeDir(path string) (bool, error) {
	err := os.Mkdir(path, 0o755)
	if err == nil {
		return true, nil
	}
	if os.IsExist(err) {
		return false, nil
	}
	return false, err
}

// CopyLocalConf copies one seat's metasystem.conf.local to another as bytes
// and rewrites the one key this verb knows is wrong for a second machine.
//
// The bytes are never parsed here: the file carries this fleet's roster and
// the channel's secret, and a verb that read them would be a verb that could
// leak them. The rewrite goes through the engine's own conf-key writer on a
// temporary copy, and the result is published with one rename, so no half
// written configuration is ever visible at the destination.
func (OSHost) CopyLocalConf(source, destination, evidenceRoot string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(destination), ".metasystem.conf.local.")
	if err != nil {
		return err
	}
	name := staged.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := staged.Write(data); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := validate.SetConfKeys(name, []validate.ConfSetting{{Key: "evidence.root", Value: evidenceRoot}}); err != nil {
		return err
	}
	rewritten, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(destination, string(rewritten), filepath.Dir(destination))
	return err
}

// MakeManifest writes the adapter-declared paths where the isolation
// validator reads them.
func (OSHost) MakeManifest(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := atomicfile.WriteText(path, Manifest(), filepath.Dir(path))
	return err
}

// Stamp is the source commit an installed engine was built from, read out of
// the executable's own build information the way the steward reads it.
func (OSHost) Stamp(binary string) (string, error) {
	file, err := os.Open(binary)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := buildinfo.Read(file)
	if err != nil {
		return "", err
	}
	const assignment = "supervise.BuildStamp="
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		at := strings.Index(setting.Value, assignment)
		if at < 0 {
			continue
		}
		value := setting.Value[at+len(assignment):]
		if end := strings.IndexAny(value, " \t\r\n\"'"); end >= 0 {
			value = value[:end]
		}
		return strings.TrimSpace(value), nil
	}
	return "", nil
}

// Enrolled is the identity the installation carries for the engine now
// installed there, which is the steward's own verification and not a file's
// existence.
func (OSHost) Enrolled(installation string) (Identity, bool) {
	installed, err := steward.VerifyEnrolledBinary(installation)
	if err != nil {
		return Identity{}, false
	}
	return Identity{RepoIdentity: installed.RepoIdentity, Generation: installed.Generation}, true
}

// SupervisionUp reads the supervision owner lock and asks the kernel whether
// the process it names is alive.
//
// It is deliberately not the steward's recorded health verdict. That verdict
// is written by a tick, and the whole of this step happens before a newly
// armed machine has ticked: reading it would mean waiting on a file nobody
// has written yet, or reading a stale one from a previous generation. The
// lock and the pid in it are the present tense.
func (OSHost) SupervisionUp(installation string) bool {
	data, err := os.ReadFile(filepath.Join(installation, "artifacts", "agents", "supervision", "lock.d", "owner.json"))
	if err != nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var owner struct {
		Pid          int64  `json:"pid"`
		PidStartedAt int64  `json:"pidStartedAt"`
		PidTicks     int64  `json:"pidStartTicks"`
		BootID       string `json:"bootId"`
	}
	if decoder.Decode(&owner) != nil || owner.Pid < 1 || owner.PidStartedAt < 1 {
		return false
	}
	return identity.AliveRef(identity.KernelProber{}, identity.Ref{
		Pid: owner.Pid, StartedAtSec: owner.PidStartedAt,
		StartTicks: owner.PidTicks, BootID: owner.BootID,
	}) == identity.Alive
}

// Free is the bytes available to this user at a directory.
func (OSHost) Free(directory string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(directory, &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}

// Size is the bytes one tree takes, counting regular files only.
//
// A file that cannot be read is skipped rather than failing the walk: this
// number decides whether there is room for a clone, and one unreadable file
// must not stop a launch that has room.
func (OSHost) Size(root string) (uint64, error) {
	var total uint64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		total += uint64(info.Size())
		return nil
	})
	return total, err
}

/* ------------------------------------------------------------ the runner -- */

// OSRunner runs one step as a subprocess: bounded, on its own process group,
// under a scrubbed environment.
type OSRunner struct{}

// Run executes one command and answers what it printed.
//
// A refusal carries the owner's own words — its standard error, trimmed —
// because that is what the record keeps and the card shows. A command that
// printed nothing is named by what it was, so a step never fails silently.
func (OSRunner) Run(command Command) (string, error) {
	process := exec.Command(command.Name, command.Args...)
	process.Dir = command.Dir
	process.Env = Scrubbed(os.Environ())
	var out, problem strings.Builder
	process.Stdout = &out
	process.Stderr = &problem
	what := filepath.Base(command.Name)
	if len(command.Args) > 0 {
		what += " " + command.Args[0]
	}
	if err := boundedexec.Run(process, boundedexec.FixedBound(command.Budget, "the launch step budget"), what); err != nil {
		said := strings.TrimSpace(problem.String())
		if said == "" {
			said = strings.TrimSpace(out.String())
		}
		if said == "" {
			return out.String(), fmt.Errorf("%s: %v", what, err)
		}
		return out.String(), fmt.Errorf("%s", said)
	}
	return out.String(), nil
}

// Scrubbed is the environment every step runs under: this process's, without
// its METASYSTEM_* variables.
//
// Configuration precedence puts the environment above the file, and the
// interface's own spawn hands its whole environment to what it launches. So a
// server started with METASYSTEM_EVIDENCE_ROOT set would give every machine
// it launches that root, whatever the configuration this verb just wrote into
// the clone says. Dropping them is what makes the copied file the last word.
func Scrubbed(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		if strings.HasPrefix(entry, "METASYSTEM_") {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}
