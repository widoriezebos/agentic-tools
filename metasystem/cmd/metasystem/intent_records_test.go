package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const recordsPath = "metasystem/plans/designs/records.md"

type recordsTransport struct{ bed *recordsBed }

func (x recordsTransport) RemoteTip(string, string, string) (string, bool, error) {
	return x.bed.published, x.bed.published != "", nil
}
func (x recordsTransport) Fetch(string, string, string, string) error {
	x.bed.t.Fatal("unexpected transport fetch")
	return nil
}
func (x recordsTransport) Push(string, string, string, string, string) (branch.CASOutcome, error) {
	x.bed.t.Fatal("publication must use the push owner")
	return "", nil
}

// recordsBed models only the Git reads and mutations this route owns. No
// fixture setup or command falls back to the host's Git.
type recordsBed struct {
	*intentBed
	owners                                      intentOwners
	top, worktree, lane, tip, published, staged string
	requests                                    []branch.CommitRequest
	pushes                                      []branch.PushRequest
	failPush                                    bool
	section, token                              bool
	checked, restored                           bool
	conflict, registered                        bool
	captures                                    [][]string
	callerHead                                  string
	capturedMode                                string
	applied                                     []string
}

func newRecordsBed(t *testing.T) *recordsBed {
	t.Helper()
	bed := newIntentBed(t, false, nil)
	top, err := filepath.EvalSymlinks(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	install := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(install, 0755); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(top)
	for _, entry := range entries {
		if entry.Name() != "metasystem" {
			if err := os.Rename(filepath.Join(top, entry.Name()), filepath.Join(install, entry.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	bed.facts.root = install
	conf := filepath.Join(install, "metasystem.conf")
	file, err := os.OpenFile(conf, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("metasystem.template=true\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	b := &recordsBed{intentBed: bed, top: top, worktree: top, lane: t.TempDir(), tip: strings.Repeat("a", 40)}
	for _, path := range []string{recordsPath, "metasystem/records/decision.md"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(top, path)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(top, path), []byte("record"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	b.owners = bed.owners()
	b.owners.resolver = stateroot.NewResolver(fakeTop(top), noExecutable)
	b.owners.work.git = b.git
	b.owners.connection = intentConnectionOwners{
		transport:   recordsTransport{b},
		endpoint:    bed.dependencies().endpoint,
		endpointTip: func(string, goal.Endpoint) (string, error) { return strings.Repeat("a", 40), nil },
		claimCheck:  func(string, string, goal.Endpoint) func() error { return func() error { return nil } },
		isolate:     func(string, string) error { return nil },
		section: func(_ string, body func(func(func() error) error) error) error {
			b.section = true
			defer func() { b.section = false }()
			return body(func(commit func() error) error { b.token = true; defer func() { b.token = false }(); return commit() })
		},
		captureChanges: func(top, head, _ string, paths ...string) (manualCapture, error) {
			b.captures = append(b.captures, slices.Clone(paths))
			patch := []byte("records patch")
			if head == strings.Repeat("b", 40) {
				patch = nil
			}
			return manualCapture{source: top, head: head, tree: "captured", paths: slices.Clone(paths), patch: patch}, nil
		},
		recordsCheck: func(string) error {
			if !b.section || (b.staged == "" && b.tip != strings.Repeat("b", 40)) {
				t.Fatal("check must read staged records inside the checkout lock")
			}
			b.checked = true
			return nil
		},
		applyIndex: func(_ string, patch []byte, reverse bool) error {
			if !reverse {
				b.applied = append(b.applied, string(patch))
				if len(patch) == 0 {
					return errors.New("No valid patches in input")
				}
			}
			if b.conflict {
				return errors.New("conflicting goal branch edit")
			}
			if reverse {
				b.staged = ""
				b.restored = true
			} else {
				b.staged = string(patch)
			}
			return nil
		},
		commit: func(req branch.CommitRequest) (string, error) {
			if !b.section || !b.token || !b.checked || b.staged == "" {
				t.Fatal("commit needs checked staging and the commit owner's authority")
			}
			b.requests = append(b.requests, req)
			b.tip = strings.Repeat("b", 40)
			b.staged = ""
			return b.tip, nil
		},
		push: func(req branch.PushRequest) (branch.PushResult, error) {
			if b.section {
				t.Fatal("publication ran inside checkout lock")
			}
			b.pushes = append(b.pushes, req)
			if b.failPush {
				b.failPush = false
				return branch.PushResult{}, errors.New("remote unavailable")
			}
			b.published = b.tip
			return branch.PushResult{Tip: b.tip}, nil
		},
	}
	b.owners.delivery = &intentDeliveryOwners{
		now:         func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) },
		laneRoot:    func(string, time.Time) (string, bool, error) { return b.lane, true, nil },
		laneInstall: func(string) (string, error) { return b.lane, nil },
		landingGate: func(*intentInvocation, string, string) (string, error) { return "records", nil },
		branchState: func(string, string) (intentBranchState, error) {
			return intentBranchState{BranchTip: b.published, Status: branch.Status{Commits: []branch.Commit{{ID: b.published, Kind: branch.Plan}}}}, nil
		},
	}
	return b
}

func (b *recordsBed) git(dir string, args ...string) ([]byte, error) {
	b.t.Helper()
	if len(args) > 0 && args[0] == "--literal-pathspecs" {
		args = args[1:]
	}
	key := strings.Join(args, " ")
	switch {
	case key == "rev-parse --show-toplevel":
		return []byte(b.top), nil
	case strings.HasPrefix(key, "rev-parse"):
		if dir == b.top && key == "rev-parse --verify HEAD^{commit}" && b.callerHead != "" {
			return []byte(b.callerHead), nil
		}
		if strings.Contains(key, "refs/heads/goal/") && b.tip == strings.Repeat("a", 40) {
			return nil, errors.New("no branch")
		}
		return []byte(b.tip), nil
	case key == "worktree list --porcelain":
		if b.tip == strings.Repeat("a", 40) && !b.registered {
			return nil, nil
		}
		return []byte("worktree " + b.worktree + "\nbranch refs/heads/goal/standing-validation\n\n"), nil
	case strings.HasPrefix(key, "worktree add"):
		b.registered = true
		b.worktree = args[len(args)-2]
		if err := os.MkdirAll(filepath.Join(b.worktree, "metasystem"), 0755); err != nil {
			return nil, err
		}
		b.t.Cleanup(func() { os.RemoveAll(b.worktree) })
		return nil, nil
	case key == "merge-base "+strings.Repeat("a", 40)+" "+b.tip:
		return []byte(strings.Repeat("a", 40)), nil
	case strings.HasPrefix(key, "rev-list"):
		if b.tip == strings.Repeat("a", 40) {
			return nil, nil
		}
		return []byte(b.tip + " " + strings.Repeat("a", 40)), nil
	case strings.Contains(key, "--format=%(trailers:"):
		return []byte("records\n\nGoal-Plan: standing-validation\n"), nil
	case strings.HasPrefix(key, "diff-tree"):
		return nil, nil
	case strings.HasPrefix(key, "ls-tree -z"):
		mode := b.capturedMode
		if mode == "" {
			mode = "100644"
		}
		return []byte(mode + " blob " + strings.Repeat("c", 40) + "\t" + args[len(args)-1] + "\x00"), nil
	case strings.HasPrefix(key, "diff --cached --binary"):
		return []byte(b.staged), nil
	case strings.HasPrefix(key, "diff --name-only -z --no-renames"):
		if b.tip == strings.Repeat("b", 40) || args[5] == "unchanged" {
			return nil, nil
		}
		return []byte(strings.Join(args[7:], "\x00") + "\x00"), nil
	case strings.HasPrefix(key, "diff --binary --full-index --no-renames"):
		if args[4] != b.tip {
			return nil, fmt.Errorf("patch base %s must be the goal tip %s", args[4], b.tip)
		}
		return []byte("records patch"), nil
	case strings.HasPrefix(key, "diff") || strings.HasPrefix(key, "ls-files"):
		return nil, nil
	default:
		b.t.Fatalf("unexpected Git call in %s: %v", dir, args)
		return nil, nil
	}
}

func (b *recordsBed) land(paths ...string) (int, intentResult) {
	b.t.Helper()
	args := []string{"work", "land", "standing-validation", "--delivered", "The design is accepted"}
	for _, path := range paths {
		args = append(args, "--records", path)
	}
	return b.runJSON(b.owners, args...)
}

func TestRecordsRefusePathsThatAreNotFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ path, reason string }{
		{"metasystem/plans/designs", "is a directory (2 files under it: metasystem/plans/designs/link.md, " + recordsPath + ")"},
		{"metasystem/plans/designs/*.md", "is a glob (2 files it matches: metasystem/plans/designs/link.md, " + recordsPath + ")"},
		{"metasystem/plans/designs/missing.md", "is missing"},
		{"metasystem/plans/designs/link.md", "is not a regular file"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			b := newRecordsBed(t)
			if err := os.Symlink("records.md", filepath.Join(b.top, "metasystem/plans/designs/link.md")); err != nil {
				t.Fatal(err)
			}
			code, result := b.land(recordsPath, tc.path)
			want := "--records takes record files; " + tc.path + " " + tc.reason + "; name the files"
			if code == 0 || result.Outcome != intentRefused || result.Summary != want || len(b.captures) != 0 || len(b.requests) != 0 || len(b.pushes) != 0 {
				t.Fatalf("refused before capture: %d %+v captures=%v", code, result, b.captures)
			}
		})
	}
}

func TestRecordsSingleFileHandsIn(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land(recordsPath)
	entries, err := plain.Entries(b.lane)
	if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 1 || len(b.pushes) != 1 || err != nil || len(entries) != 1 || entries[0].SHA != b.published || len(b.captures) != 1 || !slices.Equal(b.captures[0], []string{recordsPath}) {
		t.Fatalf("single record handed in: %d %+v entries=%v %v", code, result, entries, err)
	}
}

func TestRecordsHandInSkipsRebase(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land(recordsPath)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("records hand-in without a rebase owner: %d %+v", code, result)
	}
	if data := result.Data.(map[string]any); data["rebase"] != nil {
		t.Fatalf("records hand-in adds rebase metadata: %+v", data)
	}
	entries, err := plain.Entries(b.lane)
	if err != nil || len(entries) != 1 || entries[0].SHA != b.published || !entries[0].Records {
		t.Fatalf("records keep the published tip: %+v %v", entries, err)
	}
}

func TestRecordsQueueKeepsTheThroughSelection(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	if _, _, err := plain.HandIn(b.lane, plain.Line{Goal: "standing-validation", Branch: "goal/standing-validation", SHA: b.landing.status.BranchTip, Records: true}); err != nil {
		t.Fatal(err)
	}
	through := b.landing.status.Status.Units[0].Commit
	code, result := b.land("--through", through)
	entries, err := plain.Entries(b.lane)
	if code != 0 || result.Outcome != intentConfirmed || b.calls != 0 || err != nil || len(entries) != 2 || entries[1].SHA != through || entries[1].Records {
		t.Fatalf("selected code commit handed in: %d %+v entries=%+v %v", code, result, entries, err)
	}
}

func TestRecordsHandInSummaryKeepsItsSubject(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land(recordsPath)
	want := "goal standing-validation's records at " + plain.Short(b.published)
	if code != 0 || !strings.HasPrefix(result.Summary, want+" handed to the lane;") {
		t.Fatalf("records hand-in summary: %d %+v", code, result)
	}
	code, result = b.land(recordsPath)
	if code != 0 || result.Outcome != intentUnchanged || !strings.HasPrefix(result.Summary, want+" is waiting in the landing lane;") {
		t.Fatalf("records repeat summary: %d %+v", code, result)
	}
}

func TestRecordsRefuseANonRegularCapturedFile(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.capturedMode = "120000"
	code, result := b.land(recordsPath)
	if code == 0 || result.Summary != "--records takes record files; "+recordsPath+" is not a regular file in the captured tree; name the files" || len(b.requests) != 0 || len(b.pushes) != 0 || b.registered {
		t.Fatalf("captured symlink refused before branch preparation: %d %+v", code, result)
	}
}

func TestRecordsLandAsOnePlanCommitOnTheGoalBranch(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land(recordsPath, "metasystem/records/decision.md")
	if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 1 || len(b.pushes) != 1 {
		t.Fatalf("hand-in: %d %+v commits=%v pushes=%v", code, result, b.requests, b.pushes)
	}
	req := b.requests[0]
	if req.Kind != branch.Plan || req.Unit != "" || len(req.Units) != 0 || req.GoalID != "standing-validation" {
		t.Fatalf("one plan request: %+v", req)
	}
	if !strings.Contains(result.Summary, "records at") || !strings.Contains(result.Summary, "handed to the lane") {
		t.Fatal(result.Summary)
	}
	entries, err := plain.Entries(b.lane)
	if err != nil || len(entries) != 1 || entries[0].SHA != b.published || entries[0].Delivered != "The design is accepted" || !entries[0].Records {
		t.Fatalf("published tip handed in: %+v %v", entries, err)
	}
	for _, paths := range b.captures {
		if !slices.Equal(paths, []string{recordsPath, "metasystem/records/decision.md"}) {
			t.Fatalf("capture must narrow to named files: %v", paths)
		}
	}
	if b.goalFile("standing-validation").State != goal.StateClaimed {
		t.Fatal("records landing must keep goal open")
	}
}
func TestRecordsRefuseACodePath(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land("metasystem/cmd/metasystem/main.go")
	if code == 0 || !strings.Contains(result.Summary, "is not a record") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "work review standing-validation --changes") || len(b.requests) != 0 {
		t.Fatalf("code refusal: %d %+v", code, result)
	}
}
func TestRecordsRefuseABehaviorReadme(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"metasystem/plans/README.md", "metasystem/records/README.md"} {
		b := newRecordsBed(t)
		code, result := b.land(path)
		if code == 0 || !strings.Contains(result.Summary, "is not a record") || len(b.captures) != 0 {
			t.Fatalf("README refusal: %d %+v", code, result)
		}
	}
}
func TestRecordsAlreadyOnTheBranchMakeNoSecondCommit(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.tip = strings.Repeat("b", 40)
	code, result := b.land(recordsPath)
	if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 0 || len(b.pushes) != 1 {
		t.Fatalf("repeat publishes existing commit: %d %+v", code, result)
	}
	code, result = b.land(recordsPath)
	entries, err := plain.Entries(b.lane)
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "goal standing-validation's records at bbbbbbbbbbbb is waiting") || len(b.requests) != 0 || err != nil || len(entries) != 1 {
		t.Fatalf("queued repeat stays one hand-in: %d %+v entries=%v %v", code, result, entries, err)
	}
	if _, _, err := plain.Return(b.lane, "standing-validation", "records check failed", time.Time{}); err != nil {
		t.Fatal(err)
	}
	code, result = b.runJSON(b.owners, "work", "land", "standing-validation")
	if code == 0 || result.Outcome != intentRefused || result.Summary != "goal standing-validation's records at bbbbbbbbbbbb was returned: records check failed" || result.Next == nil {
		t.Fatalf("ordinary repeat preserves the records subject: %d %+v", code, result)
	}
}

