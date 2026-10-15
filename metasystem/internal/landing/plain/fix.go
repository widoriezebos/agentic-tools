package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Fix is one bounded repair of a batch, retained beside its brief.
type Fix struct {
	Launch       *LaunchRecord `json:"launch,omitempty"`
	At           string        `json:"at,omitempty"`
	Attempt      string        `json:"attempt"`
	ProofAttempt string        `json:"proof_attempt,omitempty"`
	Round        int           `json:"round"`
	Parent       string        `json:"parent"`
	Verdict      string        `json:"verdict,omitempty"`
	Goal         string        `json:"goal"`
	Units        []string      `json:"units"`
	Job          string        `json:"job"`
	Read         string        `json:"read"`
	Commit       string        `json:"commit"`
	State        string        `json:"state"`

	CheckMinutes  float64 `json:"check_minutes,omitempty"`
	CheckResult   string  `json:"check_result,omitempty"`
	ReadMinutes   float64 `json:"read_minutes,omitempty"`
	ReadStartedAt string  `json:"read_started_at,omitempty"`
	ReadLaneOwned bool    `json:"read_lane_owned,omitempty"`

	Paths     []conflict.Path        `json:"paths,omitempty"`
	Tip       string                 `json:"tip,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Brief     string                 `json:"brief,omitempty"`
	Generated []testpolicy.Generated `json:"generated,omitempty"`
}

// Unit keeps the repair's identity when its follow-up proof gets a new attempt.
func (fix Fix) Unit() string {
	stamp := strings.SplitN(fix.RepairProof(), ".", 2)[0]
	if _, err := time.Parse("20060102T150405", stamp); err != nil || fix.Round < 1 {
		return ""
	}
	return fmt.Sprintf("lane-fix-%s-%d", stamp, fix.Round)
}

func (fix Fix) RepairProof() string {
	if fix.ProofAttempt != "" {
		return fix.ProofAttempt
	}
	return fix.Attempt
}

func ReadFix(install string) (*Fix, error) { return readFix(install, false) }

func readFix(install string, includeClosed bool, goals ...string) (*Fix, error) {
	return readFixRecords(install, false, includeClosed, false, goals...)
}

// readFixRecords may close stale attempts only while the caller holds the lane lock.
func readFixRecords(install string, closeStale, includeClosed, mergeOnly bool, goals ...string) (*Fix, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	running, recorded, _, readErr := ReadRunning(install, ProveSeams{})
	recorded = recorded && readErr == nil
	var active, latest *Fix
	var newest time.Time
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fix Fix
		if err := json.Unmarshal(data, &fix); err != nil {
			if len(goals) > 0 && fix.Goal != goals[0] {
				continue
			}
			return nil, err
		}
		if len(goals) > 0 && goals[0] != "" && fix.Goal != goals[0] {
			continue
		}
		if mergeOnly && fix.Tip == "" && fix.State != "resolving" && fix.State != "resolved" {
			continue
		}
		if includeClosed {
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if latest == nil || info.ModTime().After(newest) {
				latest, newest = &fix, info.ModTime()
			}
		}
		if fix.State != "running" && fix.State != "reviewing" && fix.State != "building" && fix.State != "resolving" && fix.State != "resolved" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.State != "resolving" && fix.Job == "" || fix.State != "building" && fix.Commit == "" || fix.State == "resolving" && (fix.Tip == "" || len(fix.Paths) == 0) {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if fix.Attempt == "" {
			fix.Attempt = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if due, reason := landingstate.LifecycleDue("fix", landingstate.LifecycleFacts{Attempt: fix.Attempt, CurrentAttempt: running.Attempt}, time.Now()); recorded && fix.Tip == "" && due {
			if closeStale {
				if _, err := landingstate.Transition(Dir(install), time.Now(), "fix:"+filepath.Base(path), "present", "closed", "keeper", reason, nil, func() error { fix.State = "closed"; return writeRawFix(install, &fix) }); err != nil {
					return nil, err
				}
			}
			continue
		}
		if active != nil {
			return nil, errors.New(landingstate.Phrase("more-than-one-lane-fix-is-running"))
		}
		active = &fix
	}
	if active == nil && latest != nil && latest.State == "abandoned" {
		return latest, nil
	}
	return active, nil
}

func resolutionDescription(f *Fix) landingstate.Resolution {
	commit := f.Commit
	if commit == "" {
		commit = f.Parent
	}
	paths := []string{}
	for _, p := range f.Paths {
		paths = append(paths, p.Path)
	}
	r := landingstate.Resolution{Fact: landingstate.Fact{Word: f.State}, Goal: f.Goal, Job: f.Job, Reason: f.Reason, Commit: commit, Paths: paths, Units: f.Units, Round: f.Round, CheckResult: f.CheckResult, ReadPending: f.State == "reviewing" && f.Tip != "" && (f.ReadStartedAt != "" || f.ReadMinutes > 0)}
	if f.Launch != nil {
		r.LaunchError = f.Launch.Error
		r.LaunchAt, _ = time.Parse(time.RFC3339Nano, f.Launch.At)
	}
	return r
}

// WriteFix retains a fix's lifecycle beside its attempt's brief.
func WriteFix(install string, fix *Fix, effects ...ProveSeams) error {
	s := firstEffects(effects)
	return transitionWrite(install, "resolution", "resolve", s.now(), "", func() error { return writeRawFix(install, fix, s.now()) }, effects...)
}

func writeRawFix(install string, fix *Fix, clock ...time.Time) error {
	previous, err := FixForAttempt(install, fix.Attempt)
	if err != nil {
		return err
	}
	if fix.At == "" || previous != nil && previous.State != fix.State {
		if len(clock) > 0 {
			fix.At = clock[0].UTC().Format(time.RFC3339Nano)
		} else {
			fix.At = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	data, err := json.Marshal(fix)
	if err != nil {
		return err
	}
	dir := filepath.Join(Dir(install), "fixes")
	_, err = atomicfile.WriteText(filepath.Join(dir, fix.Attempt+".json"), string(data)+"\n", dir)
	return err
}

func ActiveFix(install string) (*Fix, error) { return ReadFix(install) }

func FixForAttempt(install, attempt string) (*Fix, error) {
	data, err := os.ReadFile(filepath.Join(Dir(install), "fixes", attempt+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var fix Fix
	return &fix, json.Unmarshal(data, &fix)
}
func currentFix(install string, seams ProveSeams) (*Fix, error) {
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return nil, err
	}
	if !recorded || running.Checkpoint {
		result, present, err := LastResult(install)
		if running.Gate {
			result, present, err = LastGate(install)
		}
		if err != nil || !present {
			return nil, err
		}
		if recorded && (result.Result != Red || result.Attempt != running.Attempt || result.Commit != running.Commit) || !recorded && result.Result != Green {
			return nil, nil
		}
		running.Attempt, running.Commit = result.Attempt, result.Commit
	}
	fix, err := FixForAttempt(install, running.Attempt)
	if err != nil || fix == nil || fix.State != "reviewing" && fix.State != "running" || fix.Job == "" || len(fix.Units) == 0 || fix.Commit == "" || fix.Round != 1 || fix.Parent == "" || fix.Attempt != running.Attempt {
		return nil, err
	}
	if running.Commit != fix.Parent && running.Commit != fix.Commit {
		batch, err := ReadBatch(install)
		if err != nil || batch == nil || batch.Lane.Root == "" {
			return nil, err
		}
		contained, err := fixInFirstParent(batch.Lane.Root, running.Commit, fix.Commit, seams)
		if err != nil || !contained {
			return nil, err
		}
	}
	return fix, nil
}

// A main refresh keeps the repair on the lane's first-parent chain.
func fixInFirstParent(checkout, commit, fix string, seams ProveSeams) (bool, error) {
	if commit == fix {
		return true, nil
	}
	chain, err := seams.git(checkout, "rev-list", "--first-parent", commit)
	return slices.Contains(strings.Fields(chain), fix), err
}

func advanceFix(install, checkout string, running Running, seams ProveSeams) error {
	previous, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return err
	}
	if !recorded {
		result, present, err := LastResult(install)
		if err != nil || !present || result.Result != Green {
			return err
		}
		previous = Running{Attempt: result.Attempt, Commit: result.Commit, BatchID: result.BatchID, BatchMembers: result.BatchMembers}
	}
	if previous.Attempt == running.Attempt {
		return err
	}
	fix, err := FixForAttempt(install, previous.Attempt)
	if err != nil || fix == nil || fix.State == "closed" {
		return err
	}
	follows := false
	if !running.Trunk && fix.Commit != "" && previous.BatchID == running.BatchID && slices.Equal(previous.BatchMembers, running.BatchMembers) {
		follows, err = fixInFirstParent(checkout, running.Commit, fix.Commit, seams)
		if err != nil {
			return err
		}
	}
	fix.State = "closed"
	err = WriteFix(install, fix, seams)
	if err == nil && follows {
		fix.ProofAttempt = fix.RepairProof()
		fix.Attempt, fix.State = running.Attempt, "reviewing"
		err = WriteFix(install, fix, seams)
	}
	return err
}

func closeFix(install, goal string, effects ...ProveSeams) error {
	fix, err := readFixRecords(install, true, false, false, goal)
	if err != nil {
		return err
	}
	if fix != nil {
		fix.State = "closed"
		if err := WriteFix(install, fix, effects...); err != nil {
			return err
		}
	}
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil || !recorded || !running.Checkpoint || goal != "" && !slices.ContainsFunc(running.BatchMembers, func(m GoalSHA) bool { return m.Goal == goal }) {
		return err
	}
	return removeRunning(install, effects...)
}

// RefreshMerge closes a resolution whose pending merge no longer exists.
func RefreshMerge(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	return withLock(install, func() error { return refreshMergeLocked(install, checkout, fix, seams) })
}

func refreshMergeLocked(install, checkout string, fix *Fix, seams ProveSeams) error {
	resolving := fix != nil && fix.State == "resolving"
	if err := observeMerge(install, checkout, fix, seams); err != nil {
		return err
	}
	if resolving && fix.State == "abandoned" {
		return WriteFix(install, fix, seams)
	}
	return nil
}

func observeMerge(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	// Status can wait behind a commit; the current record owns the transition.
	data, err := os.ReadFile(filepath.Join(Dir(install), "fixes", fix.Attempt+".json"))
	if err != nil {
		return err
	}
	var current Fix
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	*fix = current
	if fix.State != "resolving" {
		return nil
	}
	tip, err := seams.git(checkout, "rev-parse", "--verify", "MERGE_HEAD^{commit}")
	if err != nil {
		var exit *exec.ExitError
		if !errors.Is(err, errNoMerge) && (!errors.As(err, &exit) || exit.ExitCode() != 128) {
			return err
		}
		fix.Reason = "the pending merge is gone; its resolution was abandoned"
	} else if tip != fix.Tip {
		fix.Reason = "the pending merge changed; its previous resolution was abandoned"
	} else {
		return nil
	}
	fix.State = "abandoned"
	return nil
}

// RecordedFixRed binds an ordinary repair to the failed proof's checkpoint.
func RecordedFixRed(install string, running Running) bool {
	mode := ProveSeams{Gate: running.Gate}
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return false
	}
	for _, result := range slices.Backward(results) {
		if result.Commit == running.Commit && result.Tree == running.Tree && result.Attempt == running.Attempt && result.BatchID == running.BatchID && slices.Equal(result.BatchMembers, running.BatchMembers) {
			return result.Result == Red
		}
	}
	return false
}

func proofFixReady(install string, seams ProveSeams) error {
	if seams.Trunk {
		return nil
	}
	fix, err := readFixRecords(install, false, false, true)
	if err != nil {
		return err
	}
	if fix != nil && (fix.State == "resolving" || fix.State == "resolved" || fix.State == "reviewing" && fix.Tip != "") {
		message := fmt.Sprintf("finish the resolution and its read for %s before landing prove (repair is %s)", fix.Goal, fix.State)
		if fix.Reason != "" {
			message += ": " + fix.Reason
		}
		return errors.New(message)
	}
	return nil
}

// UpdateReadMinutes measures the wall time of a merge read across collections.
func (fix *Fix) UpdateReadMinutes(now time.Time) {
	if started, err := time.Parse(time.RFC3339Nano, fix.ReadStartedAt); err == nil {
		fix.ReadMinutes = max(fix.ReadMinutes, now.Sub(started).Minutes())
	}
}

func candidateRepairPending(install string, running Running, recorded bool) (*RepairPending, error) {
	fix, err := ReadFix(install)
	if err != nil {
		return nil, err
	}
	repair := fix != nil
	if recorded && running.Checkpoint && !running.Trunk {
		results, err := readLines[Result](ProveSeams{Gate: running.Gate}.resultsPath(install))
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			if result.Attempt == running.Attempt {
				repair = fix != nil || result.Result == Red && (result.Cause == nil || result.Cause.Kind != "main" && result.Cause.Kind != "flake" && result.Cause.Kind != "environment")
			}
		}
	}
	if !repair {
		return nil, nil
	}
	pending := &RepairPending{Tree: running.Tree, Fix: running.Attempt}
	if fix != nil {
		pending.Fix = fix.Goal
	}
	if !recorded || !running.Checkpoint {
		pending.Tree = ""
	}
	previous, present, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return nil, err
	}
	if !present {
		result, _, err := LastResult(install)
		if err != nil {
			return nil, err
		}
		if result.Result == Green {
			previous.Attempt = result.Attempt
		}
	}
	pending.Closes = fix != nil && fix.Attempt == previous.Attempt
	if fix != nil {
		pending.MergeTip = fix.Tip != ""
	}
	return pending, nil
}

// RepairPending protects a candidate checkpoint from automatic main checks.
type RepairPending struct {
	Tree, Fix        string
	Closes, MergeTip bool
}

func (r *RepairPending) Error() string {
	if r.Tree == "" {
		return "main's check waits for the pending fix of " + r.Fix
	}
	return "main's check waits for the candidate's repair of tree " + Short(r.Tree) + " to finish"
}
