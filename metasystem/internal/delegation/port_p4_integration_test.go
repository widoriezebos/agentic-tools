package delegation_test

// Ported integration scenarios of dispatch-fixtures.sh cluster b: the
// dispatches whose owners read Git inside internal/dispatch (brief
// authority, record build, rebase plan), so they run over the dispatch bed.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// setConf replaces (or appends) one key of the bed's configuration.
func (b *bed) p4SetConf(key, value string) {
	b.t.Helper()
	path := filepath.Join(b.root, "metasystem.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		b.t.Fatal(err)
	}
	var lines []string
	replaced := false
	for _, line := range strings.Split(strings.TrimRight(string(content), "\n"), "\n") {
		if strings.HasPrefix(line, key+"=") {
			line, replaced = key+"="+value, true
		}
		lines = append(lines, line)
	}
	if !replaced {
		lines = append(lines, key+"="+value)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// compositionTemporaries lists the composition temporaries the dispatcher
// makes in record-locks (dispatch-fixtures.sh's composition_temporary_inventory).
func (b *bed) compositionTemporaries() []string {
	entries, _ := os.ReadDir(filepath.Join(b.root, "artifacts", "agents", "record-locks"))
	var found []string
	for _, entry := range entries {
		name := strings.TrimPrefix(entry.Name(), "follow-")
		for _, prefix := range []string{"composed-packet.", "composition.", "composed-staged."} {
			if strings.HasPrefix(name, prefix) {
				found = append(found, entry.Name())
			}
		}
	}
	sort.Strings(found)
	return found
}

func (b *bed) p4ReadJSON(relative string) map[string]any {
	b.t.Helper()
	record, err := dispatch.ReadRecordObject(filepath.Join(b.root, relative))
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}

// L2708-2760: a brief over the packet cap is staged beside the round and
// referenced, not inlined: the prompt fits, the one reference names the
// staged task direction byte-for-byte as the saved brief, the prompt digest
// binds composition and job input, and nothing reports a refusal.
func TestDispatchIntegrationReferencesAnOversizedBrief(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p4SetConf("dispatch.max-inline-input-kb", "64")
	brief := b.brief("oversized.md", "verify", "Verify the thing.", strings.Repeat("x", 70000))
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "verifier", "--brief", brief, "--permissions", "none", "--job-id", "oversized")
	requireExit(t, result, 0, b.stderr.String())
	for _, text := range []string{string(result.Stdout), b.stderr.String(), string(result.Outcome)} {
		if strings.Contains(text, "REFUSED-INLINE-INPUT-LIMIT") || strings.Contains(text, "pass a file reference") {
			t.Fatalf("a referenced success reported refusal text: %q", text)
		}
	}
	payload := filepath.Join(b.root, "artifacts", "agents", "oversized")
	prompt := filepath.Join(payload, "rounds", "1", "prompt.md")
	promptBytes, err := os.ReadFile(prompt)
	if err != nil || len(promptBytes) > 65536 {
		t.Fatalf("prompt %d bytes (%v), over 65536", len(promptBytes), err)
	}
	composition := b.p4ReadJSON("artifacts/agents/oversized/rounds/1/composition.json")
	references, _ := composition["references"].([]any)
	if len(references) != 1 {
		t.Fatalf("composition has %d references instead of one: %v", len(references), composition["references"])
	}
	reference, _ := references[0].(map[string]any)
	openPath := filepath.Join(payload, "rounds", "1", "staged", "task-direction.md")
	savedBrief := filepath.Join(payload, "brief.md")
	saved, _ := os.ReadFile(savedBrief)
	staged, _ := os.ReadFile(openPath)
	digest := sha256Of(t, savedBrief)
	if reference["slot"] != "task-direction" || reference["purpose"] != "brief" || reference["lifetime"] != "staged" ||
		reference["path"] != "artifacts/agents/oversized/rounds/1/staged/task-direction.md" || reference["openPath"] != openPath {
		t.Fatalf("the composition did not retain the exact task-direction reference: %v", reference)
	}
	if !bytes.Equal(saved, staged) || len(saved) == 0 {
		t.Fatal("the staged task direction differs from the augmented saved brief")
	}
	if reference["digest"] != digest || reference["bytes"] != json.Number(strconv.Itoa(len(saved))) && reference["bytes"] != float64(len(saved)) {
		t.Fatalf("the reference lost the saved brief's bytes or digest: %v (bytes %d digest %s)", reference, len(saved), digest)
	}
	promptDigest := sha256Of(t, prompt)
	if composition["packetDigest"] != promptDigest || b.record("oversized")["inputHash"] != promptDigest {
		t.Fatalf("the prompt digest does not bind composition and job input: %v / %v / %s",
			composition["packetDigest"], b.record("oversized")["inputHash"], promptDigest)
	}
	stanza := "Referenced body: open " + openPath + " (" + strconv.Itoa(len(saved)) + " bytes, sha256 " + digest + "). Read it in bounded views; it is not inlined."
	if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(stanza) + `$`).Match(promptBytes) {
		t.Fatalf("the prompt lacks the exact final-path reference stanza %q", stanza)
	}
	if bytes.Contains(promptBytes, bytes.Repeat([]byte("x"), 64)) {
		t.Fatal("the filler body was inlined")
	}
	if mismatches, err := dispatch.VerifyReferences(b.root, filepath.Join(payload, "rounds", "1", "composition.json")); err != nil || len(mismatches) != 0 {
		t.Fatalf("the final reference did not verify: %v %v", mismatches, err)
	}
}

