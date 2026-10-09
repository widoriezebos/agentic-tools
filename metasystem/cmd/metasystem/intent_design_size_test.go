package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

func sizePage(t *testing.T, bed *workBed, name, status, table string) string {
	t.Helper()
	path := filepath.Join(bed.stateRoot(), "plans", "designs", name+".md")
	data := "# " + name + "\n\n- Kind: design\n- Id: " + name + "\n- Status: " + status + "\n- Goals: " + bed.id + "\n- Critique: closed at round 1 on 0 material findings (reader)\n\n" + table + "\n"
	// Each sized unit has an exact Decision (plans/designs/briefs-carry-their-rules.md:51).
	declared := map[string]bool{}
	for _, unit := range launch.ParseUnitSizes(table) {
		if !declared[unit.Name] {
			data += "\n## " + unit.Name + " — Size\n\nBuild the admitted unit.\n"
			declared[unit.Name] = true
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func sizeTable(prefix string, count int, total, production string) string {
	table := "| Unit | Lines | Production lines |\n|---|---|---|\n"
	for i := 0; i < count; i++ {
		table += fmt.Sprintf("| %s%d | %s | %s |\n", prefix, i, total, production)
	}
	return table
}

func sizeBed(t *testing.T, tiers ...uint8) *workBed {
	t.Helper()
	tier := uint8(2)
	if len(tiers) > 0 {
		tier = tiers[0]
	}
	bed := newDesignGateBed(t, tier)
	setDesignGateMode(t, bed, "refuse")
	bed.lineage = "builder"
	bed.workOwnersHook = func(o *intentWorkOwners) {
		fallback := o.git
		o.git = func(root string, args ...string) ([]byte, error) {
			if args[0] == "log" {
				return nil, nil
			} // No historical declaration in this new repository.
			return fallback(root, args...)
		}
	}
	return bed
}

func sizeBuild(t *testing.T, bed *workBed, unit string) (int, intentResult, string) {
	t.Helper()
	brief := bed.brief("size-brief.md", "Build the admitted unit.\n")
	return bed.work(append([]string{"work", "build", bed.id, unit, "--brief", brief}, workCheck...)...)
}

func TestBuildAndLandRespectDesignSize(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"six across pages", "300 production", "five at 250", "900 total 200 production", "combined legacy", "reordered", "reader table", "missing production", "negative production", "ambiguous production", "conflicting production", "duplicate unit", "second sized table", "done ignored", "superseded ignored", "draft counted", "warn", "person", "allowed", "history unavailable", "historical accepted", "historical missing", "tier one", "duplicate production columns"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := sizeBed(t)
			table := sizeTable("u", 1, "900", "200")
			refused, verdict := false, "ok"
			switch scenario {
			case "six across pages":
				table = sizeTable("u", 3, "40", "40")
				sizePage(t, bed, "other", "accepted", sizeTable("v", 3, "40", "40"))
				refused = true
				verdict = "design-goal-too-large"
			case "300 production", "warn", "person", "allowed":
				table = sizeTable("u", 1, "900", "300")
				refused = true
				verdict = "design-size-invalid"
			case "five at 250":
				table = sizeTable("u", 5, "900", "250")
			case "combined legacy":
				table = "| Unit | Alloc |\n|---|---|\n| u0 | 900 (200) |\n"
			case "reordered":
				table = "|  PRODUCTION   LINES | LiNeS | UNIT |\n|---|---|---|\n| 200 | 900 | u0 |\n"
			case "reader table":
				table = "| Unit | Reader |\n|---|---|\n| example | reader |\n\n" + table
			case "missing production":
				table = sizeTable("u", 1, "40", "")
				refused = true
				verdict = "design-size-invalid"
			case "negative production":
				table = sizeTable("u", 1, "40", "-20")
				refused = true
				verdict = "design-size-invalid"
			case "ambiguous production":
				table = sizeTable("u", 1, "40", "20-30")
				refused = true
				verdict = "design-size-invalid"
			case "conflicting production":
				table = sizeTable("u", 1, "40 (10)", "20")
				refused = true
				verdict = "design-size-invalid"
			case "duplicate production columns":
				table = "| Unit | Lines | Production lines | Production lines |\n|---|---|---|---|\n| u0 | 900 | 300 | 20 |\n"
				refused = true
				verdict = "design-size-invalid"
			case "duplicate unit":
				table += "\n" + sizeTable("u", 1, "40", "20")
				refused = true
				verdict = "design-size-invalid"
			case "second sized table":
				table += "\n" + sizeTable("v", 1, "400", "300")
				refused = true
				verdict = "design-size-invalid"
			case "done ignored":
				sizePage(t, bed, "old", "done", sizeTable("v", 6, "900", "900"))
			case "superseded ignored":
				sizePage(t, bed, "old", "superseded", sizeTable("v", 6, "900", "900"))
			case "draft counted":
				sizePage(t, bed, "candidate", "draft", sizeTable("v", 5, "40", "40"))
				refused = true
				verdict = "design-goal-too-large"
			case "history unavailable":
				bed.workOwnersHook = func(o *intentWorkOwners) {
					fallback := o.git
					o.git = func(root string, args ...string) ([]byte, error) {
						if args[0] == "log" {
							return nil, errors.New("history unavailable")
						}
						return fallback(root, args...)
					}
				}
				verdict = "unchecked"
			case "historical accepted", "historical missing":
				if scenario == "historical accepted" {
					table = sizeTable("u", 6, "900", "900")
				} else {
					table = sizeTable("u", 1, "40", "")
				}
				bed.workOwnersHook = func(o *intentWorkOwners) {
					fallback := o.git
					o.git = func(root string, args ...string) ([]byte, error) {
						if args[0] == "log" {
							return []byte("declaration\n"), nil
						}
						if args[0] == "show" && strings.HasSuffix(args[1], ":plans/designs/main.md") {
							data, err := os.ReadFile(filepath.Join(bed.stateRoot(), "plans/designs/main.md"))
							return data, err
						}
						return fallback(root, args...)
					}
				}
			case "tier one":
				bed = sizeBed(t, 1)
				verdict = "not-design-bearing"
			}
			path := sizePage(t, bed, "main", "accepted", table)
			if scenario == "warn" {
				p := filepath.Join(bed.stateRoot(), "metasystem.conf")
				data, _ := os.ReadFile(p)
				os.WriteFile(p, []byte(strings.ReplaceAll(string(data), "design.gate.mode=refuse", "design.gate.mode=warn")), 0600)
				refused = false
			}
			if scenario == "person" {
				bed.lineage = ""
				bed.manager.Supervisor = sizeImpactStarter{workStarter: bed.starter, bed: bed, t: t}
				refused = false
			}
			if scenario == "allowed" {
				bed.lineage = ""
				code, result := bed.runJSON(bed.terminalOwners(), "goal", "allow", bed.id, goal.PermissionBuildWithoutDesign, "--reason", "Use retained design", "--fixture-human-authority")
				if code != 0 {
					t.Fatalf("appeal: %d %+v", code, result)
				}
				bed.lineage = "builder"
				refused = false
				verdict = "allowed"
			}
			before, _ := os.ReadFile(path)
			code, result, output := sizeBuild(t, bed, "u0")
			gate := resultData(t, result)["designGate"].(map[string]any)
			if gate["verdict"] != verdict {
				t.Fatalf("verdict=%v want=%s code=%d result=%+v output=%s", gate, verdict, code, result, output)
			}
			if refused {
				if code != 1 || len(bed.starter.launched()) != 0 {
					t.Fatalf("refusal launched: %d %+v %v", code, result, bed.starter.launched())
				}
				if result.Next == nil || !strings.Contains(result.Next.Reason, "draft") || strings.Contains(result.Next.Reason, "goal split propose") {
					t.Fatalf("remedy: %+v", result)
				}
				if scenario == "six across pages" && !strings.Contains(result.Next.Reason, "goal open NEW-GOAL") {
					t.Fatal("missing person's opening act")
				}
				if scenario == "six across pages" {
					bed.lineage = ""
					code, opened := bed.runJSON(bed.terminalOwners(), "goal", "open", "split-size", "--intent", "Preserve split-off requirements from "+bed.id, "--next", "Review the preserved draft", "--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "Separated local tooling", "--fixture-human-authority")
					if code != 0 || bed.goalFile("split-size").State != goal.StateQueued {
						t.Fatalf("printed person remedy failed: %d %+v", code, opened)
					}
					split := filepath.Join(bed.stateRoot(), "plans/designs/other.md")
					original, err := os.ReadFile(split)
					if err != nil {
						t.Fatal(err)
					}
					draft := strings.ReplaceAll(strings.ReplaceAll(string(original), "- Goals: "+bed.id, "- Goals: split-size"), "- Status: accepted", "- Status: draft")
					if err := os.WriteFile(split, []byte(draft), 0600); err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(draft, sizeTable("v", 3, "40", "40")) {
						t.Fatal("split requirements lost")
					}
					bed.lineage = "builder"
					code, retry, _ := sizeBuild(t, bed, "u0")
					if code != 0 || resultData(t, retry)["designGate"].(map[string]any)["verdict"] != "ok" {
						t.Fatalf("repair did not permit retry: %d %+v", code, retry)
					}
				}

				return
			}
			if code != 0 || len(bed.starter.launched()) == 0 {
				t.Fatalf("build: %d %+v %s", code, result, output)
			}
			identity, _ := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
			_, record := designGateRead(t, bed, identity, "u0")
			if scenario == "person" && (record.PersonImpact == "" || !strings.Contains(output, "impact:") || !record.Person) {
				t.Fatalf("person impact: %+v %s", record, output)
			}
			if verdict == "ok" || scenario == "person" || scenario == "warn" {
				if record.Designs[0].Size == nil || record.Designs[0].Size.Declaration == "" || len(record.Designs[0].Size.Designs) == 0 {
					t.Fatalf("missing size evidence: %+v", record)
				}
				// Landing's real fact reader reuses the build observation. Changed settings cannot re-size committed work.
				conf := filepath.Join(bed.stateRoot(), "metasystem.conf")
				data, _ := os.ReadFile(conf)
				os.WriteFile(conf, append(data, []byte("\ndesign.unit-lines-max=1\ndesign.goal-units-max=1\n")...), 0600)
				var stderr bytes.Buffer
				inv := landingGateInvocation(t, bed, &stderr)
				facts := inv.landingDesignFacts(bed.stateRoot(), bed.id)
				land := landing.ObserveDesign(facts, false)
				wantRefusal := scenario == "person" // warn-mode build keeps warning mode at landing.
				if land.RefusesAgent != wantRefusal {
					t.Fatalf("landing reused size wrongly: %+v recorded=%+v", land, record.Designs[0].Size)
				}
				human := landing.ObserveDesign(facts, true)
				pushCode, pushes := sizeLandingPush(t, bed)
				if (pushCode == 1) != wantRefusal || (pushes == 0) != wantRefusal {
					t.Fatalf("public lane push: code=%d pushes=%d wantRefusal=%t", pushCode, pushes, wantRefusal)
				}
				if human.RefusesAgent {
					t.Fatalf("person landing refused: %+v", human)
				}
				if wantRefusal && strings.Contains(land.Pair[1], "draft") {
					t.Fatalf("landing asks to edit committed units: %+v", land)
				}
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("admission rewrote acceptance")
			}
		})
	}
}

func TestDesignSizeSettings(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"250", "0", "-1", "no", "9223372036854775808"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			owners := bed.workOwners()
			owners.work.config = configSettingWithDefault
			root := bed.root()
			conf := filepath.Join(root, "metasystem.conf")
			if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude\nproof.full=true\nproof.cheap=true\ndesign.unit-lines-max="+value+"\ndesign.goal-units-max="+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{".claude/agents", ".claude/skills"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Validate the changed keys through the public command, not a parser-only test.
			var out, errout bytes.Buffer
			code := runIntentIn(mustIntentCommand(t, "settings check"), nil, &out, &errout, root, owners)
			text := out.String() + errout.String()
			if value != "250" && (code == 0 || !strings.Contains(text, "design.unit-lines-max must be a positive integer") || !strings.Contains(text, "design.goal-units-max must be a positive integer")) {
				t.Fatalf("settings check=%d %s", code, text)
			}
			if value == "250" {
				for _, key := range []string{"design.unit-lines-max", "design.goal-units-max"} {
					out.Reset()
					errout.Reset()
					code = runIntentIn(mustIntentCommand(t, "settings show"), []string{key}, &out, &errout, root, owners)
					if code != 0 || !strings.Contains(out.String(), key+" is 250") || !strings.Contains(out.String(), "conf") {
						t.Fatalf("settings show=%d %s %s", code, &out, &errout)
					}
					// The same committed resolution ignores flags, environment and a synthetic local layer.
					synthetic := filepath.Join(t.TempDir(), "size-settings.conf")
					os.WriteFile(synthetic, []byte(key+"=250\n"), 0600)
					os.WriteFile(synthetic+".local", []byte(key+"=9999\n"), 0600)
					got, code, err := config.Get(config.GetParams{Key: key, ConfPath: synthetic, Flag: "9999", LookupEnv: func(string) (string, bool) { return "9999", true }})
					if err != nil || code != 0 || got != "250" {
						t.Fatalf("overlay raised committed cap: %s %d %v", got, code, err)
					}
				}
			}
		})
	}
}

