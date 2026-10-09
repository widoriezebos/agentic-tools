package delegation_test

// Port of dispatch-fixtures.sh cluster c (lines 3301-3867): the critic read
// subject and read admission as the lifecycle sequences them. These run over
// the dispatch integration bed because the subject computation reads the
// reviewed workspace's tree and the design workspace's HEAD through Git
// inside internal/dispatch, where no lifecycle stub reaches.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// p5LiveTree is the reviewed workspace's projection tree, the value a
// conformance review records as reviewedTree.
func (b *bed) p5LiveTree() string {
	b.t.Helper()
	tree, err := (gittree.Workspace{Dir: b.root}).Snapshot("HEAD")
	if err != nil {
		b.t.Fatalf("snapshot: %v", err)
	}
	return tree
}

func (b *bed) p5WriteJSON(relative string, value any) {
	b.t.Helper()
	if result, ok := value.(map[string]any); ok && filepath.Base(relative) == "return.json" {
		job, _ := result["jobId"].(string)
		if job != "" && b.record(job)["role"] == "design-critic" {
			b.designReturnEvidence(filepath.Dir(relative), result)
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		b.t.Fatal(err)
	}
	b.writeFile(relative, string(encoded)+"\n")
}

func designFixturePage(content string) string {
	title, body, _ := strings.Cut(strings.TrimSpace(content), "\n")
	id := sha256.Sum256([]byte(title))
	return fmt.Sprintf("%s\n\n- Kind: design\n- Id: fixture-%x\n- Status: draft\n\n%s\n\n%s\n"+
		"| Function / record / act | Production caller | Freshness at the decision | Person or agent | Remedy that can succeed | Unreadable input |\n"+
		"| --- | --- | --- | --- | --- | --- |\n"+
		"| Fixture review | Dispatch | Frozen page | Agent | Follow-up | Refuse |\n", title, id[:8], body, readsubject.DesignInventoryHeading)
}

// A completed design examination carries both the structured return and its
// prose verdict, bound to the exact page frozen for that round.
func (b *bed) designReturnEvidence(relative string, result map[string]any) {
	b.t.Helper()
	data, err := os.ReadFile(filepath.Join(b.root, relative, "subject.json"))
	if err != nil {
		b.t.Fatal(err)
	}
	var subject dispatch.ReadSubject
	if err := json.Unmarshal(data, &subject); err != nil {
		b.t.Fatal(err)
	}
	defaults := map[string]any{
		"runtime": "fake", "sessionId": "fake-session", "mode": "design",
		"claimed":  map[string]any{"sessionId": nil, "model": nil},
		"model":    map[string]any{"requested": "fake-model", "effective": "fake-model"},
		"evidence": []any{map[string]any{"command": "fixture page examination", "observed": "frozen page reviewed", "level": "read"}},
		"gaps":     []any{}, "findings": []any{}, "rigor": []any{},
	}
	result["schemaVersion"] = 5
	for key, value := range defaults {
		if _, present := result[key]; !present {
			result[key] = value
		}
	}
	result["wholePageDigest"] = subject.ContentDigest
	answers := []any{}
	for question := 1; question <= 5; question++ {
		answers = append(answers, map[string]any{"question": question, "answer": "The fixture review row declares this boundary", "evidence": "Frozen inventory table", "unanswered": false})
	}
	result["coverage"] = []any{map[string]any{"row": "Fixture review", "where": readsubject.DesignInventoryHeading, "answers": answers}}
	count := 0
	rigor := result["rigor"].([]any)
	classified := map[any]bool{}
	for _, entry := range rigor {
		classified[entry.(map[string]any)["findingId"]] = true
	}
	for _, entry := range result["findings"].([]any) {
		finding := entry.(map[string]any)
		for key, value := range map[string]any{"class": "incomplete-item", "where": strings.Split(subject.DesignPage, "\n")[0], "change": "Supply the fixture's missing admission rule", "coverage": "", "relation": ""} {
			if _, present := finding[key]; !present {
				finding[key] = value
			}
		}
		if finding["material"] == true {
			count++
			if !classified[finding["id"]] {
				rigor = append(rigor, map[string]any{
					"findingId": finding["id"], "rigorClass": "bounded", "reopeningTrigger": "The fixture admission rule is absent",
					"artifact": "metasystem/internal/dispatch/build.go", "grain": "invariant", "behaviour": "", "fixture": "",
					"facts": map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
						"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false},
				})
			}
		}
	}
	result["rigor"] = rigor
	record := b.record(result["jobId"].(string))
	if parent, _ := record["parentJob"].(string); parent != "" && record["examinationRetryOf"] == nil {
		b.designRevisionDecisions(relative, result["jobId"].(string), parent)
	}
	if _, present := result["verdictMaterialCount"]; !present {
		result["verdictMaterialCount"] = count
	}
	b.writeFile(filepath.Join(relative, "return.md"), fmt.Sprintf("VERDICT: REVISE material=%d\n", count))
}

func (b *bed) designRevisionDecisions(relative, job, parent string) {
	b.t.Helper()
	root := filepath.Base(filepath.Dir(filepath.Dir(relative)))
	after, err := strconv.Atoi(fmt.Sprint(b.record(parent)["round"]))
	if err != nil {
		b.t.Fatal(err)
	}
	dir := filepath.Join("artifacts", "agents", root, "rounds", strconv.Itoa(after))
	var previous readsubject.Read
	if err := json.Unmarshal([]byte(b.readFile(filepath.Join(dir, "read.json"))), &previous); err != nil {
		b.t.Fatal(err)
	}
	returned := []byte(b.readFile(filepath.Join(dir, "return.json")))
	answer := fmt.Sprintf("Review binding: fixture work=design:%s attempt=%d subject=%s examination=%s round=%d return=%x\n",
		previous.Design.RecordID, after, previous.Subject.ContentDigest, root, after, sha256.Sum256(returned))
	brief := b.writeFile("artifacts/briefs/section-decisions-"+job+".md", fmt.Sprintf("\n## The author's decisions on examination %d\n\nThe design changed since examination %d. Judge whether each accepted finding is addressed in the new version.\n\n", after, after)+answer)
	path := filepath.Join("artifacts", "agents", "intent-review", "design-"+strings.ToLower(previous.Design.RecordID), "chain.json")
	entry := map[string]any{"Requests": []any{}}
	if b.exists(path) {
		if err := json.Unmarshal([]byte(b.readFile(path)), &entry); err != nil {
			b.t.Fatal(err)
		}
	}
	entry["Requests"] = append(entry["Requests"].([]any), map[string]any{"Root": root, "Child": job, "AfterRound": after, "Brief": brief, "DecisionsSHA256": fmt.Sprintf("%x", sha256.Sum256([]byte(answer)))})
	b.p5WriteJSON(path, entry)
}

// p5SeedImplementer writes a completed, conformance-reviewed implementer
// round over the bed's own checkout and returns the live subject a critic of
// it reads.
func (b *bed) p5SeedImplementer(job, tree string) dispatch.ReadSubject {
	b.t.Helper()
	b.writeRecord(job, map[string]any{
		"status": "completed", "round": 1, "parentJob": nil, "role": "implementer",
		"workspaceRoot": b.root, "launchMode": "shared-checkout", "sessionId": "session-" + job,
	})
	patch := "diff --git a/metasystem/review-target.txt b/metasystem/review-target.txt\n"
	b.p5WriteJSON("artifacts/agents/"+job+"/rounds/1/review.json", map[string]any{"reviewedTree": tree})
	b.writeFile("artifacts/agents/"+job+"/rounds/1/diff.patch", patch)
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{RepoRoot: b.root, Role: "code-critic", Reviews: job})
	if err != nil || !present {
		b.t.Fatalf("seeded implementer subject: %+v present=%v err=%v", subject, present, err)
	}
	return subject
}

