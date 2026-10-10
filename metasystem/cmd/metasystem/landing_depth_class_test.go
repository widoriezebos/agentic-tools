package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func seedCurrentTrunkClock(t *testing.T, install string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(plain.Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plain.Result{Trunk: true, Scope: "full", Result: plain.Green, At: at.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(plain.Dir(install), "results.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("seed trunk clock: %v %v", err, closeErr)
	}
}

func TestLandingSelectDepthClassStatus(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\nproof.trunk-every=1h\n"), 0600))
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	code, output := b.run(t, b.lane, "landing", "run")
	if code != 0 {
		t.Fatalf("landing run: exit %d, %s", code, output)
	}
	batch := b.batch(t)
	if batch.DepthClass != "cheap" || len(batch.Members) != 1 || batch.Members[0].Goal != "b" {
		t.Fatalf("mixed selection: %+v", batch)
	}
	code, output = b.run(t, b.lane, "landing", "status", "--json")
	if code != 0 {
		t.Fatalf("landing status: exit %d, %s", code, output)
	}
	var status struct {
		Data plain.Status `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatal(err)
	}
	wantReason := "full proof overdue since " + b.now.Local().Format("15:04")
	if !strings.Contains(status.Data.Summary, "batch of 1, depth class cheap") || !strings.Contains(status.Data.Summary, wantReason) {
		t.Fatalf("status lost depth or reason: %s", output)
	}
	t.Logf("observed status: %s", status.Data.Summary)
}

// Real Git proves that the selected tree is the tree published on main.
func TestLandingFullDueBatchAdapterProofPushClock(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	sha := b.seat(t, "low")
	b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return 1, nil }
	b.owners.landing.plainProve.BatchDepth = func(string, string, plain.Running) (string, string) { return "impact", "low tier" }
	b.success(t, "landing", "run")
	batch := b.batch(t)
	if batch.DepthClass != "cheap" {
		t.Fatalf("cheap selection: %+v", batch)
	}
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	want := "full proof overdue since " + b.now.Local().Format("15:04")
	if err != nil || !ok || proof.Scope != "full" || proof.ScopeReason != want || proof.FullTree != proof.Tree || proof.FullAt != proof.At {
		t.Fatalf("overdue proof: %+v %v", proof, err)
	}
	b.success(t, "landing", "push")
	push, ok, err := plain.LastPush(b.installation)
	if err != nil || !ok || push.Tree != proof.Tree || push.BatchID != batch.ID {
		t.Fatalf("push: %+v %v", push, err)
	}
	reasons, err := plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range reasons {
		if reason == plain.WakeFullDue {
			t.Fatalf("pushed full proof did not reset clock: %v", reasons)
		}
	}
	t.Logf("observed depth: %s; reason: %s; full-due cleared after push", proof.Scope, proof.ScopeReason)
}

// Real Git distinguishes a main refresh that changes only the ledger from a code change.
func TestLandingFullDueLedgerRefreshAdapterInheritsAndPaysClock(t *testing.T) {
	t.Parallel()
	for _, wait := range []bool{false, true} {
		name := "settled"
		if wait {
			name = "run"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			b.owners.landing.now = func() time.Time { return b.now }
			sha := b.seat(t, "low")
			b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return 1, nil }
			b.owners.landing.plainProve.BatchDepth = func(string, string, plain.Running) (string, string) { return "impact", "low tier" }
			b.success(t, "landing", "run")
			b.assemble(t, sha)
			b.success(t, "landing", "prove", "--wait")
			full, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || full.Scope != "full" || b.executions(t) != 1 {
				t.Fatalf("initial full proof: %+v %v", full, err)
			}
			b.now = b.now.Add(time.Minute)
			writer := filepath.Join(t.TempDir(), "main-writer")
			b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, writer)
			ledger := filepath.Join(writer, "metasystem", "plans", "goals.md")
			if err := os.MkdirAll(filepath.Dir(ledger), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ledger, []byte("ledger refresh\n"), 0644); err != nil {
				t.Fatal(err)
			}
			b.git(t, writer, "add", "metasystem/plans/goals.md")
			b.git(t, writer, "commit", "--quiet", "-m", "advance goal ledger")
			b.git(t, writer, "push", "--quiet", "origin", "HEAD:main")
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/main")
			words := []string{"landing", "prove"}
			if wait {
				words = append(words, "--wait")
			}
			b.success(t, words...)
			inherited, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || inherited.Result != plain.Green || inherited.Scope != "full" || inherited.FullTree != full.FullTree || inherited.FullAt != full.FullAt || inherited.Tree == full.Tree || !strings.HasPrefix(inherited.Reason, "inherits green from tree ") || b.executions(t) != 1 || b.launches != 0 {
				t.Fatalf("refresh did not inherit full green without a runner: %+v %v; executions=%d launches=%d", inherited, err, b.executions(t), b.launches)
			}
			reasons, err := plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
			if err != nil || !slices.Contains(reasons, plain.WakeFullDue) {
				t.Fatalf("unpublished inherited proof paid clock: %v %v", reasons, err)
			}
			b.success(t, "landing", "push")
			push, ok, err := plain.LastPush(b.installation)
			if err != nil || !ok || push.Tree != inherited.Tree || push.BatchID != full.BatchID {
				t.Fatalf("inherited tree not pushed: %+v %v", push, err)
			}
			reasons, err = plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
			if err != nil || slices.Contains(reasons, plain.WakeFullDue) {
				t.Fatalf("pushed inherited full did not pay clock: %v %v", reasons, err)
			}
			t.Logf("%s: full green inherited without another execution; original full tree and time retained; pushed refresh cleared full-due", name)
		})
	}
}

// Real Git proves the candidate includes a superseded hand-in shared by two waiting goals.
func TestLandingSelectAdapterSupersededAncestryKeepsFullBatch(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	oldA := b.seat(t, "a")
	seatA := filepath.Join(filepath.Dir(b.checkout), "seat-a")
	if err := os.WriteFile(filepath.Join(seatA, "a.txt"), []byte("a fixed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatA, "add", "a.txt")
	b.git(t, seatA, "commit", "--quiet", "-m", "fix a")
	newA := b.git(t, seatA, "rev-parse", "HEAD")
	b.git(t, seatA, "push", "--quiet", "origin", "goal/a")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newA}); err != nil {
		t.Fatal(err)
	}
	seatB := filepath.Join(t.TempDir(), "seat-b")
	b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, seatB)
	b.git(t, seatB, "checkout", "--quiet", "-b", "goal/b", oldA)
	if err := os.WriteFile(filepath.Join(seatB, "b.txt"), []byte("b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatB, "add", "b.txt")
	b.git(t, seatB, "commit", "--quiet", "-m", "b stacked on old a")
	shaB := b.git(t, seatB, "rev-parse", "HEAD")
	b.git(t, seatB, "push", "--quiet", "origin", "goal/b")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "b", SHA: shaB}); err != nil {
		t.Fatal(err)
	}
	entries, err := plain.Entries(b.installation)
	if err != nil || len(entries) != 3 || entries[0].State != plain.StateSuperseded {
		t.Fatalf("superseded fixture: %+v %v", entries, err)
	}
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	b.success(t, "landing", "run")
	batch := b.batch(t)
	if batch.DepthClass != "full" || !slices.Equal(batch.Members, []plain.GoalSHA{{Goal: "a", SHA: newA}, {Goal: "b", SHA: shaB}}) {
		t.Fatalf("shared superseded ancestry split: %+v", batch)
	}
	b.assemble(t, newA, shaB)
	b.success(t, "landing", "prove", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	if err != nil || !ok || proof.Result != plain.Green || proof.Scope != "full" || !slices.Equal(proof.BatchMembers, batch.Members) {
		t.Fatalf("shared ancestry failed proof admission: %+v %v", proof, err)
	}
	t.Log("selected both waiting goals at full depth; candidate containing superseded a passed admission")
}

// Real Git distinguishes shared landed history from an unlanded dependency.
func TestLandingSelectAdapterLandedAncestryKeepsCheapBatch(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	oldA := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, oldA)
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")

	seatA := filepath.Join(filepath.Dir(b.checkout), "seat-a")
	b.git(t, seatA, "fetch", "--quiet", "origin")
	b.git(t, seatA, "merge", "--quiet", "--no-edit", "origin/main")
	if err := os.WriteFile(filepath.Join(seatA, "a.txt"), []byte("a follow-up\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatA, "add", "a.txt")
	b.git(t, seatA, "commit", "--quiet", "-m", "a follow-up on main")
	newA := b.git(t, seatA, "rev-parse", "HEAD")
	b.git(t, seatA, "push", "--quiet", "origin", "goal/a")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newA}); err != nil {
		t.Fatal(err)
	}
	shaB := b.seat(t, "b")
	entries, err := plain.Entries(b.installation)
	if err != nil || len(entries) != 3 || entries[0].State != plain.StateSuperseded {
		t.Fatalf("landed superseded fixture: %+v %v", entries, err)
	}
	if inside, err := plain.IsAncestor(b.checkout, oldA, "origin/main"); err != nil || !inside {
		t.Fatalf("earlier a hand-in is not on main: %v %v", inside, err)
	}

	b.success(t, "landing", "run")
	cheap := b.batch(t)
	if cheap.DepthClass != "cheap" || !slices.Equal(cheap.Members, []plain.GoalSHA{{Goal: "b", SHA: shaB}}) {
		t.Fatalf("landed ancestry joined unrelated work: %+v", cheap)
	}
	b.assemble(t, shaB)
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")
	b.success(t, "landing", "run")
	full := b.batch(t)
	if full.DepthClass != "full" || !slices.Equal(full.Members, []plain.GoalSHA{{Goal: "a", SHA: newA}}) {
		t.Fatalf("remaining a follow-up: %+v", full)
	}
	t.Log("landed a1; selected and pushed b alone at cheap depth; selected waiting a2 alone at full depth")
}
