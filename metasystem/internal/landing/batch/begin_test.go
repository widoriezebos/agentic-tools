package batch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// laneBed is a nested lane checkout (checkout root ≠ installation root) with
// a bare origin, where the landing agent composes series.
type laneBed struct {
	t                       *testing.T
	checkout, install, base string
	store                   Store
}

const laneActor = "m1e+landing-m1l"

var laneBedAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func newLaneBed(t *testing.T) *laneBed {
	t.Helper()
	dir := t.TempDir()
	bed := &laneBed{t: t, checkout: filepath.Join(dir, "lane")}
	bed.install = filepath.Join(bed.checkout, "metasystem")
	origin := filepath.Join(dir, "origin.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init origin: %v %s", err, out)
	}
	if out, err := exec.Command("git", "init", "-q", "-b", "main", bed.checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	bed.write(".gitignore", "/artifacts/\n")
	bed.write("metasystem/metasystem.conf", "metasystem.template=true\n")
	bed.write("metasystem/app/base.txt", numbered("base", 1, 10))
	bed.base = bed.commit("base")
	bed.git("remote", "add", "origin", origin)
	bed.git("push", "-q", "origin", "main")
	bed.git("fetch", "-q", "origin")
	bed.store = NewStore(bed.checkout, nil)
	return bed
}

