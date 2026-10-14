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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestCarryAndFullSuitePublicAdmission(t *testing.T) {
	t.Parallel()
	t.Run("audits-equals-full", func(t *testing.T) {
		t.Parallel()
		t.Run("check-admission", func(t *testing.T) {
			t.Parallel()
			c := processchange.Check{Root: t.TempDir(), Installation: roots.Installation(t.TempDir()),
				ProcessAct: processchange.ProcessAct{Goal: "goal", Unit: "unit", Operation: "unit-check", Checkout: "checkout", After: "true", AfterArgv: []string{"true"}},
				Audits:     "printf full", FullArgv: []string{"/bin/sh", "-c", "printf full"}, Observation: true, Remedy: func(string) string { return "person check" }}
			if _, err := processchange.AdmitCheck(c); err != nil {
				t.Fatal(err)
			}
			c.Observation = false
			act, err := processchange.AdmitCheck(c)
			if err != nil || act.Class != "full-suite-exception" || act.Status != "proposed" || !slices.Equal(act.SelectedArgv, c.FullArgv) {
				t.Fatalf("unchanged check's audits escaped admission: %+v %v", act, err)
			}
			c.Person, c.Act = true, act.ID
			if admitted, err := processchange.AdmitCheck(c); err != nil || admitted.ID != act.ID || admitted.Status != "applied" {
				t.Fatalf("person check exception: %+v %v", admitted, err)
			}
			c.Person, c.Act, c.Operation = false, "", "next-unit-check"
			if next, err := processchange.AdmitCheck(c); err != nil || next.Status != "proposed" || next.Class != "full-suite-exception" || next.ID == act.ID {
				t.Fatalf("another admission reused the audits exception: %+v %v", next, err)
			}
		})
		for _, route := range []string{"build", "check", "carry"} {
			t.Run(route, func(t *testing.T) {
				t.Parallel()
				b := newWorkBed(t)
				marker := filepath.Join(t.TempDir(), "audit-started")
				audits := "printf audited > " + shellCommand([]string{marker})
				declaration := "proof.cheap=true\nproof.audits=" + audits + "\nproof.deadline=15\nproof.full=" + audits + "\n"
				b.head = strings.Repeat("c", 40)
				subject := branch.AttestationSubject{Commit: b.head, Tree: strings.Repeat("d", 40), Parent: "base-commit"}
				b.workOwnersHook = func(work *intentWorkOwners) {
					original := work.git
					work.git = func(root string, args ...string) ([]byte, error) {
						switch {
						case len(args) == 4 && args[0] == "show" && args[1] == "-s":
							return []byte("Goal-Unit: " + b.id + "/u1"), nil
						case len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
							return []byte(declaration), nil
						case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
							return []byte(subject.Tree), nil
						case args[0] == "status":
							return nil, nil
						}
						return original(root, args...)
					}
				}
				owners := b.workOwners()
				owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
				brief := b.brief("audit-full.md", "Build.\n")
				args := []string{"work", "build", b.id, "--work", "audit-full", "--brief", brief, "--lines", "5"}
				if route == "check" {
					args = append(args, "--check", "true")
				} else if route == "carry" {
					args = []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--check-only"}
				}
				code, held := checkActBuild(t, b, owners, args...)
				act := processAct(t, held)
				if route == "carry" {
					acts := checkActs(t, b)
					if len(acts) != 1 {
						t.Fatalf("carry exception acts: %+v", acts)
					}
					act = acts[0]
				}
				if code != 1 || act.Class != "full-suite-exception" || act.Status != "proposed" || act.BeforeDeclaration.Values != act.AfterDeclaration.Values || !slices.Equal(act.SelectedArgv, []string{"/bin/sh", "-c", audits}) || !slices.Equal(act.FullArgv, act.SelectedArgv) || len(b.starter.launched()) != 0 {
					t.Fatalf("unchanged full audits escaped admission: exit=%d summary=%s class=%s status=%s", code, held.Summary, act.Class, act.Status)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("audit executed before exception: %v", err)
				}
				q, err := channel.ReadQuestion(b.root(), act.Question)
				remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", act.ID}
				if route != "carry" {
					if held.Next == nil {
						t.Fatal("audit exception has no public remedy")
					}
					remedy = held.Next.Argv[1:]
				}
				if err != nil || q.State != "open" || q.Wants != shellCommand(append([]string{"metasystem"}, remedy...)) {
					t.Fatalf("audit exception question: %+v %v next=%+v", q, err, held.Next)
				}
				if code, result := checkActBuild(t, b, owners, remedy...); code != 1 || len(b.starter.launched()) != 0 {
					t.Fatalf("agent admitted full audits: %d %+v", code, result)
				}
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				code, admitted := checkActBuild(t, b, owners, remedy...)
				if code != 0 {
					t.Fatalf("person audit exception: %d %+v", code, admitted)
				}
				if route == "carry" {
					data, err := os.ReadFile(marker)
					if err != nil || string(data) != "audited" {
						t.Fatalf("admitted carry did not run audits: %q %v", data, err)
					}
				} else {
					checkActLaunches(t, b, admitted, act.ID)
					plan, err := launch.ReadUnitPlan(resultData(t, admitted)["plan"].(string))
					if err != nil || plan.Check.Audits != audits {
						t.Fatalf("admitted plan lost frozen audits: %+v %v", plan.Check, err)
					}
				}
				q, err = channel.ReadQuestion(b.root(), act.Question)
				if err != nil || q.State != "closed" {
					t.Fatalf("audit exception closure: %+v %v", q, err)
				}
			})
		}
	})
	t.Run("build-revise", func(t *testing.T) {
		t.Parallel()
		b := declaredCheckBed(t, "proof.cheap=printf full\nproof.audits=true\nproof.deadline=15\n")
		old := b.workOwnersHook
		full := "printf full"
		b.workOwnersHook = func(w *intentWorkOwners) {
			old(w)
			git := w.git
			w.git = func(root string, args ...string) ([]byte, error) {
				data, err := git(root, args...)
				if err == nil && args[0] == "show" {
					data = []byte(strings.ReplaceAll(string(data), "proof.full=true # full", "proof.full="+full))
				}
				return data, err
			}
		}
		owners := b.workOwners()
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		brief := b.brief("full.md", "Build.\n")
		args := []string{"work", "build", b.id, "--work", "full", "--brief", brief, "--lines", "5"}
		code, held := checkActBuild(t, b, owners, args...)
		act := processAct(t, held)
		if code != 1 || act.Class != "full-suite-exception" || act.BeforeDeclaration.Values != act.AfterDeclaration.Values || len(act.FullArgv) != 3 || !slices.Equal(act.FullArgv, act.SelectedArgv) || len(b.starter.launched()) != 0 {
			t.Fatalf("unchanged full selection launched: %d %+v", code, held)
		}
		owners.prove = fixedFixtureGoalAuthority
		if code, result := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 1 || len(b.starter.launched()) != 0 {
			t.Fatalf("agent exception: %d %+v", code, result)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		code, built := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
		if code != 0 {
			t.Fatalf("full remedy: %d %+v", code, built)
		}
		checkActLaunches(t, b, built, act.ID)
		count := len(b.starter.launched())
		if code, replay := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 0 || len(b.starter.launched()) != count {
			t.Fatalf("full replay: %d %+v", code, replay)
		}
		q, err := channel.ReadQuestion(b.root(), act.Question)
		if err != nil || q.State != "closed" {
			t.Fatalf("exception closure: %+v %v", q, err)
		}
		checkNextWorktree(t, b)
		owners = b.workOwners()
		other := slices.Clone(args)
		other[4] = "other"
		code, otherHeld := checkActBuild(t, b, owners, other...)
		if code != 1 || processAct(t, otherHeld).ID == act.ID || len(b.starter.launched()) != count {
			t.Fatalf("another unit reused exception: %d %+v", code, otherHeld)
		}
		wrong := append(slices.Clone(other), "--act", act.ID)
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		beforeWrong := len(b.starter.launched())
		if code, wrongResult := checkActBuild(t, b, owners, wrong...); code != 1 || len(b.starter.launched()) != beforeWrong {
			t.Fatalf("wrong subject: %d %+v", code, wrongResult)
		}
		// A person's explicit command is the exception, without a second approval.
		checkNextWorktree(t, b)
		manual := []string{"work", "build", b.id, "--work", "manual", "--brief", brief, "--lines", "5", "--reason", "Run full suite once", "--by", "Wido", "--check", "printf", "full"}
		code, manualResult := checkActBuild(t, b, owners, manual...)
		if code != 0 {
			t.Fatalf("direct exception: %d %+v", code, manualResult)
		}
		plan, err := launch.ReadUnitPlan(resultData(t, manualResult)["plan"].(string))
		if err != nil || plan.Check.ProcessAct == "" {
			t.Fatalf("manual consumed act: %+v %v", plan, err)
		}
		acts := checkActs(t, b)
		if !slices.ContainsFunc(acts, func(a processchange.ProcessAct) bool {
			return a.ID == plan.Check.ProcessAct && a.Class == "full-suite-exception" && a.Status == "applied"
		}) {
			t.Fatalf("manual exception absent: %+v", acts)
		}
		// Distinct ordered shell arguments remain an ordinary declaration.
		checkNextWorktree(t, b)
		full = "printf full-different"
		owners = b.workOwners()
		code, distinct := checkActBuild(t, b, owners, "work", "build", b.id, "--work", "distinct", "--brief", brief, "--lines", "5")
		if code != 0 {
			t.Fatalf("name classified full: %d %+v", code, distinct)
		}
		// The next round needs its own exception despite an unchanged tuple.
		record, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, built)["run"].(string))
		b.worktree = record.Worktree
		run := resultData(t, built)["run"].(string)
		full = "printf full"
		owners = b.workOwners()
		code, revised := checkActBuild(t, b, owners, "work", "revise", "run:"+run, "--brief", b.brief("next.md", "Repair.\n"), "--reason", "Next round", "--by", "Wido")
		if code != 1 || processAct(t, revised).Class != "full-suite-exception" {
			t.Fatalf("revision full selection was not held: %d %+v", code, revised)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		code, revised = checkActBuild(t, b, owners, revised.Next.Argv[1:]...)
		if code != 0 {
			t.Fatalf("revision remedy: %d %+v", code, revised)
		}
		next, err := launch.ReadUnitPlan(resultData(t, revised)["plan"].(string))
		if err != nil || next.Check.ProcessAct == act.ID || next.Check.ProcessAct == "" {
			t.Fatalf("revision reused exception: %+v %v", next, err)
		}
	})
	t.Run("carry-subject", func(t *testing.T) {
		t.Parallel()
		b, owners, _ := rebaseIntentBed(t)
		initialGoal := b.id
		script := filepath.Join(t.TempDir(), "check")
		if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf 'subject checked'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		mainContent := "proof.cheap=printf seed\nproof.audits=true\nproof.deadline=15\nproof.full=printf full\n"
		cheap, full, unavailable := script, script, false
		content := func() string {
			return "proof.cheap=" + cheap + "\nproof.audits=true\nproof.deadline=15\nproof.full=" + full + "\n"
		}
		subject := branch.AttestationSubject{Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), Parent: "seed"}
		owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
		original := owners.work.git
		owners.work.git = func(root string, args ...string) ([]byte, error) {
			switch {
			case len(args) == 4 && args[0] == "show" && args[1] == "-s":
				id := initialGoal
				if args[3] == strings.Repeat("b", 40) {
					id = "carry-b"
				}
				return []byte("Goal-Unit: " + id + "/u1"), nil
			case slices.Equal(args, []string{"rev-parse", "HEAD"}):
				return []byte(subject.Commit), nil
			case slices.Equal(args, []string{"rev-parse", "origin/main"}):
				return []byte("seed"), nil
			case args[0] == "merge-base":
				return []byte("seed"), nil
			case args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
				return []byte(subject.Tree), nil
			case args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
				if unavailable {
					return nil, fmt.Errorf("committed classification unavailable")
				}
				if strings.HasPrefix(args[1], "seed:") {
					return []byte(mainContent), nil
				}
				return []byte(content()), nil
			case args[0] == "status":
				return nil, nil
			}
			return original(root, args...)
		}
		// The branch boundary supplies the real subject gate and the same
		// declaration-unavailable result that carry retains as NeedsReview.
		owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
			gate, err := branch.ResolveReadGate(branch.ReadGateRequest{Repo: b.worktree, GoalID: b.id, UnitCommit: subject.Commit, SubjectCheck: req.SubjectCheck, Repository: owners.connection.readRepository})
			result := branch.RebaseResult{State: "held", NeedsReview: []string{}, Carried: []string{}}
			if err != nil {
				var unavailable *branch.DeclarationUnavailableError
				if !errors.As(err, &unavailable) {
					return result, err
				}
				result.NeedsReview = []string{"u1"}
				result.ReviewReasons = map[string]string{"u1": err.Error()}
			} else {
				if gate.RunID == "" {
					t.Fatal("no actual execution")
				}
				result.Carried = []string{"u1"}
			}
			return result, nil
		}
		code, held := b.runJSON(owners, "work", "rebase", b.id)
		acts := checkActs(t, b)
		if code != 0 || len(acts) != 1 || acts[0].Status != "proposed" || acts[0].Class != "full-suite-exception" {
			t.Fatalf("carry hold: %d %+v acts=%+v", code, held, acts)
		}
		paths, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "check-*"))
		if len(paths) != 0 {
			t.Fatalf("carry launched before approval: %v", paths)
		}
		q, err := channel.ReadQuestion(b.root(), acts[0].Question)
		if err != nil {
			t.Fatal(err)
		}
		// The retained public remedy fixes this commit, never a mutable unit name.
		remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", acts[0].ID}
		if q.Wants != shellCommand(append([]string{"metasystem"}, remedy...)) {
			t.Fatalf("retained remedy: %q", q.Wants)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 1 {
			t.Fatalf("agent used carry act: %d %+v", code, result)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		if err := os.MkdirAll(acts[0].Operation, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(acts[0].Operation, 0500); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 1 {
			t.Fatalf("interrupted carry publication: %d %+v", code, result)
		}
		q, _ = channel.ReadQuestion(b.root(), acts[0].Question)
		if q.State != "open" {
			t.Fatal("question closed before subject publication")
		}
		if err := os.Chmod(acts[0].Operation, 0700); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
			t.Fatalf("carry remedy: %d %+v", code, result)
		}
		q, err = channel.ReadQuestion(b.root(), acts[0].Question)
		if err != nil || q.State != "closed" {
			t.Fatalf("carry question: %+v %v acts=%+v", q, err, checkActs(t, b))
		}
		gatePath := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "gate.json")
		if err := os.Remove(gatePath); err != nil {
			t.Fatal(err)
		}
		if code, result := b.runJSON(owners, "work", "rebase", b.id); code != 0 {
			t.Fatalf("carry replay: %d %+v", code, result)
		}
		paths, _ = filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "check-*", "result.json"))
		if len(paths) != 1 {
			t.Fatalf("duplicate carry check: %v", paths)
		}
		raw, _ := os.ReadFile(paths[0])
		var executed struct {
			Check launch.UnitCheck
			Exits []launch.CheckExit
		}
		if json.Unmarshal(raw, &executed) != nil || executed.Check.ProcessAct != acts[0].ID || executed.Check.SourceTree != subject.Tree || executed.Exits[0].Output != "subject checked" {
			t.Fatalf("carry consumed identity: %s", raw)
		}
		originalGoal := b.id
		second := b.goalFile(b.id)
		second.Id = "carry-b"
		goalPath := "plans/goals/carry-b.md"
		body := goal.RenderFile(second)
		if err := os.WriteFile(filepath.Join(b.root(), goalPath), body, 0600); err != nil {
			t.Fatal(err)
		}
		for id, commit := range b.repo.commits {
			commit.files[goalPath] = body
			b.repo.commits[id] = commit
		}
		for index, id := range []string{originalGoal, "carry-b", originalGoal, "carry-b"} {
			b.id = id
			cheap, full = script+" "+id, "printf full"
			subject.Commit = strings.Repeat("a", 40)
			if id == "carry-b" {
				subject.Commit = strings.Repeat("b", 40)
			}
			owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
			owners.prove = fixedFixtureGoalAuthority
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			if code != 0 {
				t.Fatalf("alternating carry: %d %+v", code, result)
			}
			current := checkActs(t, b)
			pending := slices.DeleteFunc(slices.Clone(current), func(a processchange.ProcessAct) bool { return a.Status != "proposed" })
			if index < 2 {
				if len(pending) != 1 || pending[0].Class != "declaration" || pending[0].BeforeDeclaration.Values[0] != map[bool]string{true: script, false: "printf seed"}[id == originalGoal] {
					t.Fatalf("carry used another goal's history: %+v", pending)
				}
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", pending[0].ID}
				if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
					t.Fatalf("alternating remedy: %d %+v", code, result)
				}
			} else if len(pending) != 0 {
				t.Fatalf("unchanged alternating carry raised an act: %+v", pending)
			}
		}
		b.id = originalGoal
		subject.Commit = strings.Repeat("c", 40)
		owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
		// A frozen subject resumes even if the advisory goal reference is damaged.
		baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(b.id))))
		if err := os.WriteFile(baseline, []byte("damaged"), 0600); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
			t.Fatalf("damaged carry resume: %d %+v", code, result)
		}
		snapshot := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "subject.json")
		if err := os.WriteFile(snapshot, []byte("damaged snapshot"), 0600); err != nil {
			t.Fatal(err)
		}
		unavailable = true
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		repair := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--reason", "Repair missing classification", "--by", "Wido", "--check", script}
		mismatch := slices.Clone(repair)
		mismatch[slices.Index(mismatch, "--by")+1] = "W. Riezebos"
		code, refused := checkActBuild(t, b, owners, mismatch...)
		assertDeclaredRepairNameRefusal(t, code, refused, mismatch)
		if code, result := checkActBuild(t, b, owners, repair...); code != 0 {
			t.Fatalf("carry explicit repair: %d %+v", code, result)
		}
		if !slices.ContainsFunc(checkActs(t, b), func(a processchange.ProcessAct) bool {
			return a.Measure == "full-suite classification unknown" && a.Status == "applied"
		}) {
			t.Fatal("repair invented a classification")
		}
		preserved, _ := filepath.Glob(snapshot + ".damaged-*")
		if len(preserved) != 1 {
			t.Fatalf("damaged snapshot not preserved: %v", preserved)
		}
		if code, result := checkActBuild(t, b, owners, withoutOption(repair, "by")...); code != 0 {
			t.Fatalf("carry repair without a typed name: %d %+v", code, result)
		}
		data, err := os.ReadFile(snapshot)
		var repaired launch.UnitCheck
		if err != nil || json.Unmarshal(data, &repaired) != nil || repaired.SelectedBy != "Wido" {
			t.Fatalf("carry repair lost the enrolled name: %+v %v", repaired, err)
		}

	})
}

