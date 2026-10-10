package plain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestSelectBatchDepthClasses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, threshold, policy, class string
		tiers                          []uint8
		links                          map[string]bool
		want                           []string
	}{
		{name: "mixed cheap first", tiers: []uint8{3, 1, 2, 3}, want: []string{"g1", "g2"}, class: "cheap"},
		{name: "all cheap", tiers: []uint8{1, 2}, want: []string{"g0", "g1"}, class: "cheap"},
		{name: "all full", tiers: []uint8{3, 3}, want: []string{"g0", "g1"}, class: "full"},
		{name: "empty"},
		{name: "cheap descendant of full", tiers: []uint8{3, 1}, links: map[string]bool{"g0 g1": true}, want: []string{"g0", "g1"}, class: "full"},
		{name: "full descendant of cheap", tiers: []uint8{1, 3}, links: map[string]bool{"g0 g1": true}, want: []string{"g0", "g1"}, class: "full"},
		{name: "transitive ancestry", tiers: []uint8{1, 3, 1, 2}, links: map[string]bool{"g0 g2": true, "g1 g2": true}, want: []string{"g3"}, class: "cheap"},
		{name: "configured threshold", threshold: "2", tiers: []uint8{2, 1}, want: []string{"g1"}, class: "cheap"},
		{name: "unknown tier is full", tiers: []uint8{0, 1}, want: []string{"g1"}, class: "cheap"},
		{name: "numeric policy unchanged", policy: "2", tiers: []uint8{3, 1, 2}, want: []string{"g0", "g1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			threshold := tc.threshold
			if threshold == "" {
				threshold = "3"
			}
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("landing.full-from-tier="+threshold+"\nproof.trunk-every=1h\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for i := range tc.tiers {
				id := fmt.Sprintf("g%d", i)
				if _, _, err := HandIn(install, Line{Goal: id, SHA: id}); err != nil {
					t.Fatal(err)
				}
			}
			notAncestor := exec.Command("/usr/bin/false").Run()
			landed := map[string]bool{}
			seams := ProveSeams{Now: func() time.Time { return bedNow }, NewID: func() string { return "batch" },
				GoalTier: func(_, id string) (uint8, error) {
					var i int
					_, _ = fmt.Sscanf(id, "g%d", &i)
					return tc.tiers[i], nil
				},
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
				Git: func(_ string, args ...string) (string, error) {
					switch args[0] {
					case "fetch", "cat-file":
						return "", nil
					case "rev-parse":
						return "main", nil
					case "merge-base":
						if tc.links[args[2]+" "+args[3]] || args[3] == "main" && landed[args[2]] {
							return "", nil
						}
						return "", notAncestor
					default:
						t.Fatalf("unexpected Git: %v", args)
						return "", nil
					}
				}}
			policy := tc.policy
			if policy == "" {
				policy = "auto"
			}
			seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: policy}, nil }
			batch, err := SelectBatch(install, install, lane.Record{Root: install, Install: install, CustodyEpoch: 1}, seams)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == nil {
				if batch != nil {
					t.Fatalf("empty queue selected %+v", batch)
				}
				return
			}
			var got []string
			for _, member := range batch.Members {
				got = append(got, member.Goal)
			}
			if !reflect.DeepEqual(got, tc.want) || batch.DepthClass != tc.class {
				t.Fatalf("batch %+v, want %v at %s", batch, tc.want, tc.class)
			}
			if tc.name == "mixed cheap first" {
				// A confirmed landing closes the cheap selection before selecting the remaining full lines.
				for _, member := range batch.Members {
					landed[member.SHA] = true
				}
				batch.State, batch.ClosureReason = BatchClosed, "confirmed push accounts for the selected members"
				if err := writeBatch(install, batch); err != nil {
					t.Fatal(err)
				}
				next, err := SelectBatch(install, install, batch.Lane, seams)
				if err != nil || next == nil || next.DepthClass != "full" || !slices.Equal(next.Members, []GoalSHA{{Goal: "g0", SHA: "g0"}, {Goal: "g3", SHA: "g3"}}) {
					t.Fatalf("full batch after cheap: %+v %v", next, err)
				}
			}
		})
	}
}

