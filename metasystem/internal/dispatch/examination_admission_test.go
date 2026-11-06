package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestReservedUnknownReadAdmissionPreservesAuthorityLimits(t *testing.T) {
	t.Parallel()
	for _, variation := range []string{"full-work-box", "active-ceiling", "elapsed", "unreserved", "wrong-revision"} {
		t.Run(variation, func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			path := filepath.Join(bed.root, "plans/goals/bounded.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			file.Budget.AttemptLimit = 1
			if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
				t.Fatal(err)
			}
			bed.accept(t)
			writeBudgetJob(t, bed.root, "original", "original-op", 2, 30, "completed", budgetJobLife{})
			recordPath := filepath.Join(bed.root, "artifacts/agents/jobs/original.json")
			record := readJSONFile(t, recordPath)
			record["role"], record["round"] = "code-critic", 1
			if err := writeRecord(recordPath, record); err != nil {
				t.Fatal(err)
			}
			if variation != "unreserved" {
				if err := ReserveUnknownExaminationRetry(bed.root, "original", CustodyDeathDependencies{}); err != nil {
					t.Fatal(err)
				}
			}
			if variation == "active-ceiling" {
				writeBudgetJob(t, bed.root, "live", "live-op", 2, 30, "running", budgetJobLife{})
			}
			now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
			if variation == "elapsed" {
				now = now.Add(48 * time.Hour)
			}
			revision := uint64(2)
			if variation == "wrong-revision" {
				revision = 3
			}
			reads := ProofAdmissionReads{ResolveEndpoint: bed.reads.ResolveEndpoint, ResolveMachine: bed.reads.ResolveMachine,
				Receipt: ReceiptAdmissionSource{AcceptedLedgerTip: func(string) (string, bool, error) { return admissionFixtureTip, true, nil },
					TopLevel: func(string) (string, error) { return bed.root, nil },
					FileAt:   func(string, string, string) ([]byte, bool, error) { return nil, false, nil }}}
			verdict, err := EvaluateUnknownExaminationRetryAdmission(bed.root, "original", "bounded", revision, 120, now, reads, CustodyDeathDependencies{})
			if variation == "full-work-box" {
				if err != nil || verdict.Refused() {
					t.Fatalf("reserved read consumed another work allowance: %+v %v", verdict, err)
				}
			} else if err == nil && !verdict.Refused() {
				t.Fatalf("%s bypassed the examination's authority limits", variation)
			}
			global, globalErr := evaluateGoalAdmissionForUnknownExaminationRetryWithReads(bed.root, "coordinator", "original", now, bed.reads, CustodyDeathDependencies{})
			if variation == "full-work-box" && (globalErr != nil || global.Refused()) {
				t.Fatalf("global goal admission charged the reserved read: %+v %v", global, globalErr)
			}
			if (variation == "active-ceiling" || variation == "elapsed" || variation == "unreserved") && globalErr == nil && !global.Refused() {
				t.Fatalf("global %s admission bypassed authority limits", variation)
			}
		})
	}
}

type retryAdmissionProcessTable struct {
	*stage4ProcessTable
	err error
}

func (f retryAdmissionProcessTable) Pids() ([]int64, error) { return nil, f.err }

func TestTheReservedRetryIsAdmittedWhenItsCustodyIsGone(t *testing.T) {
	t.Parallel()
	for _, variation := range []string{"dead", "alive", "unreadable", "cancelled"} {
		t.Run(variation, func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			writeBudgetJob(t, bed.root, "original", "original-op", 2, 30, "completed", budgetJobLife{})
			path := filepath.Join(bed.root, "artifacts/agents/jobs/original.json")
			primary, child := stage4Exact(71672, 1), stage4Exact(71689, 2)
			record := stage4RecordWithPrimary(readJSONFile(t, path), primary, 71672)
			record["role"], record["round"], record["instanceTag"] = "code-critic", 1, "reserved-retry-tag"
			record["custodyProcesses"] = []any{stage4RefObject(child)}
			if err := writeRecord(path, record); err != nil {
				t.Fatal(err)
			}
			table := retryAdmissionProcessTable{stage4ProcessTable: &stage4ProcessTable{
				starts: map[int64]identity.Exact{71689: child}, startStates: map[int64]identity.Liveness{},
			}}
			custody := CustodyDeathDependencies{Reader: table.stage4ProcessTable, Processes: table,
				MatchesTag: func(argv []string, tag string) bool { return len(argv) == 2 && argv[1] == tag },
				TaggedScan: func(string) census.TaggedProcessCensus { return census.TaggedProcessCensus{} }}
			if err := ReserveUnknownExaminationRetry(bed.root, "original", custody); err != nil {
				t.Fatal(err)
			}
			var words []string
			switch variation {
			case "alive":
				table.startStates[71689] = identity.Alive
				words = []string{"71689", "custody-process-alive", "not proven quiescent", "end process 71689 and its process group 71672, then retry"}
			case "unreadable":
				table.err = errors.New("process table unavailable")
				custody.Processes = table
				words = []string{"process-table-unreadable", "not proven quiescent", "if its process group 71672 still runs, end it, then retry"}
			case "cancelled":
				record = readJSONFile(t, path)
				record["status"] = "cancelled"
				if err := writeRecord(path, record); err != nil {
					t.Fatal(err)
				}
				words = []string{"no matching reserved source read"}
			}
			err := ReservedUnknownExaminationRetry(bed.root, "original", custody)
			if variation == "dead" {
				if err != nil {
					t.Fatalf("dead custody refused the reserved retry: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s custody admitted the reserved retry", variation)
			}
			for _, word := range words {
				if !strings.Contains(err.Error(), word) {
					t.Fatalf("refusal %q lacks %q", err, word)
				}
			}
		})
	}
}