type carryFullRepository struct {
	root, common string
	subject      branch.AttestationSubject
}

func (r carryFullRepository) Range(_, _, _, _ string) ([]branch.Commit, error) {
	return nil, fmt.Errorf("range unavailable")
}
func (r carryFullRepository) Subject(repo, commit string) (branch.AttestationSubject, error) {
	if repo != r.root {
		return branch.AttestationSubject{}, fmt.Errorf("the subject was requested from another checkout")
	}
	if commit != r.subject.Commit {
		return branch.AttestationSubject{}, fmt.Errorf("different subject")
	}
	return r.subject, nil
}
func (r carryFullRepository) CommonDir(_ string) (string, error) { return r.common, nil }
func (r carryFullRepository) Entries(_, _ string) ([]branch.Entry, error) {
	return nil, fmt.Errorf("entries unavailable")
}
func (r carryFullRepository) Detached(_, _ string) (string, func() error, error) {
	return r.root, func() error { return nil }, nil
}

func TestDeclaredResumeAndRepairPublicBuild(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	args := []string{"work", "build", b.id, "--work", "resume", "--brief", b.brief("resume.md", "Build.\n"), "--lines", "5"}
	code, built := checkActBuild(t, b, owners, args...)
	if code != 3 {
		t.Fatalf("incomplete admitted round: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(b.id))))
	if err := os.WriteFile(baseline, []byte("damaged evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	owners = b.workOwners()
	count := len(b.starter.launched())
	if code, resumed := checkActBuild(t, b, owners, "work", "build", "run:"+run); code != 3 || len(b.starter.launched()) != count {
		t.Fatalf("damaged admitted resume: %d %+v", code, resumed)
	}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	repair := []string{"work", "build", "run:" + run, "--reason", "Repair unavailable declarations", "--by", "Wido", "--check", "false"}
	mismatch := slices.Clone(repair)
	mismatch[slices.Index(mismatch, "--by")+1] = "W. Riezebos"
	code, refused := checkActBuild(t, b, owners, mismatch...)
	assertDeclaredRepairNameRefusal(t, code, refused, mismatch)
	if len(b.starter.launched()) != count {
		t.Fatal("a refused repair launched a child")
	}
	code, repaired := checkActBuild(t, b, owners, repair...)
	if code != 3 || len(b.starter.launched()) != count {
		t.Fatalf("person retained repair: %d %+v", code, repaired)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil || plan.Check.Cheap != "false" || plan.Check.Audits != "true" || plan.Check.ProcessAct == "" {
		t.Fatalf("repair didn't replace check: %+v %v", plan.Check, err)
	}
	raw, _ := os.ReadFile(baseline)
	if string(raw) != "damaged evidence" {
		t.Fatal("manual repair fabricated declaration baseline")
	}
	if code, result := checkActBuild(t, b, owners, withoutOption(repair, "by")...); code != 3 || len(b.starter.launched()) != count {
		t.Fatalf("retained repair without a typed name: %d %+v", code, result)
	}
	plan, err = launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil || plan.Check.SelectedBy != "Wido" {
		t.Fatalf("retained repair lost the enrolled name: %+v %v", plan.Check, err)
	}
}