// p5SeedCritic writes a critic root that read subject at round one: folded
// clean (its register proves the read) or still unfolded with status.
func (b *bed) p5SeedCritic(root, role, status string, subject dispatch.ReadSubject, clean bool) {
	b.t.Helper()
	record := map[string]any{
		"status": status, "round": 1, "parentJob": nil, "role": role,
		"findingRegister": []any{}, "findingRegisterRound": 0,
		"capabilitySnapshot": "artifacts/agents/capabilities/absent-snapshot.json",
	}
	if subject.Kind == dispatch.SubjectDesign {
		record["design"] = subject.DesignPath
		record["declaredOutputsDigest"] = subject.DeclaredOutputsDigest
	} else {
		record["reviews"] = subject.ReviewedMember
	}
	if clean {
		record["findingRegisterRound"] = 1
		record["findingRegisterSubjectDigest"] = subject.Digest()
		record["cleanReadRounds"] = []any{map[string]any{"round": 1, "subject": subject}}
	}
	b.writeRecord(root, record)
	roundDir := filepath.Join(b.root, "artifacts", "agents", root, "rounds", "1")
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := dispatch.WriteReadSubject(filepath.Join(roundDir, "subject.json"), subject); err != nil {
		b.t.Fatal(err)
	}
	result := map[string]any{"schemaVersion": 3, "jobId": root, "round": 1, "findings": []any{}, "rigor": []any{}}
	if subject.Kind == dispatch.SubjectDesign {
		result["reviewedCommit"] = subject.ReviewedCommit
	} else {
		result["reviewedTree"] = subject.ReviewedProjectTree
	}
	b.p5WriteJSON("artifacts/agents/"+root+"/rounds/1/return.json", result)
}

var p5Digest = regexp.MustCompile(`[0-9a-f]{64}`)

// p5RequireNothingPublished is assert_read_refusal's footprint half: a
// refused read leaves no candidate record, payload, private subject or
// admission result, process-creation claim, or launch authorization.
func (b *bed) p5RequireNothingPublished(candidate string) {
	b.t.Helper()
	agents := filepath.Join(b.root, "artifacts", "agents")
	for _, path := range []string{filepath.Join(agents, "jobs", candidate+".json"), filepath.Join(agents, candidate)} {
		if exists(path) {
			b.t.Fatalf("the refused read published %s", path)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(agents, "record-locks"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "subject.") || strings.HasPrefix(entry.Name(), "read-admission.") {
			b.t.Fatalf("the refused read left %s behind", entry.Name())
		}
	}
	if exists(filepath.Join(agents, "locks", candidate+".d")) {
		b.t.Fatalf("the refused read kept its chain lock")
	}
	if creating, _ := os.ReadDir(filepath.Join(agents, "supervision", "creating")); len(creating) != 0 {
		b.t.Fatalf("the refused read left a process-creation claim: %v", creating)
	}
	claim := regexp.MustCompile(`"jobId"\s*:\s*"` + regexp.QuoteMeta(candidate) + `"`)
	_ = filepath.Walk(filepath.Join(agents, "capabilities"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			if content, _ := os.ReadFile(path); claim.Match(content) {
				b.t.Fatalf("the refused read allocated a launch authorization at %s", path)
			}
		}
		return nil
	})
}

