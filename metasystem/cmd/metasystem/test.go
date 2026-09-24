package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type testingPreparation struct {
	Installation, ControlRoot, ProjectRoot, Prefix, ConfPath string
	GoalID                                                   string
	AccountingRevision                                       uint64
	BaseCommit, PolicyBaseCommit, CandidateTree              string
	CandidateContract, BaseContract                          testpolicy.Contract
	EffectiveContract                                        testpolicy.Contract
	ContractDigest, BaseContractDigest                       string
	PolicyEngineDigest, BehaviorPolicyDigest                 string
	JudgeKey                                                 string
	EngineRearm                                              *proofrun.EngineRearm
	PolicyEngine                                             string
	FirstTestingTransition                                   bool
	Plan                                                     testpolicy.Plan
	Environment                                              []string
	AllGroups                                                bool
	Workers, AdmissionMaximum                                int
	WorkerCapabilitiesChecked                                bool
	UnmatchedInputs                                          []testingUnmatchedInput
}

type testingUnmatchedInput struct {
	Group   string `json:"group"`
	Pattern string `json:"pattern"`
}

type testingPlanOutput struct {
	SchemaVersion      int                     `json:"schemaVersion"`
	ProjectRoot        string                  `json:"projectRoot"`
	InstallationPrefix string                  `json:"installationPrefix"`
	BaseCommit         string                  `json:"baseCommit"`
	PolicyBaseCommit   string                  `json:"policyBaseCommit"`
	CandidateTree      string                  `json:"candidateTree"`
	ContractDigest     string                  `json:"contractDigest"`
	BaseContractDigest string                  `json:"baseContractDigest"`
	Plan               testpolicy.Plan         `json:"plan"`
	Groups             []testpolicy.Group      `json:"groups"`
	UnmatchedInputs    []testingUnmatchedInput `json:"unmatchedInputs,omitempty"`
}

const testWorkerProtocol = "metasystem.test-worker"

var errTestingWorkerPolicyUnsupported = errors.New("TEST_WORKER_POLICY_UNSUPPORTED")

type testingWorkerCapabilities struct {
	SchemaVersion                 int    `json:"schemaVersion"`
	Protocol                      string `json:"protocol"`
	ProtocolVersion               int    `json:"protocolVersion"`
	TestResultSchemaVersion       int    `json:"testResultSchemaVersion"`
	GroupExecutionIdentityVersion int    `json:"groupExecutionIdentityVersion"`
	WorkerPolicyVersion           int    `json:"workerPolicyVersion"`
}

func currentTestingWorkerCapabilities() testingWorkerCapabilities {
	return testingWorkerCapabilities{SchemaVersion: 1, Protocol: testWorkerProtocol,
		ProtocolVersion: proofrun.TestWorkerProtocolVersion, TestResultSchemaVersion: proofrun.TestResultSchemaVersion,
		GroupExecutionIdentityVersion: proofrun.GroupExecutionIdentityVersion, WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion}
}

func runTestWorkerCapabilities(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem test worker-capabilities")
		return 2
	}
	if err := writeTestingWorkerCapabilities(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func writeTestingWorkerCapabilities(writer io.Writer) error {
	return json.NewEncoder(writer).Encode(currentTestingWorkerCapabilities())
}

func requireTestingWorkerCapabilities(ctx context.Context, engine string, environment []string) error {
	command := exec.CommandContext(ctx, engine, "test", "worker-capabilities")
	command.Env = testingEnvironment(environment)
	data, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: trusted destination engine %q does not support worker-policy protocol %d (%v: %s); stage, prove, and install the backend compatibility release before enabling testing.workers or resources.workers", errTestingWorkerPolicyUnsupported,
			engine, proofrun.TestWorkerProtocolVersion, err, strings.TrimSpace(string(data)))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var capabilities testingWorkerCapabilities
	if err := decoder.Decode(&capabilities); err != nil {
		return fmt.Errorf("%w: trusted destination engine %q returned malformed worker capabilities: %v; stage, prove, and install the backend compatibility release first", errTestingWorkerPolicyUnsupported, engine, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trusted destination engine %q returned trailing worker capability data; stage, prove, and install the backend compatibility release first", errTestingWorkerPolicyUnsupported, engine)
	}
	want := currentTestingWorkerCapabilities()
	if capabilities != want {
		return fmt.Errorf("%w: trusted destination engine %q reports incompatible worker capabilities %+v, require %+v; stage, prove, and install the matching backend compatibility release first",
			errTestingWorkerPolicyUnsupported, engine, capabilities, want)
	}
	return nil
}

func (prepared testingPreparation) proofControlRoot() string {
	if prepared.ControlRoot != "" {
		return prepared.ControlRoot
	}
	return prepared.Installation
}

func runTestList(args []string) int {
	flags := flag.NewFlagSet("test list", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "MetaSystem installation root")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test list --root INSTALLATION [--json]")
		return 2
	}
	installation, contract, path, err := loadPhysicalTestingContract(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test list:", err)
		return 1
	}
	if *jsonOutput {
		printJSON(map[string]any{"schemaVersion": 1, "installation": installation, "contract": path, "groups": contract.Groups})
		return 0
	}
	for _, group := range contract.Groups {
		fmt.Printf("%s\t%s\t%s\n", group.ID, group.Kind, group.Adapter)
	}
	return 0
}

func runTestCheck(args []string) int {
	flags := flag.NewFlagSet("test check", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "MetaSystem installation root")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test check --root INSTALLATION [--json]")
		return 2
	}
	installation, contract, path, err := loadPhysicalTestingContract(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test check:", err)
		return 1
	}
	projectRoot, err := (gittree.Workspace{Dir: installation}).TopLevel()
	if err == nil {
		err = checkTestingTools(projectRoot, contract)
	}
	if err == nil {
		ctx := context.Background()
		lease, leaseErr := proofrun.AcquireHostResources(ctx, installation,
			filepath.Join(installation, "metasystem.conf"), "heavy", nil)
		if leaseErr != nil {
			err = leaseErr
		} else {
			err = proofrun.CheckNativeDiscovery(proofrun.WithHostResourceLease(ctx, lease), projectRoot, installation, contract,
				testingEnvironment(os.Environ()))
			err = errors.Join(err, lease.Close())
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test check:", err)
		return 1
	}
	if *jsonOutput {
		printJSON(map[string]any{"schemaVersion": 1, "status": "ready", "contract": path, "groupCount": len(contract.Groups)})
	} else {
		fmt.Printf("TEST-CONTRACT READY groups=%d contract=%s\n", len(contract.Groups), path)
	}
	return 0
}

func runTestPlan(args []string) int {
	request, jsonOutput, status := parseTestingSelection("test plan", args, false)
	if status != 0 {
		return status
	}
	request.LandedRearm = !request.PolicyChild
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test plan:", err)
		return 1
	}
	output := planOutput(prepared)
	if jsonOutput {
		printJSON(output)
	} else {
		fmt.Printf("TEST-PLAN mode=%s required=%s tree=%s groups=%s\n", prepared.Plan.ExecutedMode,
			prepared.Plan.RequiredMode, prepared.CandidateTree, strings.Join(prepared.Plan.SelectedGroups, ","))
		printUnmatchedInputs(prepared.UnmatchedInputs)
	}
	return 0
}

type testingSelectionRequest struct {
	Root, ControlRoot, GoalID, AuthorityGoalID, Tree, CapMin, RetryDecision, ResultPath string
	ExpectedGoalRevision, ExpectedAccountingRevision                                    uint64
	Mode                                                                                testpolicy.Mode
	Purpose                                                                             testpolicy.Purpose
	Groups, BatchRequirements                                                           []string
	Carried                                                                             bool
	NoReuse, ForceGroups, RequireDiagnosticHeadroom, AllGroups                          bool
	BatchPrefixReceipt, BatchTipProof, BatchAdmission                                   bool
	FreshEpisode, FreshExpiresAt                                                        string
	RequireWorkerCapabilities                                                           bool
	// CadencePreflight plans and revalidates the fetched tree before the cadence
	// tick claims standing authority. Governed cadence execution does not set it.
	CadencePreflight bool
	// LandedRearm is set by outermost plan and run commands. The pinned child
	// and the verify verb judge the engine as they find it.
	LandedRearm bool
	PolicyChild bool
}

func admitTestingRun(request testingSelectionRequest, admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	return admitTestingRunWith(request, admission, admitProofLaunch)
}

func admitTestingRunWith(request testingSelectionRequest, admission proofLaunchAdmission,
	admit func(proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error),
) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	admission.ForceAttempt = request.Purpose == testpolicy.PurposeCadence || request.NoReuse || request.ForceGroups
	if admission.CandidateTree == "" {
		admission.CandidateTree = request.Tree
	}
	return admit(admission)
}

type testingCommandAdmission struct {
	now        func() time.Time
	admitRun   func(testingSelectionRequest, proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error)
	admitProof func(proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error)
}

func (admission testingCommandAdmission) initial(request testingSelectionRequest, launch proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	launch.Now = admission.now()
	return admission.admitRun(request, launch)
}

func (admission testingCommandAdmission) forced(launch proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	launch.ForceAttempt = true
	launch.ForceGroups = true
	launch.Now = admission.now()
	return admission.admitProof(launch)
}

