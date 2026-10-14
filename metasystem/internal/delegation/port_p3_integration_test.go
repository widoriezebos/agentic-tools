package delegation_test

// Ported scenarios of dispatch-fixtures.sh lines 1863-2441 (dispatch
// cluster a, second part) whose subject reads Git inside internal/dispatch
// (brief authority, the critic read subject, the record build over the
// workspace head, worktree custody): they run over the integration bed and
// carry Integration in their names.

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

func delegationStartReader(root string) (identity.StartReader, error) {
	return dispatchproc.StartReader(root)
}

func claimVerifier() dispatch.ClaimProcessVerifier { return dispatchproc.ClaimProcessVerifier{} }

// openServingGoal opens a usable Current goal in root's legacy ledger.
func openServingGoal(t *testing.T, root, id, intent string) {
	t.Helper()
	if _, err := (&goal.Store{Root: root}).Open(goal.Caller{Class: "MAIN", Holder: true}, id, intent, "Dispatch with the projection."); err != nil {
		t.Fatal(err)
	}
}

// criticSubject commits the named design pages under plans/designs and a
// declared-outputs manifest naming the first; it returns the manifest.
func (b *bed) criticSubject(designs ...string) string {
	b.t.Helper()
	for _, design := range designs {
		b.writeFile(filepath.Join("plans", "designs", design), designFixturePage("# "+design+"\n"))
	}
	b.git("add", "-A")
	b.git("commit", "-qm", "critic subjects")
	return b.writeFile("outputs-"+designs[0]+".txt", "plans/designs/"+designs[0]+"\n")
}

// criticArgv is a design-critic dispatch of design with outputs and brief.
func criticArgv(outputs, design, brief, job string, extra ...string) []string {
	argv := []string{"dispatch", "--role", "design-critic", "--outputs", outputs, "--design", "plans/designs/" + design, "--brief", brief}
	if job != "" {
		argv = append(argv, "--job-id", job)
	}
	return append(argv, extra...)
}

// dispatchAs runs one fresh dispatch with a freshly minted claim capability.
func (b *bed) dispatchAs(argv ...string) delegation.Result {
	b.t.Helper()
	return b.runEnv(b.dispatchEnv("fresh"), argv...)
}

// setConf rewrites the bed's configuration as the dispatch bed's base with
// extra lines added or replacing the base line of the same key.
func (b *bed) setConf(extra string) {
	b.t.Helper()
	overridden := map[string]bool{}
	for _, line := range strings.Split(extra, "\n") {
		if key, _, found := strings.Cut(line, "="); found {
			overridden[key] = true
		}
	}
	var conf strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(dispatchBedConfig, "\n"), "\n") {
		if key, _, _ := strings.Cut(line, "="); !overridden[key] {
			conf.WriteString(line + "\n")
		}
	}
	b.writeFile("metasystem.conf", conf.String()+extra+"evidence.root="+filepath.Join(b.root, "evidence")+"\n")
}

