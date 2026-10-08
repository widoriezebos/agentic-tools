package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

func newDesignGateBed(t *testing.T, tier uint8) *workBed {
	t.Helper()
	bed := newWorkBedWith(t, func(f *goal.GoalFile) {
		f.Tier = tier
		f.Risk = &goal.RiskRecord{Severity: tier, Novelty: tier, Exposure: 1, Accumulation: 1, Basis: "Exercise the design gate."}
		workApprovedBox(f)
	})
	// These existing gate fixtures represent pages accepted before size declarations.
	bed.workOwnersHook = func(o *intentWorkOwners) {
		fallback := o.git
		o.git = func(root string, args ...string) ([]byte, error) {
			if args[0] == "log" {
				return []byte("historical-declaration\n"), nil
			}
			if args[0] == "show" {
				_, path, _ := strings.Cut(args[1], ":")
				if strings.Contains(path, "plans/designs/") {
					return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
				}
			}
			return fallback(root, args...)
		}
	}
	return bed

}

func designGatePage(t *testing.T, bed *workBed, critique string, sections ...string) (string, []byte) {
	t.Helper()
	path := filepath.Join(bed.stateRoot(), "plans", "designs", "gate.md")
	// Each admitted fixture unit owns a Decision.
	data := []byte("# Gate design\n\n- Kind: design\n- Id: gate-design\n- Status: accepted\n- Goals: " + bed.id + "\n" + strings.TrimRight(critique, "\n") + "\n\n## u — Gate\n\nBuild the gate.\n\n## main — Gate\n\nBuild the gate.\n\n## ../other-goal/main — Gate\n\nBuild the gate.\n")
	data = append(data, []byte(strings.Join(sections, ""))...)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Main retains the accepted bytes even when the working page changes.
	hook := bed.workOwnersHook
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		if hook != nil {
			hook(owners)
		}
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			if slices.Equal(args, []string{"show", "origin/main:plans/designs/gate.md"}) {
				return append([]byte(nil), data...), nil
			}
			return git(root, args...)
		}
	}
	return path, data
}

func designGateBuild(t *testing.T, bed *workBed, unit string) (intentResult, string) {
	t.Helper()
	brief := bed.brief("gate-brief.md", "Build the gate.\n")
	code, result, output := bed.work(append([]string{"work", "build", bed.id, unit, "--brief", brief, "--lines", "40"}, workCheck...)...)
	if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
		t.Fatalf("the build did not start: code=%d result=%+v output=%s", code, result, output)
	}
	return result, output
}

func designGateRead(t *testing.T, bed *workBed, identity, unit string) ([]byte, designGateRecord) {
	t.Helper()
	path := filepath.Join(bed.unitRoot, ".design-gate", identity, bed.id, unit+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record designGateRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return data, record
}

func designGateLaunchInputs(t *testing.T, result intentResult) []string {
	t.Helper()
	data := resultData(t, result)
	plan, err := launch.ReadUnitPlan(data["plan"].(string))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(data["inputs"].(string), "request.json"), plan.Path, plan.Build.Brief}
	if plan.HasRead() {
		paths = append(paths, plan.Read.Brief)
	}
	return paths
}

