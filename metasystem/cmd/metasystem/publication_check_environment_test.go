package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestImpactSelectionAppliesToFrozenPublicationEnvironment(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, field string }{{"same frozen environment", ""}, {"changed frozen variable", "environment"}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root, parent := t.TempDir(), "publication-parent"
			plan := launch.UnitPlan{Check: &launch.UnitCheck{
				Cheap: "metasystem test impact --check", Audits: "declared audits", Base: parent,
				Environment: []string{"Z=last", "PATH=/frozen/engine:/usr/bin:/bin", "METASYSTEM_TESTING_WORKERS=9", "A=first"},
				Declaration: &launch.UnitDeclaration{Values: [4]string{"metasystem test impact --check", "declared audits", "15", "full"}, SourceSHA256: "declaration"},
			}}
			observed := slices.Clone(plan.Check.Environment)
			slices.Reverse(observed)
			observed = append(observed, launch.CheckEnvironmentVariable+"="+launch.CheckEnvironmentDigest(plan.Check.Environment, parent),
				"PWD=/shell/directory", "SHLVL=1", "_=/shell/metasystem", "HOST_LEASE=attached", "LANDING_ONLY=", "LANDING_PROOF_BASE="+parent)
			contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "fast-static-build", Adapter: "command"}}}
			path := filepath.Join(root, "selection.json")
			err := writeImpactSelection(path, root, `go version fixture; GOOS="darwin"; GOARCH="arm64";`,
				launch.CheckSelection{Base: parent}, contract, contract,
				[]proofrun.NamedGroupResult{{ID: "fast-static-build", Workers: 9}}, nil, observed)
			helmMust(t, err)
			data, err := os.ReadFile(path)
			helmMust(t, err)
			var selected launch.CheckSelection
			helmMust(t, json.Unmarshal(data, &selected))
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