// tagSupervisors makes the bed's adapter start a supervisor whose argv is
// the fake adapter's positioned shape, so the claim's process verifier
// proves the launched job as its own (a bare sleep carries no tag).
func (b *bed) tagSupervisors() {
	b.doubles.Adapter.LaunchFunc = func(request delegation.AdapterLaunch) (int64, error) {
		command := exec.Command("/bin/sh", "-c", "sleep 120; :", "delegate-supervisor", "fake", "dispatch", "--instance-tag", request.InstanceTag)
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
}

func (b *bed) readJSON(relative string) map[string]any {
	b.t.Helper()
	content, err := os.ReadFile(filepath.Join(b.root, relative))
	if err != nil {
		b.t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(content, &object); err != nil {
		b.t.Fatalf("%s: %v", relative, err)
	}
	return object
}

// at walks a dotted path through decoded JSON.
func at(object map[string]any, path string) any {
	var current any = object
	for _, key := range strings.Split(path, ".") {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// complete moves a launched job to completed through the record owner.
func (b *bed) complete(job string) {
	b.t.Helper()
	patch := b.writeFile("complete-"+job+".json", `{"error":null}`)
	if _, err := dispatch.RecordCAS(b.root, job, "running", "completed", patch); err != nil {
		b.t.Fatalf("complete %s: %v", job, err)
	}
}

// Lines 1864 (leg_happy's launch), 1961-2020 and 2227-2244: a fresh
// design-critic round records every required field, its bootstrap-honest
// composition, its fingerprinted claim reservation, its shared-checkout
// attribution, the capability snapshot it was judged against, the critic
// preset and stdin delivery; its prompt opens with the task direction, names
// the job, and carries the brief before the role preamble; the input hash is
// the published prompt's; the session occupancy index is released when the
// job concludes.
func TestDispatchIntegrationRecordsAFreshCriticRound(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("happy.md")
	brief := b.brief("brief.md", "design", "Review the happy design.")
	result := b.dispatchAs(criticArgv(outputs, "happy.md", brief, "happy")...)
	requireExit(t, result, 0, b.stderr.String())
	record := b.record("happy")
	for _, key := range []string{"jobId", "role", "mission", "runtime", "round", "parentJob", "status", "phase", "error",
		"workspaceRoot", "baseSha", "branch", "permissions", "capMin", "pid", "pidStartedAt", "pgid", "instanceTag", "custodyProcesses",
		"sessionId", "turnId", "requestedModel", "aliasedFrom", "rosterAliasedFrom", "effectiveModel", "overridden", "capabilitySnapshot",
		"sessionEstablishedTimeoutSec", "input", "composition", "startedAt", "endedAt", "usage", "mirror"} {
		if _, has := record[key]; !has {
			t.Fatalf("the happy record lacks required field %s: %v", key, record)
		}
	}
	if record["status"] != "running" || record["design"] != "plans/designs/happy.md" {
		t.Fatalf("status %v design %v", record["status"], record["design"])
	}
	if at(record, "composition.contextProof.proofState") != "no-leak-not-proven" ||
		at(record, "composition.machineSlotAdmission.ownerGoal") != "machine-concurrency-governor" ||
		at(record, "composition.toolSurface.nameState") != "exact" ||
		fmt.Sprint(at(record, "composition.toolSurface.names")) != "[]" ||
		at(record, "composition.packetDigest") == "" || at(record, "composition.packetDigest") == nil {
		t.Fatalf("the record lost its bootstrap-honest composition: %v", record["composition"])
	}
	if record["sessionKey"] != "fake:happy" || fmt.Sprint(record["fingerprintVersion"]) != "2" || record["fingerprint"] == nil ||
		record["dispatchMode"] != "fresh" || at(record, "creatorLiveness.pid") == nil || record["reservationDeadline"] == nil {
		t.Fatalf("the fresh dispatch did not complete a fingerprinted claim-launch reservation: %v", record)
	}
	roots, _ := json.Marshal(record["productRoots"])
	scopes, _ := json.Marshal(record["productRootScopes"])
	if record["launchMode"] != "shared-checkout" || string(roots) != fmt.Sprintf("[%q]", b.root) ||
		string(scopes) != fmt.Sprintf(`[{"path":%q,"reason":"shared-checkout","standing":"attribution-only"}]`, b.root) {
		t.Fatalf("launch mode %v roots %s scopes %s", record["launchMode"], roots, scopes)
	}
	snapshot, _ := record["capabilitySnapshot"].(string)
	if !strings.HasPrefix(snapshot, "artifacts/agents/capabilities/") || at(record, "permissions.enforcementSnapshot") != snapshot {
		t.Fatalf("snapshot %q enforcement %v", snapshot, at(record, "permissions.enforcementSnapshot"))
	}
	selected := b.readJSON(snapshot)
	if fmt.Sprint(record["sessionEstablishedTimeoutSec"]) != fmt.Sprint(at(selected, "capabilities.sessionEstablishedTimeoutSec")) {
		t.Fatalf("record timeout %v, snapshot %v", record["sessionEstablishedTimeoutSec"], at(selected, "capabilities.sessionEstablishedTimeoutSec"))
	}
	if enforcement, _ := json.Marshal(selected["envelopeEnforcement"]); string(enforcement) != `{"network":"mapped","readRoots":"notEnforced","writeRoots":"mapped"}` {
		t.Fatalf("envelope enforcement %s", enforcement)
	}
	if at(record, "permissions.requested.preset") != "critic" || at(record, "input.delivery") != "stdin" {
		t.Fatalf("preset %v delivery %v", at(record, "permissions.requested.preset"), at(record, "input.delivery"))
	}
	if bytes, err := strconv.ParseInt(fmt.Sprint(at(record, "input.bytes")), 10, 64); err != nil || bytes <= 0 {
		t.Fatalf("input bytes %v", at(record, "input.bytes"))
	}
	promptPath := filepath.Join(b.root, "artifacts", "agents", "happy", "rounds", "1", "prompt.md")
	if at(record, "input.hash") != sha256Hex(t, promptPath) {
		t.Fatal("the recorded input hash is not the published prompt's")
	}
	prompt, _ := os.ReadFile(promptPath)
	preamble, _ := protocol.RoleInstructions("design-critic")
	payloadBrief, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "happy", "brief.md"))
	text := string(prompt)
	if !strings.HasPrefix(text, "# Task Direction\n\n") || !strings.Contains(text, "Job-Id: happy") {
		t.Fatalf("prompt head %q", text[:min(len(text), 200)])
	}
	briefAt, preambleAt := strings.Index(text, string(payloadBrief)), strings.Index(text, string(preamble))
	if briefAt < 0 || preambleAt < 0 || briefAt >= preambleAt {
		t.Fatalf("brief at %d, preamble at %d: the task direction must precede the role instructions", briefAt, preambleAt)
	}
	sum := sha256.Sum256([]byte("fake:happy"))
	index := filepath.Join("artifacts", "agents", "sessions", hex.EncodeToString(sum[:])+".json")
	if occupancy := b.readJSON(index); occupancy["sessionKey"] != "fake:happy" || fmt.Sprint(occupancy["occupants"]) == "[]" {
		t.Fatalf("the running job is not an occupant of its session index: %v", occupancy)
	}
	b.complete("happy")
	if occupancy := b.readJSON(index); fmt.Sprint(occupancy["occupants"]) != "[]" {
		t.Fatalf("the concluded job still occupies its session: %v", occupancy)
	}
}

// Lines 1866-1898 and 2321-2325: a second wrapper for the same fresh
// operation reaches claim-launch and answers with the standing claim rather
// than failing at the chain lock or the standing record, and launches
// nothing; the same job id with different brief bytes is a different
// request and refuses as an operation-id mismatch.
func TestDispatchIntegrationAStandingJobIDAnswersOnlyItsOwnRequest(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.tagSupervisors()
	outputs := b.criticSubject("repeat.md")
	brief := b.brief("brief.md", "design", "Review the repeated design.")
	requireExit(t, b.dispatchAs(criticArgv(outputs, "repeat.md", brief, "repeat-fresh")...), 0, b.stderr.String())
	second := b.dispatchAs(criticArgv(outputs, "repeat.md", brief, "repeat-fresh")...)
	if second.ExitCode != 0 && second.ExitCode != 3 {
		t.Fatalf("the repeated wrapper failed before claim-launch: exit %d stdout %q stderr %q", second.ExitCode, second.Stdout, b.stderr.String())
	}
	if !regexp.MustCompile(`"outcome":"(BOUND|IN-PROGRESS)"`).Match(second.Stdout) {
		t.Fatalf("the repeated wrapper did not answer with the claim state: %q", second.Stdout)
	}
	if len(b.launches) != 1 {
		t.Fatalf("the repeated operation launched again: %d launches", len(b.launches))
	}
	other := b.brief("other.md", "design", "This launch request is distinct from the first.")
	mismatch := b.dispatchAs(criticArgv(outputs, "repeat.md", other, "repeat-fresh")...)
	if mismatch.ExitCode == 0 || !strings.Contains(string(mismatch.Stdout), "REFUSED-OPID-MISMATCH") {
		t.Fatalf("a different request reused the job id: exit %d stdout %q", mismatch.ExitCode, mismatch.Stdout)
	}
	if len(b.launches) != 1 || b.record("repeat-fresh")["status"] != "running" {
		t.Fatal("the refused reuse touched the standing job")
	}
}

// Lines 1900-1942: a busy session refuses at claim-launch before the
// operation's payload exists, so the same identifier launches as soon as
// the temporary occupant is terminal.
func TestDispatchIntegrationABusySessionRefusesWithoutLeavingThePayload(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("payload.md")
	brief := b.brief("brief.md", "design", "Review the payload design.")
	digest := sha256Hex(t, brief)
	preparation := filepath.Join(t.TempDir(), "occupancy.json")
	if err := dispatch.WriteClaimOccupancyPreparation(b.root, "fake:payload-retry", preparation); err != nil {
		t.Fatal(err)
	}
	prepared, err := dispatch.ReadClaimOccupancyPreparation(preparation, "fake:payload-retry")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := delegationStartReader(b.root)
	if err != nil {
		t.Fatal(err)
	}
	resumed := ""
	blocker, err := dispatch.ClaimLaunch(dispatch.ClaimLaunchParams{
		Root: b.root, OpID: "payload-blocker", AdapterVerb: "dispatch",
		Request: dispatch.LaunchFingerprintRequest{
			SessionKey: "fake:payload-retry", DispatchMode: dispatch.DispatchModeFresh, ResumedSessionID: &resumed,
			Runtime: "fake", Model: "fake-model", Role: "design-critic", LaunchMode: dispatch.LaunchModeSharedCheckout,
			PermissionEnvelopeDigest: digest, ProductRoots: []string{b.root}, CapMinutes: 30, InputHash: digest,
			DestructiveReach: dispatch.HazardMechanical,
		},
		DefaultCapMinutes: 30, OccupancyPreparation: &prepared,
	}, dispatch.ClaimLaunchDependencies{CreatorPID: int64(os.Getpid()), IdentityReader: reader, ProcessVerifier: claimVerifier()})
	if err != nil || blocker.Outcome != dispatch.ClaimWON {
		t.Fatalf("blocker claim %+v: %v", blocker, err)
	}
	refused := b.dispatchAs(criticArgv(outputs, "payload.md", brief, "payload-retry")...)
	requireExit(t, refused, 1, b.stderr.String())
	if !strings.Contains(string(refused.Stdout), "REFUSED-SESSION-BUSY") {
		t.Fatalf("payload-retry was not refused by claim-launch: %q", refused.Stdout)
	}
	if exists(filepath.Join(b.root, "artifacts", "agents", "payload-retry")) {
		t.Fatal("a refused claim left payload-retry launch files behind")
	}
	patch := b.writeFile("blocker-failed.json", `{"error":"fixture-release"}`)
	if _, err := dispatch.RecordCAS(b.root, "payload-blocker", "pending-setup", "failed", patch); err != nil {
		t.Fatal(err)
	}
	requireExit(t, b.dispatchAs(criticArgv(outputs, "payload.md", brief, "payload-retry")...), 0, b.stderr.String())
	if b.record("payload-retry")["status"] != "running" {
		t.Fatal("payload-retry did not launch after its temporary refusal cleared")
	}
}

// Lines 2022-2057: the model-family pointer through the real dispatcher,
// from the roster and from an explicit override: every downstream identity
// is the canonical target, the target's cap row outranks the source's, and
// the source survives only in the provenance fields.
func TestDispatchIntegrationRelaysOnlyTheCanonicalModelDownstream(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("alias-roster.md", "alias-override.md")
	alias := "runtime.fake.model-alias.fake-source=fake-model\ncap.min.fake.fake-model=31\ncap.min.fake.fake-source=30\n"
	brief := b.brief("brief.md", "design", "Review the alias design.")
	b.setConf(alias + "role.design-critic.model.fake=fake-source\n")
	requireExit(t, b.dispatchAs(criticArgv(outputs, "alias-roster.md", brief, "model-alias-roster")...), 0, b.stderr.String())
	roster := b.record("model-alias-roster")
	for key, want := range map[string]string{"composition.model": "fake-model", "canonicalModelKey": "fake-model", "requestedModel": "fake-model",
		"aliasedFrom": "fake-source", "rosterAliasedFrom": "fake-source", "capMin": "31",
		"capResolution.rule": "config-pair"} {
		if got := fmt.Sprint(at(roster, key)); got != want {
			t.Fatalf("roster alias: %s = %v, want %v", key, got, want)
		}
	}
	b.setConf(alias + "role.design-critic.model.fake=fake-model\n")
	requireExit(t, b.dispatchAs(criticArgv(outputs, "alias-override.md", brief, "model-alias-override", "--model", "fake-source")...), 0, b.stderr.String())
	override := b.record("model-alias-override")
	for key, want := range map[string]any{"composition.model": "fake-model", "canonicalModelKey": "fake-model", "requestedModel": "fake-model",
		"aliasedFrom": "fake-source", "rosterAliasedFrom": nil} {
		if got := at(override, key); got != want {
			t.Fatalf("override alias: %s = %v, want %v", key, got, want)
		}
	}
}

// Lines 2161-2176 and 2191-2223: a writable preset derives quarantined
// worktree custody even for a review role; an explicit live-checkout
// workspace refuses by incident class, naming the override, before any
// reservation, payload, log or heartbeat exists; an explicit --worktree
// keeps its configured write grant.
func TestDispatchIntegrationAWritableReviewRoleIsQuarantined(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("review-default.md", "review-live.md", "review-worktree.md")
	b.setConf("dispatch.permissions.design-critic=workspace\n")
	brief := b.brief("brief.md", "design", "Review with a writable preset.")
	requireWritableWorktree := func(job string) {
		t.Helper()
		record := b.record(job)
		root, _ := record["workspaceRoot"].(string)
		// The adapter's effective-permissions.json is effective-init of this
		// requested envelope (adapter-owned, U6a); the lifecycle's part is
		// the envelope it expanded into the record.
		writes, _ := json.Marshal(at(record, "permissions.requested.writeRoots"))
		if record["launchMode"] != "worktree" || root == "" || root == b.root || !strings.Contains(string(writes), fmt.Sprintf("%q", root)) ||
			at(record, "permissions.requested.tools") != "runtime-default" {
			t.Fatalf("%s: launch mode %v root %q writes %s tools %v", job, record["launchMode"], root, writes, at(record, "permissions.requested.tools"))
		}
	}
	requireExit(t, b.dispatchAs(criticArgv(outputs, "review-default.md", brief, "review-default")...), 0, b.stderr.String())
	requireWritableWorktree("review-default")

	refused := b.dispatchAs(criticArgv(outputs, "review-live.md", brief, "review-live-write", "--workspace", b.root)...)
	requireExit(t, refused, 2, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "design-critic refused: a review role could write in the coordinator's checkout") || !strings.Contains(b.stderr.String(), "pass --worktree") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	requireNothingPublished(t, b, "review-live-write")

	requireExit(t, b.dispatchAs(criticArgv(outputs, "review-worktree.md", brief, "review-worktree", "--worktree")...), 0, b.stderr.String())
	requireWritableWorktree("review-worktree")
}

// followCritic completes a critic round with a critical finding, commits a
// revision of its design page, and follows it up with the shipped message
// template.
func (b *bed) followCritic(job, design string) delegation.Result {
	b.t.Helper()
	b.complete(job)
	b.p5WriteJSON("artifacts/agents/"+job+"/rounds/1/return.json", b.criticalDesignReturn(job))
	b.writeFile("plans/designs/"+design, designFixturePage("# "+design+"\n\nFollow-up revision.\n"))
	b.git("add", "-A")
	b.git("commit", "-qm", "revise "+design)
	template, err := protocol.Template("follow-up.md")
	if err != nil {
		b.t.Fatal(err)
	}
	message := b.writeFile("follow-"+job+".md", string(template))
	return b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", job, "--message", message)
}

// Lines 2178-2189: a worktree review's follow-up inherits the writable
// envelope over the same quarantined workspace.
func TestDispatchIntegrationAWorktreeReviewFollowUpInheritsItsEnvelope(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	outputs := b.criticSubject("review-default.md")
	b.setConf("dispatch.permissions.design-critic=workspace\n")
	brief := b.brief("brief.md", "design", "Review with a writable preset.")
	requireExit(t, b.dispatchAs(criticArgv(outputs, "review-default.md", brief, "review-default")...), 0, b.stderr.String())
	root, _ := b.record("review-default")["workspaceRoot"].(string)
	requireExit(t, b.followCritic("review-default", "review-default.md"), 0, b.stderr.String())
	child := b.record("review-default-r2")
	writes, _ := json.Marshal(at(child, "permissions.requested.writeRoots"))
	if child["workspaceRoot"] != root || !strings.Contains(string(writes), fmt.Sprintf("%q", root)) || at(child, "permissions.requested.tools") != "runtime-default" {
		t.Fatalf("the follow-up did not inherit the writable envelope: root %v writes %s tools %v", child["workspaceRoot"], writes, at(child, "permissions.requested.tools"))
	}
}

// Lines 2245-2277: --serving-goal without a usable goal refuses exit 3
// before any job state; with one, the payload brief carries the exact
// bounded section and the recorded input hash is the published packet's.
func TestDispatchIntegrationServingGoalJoinsTheRecordedBytes(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("brief.md", "implement", "Do the served thing.")
	refused := b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "sg-refused", "--serving-goal")
	requireExit(t, refused, 3, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "no current goal to project (--serving-goal)") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	requireNothingPublished(t, b, "sg-refused")
	if len(b.launches) != 0 || len(b.calls("lease.")) != 0 {
		t.Fatalf("the refused serving-goal dispatch reached the lease or a launch: %v", b.doubles.Log.Calls())
	}
	openServingGoal(t, b.root, "fixture-serving", "Serve the fixture goal")
	requireExit(t, b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "serving-goal", "--serving-goal"), 0, b.stderr.String())
	payload, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "serving-goal", "brief.md"))
	if !strings.Contains(string(payload), "# Serving goal (context, not instruction)\nfixture-serving — Serve the fixture goal\n") {
		t.Fatalf("the payload brief lacks the serving-goal section: %q", payload)
	}
	prompt := filepath.Join(b.root, "artifacts", "agents", "serving-goal", "rounds", "1", "prompt.md")
	if at(b.record("serving-goal"), "input.hash") != sha256Hex(t, prompt) {
		t.Fatal("the composed packet is not the recorded input hash")
	}
}