func TestDesignGateIdentityFallbackUsesStateOwner(t *testing.T) {
	t.Parallel()
	bed := newDesignGateBed(t, 2)
	root, err := filepath.EvalSymlinks(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	bed.facts.root = root
	bed.config = func(key, path string) (string, string, int, error) {
		value, code, err := config.Get(config.GetParams{Key: key, ConfPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
		return value, "fixture", code, err
	}
	endpoint := goal.Endpoint{Root: bed.root(), Repository: bed.repo}
	want := goal.ExistingLedgerIdentityAtEndpoint(endpoint)
	if want == "" {
		t.Fatal("the fixture has no ledger identity")
	}
	bed.designGate.identity = nil
	designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)")
	designGateBuild(t, bed, "u")
	_, record := designGateRead(t, bed, want, "u")
	if record.LedgerIdentity != want || record.Verdict != "ok" {
		t.Fatalf("public build identity record = %+v", record)
	}
	delete(bed.repo.commit(bed.repo.accepted).files, "plans/goals/backlog.md")
	installation := stateroottest.Installation(t, filepath.Join(bed.root(), "vendor", "metasystem"))
	owners := bed.workOwners()
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		if root != installation.Path() {
			t.Fatalf("identity endpoint root = %q; want installation %q", root, installation.Path())
		}
		return endpoint, nil
	}
	inv := intentInvocation{owners: owners}
	if got, err := inv.designGate().identity(installation); err != nil || got != want {
		t.Fatalf("state ledger identity = %q, %v; want %q", got, err, want)
	}
}

func TestDesignGateRecordAnchorsFirstWriteAtStore(t *testing.T) {
	t.Parallel()
	bed := newDesignGateBed(t, 2)
	bed.unitRoot = t.TempDir()
	entries, err := os.ReadDir(bed.unitRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the unit store is not empty: entries=%v err=%v", entries, err)
	}
	identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
	wantPath := filepath.Join(bed.unitRoot, ".design-gate", identity, bed.id, "u.json")
	write := (&intentInvocation{}).designGate().record
	writes := 0
	bed.designGate.record = func(path, text, anchor string) (bool, error) {
		writes++
		if path != wantPath || anchor != bed.unitRoot {
			t.Fatalf("first record write: path=%q anchor=%q; want path=%q anchor=%q", path, anchor, wantPath, bed.unitRoot)
		}
		if info, err := os.Stat(anchor); err != nil || !info.IsDir() {
			t.Fatalf("the write anchor must already be a directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(anchor, ".design-gate")); !os.IsNotExist(err) {
			t.Fatalf("gate directories exist before the first write: %v", err)
		}
		return write(path, text, anchor)
	}
	designGateBuild(t, bed, "u")
	_, record := designGateRead(t, bed, identity, "u")
	if writes != 1 || record.LedgerIdentity != identity || record.Goal != bed.id || record.Unit != "u" || record.Verdict != "no-accepted-design" {
		t.Fatalf("first record was not published: writes=%d record=%+v", writes, record)
	}
}

func TestDesignGateWarnsAndStillBuilds(t *testing.T) {
	t.Parallel()
	bed := newDesignGateBed(t, 2)
	result, output := designGateBuild(t, bed, "u")
	want := "warning: goal " + bed.id + " has no accepted design; this build runs on its brief alone\nmetasystem design write " + bed.id + " --brief FILE\nwarning: estimate unavailable; the build goes on\n"
	if output != want {
		t.Fatalf("warning pair: got %q want %q", output, want)
	}
	identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
	before, record := designGateRead(t, bed, identity, "u")
	if record.Schema != 1 || record.LedgerIdentity != identity || record.Goal != bed.id || record.Unit != "u" || record.Worktree != bed.worktree || record.Tier != 2 || record.Mode != "warn" || record.Verdict != "no-accepted-design" || !record.WouldRefuse || record.Time.IsZero() {
		t.Fatalf("dispatch record: %+v", record)
	}
	gate := resultData(t, result)["designGate"].(map[string]any)
	if gate["verdict"] != record.Verdict || gate["wouldRefuse"] != true || gate["mode"] != "warn" {
		t.Fatalf("JSON gate: %v", gate)
	}
	digest, err := narratordigest.PendingWithLayoutReader(bed.stateRoot(), bed.owners().resolver.ResolveLayout)
	if err != nil || strings.Count(digest.Message, strings.Split(want, "\n")[0]) != 1 || !strings.Contains(digest.Message, "design-gate "+bed.id+"/u") {
		t.Fatalf("digest: %+v err=%v", digest, err)
	}
	launches := bed.starter.launched()
	designGateBuild(t, bed, "u")
	after, _ := designGateRead(t, bed, identity, "u")
	repeat, err := narratordigest.PendingWithLayoutReader(bed.stateRoot(), bed.owners().resolver.ResolveLayout)
	if err != nil || string(before) != string(after) || repeat.Message != digest.Message || !slices.Equal(bed.starter.launched(), launches) {
		t.Fatalf("repeat changed the record, digest or launches: digest=%+v err=%v launches=%v", repeat, err, bed.starter.launched())
	}
}

// A build run from inside its own goal worktree keeps the design check's
// line out of that worktree's tracked digest, which would enter the unit's
// change: the line goes to the primary checkout, or nowhere when the
// primary is that worktree too.
func TestDesignGateDigestStaysOutOfTheTreeUnderBuild(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"primary elsewhere", "primary is the worktree"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			bed.worktree = bed.root()
			if err := os.MkdirAll(filepath.Join(filepath.Dir(bed.worktree), "objects"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(bed.worktree), "index"), []byte("index"), 0o600); err != nil {
				t.Fatal(err)
			}
			if name == "primary elsewhere" {
				bed.primary = t.TempDir()
			}
			_, output := designGateBuild(t, bed, "u")
			own, err := narratordigest.PendingWithLayoutReader(bed.stateRoot(), bed.owners().resolver.ResolveLayout)
			if err != nil || strings.Contains(own.Message, "design-gate") {
				t.Fatalf("the worktree's own digest took the line: %+v err=%v", own, err)
			}
			if bed.primary == "" {
				if !strings.Contains(output, "digest line was left out") {
					t.Fatalf("the left-out line is not said: %q", output)
				}
				return
			}
			primary, err := narratordigest.PendingWithLayoutReader(bed.primary, bed.owners().resolver.ResolveLayout)
			if err != nil || strings.Count(primary.Message, "has no accepted design") != 1 || !strings.Contains(primary.Message, "design-gate "+bed.id+"/u") {
				t.Fatalf("the primary's digest: %+v err=%v output=%q", primary, err, output)
			}
		})
	}
}

