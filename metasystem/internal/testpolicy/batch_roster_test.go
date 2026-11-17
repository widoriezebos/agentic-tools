package testpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBatchRosterCompleteness(t *testing.T) {
	t.Parallel()
	contract, err := Load("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]map[string]bool{}
	whole := map[string]bool{}
	for _, id := range BatchRosterIDs() {
		i := slices.IndexFunc(contract.Groups, func(g Group) bool { return g.ID == id })
		if i < 0 {
			t.Fatalf("missing batch roster group %s", id)
		}
		group := contract.Groups[i]
		if group.Adapter != "go" || group.Phase != "acceptance" || slices.Contains(contract.Always.Canary, id) || slices.Contains(contract.Always.Standard, id) {
			t.Fatalf("batch roster group %s must remain an acceptance Go group outside both always lists", id)
		}
		var names []string
		if string(group.Tests) != `"all"` {
			if err := json.Unmarshal(group.Tests, &names); err != nil {
				t.Fatal(err)
			}
		}
		for _, pkg := range group.Packages {
			if covered[pkg] == nil {
				covered[pkg] = map[string]bool{}
			}
			whole[pkg] = whole[pkg] || string(group.Tests) == `"all"`
			for _, name := range names {
				covered[pkg][name] = true
			}
		}
	}
	// The fresh-projection source audit guards repository-wide reads,
	// so its owner is part of the mandatory roster.
	if !covered["internal/testenv"]["TestAuditFreshProjectionsUseInjectedDeadline"] {
		t.Error("batch roster omits the fresh-projection source audit")
	}
	if !whole["internal/testpolicy"] {
		t.Fatal("the batch must select all of internal/testpolicy")
	}
	for _, pkg := range []string{"cmd/metasystem", "internal/testutil", "internal/goal"} {
		tests, err := untaggedGoTests(filepath.Join("../..", pkg))
		if err != nil {
			t.Fatal(err)
		}
		sourceTests := map[string]bool{}
		if pkg != "cmd/metasystem" {
			files, err := filepath.Glob(filepath.Join("../..", pkg, "*_source_test.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				for name := range tests {
					if strings.Contains(string(data), "func "+name+"(") {
						sourceTests[name] = true
					}
				}
			}
		}
		for name := range tests {
			required := sourceTests[name] || strings.HasPrefix(name, "TestAudit") || pkg == "cmd/metasystem" && (strings.HasPrefix(name, "TestVerbRatchet") || strings.Contains(name, "Bed") && strings.Contains(name, "Coverage"))
			if required && !whole[pkg] && !covered[pkg][name] {
				t.Errorf("batch roster omits %s %s", pkg, name)
			}
		}
	}
}

// Decode is the serving schema-2 reader; adding roster groups must not
// require a new field or schema that the engine reading a candidate lacks.
func TestBatchRosterUsesServingContractSchema(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if contract.SchemaVersion != 2 {
		t.Fatal("batch roster changed the serving schema")
	}
	for _, id := range BatchRosterIDs() {
		i := slices.IndexFunc(contract.Groups, func(g Group) bool { return g.ID == id })
		if i < 0 || len(contract.Groups[i].Packages) == 0 || len(contract.Groups[i].Tests) == 0 {
			t.Fatalf("invalid ordinary Go group %s", id)
		}
	}
}