// L2762-2811: fixed parts that alone exceed the packet cap refuse at
// composition with the typed inline-limit outcome (exit 9) while the task
// direction handed to composition is small, and the refusal publishes
// nothing: no record, payload, staged body, adapter log or composition
// temporary. The retired leg measured the task direction by intercepting
// the engine's compose-role-packet subprocess; composition is now an
// in-process call, so the property is read from the brief handed in and the
// refusal's own accounting (all overhead, nothing inline-eligible left).
func TestDispatchIntegrationFixedOverheadOverTheCapRefusesAndPublishesNothing(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p4SetConf("dispatch.max-inline-input-kb", "8")
	skill := filepath.Join(b.root, "skills", "verify", "SKILL.md")
	original, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, append(original, bytes.Repeat([]byte("f"), 8193)...), 0o644); err != nil {
		t.Fatal(err)
	}
	b.git("commit", "-qam", "inflate a fixed recipe source")
	if info, _ := os.Stat(skill); info.Size() <= 8192 {
		t.Fatal("the fixed recipe source does not exceed the 8192-byte cap")
	}
	brief := b.brief("packet-fixed-overhead.md", "verify", "Verify the thing.", strings.Repeat("Direction the reference can carry.\n", 64))
	if info, _ := os.Stat(brief); info.Size() >= 32768 {
		t.Fatalf("the task direction is %d bytes, not below 32 KiB", info.Size())
	}
	before := b.compositionTemporaries()
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "verifier", "--brief", brief, "--permissions", "none", "--job-id", "packet-fixed-overhead")
	requireExit(t, result, 9, b.stderr.String())
	var lines []string
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		if strings.Contains(line, `"outcome":"REFUSED-INLINE-INPUT-LIMIT"`) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("the refusal did not print exactly one typed outcome: %q", result.Stdout)
	}
	var refusal map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &refusal); err != nil {
		t.Fatal(err)
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INLINE-INPUT-LIMIT" {
		t.Fatalf("recorded outcome %v", outcome)
	}
	if refusal["headline"] != "refused" || refusal["source"] != "dispatch.max-inline-input-kb" {
		t.Fatalf("the refusal lost its headline or source: %v", refusal)
	}
	detail, _ := refusal["detail"].(string)
	match := regexp.MustCompile(`^role packet exceeds dispatch\.max-inline-input-kb: ([0-9]+) bytes > ([0-9]+) bytes; fixed/reference overhead ([0-9]+) bytes$`).FindStringSubmatch(detail)
	if match == nil {
		t.Fatalf("the refusal detail has the wrong template: %q", detail)
	}
	packetBytes, _ := strconv.Atoi(match[1])
	capBytes, _ := strconv.Atoi(match[2])
	overhead, _ := strconv.Atoi(match[3])
	if capBytes != 8192 || packetBytes <= capBytes || overhead != packetBytes {
		t.Fatalf("inconsistent refusal counts: packet=%d cap=%d overhead=%d", packetBytes, capBytes, overhead)
	}
	for _, relative := range []string{"artifacts/agents/jobs/packet-fixed-overhead.json", "artifacts/agents/jobs/packet-fixed-overhead.log", "artifacts/agents/packet-fixed-overhead"} {
		if b.exists(relative) {
			t.Fatalf("the refused packet published %s", relative)
		}
	}
	if after := b.compositionTemporaries(); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Fatalf("the refusal left composition temporaries: %v", after)
	}
	if len(b.launches) != 0 {
		t.Fatal("a refused packet launched an adapter")
	}
}

