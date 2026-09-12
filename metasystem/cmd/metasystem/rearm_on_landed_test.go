package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestLandedRearmDecidesFromTheThreeFacts(t *testing.T) {
	base := landedRearmFacts{LandingRef: "refs/remotes/origin/main", Tip: strings.Repeat("b", 40), Head: strings.Repeat("a", 40), Source: strings.Repeat("a", 40)}
	owns := base
	owns.SourceOwnsTip = true
	if d := decideLandedRearm(owns, "/c"); d.Rearm || d.Refusal != "" {
		t.Fatalf("an engine that owns the tip was not left alone: %+v", d)
	}
	landed := base
	landed.HeadIsAncestor = true
	if d := decideLandedRearm(landed, "/c"); !d.Rearm || d.Refusal != "" {
		t.Fatalf("a landed-only skew was not re-armed: %+v", d)
	}
	diverged := base
	diverged.HeadIsAncestor = false
	d := decideLandedRearm(diverged, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "TEST_POLICY_ENGINE_REQUIRED") || !strings.Contains(d.Refusal, base.Source) ||
		!strings.Contains(d.Refusal, base.Tip) || !strings.Contains(d.Refusal, "not an ancestor") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a diverged checkout was not refused with the two commits and the command: %+v", d)
	}
	dirty := landed
	dirty.DirtyEnginePaths = []string{"metasystem/cmd/metasystem/main.go"}
	d = decideLandedRearm(dirty, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "dirty in engine inputs (metasystem/cmd/metasystem/main.go)") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a checkout dirty in an engine input was not refused naming the path and the command: %+v", d)
	}
	// A delivery run that names the exact index it proves keeps the manual
	// path: a fast-forward would move that index under its receipt.
	named := landed
	named.NamedDeliveryTree = true
	d = decideLandedRearm(named, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "names the exact index it proves") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a delivery run naming its tree was not kept on the manual path: %+v", d)
	}
	// The engine is never rebuilt under a live attempt of this installation.
	busy := landed
	busy.LiveAttempts = []string{"proof-live-1"}
	d = decideLandedRearm(busy, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "live (proof-live-1)") {
		t.Fatalf("a rebuild under a live attempt was not refused: %+v", d)
	}
}

type landedRearmFixture struct {
	remote, projectRoot, installation string
}

func landedGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
		"-c", "clone.defaultRemoteName=origin", "-c", "init.defaultBranch=main", "-c", "diff.relative=false"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newLandedRearmFixture is a checkout with its installation under