func (bed *laneBed) git(args ...string) string {
	bed.t.Helper()
	command := exec.Command("git", append([]string{"-C", bed.checkout, "-c", "user.name=Lane Agent", "-c", "user.email=lane@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-01T09:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T09:00:00Z")
	out, err := command.CombinedOutput()
	if err != nil {
		bed.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (bed *laneBed) write(path, content string) {
	bed.t.Helper()
	full := filepath.Join(bed.checkout, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		bed.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		bed.t.Fatal(err)
	}
}

func (bed *laneBed) commit(message string) string {
	bed.t.Helper()
	bed.git("add", "-A")
	bed.git("commit", "-q", "--allow-empty", "-m", message)
	return bed.git("rev-parse", "HEAD")
}

func (bed *laneBed) tree(rev string) string {
	bed.t.Helper()
	return bed.git("rev-parse", rev+"^{tree}")
}

// change makes a change member's commit on main's tip, in the seat's name.
func (bed *laneBed) change(path, content, subject string) Unit {
	bed.t.Helper()
	bed.git("checkout", "-q", "--detach", bed.base)
	bed.write(path, content)
	commit := bed.commit(subject + "\n\nMachine: m1e+human")
	bed.git("checkout", "-q", "main")
	return NewChangeUnit(ChangeMember{Commit: commit, Parent: bed.base, AskedBy: "m1e+human", Subject: subject},
		"/seat", "m1e", "human", []string{path}, nil)
}

// goalBranch makes goal/<id> on main's tip from steps, each one commit;
// a step whose name starts with "fold" is a fold of the next build.
type branchStep struct{ name, path, content string }

func (bed *laneBed) goalBranch(id string, steps []branchStep) (commits map[string]string, builds []BranchBuild) {
	bed.t.Helper()
	bed.git("checkout", "-q", "-B", "goal/"+id, bed.base)
	commits = map[string]string{}
	var folds []goalbranch.Commit
	for _, step := range steps {
		bed.write(step.path, step.content)
		commit := bed.commit(step.name)
		commits[step.name] = commit
		if strings.HasPrefix(step.name, "fold") {
			folds = append(folds, goalbranch.Commit{ID: commit, Kind: goalbranch.Read, Unit: step.name})
			continue
		}
		digest, err := treeTransitionDigest(bed.checkout, commit+"^", commit)
		if err != nil {
			bed.t.Fatal(err)
		}
		builds = append(builds, BranchBuild{Units: []string{step.name}, Commit: commit, Digest: digest, Folds: folds})
		folds = nil
	}
	bed.git("checkout", "-q", "main")
	return commits, builds
}

func goalUnit(id string, builds []BranchBuild, tip string) Unit {
	unit := Unit{GoalID: id, Chain: tip, SeatRoot: "/seat", State: UnitJoined, Approver: "Wido", AuthorName: "Wido Riezebos", AuthorEmail: "wido@example.invalid",
		Claim: Claim{Machine: "m1e", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}}
	return BindBranchMember(unit, BranchMember{GoalID: id, Tip: tip, Last: true, Builds: builds})
}

func (bed *laneBed) batch(id string, units ...Unit) Record {
	bed.t.Helper()
	for index := range units {
		units[index].State = UnitJoined
	}
	record := Record{Schema: 1, BatchID: id, BaseTree: bed.tree(bed.base), TipTree: bed.tree(bed.base), State: StateOpen, Units: units}
	if err := bed.store.Create(record); err != nil {
		bed.t.Fatal(err)
	}
	return record
}

// compose builds the agent's series on main's tip: each step writes files
// and commits with message; it returns the head.
type composeStep struct {
	files   map[string]string
	message string
}

func (bed *laneBed) compose(steps ...composeStep) string {
	bed.t.Helper()
	bed.git("checkout", "-q", "-B", "lane/compose", bed.base)
	for _, step := range steps {
		for path, content := range step.files {
			bed.write(path, content)
		}
		bed.commit(step.message)
	}
	head := bed.git("rev-parse", "HEAD")
	bed.git("checkout", "-q", "main")
	return head
}

func (bed *laneBed) begin(record Record, members []string, head string) (Opening, error) {
	bed.t.Helper()
	return PlanOpening(bed.checkout, record, BeginRequest{BatchID: record.BatchID, Members: members, Base: bed.base, Head: head,
		LedgerRoot: bed.install, Actor: laneActor, At: laneBedAt, OpID: "op-" + record.BatchID[20:]})
}

func numbered(prefix string, from, to int) string {
	var lines strings.Builder
	for index := from; index <= to; index++ {
		fmt.Fprintf(&lines, "%s line %d\n", prefix, index)
	}
	return lines.String()
}

// K4, D3: one aggregate cap covers every agent-authored deviation — each
// Lane-Resolved replay's difference from its pin, plus the integration
// commit — and over it begin refuses, recording seam-too-large evidence for
// the members whose work goes back.
func TestAggregateCapCountsResolvedReplays(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	first := bed.change("metasystem/app/first.txt", numbered("first", 1, 5), "record: first change")
	second := bed.change("metasystem/app/second.txt", numbered("second", 1, 5), "record: second change")
	record := bed.batch("01j5x00000000000000000cap1", first, second)
	members := []string{first.GoalID, second.GoalID}

	// Two resolved replays: 25 and 20 lines of the lane's own, no
	// integration commit — 45 lines, over the cap.
	head := bed.compose(
		composeStep{files: map[string]string{"metasystem/app/first.txt": numbered("first", 1, 5) + numbered("seam a", 1, 25)},
			message: "record: first change\n\nMachine: m1e+human\nLane-Resolved: seam with main's base file"},
		composeStep{files: map[string]string{"metasystem/app/second.txt": numbered("second", 1, 5) + numbered("seam b", 1, 20)},
			message: "record: second change\n\nMachine: m1e+human\nLane-Resolved: seam with the first change"})
	_, err := bed.begin(record, members, head)
	var composition *CompositionRefusal
	if !errors.As(err, &composition) || composition.Evidence.Kind != CompositionSeamTooLarge || composition.Evidence.Deviation != 45 {
		t.Fatalf("resolved replays over the cap = %v (%+v); want seam-too-large at 45 lines", err, composition)
	}
	if !slices.Equal(composition.Evidence.Members, members) {
		t.Fatalf("seam-too-large names %v; want both resolved members %v", composition.Evidence.Members, members)
	}
	if err := RecordComposition(bed.store, record.BatchID, composition.Evidence, laneBedAt); err != nil {
		t.Fatal(err)
	}
	loaded, err := bed.store.Load(record.BatchID)
	if err != nil || len(loaded.Compositions) != 1 || loaded.Compositions[0].Kind != CompositionSeamTooLarge {
		t.Fatalf("recorded composition evidence = %+v, %v", loaded.Compositions, err)
	}

	// One resolved replay (25) plus an integration commit (20) is also 45.
	head = bed.compose(
		composeStep{files: map[string]string{"metasystem/app/first.txt": numbered("first", 1, 5) + numbered("seam a", 1, 25)},
			message: "record: first change\n\nMachine: m1e+human\nLane-Resolved: seam with main's base file"},
		composeStep{files: map[string]string{"metasystem/app/second.txt": numbered("second", 1, 5)}, message: "record: second change\n\nMachine: m1e+human"},
		composeStep{files: map[string]string{"metasystem/app/glue.txt": numbered("glue", 1, 20)}, message: "glue the two\n\nLane-Integration: the combination needs glue"})
	_, err = bed.begin(record, members, head)
	if !errors.As(err, &composition) || composition.Evidence.Deviation != 45 || !slices.Equal(composition.Evidence.Members, members) {
		t.Fatalf("resolved replay plus integration over the cap = %v (%+v); want seam-too-large at 45 naming both", err, composition)
	}

	// Within the cap (25 + 10): begin takes the series.
	head = bed.compose(
		composeStep{files: map[string]string{"metasystem/app/first.txt": numbered("first", 1, 5) + numbered("seam a", 1, 25)},
			message: "record: first change\n\nMachine: m1e+human\nLane-Resolved: seam with main's base file"},
		composeStep{files: map[string]string{"metasystem/app/second.txt": numbered("second", 1, 5)}, message: "record: second change\n\nMachine: m1e+human"},
		composeStep{files: map[string]string{"metasystem/app/glue.txt": numbered("glue", 1, 10)}, message: "glue the two\n\nLane-Integration: the combination needs glue"})
	opening, err := bed.begin(record, members, head)
	if err != nil {
		t.Fatalf("a series within the cap: %v", err)
	}
	kinds := []string{}
	for _, entry := range opening.Series {
		kinds = append(kinds, entry.Kind)
	}
	if opening.Deviation != 35 || !slices.Equal(kinds, []string{SeriesResolved, SeriesReplay, SeriesIntegration}) || opening.Tree != bed.tree(head) {
		t.Fatalf("opening = deviation %d kinds %v tree %s; want 35, resolved/replay/integration, the head's tree %s", opening.Deviation, kinds, opening.Tree, bed.tree(head))
	}
}

// K4: a replay that differs from its pin must say so; an unmarked one is
// refused, and nothing is recorded.
func TestBeginRefusesAnUnmarkedDeviation(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	only := bed.change("metasystem/app/only.txt", numbered("only", 1, 3), "record: only change")
	record := bed.batch("01j5x00000000000000000cap2", only)
	head := bed.compose(composeStep{files: map[string]string{"metasystem/app/only.txt": numbered("only", 1, 4)}, message: "record: only change\n\nMachine: m1e+human"})
	_, err := bed.begin(record, []string{only.GoalID}, head)
	var refusal *BeginRefusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "not marked "+LaneResolvedTrailer) {
		t.Fatalf("unmarked deviation = %v; want a begin refusal naming %s", err, LaneResolvedTrailer)
	}
}

// K4: every joined member is pinned, the series is linear on B, and its
// length is one commit per pinned step plus at most one integration commit.
func TestBeginPinsEveryMemberInALinearSeries(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	first := bed.change("metasystem/app/first.txt", "first\n", "record: first change")
	second := bed.change("metasystem/app/second.txt", "second\n", "record: second change")
	record := bed.batch("01j5x00000000000000000cap3", first, second)
	head := bed.compose(composeStep{files: map[string]string{"metasystem/app/first.txt": "first\n"}, message: "record: first change\n\nMachine: m1e+human"})
	var refusal *BeginRefusal
	if _, err := bed.begin(record, []string{first.GoalID}, head); !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, second.GoalID) {
		t.Fatalf("a member left out = %v; want a refusal naming %s", err, second.GoalID)
	}
	if _, err := bed.begin(record, []string{first.GoalID, second.GoalID}, head); !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "need 2") {
		t.Fatalf("a short series = %v; want a refusal counting the pins", err)
	}
	if _, err := bed.begin(record, []string{first.GoalID, second.GoalID}, bed.base); !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "empty") {
		t.Fatalf("an empty series = %v; want a refusal", err)
	}
	bed.git("checkout", "-q", "-B", "lane/merge", bed.base)
	bed.write("metasystem/app/first.txt", "first\n")
	bed.commit("record: first change")
	bed.git("checkout", "-q", "-B", "lane/other", bed.base)
	bed.write("metasystem/app/second.txt", "second\n")
	bed.commit("record: second change")
	bed.git("merge", "-q", "--no-ff", "-m", "merge", "lane/merge")
	merged := bed.git("rev-parse", "HEAD")
	bed.git("checkout", "-q", "main")
	if _, err := bed.begin(record, []string{first.GoalID, second.GoalID}, merged); !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "merge") {
		t.Fatalf("a merge in the series = %v; want a linear-series refusal", err)
	}
}