func parseTestingSelection(name string, args []string, execution bool) (testingSelectionRequest, bool, int) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	request := testingSelectionRequest{}
	pathFlagVar(flags, &request.Root, "root", "", "MetaSystem installation root")
	flags.StringVar(&request.GoalID, "goal", "", "accepted goal owning delivery")
	flags.StringVar(&request.AuthorityGoalID, "authority", "", "claimed goal authorizing the proof reservation")
	flags.StringVar(&request.Tree, "tree", "", "exact whole-project candidate tree")
	mode := flags.String("mode", "auto", "auto, standard, deep, or diagnostic canary")
	purpose := flags.String("purpose", "delivery", "delivery, diagnostic, or cadence")
	groups := flags.String("groups", "", "comma-separated diagnostic groups")
	batchRequirements := flags.String("batch-requirements", "", "strict JSON batch delivery requirements")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	flags.BoolVar(&request.PolicyChild, "policy-child", false, "judge policy in the pinned child without re-arming")
	flags.BoolVar(&request.BatchPrefixReceipt, "batch-prefix", false, "compose delivery evidence for a batch prefix")
	flags.BoolVar(&request.Carried, "carried", false, "compose a completed red result for carried-landing classification")
	flags.StringVar(&request.FreshEpisode, "fresh-episode", "", "retained freshness episode for a proof decision")
	flags.StringVar(&request.FreshExpiresAt, "fresh-expires-at", "", "expiry for a retained freshness episode")
	if execution {
		pathFlagVar(flags, &request.ControlRoot, "control-root", "", "durable proof control root for an internal batch proof")
		flags.BoolVar(&request.BatchTipProof, "batch-tip", false, "prove a batch tip projected into its own detached worktree")
		flags.BoolVar(&request.BatchAdmission, "batch-admission", false, "run selected batch admission checks on an exact tree")
		flags.StringVar(&request.CapMin, "cap-min", "", "reserved proof minutes")
		flags.StringVar(&request.RetryDecision, "retry-decision", "", "accountable version-1 retry decision")
		flags.StringVar(&request.ResultPath, "result", "", "atomic result projection path")
		flags.BoolVar(&request.ForceGroups, "force-groups", false, "execute every selected group regardless of retained evidence")
		flags.Uint64Var(&request.ExpectedGoalRevision, "expected-goal-revision", 0, "sealed goal revision")
		flags.Uint64Var(&request.ExpectedAccountingRevision, "expected-accounting-revision", 0, "sealed accounting revision")
		flags.BoolVar(&request.NoReuse, "no-reuse", false, "execute diagnostic groups freshly")
		flags.BoolVar(&request.RequireDiagnosticHeadroom, "require-diagnostic-headroom", false, "reserve the mandatory batch-tip diagnostic")
		flags.BoolVar(&request.AllGroups, "all-groups", false, "run every selected delivery group after a failure")
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || request.Root == "" {
		fmt.Fprintf(os.Stderr, "usage: metasystem %s --root INSTALLATION [--goal ID] [--authority ID] [--tree TREE] [--mode auto|standard|deep|canary] [--purpose delivery|diagnostic|cadence] [--groups ID,ID]\n", name)
		return request, false, 2
	}
	if request.PolicyChild && (name != "test plan" || execution) {
		fmt.Fprintln(os.Stderr, "--policy-child is internal to the pinned test plan child")
		return request, false, 2
	}
	purposeSet := false
	flags.Visit(func(value *flag.Flag) { purposeSet = purposeSet || value.Name == "purpose" })
	if *mode == string(testpolicy.ModeCanary) && !purposeSet {
		*purpose = string(testpolicy.PurposeDiagnostic)
	}
	if *groups != "" {
		for _, id := range strings.Split(*groups, ",") {
			if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
				fmt.Fprintln(os.Stderr, "diagnostic groups must be a comma-separated list of exact identifiers")
				return request, false, 2
			}
			request.Groups = append(request.Groups, id)
		}
	}
	request.Mode, request.Purpose = testpolicy.Mode(*mode), testpolicy.Purpose(*purpose)
	requirementsSet, groupsSet := false, false
	flags.Visit(func(value *flag.Flag) {
		requirementsSet = requirementsSet || value.Name == "batch-requirements"
		groupsSet = groupsSet || value.Name == "groups"
	})
	if requirementsSet {
		if !request.BatchPrefixReceipt || request.Purpose != testpolicy.PurposeDelivery || groupsSet {
			fmt.Fprintln(os.Stderr, "--batch-requirements requires delivery --batch-prefix and cannot be combined with --groups")
			return request, false, 2
		}
		var err error
		request.BatchRequirements, err = parseBatchRequirements(*batchRequirements)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid --batch-requirements:", err)
			return request, false, 2
		}
	}
	if request.BatchPrefixReceipt && groupsSet {
		fmt.Fprintln(os.Stderr, "--groups is diagnostic-only and cannot be combined with --batch-prefix")
		return request, false, 2
	}
	if request.BatchPrefixReceipt && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(os.Stderr, "--batch-prefix requires delivery purpose")
		return request, false, 2
	}
	if request.BatchTipProof && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(os.Stderr, "--batch-tip requires delivery purpose")
		return request, false, 2
	}
	if request.BatchAdmission && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(os.Stderr, "--batch-admission requires delivery purpose")
		return request, false, 2
	}
	// Both internal batch proofs execute in a detached worktree holding the
	// exact tree they name, so both direct durable writes back at the control
	// root that owns them. batchPrefixProofControlRoot is what makes that safe:
	// it admits a control root only when the execution root is a linked
	// worktree sharing its git common directory and prefix.
	if request.ControlRoot != "" && !request.BatchPrefixReceipt && !request.BatchTipProof && !request.BatchAdmission {
		fmt.Fprintln(os.Stderr, "--control-root is internal to a batch proof")
		return request, false, 2
	}
	if request.NoReuse && request.Purpose != testpolicy.PurposeDiagnostic {
		fmt.Fprintln(os.Stderr, "--no-reuse is available only for diagnostic purpose")
		return request, false, 2
	}
	if request.FreshEpisode != "" {
		if len(request.FreshEpisode) != 64 {
			fmt.Fprintln(os.Stderr, "--fresh-episode must be a 64-digit hexadecimal identifier")
			return request, false, 2
		}
		if _, err := hex.DecodeString(request.FreshEpisode); err != nil {
			fmt.Fprintln(os.Stderr, "--fresh-episode must be a 64-digit hexadecimal identifier")
			return request, false, 2
		}
	}
	if request.FreshExpiresAt != "" {
		if request.FreshEpisode == "" {
			fmt.Fprintln(os.Stderr, "--fresh-expires-at requires --fresh-episode")
			return request, false, 2
		}
		if _, err := time.Parse(time.RFC3339Nano, request.FreshExpiresAt); err != nil {
			fmt.Fprintln(os.Stderr, "--fresh-expires-at must be an RFC3339 timestamp")
			return request, false, 2
		}
	}
	if request.RequireDiagnosticHeadroom && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(os.Stderr, "--require-diagnostic-headroom is available only for delivery purpose")
		return request, false, 2
	}
	if request.AllGroups && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(os.Stderr, "--all-groups is available only for delivery purpose")
		return request, false, 2
	}
	if (request.ExpectedGoalRevision == 0) != (request.ExpectedAccountingRevision == 0) {
		fmt.Fprintln(os.Stderr, "expected goal and accounting revisions must be supplied together")
		return request, false, 2
	}
	return request, *jsonOutput, 0
}

// The batch transport carries only exact group IDs. The current protected
// policy and prerequisite closure are recomputed by the selector.
func parseBatchRequirements(value string) ([]string, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, fmt.Errorf("expected an object with a groups array")
	}
	key, err := decoder.Token()
	if err != nil || key != "groups" {
		return nil, fmt.Errorf("expected only the groups field")
	}
	var groups []string
	if err := decoder.Decode(&groups); err != nil || groups == nil {
		return nil, fmt.Errorf("groups must be an array of identifiers")
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("unexpected batch requirements field")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing batch requirements content")
	}
	seen := map[string]bool{}
	for _, id := range groups {
		if id == "" || id != strings.TrimSpace(id) || seen[id] {
			return nil, fmt.Errorf("groups must contain unique exact identifiers")
		}
		seen[id] = true
	}
	return groups, nil
}

func batchRequirementsArgument(groups []string) string {
	if groups == nil {
		groups = []string{}
	}
	encoded, _ := json.Marshal(struct {
		Groups []string `json:"groups"`
	}{Groups: groups})
	return string(encoded)
}

const preparationRestartedEnv = "METASYSTEM_PREPARATION_RESTARTED"

type preparationBaseMove struct {
	ours, engine string
}

func (move *preparationBaseMove) Error() string {
	return fmt.Sprintf("the landing ref moved under preparation from %s to %s", move.ours, move.engine)
}

type testingPreparationAttempt func(testingSelectionRequest) (testingPreparation, error)

func prepareTesting(request testingSelectionRequest) (testingPreparation, error) {
	return prepareTestingWith(request, prepareTestingOnce)
}

func prepareTestingWith(request testingSelectionRequest, attempt testingPreparationAttempt) (testingPreparation, error) {
	restarted := os.Getenv(preparationRestartedEnv) == "1"
	for {
		prepared, err := attempt(request)
		var move *preparationBaseMove
		if !errors.As(err, &move) {
			return prepared, err
		}
		if restarted {
			return testingPreparation{}, engineRefusal("base-moved", []enginecause.Fact{
				enginecause.Value("ours", move.ours), enginecause.Value("engine", move.engine), enginecause.Value("restarts", "1"),
			}, "the landing ref moved a second time during one test invocation")
		}
		fmt.Fprintf(os.Stderr, "metasystem test: the landing ref moved under the run (ours=%s engine=%s); restarting preparation once\n", move.ours, move.engine)
		if err := os.Setenv(preparationRestartedEnv, "1"); err != nil {
			return testingPreparation{}, fmt.Errorf("record the one policy-base restart: %w", err)
		}
		restarted = true
	}
}