// conclude moves a launched round to a terminal status through the record
// owner, as its runtime would.
func (b *bed) conclude(job, from, to, patch string) {
	b.t.Helper()
	path := filepath.Join(b.t.TempDir(), job+"-"+to+".json")
	if err := os.WriteFile(path, []byte(patch), 0o600); err != nil {
		b.t.Fatal(err)
	}
	if _, err := dispatch.RecordCAS(b.root, job, from, to, path); err != nil {
		b.t.Fatalf("conclude %s %s->%s: %v", job, from, to, err)
	}
}

const p4UnitUsage = `{"phase":"validation","error":null,"usage":{"providerUnits":{"name":"fake-unit","value":1}}}`

// L3184-3240 (the chain lifecycle, on an implementer worktree chain where
// the fixture ran a shared-checkout design critic): an ordinary follow-up of a completed
// round creates round 2 of the same chain through a fingerprinted
// follow-up claim of the parent's session, keeps round 1, inherits the
// chain's cap and its snapshot's handshake budget, releases the session
// occupancy on completion, aggregates the chain's usage onto the root, and
// mirrors both records under one chain manifest.
func TestFollowUpIntegrationCreatesTheChainsNextRound(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.p4SetConf("evidence.root", t.TempDir())
	brief := b.brief("brief.md", "implement", "Do the thing.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", "happy"), 0, b.stderr.String())
	b.conclude("happy", "running", "completed", p4UnitUsage)
	parent := b.record("happy")
	message := b.writeFile("follow.md", "Follow up on the thing.\n")
	result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "happy", "--message", message)
	requireExit(t, result, 0, b.stderr.String())
	if string(result.Stdout) != "happy-r2\n" {
		t.Fatalf("stdout %q", result.Stdout)
	}
	if !b.exists("artifacts/agents/happy/rounds/1") || !b.exists("artifacts/agents/happy/rounds/2/prompt.md") {
		t.Fatal("the follow-up did not keep round 1 and create round 2")
	}
	child := b.record("happy-r2")
	sessionKey := "fake:" + parent["sessionId"].(string)
	for field, want := range map[string]any{
		"parentJob": "happy", "dispatchMode": "follow-up", "resumedSessionId": parent["sessionId"],
		"sessionKey": sessionKey, "launchMode": parent["launchMode"], "capMin": parent["capMin"],
	} {
		if child[field] != want {
			t.Fatalf("child %s = %v, want %v (child %v)", field, child[field], want, child)
		}
	}
	if round, _ := strconv.Atoi(stringOf(child["round"])); round != 2 || stringOf(child["fingerprintVersion"]) != "2" || stringOf(child["fingerprint"]) == "" {
		t.Fatalf("child round/fingerprint %v %v %v", child["round"], child["fingerprintVersion"], child["fingerprint"])
	}
	// An implementer chain runs in its job worktree; the child claims the
	// same product roots and scopes its root launched with.
	if stringOf(child["productRoots"]) != stringOf(parent["productRoots"]) || stringOf(child["productRootScopes"]) != stringOf(parent["productRootScopes"]) ||
		stringOf(child["productRoots"]) != `["`+stringOf(parent["workspaceRoot"])+`"]` {
		t.Fatalf("product roots %v scopes %v, parent %v %v", child["productRoots"], child["productRootScopes"], parent["productRoots"], parent["productRootScopes"])
	}
	if stringOf(child["startedAt"]) < stringOf(parent["startedAt"]) {
		t.Fatal("the follow-up child started before its parent")
	}
	snapshot := b.p4ReadJSON(stringOf(child["capabilitySnapshot"]))
	capabilities, _ := snapshot["capabilities"].(map[string]any)
	if stringOf(child["sessionEstablishedTimeoutSec"]) != stringOf(capabilities["sessionEstablishedTimeoutSec"]) {
		t.Fatalf("the child's handshake budget %v is not its snapshot's %v", child["sessionEstablishedTimeoutSec"], capabilities["sessionEstablishedTimeoutSec"])
	}
	b.conclude("happy-r2", "running", "completed", p4UnitUsage)
	index := b.p4ReadJSON("artifacts/agents/sessions/" + p4SHA256Hex(sessionKey) + ".json")
	if index["sessionKey"] != sessionKey || len(index["occupants"].([]any)) != 0 {
		t.Fatalf("the session occupancy index was not maintained and released: %v", index)
	}
	requireExit(t, b.run("reap", "--job", "happy"), 0, b.stderr.String())
	requireExit(t, b.run("reap", "--job", "happy-r2"), 0, b.stderr.String())
	root := b.record("happy")
	usage, _ := root["chainUsage"].(map[string]any)
	units, _ := usage["providerUnits"].(map[string]any)
	fakeUnits, _ := units["fake"].(map[string]any)
	if stringOf(fakeUnits["fake-unit"]) != "2" {
		t.Fatalf("chain usage did not aggregate two fake units: %v", root["chainUsage"])
	}
	parentMirror, _ := root["mirror"].(map[string]any)
	childMirror, _ := b.record("happy-r2")["mirror"].(map[string]any)
	if parentMirror == nil || childMirror == nil || parentMirror["path"] != childMirror["path"] {
		t.Fatalf("parent and child mirror to different homes: %v / %v", parentMirror, childMirror)
	}
	home := childMirror["path"].(string)
	if sha256Of(t, filepath.Join(home, "manifest.json")) != childMirror["manifest"] {
		t.Fatal("the chain manifest digest does not match the child's stamp")
	}
	manifest, _ := dispatch.ReadRecordObject(filepath.Join(home, "manifest.json"))
	files, _ := manifest["files"].(map[string]any)
	if files["jobs/happy.json"] == nil || files["jobs/happy-r2.json"] == nil {
		t.Fatalf("the shared manifest does not cover both chain records: %v", files)
	}
}