// K6, R9-02: member:M is B plus all of M's selected builds with their
// assigned folds, where a later build depends on an earlier one and on a
// fold — and nothing of another member.
func TestMemberSubjectDependentTwoBuilds(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	commits, builds := bed.goalBranch("dependent", []branchStep{
		{name: "build-one", path: "metasystem/app/feature.txt", content: "alpha\nbeta\ngamma\n"},
		{name: "fold-read-fix", path: "metasystem/app/feature.txt", content: "alpha\nBETA\ngamma\n"},
		{name: "build-two", path: "metasystem/app/feature.txt", content: "alpha\nBETA\nGAMMA\ndelta\n"},
	})
	if len(builds) != 2 || len(builds[1].Folds) != 1 {
		t.Fatalf("fixture builds = %+v; want two builds, the second with one fold", builds)
	}
	member := goalUnit("dependent", builds, commits["build-two"])
	other := bed.change("metasystem/app/other.txt", "other member\n", "record: another member")
	bed.batch("01j5x00000000000000000mem1", member, other)
	tree, err := MemberSubjectTree(bed.checkout, bed.tree(bed.base), member)
	if err != nil {
		t.Fatalf("member subject tree: %v", err)
	}
	if want := bed.tree(commits["build-two"]); tree != want {
		t.Fatalf("member:dependent tree = %s; want B plus both builds and the fold, %s", tree, want)
	}
	if content, present, _ := fileAtTree(bed, tree, "metasystem/app/other.txt"); present {
		t.Fatalf("member:dependent holds another member's file: %q", content)
	}
}