// p5RequireReadRefusal is assert_read_refusal: exit 11 naming the reason,
// the prior critic root, its round and the subject digest, with nothing of
// the candidate published.
func (b *bed) p5RequireReadRefusal(result delegation.Result, reason, priorRoot, candidate string) {
	b.t.Helper()
	stderr := b.stderr.String()
	requireExit(b.t, result, 11, stderr)
	for _, want := range []string{"critic root " + priorRoot, "round 1"} {
		if !strings.Contains(stderr, want) {
			b.t.Fatalf("refusal %q does not carry %q", stderr, want)
		}
	}
	// The code is the outcome's data, never the words a person reads.
	if outcome := outcomeOf(b.t, result); strings.Contains(stderr, reason) || outcome["code"] != reason {
		b.t.Fatalf("refusal %q or outcome %v misplaces the code %s", stderr, outcome, reason)
	}
	if !p5Digest.MatchString(stderr) {
		b.t.Fatalf("refusal %q names no subject digest", stderr)
	}
	b.p5RequireNothingPublished(candidate)
}

func (b *bed) p5DispatchCritic(env delegation.Env, candidate, reviews string) delegation.Result {
	b.t.Helper()
	brief := b.brief("artifacts/briefs/code-"+candidate+".md", "implement", "Review the implementation.")
	return b.runEnv(env, "dispatch", "--role", "code-critic", "--brief", brief,
		"--reviews", reviews, "--runtime", "fake", "--job-id", candidate)
}

