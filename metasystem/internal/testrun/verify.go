package testrun

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func NewFreshEpisode() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func FreshGroups(prepared Preparation, request SelectionRequest) (map[string]bool, time.Duration) {
	selected := make(map[string]bool, len(prepared.Plan.SelectedGroups))
	for _, id := range prepared.Plan.SelectedGroups {
		selected[id] = true
	}
	fresh := map[string]bool{}
	var maxAge time.Duration
	for _, group := range prepared.EffectiveContract.Groups {
		if !selected[group.ID] {
			continue
		}
		if group.Freshness == "episode" || request.NoReuse || request.Purpose == testpolicy.PurposeCadence {
			fresh[group.ID] = true
			if group.FreshnessMaxAgeMS != nil {
				age := time.Duration(*group.FreshnessMaxAgeMS) * time.Millisecond
				if maxAge == 0 || age < maxAge {
					maxAge = age
				}
			}
		}
	}
	return fresh, maxAge
}

func OwnedResources(prepared Preparation, attempt proofrun.Attempt) (string, []string) {
	selected := map[string]bool{}
	for _, id := range prepared.Plan.SelectedGroups {
		selected[id] = true
	}
	class := ""
	resources := map[string]bool{}
	for _, group := range prepared.EffectiveContract.Groups {
		if !selected[group.ID] {
			continue
		}
		if len(attempt.TestInventory) != 0 && attempt.TestOwned[group.ID] == "" {
			continue
		}
		if class == "" {
			class = "cheap"
		}
		if group.Resources.Class != "cheap" || group.Adapter == "go" {
			class = "heavy"
		}
		for _, resource := range group.Resources.Exclusive {
			resources[resource] = true
		}
	}
	names := make([]string, 0, len(resources))
	for name := range resources {
		names = append(names, name)
	}
	sort.Strings(names)
	return class, names
}

func BindFreshnessProjection(request *proofrun.TestRunRequest, installation string) error {
	return BindFreshnessProjectionWithWorkspace(request, installation, gittree.Workspace{Dir: installation})
}

type projectionAccess struct {
	raw func(gittree.RawRequest) gittree.RawResult
}

func (a projectionAccess) TopLevel(root string) (string, error) {
	return (gittree.Workspace{Dir: root, RawSource: a.raw}).TopLevel()
}

func (a projectionAccess) Prefix(root string) (string, error) {
	return (gittree.Workspace{Dir: root, RawSource: a.raw}).Prefix()
}

func (a projectionAccess) FilterPrefixes(root, tree string, paths []string) (string, error) {
	return (gittree.Workspace{Dir: root, RawSource: a.raw}).FilterTreePrefixes(tree, paths)
}

func BindFreshnessProjectionWithWorkspace(request *proofrun.TestRunRequest, installation string, workspace gittree.Workspace) error {
	if request.FreshnessEpisode == "" {
		return nil
	}
	tree, err := landing.ProjectWorkspaceTreeWith(installation, request.CandidateTree, projectionAccess{raw: workspace.RawSource})
	if err != nil {
		return err
	}
	request.FreshnessCandidateProjection = tree
	return nil
}

