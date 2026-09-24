package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type portableFile struct {
	data string
	mode os.FileMode
}
type portableFileProof struct {
	t        *testing.T
	root     string
	counter  string
	files    map[string]portableFile
	contract testpolicy.Contract
	sequence int
}

func newPortableFileProof(t *testing.T) *portableFileProof {
	t.Helper()
	f := &portableFileProof{t: t, root: t.TempDir(), counter: filepath.Join(t.TempDir(), "native.log"), files: map[string]portableFile{}}
	f.contract = testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"app/**", "testing.json"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}}, {ID: "docs", Paths: []string{"docs/**"}, Standard: []string{"app-a"}}},
		Groups:      []testpolicy.Group{f.group("app-a", "a")}, Always: testpolicy.Always{Canary: []string{"app-a"}}, Unknown: []string{"app-a"}, Cadence: []string{"app-a"}}
	f.put("app/a.txt", "green a\n", 0o644)
	f.put("app/shared-b.txt", "shared b v1\n", 0o644)
	f.put("scripts/check.sh", portableCommandCheck, 0o755)
	f.put("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\ntesting.workers=1\n", 0o644)
	if err := testexec.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte(f.files["metasystem.conf"].data), 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeContract()
	return f
}
func (f *portableFileProof) group(id, input string) testpolicy.Group {
	report := "reports-" + input
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".",
		Inputs: []string{"app/" + input + ".txt", "scripts/check.sh"}, Outputs: []string{report},
		Tools:       []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf portable-shell"}}},
		Obligations: []string{id + "-observed"}, Platforms: []string{"any"}, TargetMS: 1000,
		Env:     map[string]string{"PORTABLE_NATIVE_COUNTER": f.counter},
		Argv:    []string{"sh", "scripts/check.sh", input, "app/" + input + ".txt", report},
		Reports: []string{report}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: report + "/result.xml", Classname: "portable", Name: input}}}
}
func (f *portableFileProof) put(path, data string, mode os.FileMode) {
	f.files[path] = portableFile{data, mode}
}
func (f *portableFileProof) writeContract() {
	f.t.Helper()
	if err := f.contract.Validate(); err != nil {
		f.t.Fatal(err)
	}
	encoded, err := json.Marshal(f.contract)
	if err != nil {
		f.t.Fatal(err)
	}
	f.put("testing.json", string(encoded)+"\n", 0o644)
}
func (f *portableFileProof) snapshot() (string, map[string]portableFile) {
	f.t.Helper()
	f.sequence++
	tree := fmt.Sprintf("%040x", f.sequence)
	files := make(map[string]portableFile, len(f.files))
	for path, file := range f.files {
		files[path] = file
	}
	return tree, files
}
func (f *portableFileProof) open(tree string, files map[string]portableFile) func(string, string) (proofrun.CandidateWorkspace, error) {
	return func(root, actual string) (proofrun.CandidateWorkspace, error) {
		if root != f.root || actual != tree {
			return nil, fmt.Errorf("undeclared portable candidate root=%q tree=%q", root, actual)
		}
		dir, err := os.MkdirTemp("", "portable-file-candidate-")
		if err != nil {
			return nil, err
		}
		for path, file := range files {
			if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || slices.Contains(strings.Split(filepath.ToSlash(path), "/"), ".git") || slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..") {
				_ = os.RemoveAll(dir)
				return nil, fmt.Errorf("bad declared path %q", path)
			}
			target := filepath.Join(dir, filepath.FromSlash(path))
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err == nil {
				err = testexec.WriteFile(target, []byte(file.data), file.mode)
			}
			if err == nil {
				err = os.Chmod(target, file.mode)
			}
			if err != nil {
				_ = os.RemoveAll(dir)
				return nil, err
			}
			got, readErr := os.ReadFile(target)
			info, statErr := os.Stat(target)
			if readErr != nil || statErr != nil || string(got) != file.data || !info.Mode().IsRegular() || info.Mode().Perm() != file.mode.Perm() {
				_ = os.RemoveAll(dir)
				return nil, fmt.Errorf("candidate bytes/mode %s: read=%v stat=%v", path, readErr, statErr)
			}
		}
		return &portableFileCandidate{dir: dir}, nil
	}
}

