package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// proofAdmissionRepository stores complete immutable repository trees. Goal
// reads use installation paths; receipt reads use repository paths.
type proofAdmissionRepository struct {
	mu                  sync.Mutex
	top, root           string
	commits             map[string]proofAdmissionCommit
	canonical, accepted string
	sequence            uint64
	operations          map[string]string
}
type proofAdmissionCommit struct {
	parent, operation string
	files             map[string][]byte
	at                time.Time
}

func proofAdmissionClone(files map[string][]byte) map[string][]byte {
	copied := make(map[string][]byte, len(files))
	for p, data := range files {
		copied[p] = append([]byte(nil), data...)
	}
	return copied
}
func (r *proofAdmissionRepository) next(parent, operation string, files map[string][]byte) string {
	r.sequence++
	digest := sha1.Sum([]byte(fmt.Sprintf("proof-admission-%d:%s:%s", r.sequence, parent, operation)))
	id := fmt.Sprintf("%x", digest)
	r.commits[id] = proofAdmissionCommit{parent: parent, operation: operation, files: proofAdmissionClone(files), at: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)}
	return id
}
func (r *proofAdmissionRepository) known(commit string) (proofAdmissionCommit, error) {
	c, ok := r.commits[commit]
	if !ok {
		return proofAdmissionCommit{}, fmt.Errorf("unknown proof repository commit %q", commit)
	}
	return c, nil
}
func (r *proofAdmissionRepository) validOperation(opid string) error {
	if opid == "" || strings.ContainsAny(opid, "/\\ \t\n") {
		return fmt.Errorf("invalid proof repository operation %q", opid)
	}
	return nil
}
func (r *proofAdmissionRepository) Capture(opid string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validOperation(opid); err != nil {
		return "", err
	}
	if _, err := r.known(r.canonical); err != nil {
		return "", err
	}
	r.operations[opid] = r.canonical
	return r.canonical, nil
}
func (r *proofAdmissionRepository) Accepted() (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.known(r.accepted); err != nil {
		return "", false, err
	}
	return r.accepted, true, nil
}
func proofAdmissionGoalPath(p string) bool {
	if p == "plans/goals/backlog.md" || p == "plans/goals/trunk-red.json" {
		return true
	}
	for _, prefix := range []string{"plans/goals/", "plans/goals/done/", "records/goals/", "plans/channel/"} {
		if strings.HasPrefix(p, prefix) && len(p) > len(prefix) && path.Clean(p) == p && !strings.Contains(p, "..") {
			return true
		}
	}
	return false
}
func proofAdmissionReadPrefix(p string) bool {
	switch p {
	case "plans/goals/", "records/goals/", "plans/channel/", "plans/goals/backlog.md", "plans/goals/done/", "plans/goals/trunk-red.json":
		return true
	}
	return proofAdmissionGoalPath(p)
}
func (r *proofAdmissionRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.known(commit)
	if err != nil {
		return nil, err
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("proof repository requires read prefixes")
	}
	result := map[string][]byte{}
	for _, prefix := range prefixes {
		if !proofAdmissionReadPrefix(prefix) {
			return nil, fmt.Errorf("unknown proof repository read prefix %q", prefix)
		}
		for full, data := range c.files {
			relative, ok := strings.CutPrefix(full, "metasystem/")
			if !ok {
				continue
			}
			if strings.HasPrefix(relative, prefix) {
				result[relative] = append([]byte(nil), data...)
			}
		}
	}
	return result, nil
}
func (r *proofAdmissionRepository) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validOperation(opid); err != nil {
		return "", err
	}
	if strings.TrimSpace(message) == "" || len(changes) == 0 {
		return "", fmt.Errorf("incomplete proof repository operation %q", opid)
	}
	if captured, ok := r.operations[opid]; !ok || captured != parent {
		return "", fmt.Errorf("operation %q was not captured at parent %q", opid, parent)
	}
	c, err := r.known(parent)
	if err != nil {
		return "", err
	}
	files := proofAdmissionClone(c.files)
	seen := map[string]bool{}
	for _, change := range changes {
		if !proofAdmissionGoalPath(change.Path) {
			return "", fmt.Errorf("undeclared proof repository write %q", change.Path)
		}
		if seen[change.Path] || change.Delete && len(change.Content) != 0 {
			return "", fmt.Errorf("invalid proof repository change %q", change.Path)
		}
		seen[change.Path] = true
		full := "metasystem/" + change.Path
		if change.Delete {
			delete(files, full)
		} else {
			files[full] = append([]byte(nil), change.Content...)
		}
	}
	return r.next(parent, opid, files), nil
}
func (r *proofAdmissionRepository) Publish(parent, commit string) (goal.CASOutcome, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.known(commit)
	if err != nil {
		return goal.CASUnknown, err
	}
	if _, err := r.known(parent); err != nil {
		return goal.CASUnknown, err
	}
	if c.parent != parent || c.operation == "" {
		return goal.CASUnknown, fmt.Errorf("proof repository publication has wrong parent or no operation")
	}
	if r.canonical != parent {
		return goal.CASRefused, fmt.Errorf("proof repository canonical parent moved")
	}
	r.canonical = commit
	return goal.CASLanded, nil
}
func (r *proofAdmissionRepository) ancestor(ancestor, descendant string) bool {
	for descendant != "" {
		if descendant == ancestor {
			return true
		}
		descendant = r.commits[descendant].parent
	}
	return false
}
func (r *proofAdmissionRepository) AcceptedCAS(old, next string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.known(old); err != nil {
		return err
	}
	if _, err := r.known(next); err != nil {
		return err
	}
	if r.accepted != old || !r.ancestor(old, next) || !r.ancestor(next, r.canonical) {
		return fmt.Errorf("proof repository accepted compare or ancestry failed")
	}
	r.accepted = next
	return nil
}
func (r *proofAdmissionRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.known(ancestor); err != nil {
		return false, err
	}
	if _, err := r.known(descendant); err != nil {
		return false, err
	}
	return r.ancestor(ancestor, descendant), nil
}
func (r *proofAdmissionRepository) TrailerPresent(tip, opid string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validOperation(opid); err != nil {
		return false, err
	}
	if _, err := r.known(tip); err != nil {
		return false, err
	}
	for tip != "" {
		c := r.commits[tip]
		if c.operation == opid {
			return true, nil
		}
		tip = c.parent
	}
	return false, nil
}
func (r *proofAdmissionRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	return "", fmt.Errorf("proof repository refuses CommitWithTrailer")
}
func (r *proofAdmissionRepository) CommitTime(commit string) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.known(commit)
	return c.at, err
}
func (r *proofAdmissionRepository) Release(opid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validOperation(opid); err != nil {
		return err
	}
	if _, ok := r.operations[opid]; !ok {
		return fmt.Errorf("unknown proof repository operation %q", opid)
	}
	delete(r.operations, opid)
	return nil
}
func (r *proofAdmissionRepository) seed(changes map[string][]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	files := map[string][]byte{}
	if r.canonical != "" {
		files = proofAdmissionClone(r.commits[r.canonical].files)
	}
	for p, data := range changes {
		if p != "metasystem/memory/receipts.log" && p != "metasystem/metasystem.conf" && p != "development/metasystem-design.md" && !strings.HasPrefix(p, "metasystem/plans/goals/") {
			panic("unknown proof fixture seed path: " + p)
		}
		files[p] = append([]byte(nil), data...)
	}
	tip := r.next(r.canonical, "", files)
	r.canonical, r.accepted = tip, tip
}
func (r *proofAdmissionRepository) receiptTip(root string) (string, bool, error) {
	if root != r.root {
		return "", false, fmt.Errorf("proof receipt root %q differs from %q", root, r.root)
	}
	return r.Accepted()
}
func (r *proofAdmissionRepository) receiptTop(root string) (string, error) {
	if root != r.root {
		return "", fmt.Errorf("proof receipt root %q differs from %q", root, r.root)
	}
	return r.top, nil
}
func (r *proofAdmissionRepository) receiptFile(root, tip, p string) ([]byte, bool, error) {
	if root != r.root || p != "metasystem/memory/receipts.log" {
		return nil, false, fmt.Errorf("unknown proof receipt coordinate %q %q", root, p)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if tip != r.accepted {
		return nil, false, fmt.Errorf("proof receipt tip %q is not accepted", tip)
	}
	c, err := r.known(tip)
	if err != nil {
		return nil, false, err
	}
	data, ok := c.files[p]
	return append([]byte(nil), data...), ok, nil
}
func (r *proofAdmissionRepository) goalFile(t *testing.T, id string) *goal.GoalFile {
	t.Helper()
	data := r.rawFile(t, "metasystem/plans/goals/"+id+".md")
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse proof goal %s: %v", id, problems)
	}
	return file
}
func (r *proofAdmissionRepository) rawFile(t *testing.T, p string) []byte {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	data, ok := r.commits[r.accepted].files[p]
	if !ok {
		t.Fatalf("proof repository has no accepted file %s", p)
	}
	return append([]byte(nil), data...)
}
func (r *proofAdmissionRepository) reads() dispatchcore.ProofAdmissionReads {
	return dispatchcore.ProofAdmissionReads{
		ResolveEndpoint: func(root string) (goal.Endpoint, error) {
			if root != r.root {
				return goal.Endpoint{}, fmt.Errorf("proof endpoint root %q differs from %q", root, r.root)
			}
			return goal.Endpoint{Root: r.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: r}, nil
		},
		ResolveMachine: func(root string) (string, error) {
			if root != r.root {
				return "", fmt.Errorf("proof machine root %q differs from %q", root, r.root)
			}
			return "mac-cli", nil
		},
		Receipt: dispatchcore.ReceiptAdmissionSource{AcceptedLedgerTip: r.receiptTip, TopLevel: r.receiptTop, FileAt: r.receiptFile},
	}
}

