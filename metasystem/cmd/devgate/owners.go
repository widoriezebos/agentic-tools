package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	goalpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/parallelratchet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// owners is every in-process owner the static and full gates consult. Each
// check returns the text its former verb printed (standard output and
// standard error in the order the verb wrote them) and whether it passed, so
// the gate's collected reds read exactly as they did when the script captured
// the verb with 2>&1. A test replaces any of them with a stub.
type owners struct {
	dependencyRatchet func(root string) (string, bool)
	parallelRatchet   func(root string) (string, bool)
	hookStartExits    func(root string) (string, bool)
	stopSurface       func(root string) (string, bool)
	projectCheck      func(root string) (string, bool)

	register    func(root string, pid int64, gate string) (string, error)
	runOwnerRef func(pid int64) (string, error)
	processPair func(pid int64) (identity.Ref, error)
	descendant  func(consumer int64, controller identity.Ref) error

	freeze        func(root string) (proofrun.FrozenExport, error)
	cleanupFreeze func(snapshot string) error
	verify        func(root, expected string) (string, error)
	surface       func() (behaviorsurface.Policy, error)

	goGateTests      func(ctx context.Context, request proofrun.GoGateTestRequest) (proofrun.GoGateTestResult, int, error)
	hostResources    func(ctx context.Context, controlRoot string) (func(), context.Context, error)
	coverageEligible func(proofrun.CoverageBeginOptions) (bool, error)
	coverageBegin    func(proofrun.CoverageBeginOptions) error
	coverageComplete func(proofrun.CoverageCompleteOptions) (proofrun.CoverageEvidence, error)
}

func nativeOwners() owners {
	return owners{
		dependencyRatchet: dependencyRatchet,
		parallelRatchet:   parallelRatchet,
		hookStartExits:    hookStartExits,
		stopSurface:       stopSurface,
		projectCheck:      projectCheck,
		register:          gaterun.Register,
		runOwnerRef:       runOwnerRef,
		processPair:       processPair,
		descendant:        gaterun.ControllerDescendant,
		freeze:            proofrun.Freeze,
		cleanupFreeze:     proofrun.CleanupFrozenExport,
		verify:            proofrun.Verify,
		surface:           behaviorsurface.Load,
		goGateTests:       proofrun.RunGoGateTests,
		hostResources:     hostResources,
		coverageEligible:  proofrun.CoverageProducerEligible,
		coverageBegin:     proofrun.BeginCoverage,
		coverageComplete:  proofrun.CompleteCoverage,
	}
}

// dependencyRatchet refuses undeclared executable interpreter dependencies in
// shell sources (the retired `audit dependency-ratchet` verb's text).
func dependencyRatchet(root string) (string, bool) {
	findings, err := audit.AuditDependencies(root)
	if err != nil {
		return err.Error() + "\n", false
	}
	var out strings.Builder
	for _, finding := range findings {
		out.WriteString("dependency ratchet: " + finding.String() + "\n")
	}
	if len(findings) != 0 {
		return out.String(), false
	}
	return "dependency ratchet passed\n", true
}

// parallelRatchet refuses an increase in any package's serial Go test count.
func parallelRatchet(root string) (string, bool) {
	refuse := func(prefix, reason string) (string, bool) {
		return prefix + "PARALLEL_RATCHET_REFUSED: " + reason + "\n", false
	}
	baseline, err := parallelratchet.ReadParallelRatchet(root + "/testing-parallel-ratchet.json")
	if err != nil {
		return refuse("", err.Error())
	}
	inventory, err := parallelratchet.ScanParallelTests(root)
	if err != nil {
		return refuse("", err.Error())
	}
	_, violations := parallelratchet.CheckParallelRatchet(baseline, inventory)
	if len(violations) != 0 {
		var out strings.Builder
		for _, violation := range violations {
			out.WriteString("parallel ratchet: " + violation.String() + "\n")
		}
		return refuse(out.String(), "serial Go test count increased")
	}
	return "parallel ratchet passed\n", true
}

func hookStartExits(root string) (string, bool) {
	findings, err := audit.AuditHookStartExits(root)
	if err != nil {
		return err.Error() + "\n", false
	}
	var out strings.Builder
	for _, finding := range findings {
		out.WriteString("hook start exit audit: " + finding.String() + "\n")
	}
	if len(findings) != 0 {
		return out.String(), false
	}
	return "hook start exit audit passed\n", true
}

