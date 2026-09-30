package goal

import (
	"strings"
	"testing"
)

// TestReconcileScopeRefusesAnEditOutsideTheNamedGoals (VOA-21): a scoped
// reconcile checks the rows it would publish, inside its own capture, before
// it records a pending publication. A person previews an edit of goal a and
// names only a; goal b is edited after the preview and before the owner
// captures. The owner refuses the whole session naming b, publishes
// nothing and leaves no pending record. Naming both publishes both.
func TestReconcileScopeRefusesAnEditOutsideTheNamedGoals(t *testing.T) {
	t.Parallel()
	endpoint, base := priorityReconcileBedForEndpoint(t, rankedPriorityGoals(1, "a", "b", "c"), nil)
	editFile(t, endpoint.Root, livePath("a"), func(file *GoalFile) { file.NextStep = "Reviewed next step of a." })

	// The preview the person reviewed: only a differs.
	snapshot, err := CaptureSnapshot(endpoint.Root)
	if err != nil {
		t.Fatal(err)
	}
	previewed, err := diffAgainstBaseFor(endpoint, base, snapshot)
	if err != nil || len(previewed) != 1 || previewed[0].Path != livePath("a") {
		t.Fatalf("preview = %+v %v, want only a", previewed, err)
	}
	// Another writer edits b before the owner captures.
	editFile(t, endpoint.Root, livePath("b"), func(file *GoalFile) { file.NextStep = "Unreviewed next step of b." })

	request := humanReconcileReqForEndpoint(endpoint, "01J5X000000000000000000S10")
	request.ReconcileScope = []string{"a"}
	result, err := reconcileForTest(t, request)
	if err == nil || !(RefusalCode(err) == "RECONCILE_OUTSIDE_SCOPE") || !strings.Contains(err.Error(), "edits of b,") ||
		!strings.Contains(err.Error(), "not only of a") || result.Publish.Tip != "" {
		t.Fatalf("scoped reconcile over an unnamed edit = %+v %v; want a refusal naming b", result, err)
	}
	record, exists, err := ReadBase(endpoint.Root)
	if err != nil || !exists || record.Commit != base || record.RefreshDue || record.Publishing || record.Opid == request.opid() {
		t.Fatalf("the refusal left a pending publication: %+v exists=%t %v", record, exists, err)
	}
	tree, err := loadTreeFor(endpoint, endpoint.Repository.(*fakeGoalRepository).store.canonical)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["a"].NextStep == "Reviewed next step of a." || tree.Live["b"].NextStep == "Unreviewed next step of b." {
		t.Fatalf("a refused session published: a=%q b=%q", tree.Live["a"].NextStep, tree.Live["b"].NextStep)
	}

	// Naming every edited goal publishes them together.
	request = humanReconcileReqForEndpoint(endpoint, "01J5X000000000000000000S20")
	request.ReconcileScope = []string{"b", "a"}
	result, err = reconcileForTest(t, request)
	if err != nil || result.Publish.Outcome != OutcomeConfirmed || len(result.Rows) != 2 {
		t.Fatalf("reconcile naming both edited goals = %+v %v", result, err)
	}
	tree, err = loadTreeFor(endpoint, result.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["a"].NextStep != "Reviewed next step of a." || tree.Live["b"].NextStep != "Unreviewed next step of b." || tree.Live["c"].Revision != 1 {
		t.Fatalf("published tree: a=%+v b=%+v c=%+v", tree.Live["a"], tree.Live["b"], tree.Live["c"])
	}
}

// TestOutsideReconcileScope: an empty scope holds every goal; a cascade row
// names each member it would publish.
func TestOutsideReconcileScope(t *testing.T) {
	t.Parallel()
	rows := []MappedVerb{{Verb: "edit", Id: "a"}, {Verb: "park", Id: "arc", ArcIds: []string{"x", "a"}}}
	if outside := outsideReconcileScope(rows, nil); len(outside) != 0 {
		t.Fatalf("an unscoped session refused %v", outside)
	}
	if outside := outsideReconcileScope(rows, []string{"a"}); strings.Join(outside, ",") != "arc,x" {
		t.Fatalf("outside = %v, want arc and x", outside)
	}
	if outside := outsideReconcileScope(rows, []string{"a", "arc", "x"}); len(outside) != 0 {
		t.Fatalf("a covering scope refused %v", outside)
	}
}