type portableFileCandidate struct{ dir string }

func (c *portableFileCandidate) Workspace() gittree.Workspace { return gittree.Workspace{Dir: c.dir} }
func (c *portableFileCandidate) Close() error                 { return os.RemoveAll(c.dir) }

func (f *portableFileProof) loadedContract(files map[string]portableFile) testpolicy.Contract {
	f.t.Helper()
	path := filepath.Join(f.root, "testing.json")
	if err := testexec.WriteFile(path, []byte(files["testing.json"].data), 0o644); err != nil {
		f.t.Fatal(err)
	}
	contract, err := testpolicy.Load(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return contract
}
func (f *portableFileProof) plan(contract testpolicy.Contract, changed ...string) testpolicy.Plan {
	f.t.Helper()
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: changed, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery})
	if err != nil {
		f.t.Fatal(err)
	}
	return plan
}
func (f *portableFileProof) request(tree string, files map[string]portableFile, plan testpolicy.Plan) proofrun.TestRunRequest {
	contract := f.loadedContract(files)
	request := proofrun.TestRunRequest{ProjectRoot: f.root, CandidateTree: tree, BaseCommit: "opaque-base", PolicyBaseCommit: "opaque-base", Contract: contract, Plan: plan,
		Environment: os.Environ(), LogRoot: filepath.Join(f.root, "logs", tree), Workers: 1, CandidateEngineDigest: strings.Repeat("d", 64), PolicyEngineDigest: strings.Repeat("e", 64)}
	request.WithCandidateOpener(f.open(tree, files))
	template := proofrun.NewTestResult(request)
	request.ContractDigest, request.BaseContractDigest = template.ContractDigest, template.BaseContractDigest
	request.BehaviorPolicyDigest, request.JudgeKey = template.BehaviorPolicyDigest, template.JudgeKey
	return request
}
func (f *portableFileProof) counts() map[string]int {
	f.t.Helper()
	counts := map[string]int{}
	data, err := os.ReadFile(f.counter)
	if os.IsNotExist(err) {
		return counts
	}
	if err != nil {
		f.t.Fatal(err)
	}
	for _, word := range strings.Fields(string(data)) {
		counts[word]++
	}
	return counts
}
func (f *portableFileProof) prepare(request proofrun.TestRunRequest) (map[string]string, map[string]proofrun.PreparedGroupExecution) {
	f.t.Helper()
	ids, prepared, _, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		f.t.Fatal(err)
	}
	if len(ids) != len(request.Plan.SelectedGroups) {
		f.t.Fatalf("identities=%v selected=%v", ids, request.Plan.SelectedGroups)
	}
	return ids, prepared
}

