package seat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// This is the package's one git-driven test: the adapter's contract with a
// real remote, proved against a bare repository in the test's own temporary
// directory. Nothing here reaches a real remote, and the test is parallel, so
// it stays outside the serial count the parallel ratchet limits.

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// presenceBed is a bare remote and two clones of it.
type presenceBed struct {
	remote string
	one    string
	two    string
}

func newPresenceBed(t *testing.T) presenceBed {
	t.Helper()
	root := t.TempDir()
	bed := presenceBed{
		remote: filepath.Join(root, "remote.git"),
		one:    filepath.Join(root, "one"),
		two:    filepath.Join(root, "two"),
	}
	if err := os.MkdirAll(bed.remote, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, bed.remote, "init", "-q", "--bare", "-b", "main")
	for _, clone := range []string{bed.one, bed.two} {
		if err := os.MkdirAll(clone, 0o755); err != nil {
			t.Fatal(err)
		}
		gitIn(t, clone, "init", "-q", "-b", "main")
		gitIn(t, clone, "config", "user.name", "fixture")
		gitIn(t, clone, "config", "user.email", "fixture@example.invalid")
		gitIn(t, clone, "remote", "add", "origin", bed.remote)
	}
	return bed
}

func publishFrom(t *testing.T, root, remote, machine string, at time.Time, start Rung) PublishResult {
	t.Helper()
	transport := Git{Root: root, Remote: remote}
	record, _, err := Compose(machine, fixtureRunner(), JobSet{}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	tips, err := transport.Tips(TickNamespace, BranchNamespace)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Publish(transport, PublishRequest{Record: record, Start: start, BranchTip: tips[machine]})
	if err != nil {
		t.Fatalf("publish %s: %v", machine, err)
	}
	return result
}

func TestPresenceRefRoundTripsThroughABareRemote(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	reader := Git{Root: bed.two, Remote: "origin"}

	first := publishFrom(t, bed.one, "origin", "m1e", fixtureClock, RungMetasystemRef)
	if first.Rung != RungMetasystemRef {
		t.Fatalf("a bare remote refused the metasystem ref: %+v", first)
	}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	record, seen := copied.Records["m1e"]
	if !seen || record.TickAt != FormatTime(fixtureClock) || record.Machine != "m1e" {
		t.Fatalf("the reader read %+v", copied)
	}

	second := publishFrom(t, bed.one, "origin", "m1e", fixtureClock.Add(10*time.Minute), RungMetasystemRef)
	if second.Commit == first.Commit {
		t.Fatal("the second publish reused the first commit")
	}
	parents := gitIn(t, bed.one, "rev-list", "--parents", "-n", "1", second.Commit)
	if strings.Contains(strings.TrimPrefix(parents, second.Commit), " ") {
		t.Fatalf("a presence commit carried history: %q", parents)
	}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err = reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Records["m1e"].TickAt != FormatTime(fixtureClock.Add(10*time.Minute)) {
		t.Fatalf("the ref did not move: %+v", copied.Records["m1e"])
	}
}

func TestTwoReadersFetchIntoTheirOwnNamespacesWithoutBlockingEachOther(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	publishFrom(t, bed.one, "origin", "m1e", fixtureClock, RungMetasystemRef)
	reader := Git{Root: bed.two, Remote: "origin"}

	namespaces := []string{FetchNamespacePrefix + "/reader-one", FetchNamespacePrefix + "/reader-two"}
	failures := make([]error, len(namespaces))
	var waiting sync.WaitGroup
	for index, namespace := range namespaces {
		waiting.Add(1)
		go func(index int, namespace string) {
			defer waiting.Done()
			failures[index] = reader.Fetch(namespace)
		}(index, namespace)
	}
	waiting.Wait()
	for index, err := range failures {
		if err != nil {
			t.Fatalf("reader %d: %v", index, err)
		}
	}
	for _, namespace := range namespaces {
		copied, err := reader.Read(namespace)
		if err != nil {
			t.Fatal(err)
		}
		if _, seen := copied.Records["m1e"]; !seen {
			t.Fatalf("namespace %s read %+v", namespace, copied)
		}
		if err := reader.DeleteNamespace(namespace); err != nil {
			t.Fatal(err)
		}
		if left := gitIn(t, bed.two, "for-each-ref", "--format=%(refname)", namespace); left != "" {
			t.Fatalf("namespace %s survived its cleanup: %q", namespace, left)
		}
	}
}

func TestADeletedRemoteRefIsPrunedFromEveryReadersNamespace(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	publishFrom(t, bed.one, "origin", "m1e", fixtureClock, RungMetasystemRef)
	publishFrom(t, bed.one, "origin", "m1c", fixtureClock, RungMetasystemRef)
	reader := Git{Root: bed.two, Remote: "origin"}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied.Records) != 2 {
		t.Fatalf("the reader read %+v", copied)
	}
	// The operator's one git act for a machine that is gone for good.
	gitIn(t, bed.one, "push", "origin", ":refs/metasystem/presence/m1c")
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err = reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if _, survived := copied.Records["m1c"]; survived {
		t.Fatalf("a deleted ref survived the prune: %+v", copied)
	}
	if _, kept := copied.Records["m1e"]; !kept {
		t.Fatalf("the prune took a live ref too: %+v", copied)
	}
}