func stopSurface(root string) (string, bool) {
	result, err := audit.AuditStopDecisionSurface(root, audit.StopSurfaceOptions{GoalRecord: goalpkg.StopSurfaceGoalReader})
	if err != nil {
		return err.Error() + "\n", false
	}
	var out strings.Builder
	for _, line := range result.Added {
		fmt.Fprintf(&out, "added: %s: %s\n", line.File, line.Line)
	}
	for _, line := range result.Moved {
		fmt.Fprintf(&out, "moved: %s: %s (goal %s)\n", line.File, line.Line, line.Goal)
	}
	for _, line := range result.Removed {
		fmt.Fprintf(&out, "removed: %s: %s\n", line.File, line.Line)
	}
	for _, problem := range result.Problems {
		out.WriteString("stop decision surface: " + problem + "\n")
	}
	out.WriteString(result.Summary() + "\n")
	if result.Refused() {
		out.WriteString("restore the assertion, or run metasystem internal audit stop-decision-surface --declare --goal <goal-id> --reason <text> with the goal that permits the move\n")
		return out.String(), false
	}
	return out.String(), true
}

// projectCheck reads the project's declared memory: the design homes, the
// decision home, the books and the question register.
func projectCheck(root string) (string, bool) {
	roots, err := project.ResolveRoots(root)
	if err != nil {
		return err.Error() + "\n", false
	}
	read, err := project.Read(roots)
	if err != nil {
		return err.Error() + "\n", false
	}
	var out strings.Builder
	for _, problem := range read.Problems {
		out.WriteString(problem.String() + "\n")
	}
	if len(read.Problems) > 0 {
		return out.String(), false
	}
	fmt.Fprintf(&out, "project check passed: %d record(s) in %d home(s), %d question(s), %d ledger goal(s)\n",
		len(read.Records), len(read.Homes), len(read.Questions), len(read.Goals))
	return out.String(), true
}

// runOwnerRef is the exact reference of the gate process that its test
// children name as their run owner.
func runOwnerRef(pid int64) (string, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil {
		return "", err
	}
	if state != identity.Alive {
		return "", fmt.Errorf("pid %d is not alive", pid)
	}
	return identity.EncodeRef(exact.Ref())
}

// processPair is the start identity a witness records for its controller.
func processPair(pid int64) (identity.Ref, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil {
		return identity.Ref{}, err
	}
	if state != identity.Alive {
		return identity.Ref{}, fmt.Errorf("pid %d is not alive", pid)
	}
	return identity.Ref{Pid: pid, StartedAtSec: exact.StartedAt.Unix(), StartTicks: exact.StartTicks, BootID: exact.BootID}, nil
}

// hostResources inherits the admitted parent's host reservation, exactly as
// the retired native-test verb did before it ran the selection.
func hostResources(ctx context.Context, controlRoot string) (func(), context.Context, error) {
	lease, err := proofrun.AcquireHostResources(ctx, controlRoot, controlRoot+"/metasystem.conf", "heavy", nil)
	if err != nil {
		return nil, ctx, err
	}
	return func() { _ = lease.Close() }, proofrun.WithHostResourceLease(ctx, lease), nil
}

// coverageRatchet judges measured coverage against the checked-in floors,
// joined with the independent package inventory (the retired `audit
// coverage-ratchet` verb). It returns the refusal lines.
func coverageRatchet(baselinePath string, coverage, inventory []byte) ([]string, error) {
	const module = "github.com/widoriezebos/agentic-tools/metasystem/"
	baseline, err := audit.ReadCoverageBaseline(baselinePath)
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, line := range strings.Split(string(inventory), "\n") {
		if pkg := strings.TrimSpace(line); pkg != "" {
			packages = append(packages, strings.TrimPrefix(pkg, module))
		}
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("package inventory is empty; refusing a joinless ratchet run")
	}
	var lines []string
	for _, violation := range audit.CheckCoverage(baseline, audit.ParseCoverage(string(coverage), module), packages) {
		lines = append(lines, "coverage ratchet: "+violation)
	}
	return lines, nil
}

func encodeEvidence(evidence proofrun.CoverageEvidence) string {
	var out bytes.Buffer
	encoded, _ := json.Marshal(evidence)
	out.Write(encoded)
	out.WriteByte('\n')
	return out.String()
}
