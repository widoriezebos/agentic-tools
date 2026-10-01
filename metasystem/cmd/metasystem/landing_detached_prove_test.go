package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// landing prove starts the proof detached and returns at once, in two
// lines: what it proves as which attempt, and the command that shows when
// it ends. A repeat while that tree's proof runs is success and starts
// nothing; --wait keeps the blocking prove.
func TestLandingProveStartsTheProofAndReturns(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	owners := bed.owners()
	owners.landing.prove = func(kernel.ProveRequest) (kernel.TreeProof, error) {
		t.Fatalf("landing prove without --wait ran the proof in the caller")
		return kernel.TreeProof{}, nil
	}
	var asked kernel.ProveRequest
	running := kernel.TreeProof{Tree: strings.Repeat("a", 40), Commit: strings.Repeat("b", 40), Attempt: "a1", Status: batch.AttemptRunning}
	already := false
	owners.landing.startProof = func(request kernel.ProveRequest) (kernel.Started, error) {
		asked = request
		return kernel.Started{Proof: running, Already: already}, nil
	}
	code, text := bed.runWith(t, owners, "landing", "prove")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if code != 0 || len(lines) != 2 || !strings.Contains(lines[0], "proving bbbbbbbbbbbb (tree aaaaaaaaaaaa) as attempt a1") ||
		!strings.Contains(lines[1], "metasystem landing status") || !strings.Contains(lines[1], "woken") ||
		asked.Home != bed.home || string(asked.Layout.Checkout) != bed.checkout || !strings.HasSuffix(asked.Actor, "+"+lane.ClaimLineage) {
		t.Fatalf("landing prove = %d (asked %+v)\n%s", code, asked, text)
	}
	already = true
	code, text = bed.runWith(t, owners, "landing", "prove", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already") {
		t.Fatalf("a repeat while proving = %d %+v %v\n%s", code, result, err, text)
	}

	var waited kernel.ProveRequest
	owners.landing.prove = func(request kernel.ProveRequest) (kernel.TreeProof, error) {
		waited = request
		return kernel.TreeProof{Tree: strings.Repeat("a", 40), Attempt: "a7", Status: batch.AttemptGreen}, nil
	}
	if code, text := bed.runWith(t, owners, "landing", "prove", "--wait", "--tree", "HEAD", "--attempt", "a7"); code != 0 ||
		!strings.Contains(text, "landing push may put it on main") || waited.Tree != "HEAD" || waited.Attempt != "a7" {
		t.Fatalf("landing prove --wait = %d (asked %+v)\n%s", code, waited, text)
	}
}

// landing status shows the lane's running proof (attempt, tree, since),
// and a proof whose process died without a result as died; the keeper of
// the lane reads the same proof.
func TestLandingStatusShowsTheRunningProof(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.landMain(t)
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	exact, live, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || live != identity.Alive {
		t.Fatalf("probe self: %v %v", live, err)
	}
	self, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	keep := func(process string) {
		data, err := json.Marshal(kernel.TreeProof{Tree: tree, Attempt: "a9", Status: batch.AttemptRunning, Process: process, StartedAt: "2026-10-01T09:00:00Z"})
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(bed.checkout, "artifacts", "agents", "landing-proofs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, tree+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	status := func() (landingRunningProof, *lane.Wake, string) {
		code, text := bed.runWith(t, bed.owners(), "landing", "status", "--json")
		var result struct {
			Data struct {
				Wake         *lane.Wake           `json:"wake"`
				RunningProof *landingRunningProof `json:"running_proof"`
			}
		}
		if err := json.Unmarshal([]byte(text), &result); err != nil || code != 0 || result.Data.RunningProof == nil {
			t.Fatalf("landing status --json = %d %v\n%s", code, err, text)
		}
		_, page := bed.runWith(t, bed.owners(), "landing", "status")
		return *result.Data.RunningProof, result.Data.Wake, page
	}
	keep(self)
	running, wake, page := status()
	if running.State != "running" || running.Attempt != "a9" || running.Tree != tree || running.Since != "2026-10-01T09:00:00Z" ||
		wake == nil || wake.Proving == nil || !strings.Contains(page, "proving tree "+tree[:12]) || !strings.Contains(page, "a9") {
		t.Fatalf("a running proof reads %+v, wake %+v\n%s", running, wake, page)
	}
	fact, err := newLandingAgentKeeper(bed.installation, bed.home, newLandingAgent()).Sources.Proof(bed.checkout)
	if err != nil || !fact.Running || fact.Attempt != "a9" {
		t.Fatalf("the keeper reads the proof %+v %v; want a9 running", fact, err)
	}

	gone := exact.Ref()
	if gone.StartTicks != 0 {
		gone.StartTicks = 1
	} else {
		gone.StartedAtUnixMicro, gone.StartedAtSec = 1_000_000, 1
	}
	died, err := identity.EncodeRef(gone)
	if err != nil {
		t.Fatal(err)
	}
	keep(died)
	running, wake, page = status()
	if running.State != "died" || wake == nil || wake.Proving != nil || !strings.Contains(page, "died") {
		t.Fatalf("a died proof reads %+v, wake %+v\n%s", running, wake, page)
	}
}

// witnessLandingProveRepeat: the first landing prove starts the tree's
// proof in the background; the repeat while it runs is success, starts no
// second proof and leaves the host home as it was. The stand-in proof is a
// process this witness starts and stops by its own pid.
func witnessLandingProveRepeat(t *testing.T) {
	bed := newKernelBed(t)
	bed.landMain(t)
	launches := 0
	seams := kernel.ProductionStartSeams()
	seams.Executable = func() (string, error) { return "/fixture/metasystem", nil }
	seams.Launch = func([]string, string, string) (int64, error) {
		launches++
		proof := exec.Command("/bin/sleep", "600")
		if err := proof.Start(); err != nil {
			return 0, err
		}
		t.Cleanup(func() { _ = proof.Process.Kill(); _ = proof.Wait() })
		return int64(proof.Process.Pid), nil
	}
	owners := bed.owners()
	owners.landing.startProof = func(request kernel.ProveRequest) (kernel.Started, error) { return kernel.StartProof(request, seams) }
	if code, text := bed.runWith(t, owners, "landing", "prove"); code != 0 || !strings.Contains(text, "proving ") {
		t.Fatalf("first landing prove = %d\n%s", code, text)
	}
	home := idemTreeDigest(t, bed.home)
	if code, text := bed.runWith(t, owners, "landing", "prove"); code != 0 || !strings.Contains(text, "already proving") {
		t.Fatalf("repeated landing prove = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated landing prove (home)", home, idemTreeDigest(t, bed.home))
	if launches != 1 {
		t.Fatalf("a repeated landing prove started %d proofs; want 1", launches)
	}
}