// K6, R9-02: a member joined --through a build proves only that prefix: a
// build past it on the goal branch is not part of member:M.
func TestMemberSubjectThroughPrefix(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	commits, builds := bed.goalBranch("prefix", []branchStep{
		{name: "build-one", path: "metasystem/app/one.txt", content: "one\n"},
		{name: "build-two", path: "metasystem/app/two.txt", content: "two\n"},
		{name: "build-three", path: "metasystem/app/three.txt", content: "three\n"},
	})
	// Joined with --through build-two: the retained member holds the first
	// two builds; the branch tip holds the third.
	member := goalUnit("prefix", builds[:2], commits["build-three"])
	member.GoalLast = false
	bed.batch("01j5x00000000000000000mem2", member)
	tree, err := MemberSubjectTree(bed.checkout, bed.tree(bed.base), member)
	if err != nil {
		t.Fatalf("member subject tree: %v", err)
	}
	if want := bed.tree(commits["build-two"]); tree != want {
		t.Fatalf("member:prefix tree = %s; want B plus the --through prefix, %s (tip %s)", tree, want, bed.tree(commits["build-three"]))
	}
}

// K8: a conflict the agent reports is recorded only when the kernel sees it:
// the member's contribution must fail to apply on the series commit named.
func TestRecordConflictIsVerified(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	first := bed.change("metasystem/app/base.txt", numbered("base", 1, 9)+"first's line 10\n", "record: first edits line 10")
	second := bed.change("metasystem/app/base.txt", numbered("base", 1, 9)+"second's line 10\n", "record: second edits line 10")
	record := bed.batch("01j5x00000000000000000cf01", first, second)
	onto := bed.compose(composeStep{files: map[string]string{"metasystem/app/base.txt": numbered("base", 1, 9) + "first's line 10\n"},
		message: "record: first edits line 10\n\nMachine: m1e+human"})
	evidence, err := CheckConflict(bed.checkout, record, ConflictRequest{BatchID: record.BatchID, Member: second.GoalID, Base: bed.base, Onto: onto, Actor: laneActor, OpID: "op-1", At: laneBedAt})
	if err != nil || evidence.Kind != CompositionConflict || !slices.Equal(evidence.Members, []string{second.GoalID}) || !slices.Contains(evidence.Paths, "metasystem/app/base.txt") {
		t.Fatalf("a real conflict = %+v, %v; want conflict evidence naming %s and the file", evidence, err, second.GoalID)
	}
	var refusal *BeginRefusal
	if _, err := CheckConflict(bed.checkout, record, ConflictRequest{BatchID: record.BatchID, Member: first.GoalID, Base: bed.base, Onto: bed.base, Actor: laneActor, OpID: "op-2", At: laneBedAt}); !errors.As(err, &refusal) {
		t.Fatalf("a member that applies = %v; want a refusal, nothing recorded", err)
	}
}