func prepareTestingOnce(request testingSelectionRequest) (testingPreparation, error) {
	installation, err := canonicalProofRoot(request.Root)
	if err != nil {
		return testingPreparation{}, err
	}
	controlRoot := installation
	if request.ControlRoot != "" {
		controlRoot, err = batchPrefixProofControlRoot(installation, request.ControlRoot)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, present, err := config.ConfLookup(confPath, "testing.contract")
	if err != nil || !present {
		return testingPreparation{}, fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return testingPreparation{}, fmt.Errorf("testing.contract must be a relative normalized path")
	}
	installationWorkspace := gittree.Workspace{Dir: installation}
	projectRoot, err := installationWorkspace.TopLevel()
	if err != nil {
		return testingPreparation{}, err
	}
	prefix, err := installationWorkspace.Prefix()
	if err != nil {
		return testingPreparation{}, err
	}
	workspace := gittree.Workspace{Dir: projectRoot}
	baseCommit, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return testingPreparation{}, fmt.Errorf("testing requires a committed project HEAD")
	}
	goalID := request.GoalID
	accountToGoal, err := testingPreparationAccountsToGoal(request)
	if err != nil {
		return testingPreparation{}, err
	}
	if accountToGoal {
		goalID, err = resolveTestingGoal(installation, request.GoalID)
		if err != nil {
			return testingPreparation{}, err
		}
		// The trusted policy engine must judge the same candidate even when a
		// parent attempt, rather than an explicit flag, supplied its identity.
		request.GoalID = goalID
	}
	// A landed engine is trusted by its landing: when the enrolled engine is
	// behind the landing ref by landed commits only, the run fetches,
	// fast-forwards, rebuilds and re-arms before it judges anything.
	var engineRearm *proofrun.EngineRearm
	_, policyBaseBeforeRearmErr := trustedTestingPolicyBase(projectRoot, workspace)
	// A non-delivery plan remains informative without a configured destination;
	// delivery still enters re-arm so missing landing authority is a refusal.
	if request.LandedRearm && (request.Purpose == testpolicy.PurposeDelivery || policyBaseBeforeRearmErr == nil) {
		namedDeliveryTree := request.Tree != "" && request.Purpose == testpolicy.PurposeDelivery
		rearm, rearmErr := landedRearm(installation, projectRoot, prefix, namedDeliveryTree)
		if rearmErr != nil {
			return testingPreparation{}, rearmErr
		}
		engineRearm = rearm
	}
	policyBaseCommit, policyBaseErr := trustedTestingPolicyBase(projectRoot, workspace)
	if policyBaseErr != nil {
		if request.Purpose == testpolicy.PurposeDelivery {
			return testingPreparation{}, policyBaseErr
		}
		policyBaseCommit = baseCommit
	}
	policyBaseTree, err := workspace.TreeOf(policyBaseCommit)
	if err != nil {
		return testingPreparation{}, err
	}
	indexTree, err := workspace.StagedTree()
	if err != nil {
		return testingPreparation{}, fmt.Errorf("capture real candidate index: %w", err)
	}
	candidateTree := request.Tree
	if candidateTree == "" {
		candidateTree = indexTree
	} else {
		candidateTree, err = workspace.ResolveTree(candidateTree)
	}
	if err != nil {
		return testingPreparation{}, err
	}
	if request.Purpose == testpolicy.PurposeDelivery && candidateTree != indexTree {
		return testingPreparation{}, fmt.Errorf("delivery candidate must equal the real whole-project index: requested=%s index=%s", candidateTree, indexTree)
	}
	treeContractPath := filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), contractRel))
	treeContractPath = strings.TrimPrefix(treeContractPath, "./")
	baseBytes, basePresent, err := workspace.FileAt(policyBaseTree, treeContractPath)
	if err != nil {
		return testingPreparation{}, err
	}
	candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, treeContractPath)
	if err != nil || !candidatePresent {
		return testingPreparation{}, fmt.Errorf("candidate testing contract %s is absent", treeContractPath)
	}
	candidateContract, err := testpolicy.Decode(candidateBytes)
	if err != nil {
		return testingPreparation{}, fmt.Errorf("candidate testing contract: %w", err)
	}
	baseContract := candidateContract
	baseContractDigest := bytesSHA256([]byte("absent\x00" + treeContractPath))
	if basePresent {
		baseContract, err = testpolicy.Decode(baseBytes)
		if err != nil {
			return testingPreparation{}, fmt.Errorf("base testing contract: %w", err)
		}
		baseContractDigest = bytesSHA256(baseBytes)
	}
	policyEngine, policyEngineDigest, currentIsPolicyEngine, err := trustedPolicyEngine(installation, policyBaseCommit, !basePresent)
	if err != nil {
		return testingPreparation{}, err
	}
	workerCapabilitiesChecked := false
	if request.RequireWorkerCapabilities {
		capabilityContext, cancelCapabilities := context.WithCancel(context.Background())
		capabilityErr := requireTestingWorkerCapabilities(capabilityContext, policyEngine, testingEnvironment(os.Environ()))
		cancelCapabilities()
		if capabilityErr != nil {
			return testingPreparation{}, capabilityErr
		}
		workerCapabilitiesChecked = true
	}
	effective := testpolicy.ProtectedContract(baseContract, candidateContract)
	if effective.Fallback == "" {
		effective.Fallback = candidateContract.Fallback
	}
	if err := effective.Validate(); err != nil {
		return testingPreparation{}, fmt.Errorf("protected testing contract: %w", err)
	}
	if basePresent {
		if err := protectCoverageRatchets(workspace, policyBaseTree, candidateTree, prefix); err != nil {
			return testingPreparation{}, err
		}
	}
	mergeBases, err := workspace.MergeBases(baseCommit, policyBaseCommit)
	if err != nil || len(mergeBases) != 1 {
		return testingPreparation{}, fmt.Errorf("testing implementation and destination bases have no unique merge base")
	}
	changeBaseTree, err := workspace.TreeOf(mergeBases[0])
	if err != nil {
		return testingPreparation{}, err
	}
	changedPaths, err := workspace.ChangedPaths(changeBaseTree, candidateTree)
	if err != nil {
		return testingPreparation{}, err
	}
	// Expand protected Go package selectors against the exact trees before
	// policy selection. Each affected package becomes its own reusable group.
	selectionEnvironment := testingEnvironment(os.Environ())
	effective, err = proofrun.ExpandGoPackageGroupsWithEnvironment(effective, projectRoot, changeBaseTree, candidateTree, selectionEnvironment)
	if err != nil {
		return testingPreparation{}, err
	}
	risk, accountingRevision := testpolicy.GoalRisk{}, uint64(0)
	if accountToGoal {
		risk, accountingRevision, err = testingGoalRisk(installation, goalID)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	plan, err := testpolicy.Select(effective, testpolicy.SelectionRequest{ChangedPaths: changedPaths, GoalRisk: risk,
		RequestedMode: request.Mode, Purpose: request.Purpose, Groups: request.Groups})
	if err != nil {
		return testingPreparation{}, err
	}
	if basePresent && !currentIsPolicyEngine {
		basePlan, basePlanErr := planWithTrustedPolicyEngine(policyEngine, trustedPolicyFloorRequest(request), installation, candidateTree)
		if basePlanErr != nil {
			return testingPreparation{}, basePlanErr
		}
		if afterDigest, digestErr := fileSHA256(policyEngine); digestErr != nil || afterDigest != policyEngineDigest {
			return testingPreparation{}, engineRefusal("enrollment-drift", engineCheckoutFacts(installation), "retained trusted-base engine changed during policy selection")
		}
		if mismatch := compareTrustedPolicyDecision(installation, projectRoot, candidateTree, policyBaseCommit, baseContractDigest, basePlan); mismatch != nil {
			return testingPreparation{}, mismatch
		}
		plan = basePlan.Plan
	}
	if len(request.BatchRequirements) > 0 {
		plan, err = testpolicy.WithBatchRequirements(effective, plan, request.BatchRequirements)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	if !basePresent && request.Purpose == testpolicy.PurposeDelivery {
		plan, err = testpolicy.RequireFirstTransition(effective, plan)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	if request.Purpose == testpolicy.PurposeDelivery && len(plan.Uncertainty) > 0 {
		return testingPreparation{}, fmt.Errorf("delivery impact is unresolved: %s; use diagnostic purpose for the bounded unknown groups", strings.Join(plan.Uncertainty, "; "))
	}
	if request.Purpose == testpolicy.PurposeDelivery {
		if err := checkDeliveryInputParity(workspace, candidateTree, prefix, contractRel, effective, plan); err != nil {
			return testingPreparation{}, err
		}
	}
	// Admission is a phase of the fully protected delivery decision. Preserve
	// the complete floor through trusted policy comparison and relevant-input
	// checks, then execute only the contract's admission subset. Legacy v1
	// contracts conservatively retain their whole selected plan here.
	if request.BatchAdmission {
		plan, err = testpolicy.AdmissionPlan(effective, plan)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	unmatchedInputs, err := unmatchedTestingInputs(workspace, candidateTree, effective, plan)
	if err != nil {
		return testingPreparation{}, err
	}
	if policyBaseErr != nil {
		plan.Uncertainty = append(plan.Uncertainty, "trusted destination policy base unavailable: "+policyBaseErr.Error())
	}
	return testingPreparation{Installation: installation, ControlRoot: controlRoot, ProjectRoot: projectRoot, Prefix: strings.TrimSuffix(prefix, "/"),
		GoalID: goalID, AccountingRevision: accountingRevision,
		ConfPath: confPath, BaseCommit: baseCommit, PolicyBaseCommit: policyBaseCommit, CandidateTree: candidateTree, BaseContract: baseContract,
		CandidateContract: candidateContract, EffectiveContract: effective, ContractDigest: bytesSHA256(candidateBytes),
		BaseContractDigest: baseContractDigest, PolicyEngineDigest: policyEngineDigest, PolicyEngine: policyEngine,
		JudgeKey:                  proofrun.ComputeJudgeKey(context.Background(), projectRoot, policyBaseCommit, strings.TrimSuffix(prefix, "/")),
		EngineRearm:               engineRearm,
		WorkerCapabilitiesChecked: workerCapabilitiesChecked,
		UnmatchedInputs:           unmatchedInputs,
		FirstTestingTransition:    !basePresent,
		BehaviorPolicyDigest:      bytesSHA256(behaviorsurface.Bytes()), Plan: plan, Environment: selectionEnvironment,
		AllGroups: request.AllGroups}, nil
}

func testingPreparationAccountsToGoal(request testingSelectionRequest) (bool, error) {
	if request.CadencePreflight {
		if request.Purpose != testpolicy.PurposeCadence {
			return false, fmt.Errorf("cadence preflight requires cadence purpose")
		}
		return false, nil
	}
	return request.GoalID != "" || request.Purpose == testpolicy.PurposeDelivery || request.Purpose == testpolicy.PurposeCadence, nil
}

var prepareTestingForCommand = prepareTesting

func compareTrustedPolicyDecision(installation, projectRoot, candidateTree, policyBaseCommit, baseContractDigest string, decision testingPlanOutput) error {
	seconds := 0
	limit := func() int {
		if seconds == 0 {
			seconds = steward.RearmResolveSeconds(installation)
		}
		return seconds
	}
	return compareTrustedPolicyDecisionWithReaders(candidateTree, policyBaseCommit, baseContractDigest, decision, projectRoot, policyBaseMoveReaders{
		isAncestor: func(root, ours, engine string) (bool, error) {
			_, err := landedRearmGitStep(context.Background(), landedRearmClock, limit(), "compare-moved-policy-base-ancestry", root,
				"merge-base", "--is-ancestor", ours, engine)
			if err != nil {
				var exitError *exec.ExitError
				if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
					return false, nil
				}
				return false, err
			}
			return true, nil
		},
		localLandingRef: func(root string) (string, error) {
			return readLocalLandingRefText(context.Background(), landedRearmClock, limit(), root)
		},
		commitAtRef: func(root, ref string) (string, error) {
			return landedRearmGitStep(context.Background(), landedRearmClock, limit(), "reread-moved-policy-base", root,
				"rev-parse", "--verify", ref+"^{commit}")
		},
	})
}

type policyBaseMoveReaders struct {
	isAncestor      func(projectRoot, ours, engine string) (bool, error)
	localLandingRef func(projectRoot string) (string, error)
	commitAtRef     func(projectRoot, ref string) (string, error)
}

func compareTrustedPolicyDecisionWithReaders(candidateTree, policyBaseCommit, baseContractDigest string, decision testingPlanOutput, projectRoot string, readers policyBaseMoveReaders) error {
	mismatch := decisionMismatchRefusal(candidateTree, policyBaseCommit, baseContractDigest, decision)
	if mismatch == nil {
		return nil
	}
	// CandidateTree cannot be explained by a moved policy base. The base
	// contract digest can: it is read from that base's testing.json.
	if candidateTree != decision.CandidateTree || policyBaseCommit == decision.PolicyBaseCommit {
		return mismatch
	}
	moved, err := authenticatedPolicyBaseMove(projectRoot, policyBaseCommit, decision.PolicyBaseCommit, readers)
	if err != nil || !moved {
		return mismatch
	}
	return &preparationBaseMove{ours: policyBaseCommit, engine: decision.PolicyBaseCommit}
}

func trustedPolicyFloorRequest(request testingSelectionRequest) testingSelectionRequest {
	request.BatchRequirements = nil
	request.BatchPrefixReceipt = false
	return request
}

func authenticatedPolicyBaseMove(projectRoot, ours, engine string, readers policyBaseMoveReaders) (bool, error) {
	ancestor, err := readers.isAncestor(projectRoot, ours, engine)
	if err != nil || !ancestor {
		return false, err
	}
	refText, err := readers.localLandingRef(projectRoot)
	if err != nil {
		return false, err
	}
	ref, _, _, err := parseLandingRefParts(refText)
	if err != nil {
		return false, err
	}
	current, err := readers.commitAtRef(projectRoot, ref)
	if err != nil {
		return false, err
	}
	return current == engine, nil
}

type protectedCoverageBaseline struct {
	Floors map[string]float64 `json:"floors"`
}

func protectCoverageRatchets(workspace gittree.Workspace, baseTree, candidateTree, prefix string) error {
	for _, relative := range []string{"scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"} {
		path := strings.TrimPrefix(filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), relative)), "./")
		baseBytes, basePresent, err := workspace.FileAt(baseTree, path)
		if err != nil || !basePresent {
			continue
		}
		candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, path)
		if err != nil || !candidatePresent {
			return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: protected coverage baseline %s is absent", path)
		}
		var base, candidate protectedCoverageBaseline
		if json.Unmarshal(baseBytes, &base) != nil || json.Unmarshal(candidateBytes, &candidate) != nil || len(base.Floors) == 0 || len(candidate.Floors) == 0 {
			return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: protected coverage baseline %s is malformed", path)
		}
		for packageName, floor := range base.Floors {
			candidateFloor, present := candidate.Floors[packageName]
			if !present || candidateFloor < floor {
				return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: %s floor %s changed from %.1f to %.1f", path, packageName, floor, candidateFloor)
			}
		}
	}
	return nil
}

func trustedPolicyEngine(installation, policyBaseCommit string, firstTransition bool) (string, string, bool, error) {
	current, err := os.Executable()
	if err != nil {
		return "", "", false, err
	}
	engine := current
	if !firstTransition {
		enrollmentRoot := installation
		pinned, openErr := steward.OpenEnrolledBinary(enrollmentRoot)
		if openErr != nil {
			if borrowed, linked := linkedEnrollmentRoot(installation); linked {
				enrollmentRoot = borrowed
				pinned, openErr = steward.OpenEnrolledBinary(enrollmentRoot)
			}
		}
		if openErr != nil {
			return "", "", false, enrollmentRefusal(installation, openErr)
		}
		identity := pinned.Install
		defer pinned.Close()
		if sourceErr := pinned.VerifySourceAtDestination(enrollmentRoot, policyBaseCommit); sourceErr != nil {
			facts := append(engineCheckoutFacts(installation), enginecause.Value("destination", policyBaseCommit))
			return "", "", false, judgmentRefusal(sourceErr, facts, "retained destination engine does not bind the captured policy base")
		}
		if prepareErr := pinned.PrepareForExecution(); prepareErr != nil {
			return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, engineCheckoutFacts(installation), "retain destination engine descriptor: "+prepareErr.Error())
		}
		engine = steward.EnrolledExecutionPath(enrollmentRoot, identity)
	}
	engineInfo, err := os.Stat(engine)
	if err != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode().Perm()&0o111 == 0 {
		facts := append(engineCheckoutFacts(installation), enginecause.Path("engine", engine))
		return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, facts, "retained immutable trusted-base engine is unavailable")
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		facts := append(engineCheckoutFacts(installation), enginecause.Path("engine", engine))
		return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, facts, "hash retained trusted-base engine: "+err.Error())
	}
	currentInfo, currentErr := os.Stat(current)
	return engine, digest, currentErr == nil && os.SameFile(engineInfo, currentInfo), nil
}