func stringOf(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func p4SHA256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// launchTaggedSupervisor is launchInertSupervisor with the fake adapter's
// argv shape (delegate-supervisor fake VERB ... --instance-tag TAG), so a claim can verify
// the live supervisor as the reservation's own.
func (b *bed) launchTaggedSupervisor(request delegation.AdapterLaunch) (int64, error) {
	command := exec.Command("sh", "-c", "sleep 120; :", "delegate-supervisor", "fake", request.Verb, "--job", request.Job, "--instance-tag", request.InstanceTag)
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

const p4CapPatch = `{"error":"budget-cap","phase":"supervision","groupDeathProvenAt":"2026-09-27T12:00:00Z"}`

// L2990-3071: a round cut off at its cap is continued, not restarted. The
// follow-up of a capped implementer worktree round composes a fresh-context
// continuation told what happened, with the work still in the worktree; a
// repeated wrapper of the standing continuation binds to its operation; and
// a continuation that is itself capped is continued again, naming its
// round.
func TestFollowUpIntegrationContinuesARoundCutOffAtItsCap(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	// The repeated wrapper proves the standing supervisor is the operation's
	// own by its positioned instance tag, as the fake adapter's argv carries
	// it; the bed's inert supervisor gets that argv.
	b.doubles.Adapter.LaunchFunc = b.launchTaggedSupervisor
	brief := b.brief("capped.md", "implement", "Write the marker and hold.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", "capped-wt"), 0, b.stderr.String())
	workspace := stringOf(b.record("capped-wt")["workspaceRoot"])
	marker := filepath.Join(workspace, "capped-marker.txt")
	if err := os.WriteFile(marker, []byte("round one's work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.conclude("capped-wt", "running", "timeout", p4CapPatch)
	message := b.writeFile("follow.md", "Carry on.\n")
	result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "capped-wt", "--message", message)
	requireExit(t, result, 0, b.stderr.String())
	child := b.record("capped-wt-r2")
	if child["continuation"] != "after-cap" || child["resumeMode"] != "fresh-context" || child["parentJob"] != "capped-wt" {
		t.Fatalf("the continuation did not record its shape: %v", child)
	}
	// resumeMode fresh-context tells the adapter to start a new session (the
	// claim still names the parent session as part of the operation's
	// identity); the bed's runtime publishes a session of its own.
	if child["sessionId"] == b.record("capped-wt")["sessionId"] {
		t.Fatalf("the continuation resumed the killed session %v", child["sessionId"])
	}
	promptBytes, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "capped-wt", "rounds", "2", "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(promptBytes)
	for _, want := range []string{"# Prior Worktree", "cut off at its", "capped-marker.txt", "# Prior Brief"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the continuation packet lacks %q", want)
		}
	}
	if strings.Contains(prompt, "# Prior Return") {
		t.Fatal("the continuation carried a prior return the capped round never wrote")
	}
	composition, _ := child["composition"].(map[string]any)
	sources := stringOf(composition["sources"])
	if !strings.Contains(sources, `"source":"engine:prior-worktree"`) || strings.Contains(sources, `"source":"engine:prior-return"`) {
		t.Fatalf("the continuation composition lost its prior-worktree provenance: %s", sources)
	}
	if !b.exists("artifacts/agents/capped-wt/rounds/2/prior-worktree.md") {
		t.Fatal("the continuation paragraph was not kept in the round directory")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the continuation lost the capped round's work: %v", err)
	}

	// The standing continuation, repeated: the second wrapper binds to the
	// first's reservation.
	result = b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "capped-wt", "--message", message)
	if (result.ExitCode != 0 && result.ExitCode != 3) || !regexp.MustCompile(`"outcome":"(BOUND|IN-PROGRESS)"`).Match(result.Stdout) ||
		bytes.Contains(result.Stdout, []byte("REFUSED-OPID-MISMATCH")) {
		t.Fatalf("the repeated continuation did not bind to its standing operation: exit %d stdout %q stderr %q", result.ExitCode, result.Stdout, b.stderr.String())
	}
	if b.record("capped-wt-r2")["continuation"] != "after-cap" || b.exists("artifacts/agents/jobs/capped-wt-r3.json") {
		t.Fatal("the repeated wrapper changed the standing continuation")
	}

	// A continuation cut off at its cap is continued again.
	b.conclude("capped-wt-r2", "running", "timeout", p4CapPatch)
	requireExit(t, b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "capped-wt", "--message", message), 0, b.stderr.String())
	third := b.record("capped-wt-r3")
	if third["continuation"] != "after-cap" || third["parentJob"] != "capped-wt-r2" {
		t.Fatalf("a capped continuation was not continued again: %v", third)
	}
	paragraph, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "capped-wt", "rounds", "3", "prior-worktree.md"))
	if !strings.Contains(string(paragraph), "Round 2 of this chain was cut off") {
		t.Fatalf("the second continuation's paragraph does not name round 2: %s", paragraph)
	}
	b.conclude("capped-wt-r3", "running", "completed", `{"phase":"validation","error":null}`)
	latest, err := dispatch.LatestChainRecord(filepath.Join(b.root, "artifacts", "agents", "jobs"), "capped-wt")
	if err != nil || filepath.Base(latest) != "capped-wt-r3.json" || b.record("capped-wt-r3")["status"] != "completed" {
		t.Fatalf("the chain's newest record is %s (%v), not the completed continuation", latest, err)
	}
}