func (b *bed) p5Capabilities() []string {
	b.t.Helper()
	var files []string
	_ = filepath.Walk(filepath.Join(b.root, "artifacts", "agents", "capabilities"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// subject-mismatch (3318-3335): a reviewed workspace that changed after its
// conformance review refuses the critic before any record or payload.
func TestPortP5DispatchIntegrationSubjectMismatchRefusesBeforeReservation(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p5SeedImplementer("review-target", b.p5LiveTree())
	b.writeFile("metasystem/review-target.txt", "workspace changed after review\n")
	result := b.p5DispatchCritic(b.dispatchEnv("fresh"), "subject-mismatch", "review-target")
	requireExit(t, result, 11, b.stderr.String())
	if strings.Contains(b.stderr.String(), "SUBJECT_MISMATCH") || !strings.Contains(b.stderr.String(), "a read now would review a tree nobody recorded") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	b.p5RequireNothingPublished("subject-mismatch")
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" || outcome["code"] != "SUBJECT_MISMATCH" {
		t.Fatalf("outcome %v", outcome)
	}
}

// concurrent-live, concurrent-completed and redundant-live-fresh
// (3415-3468): one live target exercises both unfolded states. A running
// critic read (in its own worktree on its own branch) refuses a second read
// of the same subject with CONCURRENT_READ and spends no launch
// authorization; once it is cancelled, a completed read still refuses until
// its fold, after which the same request is a REDUNDANT_READ whose event is
// anchored only on the prior critic root.
func TestPortP5DispatchIntegrationUnfoldedReadsRefuseConcurrentThenRedundant(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.doubles.Adapter.LaunchFunc = b.p5LaunchTaggedSupervisor
	tree := b.p5LiveTree()
	b.p5SeedImplementer("concurrent-target", tree)
	agents := filepath.Join(b.root, "artifacts", "agents")

	brief := b.brief("artifacts/briefs/running.md", "implement", "Review the implementation.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "code-critic", "--brief", brief,
		"--reviews", "concurrent-target", "--runtime", "fake", "--job-id", "concurrent-running", "--worktree"), 0, b.stderr.String())
	workspace, _ := b.record("concurrent-running")["workspaceRoot"].(string)
	if !isDirP5(workspace) || b.git("rev-parse", "--verify", "--quiet", "refs/heads/agent/concurrent-running") == "" {
		t.Fatalf("concurrent-running did not establish its live workspace %q and branch", workspace)
	}
	env := b.dispatchEnv("fresh")
	capsBefore := b.p5Capabilities()
	result := b.p5DispatchCritic(env, "concurrent-live", "concurrent-target")
	b.p5RequireReadRefusal(result, "CONCURRENT_READ", "concurrent-running", "concurrent-live")
	if caps := b.p5Capabilities(); strings.Join(caps, "\n") != strings.Join(capsBefore, "\n") {
		t.Fatalf("concurrent-live changed the launch authorizations: %v", caps)
	}
	cancelled := b.writeFile("artifacts/briefs/cancelled.json", `{"phase":"cancelled","error":null}`)
	if _, err := dispatch.RecordCAS(b.root, "concurrent-running", "running", "cancelled", cancelled); err != nil {
		t.Fatal(err)
	}

	requireExit(t, b.p5DispatchCritic(b.dispatchEnv("fresh"), "concurrent-completed", "concurrent-target"), 0, b.stderr.String())
	b.p5Complete("concurrent-completed", "concurrent-completed", 1, map[string]any{"reviewedTree": tree, "findings": []any{}, "rigor": []any{}})
	result = b.p5DispatchCritic(b.dispatchEnv("fresh"), "concurrent-completed-candidate", "concurrent-target")
	b.p5RequireReadRefusal(result, "CONCURRENT_READ", "concurrent-completed", "concurrent-completed-candidate")
	if exists(filepath.Join(agents, "concurrent-completed", "reads-refused.jsonl")) {
		t.Fatal("a concurrent refusal recorded a redundant-read event")
	}

	if _, err := dispatch.CritiqueRegisterAdvance(b.root, "concurrent-completed", "concurrent-completed"); err != nil {
		t.Fatalf("fold: %v", err)
	}
	result = b.p5DispatchCritic(b.dispatchEnv("fresh"), "redundant-live-fresh", "concurrent-target")
	b.p5RequireReadRefusal(result, "REDUNDANT_READ", "concurrent-completed", "redundant-live-fresh")
	if !fileNonEmptyP5(filepath.Join(agents, "concurrent-completed", "reads-refused.jsonl")) ||
		exists(filepath.Join(agents, "redundant-live-fresh", "reads-refused.jsonl")) ||
		exists(filepath.Join(agents, "concurrent-target", "reads-refused.jsonl")) {
		t.Fatal("the live redundant event was not anchored only on the prior critic root")
	}
}

func isDirP5(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// p5DesignPage commits a design page and returns its path and the commit.
func (b *bed) p5DesignPage(page, content string) string {
	b.t.Helper()
	b.writeFile(page, designFixturePage(content))
	b.git("add", "--", page)
	b.git("commit", "-qm", "add "+page)
	return b.git("rev-parse", "HEAD")
}

func (b *bed) p5DesignSubject(page, outputs string) dispatch.ReadSubject {
	b.t.Helper()
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{
		RepoRoot: b.root, Role: "design-critic", Workspace: b.root, Design: page, DeclaredOutputs: outputs,
	})
	if err != nil || !present {
		b.t.Fatalf("design subject: %+v %v %v", subject, present, err)
	}
	return subject
}

const p5Outputs = "metasystem/internal/dispatch/build.go\n"

// redundant-design-fresh (3470-3494): design reuse is scoped by the design
// path; the event is anchored on the prior critic root, never the proposed.
func TestPortP5DispatchIntegrationRedundantDesignReadRefusesAndAnchorsOnThePriorRoot(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	page := "metasystem/fixture-admission/redundant-fresh-design.md"
	commit := b.p5DesignPage(page, "# Redundant fresh design\n")
	outputs := b.writeFile("artifacts/briefs/outputs.md", p5Outputs)
	subject := b.p5DesignSubject(page, outputs)
	if subject.ReviewedCommit != commit {
		t.Fatalf("the design subject reviews %s, not the commit holding its page %s", subject.ReviewedCommit, commit)
	}
	b.p5SeedCritic("redundant-design-root", "design-critic", "completed", subject, true)
	brief := b.brief("artifacts/briefs/design.md", "design", "Critique the design.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", page, "--brief", brief, "--job-id", "redundant-design-fresh")
	b.p5RequireReadRefusal(result, "REDUNDANT_READ", "redundant-design-root", "redundant-design-fresh")
	agents := filepath.Join(b.root, "artifacts", "agents")
	if !fileNonEmptyP5(filepath.Join(agents, "redundant-design-root", "reads-refused.jsonl")) ||
		exists(filepath.Join(agents, "redundant-design-fresh", "reads-refused.jsonl")) {
		t.Fatal("the design redundant event was not anchored only on the prior critic root")
	}
}

// refusal-durable (3496-3542): a redundant refusal stays exit 11 when its
// immediate mirror of the prior root fails; the local event is intact and
// the ordinary mirror repairs the evidence afterwards.
func TestPortP5DispatchIntegrationRedundantRefusalSurvivesAFailedMirror(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	evidence := t.TempDir()
	b.writeFile("metasystem.conf", dispatchBedConfig+"evidence.root="+evidence+"\n")
	b.armSupervision()
	page := "metasystem/fixture-admission/refusal-durable.md"
	b.p5DesignPage(page, "# Refusal durability design\n")
	outputs := b.writeFile("artifacts/briefs/outputs.md", p5Outputs)
	subject := b.p5DesignSubject(page, outputs)
	b.p5SeedCritic("refusal-durable-root", "design-critic", "completed", subject, true)
	b.writeFile("artifacts/agents/refusal-durable-root/.mirror-fail-once", "")
	brief := b.brief("artifacts/briefs/design.md", "design", "Critique the design.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", page, "--brief", brief, "--job-id", "refusal-durable-candidate")
	b.p5RequireReadRefusal(result, "REDUNDANT_READ", "refusal-durable-root", "refusal-durable-candidate")
	payload := filepath.Join(b.root, "artifacts", "agents", "refusal-durable-root")
	local := filepath.Join(payload, "reads-refused.jsonl")
	content, err := os.ReadFile(local)
	if err != nil || strings.Count(string(content), "\n") != 1 || !exists(filepath.Join(payload, ".mirror-failed")) {
		t.Fatalf("the local event or the scripted mirror failure is missing: %q %v", content, err)
	}
	if !strings.Contains(b.stderr.String(), "cannot mirror refusal-durable-root") {
		t.Fatalf("the mirror failure was not reported: %q", b.stderr.String())
	}
	// The ordinary mirror repairs it; the mirrored file is the local one.
	mirrorResult := filepath.Join(t.TempDir(), "mirror.json")
	if err := dispatch.Mirror(b.root, b.root, evidence, "refusal-durable-root", "refusal-durable-root", mirrorResult); err != nil {
		t.Fatalf("repair mirror: %v", err)
	}
	var mirrored struct {
		Path string `json:"path"`
	}
	raw, _ := os.ReadFile(mirrorResult)
	if err := json.Unmarshal(raw, &mirrored); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(mirrored.Path, "reads-refused.jsonl"))
	if err != nil || string(copied) != string(content) {
		t.Fatalf("the repaired mirror does not carry the one local refusal event: %q %v", copied, err)
	}
}