func planWithTrustedPolicyEngine(engine string, request testingSelectionRequest, installation, candidateTree string) (testingPlanOutput, error) {
	args := []string{"test", "plan", "--root", installation, "--tree", candidateTree, "--mode", string(request.Mode), "--purpose", string(request.Purpose), "--json", "--policy-child"}
	if request.GoalID != "" {
		args = append(args, "--goal", request.GoalID)
	}
	if len(request.Groups) > 0 {
		args = append(args, "--groups", strings.Join(request.Groups, ","))
	}
	if request.BatchPrefixReceipt {
		args = append(args, "--batch-prefix")
	}
	// No clock on the planning engine: a plan that never returns is ended
	// by the attempt's cancellation, never by a wall bound under load.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, engine, args...)
	command.Env = testingEnvironment(os.Environ())
	if os.Getenv(policyProbeWorkerEnvironment) == "1" {
		command.Env = append(command.Env, policyProbeWorkerEnvironment+"=1")
	}
	data, err := command.CombinedOutput()
	if err != nil {
		return testingPlanOutput{}, engineRefusal("child-failed", []enginecause.Fact{enginecause.Path("engine", engine)}, fmt.Sprintf("retained trusted-base engine could not decide version-1 policy: %v: %s", err, strings.TrimSpace(string(data))))
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var output testingPlanOutput
	if err := decoder.Decode(&output); err != nil {
		return testingPlanOutput{}, engineRefusal("child-output", []enginecause.Fact{enginecause.Path("engine", engine)}, "retained trusted-base engine returned malformed policy output: "+err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return testingPlanOutput{}, engineRefusal("child-output", []enginecause.Fact{enginecause.Path("engine", engine)}, "retained trusted-base engine returned trailing policy output")
	}
	return output, nil
}

func testingRelevantInputs(prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan) ([]string, error) {
	joinPrefix := func(path string) string {
		return strings.TrimPrefix(filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), path)), "./")
	}
	values := map[string]bool{
		joinPrefix(contractRel):       true,
		joinPrefix("metasystem.conf"): true,
	}
	groups := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		groups[group.ID] = group
	}
	for _, id := range plan.SelectedGroups {
		group, ok := groups[id]
		if !ok {
			return nil, fmt.Errorf("selected testing group %s is absent", id)
		}
		for _, input := range group.Inputs {
			values[input] = true
		}
		if group.Adapter == "section" {
			values[joinPrefix("scripts/agents/validate-section-selector.sh")] = true
		}
		if group.Adapter == "command" && len(group.Argv) > 0 && strings.Contains(group.Argv[0], "/") && !filepath.IsAbs(group.Argv[0]) {
			commandPath := filepath.ToSlash(filepath.Clean(filepath.Join(group.CWD, group.Argv[0])))
			if commandPath == ".." || strings.HasPrefix(commandPath, "../") {
				return nil, fmt.Errorf("testing group %s command executable escapes the project", id)
			}
			values[commandPath] = true
		}
	}
	paths := make([]string, 0, len(values))
	for path := range values {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func checkDeliveryInputParity(workspace gittree.Workspace, candidateTree, prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan) error {
	return checkDeliveryInputParityWith(candidateTree, prefix, contractRel, contract, plan, workspace.SnapshotRelevant)
}

func checkDeliveryInputParityWith(candidateTree, prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan, snapshot func(candidateTree string, declarations []string) (string, error)) error {
	declarations, err := testingRelevantInputs(prefix, contractRel, contract, plan)
	if err != nil {
		return err
	}
	workingTree, err := snapshot(candidateTree, declarations)
	if err != nil {
		return fmt.Errorf("capture relevant candidate working inputs: %w", err)
	}
	if candidateTree != workingTree {
		return fmt.Errorf("delivery candidate differs from relevant working-tree inputs: candidate=%s working=%s", candidateTree, workingTree)
	}
	return nil
}

func unmatchedTestingInputs(workspace gittree.Workspace, tree string, contract testpolicy.Contract, plan testpolicy.Plan) ([]testingUnmatchedInput, error) {
	entries, err := workspace.Entries(tree, []string{"."})
	if err != nil {
		return nil, fmt.Errorf("inspect candidate input patterns: %w", err)
	}
	selected := map[string]bool{}
	for _, id := range plan.SelectedGroups {
		selected[id] = true
	}
	var unmatched []testingUnmatchedInput
	for _, group := range contract.Groups {
		if !selected[group.ID] {
			continue
		}
		for _, declaration := range group.Inputs {
			pattern, err := pathpattern.Parse(declaration)
			if err != nil {
				return nil, fmt.Errorf("testing group %s input %q: %w", group.ID, declaration, err)
			}
			found := false
			for name := range entries {
				if pattern.Covers(name) {
					found = true
					break
				}
			}
			if !found {
				unmatched = append(unmatched, testingUnmatchedInput{Group: group.ID, Pattern: pattern.String()})
			}
		}
	}
	return unmatched, nil
}

func printUnmatchedInputs(values []testingUnmatchedInput) {
	for _, item := range values {
		fmt.Fprintf(os.Stderr, "TEST-INPUT-NO-MATCH group=%q pattern=%q\n", item.Group, item.Pattern)
	}
}

func planOutput(prepared testingPreparation) testingPlanOutput {
	groupsByID := map[string]testpolicy.Group{}
	for _, group := range prepared.EffectiveContract.Groups {
		groupsByID[group.ID] = group
	}
	groups := make([]testpolicy.Group, 0, len(prepared.Plan.SelectedGroups))
	for _, id := range prepared.Plan.SelectedGroups {
		groups = append(groups, groupsByID[id])
	}
	return testingPlanOutput{SchemaVersion: 1, ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit, CandidateTree: prepared.CandidateTree, ContractDigest: prepared.ContractDigest,
		BaseContractDigest: prepared.BaseContractDigest, Plan: prepared.Plan, Groups: groups, UnmatchedInputs: prepared.UnmatchedInputs}
}

func testingRunRequest(prepared testingPreparation, attemptID, logRoot, candidateEngine, candidateEngineDigest, candidateEngineBuildIdentity string) proofrun.TestRunRequest {
	request := proofrun.TestRunRequest{ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		ControlRoot:   prepared.proofControlRoot(),
		CandidateTree: prepared.CandidateTree, BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit,
		Contract: prepared.EffectiveContract, Plan: prepared.Plan, AttemptID: attemptID, Environment: prepared.Environment,
		LogRoot: logRoot, ContractDigest: prepared.ContractDigest, BaseContractDigest: prepared.BaseContractDigest,
		PolicyEngineDigest: prepared.PolicyEngineDigest, JudgeKey: prepared.JudgeKey, PolicyEngine: prepared.PolicyEngine, BehaviorPolicyDigest: prepared.BehaviorPolicyDigest,
		EngineRearm:     prepared.EngineRearm,
		CandidateEngine: candidateEngine, CandidateEngineDigest: candidateEngineDigest,
		CandidateEngineBuildIdentity: candidateEngineBuildIdentity, AllGroups: prepared.AllGroups}
	request.Workers, request.AdmissionMaximum = prepared.Workers, prepared.AdmissionMaximum
	return request
}

func resolveTestingPreparationWorkerPolicy(prepared *testingPreparation) (proofRunLimits, error) {
	limits, err := resolveProofRunLimits(prepared.ConfPath)
	if err != nil {
		return proofRunLimits{}, err
	}
	prepared.Workers, prepared.AdmissionMaximum = limits.workers, limits.admissionMaximum
	return limits, nil
}

type candidateEngineBuild struct {
	Path, Digest, Commit string
	QueueDurationMS      int64
	directory            string
}

type candidateEngineCacheRecord struct {
	Version       int    `json:"version"`
	BuildIdentity string `json:"buildIdentity"`
	Digest        string `json:"digest"`
}

type candidateDetachedWorkspace interface {
	Workspace() gittree.Workspace
	Close() error
}

type candidateEngineIO struct {
	runGit func(*exec.Cmd) error
	open   func(gittree.Workspace, string) (candidateDetachedWorkspace, error)
}

func nativeCandidateEngineIO() candidateEngineIO {
	return candidateEngineIO{
		runGit: func(command *exec.Cmd) error { return command.Run() },
		open: func(workspace gittree.Workspace, tree string) (candidateDetachedWorkspace, error) {
			return workspace.NewDetachedWorktree(tree)
		},
	}
}

func selectedCandidateEngineIO(options []candidateEngineIO) candidateEngineIO {
	if len(options) == 0 {
		return nativeCandidateEngineIO()
	}
	return options[0]
}

func candidateEngineBuildEnvironment(environment []string, stamp string) []string {
	owned := map[string]bool{
		"CGO_ENABLED": true, "GOAMD64": true, "GOARM": true, "GOARM64": true,
		"GOENV": true, "GOEXPERIMENT": true, "GOFLAGS": true, "GOTOOLCHAIN": true,
		"GOWORK": true, "METASYSTEM_BUILD_STAMP": true,
	}
	result := make([]string, 0, len(environment)+10)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if !owned[key] {
			result = append(result, entry)
		}
	}
	// Microarchitecture feature levels affect compiled bytes, so proof builds use Go's defaults.
	return append(result, "CGO_ENABLED=0", "GOAMD64=v1", "GOARM64=v8.0", "GOARM=7", "GOENV=off",
		"GOEXPERIMENT=", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOWORK=off", "METASYSTEM_BUILD_STAMP="+stamp)
}

// Proof custody identifies the enclosing run, not the candidate engine's
// compiled behavior. It reaches legacy build scripts but does not fragment
// the cache key across equivalent attempts.
func candidateEngineSemanticEnvironment(environment []string) []string {
	nonsemantic := map[string]bool{
		"METASYSTEM_PROOF_CONTROL_ROOT": true, "METASYSTEM_PROOF_ATTEMPT": true,
		"METASYSTEM_PROOF_RECORD_KEY": true, "METASYSTEM_PROOF_CREATION_CLAIM": true,
		"METASYSTEM_PROOF_AUTH_BIN": true, identity.RunOwnerEnv: true,
		identity.FixtureAttemptEnv: true,
	}
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if ok && !nonsemantic[name] {
			result = append(result, entry)
		}
	}
	return result
}

func (build *candidateEngineBuild) Close() error {
	if build == nil || build.directory == "" {
		return nil
	}
	directory := build.directory
	build.directory = ""
	return os.RemoveAll(directory)
}

// prepareCandidateEngine keeps build outputs under the proof control root.
// The existing build identity covers the tracked engine closure, platform and
// toolchain. Every cache hit also checks the published bytes before use.
func prepareCandidateEngine(ctx context.Context, controlRoot string, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, options ...candidateEngineIO) (*candidateEngineBuild, error) {
	return prepareCandidateEngineWithColdPreflight(ctx, controlRoot, workspace, installationPrefix, candidateTree, environment, nil, selectedCandidateEngineIO(options))
}

func prepareCandidateEngineWithColdPreflight(ctx context.Context, controlRoot string, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, beforeColdBuild func() error, options ...candidateEngineIO) (*candidateEngineBuild, error) {
	io := selectedCandidateEngineIO(options)
	buildIdentity, err := candidateEngineBuildIdentityUsing(ctx, workspace, installationPrefix, candidateTree, environment, io)
	if err != nil {
		return nil, err
	}
	cacheRoot := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines")
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create candidate engine cache: %w", err)
	}
	// The file lock is the identity reservation. A crashed producer releases
	// it through the kernel; a partial staging directory is never a hit.
	lockFile, err := os.OpenFile(filepath.Join(cacheRoot, buildIdentity+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("reserve candidate engine identity: %w", err)
	}
	defer lockFile.Close()
	cacheWaitStarted := time.Now()
	for {
		if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		} else if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			return nil, fmt.Errorf("reserve candidate engine identity: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	cacheWaitMS := time.Since(cacheWaitStarted).Milliseconds()
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
	entry := filepath.Join(cacheRoot, buildIdentity)
	if cached := validatedCandidateEngine(entry, buildIdentity); cached != nil {
		cached.QueueDurationMS = cacheWaitMS
		return cached, nil
	}
	if beforeColdBuild != nil {
		if err := beforeColdBuild(); err != nil {
			return nil, err
		}
	}
	lease, err := proofrun.AcquireHostResources(ctx, controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", nil)
	if err != nil {
		return nil, fmt.Errorf("admit candidate engine build: %w", err)
	}
	defer lease.Close()
	built, err := buildCandidateEngine(proofrun.WithHostResourceLease(ctx, lease), workspace, installationPrefix, candidateTree, environment, io)
	if err != nil {
		return nil, err
	}
	defer built.Close()
	if built.Commit != buildIdentity {
		return nil, fmt.Errorf("candidate engine build identity changed during preparation")
	}
	stage, err := os.MkdirTemp(cacheRoot, ".candidate-engine-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	artifact := filepath.Join(stage, "metasystem")
	if err := copyCandidateEngineArtifact(built.Path, artifact); err != nil {
		return nil, err
	}
	record := candidateEngineCacheRecord{Version: 1, BuildIdentity: buildIdentity, Digest: built.Digest}
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, "record.json"), encoded, 0o600); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(entry); err != nil {
		return nil, fmt.Errorf("discard invalid candidate engine artifact: %w", err)
	}
	if err := os.Rename(stage, entry); err != nil {
		return nil, fmt.Errorf("publish candidate engine artifact: %w", err)
	}
	if cached := validatedCandidateEngine(entry, buildIdentity); cached != nil {
		cached.QueueDurationMS = cacheWaitMS + lease.Waited().Milliseconds()
		return cached, nil
	}
	return nil, fmt.Errorf("published candidate engine artifact failed validation")
}

func validatedCandidateEngine(entry, buildIdentity string) *candidateEngineBuild {
	var record candidateEngineCacheRecord
	encoded, err := os.ReadFile(filepath.Join(entry, "record.json"))
	if err != nil || json.Unmarshal(encoded, &record) != nil || record.Version != 1 ||
		record.BuildIdentity != buildIdentity || len(record.Digest) != sha256.Size*2 {
		return nil
	}
	path := filepath.Join(entry, "metasystem")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil
	}
	digest, err := fileSHA256(path)
	if err != nil || digest != record.Digest {
		return nil
	}
	return &candidateEngineBuild{Path: path, Digest: digest, Commit: buildIdentity}
}