func fileAtTree(bed *laneBed, tree, path string) (string, bool, error) {
	out, err := exec.Command("git", "-C", bed.checkout, "show", tree+":"+path).Output()
	if err != nil {
		return "", false, nil
	}
	return string(out), true, nil
}

// beginOne begins a batch of one change whose pin writes path as pinned and
// whose replay writes it as replayed, with the Lane-Resolved mark when
// marked; extra are further files the replay writes.
func (bed *laneBed) beginOne(t *testing.T, id, path, pinned, replayed string, marked bool, extra map[string]string) (Opening, error) {
	t.Helper()
	unit := bed.change(path, pinned, "record: one change")
	record := bed.batch(id, unit)
	message := "record: one change\n\nMachine: m1e+human"
	if marked {
		message += "\n" + LaneResolvedTrailer + ": a seam"
	}
	files := map[string]string{path: replayed}
	for name, content := range extra {
		files[name] = content
	}
	head := bed.compose(composeStep{files: files, message: message})
	return bed.begin(record, []string{unit.GoalID}, head)
}

// Critique F-1: a whitespace-only edit of the pin is a deviation, never an
// exact replay — in a shell line, an indentation or a string literal it
// changes what runs.
func TestBeginCountsAWhitespaceEdit(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	var refusal *BeginRefusal
	if _, err := bed.beginOne(t, "01j5x00000000000000000ws01", "metasystem/app/clean.sh", "rm -rf /tmp/x/*\n", "rm -rf /tmp/x/ *\n", false, nil); !errors.As(err, &refusal) ||
		!strings.Contains(refusal.Reason, "not marked "+LaneResolvedTrailer) {
		t.Fatalf("an unmarked whitespace edit = %v; want it refused as a deviation", err)
	}
	opening, err := bed.beginOne(t, "01j5x00000000000000000ws02", "metasystem/app/clean2.sh", "rm -rf /tmp/x/*\n", "rm -rf /tmp/x/ *\n", true, nil)
	if err != nil || opening.Deviation != 2 {
		t.Fatalf("a marked whitespace edit = deviation %d, %v; want 2 lines", opening.Deviation, err)
	}
}

