package launch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func applicableCheckFixture(t *testing.T) (*CheckEvidence, CheckNeed) {
	t.Helper()
	check := UnitCheck{Cheap: "metasystem test impact --check", Audits: "declared audits", Minutes: 15,
		Environment: []string{}, Declaration: &UnitDeclaration{Values: [4]string{"metasystem test impact --check", "declared audits", "15", "full"}, SourceSHA256: "declaration"}}
	current := CheckSelection{Version: 1, Base: "head", BaseTree: "base-tree", Groups: []string{"fast-static-build", "prerequisite", "unit/example"},
		GroupDigests: map[string]string{"fast-static-build": "definition"}, PackageTests: map[string][]string{"example": {"TestExample"}},
		Mode: "impact", ContractDigest: "contract", EngineDigest: "engine", GoVersion: "go fixture", GOOS: "darwin", GOARCH: "arm64",
		EnvironmentSHA256: "environment", BuildTags: map[string][]string{"fast-static-build": {"tag"}},
		Settings: map[string]string{"landing.impact-max-share": "50"}, Workers: 9, VerdictPolicyVersion: 1}
	record := &CheckEvidence{Version: 1, Selection: current, Declaration: check.Declaration, Commands: [2]string{check.Cheap, check.Audits}, Source: "check-source", RunDirectory: ".", CheapDirectory: "."}
	record.Content.Before, record.Content.After = "round-tree", "round-tree"
	record.Strength.Cheap, record.Strength.Audited, record.Strength.Complete = true, true, true
	// Keep the observed record independent of the consumer's live context.
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var retained CheckEvidence
	if err := json.Unmarshal(data, &retained); err != nil {
		t.Fatal(err)
	}
	return &retained, CheckNeed{Tree: "round-tree", Base: "head", Strength: "audited", Check: check, Context: current, Requirements: []string{"fast-static-build", "prerequisite"}}
}

var applicabilityRows = []struct{ name, field string }{
	{"match", ""}, {"tree", "tree"}, {"base", "base"}, {"base tree", "base"},
	{"declaration", "declaration"}, {"declaration digest", "declaration"}, {"commands", "commands"},
	{"engine", "engine"}, {"toolchain", "toolchain"}, {"platform", "platform"}, {"architecture", "platform"}, {"build tags", "platform"},
	{"environment", "environment"}, {"setting", "settings"}, {"workers", "settings"}, {"requirements", "requirements"}, {"closure", "requirements"},
	{"contract", "contract digest"}, {"policy", "contract digest"}, {"audits true", "strength"}, {"manual publication only", "strength"},
	{"publication only", "strength"}, {"declared green audits", ""}, {"legacy", "version (legacy evidence)"}, {"version", "version"},
	{"missing", "missing evidence"}, {"unreadable", "unreadable evidence"}, {"nonzero exit", "completeness"}, {"tree changed during run", "completeness"},
	{"selection absent", "completeness"}, {"source absent", "completeness"}, {"cheap only", "strength"}, {"cheap consumer", ""}, {"unknown strength", "strength"},
	{"first field", "version"},
}