func copyCandidateEngineArtifact(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o500)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

// buildCandidateEngine materializes the exact project tree, stamps its
// synthetic candidate commit into one proof build, and leaves bin/metasystem
// untouched. The output survives worktree cleanup for the authenticated
// worker and every detached group it launches.
func buildCandidateEngine(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, options ...candidateEngineIO) (*candidateEngineBuild, error) {
	io := selectedCandidateEngineIO(options)
	detached, err := io.open(workspace, candidateTree)
	if err != nil {
		return nil, fmt.Errorf("candidate engine build failed while materializing tree %s: %w", candidateTree, err)
	}
	removeDetached := func() error { return detached.Close() }
	candidateCommit, err := bindMaterializedCandidateCommit(ctx, detached.Workspace(), installationPrefix, environment, io)
	if err != nil {
		closeErr := removeDetached()
		return nil, fmt.Errorf("candidate engine build failed while resolving the materialized candidate commit: %v (cleanup: %v)", err, closeErr)
	}
	directory, err := os.MkdirTemp("", "metasystem-candidate-engine.*")
	if err != nil {
		closeErr := removeDetached()
		return nil, fmt.Errorf("candidate engine build failed while allocating its private output: %v (cleanup: %v)", err, closeErr)
	}
	build := &candidateEngineBuild{Path: filepath.Join(directory, "metasystem"), Commit: candidateCommit, directory: directory}
	fail := func(cause error, output []byte) (*candidateEngineBuild, error) {
		closeErr := removeDetached()
		removeErr := build.Close()
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			cause = fmt.Errorf("%w: %s", cause, detail)
		}
		if closeErr != nil || removeErr != nil {
			cause = fmt.Errorf("%w (cleanup: worktree=%v output=%v)", cause, closeErr, removeErr)
		}
		return nil, fmt.Errorf("candidate engine build failed at commit %s through scripts/agents/go-build.sh --trimpath --out: %w", candidateCommit, cause)
	}
	installationRoot := filepath.Join(detached.Workspace().Dir, filepath.FromSlash(installationPrefix))
	command := exec.CommandContext(ctx, "bash", "scripts/agents/go-build.sh", "--trimpath", "--out", build.Path)
	command.Dir = installationRoot
	command.Env = candidateEngineBuildEnvironment(environment, candidateCommit)
	proofrun.AttachHostResourceLease(ctx, command)
	command.WaitDelay = 5 * time.Second
	var combined bytes.Buffer
	command.Stdout, command.Stderr = &combined, &combined
	commandErr := proofrun.RunResourceCommand(ctx, command, proofrun.HostResourceLeaseFromContext(ctx))
	output := combined.Bytes()
	if commandErr != nil {
		return fail(commandErr, output)
	}
	if err := removeDetached(); err != nil {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed while cleaning its materialized worktree: %w", err)
	}
	info, err := os.Stat(build.Path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed: scripts/agents/go-build.sh did not produce a regular executable: %v", err)
	}
	build.Digest, err = fileSHA256(build.Path)
	if err != nil {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed while hashing its proof output: %w", err)
	}
	return build, nil
}

func candidateEngineBuildIdentity(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string) (string, error) {
	return candidateEngineBuildIdentityUsing(ctx, workspace, installationPrefix, candidateTree, environment, nativeCandidateEngineIO())
}

func candidateEngineBuildIdentityUsing(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, io candidateEngineIO) (string, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return "", err
	}
	installationPrefix = strings.Trim(filepath.ToSlash(installationPrefix), "/")
	installationTree := candidateTree
	if installationPrefix != "" {
		installationTree, err = workspace.ResolveTree(candidateTree + ":" + installationPrefix)
		if err != nil {
			return "", fmt.Errorf("resolve candidate installation subtree: %w", err)
		}
	}
	engineTree, err := engineProjectionTree(ctx, workspace.Dir, installationTree, policy.EnginePaths, io)
	if err != nil {
		return "", err
	}
	buildEnvironment := candidateEngineBuildEnvironment(candidateEngineSemanticEnvironment(environment), "")
	// The build script receives this explicit environment. Its tracked bytes
	// can consume any supplied value, so the build key covers all of them.
	environmentDigest := bytesSHA256([]byte(strings.Join(buildEnvironment, "\x00")))
	installationRoot := workspace.Dir
	if installationPrefix != "" {
		installationRoot = filepath.Join(workspace.Dir, filepath.FromSlash(installationPrefix))
	}
	toolchainClosure, err := proofrun.ToolchainClosureIdentity(installationRoot, buildEnvironment)
	if err != nil {
		return "", fmt.Errorf("candidate engine toolchain closure: %w", err)
	}
	message := strings.Join([]string{
		"stable candidate proof snapshot",
		"engine-tree=" + engineTree,
		"toolchain-closure=" + toolchainClosure,
		"build-environment=" + environmentDigest,
		"platform=" + runtime.GOOS + "/" + runtime.GOARCH,
		"build-context=CGO_ENABLED=0,GOAMD64=v1,GOARM64=v8.0,GOARM=7,GOENV=off,GOEXPERIMENT=,GOFLAGS=-mod=readonly,GOTOOLCHAIN=local,GOWORK=off,-buildvcs=false,-trimpath",
	}, "\n")
	return commitCandidateEngineTree(ctx, workspace.Dir, engineTree, message, io)
}

func engineProjectionTree(ctx context.Context, root, tree string, paths []string, io candidateEngineIO) (string, error) {
	directory, err := os.MkdirTemp("", "metasystem-engine-projection.*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	run := func(index string, stdin []byte, args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.fileMode=true", "-c", "core.useReplaceRefs=false"}, args...)...)
		command.Env = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + index)
		command.Stdin = bytes.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		if err := io.runGit(command); err != nil {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}
	sourceIndex := filepath.Join(directory, "source-index")
	targetIndex := filepath.Join(directory, "target-index")
	if _, err := run(sourceIndex, nil, "read-tree", tree); err != nil {
		return "", fmt.Errorf("seed candidate engine projection: %w", err)
	}
	entries, err := run(sourceIndex, nil, append([]string{"ls-files", "-s", "-z", "--"}, paths...)...)
	if err != nil {
		return "", fmt.Errorf("enumerate candidate engine projection: %w", err)
	}
	if _, err := run(targetIndex, nil, "read-tree", "--empty"); err != nil {
		return "", fmt.Errorf("initialize candidate engine projection: %w", err)
	}
	if len(entries) > 0 {
		if _, err := run(targetIndex, entries, "update-index", "-z", "--index-info"); err != nil {
			return "", fmt.Errorf("write candidate engine projection: %w", err)
		}
	}
	output, err := run(targetIndex, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write candidate engine tree: %w", err)
	}
	engineTree := strings.TrimSpace(string(output))
	if len(engineTree) != 40 && len(engineTree) != 64 {
		return "", fmt.Errorf("candidate engine projection returned invalid tree %q", engineTree)
	}
	return engineTree, nil
}

func commitCandidateEngineTree(ctx context.Context, root, tree, message string, io candidateEngineIO) (string, error) {
	gitLine := func(environment []string, args ...string) (string, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		command.Env = environment
		var combined bytes.Buffer
		command.Stdout, command.Stderr = &combined, &combined
		err := io.runGit(command)
		output := combined.Bytes()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return strings.TrimSpace(string(output)), nil
	}
	environment := gittree.ScrubbedEnviron()
	commitEnvironment := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
			"GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE":
			continue
		}
		commitEnvironment = append(commitEnvironment, entry)
	}
	commitEnvironment = append(commitEnvironment,
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	commit, err := gitLine(commitEnvironment, "-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid",
		"-c", "author.name=MetaSystem", "-c", "author.email=metasystem@invalid",
		"-c", "committer.name=MetaSystem", "-c", "committer.email=metasystem@invalid",
		"-c", "i18n.commitEncoding=UTF-8", "commit-tree", tree, "-m", message)
	if err != nil {
		return "", err
	}
	return commit, nil
}