func TestDesignGateStandingEvidence(t *testing.T) {
	t.Parallel()
	for _, tier := range []uint8{1, 2, 3} {
		t.Run(fmt.Sprint(tier), func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, tier)
			var page string
			var data []byte
			if tier > 1 {
				page, data = designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)\n")
			}
			result, output := designGateBuild(t, bed, "u")
			identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
			_, record := designGateRead(t, bed, identity, "u")
			want := "ok"
			if tier == 1 {
				want = "not-design-bearing"
			}
			if output != "warning: estimate unavailable; the build goes on\n" || record.Verdict != want || record.WouldRefuse {
				t.Fatalf("standing evidence: record=%+v output=%q", record, output)
			}
			if tier > 1 {
				if len(record.Designs) != 1 {
					t.Fatalf("accepted designs: %+v", record.Designs)
				}
				d := record.Designs[0]
				if d.ID != "gate-design" || d.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) || d.Path != "plans/designs/gate.md" || !strings.HasSuffix(filepath.ToSlash(page), d.Path) || d.Critique != "closed at round 2 on 0 material findings (WHO)" {
					t.Fatalf("design evidence: %+v", d)
				}
			}
			if resultData(t, result)["designGate"].(map[string]any)["verdict"] != want {
				t.Fatalf("JSON disagrees with record: %+v", result)
			}
		})
	}
}

func TestDesignGateNeverStallsWhenItBreaks(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"chain", "identity", "invalid identity", "record", "digest"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			problem := errors.New("fixture failure")
			want := "warning: the design check's record could not be written (fixture failure); the build goes on\nnothing to do: the landing check runs without it\n"
			switch failure {
			case "chain":
				designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)\n")
				bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return nil, problem }
				want = "warning: the design check could not run (fixture failure); this build was not checked\nmetasystem design list --goal " + bed.id + "\n"
			case "identity":
				bed.designGate.identity = func(stateroot.Installation) (string, error) { return "", problem }
			case "invalid identity":
				bed.designGate.identity = func(stateroot.Installation) (string, error) { return "../../escape", nil }
				want = "warning: the design check's record could not be written (the goal ledger identity is not 26 Crockford base32 characters); the build goes on\nnothing to do: the landing check runs without it\n"
			case "record":
				bed.designGate.record = func(string, string, string) (bool, error) { return false, problem }
			case "digest":
				bed.designGate.digest = func(string, narratordigest.Entry, time.Time) error { return problem }
			}
			result, output := designGateBuild(t, bed, "u")
			if !strings.Contains(output, want) {
				t.Fatalf("failure pair absent: %q", output)
			}
			if failure == "chain" {
				identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
				_, record := designGateRead(t, bed, identity, "u")
				if record.Verdict != "unchecked" || record.WouldRefuse || resultData(t, result)["designGate"].(map[string]any)["verdict"] != "unchecked" {
					t.Fatalf("broken check: %+v", record)
				}
			}
			if failure == "identity" || failure == "invalid identity" || failure == "record" {
				if _, err := os.Stat(filepath.Join(bed.unitRoot, ".design-gate")); !os.IsNotExist(err) {
					t.Fatalf("a failed record writer left gate records: %v", err)
				}
			}
			for _, path := range designGateLaunchInputs(t, result) {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("gate failure damaged launch inputs: %s: %v", path, err)
				}
			}
		})
	}
}