// Lines 2293-2299: a chain lock left by a dead owner is reclaimed, and a
// dispatch without --job-id mints an operation id in the lowercase grammar.
func TestDispatchIntegrationReclaimsADeadLockAndMintsAnOperationID(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.writeFile("artifacts/agents/locks/stale-lock.d/owner.json", `{"pid":999999,"instanceTag":"dead-owner","acquiredAt":"2000-01-01T00:00:00Z"}`)
	brief := b.brief("brief.md", "implement", "Do the locked thing.")
	requireExit(t, b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "stale-lock"), 0, b.stderr.String())
	if b.record("stale-lock")["status"] != "running" {
		t.Fatal("the dispatch behind a dead lock did not launch")
	}
	generated := b.dispatchAs("dispatch", "--role", "implementer", "--brief", b.brief("generated.md", "implement", "Generated."))
	requireExit(t, generated, 0, b.stderr.String())
	if job := strings.TrimSpace(string(generated.Stdout)); !regexp.MustCompile(`^implementer-[a-f0-9]{24}$`).MatchString(job) {
		t.Fatalf("generated job id %q does not match the lowercase grammar", job)
	}
}

// Lines 2334-2347: a writable delegate with no workspace choice derives
// worktree custody, and a design-bearing implementer carries the xhigh
// builder effort into its record and its configuration obligations.
func TestDispatchIntegrationADesignBearingBuilderDerivesItsWorktree(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("code.md", "implement", "Build it.")
	result := b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "delegate-derived-worktree", "--destructive-reach", "DESIGN-BEARING")
	requireExit(t, result, 0, b.stderr.String())
	record := b.record("delegate-derived-worktree")
	if record["launchMode"] != "worktree" || record["reasoningEffort"] != "xhigh" || at(record, "configurationObligations.builderReasoningEffort") != "xhigh" {
		t.Fatalf("launch mode %v effort %v obligation %v", record["launchMode"], record["reasoningEffort"], at(record, "configurationObligations.builderReasoningEffort"))
	}
}