func TestAFailedFetchLeavesTheNamespaceAsItWas(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	publishFrom(t, bed.one, "origin", "m1e", fixtureClock, RungMetasystemRef)
	reader := Git{Root: bed.two, Remote: "origin"}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	before, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}

	broken := Git{Root: bed.two, Remote: filepath.Join(t.TempDir(), "no-such-remote.git")}
	if err := broken.Fetch(TickNamespace); err == nil {
		t.Fatal("a fetch from a remote that does not exist reported success")
	}
	after, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if after.Records["m1e"].TickAt != before.Records["m1e"].TickAt {
		t.Fatalf("a failed fetch changed the namespace: %+v then %+v", before, after)
	}
}

func TestLocalModeUpdatesTheLocalRefAndPushesNothing(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	local := Git{Root: bed.one, Remote: "local", Local: true}
	record, _, err := Compose("m1e", fixtureRunner(), JobSet{}, nil, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Publish(local, PublishRequest{Record: record})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rung != RungMetasystemRef {
		t.Fatalf("local mode used rung %d", result.Rung)
	}
	if got := gitIn(t, bed.one, "rev-parse", "refs/metasystem/presence/m1e"); got != result.Commit {
		t.Fatalf("local ref = %s, want %s", got, result.Commit)
	}
	if atRemote := gitIn(t, bed.remote, "for-each-ref", "--format=%(refname)", "refs/metasystem/presence"); atRemote != "" {
		t.Fatalf("local mode pushed to the remote: %q", atRemote)
	}
	// A local seat still sees itself: the reader reads the publishing refs
	// because nothing was fetched.
	if err := local.Fetch(TickNamespace); err != nil {
		t.Fatalf("local mode fetched: %v", err)
	}
	copied, err := local.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Records["m1e"].TickAt != FormatTime(fixtureClock) {
		t.Fatalf("a local seat did not see itself: %+v", copied)
	}
}

func TestABranchRungRoundTripsAndKeepsOneWriterPerMachine(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	first := publishFrom(t, bed.one, "origin", "m1e", fixtureClock, RungBranchForce)
	if first.Rung != RungBranchForce {
		t.Fatalf("result = %+v", first)
	}
	reader := Git{Root: bed.two, Remote: "origin"}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Records["m1e"].TickAt != FormatTime(fixtureClock) {
		t.Fatalf("the branch rung did not reach the reader: %+v", copied)
	}
	// Rung 3 is the same branch with each record a child of the last.
	publisher := Git{Root: bed.one, Remote: "origin"}
	if err := publisher.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	second := publishFrom(t, bed.one, "origin", "m1e", fixtureClock.Add(10*time.Minute), RungBranchFastForward)
	if second.Rung != RungBranchFastForward {
		t.Fatalf("result = %+v", second)
	}
	parents := gitIn(t, bed.one, "rev-list", "--parents", "-n", "1", second.Commit)
	if !strings.Contains(parents, first.Commit) {
		t.Fatalf("the fast-forward rung dropped its parent: %q", parents)
	}
}

func TestAMalformedRecordAtTheRemoteReadsAsUnknownAndNeverAsReachable(t *testing.T) {
	t.Parallel()
	bed := newPresenceBed(t)
	// Write a record the reader refuses, through git's own plumbing.
	torn := filepath.Join(t.TempDir(), "presence.json")
	if err := os.WriteFile(torn, []byte(`{"presenceSchema":1,"machine":"m1c"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	object := gitIn(t, bed.one, "hash-object", "-w", torn)
	tree := writeTree(t, bed.one, object)
	commit := gitIn(t, bed.one, "commit-tree", tree, "-m", "seat presence m1c torn")
	gitIn(t, bed.one, "push", "origin", "+"+commit+":refs/metasystem/presence/m1c")

	reader := Git{Root: bed.two, Remote: "origin"}
	if err := reader.Fetch(TickNamespace); err != nil {
		t.Fatal(err)
	}
	copied, err := reader.Read(TickNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if _, reachable := copied.Records["m1c"]; reachable {
		t.Fatalf("a torn record read as a record: %+v", copied)
	}
	if !strings.Contains(copied.Malformed["m1c"], "SEAT_PRESENCE_MALFORMED") {
		t.Fatalf("a torn record read as %q", copied.Malformed["m1c"])
	}
	standings := Fleet(FleetInput{Copy: copied, Now: fixtureClock, Window: 30 * time.Minute})
	if len(standings) != 1 || standings[0].Standing != Unknown {
		t.Fatalf("standings = %+v", standings)
	}
}

func writeTree(t *testing.T, root, blob string) string {
	t.Helper()
	command := exec.Command("git", "mktree")
	command.Dir = root
	command.Stdin = strings.NewReader("100644 blob " + blob + "\tpresence.json\n")
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git mktree: %v", err)
	}
	return strings.TrimSpace(string(out))
}