func bindMaterializedCandidateCommit(ctx context.Context, workspace gittree.Workspace, installationPrefix string, environment []string, io candidateEngineIO) (string, error) {
	root := workspace.Dir
	tree, err := workspace.HeadTree()
	if err != nil {
		return "", err
	}
	commit, err := candidateEngineBuildIdentityUsing(ctx, workspace, installationPrefix, tree, environment, io)
	if err != nil {
		return "", err
	}
	current, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return "", fmt.Errorf("resolve temporary candidate HEAD: %v", err)
	}
	command := exec.CommandContext(ctx, "git", "-C", root, "update-ref", "--no-deref", "HEAD", commit, current)
	command.Env = gittree.ScrubbedEnviron()
	var combined bytes.Buffer
	command.Stdout, command.Stderr = &combined, &combined
	if err := io.runGit(command); err != nil {
		output := combined.Bytes()
		return "", fmt.Errorf("git update-ref --no-deref HEAD: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return commit, nil
}

func runTestRun(args []string) int {
	commandStarted := time.Now().UTC()
	request, _, status := parseTestingSelection("test run", args, true)
	if status != 0 {
		return status
	}
	request.LandedRearm = true
	request.RequireWorkerCapabilities = true
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		if errors.Is(err, errTestingWorkerPolicyUnsupported) {
			return proofrun.ExitAdmissionRefused
		}
		return 1
	}
	printUnmatchedInputs(prepared.UnmatchedInputs)
	controlRoot := prepared.proofControlRoot()
	commandClock, fixtureClock, err := goalCommandClock(controlRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return proofrun.ExitAdmissionRefused
	}
	semanticCommandStarted := commandClock()
	commandAdmission := testingCommandAdmission{now: commandClock, admitRun: admitTestingRun, admitProof: admitProofLaunch}
	limits, err := resolveTestingPreparationWorkerPolicy(&prepared)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	engine, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	workerEngine := engine
	if !prepared.FirstTestingTransition {
		workerEngine = prepared.PolicyEngine
	}
	if !prepared.WorkerCapabilitiesChecked {
		capabilityContext, cancelCapabilities := context.WithCancel(context.Background())
		capabilityErr := requireTestingWorkerCapabilities(capabilityContext, workerEngine, prepared.Environment)
		cancelCapabilities()
		if capabilityErr != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run:", capabilityErr)
			return proofrun.ExitAdmissionRefused
		}
	}
	unmark, markErr := proofrun.MarkManagedProofProcess()
	if markErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: mark host admission process:", markErr)
		return proofrun.ExitAdmissionRefused
	}
	defer unmark()
	buildContext, cancelBuild := context.WithCancel(context.Background())
	candidateEngine, err := prepareCandidateEngineWithColdPreflight(buildContext, controlRoot, gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.Prefix,
		prepared.CandidateTree, inheritedTestingEnvironment(prepared.Environment, os.Environ()), func() error {
			return refuseKnownColdBuildBudget(prepared, request)
		})
	cancelBuild()
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		var budgetRefusal *coldBuildBudgetRefusal
		if errors.As(err, &budgetRefusal) {
			return proofrun.ExitAdmissionRefused
		}
		return 1
	}
	defer candidateEngine.Close()
	planDigest := proofrun.TestPlanDigest(prepared.EffectiveContract, prepared.Plan, prepared.CandidateTree)
	manifestDigest, err := testingCandidateManifest(gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.CandidateTree)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: capture candidate manifest:", err)
		return proofrun.ExitAdmissionRefused
	}
	preRequest := testingRunRequest(prepared, "", "", candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	metadataContext, cancelMetadata := context.WithCancel(context.Background())
	metadataLease, leaseErr := proofrun.AcquireHostResources(metadataContext, controlRoot, prepared.ConfPath, "heavy", nil)
	if leaseErr != nil {
		cancelMetadata()
		fmt.Fprintln(os.Stderr, "metasystem test run: admit testing metadata preparation:", leaseErr)
		return proofrun.ExitAdmissionRefused
	}
	queueDurationMS := candidateEngine.QueueDurationMS + metadataLease.Waited().Milliseconds()
	metadataStarted := time.Now()
	identities, preparedGroups, preparationLaunches, identityErr := proofrun.PrepareGroupExecutionIdentities(
		proofrun.WithHostResourceLease(metadataContext, metadataLease), preRequest)
	closeLeaseErr := metadataLease.Close()
	cancelMetadata()
	if identityErr == nil {
		identityErr = closeLeaseErr
	}
	preparationDuration := time.Since(metadataStarted).Milliseconds()
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS = preparationDuration
	preRequest.QueueDurationMS = queueDurationMS
	preRequest.CommandStartedAt = commandStarted.Format(time.RFC3339Nano)
	var identityInputs []string
	if identityErr == nil {
		for _, id := range prepared.Plan.SelectedGroups {
			identityInputs = append(identityInputs, "group:"+id+":"+identities[id])
		}
	} else {
		identityInputs = append(identityInputs, "group-identity-unavailable:"+identityErr.Error())
	}
	if identityErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: bounded testing metadata preparation:", identityErr)
		return proofrun.ExitAdmissionRefused
	}
	freshGroups, maxFreshAge := testingFreshGroups(prepared, request)
	if request.FreshEpisode == "" && len(freshGroups) != 0 {
		request.FreshEpisode, err = newTestingFreshEpisode()
		if err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: create freshness episode:", err)
			return proofrun.ExitAdmissionRefused
		}
		if maxFreshAge > 0 && request.FreshExpiresAt == "" {
			request.FreshExpiresAt = semanticCommandStarted.Add(maxFreshAge).Format(time.RFC3339Nano)
		}
	}
	if maxFreshAge > 0 && request.FreshExpiresAt == "" {
		fmt.Fprintln(os.Stderr, "metasystem test run: selected fresh group requires --fresh-expires-at")
		return proofrun.ExitAdmissionRefused
	}
	preRequest.FreshnessEpisode, preRequest.FreshnessExpiresAt = request.FreshEpisode, request.FreshExpiresAt
	preRequest.FreshGroups = freshGroups
	if err := bindTestingFreshnessProjection(&preRequest, prepared.Installation); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: project freshness candidate:", err)
		return proofrun.ExitAdmissionRefused
	}
	freshnessProjection := preRequest.FreshnessCandidateProjection
	freshBinding := testingFreshnessBinding(preRequest, identities, request.FreshEpisode)
	preRequest.FreshnessBinding = freshBinding
	admission := proofLaunchAdmission{ControlRoot: controlRoot,
		ExecutionRoot: prepared.ProjectRoot, ConfPath: prepared.ConfPath, GoalID: request.GoalID, AuthorityGoalID: request.AuthorityGoalID,
		CandidateRevision: prepared.AccountingRevision, RetryDecision: request.RetryDecision,
		CapMin: request.CapMin, ExpectedGoalRevision: request.ExpectedGoalRevision,
		ExpectedAccountingRevision: request.ExpectedAccountingRevision,
		ScopeClass:                 "selected", CommandClass: "testing", CandidateTree: prepared.CandidateTree, IdentityInputs: append([]string{prepared.ContractDigest,
			prepared.BaseContractDigest, prepared.PolicyEngineDigest, candidateEngine.Digest, prepared.BehaviorPolicyDigest, planDigest}, identityInputs...), Environment: prepared.Environment,
		SharedEngine: engine, SharedManifestDigest: manifestDigest, ComponentIdentities: identities,
		FreshnessEpisode: request.FreshEpisode, FreshnessBinding: freshBinding, FreshnessExpiresAt: request.FreshExpiresAt,
		FreshGroups:     freshGroups,
		ForceGroups:     request.ForceGroups,
		ManagedCapacity: true,
		// A cadence attempt is the fresh sweep: it never inherits a
		// reusable-success answer from an earlier run of its goal.
		RequireDiagnosticHeadroom: request.RequireDiagnosticHeadroom}
	attempt, decision, joined, err := commandAdmission.initial(request, admission)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition == proofrun.DispositionReusableSuccess {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		if readErr != nil || identityErr != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: reusable component evidence is unreadable")
			return 1
		}
		template := proofrun.NewTestResultAt(preRequest, commandClock())
		projection, exact := proofrun.ExactReusableTestResult(template, attempts, identities, prepared.GoalID, prepared.AccountingRevision)
		if !exact {
			projection = proofrun.ReusedTestResult(template, attempts, identities, prepared.EffectiveContract)
		}
		if projection.Delivery.Sufficient {
			if err := publishTestingResult(controlRoot, request.ResultPath, projection); err != nil {
				fmt.Fprintln(os.Stderr, "metasystem test run:", err)
				return 1
			}
			return decision.ExitStatus
		}
		// The goal-scoped admission saw only this goal's successes; the seat's
		// newest observations say otherwise (a newer failure or a live plan
		// under another goal). Run afresh instead of stranding the caller.
		attempt, decision, joined, err = commandAdmission.forced(admission)
		if err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run:", err)
			return proofrun.ExitAdmissionRefused
		}
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Reason != "" {
			fmt.Fprintln(os.Stderr, decision.Reason)
		}
		if err := proofrun.EncodeResult(os.Stdout, request.ResultPath, decision); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: publish no-child result:", err)
			return 1
		}
		return decision.ExitStatus
	}
	prepared.GoalID, prepared.AccountingRevision = attempt.AccountedGoal(), attempt.AccountedRevision()
	preRequest = testingRunRequest(prepared, "", "", candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	preRequest.FreshnessEpisode, preRequest.FreshnessBinding, preRequest.FreshnessExpiresAt = request.FreshEpisode, freshBinding, request.FreshExpiresAt
	preRequest.FreshGroups = freshGroups
	preRequest.FreshnessCandidateProjection = freshnessProjection
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS, preRequest.CommandStartedAt = preparationDuration, commandStarted.Format(time.RFC3339Nano)
	preRequest.QueueDurationMS = queueDurationMS
	reusedGroups := map[string]proofrun.GroupResult{}
	if identityErr == nil {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: read reusable component evidence:", readErr)
			return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
		}
		reused := proofrun.ReusedTestResultExcludingWithPolicy(proofrun.NewTestResultAt(preRequest, commandClock()), attempts, identities,
			prepared.EffectiveContract, attempt.AttemptID, proofrun.ReusePolicy{ForceGroups: request.ForceGroups})
		for _, group := range reused.Groups {
			if group.Status == "reused" {
				reusedGroups[group.ID] = group
			}
		}
	}
	pathsRoot := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", attempt.AttemptID, "testing", planDigest)
	if err := os.MkdirAll(pathsRoot, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
	}
	runRequest := testingRunRequest(prepared, attempt.AttemptID, filepath.Join(pathsRoot, "groups"), candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	runRequest.FreshnessEpisode, runRequest.FreshnessBinding, runRequest.FreshnessExpiresAt = request.FreshEpisode, freshBinding, request.FreshExpiresAt
	runRequest.FreshGroups = freshGroups
	runRequest.FreshnessCandidateProjection = freshnessProjection
	runRequest.ProgressPath = filepath.Join(pathsRoot, "progress.jsonl")
	runRequest.Reused, runRequest.ComponentIdentities = reusedGroups, identities
	runRequest.PreparedGroups, runRequest.PreparationLaunches = preparedGroups, preparationLaunches
	runRequest.PreparationDurationMS, runRequest.CommandStartedAt = preparationDuration, preRequest.CommandStartedAt
	runRequest.QueueDurationMS = queueDurationMS
	runRequest.EvidenceTimeoutMS, runRequest.EvidenceMaxBytes = limits.evidenceTimeout.Milliseconds(), limits.evidenceMax
	runRequest.Concurrency = limits.concurrency
	packetPath, workerResultPath := filepath.Join(pathsRoot, "request.json"), filepath.Join(pathsRoot, "result.json")
	deadline, deadlineCheck, err := proofDeadline(attempt.Deadline, commandClock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: admit native testing:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
	}
	nativeContext, cancelNative := proofDeadlineContext(context.Background(), deadline, fixtureClock)
	defer cancelNative()
	var retained *proofrun.TestResult
	workerEnvironment, err := testingWorkerEnvironment(prepared.Environment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: export run owner:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
	}
	workerEnvironment, err = authorizedFixtureClockEnvironment(controlRoot, workerEnvironment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: export fixture clock:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
	}
	workerEnvironment = resolvedTestWorkerEnvironment(workerEnvironment, limits.workers)
	producerWaitStarted := time.Now()
	for _, id := range prepared.Plan.SelectedGroups {
		if attempt.TestWaits[id] == "" {
			continue
		}
		if _, err := proofrun.WaitForTestProducerWithWaitCheck(nativeContext, controlRoot, attempt, id, deadlineCheck); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: await shared producer:", err)
			return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
		}
	}
	runRequest.QueueDurationMS += time.Since(producerWaitStarted).Milliseconds()
	resourceClass, exclusive := testingOwnedResources(prepared, attempt)
	var nativeLease *proofrun.HostResourceLease
	if resourceClass != "" {
		nativeLease, err = proofrun.AcquireHostResourcesWithWaitCheck(nativeContext, controlRoot, prepared.ConfPath, resourceClass, exclusive, deadlineCheck)
		if err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: admit native testing:", err)
			return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
		}
		defer nativeLease.Close()
		runRequest.QueueDurationMS += nativeLease.Waited().Milliseconds()
	}
	if err := writePrivateJSON(packetPath, runRequest); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
	}
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, 1)
	}
	launchStatus := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "testing", Root: prepared.ProjectRoot,
		ControlRoot: controlRoot, AttemptID: attempt.AttemptID, JoinedAttempt: joined, Deadline: deadline, ConfPath: prepared.ConfPath,
		ProgressPath: runRequest.ProgressPath, LogPath: filepath.Join(pathsRoot, "launcher.log"),
		Banner: "TESTING-CONTRACT plan=" + planDigest, Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Second, TermGrace: 5 * time.Second,
		KillGrace: time.Second, Command: []string{workerEngine, "test", "worker", "--packet", packetPath, "--packet-sha256", packetDigest, "--result", workerResultPath},
		Environment: workerEnvironment, HostResourceFiles: nativeLease.Files(), RequireCustody: true, Output: os.Stdout, ErrorOutput: os.Stderr,
		Now: commandClock,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			result, readErr := readTestingWorkerResult(workerResultPath)
			if readErr != nil {
				return nil, readErr
			}
			retained = &result
			if request.BatchPrefixReceipt || request.BatchAdmission || !testingReceiptWanted(joined, prepared.Plan.Purpose, result.Delivery.Sufficient) {
				return nil, nil
			}
			receipt, payload, prepareErr := landing.PrepareTestingReceiptPayload(prepared.Installation,
				prepared.CandidateTree, result, completion.CompletedAt)
			if prepareErr == nil && receipt.Testing != nil {
				updated := *receipt.Testing
				retained = &updated
			}
			return payload, prepareErr
		},
		CommitTerminal: testingTerminalCommit(workerResultPath, &retained)})
	launchStatus = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, launchStatus)
	if retained == nil {
		if result, readErr := readTestingWorkerResult(workerResultPath); readErr == nil {
			retained = &result
		}
	}
	if retained != nil {
		if joined {
			if _, err := proofrun.RecordTestResultAt(controlRoot, attempt.AttemptID, *retained, commandClock()); err != nil {
				fmt.Fprintln(os.Stderr, "metasystem test run: retain joined result:", err)
				return 1
			}
		}
		if err := publishTestingResult(controlRoot, request.ResultPath, *retained); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run:", err)
			return 1
		}
	}
	return launchStatus
}

