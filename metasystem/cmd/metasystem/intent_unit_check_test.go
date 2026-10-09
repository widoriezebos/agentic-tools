package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

type declaredCheckStarter struct {
	bed         *workBed
	t           *testing.T
	results     []intentResult
	calls       []int
	beforeBuild func()
	beforeCheck func(launch.Record) func()
}

func (s *declaredCheckStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	var args []string
	directory := record.WorkingDirectory
	if record.Kind == "build" {
		if s.beforeBuild != nil {
			s.beforeBuild()
		}
		var brief string
		if err := json.Unmarshal(record.AdapterData["brief"], &brief); err != nil {
			s.t.Fatal(err)
		}
		data, err := os.ReadFile(brief)
		if err != nil {
			s.t.Fatal(err)
		}
		prefix := "Before returning, run: metasystem test run --unit-run "
		if strings.Contains(string(data), "Proof after the build, run without a shell") {
			s.t.Fatal("builder received a second, unfrozen check")
		}
		line := strings.SplitN(string(data), "\n", 2)[0]
		if !strings.HasPrefix(line, prefix) {
			s.t.Fatalf("builder has no frozen-check invocation: %s", line)
		}
		run := strings.TrimPrefix(line, prefix)
		// Candidate settings and a later tip must not choose this round's checks.
		s.bed.head = "candidate-commit"
		if err := os.WriteFile(filepath.Join(s.bed.worktree, "metasystem.conf"), []byte("proof.cheap=true\nproof.audits=true\nproof.deadline=1\n"), 0600); err != nil {
			s.t.Fatal(err)
		}
		args = []string{"test", "run", "--unit-run", run}
	} else if record.Kind == "proof" {
		// Only the external process boundary is replaced: decode the actual plain
		// adapter command, then enter that public verb in this test process.
		command, err := (launch.PlainExec{}).Command(record, state)
		if err != nil {
			s.t.Fatal(err)
		}
		if !slices.Equal(command.Args[:3], []string{"test", "run", "--unit-run"}) {
			s.t.Fatalf("extra proof selected: %+v", command)
		}
		args = command.Args
		directory = command.Directory
	}
	if args != nil {
		if s.beforeCheck != nil {
			defer s.beforeCheck(record)()
		}
		code, result := declaredCheckAt(s.t, s.bed, directory, record.Kind, args...)
		s.results, s.calls = append(s.results, result), append(s.calls, code)
		if record.Kind == "proof" {
			s.bed.starter.fail["proof"] = code != 0
		}
	}
	_, err = s.bed.starter.StartSupervisor(id, state)
	return workProcessRef(30), err
}

func declaredCheckBed(t *testing.T, body string) *workBed {
	t.Helper()
	b := newWorkBed(t)
	b.workOwnersHook = func(owners *intentWorkOwners) {
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			switch {
			case slices.Equal(args, []string{"rev-parse", "base-commit^{tree}"}):
				return []byte("declaration-tree"), nil
			case slices.Equal(args, []string{"rev-parse", "candidate-commit^{tree}"}):
				return []byte("candidate-tree"), nil
			case len(args) == 2 && args[0] == "show" && strings.HasPrefix(args[1], "base-commit:"):
				return []byte(body), nil
			case len(args) == 2 && args[0] == "show" && strings.HasPrefix(args[1], "candidate-commit:"):
				return []byte("proof.cheap=true\nproof.audits=true\nproof.deadline=1\n"), nil
			}
			return git(root, args...)
		}
		owners.testRun = func(string, []string, io.Writer) ([]byte, int, error) {
			t.Fatal("frozen check resolved a fresh test selection")
			return nil, 1, nil
		}
		owners.adapter = func(string) (adapter.Adapter, error) {
			t.Fatal("declared check selected adapter tests")
			return nil, nil
		}
	}
	return b
}

