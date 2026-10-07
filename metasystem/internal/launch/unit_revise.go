package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// UnitRevision is one retained correction request of a run: the attempt it
// corrects, the digest of its frozen brief (and of the frozen decisions file
// when one was given) and the attempt it creates. It is written into the run
// before the new attempt is added or anything is launched, so a repeated or
// interrupted request reaches the same attempt instead of another one.
type UnitRevision struct {
	After              int             `json:"after"`
	Attempt            int             `json:"attempt"`
	BriefSHA256        string          `json:"briefSha256"`
	Brief              string          `json:"brief"`
	DecisionsSHA256    string          `json:"decisionsSha256,omitempty"`
	Decisions          string          `json:"decisions,omitempty"`
	RequestedAtUnixSec int64           `json:"requestedAt"`
	Rebase             *UnitRebasePlan `json:"rebase,omitempty"`
	Person             string          `json:"person,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Impact             string          `json:"impact,omitempty"`
	Findings           []string        `json:"findings,omitempty"`
}

// UnitRebasePlan binds a correction to a stopped replay, whose HEAD is its base.
type UnitRebasePlan struct{ Worktree, Base, Commit string }

// UnitRevisionRequest asks for one correction. After is the attempt being
// corrected; zero means the request is first matched against the run's
// retained requests and otherwise binds the current attempt. Brief and
// Decisions are the caller's bytes; they are frozen with the request.
type UnitRevisionRequest struct {
	Run                    string
	After                  int
	Brief                  []byte
	Decisions              []byte
	Rebase                 *UnitRebasePlan
	Person, Reason, Impact string
}

// UnitRevisionResult is the attempt a request reached. Rejoined is true when
// an earlier identical request already created it; Current is the run's
// newest attempt, which differs from Attempt when later work followed.
type UnitRevisionResult struct {
	UnitResult
	Revision UnitRevision
	Rejoined bool
	Current  int
}

// ErrUnitRevisionStale is returned when After names an attempt that is no
// longer the run's newest and no identical request exists for it.
var ErrUnitRevisionStale = errors.New("a correction names the newest attempt, and this one is not")

// Revise creates, or rejoins, the one attempt a correction request asks for.
// It holds the unit's named lock (when the run is a named unit) and the run
// lock, so concurrent identical calls reach one attempt: the second is told
// the unit is busy and repeating the same command continues it.
func (runner *UnitRunner) Revise(request UnitRevisionRequest) (UnitRevisionResult, error) {
	if runner.Manager == nil {
		return UnitRevisionResult{}, errors.New("unit launch manager is unavailable")
	}
	if len(request.Brief) == 0 {
		return UnitRevisionResult{}, coded("UNIT_FOLLOW_UP_MISSING", "", errors.New("the correction brief is empty"))
	}
	if request.After < 0 {
		return UnitRevisionResult{}, coded("UNIT_REVISION_INVALID", fmt.Sprintf("after=%d", request.After), fmt.Errorf("--after %d is not an attempt number", request.After))
	}
	record, err := runner.read(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	bound := *runner
	worktree, key, identityErr := namedUnitIdentity(UnitPlan{Worktree: record.Worktree, Goal: record.Goal, Unit: record.Unit})
	if identityErr == nil {
		entry, found, err := runner.readNamed(key)
		if err != nil {
			return UnitRevisionResult{}, err
		}
		if found && entry.Run == record.ID {
			lock, err := runner.namedLock(key, UnitPlan{Unit: record.Unit, Goal: record.Goal})
			if err != nil {
				return UnitRevisionResult{}, err
			}
			defer releaseUnitLock(lock)
			if entry, found, err = runner.readNamed(key); err != nil {
				return UnitRevisionResult{}, err
			}
			if !found || entry.Run != record.ID || entry.Digest == "" {
				return UnitRevisionResult{}, coded("UNIT_NAMED_ENTRY_CORRUPT", unitFacts(record.Unit, record.Goal, "run="+record.ID), fmt.Errorf("the record of unit %s changed while this command held it; run the command again", record.Unit))
			}
			bound.named = &namedBinding{unit: record.Unit, goal: record.Goal, worktree: worktree, digest: entry.Digest, run: entry.Run,
				options: UnitOptions{BuildModel: record.BuildModel, BuildEffort: record.BuildEffort}}
		}
	}
	return bound.reviseLocked(request)
}

func (runner *UnitRunner) reviseLocked(request UnitRevisionRequest) (UnitRevisionResult, error) {
	lock, err := runner.lock(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	defer releaseUnitLock(lock)
	record, err := runner.read(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	plan, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	if runner.named != nil {
		runner.named.plan = plan
		if err := runner.named.verify(runner); err != nil {
			return UnitRevisionResult{}, err
		}
	}
	briefDigest := digestHex(request.Brief)
	if request.Rebase != nil {
		original, err := os.ReadFile(plan.Build.Brief)
		if err != nil {
			return UnitRevisionResult{}, err
		}
		request.Brief = append(original, request.Brief...)
		briefDigest = digestHex(request.Brief)
	}
	decisionsDigest := ""
	if len(request.Decisions) > 0 {
		decisionsDigest = digestHex(request.Decisions)
	}
	current := len(record.Rounds)
	same := func(revision UnitRevision) bool {
		return revision.Person == request.Person && revision.Reason == request.Reason && revision.BriefSHA256 == briefDigest && revision.DecisionsSHA256 == decisionsDigest &&
			(revision.Rebase == nil && request.Rebase == nil || revision.Rebase != nil && request.Rebase != nil && *revision.Rebase == *request.Rebase)
	}
	var retained *UnitRevision
	for index := range record.Revisions {
		revision := record.Revisions[index]
		if request.After != 0 && revision.After != request.After {
			continue
		}
		if same(revision) {
			retained = &revision
		} else if request.After != 0 {
			return UnitRevisionResult{}, coded("UNIT_REVISION_CONFLICT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d attempt=%d", revision.After, revision.Attempt)),
				fmt.Errorf("attempt %d was already corrected by another brief, which started attempt %d", revision.After, revision.Attempt))
		}
	}
	if retained == nil {
		after := request.After
		if after == 0 {
			after = current
		}
		if after != current || current == 0 {
			return UnitRevisionResult{Current: current}, coded("UNIT_REVISION_STALE", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d current=%d", after, current)),
				fmt.Errorf("%w (you named %d, the newest is %d)", ErrUnitRevisionStale, after, current))
		}
		if record.State != "awaiting-judgement" {
			return UnitRevisionResult{Current: current}, coded("UNIT_RUN_NOT_AWAITING", unitFacts(record.Unit, record.Goal, "state="+string(record.State)), fmt.Errorf("attempt %d is still running", current))
		}
		if request.Person == "" {
			if request.Rebase == nil || record.Rounds[after-1].Stop != nil && record.Rounds[after-1].Stop.Loop == "unit-build" {
				if err := runner.allowCorrection(record); err != nil {
					return UnitRevisionResult{UnitResult: UnitResult{Record: record}, Current: current}, err
				}
			}
			if record.MaxRounds > 0 && current >= record.MaxRounds {
				return UnitRevisionResult{Current: current}, roundLimit(record, current)
			}
			if err := runner.countedCap(record); err != nil {
				return UnitRevisionResult{Current: current}, err
			}
			if err := runner.roundDivergent(record); err != nil {
				return UnitRevisionResult{UnitResult: UnitResult{Record: record}, Current: current}, err
			}
			if request.Rebase == nil {
				if err := runner.reviseDecided(record, record.Rounds[after-1], request.Brief, request.Decisions); err != nil {
					return UnitRevisionResult{Current: current, Revision: UnitRevision{After: after}}, err
				}
			}
		}
		revision := UnitRevision{Person: request.Person, Reason: request.Reason, Impact: request.Impact, After: after, Attempt: after + 1, BriefSHA256: briefDigest, DecisionsSHA256: decisionsDigest,
			RequestedAtUnixSec: runner.Manager.Now().Unix(), Rebase: request.Rebase}
		for _, read := range record.Rounds[after-1].Reads {
			for _, f := range read.Findings {
				if f.Material {
					revision.Findings = append(revision.Findings, f.ID)
				}
			}
		}
		directory := filepath.Join(runner.runDir(record.ID), "revisions")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return UnitRevisionResult{}, err
		}
		revision.Brief = filepath.Join(directory, fmt.Sprintf("after-%d-brief.md", after))
		if _, err := atomicfile.WriteText(revision.Brief, string(request.Brief), runner.root()); err != nil {
			return UnitRevisionResult{}, err
		}
		if decisionsDigest != "" {
			revision.Decisions = filepath.Join(directory, fmt.Sprintf("after-%d-decisions.md", after))
			if _, err := atomicfile.WriteText(revision.Decisions, string(request.Decisions), runner.root()); err != nil {
				return UnitRevisionResult{}, err
			}
		}
		record.Revisions = append(record.Revisions, revision)
		if err := runner.save(record); err != nil {
			return UnitRevisionResult{}, err
		}
		retained = &revision
	}
	bound := *runner
	if retained.Rebase != nil {
		plan, err = runner.rebasePlan(plan, retained.Rebase)
		if err != nil {
			return UnitRevisionResult{}, err
		}
		bound.resolving = true
	}
	runner = &bound
	require := func() error {
		if runner.resolving {
			return nil
		}
		return runner.requireGoalBranch(plan)
	}
	if retained.Attempt <= current {
		// The attempt exists: the request is answered by it, continuing it
		// only while it still runs.
		result := UnitRevisionResult{Revision: *retained, Rejoined: true, Current: current}
		if retained.Attempt < current || record.State == "awaiting-judgement" {
			result.UnitResult = UnitResult{Record: record, Round: retained.Attempt}
			return result, nil
		}
		if err := require(); err != nil {
			return result, err
		}
		unit, err := runner.continueRunning(&record, plan)
		result.UnitResult = unit
		return result, err
	}
	// The request is retained and its attempt not yet added: add it with
	// the frozen brief. The previous inputs are the corrected attempt's read
	// outputs and the frozen decisions, which carry the reviewed findings
	// even when the corrected attempt itself produced no read.
	if data, err := os.ReadFile(retained.Brief); err != nil || digestHex(data) != retained.BriefSHA256 {
		return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d", retained.After)), fmt.Errorf("the kept brief of the correction after attempt %d is missing or changed", retained.After))
	}
	if err := require(); err != nil {
		return UnitRevisionResult{}, err
	}
	previous := readOutputs(runner.Manager, record.Rounds[retained.After-1])
	if retained.Decisions != "" {
		if data, err := os.ReadFile(retained.Decisions); err != nil || digestHex(data) != retained.DecisionsSHA256 {
			return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d", retained.After)), fmt.Errorf("the kept decisions of the correction after attempt %d are missing or changed", retained.After))
		}
		previous = append(previous, retained.Decisions)
	}
	if retained.After != current {
		return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d current=%d", retained.After, current)),
			fmt.Errorf("the kept correction follows attempt %d, but the newest attempt is %d", retained.After, current))
	}
	if err := runner.admitRound(plan, retained.Brief, previous); err != nil {
		return UnitRevisionResult{}, err
	}
	if err := runner.addRound(&record, plan, retained.Brief); err != nil {
		return UnitRevisionResult{}, err
	}
	unit, err := runner.continueRunning(&record, plan)
	return UnitRevisionResult{UnitResult: unit, Revision: *retained, Current: len(record.Rounds)}, err
}

func (runner *UnitRunner) continueRunning(record *UnitRunRecord, plan UnitPlan) (UnitResult, error) {
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return UnitResult{}, err
	}
	deadline := runner.Manager.Now().Add(time.Duration(settings.WaitCapSeconds) * time.Second)
	return runner.advanceRunning(record, plan, deadline)
}

func decisionsSection(brief []byte, round int) string {
	lines := strings.Split(string(brief), "\n")
	for start, line := range lines {
		if strings.TrimSpace(line) != fmt.Sprintf("## Decisions on round %d", round) {
			continue
		}
		end := start + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "## ") && !strings.HasPrefix(lines[end], "# ") {
			end++
		}
		return strings.Join(lines[start:end], "\n")
	}
	return ""
}

var fixedLocation = regexp.MustCompile(`\S+:[0-9]+(?:-[0-9]+)?`)

func (runner *UnitRunner) readFindings(record UnitRunRecord, round UnitRound) (string, []string, error) {
	if len(round.Reads) == 0 {
		return "", nil, nil
	}
	var ids []string
	for _, read := range round.Reads {
		for _, f := range read.Findings {
			if f.Material {
				ids = append(ids, f.ID)
			}
		}
	}
	data, err := json.Marshal(round.Reads)
	return string(data), ids, err
}

func (runner *UnitRunner) reviseDecided(record UnitRunRecord, round UnitRound, brief []byte, supplied ...[]byte) error {
	if round.Outcome == "build-gap" {
		return nil
	}
	if round.Stop == nil && (strings.HasPrefix(round.Outcome, "proof-") || strings.HasPrefix(round.Outcome, "build-")) {
		return nil
	}
	_, findings, err := runner.readFindings(record, round)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		return coded("UNIT_REVISE_CLEAN", "", fmt.Errorf("a clean or unavailable read admits no automatic correction; nothing was started"))
	}
	decided := map[string]int{}
	documents := []string{decisionsSection(brief, round.Number)}
	for _, document := range supplied {
		documents = append(documents, string(document))
	}
	for _, document := range documents {
		joined := map[string]int{}
		for _, row := range strings.Split(document, "\n") {
			cells := strings.Split(strings.Trim(row, " |\t"), "|")
			if len(cells) < 3 {
				continue
			}
			id, decision, evidence := strings.Trim(cells[0], " `\t"), strings.TrimSpace(cells[1]), strings.TrimSpace(strings.Join(cells[2:], "|"))
			if decision == "fixed" && fixedLocation.MatchString(evidence) || (decision == "accepted" || decision == "refuted" || decision == "follow-up") && evidence != "" {
				joined[id]++
			}
		}
		for id, count := range joined {
			// The correction's result supersedes its earlier review decision.
			// Bound review decisions cover findings absent from the correction section.
			if count > 1 || decided[id] == 0 {
				decided[id] = count
			}
		}
	}
	for _, finding := range findings {
		if decided[finding] != 1 {
			return coded("UNIT_REVISE_UNDECIDED", fmt.Sprintf("run=%s round=%d finding=%s", record.ID, round.Number, finding),
				fmt.Errorf("finding %s of round %d has no decision; nothing was started", finding, round.Number))
		}
	}
	return nil
}