func newTestingFreshEpisode() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func testingFreshGroups(prepared testingPreparation, request testingSelectionRequest) (map[string]bool, time.Duration) {
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

func testingOwnedResources(prepared testingPreparation, attempt proofrun.Attempt) (string, []string) {
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

func bindTestingFreshnessProjection(request *proofrun.TestRunRequest, installation string) error {
	if request.FreshnessEpisode == "" {
		return nil
	}
	tree, err := landing.ProjectWorkspaceTree(installation, request.CandidateTree)
	if err != nil {
		return err
	}
	request.FreshnessCandidateProjection = tree
	return nil
}

// A supplied episode is reusable only while the decision it names still
// covers the same candidate, base, plan, and execution inputs.
func testingFreshnessBinding(request proofrun.TestRunRequest, identities map[string]string, episode string) string {
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

func testingWorkerEnvironment(environment []string) ([]string, error) {
	return identity.ExportRunOwner(environment)
}

func testingCandidateManifest(workspace gittree.Workspace, candidateTree string) (string, error) {
	detached, err := workspace.NewDetachedWorktree(candidateTree)
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

func runTestWorker(args []string) int {
	flags := flag.NewFlagSet("test worker", flag.ContinueOnError)
	packet := flags.String("packet", "", "private testing request")
	packetDigest := flags.String("packet-sha256", "", "SHA-256 identity of the immutable testing request")
	resultPath := flags.String("result", "", "private testing result")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *packet == "" || *packetDigest == "" || *resultPath == "" {
		return 2
	}
	actualPacketDigest, err := fileSHA256(*packet)
	if err != nil || actualPacketDigest != *packetDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: immutable request identity mismatch")
		return 3
	}
	var request proofrun.TestRunRequest
	if err := readStrictJSON(*packet, &request); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 2
	}
	legacyPolicyProbe := os.Getenv(policyProbeWorkerEnvironment) == "1"
	if legacyPolicyProbe {
		refusal := frozenPolicyProbeRefusal(request, *resultPath)
		if refusal != "" {
			fmt.Fprintln(os.Stderr, "metasystem test worker: unrecognized frozen policy probe:", refusal)
			return 3
		}
		request.SyntheticProbe = true
	}
	if request.CandidateEngine == "" || request.CandidateEngineDigest == "" ||
		(request.CandidateEngineBuildIdentity == "" && !legacyPolicyProbe) {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound candidate engine is absent")
		return 3
	}
	policyDigest, policyDigestErr := fileSHA256(request.PolicyEngine)
	if request.PolicyEngine == "" || policyDigestErr != nil || policyDigest != request.PolicyEngineDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound policy engine changed")
		return 3
	}
	engineInfo, statErr := os.Stat(request.CandidateEngine)
	engineDigest, digestErr := fileSHA256(request.CandidateEngine)
	if statErr != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode()&0o111 == 0 || digestErr != nil || engineDigest != request.CandidateEngineDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound candidate engine changed")
		return 3
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil || canonicalControl == "" || attemptID == "" || attemptID != request.AttemptID {
		fmt.Fprintln(os.Stderr, "metasystem test worker: attempt-bound proof locator mismatch")
		return 3
	}
	if err := proofrun.AuthenticateWorker(canonicalControl, attemptID, os.Getenv("METASYSTEM_PROOF_RECORD_KEY"),
		os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid())); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 3
	}
	attempt, err := proofrun.ReadAttempt(canonicalControl, attemptID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 3
	}
	packetControl, controlErr := canonicalProofRoot(request.ControlRoot)
	packetProject, projectErr := canonicalProofRoot(request.ProjectRoot)
	admittedControl, admittedControlErr := canonicalProofRoot(attempt.ControlRoot)
	admittedProject, admittedProjectErr := canonicalProofRoot(attempt.ExecutionRoot)
	if controlErr != nil || projectErr != nil || admittedControlErr != nil || admittedProjectErr != nil ||
		packetControl != canonicalControl || admittedControl != canonicalControl ||
		(!legacyPolicyProbe && packetProject != admittedProject) {
		fmt.Fprintln(os.Stderr, "metasystem test worker: authenticated request roots do not match the worker packet")
		return 3
	}
	request.ControlRoot = canonicalControl
	if legacyPolicyProbe {
		request.ProjectRoot = packetProject
	} else {
		request.ProjectRoot = admittedProject
	}
	if _, err := time.Parse(time.RFC3339Nano, attempt.Deadline); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker: admitted deadline is invalid")
		return 3
	}
	request.Environment = inheritedTestingEnvironment(request.Environment, os.Environ())
	request.Environment = proofrun.TestingEnvironment(request.Environment, map[string]string{
		proofWitnessExecutionRootEnv: attempt.ExecutionRoot,
	})
	// The worker's context carries no deadline: the reservation is a
	// figure, not a kill rule (proof-groups-detect-hangs-by-progress-not-
	// the-clock, decision 3). A recorded cancellation intent cancels it.
	workerContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cancelOnRecordedIntent(workerContext, cancel, canonicalControl, attemptID)
	if err := runFrozenPolicyProtectionCorpus(workerContext, request); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 1
	}
	result, status, runErr := proofrun.RunTestPlan(workerContext, request)
	response := result
	if runErr == nil && request.SyntheticProbe {
		response, err = frozenNegativeProbeResponse(request, result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
			return 1
		}
	}
	if runErr != nil {
		if err := proofrun.ValidateTestResult(response); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test worker:", runErr)
			fmt.Fprintln(os.Stderr, "metasystem test worker: operational result was not retained:", err)
			return 1
		}
	}
	if err := writePrivateJSON(*resultPath, response); err != nil {
		if runErr != nil {
			fmt.Fprintln(os.Stderr, "metasystem test worker:", runErr)
		}
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 1
	}
	printTestingSummary(result)
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", runErr)
		return 1
	}
	return status
}

func frozenPolicyProbeRefusal(request proofrun.TestRunRequest, resultPath string) string {
	switch {
	case request.Contract.SchemaVersion != 1:
		return fmt.Sprintf("contract schema=%d, want 1", request.Contract.SchemaVersion)
	case len(request.Contract.Groups) != 1:
		return fmt.Sprintf("contract group count=%d, want 1", len(request.Contract.Groups))
	case request.Contract.Groups[0].ID != "literal":
		return fmt.Sprintf("contract group=%q, want literal", request.Contract.Groups[0].ID)
	case len(request.Plan.SelectedGroups) != 1:
		return fmt.Sprintf("selected group count=%d, want 1", len(request.Plan.SelectedGroups))
	case request.Plan.SelectedGroups[0] != "literal":
		return fmt.Sprintf("selected group=%q, want literal", request.Plan.SelectedGroups[0])
	case !strings.HasPrefix(filepath.Base(request.ProjectRoot), "metasystem-policy-probe."):
		return fmt.Sprintf("project root base=%q lacks metasystem-policy-probe prefix", filepath.Base(request.ProjectRoot))
	case filepath.Dir(resultPath) != request.ProjectRoot:
		return fmt.Sprintf("result directory=%q differs from project root=%q", filepath.Dir(resultPath), request.ProjectRoot)
	default:
		return ""
	}
}

func runTestVerify(args []string) int {
	request, jsonOutput, status := parseTestingSelection("test verify", args, false)
	if status != 0 {
		return status
	}
	if request.Tree == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test verify --root INSTALLATION [--goal ID] --tree TREE [--json]")
		return 2
	}
	result, err := verifyRetainedTesting(request)
	if err != nil {
		printMovedProofInputsWithoutCandidateEngine(request)
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	if jsonOutput {
		printJSON(result)
	} else {
		printTestingSummary(result)
	}
	if !result.Delivery.Sufficient {
		printMovedProofInputs(request, result)
		fmt.Fprintf(os.Stderr, "missing required proof; run metasystem test run --root %s --goal %s --tree %s --mode auto; missing groups: %s\n",
			request.Root, request.GoalID, result.CandidateTree, strings.Join(result.Delivery.MissingGroups, ","))
		return 1
	}
	return 0
}

// verifyRetainedTesting composes the retained proof that covers request.Tree
// (the index when empty) for the request's goal and accounting revision. It
// launches nothing and creates no attempt. The result's per-group
// ExecutionIdentity is the identity on the current tree.
func verifyRetainedTesting(request testingSelectionRequest) (proofrun.TestResult, error) {
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	commandClock, _, err := goalCommandClock(prepared.proofControlRoot())
	if err != nil {
		return proofrun.TestResult{}, err
	}
	return verifyRetainedTestingPrepared(request, prepared, retainedTestingVerification{
		clock: commandClock, revalidate: proofrun.RevalidateRetainedGroupExecutionIdentities,
	})
}

type retainedTestingVerification struct {
	clock      func() time.Time
	revalidate func(context.Context, proofrun.TestRunRequest, []proofrun.Attempt) (map[string]string, error)
}

func verifyRetainedTestingPrepared(request testingSelectionRequest, prepared testingPreparation, dependencies retainedTestingVerification) (proofrun.TestResult, error) {
	initialFreshnessCheckAt := dependencies.clock()
	if request.FreshExpiresAt != "" {
		expires, parseErr := time.Parse(time.RFC3339Nano, request.FreshExpiresAt)
		if parseErr != nil || !expires.After(initialFreshnessCheckAt) {
			return proofrun.TestResult{}, fmt.Errorf("freshness episode has expired; renew the proof decision")
		}
	}
	// The limits are read for their validity; the retained-result checks
	// below run under no clock (proof-groups-detect-hangs-by-progress-not-
	// the-clock, slice 2).
	if _, err := resolveTestingPreparationWorkerPolicy(&prepared); err != nil {
		return proofrun.TestResult{}, err
	}
	attempts, err := proofrun.ReadAttempts(prepared.proofControlRoot())
	if err != nil {
		return proofrun.TestResult{}, err
	}
	identityContext, cancelIdentity := context.WithCancel(context.Background())
	candidateEngineBuildIdentity, err := candidateEngineBuildIdentity(identityContext, gittree.Workspace{Dir: prepared.ProjectRoot},
		prepared.Prefix, prepared.CandidateTree, prepared.Environment)
	cancelIdentity()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	candidateEngineDigest, err := retainedCandidateEngineDigest(prepared, attempts, candidateEngineBuildIdentity, request.Carried)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest := testingRunRequest(prepared, "", "", "", candidateEngineDigest, candidateEngineBuildIdentity)
	metadataContext, cancelMetadata := context.WithCancel(context.Background())
	identities, err := dependencies.revalidate(metadataContext, runRequest, attempts)
	cancelMetadata()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest.FreshnessEpisode = request.FreshEpisode
	runRequest.FreshnessExpiresAt = request.FreshExpiresAt
	freshGroups, maxFreshAge := testingFreshGroups(prepared, request)
	if len(freshGroups) > 0 && request.FreshEpisode == "" ||
		len(freshGroups) == 0 && request.FreshEpisode != "" ||
		maxFreshAge > 0 && request.FreshExpiresAt == "" {
		return proofrun.TestResult{}, fmt.Errorf("selected fresh testing groups require the same explicit episode and expiry used by test run")
	}
	runRequest.FreshGroups = freshGroups
	if err := bindTestingFreshnessProjection(&runRequest, prepared.Installation); err != nil {
		return proofrun.TestResult{}, err
	}
	runRequest.FreshnessBinding = testingFreshnessBinding(runRequest, identities, request.FreshEpisode)
	reuseDecisionAt := dependencies.clock()
	return proofrun.ReusedTestResult(proofrun.NewTestResultAt(runRequest, reuseDecisionAt), attempts, identities,
		prepared.EffectiveContract), nil
}

