package goal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeGoalEndpoint(t *testing.T, live ...*GoalFile) (Endpoint, *fakeGoalRepository) {
	t.Helper()
	store := newFakeGoalStore()
	seed := store.commits[store.canonical]
	seed.files = vTree(vRoot(), live, nil)
	store.commits[store.canonical] = seed
	client := store.client()
	client.accepted = store.canonical
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", Repository: client}, client
}

func fakeGoalEndpointPair(t *testing.T) (Endpoint, Endpoint) {
	t.Helper()
	first, client := fakeGoalEndpoint(t)
	secondClient := client.store.client()
	secondClient.accepted = client.accepted
	secondRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(secondRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := Endpoint{Root: secondRoot, Remote: first.Remote, Branch: first.Branch, Repository: secondClient}
	return first, second
}

func acceptedTipForEndpoint(t *testing.T, endpoint Endpoint) string {
	t.Helper()
	if endpoint.Repository == nil {
		t.Fatal("accepted-tip fixture requires an injected repository")
	}
	tip, present, err := endpoint.Repository.Accepted()
	if err != nil || !present || tip == "" {
		t.Fatalf("accepted tip absent or unreadable: tip=%q present=%t err=%v", tip, present, err)
	}
	return tip
}

func acceptedTreeForEndpoint(t *testing.T, endpoint Endpoint) (*TreeGoals, string) {
	t.Helper()
	tip := acceptedTipForEndpoint(t, endpoint)
	tree, err := loadTreeFor(endpoint, tip)
	if err != nil {
		t.Fatal(err)
	}
	return tree, tip
}

func fakeRootPublishRequest(e Endpoint, opid string) PublishRequest {
	return PublishRequest{
		Opid: opid, Machine: "mac-a", Lineage: "lin-1", Intent: testIntentFor("edit"), Message: "fake protocol mutation",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(vRoot())}}, nil
		},
		Validate: func(commit string) error { return validateCommitFor(e, commit) },
	}
}