// Lines 2350-2361: a verifier claim derives its live-proof reference onto
// the reviewed chain root, and the investigator role launches on an
// explicit runtime with no write grant.
func TestDispatchIntegrationVerifierAndInvestigatorRounds(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	target := b.brief("target.md", "implement", "Build the target.")
	requireExit(t, b.dispatchAs("dispatch", "--role", "implementer", "--brief", target, "--job-id", "review-target", "--worktree"), 0, b.stderr.String())
	b.complete("review-target")
	verify := b.brief("verify.md", "verify", "Prove it.")
	requireExit(t, b.dispatchAs("dispatch", "--role", "verifier", "--brief", verify, "--reviews", "review-target", "--permissions", "none", "--job-id", "review-proof"), 0, b.stderr.String())
	if got := b.record("review-target")["liveProofEvidenceRef"]; got != "review-proof" {
		t.Fatalf("the verifier claim did not derive its reference onto the reviewed chain root: %v", got)
	}
	investigate := b.brief("investigate.md", "take-a-step-back", "Step back.")
	requireExit(t, b.dispatchAs("dispatch", "--role", "investigator", "--brief", investigate, "--runtime", "fake", "--permissions", "none", "--job-id", "investigator-role"), 0, b.stderr.String())
	if record := b.record("investigator-role"); record["status"] != "running" || record["role"] != "investigator" {
		t.Fatalf("investigator record %v", record)
	}
}