func printMovedProofInputs(request testingSelectionRequest, current proofrun.TestResult) {
	installation, err := canonicalProofRoot(request.Root)
	if err != nil {
		return
	}
	goalID, err := resolveTestingGoal(installation, request.GoalID)
	if err != nil {
		return
	}
	_, accountingRevision, err := testingGoalRisk(installation, goalID)
	if err != nil {
		return
	}
	attempts, err := proofrun.ReadAttempts(installation)
	if err != nil {
		return
	}
	currentGroups := map[string]proofrun.GroupResult{}
	for _, group := range current.Groups {
		currentGroups[group.ID] = group
	}
	workspace := gittree.Workspace{Dir: current.ProjectRoot}
	for _, id := range current.Delivery.MissingGroups {
		var source *proofrun.GroupResult
		var sourceTree string
		var newest time.Time
		for _, attempt := range attempts {
			if !attemptAccountsForCandidate(attempt, goalID, accountingRevision) ||
				attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
				continue
			}
			started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if startedErr != nil || (!newest.IsZero() && !started.After(newest)) {
				continue
			}
			for groupIndex := range attempt.TestResult.Groups {
				group := &attempt.TestResult.Groups[groupIndex]
				if group.ID == id && group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") &&
					group.ExecutionIdentity != currentGroups[id].ExecutionIdentity {
					copyGroup := *group
					source, sourceTree, newest = &copyGroup, attempt.TestResult.CandidateTree, started
				}
			}
		}
		if source == nil {
			continue
		}
		changed, err := workspace.ChangedPaths(sourceTree, current.CandidateTree)
		if err != nil {
			continue
		}
		var moved []string
		for _, changedPath := range changed {
			if testInputManifestContains(source.InputManifest, changedPath) {
				moved = append(moved, changedPath)
			}
		}
		if len(moved) == 0 {
			fmt.Fprintf(os.Stderr, "proof-input-moved-after-receipt: group %s was proved on tree %s with a different input identity; no declared path moved; the environment or a tool identity changed\n", id, sourceTree)
			continue
		}
		fmt.Fprintf(os.Stderr, "proof-input-moved-after-receipt: group %s was proved on tree %s with a different input identity; moved declared paths: %s\n", id, sourceTree, strings.Join(moved, ","))
	}
}

func printMovedProofInputsWithoutCandidateEngine(request testingSelectionRequest) {
	prepared, err := prepareTesting(request)
	if err != nil {
		return
	}
	attempts, err := proofrun.ReadAttempts(prepared.Installation)
	if err != nil {
		return
	}
	workspace := gittree.Workspace{Dir: prepared.ProjectRoot}
	for _, id := range prepared.Plan.RequiredGroups {
		var source *proofrun.GroupResult
		var sourceTree string
		var newest time.Time
		for _, attempt := range attempts {
			if !attemptAccountsForCandidate(attempt, prepared.GoalID, prepared.AccountingRevision) ||
				attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
				continue
			}
			started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if startedErr != nil || (!newest.IsZero() && !started.After(newest)) {
				continue
			}
			for groupIndex := range attempt.TestResult.Groups {
				group := &attempt.TestResult.Groups[groupIndex]
				if group.ID == id && group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") {
					copyGroup := *group
					source, sourceTree, newest = &copyGroup, attempt.TestResult.CandidateTree, started
				}
			}
		}
		if source == nil {
			continue
		}
		changed, changedErr := workspace.ChangedPaths(sourceTree, prepared.CandidateTree)
		if changedErr != nil {
			continue
		}
		var moved []string
		for _, changedPath := range changed {
			if testInputManifestContains(source.InputManifest, changedPath) {
				moved = append(moved, changedPath)
			}
		}
		if len(moved) > 0 {
			fmt.Fprintf(os.Stderr, "proof-input-moved-after-receipt: group %s was proved on tree %s with a different input identity; moved declared paths: %s\n", id, sourceTree, strings.Join(moved, ","))
		}
	}
}

func attemptAccountsForCandidate(attempt proofrun.Attempt, goalID string, accountingRevision uint64) bool {
	return attempt.AccountedGoal() == goalID && attempt.AccountedRevision() == accountingRevision
}

func testInputManifestContains(manifest []string, candidate string) bool {
	for _, declaration := range manifest {
		matched, err := pathpattern.MatchManifestEntry(declaration, candidate)
		if err != nil || matched {
			return true
		}
	}
	return false
}

func retainedCandidateEngineDigest(prepared testingPreparation, attempts []proofrun.Attempt, buildIdentity string, carried bool) (string, error) {
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
	receiptsRoot := filepath.Dir(landing.TestReceiptPath(prepared.proofControlRoot(), prepared.CandidateTree))
	entries, readErr := os.ReadDir(receiptsRoot)
	if readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			var receipt landing.TestReceipt
			if readStrictJSON(filepath.Join(receiptsRoot, entry.Name()), &receipt) != nil ||
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
	return "", fmt.Errorf("%w for build identity %s; run metasystem test run", errRetainedCandidateEngineAbsent, buildIdentity)
}

func loadPhysicalTestingContract(root string) (string, testpolicy.Contract, string, error) {
	installation, err := canonicalProofRoot(root)
	if err != nil {
		return "", testpolicy.Contract{}, "", err
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, found, err := config.ConfLookup(confPath, "testing.contract")
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

func checkTestingTools(projectRoot string, contract testpolicy.Contract) error {
	var unavailable []string
	for _, group := range contract.Groups {
		cwd := filepath.Join(projectRoot, filepath.FromSlash(group.CWD))
		if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
			unavailable = append(unavailable, group.ID+":cwd")
			continue
		}
		environment := proofrun.GroupTestingEnvironment(testingEnvironment(os.Environ()), group)
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

func testingGoalRisk(root, id string) (testpolicy.GoalRisk, uint64, error) {
	if id == "" {
		return testpolicy.GoalRisk{}, 0, nil
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return testpolicy.GoalRisk{}, 0, err
	}
	return testingGoalRiskWithEndpoint(endpoint, id, time.Now().UTC())
}

func testingGoalRiskWithEndpoint(endpoint goal.Endpoint, id string, now time.Time) (testpolicy.GoalRisk, uint64, error) {
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

func resolveTestingGoal(root, requested string) (string, error) {
	return resolveTestingGoalWithReads(root, requested, goal.ResolveMachine, goal.ResolveEndpoint, time.Now)
}

func resolveTestingGoalWithReads(root, requested string, resolveMachine func(string) (string, error), resolveEndpoint func(string) (goal.Endpoint, error), now func() time.Time) (string, error) {
	if requested != "" {
		return requested, nil
	}
	parentRoot, parentAttempt := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if parentRoot != "" || parentAttempt != "" {
		if parentRoot == "" || parentAttempt == "" {
			return "", fmt.Errorf("proof parent locator is incomplete")
		}
		canonicalParent, err := canonicalProofRoot(parentRoot)
		if err != nil || canonicalParent != root {
			return "", fmt.Errorf("proof parent control root does not match testing root")
		}
		attempt, err := proofrun.AuthenticateContext(root, parentAttempt, int64(os.Getppid()))
		if err != nil {
			return "", err
		}
		return attempt.AccountedGoal(), nil
	}
	return uniqueActiveProofGoalWithReads(root, now().UTC(), resolveMachine, resolveEndpoint)
}

func trustedTestingPolicyBase(projectRoot string, workspace gittree.Workspace) (string, error) {
	data, err := testingLandingRef(projectRoot, "--worktree")
	if err != nil {
		data, err = testingLandingRef(projectRoot, "--local")
	}
	ref := strings.TrimSpace(string(data))
	tail := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if err != nil || tail == ref || !qualified || remote == "" || branch == "" {
		return "", fmt.Errorf("trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>")
	}
	commit, err := workspace.ResolveCommit(ref)
	if err != nil {
		return "", fmt.Errorf("resolve trusted testing policy base %s: %w", ref, err)
	}
	return commit, nil
}

func testingLandingRef(projectRoot, scope string) ([]byte, error) {
	command := exec.Command("git", "-C", projectRoot, "config", scope, "--no-includes", "--get", "metasystem.steward.landing-ref")
	command.Env = gittree.ScrubbedEnviron()
	return command.Output()
}

func testingEnvironment(environment []string) []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "TMP": true, "TEMP": true,
		"GOCACHE": true, "GOMODCACHE": true, "GOTMPDIR": true, "STATICCHECK_CACHE": true, "GOPATH": true, "GOROOT": true, "GOFLAGS": true, "GOWORK": true,
		"CGO_ENABLED": true, "GOTOOLCHAIN": true, "GOEXPERIMENT": true, "JAVA_HOME": true, "LANG": true,
		"LC_ALL": true, "SYSTEMROOT": true, "TZ": true, identity.RunOwnerEnv: true}
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

func inheritedTestingEnvironment(prepared, inherited []string) []string {
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

func readTestingWorkerResult(path string) (proofrun.TestResult, error) {
	var result proofrun.TestResult
	if err := readStrictJSON(path, &result); err != nil {
		return proofrun.TestResult{}, err
	}
	if err := proofrun.ValidateTestResult(result); err != nil {
		return proofrun.TestResult{}, err
	}
	return result, nil
}

func readStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON in %s", path)
	}
	return nil
}

func writePrivateJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteVolatile(path, string(data)+"\n")
}

func publishTestingResult(root, path string, result proofrun.TestResult) error {
	if path != "" {
		return writeIdentityJSON(path, result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if len(encoded) <= output.MaxInlineBytes {
		fmt.Println(string(encoded))
		return nil
	}
	reference, err := output.Spill(root, "test-run", "json", append(encoded, '\n'), time.Now().UTC())
	if err != nil {
		return err
	}
	printJSON(reference)
	return nil
}

func printTestingSummary(result proofrun.TestResult) {
	admission := "unlimited"
	if result.AdmissionMaximum != nil && *result.AdmissionMaximum > 0 {
		admission = strconv.Itoa(*result.AdmissionMaximum)
	}
	fmt.Printf("TEST-RESULT sufficient=%t tree=%s selected=%s workers=%d admissionMaximum=%s\n",
		result.Delivery.Sufficient, result.CandidateTree, strings.Join(result.SelectedGroups, ","), result.Workers, admission)
	for _, group := range result.Groups {
		if group.Status != "passed" && group.Status != "reused" {
			fmt.Printf("TEST-GROUP %s status=%s reason=%s log=%s\n", group.ID, group.Status, group.NotRunReason, group.LogPath)
		}
	}
}

func bytesSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return bytesSHA256(data), nil
}

// testingReceiptWanted says whether a sufficient result becomes a testing
// receipt on this checkout. A joined attempt has no result of its own, and
// a diagnostic attempt never lands: it proves a tree that need not be this
// checkout's index (a delegate round's worktree through job prove-round,
// the bounded unknown groups), and the receipt preparation would refuse the
// moved candidate and turn a sufficient attempt into a failed terminal.
// testingTerminalCommit is the outer testing launch's terminal commit: the
// worker's result is read once and committed with the terminal. A worker
// that left no usable result (it refused before its first group, or its
// suite was ended under it, or it wrote a result the reader refuses) ends
// the attempt failed with the reader's error named, instead of failing the
// commit itself and leaving an unknown terminal that hides the cause:
// thirteen of the nineteen unknown terminals on four checkouts between
// 2026-09-09 and 09-12 read that way. A success without its result is a
// contradiction and is refused; in a real launch PrepareSuccess reads the
// result first and turns a missing one into a failed exit, so this guard is
// the commit's own defence.
func testingTerminalCommit(workerResultPath string, retained **proofrun.TestResult) func(proofrun.CompletionContext, json.RawMessage) error {
	return testingTerminalCommitWithReads(workerResultPath, retained, nil)
}

func testingTerminalCommitWithReads(workerResultPath string, retained **proofrun.TestResult, reads *dispatchcore.ProofAdmissionReads) func(proofrun.CompletionContext, json.RawMessage) error {
	return func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
		if *retained == nil {
			result, readErr := readTestingWorkerResult(workerResultPath)
			if readErr != nil {
				if completion.ExitStatus == 0 {
					return readErr
				}
				return commitProofTerminalWithReasonAndReads(completion, receipt, nil,
					"proof launcher completed; the worker left no usable result: "+readErr.Error(), reads)
			}
			*retained = &result
		}
		return commitProofTerminalWithReasonAndReads(completion, receipt, *retained, "proof launcher completed", reads)
	}
}

func testingReceiptWanted(joined bool, purpose testpolicy.Purpose, sufficient bool) bool {
	return !joined && sufficient && purpose != testpolicy.PurposeDiagnostic
}

// cancelOnRecordedIntent cancels the worker's context once a cancellation
// intent is recorded on its attempt; it reads the record at a slow pace
// and ends with the context.
func cancelOnRecordedIntent(ctx context.Context, cancel context.CancelFunc, controlRoot, attemptID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if attempt, err := proofrun.ReadAttempt(controlRoot, attemptID); err == nil && attempt.CancellationIntent != "" {
				fmt.Fprintf(os.Stderr, "metasystem test worker: cancellation intent recorded: %s\n", attempt.CancellationIntent)
				cancel()
				return
			}
		}
	}
}
