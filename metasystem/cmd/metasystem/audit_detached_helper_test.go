package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const detachedBeforeTheAudit = "detached before this audit (2026-11-03); its move into fixture custody is goal every-detached-test-helper-is-in-custody, note astra-brief-r1-1"
const custodyWitness = "the custody's own witness: it starts detached processes to prove the custodian or the scan ends them"

type detachedHelperAllowance struct {
	count  int
	reason string
}

var detachedHelperAllowances = map[string]detachedHelperAllowance{
	"cmd/metasystem/fleet_boundary_test.go":                   {1, detachedBeforeTheAudit},
	"cmd/metasystem/goal_landing_workers_public_test.go":      {1, detachedBeforeTheAudit},
	"cmd/metasystem/helm_admits_e2e_test.go":                  {2, detachedBeforeTheAudit},
	"cmd/metasystem/intent_policy_test.go":                    {1, detachedBeforeTheAudit},
	"cmd/metasystem/landing_guard_helm_test.go":               {1, detachedBeforeTheAudit},
	"cmd/metasystem/proof_run_test.go":                        {1, detachedBeforeTheAudit},
	"cmd/metasystem/provider_recovery_test.go":                {1, detachedBeforeTheAudit},
	"cmd/metasystem/signed_in_launch_e2e_test.go":             {1, detachedBeforeTheAudit},
	"cmd/metasystem/steward_arm_session_test.go":              {1, detachedBeforeTheAudit},
	"cmd/metasystem/test_scratch_public_test.go":              {2, detachedBeforeTheAudit},
	"internal/adapter/supervisor/deadline_test.go":            {2, detachedBeforeTheAudit},
	"internal/adapter/supervisor/fake_helpers_test.go":        {1, detachedBeforeTheAudit},
	"internal/applaunch/census_test.go":                       {1, detachedBeforeTheAudit},
	"internal/delegation/dispatchbed_test.go":                 {1, detachedBeforeTheAudit},
	"internal/delegation/port_p3_integration_test.go":         {1, detachedBeforeTheAudit},
	"internal/delegation/port_p4_integration_test.go":         {1, detachedBeforeTheAudit},
	"internal/delegation/port_p5_integration_test.go":         {1, detachedBeforeTheAudit},
	"internal/delegation/port_p6_test.go":                     {1, detachedBeforeTheAudit},
	"internal/deploy/bed_producer_test.go":                    {1, detachedBeforeTheAudit},
	"internal/diskstore/process_test.go":                      {3, detachedBeforeTheAudit},
	"internal/diskstore/process_zombie_linux_test.go":         {2, detachedBeforeTheAudit},
	"internal/dispatch/proof_attempt_test.go":                 {2, detachedBeforeTheAudit},
	"internal/identity/fixture_custodian_witness_test.go":     {4, custodyWitness},
	"internal/identity/processtable_test.go":                  {1, custodyWitness},
	"internal/identity/survivors_test.go":                     {1, custodyWitness},
	"internal/identity/witnessevent_test.go":                  {2, custodyWitness},
	"internal/missionrunner/host_process_test.go":             {1, detachedBeforeTheAudit},
	"internal/missionrunner/hostturn/fake_test.go":            {1, detachedBeforeTheAudit},
	"internal/missionrunner/launch_progress_test.go":          {1, detachedBeforeTheAudit},
	"internal/missionrunner/preflight_fixture_test.go":        {1, detachedBeforeTheAudit},
	"internal/missionrunner/stop_test.go":                     {2, detachedBeforeTheAudit},
	"internal/missionrunner/winddown_test.go":                 {1, detachedBeforeTheAudit},
	"internal/proofrun/group_table_test.go":                   {1, detachedBeforeTheAudit},
	"internal/proofrun/resource_custody_review_test.go":       {1, detachedBeforeTheAudit},
	"internal/proofrun/supervisor_test.go":                    {1, detachedBeforeTheAudit},
	"internal/steward/runner_git_process_fixture_test.go":     {1, detachedBeforeTheAudit},
	"internal/stoptransition/proof_attempt_inventory_test.go": {1, detachedBeforeTheAudit},
	"internal/supervise/testdata/ownerfixture/process.go":     {1, detachedBeforeTheAudit},
	"internal/testenv/fixture_exit_test.go":                   {1, custodyWitness},
	"internal/testutil/fixture_test.go":                       {1, custodyWitness},
}