// Lines 2363-2376: a runtime that never signals its session fails the
// dispatch when the stamped deadline passes, and the record keeps the
// handshake_timeout verdict.
func TestDispatchIntegrationASilentRuntimeFailsItsHandshake(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.noHandshake["no-signal"] = true
	brief := b.brief("brief.md", "implement", "Never signal.")
	result := b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "no-signal", "--wait")
	requireExit(t, result, 3, b.stderr.String())
	record := b.record("no-signal")
	if record["status"] != "failed" || record["error"] != "handshake_timeout" {
		t.Fatalf("record %v", record)
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "HANDSHAKE-FAILED" {
		t.Fatalf("outcome %v", outcome)
	}
	if len(b.calls("host.WaitJob")) != 0 {
		t.Fatal("a failed handshake still waited for the job")
	}
}

// Lines 2388-2436: a large brief rides the packet as a staged
// task-direction reference (never inlined, the staged copy byte-equal to the
// composed job brief, the prompt within 64 KiB); under an 8 KiB packet cap
// the same brief refuses with the typed composition refusal, publishes
// nothing, leaves no composition temporaries, and refuses the same way on a
// retry.
func TestDispatchIntegrationALargeBriefIsReferencedOrRefusedWhole(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.setConf("dispatch.max-inline-input-kb=64\n")
	outputs := b.criticSubject("large.md")
	large := b.brief("large.md", "design", strings.Repeat("filler line for the forty kibibyte referenced brief\n", 40960/52))
	requireExit(t, b.dispatchAs(criticArgv(outputs, "large.md", large, "large-brief")...), 0, b.stderr.String())
	round := filepath.Join("artifacts", "agents", "large-brief", "rounds", "1")
	composition := b.readJSON(filepath.Join(round, "composition.json"))
	if references, _ := json.Marshal(composition["references"]); !strings.Contains(string(references), "task-direction") {
		t.Fatalf("composition references %s", references)
	}
	prompt, _ := os.ReadFile(filepath.Join(b.root, round, "prompt.md"))
	if strings.Contains(string(prompt), "filler line for the forty") || len(prompt) > 65536 {
		t.Fatalf("the large brief was inlined or the prompt is %d bytes", len(prompt))
	}
	staged, _ := os.ReadFile(filepath.Join(b.root, round, "staged", "task-direction.md"))
	payload, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "large-brief", "brief.md"))
	if len(staged) == 0 || string(staged) != string(payload) {
		t.Fatal("the staged task direction differs from the composed job brief")
	}

	b.setConf("dispatch.max-inline-input-kb=8\n")
	temporaries := compositionTemporaries(t, b)
	for attempt := 1; attempt <= 2; attempt++ {
		result := b.dispatchAs(criticArgv(outputs, "large.md", large, "packet-bound", "--wait")...)
		requireExit(t, result, 9, b.stderr.String())
		requireInlineLimitRefusal(t, result.Stdout, 8192)
		requireNothingPublished(t, b, "packet-bound")
		if after := compositionTemporaries(t, b); after != temporaries {
			t.Fatalf("attempt %d left composition temporaries: %q", attempt, after)
		}
	}
}