func fileNonEmptyP5(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// p5Complete concludes a launched round as its adapter would: the record
// moves to completed and the round's return is written.
func (b *bed) p5Complete(job, root string, round int, result map[string]any) {
	b.t.Helper()
	patch := b.writeFile("artifacts/briefs/complete-"+job+".json", `{"error":null,"phase":"completed"}`)
	if _, err := dispatch.RecordCAS(b.root, job, "running", "completed", patch); err != nil {
		b.t.Fatalf("complete %s: %v", job, err)
	}
	result["schemaVersion"], result["jobId"], result["round"] = 3, job, round
	b.p5WriteJSON(filepath.Join("artifacts/agents", root, "rounds", strconv.Itoa(round), "return.json"), result)
}

func (b *bed) p5ReadSubjectFile(root string, round int) dispatch.ReadSubject {
	b.t.Helper()
	raw, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", root, "rounds", strconv.Itoa(round), "subject.json"))
	if err != nil {
		b.t.Fatalf("round %d of %s persisted no subject: %v", round, root, err)
	}
	var subject dispatch.ReadSubject
	if err := json.Unmarshal(raw, &subject); err != nil {
		b.t.Fatal(err)
	}
	return subject
}

// subject-persisted (3304-3316): a launched critic persists its read
// subject in the round directory: the live subject names the implementer
// root and the reviewed tree, a commit subject its exact tree.
func TestPortP5DispatchIntegrationCriticRoundsPersistTheirReadSubject(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	tree := b.p5LiveTree()
	b.p5SeedImplementer("review-target", tree)
	requireExit(t, b.p5DispatchCritic(b.dispatchEnv("fresh"), "flag-runtime", "review-target"), 0, b.stderr.String())
	live := b.p5ReadSubjectFile("flag-runtime", 1)
	if live.Kind != dispatch.SubjectLive || live.ImplementerRoot != "review-target" || live.ReviewedProjectTree != tree {
		t.Fatalf("live subject %+v", live)
	}
	if record := b.record("flag-runtime"); record["reviews"] != "review-target" || record["runtime"] != "fake" {
		t.Fatalf("critic record %v", record)
	}

	b.writeFile("metasystem/commit-subject.txt", "commit subject fixture\n")
	b.git("add", "--", "metasystem/commit-subject.txt")
	b.git("commit", "-qm", "commit subject fixture")
	commit := b.git("rev-parse", "HEAD")
	commitTree := b.git("rev-parse", "HEAD^{tree}")
	requireExit(t, b.p5DispatchCritic(b.dispatchEnv("fresh"), "commit-subject", "commit:"+commit), 0, b.stderr.String())
	subject := b.p5ReadSubjectFile("commit-subject", 1)
	if subject.Kind != dispatch.SubjectCommit || subject.Tree != commitTree || subject.Commit != commit {
		t.Fatalf("commit subject %+v, want tree %s", subject, commitTree)
	}
}

// redundant-follow-up and changed-design-follow-up (3364-3413): a follow-up
// of a design read whose critical finding was refuted refuses over an
// unchanged page before the review budget, round namespace or launch
// authorization advances;
// once the page changes, the same follow-up launches round two over the
// changed subject at the same reviewed commit.
func TestPortP5DispatchIntegrationRedundantFollowUpRefusesUntilTheDesignChanges(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	page := "metasystem/fixture-admission/redundant-follow-up.md"
	commit := b.p5DesignPage(page, "# Redundant follow-up design\n")
	outputs := b.writeFile("artifacts/briefs/outputs.md", p5Outputs)
	brief := b.brief("artifacts/briefs/design.md", "design", "Critique the design.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", page, "--brief", brief, "--job-id", "redundant-follow-up"), 0, b.stderr.String())
	first := b.p5ReadSubjectFile("redundant-follow-up", 1)
	if first.ReviewedCommit != commit {
		t.Fatalf("round one reviews %s, not the commit holding its page %s", first.ReviewedCommit, commit)
	}
	b.p5Complete("redundant-follow-up", "redundant-follow-up", 1, b.criticalDesignReturn("redundant-follow-up"))
	if _, err := dispatch.CritiqueRegisterAdvance(b.root, "redundant-follow-up", "redundant-follow-up"); err != nil {
		t.Fatalf("fold round one: %v", err)
	}
	if err := dispatch.CritiqueRegisterApplyDecisions(b.root, "redundant-follow-up", map[string]string{"DESIGN-1": "refuted"}); err != nil {
		t.Fatalf("refute the critical finding: %v", err)
	}
	record := b.record("redundant-follow-up")
	payload := filepath.Join(b.root, "artifacts", "agents", "redundant-follow-up")
	subjectBefore, _ := os.ReadFile(filepath.Join(payload, "rounds", "1", "subject.json"))
	returnBefore, _ := os.ReadFile(filepath.Join(payload, "rounds", "1", "return.json"))
	message := b.writeFile("artifacts/briefs/follow.md", "Working Mode: design\n\nLook again.\n")

	env := b.dispatchEnv("follow-up")
	capsBefore := b.p5Capabilities()
	result := b.runEnv(env, "follow-up", "--job", "redundant-follow-up", "--message", message)
	b.p5RequireReadRefusal(result, "REDUNDANT_READ", "redundant-follow-up", "redundant-follow-up-r2")
	if exists(filepath.Join(payload, "rounds", "2")) {
		t.Fatal("the redundant follow-up created round two")
	}
	after := b.record("redundant-follow-up")
	for _, key := range []string{"criticRoundsConsumed", "critiqueExhaustions"} {
		if !reflectEqualP5(after[key], record[key]) {
			t.Fatalf("the redundant follow-up spent %s: %v -> %v", key, record[key], after[key])
		}
	}
	subjectAfter, _ := os.ReadFile(filepath.Join(payload, "rounds", "1", "subject.json"))
	returnAfter, _ := os.ReadFile(filepath.Join(payload, "rounds", "1", "return.json"))
	if string(subjectAfter) != string(subjectBefore) || string(returnAfter) != string(returnBefore) {
		t.Fatal("the redundant follow-up rewrote round one's subject or return")
	}
	if caps := b.p5Capabilities(); strings.Join(caps, "\n") != strings.Join(capsBefore, "\n") {
		t.Fatalf("the redundant follow-up changed the launch authorizations: %v", caps)
	}

	workspace := b.record("redundant-follow-up")["workspaceRoot"].(string)
	changedPath := filepath.Join(workspace, page)
	f, err := os.OpenFile(changedPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\nChanged design content.\n")
	_ = f.Close()
	changed, _ := sha256FileP5(changedPath)
	result = b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "redundant-follow-up", "--message", message)
	requireExit(t, result, 0, b.stderr.String())
	second := b.p5ReadSubjectFile("redundant-follow-up", 2)
	if second.ContentDigest != changed || second.ContentDigest == first.ContentDigest || second.ReviewedCommit != commit {
		t.Fatalf("round two subject %+v, want content %s at %s", second, changed, commit)
	}
	if child := b.record("redundant-follow-up-r2"); child["parentJob"] != "redundant-follow-up" || fmt.Sprint(child["round"]) != "2" {
		t.Fatalf("round two record %v", child)
	}
}