func changeApplicableCheck(name string, e *CheckEvidence, n *CheckNeed) (*CheckEvidence, error) {
	switch name {
	case "tree":
		e.Content.Before, e.Content.After = "changed", "changed"
	case "base":
		n.Base = "actual-parent"
	case "base tree":
		n.Context.BaseTree = "different"
	case "declaration":
		n.Check.Declaration.Values[0] = "different check"
	case "declaration digest":
		n.Check.Declaration.SourceSHA256 = "different"
	case "commands":
		n.Check.Cheap = "different command"
	case "engine":
		n.Context.EngineDigest = "different"
	case "toolchain":
		n.Context.GoVersion = "different"
	case "platform":
		n.Context.GOOS = "linux"
	case "architecture":
		n.Context.GOARCH = "amd64"
	case "build tags":
		n.Context.BuildTags["fast-static-build"] = []string{"different"}
	case "environment":
		n.Context.EnvironmentSHA256 = "different"
	case "setting":
		n.Context.Settings["landing.impact-max-share"] = "25"
	case "workers":
		n.Context.Workers = 3
	case "requirements":
		e.Selection.Groups = []string{"prerequisite", "unit/example"}
	case "closure":
		e.Selection.Groups = []string{"fast-static-build", "unit/example"}
	case "contract":
		n.Context.ContractDigest = "different"
	case "policy":
		n.Context.VerdictPolicyVersion = 2
	case "audits true":
		e.Commands[1], e.Declaration.Values[1] = "true", "true"
		n.Check.Audits, n.Check.Declaration.Values[1], e.Strength.Audited = "true", "true", false
	case "manual publication only":
		e.Manual, e.Declaration, e.Commands[1] = true, nil, "metasystem test impact --publication-only"
		n.Check.Declaration, n.Check.Audits, e.Strength.Audited = nil, e.Commands[1], false
	case "publication only":
		e.Commands[1], e.Declaration.Values[1] = "metasystem test impact --publication-only", "metasystem test impact --publication-only"
		n.Check.Audits, n.Check.Declaration.Values[1], e.Strength.Audited = e.Commands[1], e.Commands[1], false
	case "legacy":
		e.Version = 0
	case "version":
		e.Version = 2
	case "missing":
		return nil, os.ErrNotExist
	case "unreadable":
		return e, errors.New("cannot decode evidence")
	case "nonzero exit":
		e.Strength.Complete = false
	case "tree changed during run":
		e.Content.After = "changed"
	case "selection absent":
		e.Selection.Version = 0
	case "source absent":
		e.Source = ""
	case "cheap only":
		e.Strength.Audited = false
	case "cheap consumer":
		n.Strength, e.Strength.Audited = "cheap", false
	case "unknown strength":
		n.Strength = "unknown"
	case "first field":
		e.Version, e.Strength.Complete, n.Tree = 2, false, "different"
	}
	return e, nil
}

func TestApplicableCheckDecisionTable(t *testing.T) {
	t.Parallel()
	for _, row := range applicabilityRows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			evidence, need := applicableCheckFixture(t)
			evidence, err := changeApplicableCheck(row.name, evidence, &need)
			decision := ApplicableCheck(evidence, "evidence.json", err, need)
			if decision.Field != row.field || (decision.Reuse != nil) != (row.field == "") {
				t.Fatalf("decision = %+v, want field %q", decision, row.field)
			}
			if decision.Reuse != nil && (decision.Reuse.Source != "check-source" || decision.Reuse.Path != "evidence.json" || decision.Reuse.Why == "") {
				t.Fatalf("reuse lost its source: %+v", decision.Reuse)
			}
		})
	}
}

// The shared fixture inventories refs at each snapshot. Publication omits
// that inventory, so consume its fixture observation without changing Git.
type publicationSnapshotGit struct{ git *stubGit }

type publicationStarter struct{ starter *completingStarter }

func (s publicationStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	_, err := s.starter.StartSupervisor(id, state)
	return ref(30), err
}

func (g publicationSnapshotGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	data, err := g.git.Run(dir, env, args...)
	if err == nil && len(args) == 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
		_, err = g.git.Run(dir, env, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags", "refs/notes", "refs/stash")
	}
	return data, err
}