// compositionTemporaries lists the composition temporaries under
// record-locks, as the bash bed's inventory did.
func compositionTemporaries(t *testing.T, b *bed) string {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(b.root, "artifacts", "agents", "record-locks"))
	var names []string
	for _, entry := range entries {
		for _, prefix := range []string{"composed-packet.", "composition.", "composed-staged.", "follow-composed-packet.", "follow-composition.", "follow-composed-staged."} {
			if strings.HasPrefix(entry.Name(), prefix) {
				names = append(names, entry.Name())
			}
		}
	}
	return strings.Join(names, ",")
}

// requireInlineLimitRefusal is assert_typed_inline_limit_refusal: exactly
// one typed REFUSED-INLINE-INPUT-LIMIT line whose detail carries a packet
// larger than the cap, the whole packet as overhead, and no shell advice.
func requireInlineLimitRefusal(t *testing.T, stdout []byte, capBytes int) {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(string(stdout), "\n") {
		if strings.Contains(line, `"outcome":"REFUSED-INLINE-INPUT-LIMIT"`) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("want exactly one inline-limit refusal, got %q", stdout)
	}
	var refusal map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &refusal); err != nil {
		t.Fatalf("refusal is not JSON: %v", err)
	}
	if refusal["headline"] != "refused" || refusal["source"] != "dispatch.max-inline-input-kb" {
		t.Fatalf("refusal %v", refusal)
	}
	detail, _ := refusal["detail"].(string)
	match := regexp.MustCompile(`^role packet exceeds dispatch\.max-inline-input-kb: ([0-9]+) bytes > ([0-9]+) bytes; fixed/reference overhead ([0-9]+) bytes$`).FindStringSubmatch(detail)
	if match == nil {
		t.Fatalf("detail template %q", detail)
	}
	var packet, limit, overhead int
	fmt.Sscan(match[1], &packet)
	fmt.Sscan(match[2], &limit)
	fmt.Sscan(match[3], &overhead)
	if limit != capBytes || packet <= limit || overhead != packet || strings.Contains(detail, "pass a file reference") {
		t.Fatalf("inconsistent counts in %q", detail)
	}
}