// Critique F-2: the deviation does not obey git attributes the agent can
// write: a `-diff` attribute that makes a pinned text file read as binary
// still counts its lines, and a binary the lane changes needs a person.
func TestDeviationIgnoresGitAttributes(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	info := filepath.Join(bed.checkout, ".git", "info", "attributes")
	if err := os.MkdirAll(filepath.Dir(info), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(info, []byte("*.go -diff\n*.bin -diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := bed.beginOne(t, "01j5x00000000000000000at01", "metasystem/app/pinned.go", numbered("pinned", 1, 5), numbered("pinned", 1, 5)+numbered("smuggled", 1, 45), true, nil)
	var composition *CompositionRefusal
	if !errors.As(err, &composition) || composition.Evidence.Deviation < 45 {
		t.Fatalf("a rewrite hidden behind -diff = %v (%+v); want seam-too-large counting its 45 lines", err, composition)
	}
	_, err = bed.beginOne(t, "01j5x00000000000000000at02", "metasystem/app/blob.bin", "pinned\x00bytes\n", "swapped\x00bytes\n", true, nil)
	if !errors.As(err, &composition) || composition.Evidence.Deviation <= SeamCapLines {
		t.Fatalf("a swapped binary = %v (%+v); want it over the cap", err, composition)
	}
}

// Critique F-3: the deviation is ordered, against the pin replayed on the
// series parent: reordering the pin's lines, or removing another identical
// line, is not free; an empty file or a mode change counts too.
func TestDeviationIsOrderedAgainstThePinOnItsParent(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	opening, err := bed.beginOne(t, "01j5x00000000000000000rd01", "metasystem/app/order.txt", "first\nsecond\nthird\n", "third\nsecond\nfirst\n", true, nil)
	if err != nil || opening.Deviation == 0 {
		t.Fatalf("a reordered pin = deviation %d, %v; want it counted", opening.Deviation, err)
	}
	opening, err = bed.beginOne(t, "01j5x00000000000000000rd02", "metasystem/app/base.txt", numbered("base", 1, 10)+"}\nreturn nil\n}\n", numbered("base", 1, 10)+"return nil\n}\n", true, nil)
	if err != nil || opening.Deviation == 0 {
		t.Fatalf("another identical line removed = deviation %d, %v; want it counted", opening.Deviation, err)
	}
	opening, err = bed.beginOne(t, "01j5x00000000000000000rd03", "metasystem/app/plain.txt", "plain\n", "plain\n", true, map[string]string{"metasystem/app/empty.txt": ""})
	if err != nil || opening.Deviation < 1 {
		t.Fatalf("an empty file added in a resolution = deviation %d, %v; want at least 1", opening.Deviation, err)
	}
}

// Re-review: a replace ref cannot hide the lane's own content. The resolved
// replay carries 45 extra lines; `git replace` maps its tree to the exact
// pin's tree, so a diff that honours replace refs sees no change. Begin's
// count must read the real objects.
func TestDeviationIgnoresReplaceRefs(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	unit := bed.change("metasystem/app/replaced.txt", numbered("pinned", 1, 5), "record: one change")
	record := bed.batch("01j5x00000000000000000rp01", unit)
	exact := bed.compose(composeStep{files: map[string]string{"metasystem/app/replaced.txt": numbered("pinned", 1, 5)},
		message: "record: one change\n\nMachine: m1e+human"})
	head := bed.compose(composeStep{files: map[string]string{"metasystem/app/replaced.txt": numbered("pinned", 1, 5) + numbered("smuggled", 1, 45)},
		message: "record: one change\n\nMachine: m1e+human\n" + LaneResolvedTrailer + ": a seam"})
	bed.git("replace", bed.tree(head), bed.tree(exact))
	_, err := bed.begin(record, []string{unit.GoalID}, head)
	var composition *CompositionRefusal
	if !errors.As(err, &composition) || composition.Evidence.Deviation < 45 {
		t.Fatalf("content hidden behind a replace ref = %v (%+v); want seam-too-large counting its 45 lines", err, composition)
	}
}