// A supplied episode is reusable only while the decision it names still
// covers the same candidate, base, plan, and execution inputs.
func FreshnessBinding(request proofrun.TestRunRequest, identities map[string]string, episode string) string {
	if episode == "" {
		return ""
	}
	candidate := request.CandidateTree
	if request.FreshnessCandidateProjection != "" {
		candidate = request.FreshnessCandidateProjection
	}
	encoded, _ := json.Marshal(struct {
		Purpose, Candidate, Base, PolicyBase, Contract, BaseContract, Plan, ExpiresAt string
		Groups                                                                        map[string]string
		Environment                                                                   []string
	}{string(request.Plan.Purpose), candidate, request.BaseCommit, request.PolicyBaseCommit,
		request.ContractDigest, request.BaseContractDigest,
		proofrun.TestPlanDigest(request.Contract, request.Plan, candidate), request.FreshnessExpiresAt, identities, request.Environment})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func CandidateManifest(workspace gittree.Workspace, candidateTree string, scratch *proofrun.ScratchRun) (string, error) {
	var detached *gittree.DetachedWorktree
	var err error
	if scratch != nil {
		var plan *gittree.WorktreePlan
		if plan, err = scratch.PlanWorktree(workspace, "engine"); err == nil {
			detached, err = plan.Create(candidateTree)
		}
	} else {
		detached, err = workspace.NewDetachedWorktree(candidateTree)
	}
	if err != nil {
		return "", err
	}
	digest, digestErr := proofrun.FullDigest(detached.Workspace().Dir)
	closeErr := detached.Close()
	if digestErr != nil {
		return "", digestErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return digest, nil
}

// WorkerPolicy is the testing worker allowance a configuration resolves to:
// the workers, the host's admission maximum, and whether the allowance is
// the automatic one read from this host (and the most it could choose).
type WorkerPolicy struct {
	Workers, AdmissionMaximum int
	Automatic                 bool
	AutomaticCeiling          int
}

// ApplyWorkerPolicy resolves prepared's worker policy through resolve and
// records its workers and admission maximum on prepared.
func ApplyWorkerPolicy(prepared *Preparation, resolve func(confPath string) (WorkerPolicy, error)) (WorkerPolicy, error) {
	policy, err := resolve(prepared.ConfPath)
	if err != nil {
		return WorkerPolicy{}, err
	}
	prepared.Workers, prepared.AdmissionMaximum = policy.Workers, policy.AdmissionMaximum
	return policy, nil
}

// Verification is what VerifyPrepared reads through: the semantic clock,
// the retained group-identity revalidation, the project workspace, the
// candidate engine's seams, the candidate opener, the verification's own
// scratch run (nil keeps host temp) and the worker policy resolver.
type Verification struct {
	Clock         func() time.Time
	Revalidate    func(context.Context, proofrun.TestRunRequest, []proofrun.Attempt) (map[string]string, error)
	Workspace     gittree.Workspace
	CandidateIO   candidateengine.IO
	OpenCandidate func(string, string) (proofrun.CandidateWorkspace, error)
	Scratch       *proofrun.ScratchRun
	WorkerPolicy  func(confPath string) (WorkerPolicy, error)
}

// VerifyPrepared composes the retained proof covering prepared's candidate
// for the request's goal and accounting revision; it launches nothing and
// creates no attempt.
func VerifyPrepared(request SelectionRequest, prepared Preparation, dependencies Verification) (proofrun.TestResult, error) {
	initialFreshnessCheckAt := dependencies.Clock()
	if request.FreshExpiresAt != "" {
		expires, parseErr := time.Parse(time.RFC3339Nano, request.FreshExpiresAt)
		if parseErr != nil || !expires.After(initialFreshnessCheckAt) {
			return proofrun.TestResult{}, fmt.Errorf("freshness episode has expired; renew the proof decision")
		}
	}
	// The limits are read for their validity; the retained-result checks
	// below run under no clock (proof-groups-detect-hangs-by-progress-not-
	// the-clock, slice 2).
	limits, err := ApplyWorkerPolicy(&prepared, dependencies.WorkerPolicy)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	attempts, err := proofrun.ReadAttempts(prepared.ProofControlRoot())
	if err != nil {
		return proofrun.TestResult{}, err
	}
	if request.ExecutedWorkers > 0 {
		prepared.Workers = request.ExecutedWorkers
	} else if limits.Automatic {
		// An automatic allowance is the run's reading of this host's free
		// memory, and group execution identity binds it because it shapes
		// what a performance group or a whole-allowance command runs. A
		// verification checks that run; re-sampling memory here would ask a
		// different question and refuse an unchanged tree whenever the load
		// moved (2026-09-28: five workers at run, six at verify).
		if recorded, ok := recordedAutomaticWorkers(prepared, attempts, limits.AutomaticCeiling); ok {
			prepared.Workers = recorded
		}
	}
	identityBase := context.Background()
	if dependencies.Scratch != nil {
		// The identity projection's indexes and Git children belong to the
		// verification's own scratch run, like a run's do.
		identityBase = proofrun.WithScratchRun(identityBase, dependencies.Scratch)
	}
	identityContext, cancelIdentity := context.WithCancel(identityBase)
	candidateEngineBuildIdentity, err := candidateengine.BuildIdentityUsing(identityContext, dependencies.Workspace,
		prepared.Prefix, prepared.CandidateTree, prepared.Environment, dependencies.CandidateIO)
	cancelIdentity()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	candidateEngineDigest, err := RetainedCandidateEngineDigest(prepared, attempts, candidateEngineBuildIdentity, request.Carried)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest := RunRequest(prepared, "", "", "", candidateEngineDigest, candidateEngineBuildIdentity)
	runRequest.WithCandidateOpener(dependencies.OpenCandidate)
	metadataBase := context.Background()
	if dependencies.Scratch != nil {
		if err := PrepareScratch(context.Background(), &runRequest, dependencies.Scratch, prepared); err != nil {
			return proofrun.TestResult{}, err
		}
		metadataBase = proofrun.WithScratchRun(metadataBase, dependencies.Scratch)
	}
	metadataContext, cancelMetadata := context.WithCancel(metadataBase)
	identities, err := dependencies.Revalidate(metadataContext, runRequest, attempts)
	cancelMetadata()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest.FreshnessEpisode = request.FreshEpisode
	runRequest.FreshnessExpiresAt = request.FreshExpiresAt
	freshGroups, maxFreshAge := FreshGroups(prepared, request)
	if len(freshGroups) > 0 && request.FreshEpisode == "" ||
		len(freshGroups) == 0 && request.FreshEpisode != "" ||
		maxFreshAge > 0 && request.FreshExpiresAt == "" {
		return proofrun.TestResult{}, fmt.Errorf("selected fresh testing groups require the same explicit episode and expiry used by test run")
	}
	runRequest.FreshGroups = freshGroups
	if err := BindFreshnessProjectionWithWorkspace(&runRequest, prepared.Installation, dependencies.Workspace); err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest.FreshnessBinding = FreshnessBinding(runRequest, identities, request.FreshEpisode)
	reuseDecisionAt := dependencies.Clock()
	return proofrun.ReusedTestResult(proofrun.NewTestResultAt(runRequest, reuseDecisionAt), attempts, identities,
		prepared.EffectiveContract), nil
}

func AttemptAccountsFor(attempt proofrun.Attempt, goalID string, accountingRevision uint64) bool {
	return attempt.AccountedGoal() == goalID && attempt.AccountedRevision() == accountingRevision
}

func InputManifestContains(manifest []string, candidate string) bool {
	for _, declaration := range manifest {
		matched, err := pathpattern.MatchManifestEntry(declaration, candidate)
		if err != nil || matched {
			return true
		}
	}
	return false
}

// ErrRetainedCandidateEngineAbsent is RetainedCandidateEngineDigest's answer
// when no retained evidence names the candidate engine's digest.
var ErrRetainedCandidateEngineAbsent = errors.New("candidate engine digest is absent from retained evidence")

// RetainedCandidateEngineDigest is the candidate engine digest the newest
// retained measurement of buildIdentity recorded, from an attempt or else a
// testing receipt.
func RetainedCandidateEngineDigest(prepared Preparation, attempts []proofrun.Attempt, buildIdentity string, carried bool) (string, error) {
	matching := func(result proofrun.TestResult) (string, bool) {
		matches := result.CandidateEngineIdentityVersion == proofrun.CandidateEngineIdentitySchemaVersion &&
			result.CandidateEngineBuildIdentity == buildIdentity && (carried || result.Delivery.Sufficient) &&
			proofrun.ValidateTestResult(result) == nil
		if !matches {
			return "", false
		}
		if len(result.CandidateEngineDigest) == sha256.Size*2 {
			return result.CandidateEngineDigest, true
		}
		return "", false
	}
	var newestStarted time.Time
	digest := ""
	for _, attempt := range attempts {
		started, startErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		candidateDigest, matches := "", false
		if attempt.TestResult != nil {
			candidateDigest, matches = matching(*attempt.TestResult)
		}
		completedMeasurement := attempt.Terminal != nil &&
			(attempt.Terminal.Result == proofrun.TerminalSuccess || carried && attempt.Terminal.Result == proofrun.TerminalFailed)
		if startErr == nil && completedMeasurement && matches &&
			(newestStarted.IsZero() || started.After(newestStarted)) {
			newestStarted, digest = started, candidateDigest
		}
	}
	if digest != "" {
		return digest, nil
	}
	receiptsRoot := filepath.Dir(landing.TestReceiptPath(prepared.ProofControlRoot(), prepared.CandidateTree))
	entries, readErr := os.ReadDir(receiptsRoot)
	if readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			var receipt landing.TestReceipt
			if strictjson.Read(filepath.Join(receiptsRoot, entry.Name()), &receipt) != nil ||
				receipt.SchemaVersion != 2 || receipt.Testing == nil {
				continue
			}
			candidateDigest, matches := matching(*receipt.Testing)
			markedIdentityMatches := receipt.PolicyEngineDigest == receipt.Testing.PolicyEngineDigest &&
				receipt.CandidateEngineDigest == receipt.Testing.CandidateEngineDigest &&
				receipt.CandidateEngineBuildIdentity == receipt.Testing.CandidateEngineBuildIdentity
			if matches && markedIdentityMatches {
				return candidateDigest, nil
			}
		}
	}
	return "", fmt.Errorf("%w for build identity %s; run metasystem test run", ErrRetainedCandidateEngineAbsent, buildIdentity)
}