func TestDesignGateMalformedJobStillBuilds(t *testing.T) {
	t.Parallel()
	bed := newDesignGateBed(t, 2)
	bed.designGate.chains = nil
	designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)\n")
	chainRoot := t.TempDir()
	path := filepath.Join(chainRoot, "artifacts", "agents", "jobs", "design-critic-broken.json")
	bed.designGate.chains = func(_, id, page string) ([]designgate.Chain, error) {
		_, err := dispatchcore.ReadDesignCritiqueChains(chainRoot, id, page)
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"jobId":`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, output := designGateBuild(t, bed, "u")
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "warning: the design check could not run (") || !strings.Contains(lines[0], path) || !strings.HasSuffix(lines[0], "); this build was not checked") || lines[1] != "metasystem design list --goal "+bed.id {
		t.Fatalf("broken job warning pair: %q", output)
	}
	problem := strings.TrimSuffix(strings.TrimPrefix(lines[0], "warning: the design check could not run ("), "); this build was not checked")
	if lines[2] != "warning: estimate unavailable ("+problem+"); the build goes on at your word" {
		t.Fatalf("broken job estimate warning: %q", lines[2])
	}
	identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
	_, record := designGateRead(t, bed, identity, "u")
	gate := resultData(t, result)["designGate"].(map[string]any)
	if record.Verdict != "unchecked" || record.WouldRefuse || record.Mode != "warn" || gate["verdict"] != "unchecked" || gate["wouldRefuse"] != false || gate["mode"] != "warn" {
		t.Fatalf("broken check: record=%+v gate=%v", record, gate)
	}
	digest, err := narratordigest.PendingWithLayoutReader(bed.stateRoot(), bed.owners().resolver.ResolveLayout)
	if err != nil || strings.Count(digest.Message, lines[0]) != 1 || !strings.Contains(digest.Message, "design-gate "+bed.id+"/u") {
		t.Fatalf("broken check digest: %+v err=%v", digest, err)
	}
}

func TestDesignGateRecordsStayApartByProject(t *testing.T) {
	t.Parallel()
	store := t.TempDir()
	identities := []string{"01M4189Q0RH1NSPD3PNAS6G177", "01M4189Q0RH1NSPD3PNAS6G178"}
	var beds []*workBed
	for _, identity := range []string{identities[0], identities[1], identities[0]} {
		bed := newDesignGateBed(t, 2)
		bed.unitRoot = store
		bed.designGate.identity = func(stateroot.Installation) (string, error) { return identity, nil }
		designGateBuild(t, bed, "main")
		beds = append(beds, bed)
		for index, id := range identities[:min(len(beds), 2)] {
			_, record := designGateRead(t, bed, id, "main")
			owner := beds[index]
			if len(beds) == 3 && index == 0 {
				owner = beds[2]
			}
			if record.LedgerIdentity != id || record.Worktree != owner.worktree || record.Goal != bed.id || record.Unit != "main" {
				t.Fatalf("project records crossed: %+v", record)
			}
		}
	}
}

func TestDesignGateRecordRejectsUnsafePathSegments(t *testing.T) {
	t.Parallel()
	for _, part := range []string{"goal ledger identity", "goal id", "work name"} {
		for _, value := range []string{"", ".", "..", "other/main", `other\main`} {
			t.Run(part+"/"+value, func(t *testing.T) {
				t.Parallel()
				bed := newDesignGateBed(t, 2)
				facts, unit := designgate.Facts{Goal: bed.id}, "u"
				switch part {
				case "goal ledger identity":
					bed.designGate.identity = func(stateroot.Installation) (string, error) { return value, nil }
				case "goal id":
					facts.Goal = value
				case "work name":
					unit = value
				}
				bed.designGate.record = func(string, string, string) (bool, error) {
					t.Error("an unsafe path segment reached the record writer")
					return true, nil
				}
				bed.designGate.digest = func(string, narratordigest.Entry, time.Time) error { return nil }
				var output strings.Builder
				inv := intentInvocation{owners: bed.workOwners(), stateRoot: bed.stateRoot(), stderr: &output}
				inv.recordDesignGate(bed.unitRoot, bed.worktree, unit, facts, designgate.Result{}, false)
				if !strings.HasPrefix(output.String(), "warning: the design check's record could not be written (the "+part+" ") || !strings.HasSuffix(output.String(), "); the build goes on\nnothing to do: the landing check runs without it\n") {
					t.Fatalf("unsafe %s warning pair: %q", part, output.String())
				}
			})
		}
	}
}

func TestDesignGateRecordWorkNameCannotEscape(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"warn", "refuse"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			bed.lineage = "builder"
			setDesignGateMode(t, bed, mode)
			designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)")
			writes, write := 0, (&intentInvocation{}).designGate().record
			bed.designGate.record = func(path, text, anchor string) (bool, error) {
				writes++
				return write(path, text, anchor)
			}
			var lowlight string
			bed.designGate.digest = func(_ string, entry narratordigest.Entry, _ time.Time) error {
				lowlight = entry.Text
				return nil
			}
			_, output := designGateBuild(t, bed, "../other-goal/main")
			want := "warning: the design check's record could not be written (the work name is not one plain path segment); the build goes on\nnothing to do: the landing check runs without it\nwarning: estimate unavailable; the build goes on\n"
			if output != want || lowlight != strings.Split(want, "\n")[0] || writes != 0 {
				t.Fatalf("unsafe work name: writes=%d output=%q lowlight=%q", writes, output, lowlight)
			}
			if _, err := os.Stat(filepath.Join(bed.unitRoot, ".design-gate")); !os.IsNotExist(err) {
				t.Fatalf("unsafe work name left gate records: %v", err)
			}
		})
	}
}