// warmRead retains the evidence a reader needs before checking corrections.
// With no readable predecessor it creates nothing and keeps the cold brief.
func (runner *UnitRunner) warmRead(record UnitRunRecord, round UnitRound) (string, error) {
	path := filepath.Join(round.Directory, "read-context.md")
	if data, err := os.ReadFile(path); !os.IsNotExist(err) {
		return string(data), err
	}
	for index := round.Number - 2; index >= 0; index-- {
		previous := record.Rounds[index]
		findings, _, err := runner.readFindings(record, previous)
		if err != nil {
			return "", err
		}
		if findings == "" {
			continue
		}
		var decisions string
		for _, correction := range record.Rounds[index+1 : round.Number] {
			brief, err := os.ReadFile(correction.FollowUp)
			if err != nil {
				return "", err
			}
			decisions += decisionsSection(brief, previous.Number)
		}
		diff, err := runner.diffSince(record, previous, round)
		if err != nil {
			return "", err
		}
		var proof []UnitStep
		for _, step := range round.Steps {
			if strings.HasPrefix(step.Name, "proof:") {
				proof = append(proof, step)
			}
		}
		result, _ := json.MarshalIndent(proof, "", "  ")
		packet := fmt.Sprintf("\n## Follow-up read of round %d\n\nCheck every fold first, citing the line that proves it; a fold that does not hold is the first finding. Never re-raise a refuted finding without new evidence. Seek new defects in the changed lines only. Label each finding's relation: `new`, `fold-not-holding`, or `same-rule-as N`.\n\n### Previous findings (verbatim)\n\n%s\n\n%s\n\n### Diff since round %d's tree\n\n```diff\n%s\n```\n\n### Proof result of round %d\n\n```json\n%s\n```\n", previous.Number, findings, decisions, previous.Number, diff, round.Number, result)
		_, err = atomicfile.WriteText(path, packet, runner.root())
		return packet, err
	}
	return "", nil
}