func TestDesignSizeFreshDispatch(t *testing.T) {
	t.Parallel()
	for _, movement := range []string{"bytes", "design set", "declaration"} {
		t.Run(movement, func(t *testing.T) {
			t.Parallel()
			bed := sizeBed(t)
			path := sizePage(t, bed, "main", "accepted", sizeTable("u", 1, "900", "200"))
			initialRead := false
			bed.workOwnersHook = func(o *intentWorkOwners) {
				fallback := o.git
				o.git = func(root string, args ...string) ([]byte, error) {
					if args[0] == "log" {
						initialRead = true
						return nil, nil
					}
					if args[0] == "worktree" && initialRead {
						initialRead = false
						switch movement {
						case "bytes":
							sizePage(t, bed, "main", "accepted", sizeTable("u", 1, "900", "300"))
						case "design set":
							sizePage(t, bed, "second", "draft", sizeTable("v", 5, "40", "40"))
						case "declaration":
							conf := filepath.Join(bed.stateRoot(), "metasystem.conf")
							data, err := os.ReadFile(conf)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(conf, append(data, []byte("\ndesign.unit-lines-max=100\n")...), 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					return fallback(root, args...)
				}
			}
			code, result, output := sizeBuild(t, bed, "u0")
			if code == 0 || len(bed.starter.launched()) != 0 || !strings.Contains(result.Summary, "warning:") {
				t.Fatalf("moved input inherited a pass: %d %+v launches=%v output=%s path=%s", code, result, bed.starter.launched(), output, path)
			}
		})
	}
}

func TestDesignSizeFreshPersonImpactBeforeLaunch(t *testing.T) {
	t.Parallel()
	bed := sizeBed(t)
	bed.lineage = ""
	sizePage(t, bed, "main", "accepted", sizeTable("u", 1, "900", "200"))
	initialRead := false
	bed.workOwnersHook = func(o *intentWorkOwners) {
		fallback := o.git
		o.git = func(root string, args ...string) ([]byte, error) {
			if args[0] == "show" && strings.HasSuffix(args[1], ":plans/designs/main.md") {
				return os.ReadFile(filepath.Join(bed.stateRoot(), "plans/designs/main.md"))
			}
			if args[0] == "log" {
				initialRead = true
				return nil, nil
			}
			if args[0] == "worktree" && initialRead {
				initialRead = false
				sizePage(t, bed, "main", "accepted", sizeTable("u", 1, "900", "300"))
			}
			return fallback(root, args...)
		}
	}
	var stdout, stderr bytes.Buffer
	bed.manager.Supervisor = sizeImpactStarter{workStarter: bed.starter, bed: bed, t: t, output: &stderr}
	brief := bed.brief("size-brief.md", "Build the admitted unit.\n")
	args := append([]string{"work", "build", bed.id, "u0", "--brief", brief}, workCheck...)
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatal("the public build command is unavailable")
	}
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, bed.root(), bed.workOwners())
	if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
		t.Fatalf("person build did not launch: code=%d out=%s stderr=%s", code, &stdout, &stderr)
	}
}