func detachedHelperSites(root string) (map[string][]int, error) {
	sites := map[string][]int{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (name == "artifacts" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasSuffix(path, "_test.go") && !(strings.HasSuffix(path, ".go") && slices.Contains(strings.Split(relative, "/"), "testdata")) {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			custody := false
			var detached []int
			ast.Inspect(declaration, func(node ast.Node) bool {
				if identifier, ok := node.(*ast.Ident); ok && identifier.Name == "FixtureOwnerEnv" {
					custody = true
				}
				if call, ok := node.(*ast.CallExpr); ok {
					name := detachedHelperName(call.Fun)
					_, method := call.Fun.(*ast.SelectorExpr)
					if name == "SetupFixtureTag" || name == "StartHeldProcess" || method && (name == "Shell" || (name == "Env" || name == "env") && len(call.Args) == 1) {
						custody = true
					}
				}
				literal, ok := node.(*ast.CompositeLit)
				if !ok || detachedHelperName(literal.Type) != "SysProcAttr" {
					return true
				}
				for _, element := range literal.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if ok && (detachedHelperName(field.Key) == "Setsid" || detachedHelperName(field.Key) == "Setpgid") && detachedHelperName(field.Value) == "true" {
						detached = append(detached, positions.Position(literal.Pos()).Line)
						break
					}
				}
				return true
			})
			if !custody && len(detached) != 0 {
				sites[relative] = append(sites[relative], detached...)
			}
		}
		return nil
	})
	return sites, err
}

func detachedHelperName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	}
	return ""
}

func detachedHelperProblems(sites map[string][]int, allowance map[string]detachedHelperAllowance) []string {
	var problems []string
	for path, lines := range sites {
		allowed := allowance[path].count
		if len(lines) > allowed {
			problems = append(problems, fmt.Sprintf("%s: detached helper sites at lines %v (allowed %d): start it with a fixture key: testutil.Fixture's Env or Shell, testenv.SetupFixtureTag, or testutil.StartHeldProcess", path, lines, allowed))
		} else if len(lines) < allowed && len(lines) != 0 {
			problems = append(problems, fmt.Sprintf("%s: lower the allowance to %d (%s)", path, len(lines), allowance[path].reason))
		}
	}
	for path := range allowance {
		if len(sites[path]) == 0 {
			problems = append(problems, path+": remove the allowance")
		}
	}
	slices.Sort(problems)
	return problems
}

func TestAuditNoTestStartsADetachedHelperOutsideCustody(t *testing.T) {
	t.Parallel()
	sites, err := detachedHelperSites("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range detachedHelperProblems(sites, detachedHelperAllowances) {
		t.Error(problem)
	}
}

func TestAuditDetachedHelperCountSeesEveryShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sources := map[string]string{
		"a_test.go": `package fixture
import ("os"; "os/exec"; "syscall"; sys "golang.org/x/sys/unix")
func raw() { command := exec.Command(os.Args[0], "--", "--helper"); command.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func aliased() { _ = sys.SysProcAttr{Setpgid: true} }
func held() { command.Env = fixture.Env(os.Environ()); _ = syscall.SysProcAttr{Setsid: true} }
func attached() { _ = syscall.SysProcAttr{Setsid: false} }
func text() { _ = "SysProcAttr{Setsid: true}" }
func page() { page.Env(); _ = syscall.SysProcAttr{Setsid: true} }
`,
		"testdata/helper/helper.go": "package helper\nvar command = &syscall.SysProcAttr{Setsid: true}\n",
		"prod.go":                   "package prod\nvar command = &syscall.SysProcAttr{Setsid: true}\n",
	}
	for path, source := range sources {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sites, err := detachedHelperSites(root)
	want := map[string][]int{"a_test.go": {3, 4, 8}, "testdata/helper/helper.go": {2}}
	if err != nil || !reflect.DeepEqual(sites, want) {
		t.Fatalf("detached sites=%v want=%v err=%v", sites, want, err)
	}
	problems := detachedHelperProblems(sites, nil)
	if len(problems) != 2 || !strings.Contains(problems[0], "a_test.go: detached helper sites at lines [3 4 8]") || !strings.Contains(problems[1], "testdata/helper/helper.go: detached helper sites at lines [2]") {
		t.Fatalf("raw helpers were not refused with their lines: %v", problems)
	}
}
