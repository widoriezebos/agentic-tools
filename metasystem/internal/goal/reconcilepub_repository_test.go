package goal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// priorityReconcileBedForEndpoint seeds one immutable tree, including archived
// goals, and materializes those committed bytes as ordinary checkout files.
func priorityReconcileBedForEndpoint(t *testing.T, live, done []*GoalFile) (Endpoint, string) {
	t.Helper()
	store := newFakeGoalStore()
	base := store.canonical
	commit := store.commits[base]
	commit.files = vTree(vRoot(), live, done)
	store.commits[base] = commit
	client := store.client()
	client.accepted = base
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
	if err := WriteBase(root, BaseRecord{Commit: base, WrittenAt: nowISO8601()}); err != nil {
		t.Fatal(err)
	}
	return Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", Repository: client}, base
}

func humanReconcileReqForEndpoint(endpoint Endpoint, ulid string) VerbRequest {
	return VerbRequest{
		Endpoint: endpoint,
		Actor:    Actor{Machine: "mac-a", Lineage: "lin-1", Human: "wido"},
		Ulid:     ulid,
		Now:      time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC),
	}
}

// reconcileForTest gives each session checkout callbacks bound to its own
// endpoint. A recorded base cannot use the HEAD maintenance probe to choose
// its commit, and only this session's accepted publication may become the
// new materialized anchor.
func reconcileForTest(t *testing.T, request VerbRequest) (ReconcileResult, error) {
	t.Helper()
	base, exists, readErr := ReadBase(request.Endpoint.Root)
	if readErr != nil || !exists || base.Commit == "" {
		t.Fatalf("reconcile requires a recorded materialized base: %+v exists=%t err=%v", base, exists, readErr)
	}
	headCalls := 0
	anchors := 0
	result, err := reconcileFor(request,
		func(root string) (string, error) {
			headCalls++
			if root != request.Endpoint.Root || headCalls != 1 {
				t.Fatalf("reconcile HEAD probe root=%q calls=%d; want only one maintenance probe at %q", root, headCalls, request.Endpoint.Root)
			}
			return "", errors.New("HEAD is unavailable for a recorded materialized base")
		},
		func(root, commit string) error {
			anchors++
			if root != request.Endpoint.Root || anchors != 1 {
				t.Fatalf("reconcile anchor root=%q calls=%d; want root=%q and one call", root, anchors, request.Endpoint.Root)
			}
			repo, ok := request.Endpoint.Repository.(*fakeGoalRepository)
			if !ok {
				t.Fatalf("reconcile endpoint has no fixture repository: %T", request.Endpoint.Repository)
			}
			published := acceptedTipForEndpoint(t, request.Endpoint)
			repo.store.mu.Lock()
			trailer := repo.store.commits[commit].trailer
			repo.store.mu.Unlock()
			if commit != published || trailer != request.opid() {
				t.Fatalf("reconcile anchor commit=%q; want this session's published commit=%q", commit, published)
			}
			return nil
		})
	wantHeadCalls := 1
	if request.Actor.Human == "" {
		wantHeadCalls = 0
	}
	if headCalls != wantHeadCalls {
		t.Fatalf("reconcile HEAD calls=%d; want %d", headCalls, wantHeadCalls)
	}
	wantAnchors := 0
	if result.Publish.Outcome == OutcomeConfirmed {
		wantAnchors = 1
	}
	if anchors != wantAnchors {
		t.Fatalf("reconcile anchor calls=%d; want %d for outcome %q", anchors, wantAnchors, result.Publish.Outcome)
	}
	return result, err
}