func (runner *UnitRunner) diffSince(record UnitRunRecord, previous, current UnitRound, excluded ...string) ([]byte, error) {
	directory, done, err := diskstore.ScratchDir("metasystem-unit-fold.")
	if err != nil {
		return nil, err
	}
	defer done()
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	objects, err := git.Run(record.Worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(directory, "objects"), 0o700); err != nil {
		return nil, err
	}
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(directory, "index"), "GIT_OBJECT_DIRECTORY=" + filepath.Join(directory, "objects"), "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strings.TrimSpace(string(objects))}
	var tree []byte
	for _, round := range []UnitRound{previous, current} {
		if _, err := git.Run(record.Worktree, env, "read-tree", record.Base); err != nil {
			return nil, err
		}
		path := filepath.Join(round.Directory, "worktree.diff")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			if _, err := git.Run(record.Worktree, env, "apply", "--cached", "--binary", path); err != nil {
				return nil, err
			}
		}
		if round.Number == previous.Number {
			tree, err = git.Run(record.Worktree, env, "write-tree")
			if err != nil {
				return nil, err
			}
		}
	}
	args := []string{"diff", "--cached", "--binary", strings.TrimSpace(string(tree)), "--", "."}
	for _, path := range excluded {
		args = append(args, ":(exclude,literal)"+path)
	}
	return git.Run(record.Worktree, env, args...)
}