func TestIntentDeclaredCheckFrozenAcrossBuilderAndProof(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		cheap, audit int
	}{{"green", 0, 0}, {"audit-red", 0, 17}, {"cheap-red", 9, 0}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			script := filepath.Join(t.TempDir(), "check")
			text := fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s|%%s\\n' \"$1\" \"$PWD\" \"$PATH\"\ncase $1 in cheap) exit %d;; audits) exit %d;; esac\n", row.cheap, row.audit)
			if err := testexec.WriteFile(script, []byte(text), 0700); err != nil {
				t.Fatal(err)
			}
			body := "proof.cheap=" + shellCommand([]string{script, "cheap"}) + "\nproof.audits=" + shellCommand([]string{script, "audits"}) + "\nproof.deadline=15\n"
			b := declaredCheckBed(t, body)
			starter := &declaredCheckStarter{bed: b, t: t}
			b.manager.Supervisor = starter
			brief := b.brief("declared.md", "Build the declared check.\n")
			code, built, _ := b.work("work", "build", b.id, "declared", "--brief", brief, "--lines", "100")
			want := "green"
			checkCode := 0
			if row.cheap != 0 || row.audit != 0 {
				want, checkCode = "proof-red", 1
			}
			if code != checkCode || resultData(t, built)["outcome"] != want || !slices.Equal(starter.calls, []int{checkCode, checkCode}) {
				t.Fatalf("build=%d %+v calls=%v", code, built, starter.calls)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if plan.Check.Base != plan.Base || plan.Check.SourceTree != "declaration-tree" || plan.Check.Minutes != 15 || !slices.Equal(plan.Check.Environment, os.Environ()) || len(plan.Proof) != 1 {
				t.Fatal("declaration tree, minutes, environment or commands were not frozen")
			}
			canonicalDirectory, err := filepath.EvalSymlinks(b.worktree)
			if err != nil {
				t.Fatal(err)
			}
			var first string
			for _, result := range starter.results {
				data := resultData(t, result)
				execution := data["execution"].(string)
				if execution == first {
					t.Fatal("builder and attestation borrowed one execution")
				}
				first = execution
				var retained struct {
					Check launch.UnitCheck
					Exits []launch.CheckExit
				}
				raw, err := os.ReadFile(filepath.Join(execution, "result.json"))
				if err != nil || json.Unmarshal(raw, &retained) != nil {
					t.Fatalf("retained results: %s %v", raw, err)
				}
				if len(retained.Exits) != 2 || retained.Exits[0].Exit != row.cheap || retained.Exits[1].Exit != row.audit {
					t.Fatalf("exits lost: %+v", retained.Exits)
				}
				for _, exit := range retained.Exits {
					if exit.Minutes < 0 || exit.Output != exit.Name+"|"+canonicalDirectory+"|"+os.Getenv("PATH")+"\n" {
						t.Fatalf("different environment/directory/output: %+v", exit)
					}
				}
			}
			// Repeated public execution also ignores the moved tip and candidate settings.
			code, result := declaredCheckAt(t, b, b.root(), "", "test", "run", "--unit-run", resultData(t, built)["run"].(string))
			if code != checkCode || resultData(t, result)["execution"] == first {
				t.Fatalf("public replay: %d %+v", code, result)
			}
		})
	}
}

func TestIntentDeclaredCheckRejectsMissingAndUnreadableDeclarations(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "proof.cheap=true\n", "proof.cheap=true\nproof.audits=true\nproof.deadline=0\n", "proof.cheap=true\nproof.cheap=false\nproof.audits=true\nproof.deadline=15\n"} {
		t.Run(fmt.Sprintf("%x", body), func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, body)
			brief := b.brief("missing.md", "Build declarations.\n")
			code, result, _ := b.work("work", "build", b.id, "missing", "--brief", brief, "--lines", "100")
			if code == 0 || !strings.Contains(result.Summary, "proof.") || len(b.starter.launched()) != 0 {
				t.Fatalf("bad declaration launched: %d %+v", code, result)
			}
		})
	}
	b := declaredCheckBed(t, "")
	hook := b.workOwnersHook
	b.workOwnersHook = func(owners *intentWorkOwners) {
		hook(owners)
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "show" {
				return nil, errors.New("committed content unreadable")
			}
			return git(root, args...)
		}
	}
	code, result, _ := b.work("work", "build", b.id, "unreadable", "--brief", b.brief("unreadable.md", "Build.\n"), "--lines", "100")
	if code == 0 || !strings.Contains(result.Summary, "unreadable") || len(b.starter.launched()) != 0 {
		t.Fatalf("unreadable declaration launched: %d %+v", code, result)
	}
}