// criticReturn is a schema-valid design-critic return for one round.
func criticReturn(job string, round int) string {
	return `{"schemaVersion":3,"jobId":"` + job + `","round":` + strconv.Itoa(round) + `,"runtime":"fake",
  "sessionId":"fake-session-` + job + `","model":{"requested":"fake-model","effective":"fake-model"},
  "claimed":{"model":null,"sessionId":null},"mode":"design","reviewedCommit":"abc1234",
  "evidence":[{"command":"fixture delivery","observed":"canonical return recorded","level":"ran"}],
  "gaps":[],"findings":[],"rigor":[],"verdictMaterialCount":0}`
}

// criticalDesignReturn permits a second examination of the recorded subject.
func (b *bed) criticalDesignReturn(job string) map[string]any {
	b.t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(criticReturn(job, 1)), &result); err != nil {
		b.t.Fatal(err)
	}
	result["reviewedCommit"] = b.p5ReadSubjectFile(job, 1).ReviewedCommit
	result["findings"] = []any{map[string]any{
		"id": "DESIGN-1", "severity": "critical", "material": true,
		"claim": "the design cannot be built without the missing admission rule", "evidence": "the fixture design omits the rule",
	}}
	result["verdictMaterialCount"] = 1
	return result
}

// dispatchCritic starts one design-critic round of the shipped role page.
func (b *bed) dispatchCritic(job string) {
	b.t.Helper()
	outputs := b.writeFile("declared-outputs.txt", "plans/designs/p4-design.md\n")
	if !b.exists("plans/designs/p4-design.md") {
		b.writeFile("plans/designs/p4-design.md", designFixturePage("# A design under review\n\nThe first revision.\n"))
		b.git("add", "plans/designs/p4-design.md")
		b.git("commit", "-qm", "a design under review")
	}
	brief := b.brief(job+"-brief.md", "design", "Review the design.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", "plans/designs/p4-design.md", "--brief", brief, "--job-id", job)
	requireExit(b.t, result, 0, b.stderr.String())
	if string(result.Stdout) != job+"\n" {
		b.t.Fatalf("a critic dispatch printed %q, want only its job", result.Stdout)
	}
}