// engine-owns-disk-lifetimes U5f: the job worktree a dispatch creates is a
// registered store before it holds a byte (the chain's delegate workspace,
// identified by its gitdir and .git inode), and its quarantine is linked
// into the common store's alternates under the alternates lock.
func TestDispatchIntegrationRegistersTheJobWorktreeAsTheChainsWorkspace(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("code.md", "implement", "Build it.")
	requireExit(t, b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief, "--job-id", "registered-worktree", "--worktree"), 0, b.stderr.String())
	records, unreadable := diskstore.CheckoutRegistry(b.root).Inventory()
	if len(unreadable) != 0 || len(records) != 1 {
		t.Fatalf("one registered store: %+v %+v", records, unreadable)
	}
	record := records[0]
	worktree := filepath.Join(b.root, "artifacts", "agents", "worktrees", "registered-worktree")
	if record.Class != diskstore.DelegateClass || record.Owner != (diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "registered-worktree"}) ||
		record.State != diskstore.StateAccepted || record.Identity.Gitdir == "" || record.Identity.Marker || record.Layout != diskstore.LayoutCopy {
		t.Fatalf("the job worktree is the chain's accepted delegate workspace: %+v", record)
	}
	if err := diskstore.Revalidate(record); err != nil {
		t.Fatalf("the record names the worktree at %s: %v", worktree, err)
	}
	alternates, err := os.ReadFile(filepath.Join(b.root, ".git", "objects", "info", "alternates"))
	if err != nil || !strings.Contains(string(alternates), filepath.Join(record.Identity.Gitdir, diskstore.QuarantineName)) {
		t.Fatalf("the quarantine is linked: %q %v", alternates, err)
	}
}