func publicationFixture(t *testing.T, change string) (unitFixture, string, *CheckEvidence, CheckNeed, *[]string) {
	t.Helper()
	f := newUnitFixture(t, "")
	result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil {
		t.Fatal(err)
	}
	f.runner.Git = publicationSnapshotGit{f.git}
	f.manager.Supervisor = publicationStarter{f.starter}
	round := result.Record.Rounds[0]
	evidence, need := applicableCheckFixture(t)
	evidence, readErr := changeApplicableCheck(change, evidence, &need)
	planPath := filepath.Join(round.Directory, "plan.json")
	plan, err := ReadUnitPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	need.Check.Directory = f.worktree
	plan.Check = &need.Check
	if err := writeUnitJSON(planPath, plan, f.runner.Root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(round.Directory, "check-source", "evidence.json")
	if readErr == nil {
		if err := writeUnitJSON(path, evidence, f.runner.Root); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(readErr) {
		if err := writeUnitJSON(path, "malformed record", f.runner.Root); err != nil {
			t.Fatal(err)
		}
	}
	f.runner.PublicationCheck = func(worktree, parent string, check UnitCheck, previous CheckSelection) (CheckSelection, []string, error) {
		if worktree != f.worktree || parent != need.Base {
			t.Fatalf("publication context: %s %s", worktree, parent)
		}
		return need.Context, need.Requirements, nil
	}
	messages := &[]string{}
	f.runner.CheckMismatch = func(message string) { *messages = append(*messages, message) }
	return f, result.Record.ID, evidence, need, messages
}

func provePublication(t *testing.T, runner *UnitRunner, run, worktree, parent string) UnitSubject {
	t.Helper()
	var subject UnitSubject
	err := runner.ReviewSubject(run, func(review UnitReview, retain func(UnitSubject) error) error {
		subject = UnitSubject{Round: review.Round.Number, DiffDigest: review.DiffDigest}
		if review.Subject != nil {
			subject = *review.Subject
		}
		proof, err := runner.ProveRoundResult(review, worktree, parent, &subject, retain)
		if err != nil {
			return err
		}
		subject.GateRunID = proof
		return retain(subject)
	})
	if err != nil {
		for _, id := range subject.GateLaunches {
			record, readErr := runner.Manager.Store.Read(id)
			t.Logf("publication launch: %+v %v", record, readErr)
		}
		t.Fatalf("publication: %v", err)
	}
	return subject
}

func TestPublicationReusesTheUnitsCompletedCheck(t *testing.T) {
	t.Parallel()
	f, run, _, need, messages := publicationFixture(t, "match")
	launches := len(f.starter.ids)
	subject := provePublication(t, f.runner, run, f.worktree, need.Base)
	if len(f.starter.ids) != launches || len(subject.GateLaunches) != 0 || subject.GateReuse == nil || subject.GateReuse.Source != "check-source" || subject.GateRunID != "check-source" || len(*messages) != 0 {
		t.Fatalf("publication launched instead of retaining reuse: %+v launches=%v messages=%v", subject, f.starter.order, *messages)
	}
	record, err := f.runner.Status(run)
	if err != nil || record.Subjects[0].GateReuse == nil || record.Subjects[0].GateReuse.Path != subject.GateReuse.Path {
		t.Fatalf("reuse not persisted: %+v %v", record.Subjects, err)
	}
}

func TestPublicationRerunsWhenTheCheckDoesNotApply(t *testing.T) {
	t.Parallel()
	for _, row := range applicabilityRows {
		if row.name == "cheap consumer" || row.name == "unknown strength" {
			continue
		}
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			f, run, _, need, messages := publicationFixture(t, row.name)
			launches := len(f.starter.ids)
			subject := provePublication(t, f.runner, run, f.worktree, need.Base)
			if row.field == "" {
				if len(f.starter.ids) != launches || subject.GateReuse == nil {
					t.Fatalf("eligible check was run: %+v", subject)
				}
				return
			}
			if len(f.starter.ids) != launches+1 || subject.GateReuse != nil || len(*messages) != 1 || !strings.Contains((*messages)[0], row.field) || strings.Contains((*messages)[0], "\n") {
				t.Fatalf("rerun did not name %q: %+v launches=%v messages=%v", row.field, subject, f.starter.order, *messages)
			}
		})
	}
}

func TestPublicationReuseSurvivesRestart(t *testing.T) {
	t.Parallel()
	f, run, _, need, messages := publicationFixture(t, "match")
	first := provePublication(t, f.runner, run, f.worktree, need.Base)
	launches := len(f.starter.ids)
	restarted := &UnitRunner{Manager: f.manager, Root: f.runner.Root, Git: f.runner.Git, PublicationCheck: f.runner.PublicationCheck, CheckMismatch: f.runner.CheckMismatch}
	second := provePublication(t, restarted, run, f.worktree, need.Base)
	if len(f.starter.ids) != launches || len(*messages) != 0 || second.GateReuse == nil || *first.GateReuse != *second.GateReuse {
		t.Fatalf("restart lost reuse: first=%+v second=%+v launches=%v", first, second, f.starter.order)
	}
	path := second.GateReuse.Path
	if err := os.WriteFile(path, []byte("unreadable"), 0600); err != nil {
		t.Fatal(err)
	}
	third := provePublication(t, restarted, run, f.worktree, need.Base)
	if len(f.starter.ids) != launches+1 || third.GateReuse != nil || len(*messages) != 1 || !strings.Contains((*messages)[0], "unreadable evidence") {
		t.Fatalf("restart trusted stale reuse: %+v launches=%v messages=%v", third, f.starter.order, *messages)
	}
}
