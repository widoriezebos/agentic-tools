// Package dispatchproc holds the process-facing dispatch owners that sit
// between internal/dispatch and the process recognizers (internal/janitor
// and internal/census import internal/dispatch, so these cannot live there):
// the fixture-authorized start reader, the claim's process verifier and
// tagged-process scanner, and the claim-launch surface authorization. The
// engine's job verbs and the delegation lifecycle both use them
// (verbs-object-action U6b; they left package main).
package dispatchproc

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

// StartReader is the kernel start reader, answering from the fixture
// process table only when root authorizes one.
func StartReader(root string) (identity.StartReader, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return nil, err
	}
	return identity.FixtureStartReader{Kernel: identity.KernelProber{}, Fixture: authorization.Identity()}, nil
}

// PositionedJobTagAt matches a tag in a known argv position among an
// installation's shapes, its external runtimes' and overrides' included
// (janitor.ShapesAt). An empty root is the committed shapes alone.
func PositionedJobTagAt(root string) func(argv []string, tag string) bool {
	shapes := janitor.ShapesAt(root)
	return func(argv []string, tag string) bool {
		_, matches := janitor.MatchShape(shapes, argv, tag)
		return matches
	}
}

// ClaimProcessVerifier proves a claimed process by its tagged argv among
// Root's shapes (an external runtime's supervisor included).
type ClaimProcessVerifier struct{ Root string }

func (v ClaimProcessVerifier) Verify(pid int64, instanceTag string) identity.Verification {
	matches := PositionedJobTagAt(v.Root)
	return identity.VerifyProcess(identity.KernelProber{}, pid, func(argv []string) bool {
		return matches(argv, instanceTag)
	})
}

// TaggedProcessScanner scans the process table (or root's fixture table) for
// a reservation's instance tag.
type TaggedProcessScanner struct{ Root string }

func (s TaggedProcessScanner) ScanTag(tag string, reservationCreatedAt time.Time) census.TaggedProcessCensus {
	dependencies := census.TaggedScanDependencies{MatchesTag: PositionedJobTagAt(s.Root), ReservationCreatedAt: reservationCreatedAt}
	processes, configured, err := census.ConfiguredProcessFixture(s.Root)
	if err != nil {
		return census.TaggedProcessCensus{EnumerationError: err.Error()}
	}
	if configured {
		reader, readerErr := StartReader(s.Root)
		if readerErr != nil {
			return census.TaggedProcessCensus{EnumerationError: readerErr.Error()}
		}
		pids := make([]int64, 0, len(processes))
		byPID := make(map[int64]census.Process, len(processes))
		argv := make(map[int64][]string, len(processes))
		argvKnown := make(map[int64]bool, len(processes))
		for _, process := range processes {
			if !process.Alive {
				continue
			}
			pids = append(pids, process.Pid)
			byPID[process.Pid] = process
			// Fixture process rows store a flat command string. Only simple
			// whitespace-separated commands can recover exact token
			// boundaries; quoted or escaped commands remain unreadable and
			// cannot prove a tag.
			if !strings.ContainsAny(process.Argv, "'\"\\") {
				argv[process.Pid] = strings.Fields(process.Argv)
				argvKnown[process.Pid] = true
			}
		}
		dependencies.PIDs = func() ([]int64, error) { return pids, nil }
		dependencies.Signal = func(pid int64) error {
			if _, exists := byPID[pid]; !exists {
				return unix.ESRCH
			}
			return nil
		}
		dependencies.PGID = func(pid int64) (int64, error) {
			process, exists := byPID[pid]
			if !exists {
				return 0, unix.ESRCH
			}
			return process.PGID, nil
		}
		dependencies.Reader = configuredProcessReader{starts: reader, argv: argv, argvKnown: argvKnown}
	}
	return census.ScanTaggedProcesses(tag, dependencies)
}

type configuredProcessReader struct {
	starts    identity.StartReader
	argv      map[int64][]string
	argvKnown map[int64]bool
}

func (r configuredProcessReader) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	return r.starts.ReadStart(pid)
}

func (r configuredProcessReader) ReadArgv(pid int64) ([]string, bool) {
	return r.argv[pid], r.argvKnown[pid]
}

// ClaimSurface is the claim-launch caller's standing: whether it is the
// delegate boundary (METASYSTEM_DELEGATE_INTERNAL) and the claim capability
// the boundary minted.
type ClaimSurface struct {
	DelegateInternal bool
	Capability       string
}

// ClaimAuthorized keeps reservation publication behind the delegate
// boundary. The internal marker identifies the route but carries no
// authority by itself: a real delegate also presents the short-lived bearer
// word it minted, and the authoritative claim spends that word under the job
// record lock. The complete process-table fixture remains the one test seam
// that may exercise the custody machine directly.
func ClaimAuthorized(root string, surface ClaimSurface, binding dispatch.DelegateClaimCapabilityBinding, preflight bool) bool {
	if !surface.DelegateInternal {
		return false
	}
	if FixtureAuthorized(root) {
		return true
	}
	if surface.Capability == "" {
		return false
	}
	if preflight {
		return dispatch.ValidateDelegateClaimCapability(root, surface.Capability, binding) == nil
	}
	return dispatch.ConsumeDelegateClaimCapability(root, surface.Capability, binding) == nil
}

// FixtureAuthorized reports the complete fake-runtime process-table fixture:
// both fixture files present and the root configured for the fake runtime.
func FixtureAuthorized(root string) bool {
	for _, key := range []string{"METASYSTEM_CENSUS_PROCESS_FILE", "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"} {
		path := os.Getenv(key)
		if path == "" {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	configuration, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		return false
	}
	return strings.Contains("\n"+string(configuration), "\nmetasystem.runtimes=fake\n")
}
