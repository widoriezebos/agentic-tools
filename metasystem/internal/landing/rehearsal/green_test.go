package rehearsal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

func TestFixtureGoalUsesTheRealLedgerGrammar(t *testing.T) {
	t.Parallel()
	template, err := os.ReadFile("../../../internal/landing/rehearsal/fixtures/green/goal.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(fixtureGoal(template, time.Now().UTC()))
	if len(problems) != 0 || file.Tier != 1 || file.Claimed.Machine != "rehearsal" || file.ValidateApprovalRecord() != nil {
		t.Fatalf("invalid tier-one fixture: %+v: %v", file, problems)
	}
}

func TestFixtureDesignUsesTheRealRecordAndUnitGrammar(t *testing.T) {
	t.Parallel()
	path := "../../../internal/landing/rehearsal/fixtures/green/design.md.tmpl"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record, problems, found := project.ParseRecord("plans/designs/rehearsal-green.md", string(data))
	if !found || len(problems) != 0 || record.Status != "accepted" || len(record.Goals) != 1 || record.Goals[0] != GreenGoal {
		t.Fatalf("invalid fixture design: %+v: %v", record, problems)
	}
	units, err := launch.DeclaredUnits(path)
	if err != nil || len(units) != 1 || units[0].Name != "u1" || units[0].Production == nil || *units[0].Production != 2 {
		t.Fatalf("fixture has no complete one-unit declaration: %+v: %v", units, err)
	}
}

func TestGreenProofRefusesFullMissingReasonAndMissingPlan(t *testing.T) {
	t.Parallel()
	green := plain.Result{Result: plain.Green, Scope: "impact", ScopeReason: "tiers 1; 0% of 200 packages", PlanHash: "plan"}
	if err := CheckGreenProof(green, true, nil); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"full", "reason", "plan", "counted", "execution", "missing", "read"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			proof, found, err := green, true, error(nil)
			switch mutation {
			case "full":
				proof.Scope = "full"
			case "reason":
				proof.ScopeReason = ""
			case "plan":
				proof.PlanHash = ""
			case "counted":
				proof.CountedFull = true
			case "execution":
				proof.Executions = []plain.ExecutionAdmission{{Scope: "full"}}
			case "missing":
				found = false
			case "read":
				err = errors.New("unreadable proof")
			}
			if err := CheckGreenProof(proof, found, err); err == nil || !strings.Contains(err.Error(), "depth") {
				t.Fatalf("mutation %s passed: %v", mutation, err)
			}
		})
	}
}

func TestRehearsalIdentityAndRecordKeepFailedScratch(t *testing.T) {
	t.Parallel()
	module := t.TempDir()
	path := recordScratch(t)
	engine := filepath.Join(path, "engine")
	if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("another-commit")), 0644); err != nil {
		t.Fatal(err)
	}
	s := &Scratch{Record: Record{Candidate: "candidate", Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path)), ctx: context.Background()}
	if err := s.Finish(module, time.Now(), false, nil); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("mismatched engine passed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(path, "records/rehearsals/tree.json"))
	var record Record
	if err != nil || json.Unmarshal(data, &record) != nil || record.Outcome != "failed" || record.Candidate != "candidate" {
		t.Fatalf("record = %s: %v", data, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("failed scratch was removed", err)
	}
}

func TestRehearsalRecordSuccessCleanupAndKeep(t *testing.T) {
	t.Parallel()
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "cleanup", true: "keep"}[keep], func(t *testing.T) {
			t.Parallel()
			module, path := t.TempDir(), recordScratch(t)
			engine := filepath.Join(path, "engine")
			if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("candidate")), 0644); err != nil {
				t.Fatal(err)
			}
			s := &Scratch{Record: Record{Candidate: "candidate", Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path))}
			if err := s.Finish(module, time.Now(), keep, nil); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(path)
			if (err == nil) != keep {
				t.Fatalf("keep=%v: scratch stat %v", keep, err)
			}
			stores, unreadable := diskstore.CheckoutRegistry(s.base).Inventory()
			want := diskstore.StateReleased
			if keep {
				want = diskstore.StateAccepted
			}
			if len(unreadable) != 0 || len(stores) != 1 || stores[0].State != want {
				t.Fatalf("cleanup lost the owner's storage history: %+v: %v", stores, unreadable)
			}
		})
	}
}

func TestRehearsalCancelledChildDoesNotRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := t.TempDir()
	origin := filepath.Join(root, "never-created.git")
	s := &Scratch{ctx: ctx, Env: os.Environ()}
	if _, err := s.Run(root, "git", "init", "--bare", origin); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled command was not refused: %v", err)
	}
	if _, err := os.Stat(origin); !os.IsNotExist(err) || len(s.Steps) != 1 || s.Steps[0].Exit != -1 {
		t.Fatalf("cancelled command ran or lost its exit: %v: %+v", err, s.Steps)
	}
}