func TestRecordsExistingGoalStagesTheDiffFromItsTip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, tip, head string
		commits         int
	}{
		{"committed on caller", strings.Repeat("c", 40), strings.Repeat("b", 40), 1},
		{"only in working tree", strings.Repeat("c", 40), strings.Repeat("d", 40), 1},
		{"identical on goal tip", strings.Repeat("b", 40), strings.Repeat("d", 40), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newRecordsBed(t)
			b.tip, b.callerHead, b.worktree = tc.tip, tc.head, t.TempDir()
			if err := os.MkdirAll(filepath.Join(b.worktree, "metasystem"), 0755); err != nil {
				t.Fatal(err)
			}
			capture := b.owners.connection.captureChanges
			b.owners.connection.captureChanges = func(top, head, brief string, paths ...string) (manualCapture, error) {
				c, err := capture(top, head, brief, paths...)
				if len(c.patch) == 0 {
					c.paths = nil
				} else {
					c.patch = []byte("caller-HEAD patch")
				}
				return c, err
			}
			code, result := b.land(recordsPath)
			if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != tc.commits {
				t.Fatalf("hand-in: %d %+v commits=%v", code, result, b.requests)
			}
			wantApplied := []string{}
			if tc.commits != 0 {
				wantApplied = []string{"records patch"}
			}
			if !slices.Equal(b.applied, wantApplied) || b.staged != "" {
				t.Fatalf("goal-tip patch applied: %v; staging=%q", b.applied, b.staged)
			}
			entries, err := plain.Entries(b.lane)
			if err != nil || len(entries) != 1 || entries[0].SHA != b.tip {
				t.Fatalf("goal tip handed in: %+v %v", entries, err)
			}
		})
	}
}
func TestRecordsRefuseABadRecordHead(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.owners.connection.recordsCheck = func(string) error { return fmt.Errorf("%s:3: record head is missing Id", recordsPath) }
	code, result := b.land(recordsPath)
	if code == 0 || !strings.Contains(result.Summary, recordsPath+":3") || len(b.requests) != 0 || len(b.pushes) != 0 || !b.restored {
		t.Fatalf("bad record refuses and restores staging: %d %+v restored=%v", code, result, b.restored)
	}
}
func TestRecordsStartTheGoalBranchAtMain(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	code, result := b.land(recordsPath)
	if code != 0 || len(b.requests) != 1 || b.requests[0].EndpointTip != strings.Repeat("a", 40) || b.requests[0].Repo != filepath.Join(b.worktree, "metasystem") || b.worktree == b.top {
		t.Fatalf("first plan starts at main in goal worktree: %d %+v requests=%v", code, result, b.requests)
	}
}
func TestAFailedPublicationIsPartialAndTheRepeatPublishesIt(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.failPush = true
	code, result := b.land(recordsPath)
	if code == 0 || result.Outcome != intentPartial || !strings.Contains(result.Summary, "but not published") || len(b.requests) != 1 {
		t.Fatalf("partial publication: %d %+v", code, result)
	}
	if entries, _ := plain.Entries(b.lane); len(entries) != 0 {
		t.Fatal("unpublished commit was handed in")
	}
	code, result = b.land(recordsPath)
	if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 1 || len(b.pushes) != 2 || b.pushes[0].OpID != b.pushes[1].OpID {
		t.Fatalf("same commit is published on repeat: %d %+v", code, result)
	}
}
func TestAPlanOnlyBranchLands(t *testing.T) {
	t.Parallel()
	tip := strings.Repeat("b", 40)
	subject, count, refusal := handLandingSubject(nil, "records", "", intentBranchState{BranchTip: tip, Status: branch.Status{Commits: []branch.Commit{{ID: tip, Kind: branch.Plan}}}})
	if refusal != nil || subject != tip || count != 0 {
		t.Fatalf("plan-only tip: %s %d %+v", subject, count, refusal)
	}
}
func TestAnEmptyBranchStillRefuses(t *testing.T) {
	t.Parallel()
	_, _, refusal := handLandingSubject(nil, "records", "", intentBranchState{BranchTip: strings.Repeat("a", 40)})
	if refusal == nil || refusal.Outcome != intentRefused {
		t.Fatalf("empty range: %+v", refusal)
	}
}
func TestRecordsBehindAnUnreadUnitWait(t *testing.T) {
	t.Parallel()
	state := intentBranchState{BranchTip: strings.Repeat("b", 40), Status: branch.Status{Commits: []branch.Commit{{Kind: branch.Unit}, {Kind: branch.Plan}}, Units: []branch.UnitStatus{{Commit: strings.Repeat("a", 40)}}}}
	_, _, refusal := handLandingSubject(nil, "records", "", state)
	if refusal == nil || !strings.Contains(refusal.Summary, "no clean read") {
		t.Fatalf("records wait for unread unit: %+v", refusal)
	}
}
func TestRecordsRefuseConflictingOptions(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	for _, option := range []string{"--through", "--queue-only", "--message", "--exception"} {
		args := []string{"work", "land", "standing-validation", "--records", recordsPath, option}
		if option != "--queue-only" {
			args = append(args, "VALUE")
		}
		code, result := b.runJSON(b.owners, args...)
		if code == 0 || result.Summary != "--records takes no "+option+"; nothing was done" || result.Next == nil || len(b.captures) != 0 {
			t.Fatalf("%s refuses before capture: %d %+v", option, code, result)
		}
	}
}

