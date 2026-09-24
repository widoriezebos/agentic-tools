package goal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newFakeReconcileBed gives committed bytes one owner and materializes them
// as real checkout files. Every requested commit must exist in the fake store.
func newFakeReconcileBed(t *testing.T, live ...*GoalFile) (string, string, Endpoint) {
	t.Helper()
	if len(live) == 0 {
		editable := vGoal("editable", StateQueued)
		editable.Intent, editable.NextStep = "Original intent.", "Original next."
		live = []*GoalFile{editable}
	} else if len(live) == 1 && live[0] == nil {
		live = nil
	}
	store := newFakeGoalStore()
	tip := store.canonical
	commit := store.commits[tip]
	commit.files = vTree(vRoot(), live, nil)
	store.commits[tip] = commit
	repo := store.client()
	repo.accepted = tip
	root := t.TempDir()
	for path, data := range commit.files {
		abs := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteBase(root, BaseRecord{Commit: tip, WrittenAt: nowISO8601()}); err != nil {
		t.Fatal(err)
	}
	return root, tip, Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", Repository: repo}
}

func materializeFakeReconcile(t *testing.T, endpoint Endpoint, commit string) {
	t.Helper()
	files, err := readCommitGoals(endpoint, commit)
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range files {
		abs := filepath.Join(endpoint.Root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteBase(endpoint.Root, BaseRecord{Commit: commit, WrittenAt: nowISO8601()}); err != nil {
		t.Fatal(err)
	}
}

func TestBaseTipUsesPerCallHeadOnlyWithoutRecordedBase(t *testing.T) {
	root, tip, endpoint := newFakeReconcileBed(t)
	if err := os.Remove(baseRecordPath(root)); err != nil {
		t.Fatal(err)
	}
	calls := 0
	head := func(got string) (string, error) {
		calls++
		if got != root || calls != 1 {
			t.Fatalf("head call: root=%q count=%d", got, calls)
		}
		return "  " + tip + "\n", nil
	}
	if got, err := baseTipFor(endpoint, head); err != nil || got != tip || calls != 1 {
		t.Fatalf("fallback: tip=%q calls=%d err=%v", got, calls, err)
	}
	want := errors.New("head unavailable")
	got, err := baseTipFor(endpoint, func(gotRoot string) (string, error) {
		if gotRoot != root {
			t.Fatalf("head root = %q", gotRoot)
		}
		return "", want
	})
	if got != "" || !errors.Is(err, want) || !strings.Contains(err.Error(), "no materialized base and no HEAD") {
		t.Fatalf("head failure: tip=%q err=%v", got, err)
	}
	if _, exists, err := ReadBase(root); err != nil || exists {
		t.Fatalf("fallback wrote a base: exists=%t err=%v", exists, err)
	}
}

func TestBaseTipRecordedCommitUsesEndpointRepository(t *testing.T) {
	root, tip, endpoint := newFakeReconcileBed(t)
	head := func(string) (string, error) { t.Fatal("recorded base read HEAD"); return "", nil }
	if got, err := baseTipFor(endpoint, head); err != nil || got != tip {
		t.Fatalf("recorded base: %q %v", got, err)
	}
	repo := endpoint.Repository.(*fakeGoalRepository)
	delete(repo.store.commits, tip)
	if got, err := baseTipFor(endpoint, head); got != "" || err == nil || !strings.Contains(err.Error(), "the materialized base") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing recorded commit: %q %v", got, err)
	}
	if rec, exists, err := ReadBase(root); err != nil || !exists || rec.Commit != tip {
		t.Fatalf("record changed: %+v %t %v", rec, exists, err)
	}
}

func TestRecordMaterializedAnchorsBeforeDurableBase(t *testing.T) {
	root := t.TempDir()
	endpoint := Endpoint{Root: root}
	commit := strings.Repeat("a", 40)
	want := errors.New("anchor refused")
	calls := 0
	anchor := func(gotRoot, gotCommit string) error {
		calls++
		if gotRoot != root || gotCommit != commit {
			t.Fatalf("anchor: %q %q", gotRoot, gotCommit)
		}
		if _, exists, err := ReadBase(root); err != nil || exists {
			t.Fatalf("base preceded anchor: exists=%t err=%v", exists, err)
		}
		if calls == 1 {
			return want
		}
		return nil
	}
	if err := recordMaterializedFor(endpoint, commit, anchor); !errors.Is(err, want) {
		t.Fatalf("anchor failure = %v", err)
	}
	if _, exists, err := ReadBase(root); err != nil || exists {
		t.Fatalf("failed anchor recorded base: exists=%t err=%v", exists, err)
	}
	if err := recordMaterializedFor(endpoint, commit, anchor); err != nil || calls != 2 {
		t.Fatalf("retry: calls=%d err=%v", calls, err)
	}
	if rec, exists, err := ReadBase(root); err != nil || !exists || rec.Commit != commit || rec.RefreshDue {
		t.Fatalf("durable base: %+v %t %v", rec, exists, err)
	}
}

func TestRefreshAnchorFailureKeepsPendingRecord(t *testing.T) {
	root, tip, endpoint := newFakeReconcileBed(t)
	snapshot, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("anchor refused")
	calls := 0
	_, err = refreshFor(endpoint, tip, snapshot, func(gotRoot, gotCommit string) error {
		calls++
		if gotRoot != root || gotCommit != tip {
			t.Fatalf("anchor: %q %q", gotRoot, gotCommit)
		}
		rec, exists, readErr := ReadBase(root)
		if readErr != nil || !exists || !rec.RefreshDue || rec.Commit != tip || !reflect.DeepEqual(rec.Snapshot, snapshot.Files) {
			t.Fatalf("pending before anchor: %+v %t %v", rec, exists, readErr)
		}
		return want
	})
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("anchor failure: calls=%d err=%v", calls, err)
	}
	if rec, exists, err := ReadBase(root); err != nil || !exists || !rec.RefreshDue || !reflect.DeepEqual(rec.Snapshot, snapshot.Files) {
		t.Fatalf("pending after anchor failure: %+v %t %v", rec, exists, err)
	}
}

func TestRefreshOnlyResolvesPublicationEndpointLazily(t *testing.T) {
	root, tip, endpoint := newFakeReconcileBed(t)
	resolveCalls, anchorCalls := 0, 0
	resolve := func(string) (Endpoint, error) {
		resolveCalls++
		t.Fatal("ordinary refresh resolved publication endpoint")
		return Endpoint{}, nil
	}
	anchor := func(gotRoot, gotCommit string) error {
		anchorCalls++
		if gotRoot != root || gotCommit != tip {
			t.Fatalf("anchor: %q %q", gotRoot, gotCommit)
		}
		return nil
	}
	if _, err := refreshOnlyFor(endpoint, resolve, anchor); err == nil || !strings.Contains(err.Error(), "no refresh is pending") {
		t.Fatalf("no pending refusal: %v", err)
	}
	if err := WriteBase(root, BaseRecord{Commit: tip, RefreshDue: true, Publishing: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshOnlyFor(endpoint, resolve, anchor); err == nil || !strings.Contains(err.Error(), "no snapshot") {
		t.Fatalf("missing snapshot refusal: %v", err)
	}
	snapshot, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBase(root, BaseRecord{Commit: tip, RefreshDue: true, Snapshot: snapshot.Files}); err != nil {
		t.Fatal(err)
	}
	if skipped, err := refreshOnlyFor(endpoint, resolve, anchor); err != nil || len(skipped) != 0 {
		t.Fatalf("ordinary completion: skipped=%v err=%v", skipped, err)
	}
	if resolveCalls != 0 || anchorCalls != 1 {
		t.Fatalf("calls: resolve=%d anchor=%d", resolveCalls, anchorCalls)
	}
	if rec, exists, err := ReadBase(root); err != nil || !exists || rec.RefreshDue || rec.Commit != tip {
		t.Fatalf("completed base: %+v %t %v", rec, exists, err)
	}
}

type tracedReconcileRepository struct {
	*fakeGoalRepository
	events     *[]string
	releaseErr error
}

func (r tracedReconcileRepository) note(event string) { *r.events = append(*r.events, event) }
func (r tracedReconcileRepository) Capture(opid string) (string, error) {
	r.note("capture:" + opid)
	return r.fakeGoalRepository.Capture(opid)
}
func (r tracedReconcileRepository) Release(opid string) error {
	r.note("release:" + opid)
	if err := r.fakeGoalRepository.Release(opid); err != nil {
		return err
	}
	return r.releaseErr
}
func (r tracedReconcileRepository) TrailerPresent(tip, opid string) (bool, error) {
	r.note("trailer:" + tip + ":" + opid)
	return r.fakeGoalRepository.TrailerPresent(tip, opid)
}
func (r tracedReconcileRepository) Accepted() (string, bool, error) {
	r.note("accepted")
	return r.fakeGoalRepository.Accepted()
}
func (r tracedReconcileRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	r.note("files:" + commit + ":" + strings.Join(prefixes, ","))
	return r.fakeGoalRepository.Files(commit, prefixes...)
}
func (r tracedReconcileRepository) IsAncestor(accepted, fetched string) (bool, error) {
	r.note("ancestor:" + accepted + ":" + fetched)
	return r.fakeGoalRepository.IsAncestor(accepted, fetched)
}

func eventIndex(events []string, prefix string) int {
	for i, event := range events {
		if strings.HasPrefix(event, prefix) {
			return i
		}
	}
	return -1
}

func TestRefreshOnlyPublishingUsesEndpointRepositoryAndPreservesGates(t *testing.T) {
	for _, test := range []struct {
		name, stage string
	}{
		{"never published", "trailer"},
		{"foreign identity", "acceptance"},
		{"wrong sync mode", "sync"},
		{"invalid full tree", "validation"},
		{"accepted publication", "refresh"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, base, endpoint := newFakeReconcileBed(t)
			repo := endpoint.Repository.(*fakeGoalRepository)
			opid := "reconcile-crash"
			snapshot, err := CaptureSnapshot(root)
			if err != nil {
				t.Fatal(err)
			}
			tip := base
			if test.stage != "trailer" {
				changes := []Change{{Path: goalsPrefix + "backlog.md", Content: repo.store.commits[base].files[goalsPrefix+"backlog.md"]}}
				switch test.stage {
				case "acceptance":
					foreign := vRoot()
					foreign.Identity = "01J5X000000000000000000001"
					changes[0].Content = RenderRoot(foreign)
				case "sync":
					local := vRoot()
					local.SyncMode = SyncLocal
					changes[0].Content = RenderRoot(local)
				case "validation":
					changes = append(changes, Change{Path: livePath("editable"), Content: []byte("broken goal\n")})
				case "refresh":
					updated := vGoal("editable", StateQueued)
					updated.Intent, updated.NextStep = "Published intent.", "Original next."
					changes = append(changes, Change{Path: livePath("editable"), Content: RenderFile(updated)})
				}
				tip, err = repo.Build(opid, base, changes, "published")
				if err != nil {
					t.Fatal(err)
				}
				repo.store.canonical = tip
			}
			if err := WriteBase(root, BaseRecord{Commit: base, RefreshDue: true, Publishing: true, Opid: opid, Snapshot: snapshot.Files}); err != nil {
				t.Fatal(err)
			}
			var events []string
			tracked := tracedReconcileRepository{fakeGoalRepository: repo, events: &events}
			if test.stage == "refresh" {
				tracked.releaseErr = errors.New("cleanup failed after capture")
			}
			endpoint.Repository = tracked
			resolved, anchored := 0, 0
			resolve := func(gotRoot string) (Endpoint, error) {
				resolved++
				if gotRoot != root || resolved != 1 {
					t.Fatalf("resolve: root=%q count=%d", gotRoot, resolved)
				}
				return endpoint, nil
			}
			anchor := func(gotRoot, gotCommit string) error {
				anchored++
				if gotRoot != root || gotCommit != tip || anchored != 1 {
					t.Fatalf("anchor: root=%q commit=%q count=%d", gotRoot, gotCommit, anchored)
				}
				events = append(events, "anchor")
				return nil
			}
			skipped, runErr := refreshOnlyFor(Endpoint{Root: root, Repository: repo}, resolve, anchor)
			if resolved != 1 || len(repo.captures) != 1 || len(repo.released) != 1 || len(repo.trailerChecks) != 1 ||
				repo.captures[0] != tip || repo.trailerChecks[0] != (fakeTrailerCheck{tip, opid}) {
				t.Fatalf("publication calls: resolve=%d capture=%v release=%v trailer=%v", resolved, repo.captures, repo.released, repo.trailerChecks)
			}
			if len(events) < 3 || !strings.HasPrefix(events[0], "capture:") || events[1] != "release:"+strings.TrimPrefix(events[0], "capture:") ||
				events[2] != "trailer:"+tip+":"+opid {
				t.Fatalf("capture, cleanup and trailer order: %v", events)
			}
			if test.stage == "trailer" {
				if eventIndex(events, "accepted") != -1 || anchored != 0 || runErr == nil || !strings.Contains(runErr.Error(), "never published") {
					t.Fatalf("own-opid refusal ran later gates: events=%v anchor=%d err=%v", events, anchored, runErr)
				}
			} else {
				if eventIndex(events, "accepted") != 3 {
					t.Fatalf("accepted gate order: %v", events)
				}
				if test.stage == "acceptance" {
					if runErr == nil || !strings.Contains(runErr.Error(), "acceptance gates") || eventIndex(events, "ancestor:") != -1 || anchored != 0 {
						t.Fatalf("identity gate: events=%v anchor=%d err=%v", events, anchored, runErr)
					}
				} else {
					ancestor := eventIndex(events, "ancestor:")
					if ancestor < 4 || anchored != 0 && test.stage != "refresh" {
						t.Fatalf("ancestry gate: events=%v anchor=%d", events, anchored)
					}
					if test.stage == "sync" && (runErr == nil || !strings.Contains(runErr.Error(), "sync-mode gate") || eventIndex(events, "files:"+tip+":"+ChannelPrefix) != -1) {
						t.Fatalf("sync gate: events=%v err=%v", events, runErr)
					}
					if test.stage == "validation" && (runErr == nil || !strings.Contains(runErr.Error(), "does not validate") || anchored != 0) {
						t.Fatalf("full-tree gate: events=%v err=%v", events, runErr)
					}
					if test.stage == "refresh" && (runErr != nil || len(skipped) != 0 || anchored != 1 || eventIndex(events, "files:"+tip+":"+ChannelPrefix) < ancestor || eventIndex(events, "anchor") < eventIndex(events, "files:"+tip+":"+ChannelPrefix)) {
						t.Fatalf("validated refresh: events=%v skipped=%v err=%v", events, skipped, runErr)
					}
				}
			}
			rec, exists, readErr := ReadBase(root)
			if readErr != nil || !exists {
				t.Fatalf("base record: %+v %t %v", rec, exists, readErr)
			}
			switch test.stage {
			case "trailer":
				if rec.RefreshDue || rec.Commit != base {
					t.Fatalf("never-published record: %+v", rec)
				}
			case "refresh":
				if rec.RefreshDue || rec.Commit != tip {
					t.Fatalf("completed record: %+v", rec)
				}
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(livePath("editable"))))
				if err != nil || !strings.Contains(string(data), "Published intent.") {
					t.Fatalf("published worktree: %s %v", data, err)
				}
			default:
				if !rec.RefreshDue || rec.Commit != base || anchored != 0 {
					t.Fatalf("refused record: %+v anchor=%d", rec, anchored)
				}
			}
		})
	}
}