func TestGoalRepositoryFakePublishProtocol(t *testing.T) {
	t.Parallel()
	t.Run("confirmed trailer and refetch", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		result, err := Publish(e, fakeRootPublishRequest(e, "fake-confirmed"))
		if err != nil || result.Outcome != OutcomeConfirmed || result.Tip != result.Commit {
			t.Fatalf("confirmation: %+v %v", result, err)
		}
		if len(client.captures) != 2 || client.captures[1] != result.Tip || len(client.trailerChecks) != 1 || client.trailerChecks[0] != (fakeTrailerCheck{result.Tip, "fake-confirmed"}) {
			t.Fatalf("confirmation did not refetch and query its tip: captures=%v trailer checks=%v", client.captures, client.trailerChecks)
		}
		present, err := client.TrailerPresent(result.Tip, "fake-confirmed")
		if err != nil || !present || client.accepted != result.Tip {
			t.Fatalf("trailer or accepted ref: present=%v accepted=%s err=%v", present, client.accepted, err)
		}
		entry, err := ReadEntry(e.Root, "fake-confirmed")
		if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
			t.Fatalf("journal: %+v %v", entry, err)
		}
	})
	t.Run("stale compare retries", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		req := fakeRootPublishRequest(e, "fake-retry")
		attempts := 0
		req.BeforePush = func(attempt int) error {
			attempts = attempt
			if attempt != 1 {
				return nil
			}
			parent, err := client.Capture("competitor")
			if err != nil {
				return err
			}
			commit, err := client.Build("competitor", parent, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(vRoot())}}, "competitor")
			if err != nil {
				return err
			}
			_, err = client.Publish(parent, commit)
			return err
		}
		result, err := Publish(e, req)
		if err != nil || result.Outcome != OutcomeConfirmed || attempts != 2 {
			t.Fatalf("retry: %+v attempts=%d err=%v", result, attempts, err)
		}
	})
	t.Run("unknown publication stays pushed", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		client.unknownNext = true
		result, err := Publish(e, fakeRootPublishRequest(e, "fake-unknown"))
		entry, readErr := ReadEntry(e.Root, "fake-unknown")
		if err == nil || result.Outcome != "" || readErr != nil || entry.Phase != PhasePushed {
			t.Fatalf("unknown outcome lost: %+v entry=%+v err=%v read=%v", result, entry, err, readErr)
		}
	})
	t.Run("accepted ref never regresses", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		result, err := Publish(e, fakeRootPublishRequest(e, "fake-forward"))
		if err != nil {
			t.Fatal(err)
		}
		descendant, err := client.Build("later", result.Commit, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(vRoot())}}, "later")
		if err != nil {
			t.Fatal(err)
		}
		if err := client.AcceptedCAS(result.Commit, descendant); err != nil {
			t.Fatal(err)
		}
		if err := advanceAcceptedFor(e, result.Commit); err != nil || client.accepted != descendant {
			t.Fatalf("accepted regressed to %s: %v", client.accepted, err)
		}
	})
	t.Run("channel files share the injected snapshot", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		base, err := client.Capture("invalid-channel")
		if err != nil {
			t.Fatal(err)
		}
		commit, err := client.Build("invalid-channel", base, []Change{{Path: ChannelPrefix + "unexpected", Content: []byte("invalid")}}, "invalid channel")
		if err != nil {
			t.Fatal(err)
		}
		if err := validateCommitFor(e, commit); err == nil || !strings.Contains(err.Error(), "channel-unknown-path") {
			t.Fatalf("channel validation ignored injected files: %v", err)
		}
	})
	t.Run("invalid built commit refuses before publish", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		req := fakeRootPublishRequest(e, "fake-invalid-built")
		req.Mutate = func(string) ([]Change, error) {
			return []Change{{Path: livePath("bad"), Content: []byte("not a goal")}}, nil
		}
		result, err := Publish(e, req)
		if err != nil || result.Outcome != OutcomeRejected || client.store.canonical == result.Commit {
			t.Fatalf("invalid built tree published: %+v %v", result, err)
		}
	})
	for _, state := range []string{"absent", "valid", "broken"} {
		t.Run("accepted gate "+state, func(t *testing.T) {
			e, client := fakeGoalEndpoint(t)
			switch state {
			case "absent":
				client.accepted = ""
			case "broken":
				client.brokenAccepted = errors.New("accepted ref unreadable")
			}
			called := false
			req := fakeRootPublishRequest(e, "fake-state-"+state)
			req.Mutate = func(string) ([]Change, error) {
				called = true
				return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(vRoot())}}, nil
			}
			result, err := Publish(e, req)
			if state == "broken" {
				if err == nil || called || !strings.Contains(err.Error(), "accepted ref unreadable") {
					t.Fatalf("broken ref passed: %+v %v called=%v", result, err, called)
				}
				return
			}
			if err != nil || !called || result.Outcome != OutcomeConfirmed {
				t.Fatalf("%s ref failed: %+v %v called=%v", state, result, err, called)
			}
		})
	}
	t.Run("valid accepted gate refuses a foreign identity", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		base := client.accepted
		foreign := vRoot()
		foreign.Identity = "01J5X000000000000000000001"
		commit, err := client.Build("foreign", base, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(foreign)}}, "foreign")
		if err != nil {
			t.Fatal(err)
		}
		if outcome, err := client.Publish(base, commit); err != nil || outcome != CASLanded {
			t.Fatalf("foreign setup: %s %v", outcome, err)
		}
		called := false
		req := fakeRootPublishRequest(e, "fake-foreign-gate")
		req.Mutate = func(string) ([]Change, error) { called = true; return nil, nil }
		_, err = Publish(e, req)
		if err == nil || !strings.Contains(err.Error(), "foreign ledger refused") || called {
			t.Fatalf("foreign gate skipped: %v called=%v", err, called)
		}
	})
	t.Run("valid accepted gate refuses a rewind", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		base := client.accepted
		result, err := Publish(e, fakeRootPublishRequest(e, "fake-before-rewind"))
		if err != nil {
			t.Fatal(err)
		}
		client.store.mu.Lock()
		client.store.canonical = base
		client.store.mu.Unlock()
		called := false
		req := fakeRootPublishRequest(e, "fake-rewind-gate")
		req.Mutate = func(string) ([]Change, error) { called = true; return nil, nil }
		_, err = Publish(e, req)
		if err == nil || !strings.Contains(err.Error(), "rewound canonical branch refused") || called || client.accepted != result.Tip {
			t.Fatalf("rewind gate skipped: %v called=%v accepted=%s", err, called, client.accepted)
		}
	})
}

func TestGoalRepositoryFakeProjectOfflineFresh(t *testing.T) {
	t.Parallel()
	e, client := fakeGoalEndpoint(t, vGoal("seen", StateQueued))
	other := client.store.client()
	other.accepted = client.accepted
	otherEndpoint := e
	otherEndpoint.Root = t.TempDir()
	otherEndpoint.Repository = other
	result, err := Publish(otherEndpoint, PublishRequest{
		Opid: "fake-unseen", Machine: "mac-b", Lineage: "lin-1", Intent: testIntentFor("open"), Message: "open unseen",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: livePath("unseen"), Content: RenderFile(vGoal("unseen", StateQueued))}}, nil
		},
		Validate: func(commit string) error { return validateCommitFor(otherEndpoint, commit) },
	})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("other publisher: %+v %v", result, err)
	}
	now := time.Date(2026, 8, 20, 21, 0, 0, 0, time.UTC)
	offline, err := Project(e, false, now)
	if err != nil || offline.Tree.Live["seen"] == nil || offline.Tree.Live["unseen"] != nil {
		t.Fatalf("offline accepted pin: %+v %v", offline.Tree, err)
	}
	fresh, err := Project(e, true, now)
	if err != nil || fresh.Tree.Live["seen"] == nil || fresh.Tree.Live["unseen"] == nil || client.accepted != result.Tip {
		t.Fatalf("fresh advancement: %+v %v", fresh.Tree, err)
	}
}

func verbReqFor(endpoint Endpoint, ulid, machine string) VerbRequest {
	request := verbReq(endpoint.Root, ulid, machine)
	request.Endpoint = endpoint
	return request
}