func TestRehearsalCancelledSetupDoesNotRetry(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, err := RunGreen(ctx, t.TempDir(), os.Environ())
	if s != nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(s.Scratch))) })
	}
	if !errors.Is(err, context.Canceled) || s == nil || len(s.Steps) != 1 {
		t.Fatalf("cancelled setup retried or lost its first attempt: %+v: %v", s, err)
	}
}

func TestRehearsalSymlinkedSourceStaysInsideScratch(t *testing.T) {
	t.Parallel()
	root, alias := t.TempDir(), filepath.Join(t.TempDir(), "alias")
	module := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(module, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.invalid/fixture\n\ngo 1.27\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "rehearsal"}, {"config", "user.email", "rehearsal@example.invalid"}, {"add", "."}, {"commit", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, env
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, output, err)
		}
	}
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	s, err := Prepare(context.Background(), filepath.Join(alias, "metasystem"), env)
	if s != nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(s.Scratch))) })
	}
	if err == nil || s == nil || s.Module != filepath.Join(s.Checkout, "metasystem") {
		t.Fatalf("source alias escaped scratch before the expected missing builder: %+v: %v", s, err)
	}
}

func recordScratch(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	path, err := diskstore.CreateTempStoreAt(root, diskstore.CheckoutRegistry(root), "green", "rehearsal", diskstore.Owner{Kind: diskstore.OwnerWorkspace, Ref: filepath.Base(root)})
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRehearsalScratchSurvivesProcessRelease(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, err := Prepare(ctx, t.TempDir(), os.Environ())
	if s == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled setup did not create its retained scratch: %+v: %v", s, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(s.Scratch))) })
	if err := diskstore.ReleaseProcessScratch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Scratch); err != nil {
		t.Fatalf("process exit removed retained rehearsal: %v", err)
	}
	stores, unreadable := diskstore.CheckoutRegistry(filepath.Dir(filepath.Dir(s.Scratch))).Inventory()
	if len(unreadable) != 0 || len(stores) != 1 || stores[0].State != diskstore.StateAccepted || diskstore.Revalidate(stores[0]) != nil {
		t.Fatalf("rehearsal has no accepted private storage owner: %+v: %v", stores, unreadable)
	}
}

func TestRehearsalSetupRetryKeepsBothFailedScratches(t *testing.T) {
	t.Parallel()
	first, second := recordScratch(t), recordScratch(t)
	attempts := 0
	problem := errors.New("setup failed")
	prepare := func(ctx context.Context, _ string, _ []string) (*Scratch, error) {
		path := first
		if attempts > 0 {
			path = second
		}
		attempts++
		return &Scratch{Record: Record{Scratch: path, Steps: []Step{{Command: path, Exit: -1}}}, base: filepath.Dir(filepath.Dir(path)), ctx: ctx}, problem
	}
	s, err := runGreen(context.Background(), t.TempDir(), nil, prepare)
	if !errors.Is(err, problem) || attempts != 2 || s.Scratch != second || len(s.Steps) != 2 || s.Steps[0].Command != first {
		t.Fatalf("retry lost its failed attempts: %+v: %d: %v", s, attempts, err)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("failed setup scratch was removed: %s: %v", path, err)
		}
		stores, unreadable := diskstore.CheckoutRegistry(filepath.Dir(filepath.Dir(path))).Inventory()
		if len(unreadable) != 0 || len(stores) != 1 || stores[0].State != diskstore.StateAccepted {
			t.Fatalf("failed setup lost its owner record: %+v: %v", stores, unreadable)
		}
	}
}

func TestRehearsalSetupPreservesWorkerAllowanceAndDropsOptIn(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, err := Prepare(ctx, t.TempDir(), append(os.Environ(), "METASYSTEM_TEST_WORKERS=3", "METASYSTEM_TESTING_WORKERS=9", "METASYSTEM_REAL_REHEARSAL=1"))
	if s == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled setup: %+v: %v", s, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(s.base) })
	found := false
	for _, entry := range s.Env {
		found = found || entry == "METASYSTEM_TEST_WORKERS=3"
		if strings.HasPrefix(entry, "METASYSTEM_TESTING_WORKERS=") || strings.HasPrefix(entry, "METASYSTEM_REAL_REHEARSAL=") {
			t.Fatalf("scratch retained an unused worker setting or recursive test opt-in: %s", entry)
		}
	}
	if !found {
		t.Fatal("scratch lost the inherited test worker allowance")
	}
}

func TestRehearsalSuccessCleanupRefusesMissingStoreMarker(t *testing.T) {
	t.Parallel()
	module, path := t.TempDir(), recordScratch(t)
	engine := filepath.Join(path, "engine")
	if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("tree")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(path, diskstore.MarkerName)); err != nil {
		t.Fatal(err)
	}
	s := &Scratch{Record: Record{Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path))}
	if err := s.Finish(module, time.Now(), false, nil); err == nil {
		t.Fatal("cleanup removed a store with no owner marker")
	}
	if _, err := os.Stat(engine); err != nil {
		t.Fatalf("refused cleanup removed the payload: %v", err)
	}
}