// ownedProcessTable is the kernel's process table narrowed to the one
// process the test started, so the proof never reads another test's.
type ownedProcessTable struct{ pid int64 }

func (f ownedProcessTable) Pids() ([]int64, error) { return []int64{f.pid}, nil }
func (f ownedProcessTable) Group(pid int64) (int64, error) {
	return identity.KernelProcessTable{}.Group(pid)
}
func (f ownedProcessTable) Session(pid int64) (int64, error) {
	return identity.KernelProcessTable{}.Session(pid)
}
func (f ownedProcessTable) Parent(pid int64) (int64, bool) {
	return identity.KernelProcessTable{}.Parent(pid)
}

// The refusal's remedy, run as written against a completed examination
// whose real process survives, ends that process and admits the retry.
func TestTheReservedRetrysRemedyEndsTheLiveCustody(t *testing.T) {
	t.Parallel()
	bed := newGoalAdmissionBed(t, 2)
	writeBudgetJob(t, bed.root, "original", "original-op", 2, 30, "completed", budgetJobLife{})
	fixture := testutil.Fixture(t)
	child := fixture.Shell("exec sleep 120")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := child.Process.Pid
	fixture.Record(pid)
	waited := make(chan struct{})
	go func() { _ = child.Wait(); close(waited) }()
	t.Cleanup(func() {
		select {
		case <-waited:
		default:
			_ = child.Process.Kill()
			<-waited
		}
	})
	live, state, err := identity.KernelProber{}.ReadStart(int64(pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("the test's process has no readable identity: %v %v", state, err)
	}
	path := filepath.Join(bed.root, "artifacts/agents/jobs/original.json")
	record := stage4RecordWithPrimary(readJSONFile(t, path), live, int64(pid))
	record["role"], record["round"], record["instanceTag"] = "code-critic", 1, "reserved-retry-remedy-tag"
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	empty := &stage4ProcessTable{starts: map[int64]identity.Exact{}, startStates: map[int64]identity.Liveness{}}
	noSurvivor := func(string) census.TaggedProcessCensus { return census.TaggedProcessCensus{} }
	if err := ReserveUnknownExaminationRetry(bed.root, "original", CustodyDeathDependencies{Reader: empty, Processes: empty,
		MatchesTag: func([]string, string) bool { return false }, TaggedScan: noSurvivor}); err != nil {
		t.Fatal(err)
	}
	custody := CustodyDeathDependencies{Reader: identity.KernelProber{}, Processes: ownedProcessTable{pid: int64(pid)},
		MatchesTag: func([]string, string) bool { return false }, TaggedScan: noSurvivor}
	refusal := ReservedUnknownExaminationRetry(bed.root, "original", custody)
	if refusal == nil || !strings.Contains(refusal.Error(), strconv.Itoa(pid)) {
		t.Fatalf("live custody of a completed examination was not refused by its process: %v", refusal)
	}
	match := regexp.MustCompile(`end process \d+ and its process group (\d+)`).FindStringSubmatch(refusal.Error())
	if match == nil {
		t.Fatalf("refusal %q names no process group to end", refusal)
	}
	group, _ := strconv.Atoi(match[1])
	if group != pid {
		t.Fatalf("refusal names process group %d, the examination's is %d", group, pid)
	}
	if err := syscall.Kill(-group, syscall.SIGKILL); err != nil {
		t.Fatalf("the named process group could not be ended: %v", err)
	}
	<-waited
	if err := ReservedUnknownExaminationRetry(bed.root, "original", custody); err != nil {
		t.Fatalf("the retry stayed refused after its remedy ended the custody: %v", err)
	}
}