func (f *portableFileProof) execute(request proofrun.TestRunRequest, wantGreen bool) (proofrun.TestResult, map[string]string) {
	f.t.Helper()
	ids, prepared := f.prepare(request)
	if request.FreshnessEpisode != "" {
		request.FreshnessBinding = testingFreshnessBinding(request, ids, request.FreshnessEpisode)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		f.t.Fatal(err)
	}
	identity := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{ManifestDigest: strings.Repeat("a", 64), Configuration: strings.Repeat("b", 64), Platform: runtime.GOOS + "/" + runtime.GOARCH, Toolchain: strings.Repeat("c", 64)}, "selected", "testing", nil, 2)
	now := time.Now().UTC()
	admission := proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root, GoalID: "portable", GoalRevision: 2, AccountingRevision: 2, CandidateGoalID: "portable", CandidateRevision: 2, CandidateTree: request.CandidateTree,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now, ComponentIdentities: ids, SharedComponents: true, ForceAttempt: true,
		FreshnessEpisode: request.FreshnessEpisode, FreshnessBinding: request.FreshnessBinding, FreshnessExpiresAt: request.FreshnessExpiresAt, FreshGroups: request.FreshGroups}
	admission = privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(admission, "0"))
	attempt, decision, err := proofrun.ReserveLocked(admission)
	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		f.t.Fatalf("reserve portable: %+v %v", decision, err)
	}
	request.ControlRoot, request.AttemptID = f.root, attempt.AttemptID
	request.ComponentIdentities, request.PreparedGroups = ids, prepared
	result, status, err := proofrun.RunTestPlan(context.Background(), request)
	if err != nil || (status == 0) != wantGreen {
		f.t.Fatalf("portable status=%d wantGreen=%t err=%v result=%+v", status, wantGreen, err, result)
	}
	if err := proofrun.ValidateTestResult(result); err != nil {
		f.t.Fatalf("invalid portable result: %v", err)
	}
	terminal := proofrun.TerminalSuccess
	if !result.Delivery.Sufficient {
		terminal = proofrun.TerminalFailed
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, terminal, status, "portable command evidence", nil, &result, time.Now().UTC()); err != nil {
		f.t.Fatal(err)
	}
	retained, err := proofrun.ReadAttempts(f.root)
	if err != nil || len(retained) == 0 {
		f.t.Fatalf("retained attempts=%d err=%v", len(retained), err)
	}
	return result, ids
}
func portableGroups(result proofrun.TestResult) map[string]proofrun.GroupResult {
	groups := make(map[string]proofrun.GroupResult, len(result.Groups))
	for _, group := range result.Groups {
		groups[group.ID] = group
	}
	return groups
}
func requirePortableCounts(t *testing.T, got, want map[string]int) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native counts=%v want=%v", got, want)
	}
}