func ReadWorkerResult(path string) (proofrun.TestResult, error) {
	var result proofrun.TestResult
	if err := strictjson.Read(path, &result); err != nil {
		return proofrun.TestResult{}, err
	}
	if err := proofrun.ValidateTestResult(result); err != nil {
		return proofrun.TestResult{}, err
	}
	return result, nil
}

// BindScratch binds a launcher request to the run's scratch root and
// its managed environment; preparation, execution and verification all bind
// through here so their identities agree.
// The first binding of a run snapshots its environment once; every later
// request of the run carries that exact descriptor and only validates it, so
// the configuration identities were derived from is the one executed.
func BindScratch(request *proofrun.TestRunRequest, scratch *proofrun.ScratchRun, locator *proofrun.ScratchLocator, prepared *proofrun.ScratchEnvironment) error {
	request.BindScratch(scratch, locator)
	if prepared == nil {
		return proofrun.PrepareScratchEnvironment(request, scratch)
	}
	request.ScratchEnvironment = prepared
	return proofrun.ValidateScratchEnvironment(*request, scratch)
}

// recordedAutomaticWorkers returns the allowance the newest successful
// run for this goal and accounting revision recorded, preferring a run on the
// verified candidate tree. Only an allowance the automatic policy could have
// chosen on this host (one through ceiling) is taken; anything else leaves the
// fresh resolution, which then refuses honestly.
func recordedAutomaticWorkers(prepared Preparation, attempts []proofrun.Attempt, ceiling int) (int, bool) {
	var chosen *proofrun.Attempt
	var chosenStarted time.Time
	chosenSameTree := false
	for index := range attempts {
		attempt := &attempts[index]
		if !AttemptAccountsFor(*attempt, prepared.GoalID, prepared.AccountingRevision) ||
			attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil ||
			attempt.TestResult.WorkerPolicyVersion != proofrun.TestWorkerPolicyVersion ||
			attempt.TestResult.Workers < 1 || attempt.TestResult.Workers > ceiling {
			continue
		}
		started, err := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if err != nil {
			continue
		}
		sameTree := attempt.TestResult.CandidateTree == prepared.CandidateTree
		if chosen == nil || sameTree && !chosenSameTree || sameTree == chosenSameTree && started.After(chosenStarted) {
			chosen, chosenStarted, chosenSameTree = attempt, started, sameTree
		}
	}
	if chosen == nil {
		return 0, false
	}
	return chosen.TestResult.Workers, true
}