func TestRecordsAlreadyOnMainLeaveNoBranch(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.owners.connection.captureChanges = func(top, head, _ string, paths ...string) (manualCapture, error) {
		return manualCapture{source: top, head: head, tree: "unchanged"}, nil
	}
	code, result := b.land(recordsPath)
	if code != 0 || result.Outcome != intentUnchanged || result.Summary != recordsPath+" already says this on main" || b.registered || len(b.requests) != 0 || len(b.pushes) != 0 {
		t.Fatalf("unchanged main makes no goal branch: %d %+v", code, result)
	}
}

func TestRecordsConflictingWithTheGoalBranchRefuse(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	b.conflict = true
	code, result := b.land(recordsPath)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "conflicting goal branch edit") || b.staged != "" || len(b.requests) != 0 || len(b.pushes) != 0 {
		t.Fatalf("conflict leaves goal branch untouched: %d %+v", code, result)
	}
}

func TestRecordsKeepTheCapturedVersionWhenTheCallerEditsAgain(t *testing.T) {
	t.Parallel()
	b := newRecordsBed(t)
	capture := b.owners.connection.captureChanges
	reads := 0
	b.owners.connection.captureChanges = func(top, head, brief string, paths ...string) (manualCapture, error) {
		reads++
		if reads > 1 {
			return manualCapture{source: top, head: head, tree: "unchanged"}, nil
		}
		return capture(top, head, brief, paths...)
	}
	code, result := b.land(recordsPath)
	if code != 0 || result.Outcome != intentConfirmed || len(b.requests) != 1 {
		t.Fatalf("the captured bytes reach the branch even if a later checkout read would differ: %d %+v", code, result)
	}
}