// The application snapshots contain no engine sources. This reader checks the
// exact empty engine projection transcript; the verification owner still
// recomputes group identities from the declared candidate bytes.
func (f *portableFileProof) engineIO(tree string) (candidateEngineIO, func()) {
	f.t.Helper()
	policy, err := behaviorsurface.Load()
	if err != nil {
		f.t.Fatal(err)
	}
	for path := range f.files {
		for _, declaration := range policy.EnginePaths {
			matched, matchErr := pathpattern.MatchManifestEntry(declaration, path)
			if matchErr != nil || matched {
				f.t.Fatalf("empty engine projection contains %q under %q: %v", path, declaration, matchErr)
			}
		}
	}
	const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	call := 0
	var sourceIndex, targetIndex string
	run := func(command *exec.Cmd) error {
		if len(command.Args) < 4 || command.Args[0] != "git" || command.Args[1] != "-C" || command.Args[2] != f.root {
			f.t.Fatalf("engine projection command=%q", command.Args)
		}
		args := command.Args[3:]
		if call < 4 {
			if len(args) < 5 || !reflect.DeepEqual(args[:4], []string{"-c", "core.fileMode=true", "-c", "core.useReplaceRefs=false"}) {
				f.t.Fatalf("engine projection pins=%q", args)
			}
			args = args[4:]
			if len(command.Env) == 0 || !strings.HasPrefix(command.Env[len(command.Env)-1], "GIT_INDEX_FILE=") {
				f.t.Fatalf("engine projection environment=%q", command.Env)
			}
			index := strings.TrimPrefix(command.Env[len(command.Env)-1], "GIT_INDEX_FILE=")
			if !filepath.IsAbs(index) || filepath.Dir(filepath.Dir(index)) != os.TempDir() ||
				!strings.HasPrefix(filepath.Base(filepath.Dir(index)), "metasystem-engine-projection.") ||
				!reflect.DeepEqual(command.Env, gittree.ScrubbedEnviron("GIT_INDEX_FILE="+index)) {
				f.t.Fatalf("engine projection index=%q", index)
			}
			switch call {
			case 0:
				if !reflect.DeepEqual(args, []string{"read-tree", tree}) || filepath.Base(index) != "source-index" {
					f.t.Fatalf("engine source tree=%q", args)
				}
				sourceIndex = index
			case 1:
				if !reflect.DeepEqual(args, append([]string{"ls-files", "-s", "-z", "--"}, policy.EnginePaths...)) || index != sourceIndex {
					f.t.Fatalf("engine selectors/index=%q %q", args, index)
				}
			case 2:
				if !reflect.DeepEqual(args, []string{"read-tree", "--empty"}) || filepath.Base(index) != "target-index" || filepath.Dir(index) != filepath.Dir(sourceIndex) {
					f.t.Fatalf("engine target index=%q %q", args, index)
				}
				targetIndex = index
			case 3:
				if !reflect.DeepEqual(args, []string{"write-tree"}) || index != targetIndex {
					f.t.Fatalf("engine output index=%q %q", args, index)
				}
				_, _ = fmt.Fprintln(command.Stdout, emptyTree)
			}
		} else if call == 4 {
			if len(args) != 18 || !reflect.DeepEqual(args[:15], []string{"-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid", "-c", "author.name=MetaSystem", "-c", "author.email=metasystem@invalid", "-c", "committer.name=MetaSystem", "-c", "committer.email=metasystem@invalid", "-c", "i18n.commitEncoding=UTF-8", "commit-tree"}) || args[15] != emptyTree || args[16] != "-m" || !strings.Contains(args[17], "engine-tree="+emptyTree) {
				f.t.Fatalf("engine commit request=%q", args)
			}
			commitEnv := make([]string, 0)
			for _, entry := range gittree.ScrubbedEnviron() {
				name, _, _ := strings.Cut(entry, "=")
				switch name {
				case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE":
					continue
				}
				commitEnv = append(commitEnv, entry)
			}
			commitEnv = append(commitEnv, "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
			if !reflect.DeepEqual(command.Env, commitEnv) {
				f.t.Fatalf("engine commit environment=%q", command.Env)
			}
			digest := sha256.Sum256([]byte(args[17]))
			_, _ = fmt.Fprintf(command.Stdout, "%x\n", digest[:20])
		} else {
			f.t.Fatalf("extra engine projection command=%q", args)
		}
		call++
		return nil
	}
	return candidateEngineIO{runGit: run}, func() {
		if call != 5 {
			f.t.Fatalf("engine projection calls=%d want 5", call)
		}
	}
}
func (f *portableFileProof) engineIdentity(tree string, environment []string) string {
	f.t.Helper()
	dependency, check := f.engineIO(tree)
	identity, err := candidateEngineBuildIdentityUsing(context.Background(), gittree.Workspace{Dir: f.root}, "", tree, environment, dependency)
	if err != nil {
		f.t.Fatal(err)
	}
	check()
	return identity
}
func (f *portableFileProof) prepared(request proofrun.TestRunRequest, result proofrun.TestResult) testingPreparation {
	return testingPreparation{Installation: f.root, ControlRoot: f.root, ProjectRoot: f.root, ConfPath: filepath.Join(f.root, "metasystem.conf"),
		GoalID: "portable", AccountingRevision: 2, BaseCommit: request.BaseCommit, PolicyBaseCommit: request.PolicyBaseCommit, CandidateTree: request.CandidateTree,
		EffectiveContract: request.Contract, Plan: request.Plan, Environment: request.Environment,
		ContractDigest: result.ContractDigest, BaseContractDigest: result.BaseContractDigest, PolicyEngineDigest: result.PolicyEngineDigest,
		BehaviorPolicyDigest: result.BehaviorPolicyDigest, JudgeKey: result.JudgeKey, Workers: 1}
}
func (f *portableFileProof) verify(request proofrun.TestRunRequest, files map[string]portableFile, result proofrun.TestResult, at time.Time) (proofrun.TestResult, error) {
	f.t.Helper()
	dependency, checkEngine := f.engineIO(request.CandidateTree)
	selected := testingSelectionRequest{Root: f.root, GoalID: "portable", Tree: request.CandidateTree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		FreshEpisode: request.FreshnessEpisode, FreshExpiresAt: request.FreshnessExpiresAt}
	workspace := gittree.Workspace{Dir: f.root}
	checkProjection := func() {}
	expired := false
	if request.FreshnessExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339Nano, request.FreshnessExpiresAt)
		if err != nil {
			f.t.Fatal(err)
		}
		expired = !expires.After(at)
	}
	if request.FreshnessEpisode != "" && !expired {
		workspace, checkProjection = f.projection(request.CandidateTree)
	}
	verified, err := verifyRetainedTestingPrepared(selected, f.prepared(request, result), retainedTestingVerification{
		clock: func() time.Time { return at }, revalidate: proofrun.RevalidateRetainedGroupExecutionIdentities,
		workspace: workspace, candidateIO: dependency, openCandidate: f.open(request.CandidateTree, files)})
	if !expired {
		checkEngine()
		checkProjection()
	}
	return verified, err
}
func (f *portableFileProof) projection(tree string) (gittree.Workspace, func()) {
	f.t.Helper()
	paths := landing.WorkspaceExclusions()
	steps := [][]string{
		{"rev-parse", "--show-toplevel"}, {"rev-parse", "--show-prefix"},
		{"read-tree", tree}, {"rev-parse", "--show-toplevel"},
		append([]string{"ls-files", "-z", "--"}, paths...), {"write-tree"},
	}
	call := 0
	index := ""
	raw := func(request gittree.RawRequest) gittree.RawResult {
		if call >= len(steps) {
			f.t.Fatalf("extra freshness projection request: %+v", request)
		}
		args := steps[call]
		want := append(append([]string{"-C", f.root}, ordinaryWorkspacePins...), args...)
		if request.Dir != f.root || !reflect.DeepEqual(request.Args, want) || request.Operation != "git "+strings.Join(args, " ") || len(request.Stdin) != 0 {
			f.t.Fatalf("freshness projection request %d: dir=%q args=%q op=%q", call, request.Dir, request.Args, request.Operation)
		}
		env := gittree.ScrubbedEnviron()
		if call == 2 || call == 4 || call == 5 {
			if len(request.Env) != len(env)+1 || !reflect.DeepEqual(request.Env[:len(env)], env) || !strings.HasPrefix(request.Env[len(env)], "GIT_INDEX_FILE=") {
				f.t.Fatalf("freshness projection index environment: %q", request.Env)
			}
			actual := strings.TrimPrefix(request.Env[len(env)], "GIT_INDEX_FILE=")
			if !filepath.IsAbs(actual) || filepath.Base(actual) != "index" ||
				filepath.Dir(filepath.Dir(actual)) != os.TempDir() ||
				!strings.HasPrefix(filepath.Base(filepath.Dir(actual)), "metasystem-gittree.") {
				f.t.Fatalf("freshness projection index=%q", actual)
			}
			if index == "" {
				index = actual
			} else if index != actual {
				f.t.Fatalf("freshness projection changed index: %q != %q", actual, index)
			}
		} else if !reflect.DeepEqual(request.Env, env) {
			f.t.Fatalf("freshness projection scrubbed environment: %q", request.Env)
		}
		call++
		switch call {
		case 1, 4:
			return gittree.RawResult{Stdout: []byte(f.root + "\n")}
		case 2, 3, 5:
			return gittree.RawResult{}
		case 6:
			return gittree.RawResult{Stdout: []byte(tree + "\n")}
		}
		panic("unreachable")
	}
	return gittree.Workspace{Dir: f.root, RawSource: raw}, func() {
		if call != len(steps) {
			f.t.Fatalf("freshness projection consumed %d of %d requests", call, len(steps))
		}
	}
}
func (f *portableFileProof) bindFreshness(request *proofrun.TestRunRequest) {
	f.t.Helper()
	workspace, check := f.projection(request.CandidateTree)
	if err := bindTestingFreshnessProjectionWithWorkspace(request, f.root, workspace); err != nil {
		f.t.Fatal(err)
	}
	check()
	if request.FreshnessCandidateProjection != request.CandidateTree {
		f.t.Fatalf("freshness projected tree=%q want=%q", request.FreshnessCandidateProjection, request.CandidateTree)
	}
}
