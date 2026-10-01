package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// work land follows "Messages a Person Reads": a landing that went well is
// one line, its steps only with --verbose; a landing that stopped is its
// reason and the one command, the step log and codes only with --verbose.
// The triggers were the 2026-09-30 switch-on trial's "== STEP ... -- ok"
// lines and its "would-refuse code=goal-item-not-held ... lawful
// classification exits" refusal.

// seatLandBed is a delivery bed whose seat is a real repository with one
// base commit and no landing lane: the landing path's seat steps commit the
// named paths.
type seatLandBed struct {
	*deliveryBed
	seatGit func(args ...string) string
}

func newSeatLandBed(t *testing.T) *seatLandBed {
	t.Helper()
	b := &seatLandBed{deliveryBed: newDeliveryBed(t)}
	root := b.install
	b.seatGit = func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Wido", "-c", "user.email=wido@example.com"}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.seatGit("checkout", "-q", "-B", "main")
	b.seatGit("add", "notes.md")
	b.seatGit("commit", "-qm", "base")
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
	b.owners.landPath = func(_ landpath.Owners, request landpath.LandRequest, _, _ io.Writer) int {
		b.seatGit("add", "--", "notes.md")
		b.seatGit("commit", "-qm", "record: notes")
		return 0
	}
	return b
}

func (b *seatLandBed) edit(content string) {
	b.t.Helper()
	if err := os.WriteFile(filepath.Join(b.install, "notes.md"), []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// landText runs work land --message on the bed's seat and returns what the
// person reads.
func (b *seatLandBed) landText(extra ...string) (int, string, string) {
	b.t.Helper()
	message := filepath.Join(b.t.TempDir(), "message.txt")
	if err := os.WriteFile(message, []byte("record: notes\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.connection = b.connection
	return b.run(owners, append([]string{"work", "land", "--message", message, "--path", "notes.md"}, extra...)...)
}

func TestMessageWorkLandIsOneLineAndStepsAreDetails(t *testing.T) {
	t.Parallel()
	b := newSeatLandBed(t)
	stub := b.owners.landPath
	b.owners.landPath = func(path landpath.Owners, request landpath.LandRequest, details, stderr io.Writer) int {
		fmt.Fprintln(details, "step: verify checks")
		fmt.Fprintln(details, "  ok")
		fmt.Fprintln(details, "step: commit")
		fmt.Fprintln(details, "  ok")
		return stub(path, request, details, stderr)
	}
	b.edit("one\ntwo\n")
	code, stdout, stderr := b.landText()
	head := b.seatGit("rev-parse", "HEAD")
	if code != 0 || stdout != "landed "+head+"\n" || stderr != "" {
		t.Fatalf("work land = %d\nstdout %q\nstderr %q", code, stdout, stderr)
	}
	b.edit("one\ntwo\nthree\n")
	code, stdout, _ = b.landText("--verbose")
	if code != 0 || !strings.Contains(stdout, "  step: verify checks\n") || !strings.Contains(stdout, "  step: commit\n") || strings.Contains(stdout, "== STEP") {
		t.Fatalf("work land --verbose = %d\n%s", code, stdout)
	}
}

func TestMessageWorkLandRefusalIsTwoLines(t *testing.T) {
	t.Parallel()
	b := newSeatLandBed(t)
	b.owners.landPath = func(_ landpath.Owners, request landpath.LandRequest, details, stderr io.Writer) int {
		fmt.Fprintln(details, "step: commit")
		fmt.Fprintln(details, "verdict: would-refuse code=goal-item-not-held")
		*request.Stop = landpath.Stop{Reason: "goal g1 is not claimed by this session, so nothing was committed",
			Run: []string{"metasystem", "goal", "claim", "g1", "--take-over", "--reason", "TEXT"}, Then: "a person takes it over; then repeat this command"}
		fmt.Fprintln(stderr, request.Stop.Reason)
		fmt.Fprintln(stderr, "run: metasystem goal claim g1 --take-over --reason TEXT  (a person takes it over; then repeat this command)")
		return 1
	}
	b.edit("one\ntwo\n")
	code, stdout, stderr := b.landText()
	want := "✗ goal g1 is not claimed by this session, so nothing was committed\n" +
		"  → metasystem goal claim g1 --take-over --reason TEXT\n" +
		"    a person takes it over; then repeat this command\n"
	if code == 0 || stdout != "" || stderr != want {
		t.Fatalf("work land = %d\nstdout %q\nstderr %q\nwant   %q", code, stdout, stderr, want)
	}
	_, _, verbose := b.landText("--verbose")
	if !strings.HasPrefix(verbose, strings.Replace(want, "✗ ", "✗ metasystem work land: ", 1)) || !strings.Contains(verbose, "  verdict: would-refuse code=goal-item-not-held\n") {
		t.Fatalf("work land --verbose:\n%s", verbose)
	}
}

// withDetails is a result's line 1 and its details, for a test that looks
// for a fact wherever the result keeps it.
func withDetails(result intentResult) string {
	return result.Summary + "\n" + strings.Join(result.Details, "\n")
}