func TestIntentDeclaredCheckManualRepairRequiresPerson(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		agent  bool
		reason string
	}{{"person", false, "Repair missing declarations"}, {"agent", true, "Override declarations"}, {"missing-reason", false, ""}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "")
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			owners := b.workOwners()
			var policyCalls int
			units := owners.work.units
			owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
				r := units(layout)
				r.ReviewPolicy = func() (string, error) { policyCalls++; return "", errors.New("broken advisory policy") }
				return r
			}
			git := owners.work.git
			owners.work.git = func(root string, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "show" {
					t.Fatal("manual repair read broken declarations")
				}
				return git(root, args...)
			}
			if row.agent {
				owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, humanauthority.AgentRefused("codex")
				}
			}
			var stdout, stderr bytes.Buffer
			starter := &declaredCheckStarter{bed: b, t: t, beforeBuild: func() {
				if !strings.Contains(stderr.String(), "Impact:") {
					t.Fatal("repair launched before impact was printed")
				}
				records, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
				if err != nil || len(records) != 1 {
					t.Fatalf("impact missing before launch: %v %v", records, err)
				}
				data, err := os.ReadFile(records[0])
				if err != nil || !strings.Contains(string(data), row.reason) || !strings.Contains(string(data), "Wido") {
					t.Fatalf("repair provenance missing: %s %v", data, err)
				}
			}}
			b.manager.Supervisor = starter
			args := []string{"work", "build", b.id, "repair", "--brief", b.brief("repair.md", "Repair the declarations.\n"), "--lines", "100", "--reason", row.reason, "--by", "Wido", "--check", "/bin/sh", "-c", "exit 23"}
			command, rest, _ := resolveIntentArgv(args)
			code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, b.root(), owners)
			var result intentResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("repair result: %s %v", stdout.String(), err)
			}
			if policyCalls != 0 {
				t.Fatal("repair read the broken advisory policy")
			}
			if row.agent || row.reason == "" {
				if code == 0 || len(b.starter.launched()) != 0 {
					t.Fatalf("unproven/unreasoned repair launched: %d %+v", code, result)
				}
				return
			}
			if code != 1 || resultData(t, result)["outcome"] != "proof-red" || !slices.Equal(starter.calls, []int{1, 1}) {
				t.Fatalf("repair did not execute its actual red: %d %+v calls=%v", code, result, starter.calls)
			}
			for _, checkResult := range starter.results {
				data, err := os.ReadFile(filepath.Join(resultData(t, checkResult)["execution"].(string), "result.json"))
				var retained struct {
					ExecutionID string
					Exits       []launch.CheckExit
				}
				if err != nil || json.Unmarshal(data, &retained) != nil || retained.ExecutionID == "" || len(retained.Exits) != 2 || retained.Exits[0].Exit != 23 || retained.Exits[1].Exit != 0 {
					t.Fatalf("manual repair did not retain its actual exits: %s %v", data, err)
				}
			}
			plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
			if err != nil || plan.Check.Base != plan.Base || plan.Check.SelectedBy != "Wido" || plan.Check.Reason != row.reason || plan.Check.Audits != "true" || len(plan.FullArgv) != 0 {
				t.Fatalf("manual check provenance missing: %v", err)
			}
		})
	}
}