func reflectEqualP5(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func sha256FileP5(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// cap-driver and cap-warden terminal exhaustion (3636-3670, 3699-3738): a
// code critic and a warden whose first round raised a severe finding run
// through their fifth round, the cap (Wido 2026-10-01); the sixth-round
// follow-up refuses before any successor record or payload exists, and
// closing the chain refuses with the human's next steps. The warden reviews
// with network denied.
func TestPortP5DispatchIntegrationTerminalExhaustionRefusesBeforeTheRoundPastTheCap(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"code-critic", "warden"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			b := newDispatchBed(t)
			tree := b.p5LiveTree()
			b.p5SeedImplementer("review-target", tree)
			root := "cap-" + role
			brief := b.brief("artifacts/briefs/cap.md", "implement", "Review the implementation.")
			requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", role, "--brief", brief,
				"--reviews", "review-target", "--job-id", root), 0, b.stderr.String())
			if network := fmt.Sprint(b.record(root)["permissions"].(map[string]any)["requested"].(map[string]any)["network"]); network != "deny" {
				t.Fatalf("%s requested network %s, want deny", role, network)
			}
			facts := map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
				"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
			b.p5Complete(root, root, 1, map[string]any{
				"reviewedTree": tree,
				"findings":     []any{map[string]any{"id": "CAP-1", "severity": "high", "material": true, "claim": "cap finding", "evidence": "direct fixture evidence"}},
				"rigor": []any{map[string]any{"findingId": "CAP-1", "rigorClass": "severe", "facts": facts,
					"artifact": "metasystem/review-target.txt", "reopeningTrigger": "reopen if it recurs"}},
			})
			message := b.writeFile("artifacts/briefs/follow.md", "Working Mode: implement\n\nLook again.\n")
			parent := root
			for round := 2; round <= 5; round++ {
				result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", parent, "--message", message)
				requireExit(t, result, 0, b.stderr.String())
				parent = fmt.Sprintf("%s-r%d", root, round)
				b.p5Complete(parent, root, round, map[string]any{"reviewedTree": tree, "findings": []any{}, "rigor": []any{}})
			}
			result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", parent, "--message", message)
			if result.ExitCode != 10 || !strings.Contains(b.stderr.String(), "review-round limit is exhausted") {
				t.Fatalf("round six was not refused as terminal exhaustion: exit %d stderr %q", result.ExitCode, b.stderr.String())
			}
			if exists(b.recordPath(root+"-r6")) || exists(filepath.Join(b.root, "artifacts", "agents", root, "rounds", "6")) {
				t.Fatal("the pre-reservation cap refusal stranded a round-six record or payload")
			}
			result = b.run("close", "--job", root)
			if result.ExitCode != 1 || !strings.Contains(b.stderr.String(),
				"a person accepts each risk with metasystem goal accept-risk, or raises the goal's budget with metasystem goal budget") {
				t.Fatalf("close did not refuse with the human next steps: exit %d stderr %q", result.ExitCode, b.stderr.String())
			}
			if b.record(root)["chainClosed"] == true {
				t.Fatal("a refused close marked the chain closed")
			}
		})
	}
}

