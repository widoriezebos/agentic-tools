package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// strictAdmissionRepository exposes only the immutable accepted reads used by projection.
// The embedded nil Repository makes every unsupported operation fail immediately.
type strictAdmissionRepository struct {
	goal.Repository
	files     map[string][]byte
	committed time.Time
}

const admissionFixtureTip = "accepted-revision-fixture"

func (r *strictAdmissionRepository) Accepted() (string, bool, error) {
	return admissionFixtureTip, true, nil
}

func (r *strictAdmissionRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	if commit != admissionFixtureTip {
		return nil, fmt.Errorf("unexpected accepted commit %q", commit)
	}
	files := make(map[string][]byte)
	for path, data := range r.files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				files[path] = append([]byte(nil), data...)
				break
			}
		}
	}
	return files, nil
}

func (r *strictAdmissionRepository) CommitTime(commit string) (time.Time, error) {
	if commit != admissionFixtureTip {
		return time.Time{}, fmt.Errorf("unexpected accepted commit %q", commit)
	}
	return r.committed, nil
}

type goalAdmissionBed struct {
	root       string
	repository *strictAdmissionRepository
	reads      goalAdmissionReads
}

// accept copies the rendered files into one immutable accepted snapshot.
func (bed *goalAdmissionBed) accept(t *testing.T) {
	t.Helper()
	files := make(map[string][]byte)
	directory := filepath.Join(bed.root, "plans", "goals")
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(bed.root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = append([]byte(nil), data...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bed.repository = &strictAdmissionRepository{files: files, committed: bed.repository.committed}
}

func newGoalAdmissionBed(t *testing.T, claimRevision uint64) *goalAdmissionBed {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(relative string, data []byte) {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}))
	file := &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Bound the dispatch", Origin: goal.OriginMain,
		NextStep: "Run the bounded work.", OpenedAt: "2026-08-28T08:00:00Z", Revision: 4,
		Risk:    &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises an admitted tier-three goal."},
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T09:00:00Z", Revision: claimRevision},
		History: []goal.HistoryLine{
			{At: "2026-08-28T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000", Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T09:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001", Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T09:30:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002", Verb: "edit", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: "2026-08-28T10:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000003", Verb: "edit", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
		},
	}
	if claimRevision > 0 {
		file.Budget = &goal.Budget{ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 3}
		file.StopCapability = &goal.StopCapability{
			Generation: claimRevision, Revision: claimRevision, Machine: "bed-m1", ClaimEpoch: 7,
		}
	}
	write("plans/goals/bounded.md", goal.RenderFile(file))
	repository := &strictAdmissionRepository{committed: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
	bed := &goalAdmissionBed{root: root, repository: repository}
	bed.reads = goalAdmissionReads{
		NewWorld: func(got string) bool {
			if got != root {
				t.Fatalf("goal world root = %q, want %q", got, root)
			}
			return true
		},
		ResolveEndpoint: func(got string) (goal.Endpoint, error) {
			if got != root {
				return goal.Endpoint{}, fmt.Errorf("goal endpoint root = %q, want %q", got, root)
			}
			return goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: bed.repository}, nil
		},
		ResolveMachine: func(got string) (string, error) {
			if got != root {
				return "", fmt.Errorf("goal machine root = %q, want %q", got, root)
			}
			return "bed-m1", nil
		},
	}
	bed.accept(t)
	return bed
}

func (bed *goalAdmissionBed) revision(id string) (uint64, uint8, error) {
	return resolveGoalRevisionWithReads(bed.root, id, bed.reads)
}
func (bed *goalAdmissionBed) binding(id string, now time.Time) (GoalBinding, error) {
	return resolveGoalBindingWithReads(bed.root, id, now, bed.reads)
}
func (bed *goalAdmissionBed) admission(lineage string, now time.Time) (GoalAdmissionVerdict, error) {
	return evaluateGoalAdmissionWithReads(bed.root, lineage, now, bed.reads)
}
func (bed *goalAdmissionBed) revisionAdmission(id string, revision, cap uint64, now time.Time, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionWithReads(bed.root, id, revision, cap, now, bed.reads, hazards...)
}
func (bed *goalAdmissionBed) revisionAdmissionForDispatch(id string, revision, cap uint64, now time.Time, role, mode string, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionForDispatchWithReads(bed.root, id, revision, cap, now, role, mode, allBudgetMembers, bed.reads, hazards...)
}
func (bed *goalAdmissionBed) stops(now time.Time) ([]StopRoute, error) {
	return findBreachStopsWithReads(bed.root, now, bed.reads)
}

func (bed *goalAdmissionBed) commitTier(t *testing.T, tier uint8, tierLaw string) {
	t.Helper()
	root := bed.root
	goalPath := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture: %v", problems)
	}
	file.Tier = tier
	if err := os.WriteFile(goalPath, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(root, "plans", "goals", "backlog.md")
	data, err = os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	rootRecord, rootProblems := goal.ParseRoot(data)
	if len(rootProblems) != 0 {
		t.Fatalf("parse root fixture: %v", rootProblems)
	}
	rootRecord.TierLaw = tierLaw
	if err := os.WriteFile(rootPath, goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}

func (bed *goalAdmissionBed) addFenced(t *testing.T, id, stopID string, opids [3]string) {
	t.Helper()
	root := bed.root
	openedAt := "2026-08-28T08:00:00Z"
	claimedAt := "2026-08-28T09:00:00Z"
	closedAt := "2026-08-28T09:30:00Z"
	file := &goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Hold the stopped work", Origin: goal.OriginMain,
		NextStep: "Wait for a human resume.", OpenedAt: openedAt, Revision: 3,
		Risk: &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises a fenced tier-three goal."},
		Budget: &goal.Budget{
			ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 3,
		},
		Claimed:        &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: claimedAt, Revision: 2},
		StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1},
		StopFence: &goal.StopFence{
			StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2,
			ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit,
		},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: goal.Opid(opids[0], "bed-m1", "coordinator"), Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
			{At: claimedAt, Opid: goal.Opid(opids[1], "bed-m1", "coordinator"), Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
			{At: closedAt, Opid: goal.Opid(opids[2], "bed-m1", "coordinator"), Verb: "breach-stop", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
		},
	}
	path := filepath.Join(root, "plans", "goals", id+".md")
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}

func (bed *goalAdmissionBed) reviewChain(t *testing.T) {
	t.Helper()
	root := bed.root
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture: %v", problems)
	}
	file.Budget.AttemptLimit = 20
	file.Budget.ReservedJobMinutesLimit = 1000
	file.Budget.ActiveJobLimit = 10
	file.Budget.ReviewRoundLimit = 2
	file.History[0].Verb = "approve"
	file.History[0].Actor = "human:Wido"
	file.Approved = &goal.ApprovalRecord{
		By: "human:Wido", At: file.History[0].At, Revision: 1, EpisodeRevision: 1,
		Opid: file.History[0].Opid, Authority: goal.ApprovalAuthorityProven,
		Digest: legacyBudgetApprovalDigest(file.Intent, *file.Budget),
	}
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}

func (bed *goalAdmissionBed) amendReviewChain(t *testing.T, _ string, mutate func(*goal.GoalFile)) {
	t.Helper()
	root := bed.root
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture before amendment: %v", problems)
	}
	mutate(file)
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}

func (bed *goalAdmissionBed) commitRisk(t *testing.T, risk *goal.RiskRecord, tier uint8) {
	t.Helper()
	root := bed.root
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal binding fixture: %v", problems)
	}
	file.Risk = risk
	file.Tier = tier
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
}