func TestIntentDeclaredCheckNextRoundUsesNewDeclarations(t *testing.T) {
	t.Parallel()
	b := declaredCheckBed(t, "proof.cheap=exit 7\nproof.audits=true\nproof.deadline=15\n")
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	brief := b.brief("round.md", "Build the first round.\n")
	code, result, _ := b.work("work", "build", b.id, "round", "--brief", brief, "--lines", "100")
	if code != 1 || resultData(t, result)["outcome"] != "proof-red" {
		t.Fatalf("first round: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	first, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil {
		t.Fatal(err)
	}
	followUp := b.brief("follow.md", "Use the next round's repaired declarations.\n")
	code, result, _ = stopPublic(t, b, "broken-policy", "work", "revise", "run:"+run, "--brief", followUp, "--reason", "Repair the failed check", "--by", "Wido")
	if code != 0 || resultData(t, result)["outcome"] != "green" {
		t.Fatalf("next round: %d %+v", code, result)
	}
	next, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || first.Check.Cheap != "exit 7" || next.Check.Cheap != "true" || first.Check.SourceTree == next.Check.SourceTree {
		t.Fatalf("next round reused declarations: %v", err)
	}
	data, err := os.ReadFile(next.Build.Brief)
	if err != nil || !strings.Contains(string(data), "Use the next round's repaired declarations.") {
		t.Fatalf("follow-up lost: %s %v", data, err)
	}
	if !slices.Equal(starter.calls, []int{1, 1, 0, 0}) {
		t.Fatalf("round executions: %v", starter.calls)
	}
}

// declaredCheckAt enters the public verb with the plain adapter's actual directory.
func declaredCheckAt(t *testing.T, b *workBed, directory, kind string, args ...string) (int, intentResult) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	owners := b.workOwners()
	owners.lookupEnv = func(key string) (string, bool) { return kind, key == launch.KindEnv }
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			for _, tree := range []string{b.root(), b.worktree} {
				if _, err := fakeTop(tree)(root); err == nil {
					return []byte(tree), nil
				}
			}
			for candidate := directory; ; candidate = filepath.Dir(candidate) {
				if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
					return []byte(candidate), nil
				}
				if filepath.Dir(candidate) == candidate {
					break
				}
			}
			return []byte(directory), nil
		}
		return git(root, args...)
	}
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, directory, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("result: %s stderr=%s: %v", stdout.String(), stderr.String(), err)
	}
	return code, result
}

func TestIntentDeclaredCheckPublicationRunsInCommitWorktree(t *testing.T) {
	t.Parallel()
	phase := filepath.Join(t.TempDir(), "publication")
	script := filepath.Join(t.TempDir(), "check")
	body := "#!/bin/sh\nif test -f " + shellCommand([]string{phase}) + "; then cat gate-only; else printf 'builder\\n'; fi\n"
	if err := testexec.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	b := declaredCheckBed(t, "proof.cheap="+shellCommand([]string{script})+"\nproof.audits=printf '%s' \"$PATH\"\nproof.deadline=15\n")
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	code, built, _ := b.work("work", "build", b.id, "publication", "--brief", b.brief("publication.md", "Build.\n"), "--lines", "100")
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	gate := t.TempDir()
	if err := os.WriteFile(filepath.Join(gate, "gate-only"), []byte("commit tree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phase, nil, 0600); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	layout, err := owners.resolver.ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	runner := owners.work.units(layout)
	run := resultData(t, built)["run"].(string)
	err = runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := launch.UnitSubject{Round: review.Round.Number, Operation: "publication"}
		_, err := runner.ProveRoundResult(review, gate, &subject, retain)
		return err
	})
	if err != nil {
		t.Fatalf("publication check: %v calls=%v results=%+v", err, starter.calls, starter.results)
	}
	if len(starter.calls) != 3 || starter.calls[2] != 0 {
		t.Fatalf("gate did not enter the public check: %v", starter.calls)
	}
	data := resultData(t, starter.results[2])
	raw, err := os.ReadFile(filepath.Join(data["execution"].(string), "result.json"))
	var retained struct {
		Directory string `json:"runDirectory"`
		Check     launch.UnitCheck
		Exits     []launch.CheckExit
	}
	if err != nil || json.Unmarshal(raw, &retained) != nil {
		t.Fatalf("gate record: %s %v", raw, err)
	}
	if retained.Directory != gate || retained.Check.Directory != b.worktree || retained.Check.SourceTree != "declaration-tree" || retained.Check.Minutes != 15 || !slices.Equal(retained.Check.Environment, os.Environ()) || len(retained.Exits) != 2 || retained.Exits[0].Output != "commit tree\n" || retained.Exits[1].Output != os.Getenv("PATH") {
		t.Fatalf("gate did not record the new tree under frozen declarations: %+v", retained)
	}
}