// rebasePlan keeps every continuation on the stopped replay's tree and base.
func (runner *UnitRunner) rebasePlan(plan UnitPlan, binding *UnitRebasePlan) (UnitPlan, error) {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	for _, ref := range []struct{ name, want string }{{"HEAD", binding.Base}, {"REBASE_HEAD", binding.Commit}} {
		out, err := git.Run(binding.Worktree, nil, "rev-parse", "--verify", ref.name)
		if err != nil || strings.TrimSpace(string(out)) != ref.want {
			return UnitPlan{}, errors.Join(fmt.Errorf("resolve round no longer matches stopped rebase %s", ref.name), err)
		}
	}
	move := func(path string) string {
		rel, err := filepath.Rel(plan.Worktree, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.Join(binding.Worktree, rel)
		}
		return path
	}
	plan.Proof = append([]ProofCommand(nil), plan.Proof...)
	plan.Build.Inputs = append([]string(nil), plan.Build.Inputs...)
	plan.Build.Outputs = append([]string(nil), plan.Build.Outputs...)
	for i := range plan.Build.Inputs {
		plan.Build.Inputs[i] = move(plan.Build.Inputs[i])
	}
	for i := range plan.Build.Outputs {
		plan.Build.Outputs[i] = move(plan.Build.Outputs[i])
	}
	for i := range plan.Proof {
		plan.Proof[i].Dir = move(plan.Proof[i].Dir)
	}
	plan.Worktree, plan.Base = binding.Worktree, binding.Base
	return plan, nil
}