func (r *proofAdmissionRepository) extendBudgetInputs(t *testing.T) syncRequestDependencies {
	t.Helper()
	backlog := filepath.Join(r.root, "plans", "goals", "backlog.md")
	if err := os.WriteFile(backlog, r.rawFile(t, "metasystem/plans/goals/backlog.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(r.root, "scripts", "agents", "pre-commit-guard.sh")
	if err := os.MkdirAll(filepath.Dir(guard), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(guard, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reads := r.reads()
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts = goalAuthorityReadFacts{
		repositoryTop: r.receiptTop,
		ledgerIdentity: func(root string) string {
			if root != r.root {
				t.Fatalf("ledger identity root = %q, want %q", root, r.root)
			}
			physical, err := os.ReadFile(backlog)
			if err != nil {
				t.Fatal(err)
			}
			accepted := r.rawFile(t, "metasystem/plans/goals/backlog.md")
			if string(physical) != string(accepted) {
				t.Fatal("physical backlog differs from the accepted root")
			}
			record, problems := goal.ParseRoot(accepted)
			if record == nil || len(problems) != 0 {
				t.Fatalf("accepted backlog root: record=%+v problems=%v", record, problems)
			}
			return record.Identity
		},
	}
	dependencies.ensureGuard = func(root string) error {
		if root != r.root {
			return fmt.Errorf("guard root %q differs from %q", root, r.root)
		}
		info, err := os.Stat(guard)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			return fmt.Errorf("fixture guard is not executable: %s", guard)
		}
		return nil
	}
	dependencies.endpoint = reads.ResolveEndpoint
	dependencies.machine = reads.ResolveMachine
	return dependencies
}

func (r *proofAdmissionRepository) commandNow(now time.Time) func(string) (time.Time, error) {
	return func(root string) (time.Time, error) {
		if root != r.root {
			return time.Time{}, fmt.Errorf("admission clock root %q differs from %q", root, r.root)
		}
		return now, nil
	}
}
func (r *proofAdmissionRepository) amend(t *testing.T, id string, mutate func(*goal.GoalFile)) *goal.GoalFile {
	t.Helper()
	file := r.goalFile(t, id)
	mutate(file)
	r.seed(map[string][]byte{"metasystem/plans/goals/" + id + ".md": goal.RenderFile(file)})
	return file
}
func (r *proofAdmissionRepository) addCandidate(t *testing.T, id, arc string, risk *goal.RiskRecord) *goal.GoalFile {
	t.Helper()
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 480, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	openedAt, approvedAt := "2026-08-30T08:10:00Z", "2026-08-30T08:11:00Z"
	openOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "mac-cli", id)
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "mac-cli", id)
	file := &goal.GoalFile{Id: id, State: goal.StateApproved, Tier: 3, Risk: risk, Intent: "Prove candidate " + id + ".", Arc: arc, Origin: goal.OriginMain, NextStep: "Prove it.", OpenedAt: openedAt, Revision: 2, Budget: &budget,
		Approved: &goal.ApprovalRecord{By: "human:Wido", At: approvedAt, Revision: 2, EpisodeRevision: 2, Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven},
		History:  []goal.HistoryLine{{At: openedAt, Opid: openOpid, Verb: "open", Actor: "mac-cli+" + id, Targets: []string{id}, Keep: -1}, {At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{id}, Keep: -1}}}
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, budget, risk)
	r.seed(map[string][]byte{"metasystem/plans/goals/" + id + ".md": goal.RenderFile(file)})
	return file
}
func (r *proofAdmissionRepository) rebudget(t *testing.T, id string, now time.Time) *goal.GoalFile {
	return r.amend(t, id, func(file *goal.GoalFile) {
		file.Revision++
		file.Budget.AttemptLimit++
		event := goal.HistoryLine{At: now.UTC().Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAX", "mac-cli", id), Verb: "set-budget", Actor: "human:Wido", Targets: []string{id}, Keep: -1}
		file.History = append(file.History, event)
		file.Approved = &goal.ApprovalRecord{By: event.Actor, At: event.At, Revision: file.Revision, EpisodeRevision: file.Revision, Opid: event.Opid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)}
		file.BudgetExtension = nil
	})
}
func newProofAdmissionRepositoryFixture(t *testing.T, now time.Time, extension bool) *proofAdmissionRepository {
	t.Helper()
	top := t.TempDir()
	root := filepath.Join(top, "metasystem")
	for _, dir := range []string{root, filepath.Join(top, "development"), filepath.Join(root, "plans", "goals")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(top, "development", "metasystem-design.md")
	if err := os.WriteFile(marker, []byte("fixture template marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n")
	if extension {
		conf = append(conf, []byte("metasystem.budget.tier-3=8h/1/1200m/1/3\n")...)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), conf, 0o644); err != nil {
		t.Fatal(err)
	}
	pinProofBinaryFixture(t, root)
	conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	openedAt := now.Add(-time.Hour).Format(time.RFC3339)
	claimAt := now.Add(-55 * time.Minute).Format(time.RFC3339)
	approvedAt := now.Add(-54 * time.Minute).Format(time.RFC3339)
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	risk := &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises an admitted tier-three goal."}
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "mac-cli", "m1")
	file := &goal.GoalFile{Id: "standing-validation", State: goal.StateClaimed, Tier: 3, Intent: "Govern validation.", Origin: goal.OriginMain, NextStep: "Run it.", OpenedAt: openedAt, Revision: 3, Budget: budget, Risk: risk,
		Claimed:  &goal.ClaimRecord{Machine: "mac-cli", Lineage: "m1", At: claimAt, Revision: 2},
		Approved: &goal.ApprovalRecord{By: "human:Wido", At: approvedAt, Revision: 3, Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest("Govern validation.", 3, *budget, risk)},
		History: []goal.HistoryLine{{At: openedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-cli", "m1"), Verb: "open", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: claimAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-cli", "m1"), Verb: "claim", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{"standing-validation"}, Keep: -1}}}
	repo := &proofAdmissionRepository{top: top, root: root, commits: map[string]proofAdmissionCommit{}, operations: map[string]string{}}
	repo.seed(map[string][]byte{"development/metasystem-design.md": []byte("fixture template marker\n"), "metasystem/metasystem.conf": conf,
		"metasystem/plans/goals/backlog.md":             goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}),
		"metasystem/plans/goals/standing-validation.md": goal.RenderFile(file)})
	if extension {
		repo.amend(t, "standing-validation", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.AttemptLimit = 1
			file.Budget.ReservedJobMinutesLimit = 10000
			file.Budget.ActiveJobLimit = 10
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		receiptAt := now.Add(-time.Hour)
		receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=standing-validation|note=proof fixture\n", receiptAt.Unix(), receiptAt.Format(time.RFC3339))
		repo.seed(map[string][]byte{"metasystem/memory/receipts.log": []byte(receipt)})
	}
	return repo
}
func proofAdmissionExtensionFixture(t *testing.T) (*proofAdmissionRepository, time.Time) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	return newProofAdmissionRepositoryFixture(t, now, true), now
}
func admitCandidateProofLaunchWithRepository(t *testing.T, r *proofAdmissionRepository, request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	t.Helper()
	previous := request.BeforePublish
	request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
		if previous != nil {
			previous(reservation)
		}
		*reservation = proofrun.WithTestHostLoadSampler(*reservation, "0")
		*reservation = privateProofAdmissionRequest(*reservation)
	}
	reads := r.reads()
	classify := func(root string, callerPID int64) (lease.ClassifyResult, error) {
		return classifyVerbCallerWith(root, callerPID, reads.Receipt.TopLevel)
	}
	return admitProofLaunchWithReadsAndClassifier(candidateProofLaunchAdmission(request), func() dispatchcore.ProofAdmissionReads { return reads }, classify)
}
