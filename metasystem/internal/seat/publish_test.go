package seat

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// fakeRemote is a remote that answers by ref: a named refusal for the refs it
// refuses, a transport failure for the fault it is told to have, and an
// accepted push otherwise. It records every ref it was offered, in order.
type fakeRemote struct {
	refuse    map[string]string
	transport error
	offered   []string
	parents   []string
}

func (f *fakeRemote) Publish(ref, message string, file []byte, parent string) (string, error) {
	f.offered = append(f.offered, ref)
	f.parents = append(f.parents, parent)
	if f.transport != nil {
		return "", f.transport
	}
	if detail, refused := f.refuse[ref]; refused {
		return "", &RefRefused{Ref: ref, Detail: detail}
	}
	return fmt.Sprintf("commit-%d", len(f.offered)), nil
}

func fixtureRecord(t *testing.T) Record {
	t.Helper()
	record, _, err := Compose("m1e", fixtureRunner(), JobSet{}, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestTheLadderStartsOnTheMetasystemRef(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{}
	result, err := Publish(remote, PublishRequest{Record: fixtureRecord(t)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungMetasystemRef || result.Fell {
		t.Fatalf("result = %+v; want rung 1 with no fall", result)
	}
	if len(remote.offered) != 1 || remote.offered[0] != "refs/metasystem/presence/m1e" {
		t.Fatalf("offered = %v", remote.offered)
	}
	if remote.parents[0] != "" {
		t.Fatalf("rung 1 pushed a child of %q; it is parentless", remote.parents[0])
	}
}

func TestARefusalThatNamesTheRefMovesOneRungDown(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{refuse: map[string]string{
		"refs/metasystem/presence/m1e": "funny refname",
	}}
	result, err := Publish(remote, PublishRequest{Record: fixtureRecord(t)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungBranchForce || !result.Fell {
		t.Fatalf("result = %+v; want a fall to rung 2", result)
	}
	if len(remote.offered) != 2 || remote.offered[1] != "refs/heads/presence/m1e" {
		t.Fatalf("offered = %v", remote.offered)
	}
	if len(result.Refusals) != 1 || !strings.Contains(result.Refusals[0], "funny refname") {
		t.Fatalf("refusals = %v", result.Refusals)
	}
}

func TestRefusingBothForcedRungsLandsOnTheFastForwardBranch(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{refuse: map[string]string{
		"refs/metasystem/presence/m1e": "funny refname",
	}}
	// Rung 2 is the same ref as rung 3; the force is what a ruleset refuses,
	// so the fake refuses the forced offer and accepts the fast-forward one.
	forceRefused := &refusingForce{inner: remote}
	result, err := Publish(forceRefused, PublishRequest{Record: fixtureRecord(t), BranchTip: "tip-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungBranchFastForward || !result.Fell {
		t.Fatalf("result = %+v; want a fall to rung 3", result)
	}
	if got := remote.parents[len(remote.parents)-1]; got != "tip-1" {
		t.Fatalf("the fast-forward push had parent %q; want the known branch tip", got)
	}
	if len(result.Refusals) != 2 {
		t.Fatalf("refusals = %v; want one per refused rung", result.Refusals)
	}
}

// refusingForce refuses every forced push by name, which is what a branch
// ruleset that forbids force pushes does.
type refusingForce struct{ inner *fakeRemote }

func (r *refusingForce) Publish(ref, message string, file []byte, parent string) (string, error) {
	if parent == "" && strings.HasPrefix(ref, BranchNamespace) {
		r.inner.offered = append(r.inner.offered, ref)
		r.inner.parents = append(r.inner.parents, parent)
		return "", &RefRefused{Ref: ref, Detail: "protected branch hook declined"}
	}
	return r.inner.Publish(ref, message, file, parent)
}

func TestATimeoutMovesNoRung(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{transport: fmt.Errorf("presence push: %w after 1m0s", boundedexec.ErrTimedOut)}
	result, err := Publish(remote, PublishRequest{Record: fixtureRecord(t)})
	if err == nil {
		t.Fatal("a timed-out push reported success")
	}
	if !errors.Is(err, boundedexec.ErrTimedOut) {
		t.Fatalf("err = %v; want the timeout to survive", err)
	}
	if len(remote.offered) != 1 {
		t.Fatalf("offered = %v; a transport failure must not climb down", remote.offered)
	}
	if result.Rung != RungMetasystemRef {
		t.Fatalf("result = %+v; the rung must not move", result)
	}
}

func TestTheRememberedRungIsWhereTheNextPublishStarts(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{}
	result, err := Publish(remote, PublishRequest{Record: fixtureRecord(t), Start: RungBranchForce})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungBranchForce || result.Fell {
		t.Fatalf("result = %+v", result)
	}
	if len(remote.offered) != 1 || remote.offered[0] != "refs/heads/presence/m1e" {
		t.Fatalf("offered = %v; the remembered rung is where a publish starts", remote.offered)
	}
}

func TestThePinnedNamespaceHoldsTheLadder(t *testing.T) {
	t.Parallel()
	remote := &fakeRemote{refuse: map[string]string{"refs/metasystem/presence/m1e": "funny refname"}}
	_, err := Publish(remote, PublishRequest{Record: fixtureRecord(t), Pinned: MetasystemNamespace})
	if err == nil {
		t.Fatal("a pinned namespace fell to a branch anyway")
	}
	for _, ref := range remote.offered {
		if strings.HasPrefix(ref, BranchNamespace) {
			t.Fatalf("offered = %v; the pin must hold", remote.offered)
		}
	}
	branch := &fakeRemote{}
	result, err := Publish(branch, PublishRequest{Record: fixtureRecord(t), Pinned: BranchNamespace})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungBranchForce || branch.offered[0] != "refs/heads/presence/m1e" {
		t.Fatalf("a branch pin started at %+v (%v)", result, branch.offered)
	}
}

func TestTheConflictRefusal(t *testing.T) {
	t.Parallel()
	mine := fixtureRecord(t)
	newer := presence("m1e", fixtureClock.Add(time.Minute), 600)
	newer.RepoIdentity = "another-checkout"
	if err := Conflict(&newer, mine); err == nil || !strings.Contains(err.Error(), "SEAT_PRESENCE_CONFLICT") {
		t.Fatalf("a newer record from another checkout = %v; want the conflict refusal", err)
	}
	older := presence("m1e", fixtureClock.Add(-time.Minute), 600)
	older.RepoIdentity = "another-checkout"
	if err := Conflict(&older, mine); err != nil {
		t.Fatalf("an older record from another checkout = %v; want no refusal", err)
	}
	same := presence("m1e", fixtureClock.Add(time.Minute), 600)
	same.RepoIdentity = mine.RepoIdentity
	if err := Conflict(&same, mine); err != nil {
		t.Fatalf("this checkout's own newer record = %v; want no refusal", err)
	}
	if err := Conflict(nil, mine); err != nil {
		t.Fatalf("a first publish = %v; want no refusal", err)
	}
}

func TestPublishRefusesAnUnpublishableNickname(t *testing.T) {
	t.Parallel()
	broken := fixtureRecord(t)
	broken.Machine = "m1/e"
	remote := &fakeRemote{}
	if _, err := Publish(remote, PublishRequest{Record: broken}); err == nil ||
		!strings.Contains(err.Error(), "SEAT_MACHINE_NICKNAME_INVALID") {
		t.Fatalf("publish = %v; want the nickname refusal", err)
	}
	if len(remote.offered) != 0 {
		t.Fatalf("an unpublishable nickname reached the remote: %v", remote.offered)
	}
}

func TestGitClassifiesARefusalThatNamesTheRefAgainstOneThatDoesNot(t *testing.T) {
	t.Parallel()
	ref := "refs/metasystem/presence/m1e"
	named := []string{
		"remote: error: refusing to create funny refname 'refs/metasystem/presence/m1e'",
		"! [remote rejected] m1e -> refs/metasystem/presence/m1e (pre-receive hook declined)",
		"remote: error: GH013: Repository rule violations found; ruleset says no",
		"error: failed to push some refs; refs/metasystem/presence/m1e was rejected",
	}
	for _, output := range named {
		if !refusalNamesRef(ref, output) {
			t.Errorf("a refusal naming the ref read as transport: %q", output)
		}
	}
	transport := []string{
		"fatal: could not read Username for 'https://example.invalid': terminal prompts disabled",
		"ssh: connect to host example.invalid port 22: Operation timed out",
		"fatal: Authentication failed for 'https://example.invalid/'",
		"ssh: Could not resolve hostname example.invalid",
	}
	for _, output := range transport {
		if refusalNamesRef(ref, output) {
			t.Errorf("a transport failure read as a ref refusal: %q", output)
		}
	}
}