// Run the public lane push handler with only publication, containment and notification replaced.
func sizeLandingPush(t *testing.T, bed *workBed) (int, int) {
	t.Helper()
	var out, stderr bytes.Buffer
	inv := landingGateInvocation(t, bed, &stderr)
	inv.command = landingPushCommand()
	inv.input = intentInput{values: map[string][]string{"json": {"true"}}}
	inv.stdout = &out
	install := inv.layout.InstallationRoot.Path()
	if _, _, err := plain.HandIn(install, plain.Line{Goal: bed.id, SHA: "hand-in", At: laneTestNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	record := lane.Record{Root: inv.layout.GitRoot, Install: install}
	layout, err := record.Layout()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if _, _, err := lane.Register(home, layout, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	record, _, err = lane.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	pushes := 0
	admitted := laneAdmitted{home: home, record: record, layout: layout, installation: install, owners: laneVerbOwners{
		machine: func(string) (string, error) { return "landing", nil },
		now:     func() time.Time { return laneTestNow },
		push: func(_ string, _ string, _ time.Time, before func(string, string) error) (plain.PushOutcome, error) {
			result := plain.PushOutcome{Old: "main", Commit: "head"}
			if err := before(result.Old, result.Commit); err != nil {
				return result, err
			}
			pushes++
			result.Changed = true
			return result, nil
		},
		plainProve: plain.ProveSeams{Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }, Git: func(_ string, args ...string) (string, error) {
			if args[0] == "ls-tree" {
				return "", nil
			}
			if slices.Equal(args, []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}"}) {
				return "main", nil
			}
			if slices.Equal(args, []string{"cat-file", "-e", "main^{commit}"}) || slices.Equal(args, []string{"cat-file", "-e", "hand-in^{commit}"}) {
				return "", nil
			}
			if slices.Equal(args, []string{"merge-base", "--is-ancestor", "hand-in", "main"}) {
				return "", exec.Command("false").Run()
			}
			t.Fatalf("unexpected Git: %v", args)
			return "", nil
		}},
	}}
	effects := landingPushOwners{
		contains: func(_, ref string) func(string) (bool, error) {
			return func(sha string) (bool, error) { return ref == "head" && sha == "hand-in", nil }
		},
		facts:  func(id string) landing.DesignFacts { return inv.landingDesignFacts(install, id) },
		notify: func(plain.PushOutcome) error { return nil },
	}
	code := runIntentLandingPushWithOwners(inv, admitted, effects)
	if code != 0 {
		t.Logf("lane push refused: out=%s stderr=%s", &out, &stderr)
	}
	if code != 0 && code != 1 {
		t.Fatalf("lane push exit=%d out=%s stderr=%s", code, &out, &stderr)
	}
	return code, pushes
}

type sizeImpactStarter struct {
	*workStarter
	bed    *workBed
	t      *testing.T
	output *bytes.Buffer
}

func (s sizeImpactStarter) StartSupervisor(id, owner string) (identity.Ref, error) {
	record, err := s.m.Store.Read(id)
	if err != nil {
		s.t.Fatal(err)
	}
	if record.Kind == "build" {
		if s.output != nil && (!strings.Contains(s.output.String(), "impact:") || !strings.Contains(s.output.String(), "undo")) {
			s.t.Fatalf("build effect began before the person saw the fresh impact: %s", s.output)
		}
		identity, _ := s.bed.designGate.identity(stateroottest.Installation(s.t, s.bed.stateRoot()))
		_, gate := designGateRead(s.t, s.bed, identity, "u0")
		if !gate.Person || !strings.Contains(gate.PersonImpact, "impact:") || !strings.Contains(gate.PersonImpact, "undo") {
			s.t.Fatalf("build effect began before retained person impact: %+v", gate)
		}
	}
	return s.workStarter.StartSupervisor(id, owner)
}