// A design examination continues while material counts fall, regardless of
// severity. Equal counts stop it, and the frozen allowance never exceeds four.
func TestPortP5DispatchIntegrationDesignRoundsStopOnEqualCountsAndRespectTheFrozenLimit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, severity             string
		configured, frozen, rounds int
	}{
		{"without-critical", "medium", 5, 4, 2},
		{"with-critical", "critical", 5, 4, 2},
		{"critical-with-one-round-budget", "critical", 1, 1, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newDispatchBed(t)
			b.setConf(fmt.Sprintf("metasystem.budget.review-round-max=%d\n", row.configured))
			page := "metasystem/fixture-admission/design-round-two.md"
			b.p5DesignPage(page, "# Design round two fixture\n")
			outputs := b.writeFile("artifacts/briefs/outputs.md", p5Outputs)
			brief := b.brief("artifacts/briefs/design.md", "design", "Critique the design.")
			root := "design-round-two-fixture"
			requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
				"--design", page, "--brief", brief, "--runtime", "fake", "--job-id", root), 0, b.stderr.String())
			if limit := fmt.Sprint(b.record(root)["reviewRoundLimit"]); limit != strconv.Itoa(row.frozen) {
				t.Fatalf("design critic froze reviewRoundLimit=%s, want %d", limit, row.frozen)
			}
			commit := b.p5ReadSubjectFile(root, 1).ReviewedCommit
			facts := map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
				"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
			bounded := func(id, fixture string) map[string]any {
				return map[string]any{"findingId": id, "rigorClass": "bounded", "facts": facts, "reopeningTrigger": "if " + id + " recurs",
					"artifact": "metasystem/internal/dispatch/build.go", "grain": "mechanical", "behaviour": id, "fixture": fixture}
			}
			finding := func(id, severity string, material bool) map[string]any {
				return map[string]any{"id": id, "severity": severity, "material": material, "claim": id, "evidence": "read"}
			}
			b.p5Complete(root, root, 1, map[string]any{
				"reviewedCommit": commit, "verdictMaterialCount": 2,
				"findings": []any{finding("ROUND1-A", row.severity, true), finding("ROUND1-B", "medium", true)},
				"rigor":    []any{bounded("ROUND1-A", "go test -timeout 30m ./old-a"), bounded("ROUND1-B", "go test -timeout 30m ./old-b")},
			})
			if _, err := dispatch.CritiqueRegisterAdvance(b.root, root, root); err != nil {
				t.Fatalf("fold round one: %v", err)
			}
			workspace := b.record(root)["workspaceRoot"].(string)
			message := b.writeFile("artifacts/briefs/follow.md", "Working Mode: design\n\nLook again.\n")
			parent := root
			trajectory := `[{"material":2,"round":1}]`
			b.writeFileAbs(filepath.Join(workspace, page), designFixturePage("# Design round two fixture\n\nRound two changes the page.\n"))
			if row.rounds == 2 {
				requireExit(t, b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", parent, "--message", message), 0, b.stderr.String())
				parent = root + "-r2"
				b.p5Complete(parent, root, 2, map[string]any{
					"reviewedCommit": commit, "verdictMaterialCount": 2,
					"findings": []any{finding("ROUND1-A", "low", false), finding("ROUND1-B", "low", false), finding("ROUND2-FIXTURE", "critical", true), finding("ROUND2-OTHER", "medium", true)},
					"rigor":    []any{bounded("ROUND2-FIXTURE", "go test -timeout 30m ./internal/dispatch/ -run TestRoundTwoCloseMechanicalFallingUsesOwnFixtures"), bounded("ROUND2-OTHER", "go test -timeout 30m ./internal/dispatch/ -run TestDesignCriticLimitIsGoalMemberUnderCeiling")},
				})
				if _, err := dispatch.CritiqueRegisterAdvance(b.root, root, parent); err != nil {
					t.Fatalf("fold round two: %v", err)
				}
				trajectory = `[{"material":2,"round":1},{"material":2,"round":2}]`
				b.writeFileAbs(filepath.Join(workspace, page), designFixturePage("# Design round two fixture\n\nRound three changes the page.\n"))
			}
			data, err := json.Marshal(b.record(root)["designExaminations"])
			var examinations []readsubject.Read
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &examinations); err != nil {
				t.Fatal(err)
			}
			var materials []map[string]int
			for index, read := range examinations {
				materials = append(materials, map[string]int{"round": index + 1, "material": read.Material})
			}
			if got, _ := json.Marshal(materials); string(got) != trajectory {
				t.Fatalf("material trajectory %s, want %s", got, trajectory)
			}
			result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", parent, "--message", message)
			requireExit(t, result, 1, b.stderr.String())
			reason := "material findings did not fall"
			if row.rounds == row.frozen {
				reason = "correction allowance spent"
			}
			decision := b.record(root)["designDecision"].(map[string]any)
			if decision["decision"] != "stop" || decision["class"] != reason {
				t.Fatalf("design decision %v does not retain the stop reason %q", decision, reason)
			}
			if !strings.Contains(b.stderr.String(), "the design examination cannot continue; run metasystem design review '") || !strings.Contains(b.stderr.String(), "--dispositions FILE") {
				t.Fatalf("the stopped design named no executable exit: %q", b.stderr.String())
			}
			next := strconv.Itoa(row.rounds + 1)
			if exists(b.recordPath(root+"-r"+next)) || exists(filepath.Join(b.root, "artifacts", "agents", root, "rounds", next)) {
				t.Fatalf("the round-%s refusal created a successor record or payload", next)
			}
			if limit := fmt.Sprint(b.record(root)["reviewRoundLimit"]); limit != strconv.Itoa(row.frozen) {
				t.Fatalf("the chain changed its frozen review budget to %s", limit)
			}
		})
	}
}