func TestFullDueBatchDepthAndClock(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	old := b.base
	old.Trunk, old.At = true, bedNow.Add(-5*time.Hour).Format(time.RFC3339)
	old.FullAt = old.At
	b.save(t, old)
	batch := &Batch{ID: "cheap", DepthClass: "cheap", State: BatchRunning, Base: "main", CreatedAt: bedNow.Format(time.RFC3339), Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "g", SHA: "sha"}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	running := Running{BatchID: batch.ID, Commit: b.git.commit, Tree: b.git.tree}
	depthCalls := 0
	b.seams.BatchDepth = func(string, string, Running) (string, string) { depthCalls++; return "impact", "low tier" }
	reasons, err := wakeReasons(b.install, true, time.Time{}, bedNow)
	if err != nil || !slices.Contains(reasons, WakeFullDue) {
		t.Fatalf("wake: %v %v", reasons, err)
	}
	want := "full proof overdue since " + bedNow.Add(-time.Hour).Local().Format("15:04")
	d := proofScope(b.install, b.checkout, running, b.seams)
	if d.Scope != "full" || d.ScopeReason != want || depthCalls != 0 {
		t.Fatalf("overdue decision: %+v calls=%d", d, depthCalls)
	}
	if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "sha"}); err != nil {
		t.Fatal(err)
	}
	git := b.seams.Git
	notAncestor := exec.Command("/usr/bin/false").Run()
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "cat-file":
			return "", nil
		case "merge-base":
			if args[3] == b.git.commit && (args[2] == "main" || args[2] == "sha") {
				return "", nil
			}
			return "", notAncestor
		case "rev-list":
			if args[1] == "--first-parent" {
				return b.git.commit + " main sha", nil
			}
		case "rev-parse":
			if slices.Contains(args, "refs/remotes/origin/main^{commit}") {
				return "main", nil
			}
		}
		return git(dir, args...)
	}
	b.save(t, Result{Tree: b.git.tree, Commit: b.git.commit, Result: Green, Scope: "impact", At: bedNow.Format(time.RFC3339)})
	if _, settled, err := Settled(b.install, b.checkout, b.seams); err != nil || settled {
		t.Fatalf("overdue batch reused impact green: %v %v", settled, err)
	}
	result, err := Run(b.install, b.checkout, "proof-command", "", b.output, b.seams)
	if err != nil || result.Result != Green || result.Scope != "full" || result.ScopeReason != want || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" {
		t.Fatalf("overdue proof: %+v %v", result, err)
	}
	if !batchGreenReusable(b.install, batch.ID, result, b.seams) {
		t.Fatal("fresh full proof cannot be reused before push")
	}

	if due, err := fullProofDue(b.install, bedNow); err != nil || !due {
		t.Fatalf("unpublished proof paid clock: %v %v", due, err)
	}
	writePushRecords(t, b.install, Pushed{Tree: result.Tree, At: bedNow.Format(time.RFC3339), BatchID: batch.ID})
	if due, err := fullProofDue(b.install, bedNow); err != nil || due {
		t.Fatalf("pushed full did not pay clock: %v %v", due, err)
	}
	d = proofScope(b.install, b.checkout, running, b.seams)
	if d.Scope != "impact" || d.ScopeReason != "low tier" || depthCalls != 1 {
		t.Fatalf("paid clock did not release ordinary depth: %+v calls=%d", d, depthCalls)
	}
}

func TestSelectBatchDepthReadFailures(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	entries := []Entry{{Goal: "missing", SHA: "sha"}}
	selected, class, err := selectDepthClass(install, install, entries, ProveSeams{GoalTier: func(string, string) (uint8, error) { return 1, errors.New("unreadable") }})
	if err != nil || class != "full" || len(selected) != 1 {
		t.Fatalf("unreadable tier: %v %s %v", selected, class, err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("landing.full-from-tier=invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := selectDepthClass(install, install, entries, ProveSeams{}); err == nil {
		t.Fatal("invalid threshold admitted")
	}
}

func TestSelectBatchReadsGoalFileTier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		tier    uint8
		corrupt bool
		class   string
	}{
		{"below", 2, false, "cheap"}, {"threshold", 3, false, "full"}, {"corrupt", 1, true, "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			file := &goal.GoalFile{Id: "g", State: goal.StateQueued, Tier: tc.tier, Intent: "fixture tier", Origin: goal.OriginHuman, OpenedAt: bedNow.Format(time.RFC3339), Revision: 1,
				History: []goal.HistoryLine{{At: bedNow.Format(time.RFC3339), Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Targets: []string{"g"}, Keep: -1}}}
			data := goal.RenderFile(file)
			if _, problems := goal.ParseFile(data); len(problems) != 0 {
				t.Fatalf("invalid goal fixture: %v", problems)
			}
			if tc.corrupt {
				data = append(data, []byte("- Tier: 1\n")...)
			}
			dir := filepath.Join(install, "plans", "goals")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "g.md"), data, 0600); err != nil {
				t.Fatal(err)
			}
			_, class, err := selectDepthClass(install, install, []Entry{{Goal: "g", SHA: "sha"}}, ProveSeams{})
			if err != nil || class != tc.class {
				t.Fatalf("goal tier selected %s, want %s: %v", class, tc.class, err)
			}
		})
	}
}

func TestSelectBatchDepthAncestryFailure(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	entries := []Entry{{Goal: "cheap", SHA: "cheap-tip"}, {Goal: "full", SHA: "full-tip"}}
	seams := ProveSeams{GoalTier: func(_, id string) (uint8, error) {
		if id == "cheap" {
			return 1, nil
		}
		return 3, nil
	},
		Git: func(_ string, args ...string) (string, error) {
			if args[0] == "cat-file" {
				return "", nil
			}
			return "", errors.New("ancestry unavailable")
		}}
	if selected, _, err := selectDepthClass(install, install, entries, seams); err == nil || selected != nil {
		t.Fatalf("unknown ancestry admitted split: %v %v", selected, err)
	}
	if batch, err := ReadBatch(install); err != nil || batch != nil {
		t.Fatalf("failed selection wrote batch: %+v %v", batch, err)
	}
}

func TestFullDueBatchFreshGreenWithinSelectionSecond(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=4h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	batch := &Batch{ID: "batch", Base: "main", State: BatchRunning, CreatedAt: bedNow.Add(500 * time.Millisecond).Format(time.RFC3339Nano), Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{Now: func() time.Time { return bedNow.Add(600 * time.Millisecond) }}
	proof := Result{Result: Green, Scope: "full", Tree: "tree", FullTree: "tree", At: bedNow.Format(time.RFC3339), FullAt: bedNow.Format(time.RFC3339)}
	if !batchGreenReusable(install, batch.ID, proof, seams) {
		t.Fatal("timestamp precision rejected a fresh full green")
	}
	proof.At, proof.FullAt = bedNow.Add(-time.Second).Format(time.RFC3339), bedNow.Add(-time.Second).Format(time.RFC3339)
	if batchGreenReusable(install, batch.ID, proof, seams) {
		t.Fatal("green from before selection paid overdue proof")
	}
}