func TestIntentDeclaredCheckBuilderRetainsWithUnwritableRound(t *testing.T) {
	t.Parallel()
	b := declaredCheckBed(t, "proof.cheap=printf cheap\nproof.audits=printf audits\nproof.deadline=15\n")
	starter := &declaredCheckStarter{bed: b, t: t}
	starter.beforeCheck = func(execution launch.Record) func() {
		if execution.Kind != "build" {
			return func() {}
		}
		rounds, err := filepath.Glob(filepath.Join(b.unitRoot, "*", "round-1"))
		if err != nil || len(rounds) != 1 {
			t.Fatalf("builder round: %v %v", rounds, err)
		}
		round := rounds[0]
		if err := os.Chmod(round, 0500); err != nil {
			t.Fatal(err)
		}
		_, err = os.MkdirTemp(round, "permission-probe-")
		if !os.IsPermission(err) {
			t.Fatalf("round is not unwritable: %v", err)
		}
		return func() {
			if err := os.Chmod(round, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	b.manager.Supervisor = starter
	code, built, _ := b.work("work", "build", b.id, "permissions", "--brief", b.brief("permissions.md", "Build.\n"), "--lines", "100")
	if code != 0 || !slices.Equal(starter.calls, []int{0, 0}) {
		t.Fatalf("builder could not run its check: %d %+v calls=%v", code, built, starter.calls)
	}
	planPath := resultData(t, built)["plan"].(string)
	for index, result := range starter.results {
		execution := resultData(t, result)["execution"].(string)
		builder := index == 0
		if builder != withinDirectory(execution, filepath.Join(b.worktree, "artifacts")) || (!builder && filepath.Dir(execution) != filepath.Dir(planPath)) {
			t.Fatalf("execution written under the wrong owner: %s", execution)
		}
		original, err := os.ReadFile(filepath.Join(execution, "result.json"))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(execution)
		if builder {
			name = "builder-" + name
		}
		collected, err := os.ReadFile(filepath.Join(filepath.Dir(planPath), name, "result.json"))
		var retained struct{ Exits []launch.CheckExit }
		if err != nil || !bytes.Equal(original, collected) || json.Unmarshal(collected, &retained) != nil || len(retained.Exits) != 2 || retained.Exits[0].Output != "cheap" || retained.Exits[1].Output != "audits" {
			t.Fatalf("runner did not retain both exits under their owner: %s %v", collected, err)
		}
	}
}

func TestIntentDeclaredCheckBuilderUsesModuleDirectory(t *testing.T) {
	t.Parallel()
	b := declaredCheckBed(t, "proof.cheap=cat module-only\nproof.audits=pwd\nproof.deadline=15\n")
	for _, tree := range []string{b.root(), b.worktree} {
		if err := os.MkdirAll(filepath.Join(tree, "module"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tree, "module", "module-only"), []byte("module check\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	code, built := declaredCheckAt(t, b, filepath.Join(b.root(), "module"), "", "work", "build", b.id, "module", "--brief", filepath.Join(b.root(), b.brief("module.md", "Build.\n")), "--lines", "100")
	if code != 0 || !slices.Equal(starter.calls, []int{0, 0}) {
		t.Fatalf("builder from the tree root and proof from the module: %d %+v calls=%v", code, built, starter.calls)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil || plan.Check.Directory != filepath.Join(b.worktree, "module") {
		t.Fatalf("frozen module: %+v %v", plan.Check, err)
	}
	run := resultData(t, built)["run"].(string)
	for _, directory := range []string{b.worktree, filepath.Join(b.worktree, "module"), filepath.Join(b.root(), "module")} {
		code, result := declaredCheckAt(t, b, directory, "", "test", "run", "--unit-run", run)
		if code != 0 {
			t.Fatalf("check from %s: %d %+v", directory, code, result)
		}
		starter.results = append(starter.results, result)
	}
	for index, result := range starter.results {
		var retained struct {
			Directory string `json:"runDirectory"`
			Exits     []launch.CheckExit
		}
		raw, err := os.ReadFile(filepath.Join(resultData(t, result)["execution"].(string), "result.json"))
		if err != nil || json.Unmarshal(raw, &retained) != nil {
			t.Fatalf("execution %d: %s %v", index, raw, err)
		}
		tree := b.worktree
		if index == len(starter.results)-1 {
			tree = b.root()
		}
		want, err := filepath.EvalSymlinks(filepath.Join(tree, "module"))
		actual, actualErr := filepath.EvalSymlinks(retained.Directory)
		if err != nil || actualErr != nil || actual != want || len(retained.Exits) != 2 || retained.Exits[0].Output != "module check\n" || retained.Exits[1].Output != want+"\n" {
			t.Fatalf("execution %d did not use the calling tree's module: %+v want=%s err=%v", index, retained, want, err)
		}
	}
}

func TestIntentDeclaredCheckLaterBuilderCannotReplaceRetainedEvidence(t *testing.T) {
	t.Parallel()
	b := declaredCheckBed(t, "proof.cheap=exit 7\nproof.audits=true\nproof.deadline=15\n")
	starter := &declaredCheckStarter{bed: b, t: t}
	var builderOriginal []byte
	starter.beforeCheck = func(execution launch.Record) func() {
		if execution.Kind == "proof" && execution.Round == 1 {
			file := filepath.Join(resultData(t, starter.results[0])["execution"].(string), "result.json")
			var err error
			builderOriginal, err = os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("changed after builder collection\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return func() {}
	}
	b.manager.Supervisor = starter
	code, built, _ := b.work("work", "build", b.id, "custody", "--brief", b.brief("custody.md", "Build.\n"), "--lines", "100")
	if code != 1 || resultData(t, built)["outcome"] != "proof-red" || len(starter.results) != 2 {
		t.Fatalf("first round: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	round := filepath.Dir(resultData(t, built)["plan"].(string))
	proof := filepath.Base(resultData(t, starter.results[1])["execution"].(string))
	proofFile := filepath.Join(round, proof, "result.json")
	original, err := os.ReadFile(proofFile)
	if err != nil {
		t.Fatal(err)
	}
	builder := filepath.Base(resultData(t, starter.results[0])["execution"].(string))
	builderFile := filepath.Join(round, "builder-"+builder, "result.json")
	collected, err := os.ReadFile(builderFile)
	if err != nil || !bytes.Equal(builderOriginal, collected) {
		t.Fatalf("same-round collection replaced builder evidence: %s %v", collected, err)
	}
	code, resumed, _ := b.work("work", "build", "run:"+run)
	if code != 1 || resultData(t, resumed)["outcome"] != "proof-red" || len(starter.results) != 2 {
		t.Fatalf("same-round resume: %d %+v", code, resumed)
	}
	collected, err = os.ReadFile(builderFile)
	if err != nil || !bytes.Equal(builderOriginal, collected) {
		t.Fatalf("resumed same-round collection replaced builder evidence: %s %v", collected, err)
	}
	starter.beforeBuild = func() {
		for _, name := range []string{proof, builder, "check-later-forgery"} {
			file := filepath.Join(b.worktree, "artifacts", "unit-checks", run, "round-1", name, "result.json")
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("replaced by the next builder\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	code, revised, _ := stopPublic(t, b, "broken-policy", "work", "revise", "run:"+run, "--brief", b.brief("custody-next.md", "Repair.\n"), "--reason", "Repair the failed check", "--by", "Wido")
	if code != 0 || resultData(t, revised)["outcome"] != "green" {
		t.Fatalf("second round: %d %+v", code, revised)
	}
	retained, err := os.ReadFile(proofFile)
	if err != nil || !bytes.Equal(original, retained) {
		t.Fatalf("later builder replaced runner evidence: %s %v", retained, err)
	}
	retained, err = os.ReadFile(builderFile)
	if err != nil || len(builderOriginal) == 0 || !bytes.Equal(builderOriginal, retained) {
		t.Fatalf("later collection replaced builder evidence: %s %v", retained, err)
	}
	for _, name := range []string{proof, "check-later-forgery"} {
		if _, err := os.Stat(filepath.Join(round, "builder-"+name, "result.json")); !os.IsNotExist(err) {
			t.Fatalf("later builder's old-round record was collected: %s %v", name, err)
		}
	}
}

func TestIntentDeclaredCheckFindsRetainedRunFromBaseline(t *testing.T) {
	t.Parallel()
	bed := declaredCheckBed(t, "proof.cheap=pwd\nproof.audits=true\nproof.deadline=15\n")
	code, built, _ := bed.work("work", "build", bed.id, "baseline-lookup", "--brief", bed.brief("baseline.md", "Build the unit.\n"), "--lines", "5")
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil {
		t.Fatal(err)
	}
	command := plan.Proof[0]
	if index := slices.Index(command.Argv, "--repo"); index < 0 || index+1 >= len(command.Argv) || command.Argv[index+1] != bed.root() {
		t.Fatalf("proof does not retain its run store: %v", command.Argv)
	}
	baseline := t.TempDir()
	code, checked := declaredCheckAt(t, bed, baseline, "proof", command.Argv[1:]...)
	if code != 0 {
		t.Fatalf("check in baseline: %d %+v", code, checked)
	}
	exits := resultData(t, checked)["exits"].([]any)
	if len(exits) != 2 || exits[0].(map[string]any)["exit"] != float64(0) || exits[1].(map[string]any)["exit"] != float64(0) ||
		strings.TrimSpace(exits[0].(map[string]any)["output"].(string)) != realpath.ResolveExisting(baseline) {
		t.Fatalf("frozen checks did not execute in the baseline: %+v", checked)
	}
}

func TestIntentDeclaredCheckEmitsFailureOutputForAttribution(t *testing.T) {
	t.Parallel()
	bed := declaredCheckBed(t, "proof.cheap=printf 'FAIL: TestBroken\\n'; exit 7\nproof.audits=printf 'audit-ran\\n'\nproof.deadline=15\n")
	code, built, _ := bed.work("work", "build", bed.id, "failure-output", "--brief", bed.brief("failure.md", "Build the unit.\n"), "--lines", "5")
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	owners := bed.workOwners()
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		if root == bed.worktree && slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			return []byte(bed.worktree), nil
		}
		return git(root, args...)
	}
	command, rest, ok := resolveIntentArgv([]string{"test", "run", "--unit-run", resultData(t, built)["run"].(string), "--repo", bed.root(), "--json"})
	if !ok {
		t.Fatal("public unit check is unavailable")
	}
	var stdout, stderr bytes.Buffer
	code = runIntentIn(command, rest, &stdout, &stderr, bed.worktree, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 1 || result.Outcome != intentFailed || !strings.Contains(stderr.String(), "FAIL: TestBroken\n") || !strings.Contains(stderr.String(), "audit-ran\n") {
		t.Fatalf("check failure output cannot be attributed: %d %+v stderr=%q", code, result, stderr.String())
	}
	exits := resultData(t, result)["exits"].([]any)
	if len(exits) != 2 || exits[0].(map[string]any)["exit"] != float64(7) || exits[1].(map[string]any)["exit"] != float64(0) {
		t.Fatalf("command exits were lost: %+v", result)
	}
}