// TestDispatchRepeatAfterASetupRefusalTakesTheNextAttempt: a start refused
// at setup leaves a husk under the request's derived name; the same request
// again is its next attempt under a fresh name, says which start it follows,
// and leaves the husk as it was.
func TestDispatchRepeatAfterASetupRefusalTakesTheNextAttempt(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("generated.md", "implement", "Generated.")
	derived, err := dispatch.DefaultOperationID("", 0, dispatch.DispatchModeFresh, "implementer", sha256Hex(t, brief), "")
	if err != nil {
		t.Fatal(err)
	}
	husk := `{"jobId":"` + derived + `","role":"implementer","status":"failed","phase":"setup","refusalClass":"setup","error":"dispatch-refused"}`
	b.writeFile("artifacts/agents/jobs/"+derived+".json", husk)
	result := b.dispatchAs("dispatch", "--role", "implementer", "--brief", brief)
	requireExit(t, result, 0, b.stderr.String())
	if job := strings.TrimSpace(string(result.Stdout)); job != dispatch.OperationAttempt(derived, 2) || !strings.Contains(b.stderr.String(), "refused at setup before it: "+derived) {
		t.Fatalf("the repeat after a setup refusal: job %q, stderr %q; want attempt 2 of %s", job, b.stderr.String(), derived)
	}
	if record := b.record(derived); record["status"] != "failed" || record["phase"] != "setup" {
		t.Fatalf("the husk changed: %v", record)
	}
}
