package testrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ContractReady validates the committed testing contract and its
// declared tools without running a test; with discovery it also resolves
// every group's native tests under the host's heavy resource lease.
func ContractReady(root string, discovery bool) (string, int, error) {
	installation, contract, path, err := LoadContract(root)
	if err != nil {
		return "", 0, err
	}
	projectRoot, err := (gittree.Workspace{Dir: installation}).TopLevel()
	if err == nil {
		err = checkTools(projectRoot, contract)
	}
	if err == nil && discovery {
		ctx := context.Background()
		lease, leaseErr := proofrun.AcquireHostResources(ctx, installation,
			filepath.Join(installation, "metasystem.conf"), "heavy", nil)
		if leaseErr != nil {
			err = leaseErr
		} else {
			err = proofrun.CheckNativeDiscovery(proofrun.WithHostResourceLease(ctx, lease), projectRoot, installation, contract,
				Environment(os.Environ()))
			err = errors.Join(err, lease.Close())
		}
	}
	if err != nil {
		return "", 0, err
	}
	return path, len(contract.Groups), nil
}

func LoadContract(root string) (string, testpolicy.Contract, string, error) {
	installation, err := realpath.Canonical(root)
	if err != nil {
		return "", testpolicy.Contract{}, "", err
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, found, err := config.CommittedLookup(confPath, "testing.contract")
	if err != nil || !found {
		return "", testpolicy.Contract{}, "", fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return "", testpolicy.Contract{}, "", fmt.Errorf("testing.contract must be a relative normalized path")
	}
	path := filepath.Join(installation, filepath.FromSlash(contractRel))
	contract, err := testpolicy.Load(path)
	return installation, contract, path, err
}

func checkTools(projectRoot string, contract testpolicy.Contract) error {
	var unavailable []string
	for _, group := range contract.Groups {
		cwd := filepath.Join(projectRoot, filepath.FromSlash(group.CWD))
		if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
			unavailable = append(unavailable, group.ID+":cwd")
			continue
		}
		environment := proofrun.GroupTestingEnvironment(Environment(os.Environ()), group)
		for _, tool := range group.Tools {
			if _, err := proofrun.ResolveTestingExecutable(context.Background(), cwd, environment, []string{tool.Executable}); err != nil {
				unavailable = append(unavailable, group.ID+":"+tool.ID)
			}
		}
	}
	if len(unavailable) > 0 {
		sort.Strings(unavailable)
		return fmt.Errorf("declared testing tools or directories unavailable: %s", strings.Join(unavailable, ","))
	}
	return nil
}

func GoalRisk(root, id string) (testpolicy.GoalRisk, uint64, error) {
	if id == "" {
		return testpolicy.GoalRisk{}, 0, nil
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return testpolicy.GoalRisk{}, 0, err
	}
	return GoalRiskAt(endpoint, id, time.Now().UTC())
}

func GoalRiskAt(endpoint goal.Endpoint, id string, now time.Time) (testpolicy.GoalRisk, uint64, error) {
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return testpolicy.GoalRisk{}, 0, err
	}
	file := projection.Tree.Live[id]
	if file == nil || file.Risk == nil {
		return testpolicy.GoalRisk{}, 0, fmt.Errorf("goal %s has no accepted risk and budget episode", id)
	}
	accountingRevision := goal.BudgetEpisodeRevision(file)
	if accountingRevision == 0 {
		return testpolicy.GoalRisk{}, 0, fmt.Errorf("goal %s has no accepted budget episode", id)
	}
	return testpolicy.GoalRisk{Severity: int(file.Risk.Severity), Novelty: int(file.Risk.Novelty),
		Exposure: int(file.Risk.Exposure), Accumulation: int(file.Risk.Accumulation)}, accountingRevision, nil
}

func ResolveGoal(root, requested string) (string, error) {
	return resolveGoalFor(root, requested, int64(os.Getppid()))
}

// resolveGoalFor resolves the goal a run accounts to; a nested proof's
// parent is authenticated from the supplied caller.
func resolveGoalFor(root, requested string, callerPID int64) (string, error) {
	return ResolveGoalWithCaller(root, requested, goal.ResolveMachine, goal.ResolveEndpoint, time.Now, callerPID)
}