func (b *bed) writeFileAbs(path, content string) {
	b.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// repeat-follow (3739-3802): a second wrapper of the same follow-up claims
// the round the first one published instead of reserving round three; the
// same operation id with changed input refuses REFUSED-OPID-MISMATCH and
// leaves the running round's staged task direction untouched.
func TestPortP5DispatchIntegrationRepeatedFollowUpClaimsTheLiveRound(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	page := "metasystem/fixture-admission/repeat-follow.md"
	b.p5DesignPage(page, "# Repeat follow design\n")
	outputs := b.writeFile("artifacts/briefs/outputs.md", page+"\n")
	brief := b.brief("artifacts/briefs/design.md", "design", "Critique the design.")
	root := "repeat-follow"
	// The claim of a live round proves the recorded process by its tag in
	// the adapter supervisor's argv position, so this bed's supervisor
	// carries the fake adapter's shape.
	b.doubles.Adapter.LaunchFunc = b.p5LaunchTaggedSupervisor
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", page, "--brief", brief, "--job-id", root), 0, b.stderr.String())
	commit := b.p5ReadSubjectFile(root, 1).ReviewedCommit
	facts := map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
	b.p5Complete(root, root, 1, map[string]any{
		"reviewedCommit": commit,
		"findings":       []any{map[string]any{"id": "R-1", "severity": "critical", "material": true, "claim": "r1", "evidence": "read"}},
		"rigor": []any{map[string]any{"findingId": "R-1", "rigorClass": "bounded", "facts": facts, "reopeningTrigger": "if it recurs",
			"artifact": "metasystem/internal/dispatch/build.go", "grain": "invariant", "behaviour": "", "fixture": ""}},
	})
	workspace := b.record(root)["workspaceRoot"].(string)
	b.writeFileAbs(filepath.Join(workspace, page), designFixturePage("# Repeat follow design\n\nThe admission rule is revised.\n"))
	message := b.writeFile("artifacts/briefs/follow.md", "Working Mode: design\n\nLook again.\n"+strings.Repeat("filler line for the forty kibibyte referenced follow-up\n", 740))
	requireExit(t, b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", root, "--message", message), 0, b.stderr.String())
	child := b.record(root + "-r2")
	if child["status"] != "running" {
		t.Fatalf("round two %v", child["status"])
	}
	stage := filepath.Join(b.root, "artifacts", "agents", root, "rounds", "2", "staged", "task-direction.md")
	stageBefore, err := os.ReadFile(stage)
	if err != nil {
		t.Fatalf("round two staged no task direction: %v", err)
	}

	result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", root, "--message", message)
	if result.ExitCode != 0 && result.ExitCode != 3 {
		t.Fatalf("the repeated wrapper failed before claim-launch: exit %d stderr %q stdout %q outcome %q", result.ExitCode, b.stderr.String(), result.Stdout, result.Outcome)
	}
	if outcome := outcomeOf(t, result)["outcome"]; outcome != "BOUND" && outcome != "IN-PROGRESS" {
		t.Fatalf("the repeated wrapper returned %v, not a claim state", outcome)
	}
	if exists(b.recordPath(root + "-r3")) {
		t.Fatal("the repeated follow-up reserved round three")
	}

	changed := b.writeFile("artifacts/briefs/changed.md", string(stageBefore)+"\nThis changes the running follow-up input.\n")
	result = b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", root, "--message", changed,
		"--operation-id", fmt.Sprint(child["operationId"]))
	if result.ExitCode == 0 || outcomeOf(t, result)["outcome"] != "REFUSED-OPID-MISMATCH" {
		t.Fatalf("changed input under the running operation id: exit %d outcome %q", result.ExitCode, result.Outcome)
	}
	if stageAfter, _ := os.ReadFile(stage); string(stageAfter) != string(stageBefore) {
		t.Fatal("a refused follow-up changed the running round's staged task direction")
	}
	if exists(b.recordPath(root + "-r3")) {
		t.Fatal("the refused follow-up reserved round three")
	}
}

// p5LaunchTaggedSupervisor is launchInertSupervisor with the fake adapter's
// argv shape (delegate-supervisor fake VERB --instance-tag TAG) on an inert shell.
func (b *bed) p5LaunchTaggedSupervisor(request delegation.AdapterLaunch) (int64, error) {
	command := exec.Command("/bin/sh", "-c", "sleep 120; :", "delegate-supervisor", "fake", request.Verb, "--instance-tag", request.InstanceTag)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := int64(command.Process.Pid)
	b.t.Cleanup(func() {
		_ = syscall.Kill(-int(pid), syscall.SIGKILL)
		_, _ = command.Process.Wait()
	})
	b.doubles.Process.Tags[pid] = request.InstanceTag
	b.launches = append(b.launches, request)
	return pid, nil
}
