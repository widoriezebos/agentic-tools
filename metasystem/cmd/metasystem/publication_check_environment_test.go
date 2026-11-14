package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestImpactSelectionAppliesToFrozenPublicationEnvironment(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, field string }{{"same frozen environment", ""}, {"changed frozen variable", "environment"}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap=metasystem test impact --check\nproof.audits=printf audits\nproof.deadline=15\n")
			code, built, _ := b.work("work", "build", b.id, "environment", "--brief", b.brief("environment.md", "Check the frozen environment.\n"), "--lines", "10")
			if code != 0 {
				t.Fatalf("build: %d %+v", code, built)
			}
			planPath := resultData(t, built)["plan"].(string)
			plan, err := launch.ReadUnitPlan(planPath)
			helmMust(t, err)
			plan.Check.Environment = []string{"Z=last", "PATH=/inst/bin:/usr/bin:/bin", "METASYSTEM_TESTING_WORKERS=9", "A=first"}
			data, err := json.Marshal(plan)
			helmMust(t, err)
			helmMust(t, os.WriteFile(planPath, data, 0600))
			engine := filepath.Join(t.TempDir(), "metasystem")
			helmMust(t, testexec.WriteFile(engine, []byte("#!/bin/sh\ntest \"$*\" = 'test impact --check' || exit 2\n/usr/bin/env\n"), 0700))
			code, checked := declaredCheckAtEngine(t, b, b.worktree, "proof", engine, "test", "run", "--unit-run", resultData(t, built)["run"].(string))
			if code != 0 {
				t.Fatalf("check: %d %+v", code, checked)
			}
			var retained struct{ Exits []launch.CheckExit }
			helmMust(t, json.Unmarshal(mustRead(t, filepath.Join(resultData(t, checked)["execution"].(string), "result.json")), &retained))
			observed := strings.Split(strings.TrimSpace(retained.Exits[0].Output), "\n")
			if !slices.Contains(observed, "PATH="+filepath.Dir(engine)+string(os.PathListSeparator)+"/inst/bin:/usr/bin:/bin") {
				t.Fatalf("check did not run with the engine first on PATH: %v", observed)
			}
			slices.Reverse(observed)
			observed = append(observed, "HOST_LEASE=attached", "LANDING_ONLY=")
			root, parent := b.root(), plan.Check.Base
			contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "fast-static-build", Adapter: "command"}}}
			path := filepath.Join(root, "selection.json")
			err = writeImpactSelection(path, root, `go version fixture; GOOS="darwin"; GOARCH="arm64";`,
				launch.CheckSelection{Base: parent}, contract, contract,
				[]proofrun.NamedGroupResult{{ID: "fast-static-build", Workers: 9}}, nil, observed)
			helmMust(t, err)
			data, err = os.ReadFile(path)
			helmMust(t, err)
			var selected launch.CheckSelection
			helmMust(t, json.Unmarshal(data, &selected))
			plan, err = launch.ReadUnitPlan(planPath)
			helmMust(t, err)
			_, digest, _ := publicationCheckEnvironment(*plan.Check, parent)
			if selected.EnvironmentSHA256 != digest {
				t.Fatalf("producer recorded environment %s; consumer wants %s", selected.EnvironmentSHA256, digest)
			}
			if row.field != "" {
				plan.Check.Environment[0] = "Z=changed"
			}
			current := selected
			_, current.EnvironmentSHA256, current.Workers = publicationCheckEnvironment(*plan.Check, parent)
			evidence := launch.CheckEvidence{Version: 1, Selection: selected, Declaration: plan.Check.Declaration,
				Commands: [2]string{plan.Check.Cheap, plan.Check.Audits}, Source: "check-source"}
			evidence.Content.Before, evidence.Content.After = "candidate-tree", "candidate-tree"
			evidence.Strength.Complete, evidence.Strength.Cheap, evidence.Strength.Audited = true, true, true
			decision := launch.ApplicableCheck(&evidence, path, nil, launch.CheckNeed{Tree: "candidate-tree", Base: parent, Strength: "audited",
				Check: *plan.Check, Context: current, Requirements: []string{"fast-static-build"}})
			if decision.Field != row.field || (decision.Reuse != nil) != (row.field == "") {
				t.Fatalf("producer and consumer decision=%+v, want field %q; recorded environment=%s current=%s", decision, row.field, selected.EnvironmentSHA256, current.EnvironmentSHA256)
			}
		})
	}
}