func ResolveGoalWithCaller(root, requested string, resolveMachine func(string) (string, error), resolveEndpoint func(string) (goal.Endpoint, error), now func() time.Time, callerPID int64) (string, error) {
	if requested != "" {
		return requested, nil
	}
	parentRoot, parentAttempt := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if parentRoot != "" || parentAttempt != "" {
		if parentRoot == "" || parentAttempt == "" {
			return "", errors.New("this process inherited only half of its parent test run's settings, so it cannot be charged")
		}
		canonicalParent, err := realpath.Canonical(parentRoot)
		if err != nil || canonicalParent != root {
			return "", fmt.Errorf("the parent test run belongs to another installation than %s", root)
		}
		attempt, err := proofrun.AuthenticateContext(root, parentAttempt, callerPID)
		if err != nil {
			return "", err
		}
		return attempt.AccountedGoal(), nil
	}
	return UniqueActiveProofGoal(root, now().UTC(), resolveMachine, resolveEndpoint)
}

// errNoLandingRef refuses a checkout that names no landing branch: the
// tests are chosen against it, and the command names the usual one.
var errNoLandingRef = errors.New("this checkout names no landing branch to test against\n" +
	"run: git config --local metasystem.steward.landing-ref refs/remotes/origin/main")

func TrustedPolicyBase(projectRoot string, workspace gittree.Workspace) (string, error) {
	data, err := landingRef(projectRoot, "--worktree")
	if err != nil {
		data, err = landingRef(projectRoot, "--local")
	}
	ref := strings.TrimSpace(string(data))
	tail := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if err != nil || tail == ref || !qualified || remote == "" || branch == "" {
		return "", errNoLandingRef
	}
	commit, err := workspace.ResolveCommit(ref)
	if err != nil {
		return "", fmt.Errorf("the landing branch %s cannot be read: %w", ref, err)
	}
	return commit, nil
}

func landingRef(projectRoot, scope string) ([]byte, error) {
	command := exec.Command("git", "-C", projectRoot, "config", scope, "--no-includes", "--get", "metasystem.steward.landing-ref")
	command.Env = gittree.ScrubbedEnviron()
	return command.Output()
}

func Environment(environment []string) []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "TMP": true, "TEMP": true,
		"GOCACHE": true, "GOMODCACHE": true, "GOTMPDIR": true, "STATICCHECK_CACHE": true, "GOPATH": true, "GOROOT": true, "GOFLAGS": true, "GOWORK": true,
		"CGO_ENABLED": true, "GOTOOLCHAIN": true, "GOEXPERIMENT": true, "JAVA_HOME": true, "LANG": true,
		"LC_ALL": true, "SYSTEMROOT": true, "TZ": true, identity.RunOwnerEnv: true,
		// Go env file selection, snapshotted by PrepareScratchEnvironment.
		"GOENV": true, "XDG_CONFIG_HOME": true}
	var result []string
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			result = append(result, entry)
		}
	}
	sort.Strings(result)
	return result
}

func InheritedEnvironment(prepared, inherited []string) []string {
	allowed := map[string]bool{"METASYSTEM_PROOF_CONTROL_ROOT": true, "METASYSTEM_PROOF_ATTEMPT": true,
		"METASYSTEM_PROOF_RECORD_KEY": true, "METASYSTEM_PROOF_CREATION_CLAIM": true, "METASYSTEM_PROOF_AUTH_BIN": true,
		identity.RunOwnerEnv: true, identity.FixtureAttemptEnv: true}
	values := make(map[string]string, len(prepared)+len(allowed))
	for _, entry := range prepared {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	for _, entry := range inherited {
		name, value, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			values[name] = value
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+values[name])
	}
	return result
}

// UniqueActiveProofGoal is the one live claimed goal of this machine a
// proof accounts to when none is named; several, none, or only a
// breach-stopped claim is refused with the reason and the --goal remedy.
func UniqueActiveProofGoal(root string, now time.Time, resolveMachine func(string) (string, error), resolveEndpoint func(string) (goal.Endpoint, error)) (string, error) {
	machine, err := resolveMachine(root)
	if err != nil {
		return "", err
	}
	endpoint, err := resolveEndpoint(root)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return "", fmt.Errorf("the goals cannot be read to find the one this test run is charged to: %w", err)
	}
	if projection.Tree == nil {
		return "", fmt.Errorf("no goals are recorded, so this test run has no goal to be charged to")
	}
	selected := ""
	fenced := make([]*goal.GoalFile, 0)
	for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != machine {
			continue
		}
		if file.IsFencedClaim() {
			fenced = append(fenced, file)
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("machine %s holds several claimed goals; name the one to charge with --goal GOAL", machine)
		}
		selected = id
	}
	if selected == "" {
		if len(fenced) == 1 {
			return "", fmt.Errorf("machine %s holds only goal %s, which is stopped (%s); name a goal with --goal GOAL",
				machine, fenced[0].Id, fenced[0].StopFence.StopID)
		}
		return "", fmt.Errorf("machine %s holds no claimed goal to charge this test run to; name one with --goal GOAL", machine)
	}
	return selected, nil
}