// L2443 (leg_happy), L2674-2678, L3184-3240, L3277-3294: design-critic
// chains. A critic dispatch records its reviewed design and the critic
// preset's network denial. A follow-up after the design page changed
// publishes the changed subject as round 2 of the same chain, resuming the
// parent's session. A round that ended in a protocol error is followed up
// with its synthetic finding carried; a deadline stops the chain, and a lost
// critic round whose group death is proven is examined once more.
func TestFollowUpIntegrationCriticChains(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	message := b.writeFile("follow.md", "Follow up on the review.\n")

	b.dispatchCritic("happy")
	parent := b.record("happy")
	requested, _ := parent["permissions"].(map[string]any)["requested"].(map[string]any)
	if parent["design"] != "plans/designs/p4-design.md" || parent["launchMode"] != "shared-checkout" || requested["network"] != "deny" {
		t.Fatalf("the critic dispatch lost its design, launch mode or network denial: design %v launch %v permissions %v", parent["design"], parent["launchMode"], parent["permissions"])
	}
	b.p5WriteJSON("artifacts/agents/happy/rounds/1/return.json", b.criticalDesignReturn("happy"))
	b.conclude("happy", "running", "completed", `{"phase":"validation","error":null}`)
	before := b.p4ReadJSON("artifacts/agents/happy/rounds/1/subject.json")["contentDigest"]
	revised := b.writeFile("plans/designs/p4-design.md", designFixturePage("# A design under review\n\nThe second revision.\n"))
	b.git("commit", "-qam", "revise the design")
	requireExit(t, b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "happy", "--message", message), 0, b.stderr.String())
	if after := b.p4ReadJSON("artifacts/agents/happy/rounds/2/subject.json")["contentDigest"]; after != sha256Of(t, revised) || after == before {
		t.Fatalf("the follow-up did not publish the changed design subject: %v (before %v)", after, before)
	}
	child := b.record("happy-r2")
	roots, _ := json.Marshal(child["productRoots"])
	scopes, _ := json.Marshal(child["productRootScopes"])
	if child["parentJob"] != "happy" || stringOf(child["round"]) != "2" || child["dispatchMode"] != "follow-up" ||
		child["resumedSessionId"] != parent["sessionId"] || child["sessionKey"] != "fake:"+stringOf(parent["sessionId"]) ||
		child["launchMode"] != "shared-checkout" || string(roots) != `["`+b.root+`"]` ||
		string(scopes) != `[{"path":"`+b.root+`","reason":"shared-checkout","standing":"attribution-only"}]` {
		t.Fatalf("the critic follow-up did not chain to round 1: %v", child)
	}

	b.dispatchCritic("malformed-return")
	requireExit(t, b.run("__protocol-error", "--job", "malformed-return", "--expect", "running", "--violation", "return.json is not JSON"), 0, b.stderr.String())
	if record := b.record("malformed-return"); record["status"] != "failed" || record["error"] != "protocol_error" {
		t.Fatalf("the protocol error was not recorded: %v", record)
	}
	requireExit(t, b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "malformed-return", "--message", message), 0, b.stderr.String())
	prompt, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "malformed-return", "rounds", "2", "prompt.md"))
	if !bytes.Contains(prompt, []byte("# Canonical critique register carry")) || !bytes.Contains(prompt, []byte("- synthetic-")) {
		t.Fatalf("the protocol-error follow-up did not carry its synthetic finding identifier:\n%s", prompt)
	}

	for job, verdict := range map[string][2]string{"timed": {"timeout", "budget-cap"}, "process-loss": {"failed", "process-lost"}} {
		b.dispatchCritic(job)
		b.conclude(job, "running", verdict[0], `{"error":"`+verdict[1]+`","phase":"supervision","groupDeathProvenAt":"2026-09-27T12:00:00Z"}`)
		result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", job, "--message", message)
		if verdict[0] == "timeout" {
			requireExit(t, result, 1, b.stderr.String())
			if !strings.Contains(b.stderr.String(), "a deadline is not retried") || exists(b.recordPath(job+"-r2")) {
				t.Fatalf("the deadline granted a fresh design examination: %q", b.stderr.String())
			}
			continue
		}
		requireExit(t, result, 0, b.stderr.String())
		if retry := b.record(job + "-r2"); stringOf(retry["round"]) != "2" || retry["parentJob"] != job {
			t.Fatalf("the examination retry of %s did not create round 2 of the same chain: %v", job, retry)
		}
	}
}