func TestReconcileCheckoutDependenciesAreInstanceScoped(t *testing.T) {
	rootA, tipA, endpointA := newFakeReconcileBed(t)
	rootB, tipB, endpointB := newFakeReconcileBed(t)
	repoA := endpointA.Repository.(*fakeGoalRepository)
	repoB := endpointB.Repository.(*fakeGoalRepository)
	second, err := repoB.Build("fixture-second", tipB, []Change{{Path: goalsPrefix + "backlog.md", Content: repoB.store.commits[tipB].files[goalsPrefix+"backlog.md"]}}, "second base")
	if err != nil {
		t.Fatal(err)
	}
	repoB.store.canonical, repoB.accepted, tipB = second, second, second
	if err := WriteBase(rootB, BaseRecord{Commit: tipB}); err != nil {
		t.Fatal(err)
	}
	if rootA == rootB || tipA == tipB || repoA.store == repoB.store {
		t.Fatal("fixtures share a checkout, commit, or repository")
	}
	for _, test := range []struct {
		root, tip, value, ulid string
		endpoint               Endpoint
		repo                   *fakeGoalRepository
	}{
		{rootA, tipA, "First checkout edit.", "01J5X00000000000000000R101", endpointA, repoA},
		{rootB, tipB, "Second checkout edit.", "01J5X00000000000000000R102", endpointB, repoB},
	} {
		editFile(t, test.root, livePath("editable"), func(file *GoalFile) { file.NextStep = test.value })
		headCalls, anchorCalls := 0, 0
		head := func(gotRoot string) (string, error) {
			headCalls++
			if gotRoot != test.root || headCalls != 1 {
				t.Fatalf("crossed HEAD: root=%q calls=%d", gotRoot, headCalls)
			}
			return test.tip, nil
		}
		anchor := func(gotRoot, commit string) error {
			anchorCalls++
			if gotRoot != test.root || anchorCalls != 1 || commit == test.tip {
				t.Fatalf("crossed anchor: root=%q commit=%q calls=%d", gotRoot, commit, anchorCalls)
			}
			if _, ok := test.repo.store.commits[commit]; !ok {
				t.Fatalf("anchor commit belongs to another repository: %s", commit)
			}
			return nil
		}
		req := verbReqFor(test.endpoint, test.ulid, "mac-a")
		req.Actor.Human = "Wido"
		result, err := reconcileFor(req, head, anchor)
		if err != nil || result.Publish.Outcome != OutcomeConfirmed || headCalls != 1 || anchorCalls != 1 {
			t.Fatalf("reconcile %s: result=%+v head=%d anchor=%d err=%v", test.root, result, headCalls, anchorCalls, err)
		}
		if rec, exists, err := ReadBase(test.root); err != nil || !exists || rec.RefreshDue || rec.Commit != result.Publish.Commit {
			t.Fatalf("base in %s: %+v %t %v", test.root, rec, exists, err)
		}
		data, err := os.ReadFile(filepath.Join(test.root, filepath.FromSlash(livePath("editable"))))
		if err != nil || !strings.Contains(string(data), test.value) {
			t.Fatalf("worktree in %s: %s %v", test.root, data, err)
		}
	}
	if repoA.store.canonical == repoB.store.canonical || len(repoA.captures) != 2 || len(repoB.captures) != 2 {
		t.Fatalf("repository calls crossed: A=%v B=%v", repoA.captures, repoB.captures)
	}
	for _, item := range []struct {
		root, value, tip string
	}{
		{rootA, "First checkout edit.", repoA.store.canonical},
		{rootB, "Second checkout edit.", repoB.store.canonical},
	} {
		rec, exists, err := ReadBase(item.root)
		data, readErr := os.ReadFile(filepath.Join(item.root, filepath.FromSlash(livePath("editable"))))
		if err != nil || !exists || rec.Commit != item.tip || readErr != nil || !strings.Contains(string(data), item.value) {
			t.Fatalf("crossed final checkout %s: base=%+v file=%s errors=%v,%v", item.root, rec, data, err, readErr)
		}
	}
}