// metasystem/ and a local remote whose main is the landing ref; the remote
// then lands a commit that changes an engine input (a fake landing).
func newLandedRearmFixture(t *testing.T) landedRearmFixture {
	t.Helper()
	root, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(root, "remote.git")
	landedGit(t, root, "init", "-q", "--bare", "-b", "main", remote)
	seed := filepath.Join(root, "seed")
	landedGit(t, root, "init", "-q", "-b", "main", seed)
	for _, item := range []struct{ path, content string }{
		{"metasystem/metasystem.conf", "testing.contract=testing.json\n"},
		{"metasystem/cmd/metasystem/main.go", "package main\n"},
		{"metasystem/docs/notes.md", "notes\n"},
	} {
		path := filepath.Join(seed, item.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(item.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	landedGit(t, seed, "add", ".")
	landedGit(t, seed, "commit", "-qm", "seed")
	landedGit(t, seed, "remote", "add", "origin", remote)
	landedGit(t, seed, "push", "-q", "origin", "main")
	projectRoot := filepath.Join(root, "checkout")
	landedGit(t, root, "clone", "-q", remote, projectRoot)
	landedGit(t, projectRoot, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	// The fake landing: the remote's main gains a commit that changes an
	// engine input; the checkout has not fetched it.
	if err := os.WriteFile(filepath.Join(seed, "metasystem", "cmd", "metasystem", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	landedGit(t, seed, "add", ".")
	landedGit(t, seed, "commit", "-qm", "landed engine change")
	landedGit(t, seed, "push", "-q", "origin", "main")
	return landedRearmFixture{remote: remote, projectRoot: projectRoot, installation: filepath.Join(projectRoot, "metasystem")}
}

func TestLandedRearmReadsTheCheckoutAgainstItsRemote(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	notOwned := func(string) bool { return false }
	facts, err := readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil {
		t.Fatal(err)
	}
	remoteTip := landedGit(t, fixture.remote, "rev-parse", "main")
	if facts.Remote != "origin" || facts.Branch != "main" || facts.Tip != remoteTip || facts.Head != head || facts.Tip == head {
		t.Fatalf("the fetch did not bring the landing ref to the remote's tip: %+v (remote main %s)", facts, remoteTip)
	}
	if !facts.HeadIsAncestor || len(facts.DirtyEnginePaths) != 0 || facts.SourceOwnsTip {
		t.Fatalf("a clean checkout behind the tip was not read as landed-only: %+v", facts)
	}
	// An engine that owns the tip needs no other fact.
	owned, err := readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return true })
	if err != nil || !owned.SourceOwnsTip {
		t.Fatalf("an engine owning the tip was not reported so: %+v %v", owned, err)
	}
	// A change outside the ENGINE projection is not dirt; one inside is.
	if err := os.WriteFile(filepath.Join(fixture.installation, "docs", "notes.md"), []byte("notes, edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts, err = readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || len(facts.DirtyEnginePaths) != 0 {
		t.Fatalf("a docs edit counted as engine dirt: %+v %v", facts.DirtyEnginePaths, err)
	}
	if err := os.WriteFile(filepath.Join(fixture.installation, "cmd", "metasystem", "extra.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts, err = readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || strings.Join(facts.DirtyEnginePaths, ",") != "metasystem/cmd/metasystem/extra.go" {
		t.Fatalf("an untracked engine file was not listed as dirt: %+v %v", facts.DirtyEnginePaths, err)
	}
	if err := os.Remove(filepath.Join(fixture.installation, "cmd", "metasystem", "extra.go")); err != nil {
		t.Fatal(err)
	}
	// A local commit the remote lacks: the checkout is not an ancestor.
	landedGit(t, fixture.projectRoot, "commit", "-qam", "local only")
	facts, err = readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || facts.HeadIsAncestor {
		t.Fatalf("a checkout with a local commit was read as an ancestor of the tip: %+v %v", facts, err)
	}
	// A landing ref that cannot be fetched is judged as last fetched: an
	// engine that owns it runs, a re-arm from it is refused.
	landedGit(t, fixture.projectRoot, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "absent.git"))
	facts, err = readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || facts.FetchErr == nil || facts.Tip != remoteTip {
		t.Fatalf("a failed fetch was not recorded against the last fetched tip: %+v %v", facts, err)
	}
	if d := decideLandedRearm(facts, fixture.projectRoot); d.Rearm || !strings.Contains(d.Refusal, "could not be fetched") {
		t.Fatalf("a re-arm from an unfetchable tip was not refused: %+v", d)
	}
	if owned, err := readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return true }); err != nil || !owned.SourceOwnsTip {
		t.Fatalf("an engine owning the last fetched tip was refused for the fetch: %+v %v", owned, err)
	}
}

func TestLandedRearmFastForwardsRebuildsAndReArms(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	facts, err := readLandedRearmFacts(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	var rebuiltIn, upInstallation, upScope string
	locked := 0
	previousRebuild, previousUp, previousOpen, previousLock := landedRearmRebuild, landedRearmUp, landedRearmOpenEnrollment, landedRearmMutationLock
	upResult := upOutcome{Line: "up outcome=armed authority=writer re-armed=\"generation=2 previous=1\"", Outcome: "armed"}
	var upErr error
	enrolledGeneration := 2
	landedRearmRebuild = func(_ context.Context, installation string) error { rebuiltIn = installation; return nil }
	landedRearmUp = func(_ context.Context, installation, projectRoot string) (upOutcome, error) {
		upInstallation, upScope = installation, projectRoot
		return upResult, upErr
	}
	landedRearmOpenEnrollment = func(string) (steward.InstallIdentity, error) {
		return steward.InstallIdentity{Generation: enrolledGeneration}, nil
	}
	landedRearmMutationLock = func(string) (func(), error) { locked++; return func() { locked-- }, nil }
	t.Cleanup(func() {
		landedRearmRebuild, landedRearmUp, landedRearmOpenEnrollment, landedRearmMutationLock = previousRebuild, previousUp, previousOpen, previousLock
	})
	record, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 1)
	if err != nil {
		t.Fatal(err)
	}
	if locked != 0 {
		t.Fatalf("the proof mutation lock was not released: %d", locked)
	}
	if got := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD"); got != facts.Tip {
		t.Fatalf("the checkout was not fast-forwarded to the tip: HEAD %s tip %s", got, facts.Tip)
	}
	if rebuiltIn != fixture.installation || upInstallation != fixture.installation || upScope != fixture.projectRoot {
		t.Fatalf("the rebuild or the re-arm ran elsewhere: rebuild=%s up=%s scope=%s", rebuiltIn, upInstallation, upScope)
	}
	if record == nil || record.SourceCommit != head || record.LandedTip != facts.Tip || record.PreviousGeneration != 1 || record.Generation != 2 ||
		record.ReArmed != upResult.Line || record.UpOutcome != "armed" || record.At == "" {
		t.Fatalf("the re-arm record is incomplete: %+v", record)
	}
	// up mints first and proves the session after: a detached run gets a
	// failed up over an enrollment that did advance, and continues on it.
	upResult = upOutcome{Line: "up outcome=failed failed=session-identity remedy=no runtime ancestor", Outcome: "failed", Failed: true}
	upErr = errors.New("bin/metasystem up --repo x: exit status 1: no runtime ancestor")
	enrolledGeneration = 3
	record, err = landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 2)
	if err != nil || record == nil || record.Generation != 3 || record.UpOutcome != "failed" {
		t.Fatalf("a failed up over an advanced enrollment did not continue: %+v %v", record, err)
	}
	// An enrollment that did not advance is a refusal with up's own words.
	enrolledGeneration = 3
	if _, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 3); err == nil || !strings.Contains(err.Error(), "no runtime ancestor") {
		t.Fatalf("an enrollment that did not advance was not refused with up's words: %v", err)
	}
	// A refused decision runs none of the three acts and takes no lock.
	rebuiltIn, upInstallation, locked = "", "", 0
	diverged := facts
	diverged.HeadIsAncestor = false
	if _, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, diverged, 3); err == nil || rebuiltIn != "" || upInstallation != "" || locked != 0 {
		t.Fatalf("a refusal reached the acts: err=%v rebuild=%s up=%s locked=%d", err, rebuiltIn, upInstallation, locked)
	}
	// The real up seam runs the rebuilt binary's own up verb from the
	// installation with the checkout as --repo.
	if !strings.HasSuffix(runtimeUpCommandFor(fixture.installation, fixture.projectRoot), filepath.Join("bin", "metasystem")+" up --repo "+fixture.projectRoot) {
		t.Fatal("the up seam does not name the rebuilt binary's own up")
	}
}
