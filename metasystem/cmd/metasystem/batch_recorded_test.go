package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type recordedBatchMiss struct {
	Witness         string              `json:"witness"`
	Attempt         string              `json:"attempt"`
	Base            string              `json:"base"`
	Head            string              `json:"head"`
	Source          string              `json:"source"`
	Package         string              `json:"package"`
	Test            string              `json:"test"`
	Scope           string              `json:"scope"`
	Reason          string              `json:"reason"`
	CurrentMapping  string              `json:"current_mapping"`
	Paths           []string            `json:"paths"`
	Status          string              `json:"status_no_renames"`
	ContractSHA     string              `json:"contract_sha256"`
	FullCommand     string              `json:"full_command"`
	Impact          *goadapter.Impact   `json:"impact"`
	Inventory       map[string][]string `json:"inventory"`
	TaggedInventory map[string][]string `json:"tagged_inventory"`
	Roster          []testpolicy.Group  `json:"roster_overlay"`
}

func TestBatchPlanRecordedMissesUseTheProductionRoster(t *testing.T) {
	t.Parallel()
	directory := "testdata/batchmisses"
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	helmMust(t, err)
	var manifest []recordedBatchMiss
	helmMust(t, json.Unmarshal(data, &manifest))
	if len(manifest) != 9 {
		t.Fatalf("recorded cases=%d", len(manifest))
	}
	for _, identity := range manifest {
		t.Run(identity.Attempt, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join(directory, identity.Attempt+".json"))
			helmMust(t, err)
			var fixture recordedBatchMiss
			helmMust(t, json.Unmarshal(data, &fixture))
			if fixture.Base != identity.Base || fixture.Head != identity.Head || fixture.Package != identity.Package || fixture.Test != identity.Test || fixture.Source != identity.Source || fixture.Scope != identity.Scope || fixture.Reason != identity.Reason {
				t.Fatal("fixture provenance differs from manifest")
			}
			if len(fixture.Paths) == 0 || fixture.Status == "" {
				t.Fatal("recorded diff is empty")
			}
			var statusPaths []string
			for _, line := range strings.Split(strings.TrimSpace(fixture.Status), "\n") {
				_, path, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatal("invalid no-renames status")
				}
				statusPaths = append(statusPaths, path)
			}
			if !slices.Equal(statusPaths, fixture.Paths) {
				t.Fatal("recorded diff dropped or reordered paths")
			}
			if fixture.Attempt == "20261031T213751" && (!slices.Contains(fixture.Paths, "metasystem/internal/landing/plain/hold_state_test.go") || fixture.Package != "internal/goal" || fixture.CurrentMapping == "") {
				t.Fatal("historical deleted path or moved test provenance was lost")
			}
			root := filepath.Join(t.TempDir(), "metasystem")
			impactWrite(t, root, "metasystem.conf", "metasystem.template=true\ntesting.contract=testing.json\n")
			contractData, err := os.ReadFile(filepath.Join(directory, fixture.Attempt+"-contract.json"))
			helmMust(t, err)
			if fmt.Sprintf("%x", sha256.Sum256(contractData)) != fixture.ContractSHA {
				t.Fatal("candidate contract digest differs")
			}
			contract, err := testpolicy.Decode(contractData)
			helmMust(t, err)
			contract.Groups = append(contract.Groups, fixture.Roster...)
			contractData, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, root, "testing.json", string(contractData))
			if fixture.Scope == "impact" {
				if fixture.Impact == nil || len(fixture.Impact.Packages) == 0 || len(fixture.Inventory) == 0 {
					t.Fatal("impact fixture lacks real recorded impact or inventory")
				}
				impactWrite(t, root, "go.mod", "module example.invalid/recorded\n\ngo 1.27\n")
				recordedBatchInventory(t, root, fixture.Inventory, fixture.TaggedInventory)
			}
			calls := 0
			sources := impactSources{
				git: func(_ string, args ...string) (string, error) {
					switch args[0] {
					case "rev-parse":
						if args[2] != fixture.Base+"^{commit}" {
							t.Fatalf("base=%v", args)
						}
						return fixture.Base, nil
					case "merge-base":
						return "", nil
					case "show":
						return fixture.Attempt, nil
					case "ls-files":
						return "", nil
					case "diff":
						if !slices.Equal(args, []string{"diff", "--name-only", "--no-renames", "--no-relative", "-z", fixture.Base, "--", ":/"}) {
							t.Fatalf("incomplete diff request %v", args)
						}
						return strings.Join(fixture.Paths, "\x00") + "\x00", nil
					default:
						t.Fatalf("unexpected Git request %v", args)
						return "", nil
					}
				},
				impact: func(_ string, base string, whole bool) (goadapter.Impact, error) {
					calls++
					if fixture.Scope == "full" {
						t.Fatal("instrument route called impact adapter")
					}
					if base != fixture.Base || whole {
						t.Fatal("impact adapter received wrong policy")
					}
					return *fixture.Impact, nil
				},
				get: func(p config.GetParams) (string, int, error) {
					if p.Key == "proof.full" {
						return fixture.FullCommand, 0, nil
					}
					if p.Key == "proof.cheap" {
						return "metasystem test impact", 0, nil
					}
					return p.Default, 0, nil
				},
				getenv: func(string) string { return "" },
			}
			plan, err := sources.batchPlan(root, fixture.Base)
			helmMust(t, err)
			if plan.Scope != fixture.Scope || !slices.Equal(plan.Paths, fixture.Paths) {
				t.Fatalf("scope=%s or complete diff changed", plan.Scope)
			}
			if fixture.Scope == "full" {
				if calls != 0 || plan.Command != fixture.FullCommand || len(plan.Instruments) == 0 || len(plan.PackageTests) != 0 {
					t.Fatal("full route did not retain real command and instrument witness")
				}
				if !slices.Contains(plan.Instruments, identity.Witness) || !slices.Contains(fixture.Paths, identity.Witness) {
					t.Fatal("instrument is not in recorded diff")
				}
			} else {
				if calls != 1 || !slices.Contains(plan.PackageTests[fixture.Package], fixture.Test) || !slices.Contains(plan.GroupTests[fixture.Reason][fixture.Package], fixture.Test) {
					t.Fatal("recorded impact plan omitted failing test or its roster route")
				}
				for _, group := range fixture.Impact.Packages {
					if !slices.ContainsFunc(plan.Groups, func(g testpolicy.Group) bool { return g.ID == group.ID }) {
						t.Fatalf("real package impact was dropped: %s", group.ID)
					}
				}
				t.Logf("recorded impact: %d packages, %d distinct tests, %d group invocations", len(fixture.Impact.Packages), plan.TestCount, plan.GroupInvocations)
			}
		})
	}
}

// Materialize the recorded inventory as declaration-only source. The production
// expander still discovers and selects names; no historical test body is run.
func recordedBatchInventory(t *testing.T, root string, plain, tagged map[string][]string) {
	t.Helper()
	for pkg, names := range plain {
		impactWrite(t, root, filepath.Join(pkg, "inventory.go"), "package recorded\n")
		writeNames := func(file, header string, names []string) {
			var text strings.Builder
			text.WriteString(header + "package recorded\nimport \"testing\"\nvar _ *testing.T\n")
			for _, name := range names {
				if strings.HasPrefix(name, "Example") {
					fmt.Fprintf(&text, "func %s() {\n// Output:\n}\n", name)
				} else {
					fmt.Fprintf(&text, "func %s(t *testing.T) {}\n", name)
				}
			}
			impactWrite(t, root, filepath.Join(pkg, file), text.String())
		}
		writeNames("inventory_test.go", "", names)
		var extra []string
		for _, name := range tagged[pkg] {
			if !slices.Contains(names, name) {
				extra = append(extra, name)
			}
		}
		if len(extra) > 0 {
			writeNames("inventory_batch_test.go", "//go:build batchtest\n\n", extra)
		}
	}
}
