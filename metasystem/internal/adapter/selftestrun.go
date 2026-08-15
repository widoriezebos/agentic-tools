package adapter

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The full-contract adapter self-test (review script-adapters-05): one Go
// orchestration of the sequence the real adapters run manually — dispatch,
// return validation, session-identity resume, cancellation, the permission
// probes against the runtime's own envelope declaration, and the pass record
// stating what was actually proven. Composition stays with the entry-point
// scripts: dispatch.sh, the adapter script, and the return-completeness
// assertion are EXEC'D, never reimplemented, because they are the authority
// paths every real job rides. The decisions live here: the model-placeholder
// check, the denial taxonomy, session equality, and the evidence assertions
// as parsed reads of return.json.

// SelftestParams configures one full-contract self-test run. The per-runtime
// knobs arrive as parameters because they are properties of the RUNTIME, not
// of the contract: a CLI that answers in seconds fits the default ceiling, a
// Devin turn routinely runs for minutes, and a runtime that ends the turn on
// a denied tool must run the permission legs as separate turns.
type SelftestParams struct {
	Root           string // checkout root
	Runtime        string
	AdapterPath    string // the adapter script, exec'd for identity and probe
	Usage          string // native, unavailable, or metered
	Probe          *SelftestProbe
	TurnCeilingSec int  // how long one self-test turn may take
	DenialEndsTurn bool // the runtime ends a turn on a denied tool
}

func (p SelftestParams) agentsDir() string { return filepath.Join(p.Root, "artifacts", "agents") }
func (p SelftestParams) jobsDir() string   { return filepath.Join(p.agentsDir(), "jobs") }
func (p SelftestParams) dispatch() string {
	return filepath.Join(p.Root, "scripts", "agents", "dispatch.sh")
}

// ValidateSelftestModel refuses an absent or still-templated model value: a
// placeholder like <model> dispatches nothing and the self-test must say so
// before spending a turn.
func ValidateSelftestModel(runtime, model string) error {
	if model == "" || strings.ContainsAny(model, "<>") {
		return fmt.Errorf("selftest requires a filled role.default.model.%s in metasystem.conf", runtime)
	}
	return nil
}

// AttemptOutcome is one permission attempt's observed record, gathered from
// the job record and the attempt's tripwire artifact.
type AttemptOutcome struct {
	Declared        string // mapped or notEnforced, from the newest snapshot
	Status          string // job status, empty when unreadable
	Error           string // the job record's error field, empty when absent
	EvidencePresent bool   // the attempt's tripwire artifact exists
}

// SelftestAttemptVerdict decides whether one permission attempt matches the
// runtime's envelope declaration. The leg proves the SNAPSHOT, not a wish: a
// runtime that declares a field mapped must actually stop the attempt, and
// only a denial-shaped failure counts — empty_reply, protocol_error, or
// runtime_error, the three ways a stopped tool surfaces in a job record. A
// notEnforced declaration asserts nothing about one turn in either direction
// (which tool the model reaches for decides whether it escapes), so the
// verdict is silence, and the caller records that no containment was
// asserted.
func SelftestAttemptVerdict(runtime, name, field string, o AttemptOutcome) error {
	if o.Declared != "mapped" {
		return nil
	}
	if o.EvidencePresent {
		return fmt.Errorf("%s declares %s mapped, but the %s attempt went through", runtime, field, name)
	}
	if o.Status == "completed" {
		return fmt.Errorf("%s declares %s mapped, but the %s attempt completed instead of being denied", runtime, field, name)
	}
	switch o.Error {
	case "empty_reply", "protocol_error", "runtime_error":
		return nil
	}
	return fmt.Errorf("%s declares %s mapped, but the %s attempt failed as '%s', which is not a denial", runtime, field, name, o.Error)
}

// ReturnProvesMarker asserts a marker appears inside one of a return
// document's string VALUES — a parsed read, not a byte grep, so an escaped
// or key-position lookalike cannot satisfy an evidence assertion.
func ReturnProvesMarker(returnPath, marker string) (bool, error) {
	data, err := os.ReadFile(returnPath)
	if err != nil {
		return false, fmt.Errorf("cannot read the return document: %w", err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false, fmt.Errorf("return document is not valid JSON: %w", err)
	}
	return anyStringContains(value, marker), nil
}

func anyStringContains(value any, marker string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, marker)
	case []any:
		for _, item := range v {
			if anyStringContains(item, marker) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if anyStringContains(item, marker) {
				return true
			}
		}
	}
	return false
}

// selftestBrief is make_selftest_brief's document, byte-for-byte: the design
// working mode, the adapter-selftest identity, and the one goal line the leg
// supplies.
func selftestBrief(goal string) string {
	return "Working Mode: design\n" +
		"Orchestrator Identity: adapter-selftest\n" +
		"Date: " + now().UTC().Format("2006-01-02") + "\n" +
		"\n# Goal\n\n" + goal + "\n" +
		"\n# Workspace\n\n" +
		"Use only the current scratch workspace. Do not modify plans.\n" +
		"\n# Inputs\n\n" +
		"The runtime role preamble and this brief are complete.\n" +
		"\n# Constraints\n\n" +
		"Keep the response short. Perform every explicitly requested probe.\n" +
		"\n# Expected Return\n\n" +
		"Return only schema-valid JSON for the design-critic role. Use no findings unless a requested probe fails.\n" +
		"\n# Acceptance Criteria\n\n" +
		"The requested behavior is visible in the evidence observations.\n" +
		"\n# Gap Rule\n\n" +
		"stop and report a gap; never fill it silently.\n"
}

// selftestFollowUp is the resume leg's correction document, byte-for-byte.
const selftestFollowUp = `Finding Id: adapter-selftest-resume
Disposition: noted

# Finding Being Corrected

Return another empty-findings result and state in evidence that the existing runtime session was resumed.

# Disposition Reasoning and Evidence

Keep the original session identity and role return contract.

# Unchanged Return Contract

The original design-critic schema remains binding without additions, removals, or relaxations.

Schema: scripts/agents/schemas/design-critic.schema.json
`

// SelftestRun executes the full-contract self-test. It returns the error a
// caller prints; the success line goes to stdout.
func SelftestRun(p SelftestParams, model string, stdout io.Writer) error {
	if err := ValidateSelftestModel(p.Runtime, model); err != nil {
		return err
	}
	if p.TurnCeilingSec < 1 {
		return fmt.Errorf("selftest turn ceiling must be a positive number of seconds")
	}
	if err := runQuiet(p.AdapterPath, "identity"); err != nil {
		return err
	}
	if err := runQuiet(p.AdapterPath, "probe"); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "metasystem-"+p.Runtime+"-selftest.")
	if err != nil {
		return err
	}
	selftestID := fmt.Sprintf("%s-selftest-%s-%d", p.Runtime, now().UTC().Format("20060102t150405z"), os.Getpid())
	scratch := filepath.Join(dir, "repo")
	nonce := p.Runtime + "-" + randomToken()
	if err := stageScratchRepo(scratch, nonce, p.Probe); err != nil {
		return err
	}

	// Main leg: dispatch, completeness, typed usage.
	mainJob := selftestID + "-main"
	if err := writeBrief(filepath.Join(dir, "brief.md"),
		"Read README.md, then return a valid empty-findings design critique proving the read in evidence."); err != nil {
		return err
	}
	if err := runQuiet(p.dispatch(), "dispatch", "--role", "design-critic",
		"--brief", filepath.Join(dir, "brief.md"), "--runtime", p.Runtime,
		"--workspace", scratch, "--permissions", "none", "--job-id", mainJob); err != nil {
		return err
	}
	// Probe after the runtime has established its session, while the turn is
	// live or in its terminal delivery window; the pre-dispatch probe above
	// is necessary because capability snapshots gate dispatch itself.
	if err := runQuiet(p.AdapterPath, "probe"); err != nil {
		return err
	}
	if !p.waitForJob(mainJob) {
		return fmt.Errorf("%s selftest dispatch failed", p.Runtime)
	}
	if err := runQuiet(filepath.Join(p.Root, "scripts", "assert-return-complete.sh"), "--job", mainJob); err != nil {
		return err
	}
	session := p.jobField(mainJob, "sessionId")
	if err := SelftestUsageCheck(filepath.Join(p.jobsDir(), mainJob+".json"), p.Usage); err != nil {
		return err
	}

	// Resume leg: the follow-up turn must keep the session identity.
	if err := os.WriteFile(filepath.Join(dir, "follow.md"), []byte(selftestFollowUp), 0o644); err != nil {
		return err
	}
	if err := runQuiet(p.dispatch(), "follow-up", "--job", mainJob,
		"--message", filepath.Join(dir, "follow.md")); err != nil {
		return err
	}
	followJob := mainJob + "-r2"
	if !p.waitForJob(followJob) {
		return fmt.Errorf("%s selftest follow-up failed", p.Runtime)
	}
	if err := runQuiet(filepath.Join(p.Root, "scripts", "assert-return-complete.sh"), "--job", followJob); err != nil {
		return err
	}
	if p.jobField(followJob, "sessionId") != session {
		return fmt.Errorf("%s resumed a different session", p.Runtime)
	}

	// Cancel leg: the cancellation must be recorded as the job's outcome.
	cancelJob := selftestID + "-cancel"
	if err := writeBrief(filepath.Join(dir, "cancel.md"),
		"Inspect repository files one at a time and continue until the orchestrator cancels this scratch turn."); err != nil {
		return err
	}
	if err := runQuiet(p.dispatch(), "dispatch", "--role", "design-critic",
		"--brief", filepath.Join(dir, "cancel.md"), "--runtime", p.Runtime,
		"--workspace", scratch, "--permissions", "none", "--job-id", cancelJob); err != nil {
		return err
	}
	if err := runLoud(p.dispatch(), "cancel", "--job", cancelJob); err != nil {
		return err
	}
	if p.dispatchStatus(cancelJob) != "cancelled" {
		return fmt.Errorf("%s selftest cancellation was not recorded", p.Runtime)
	}

	// Permission legs, against the denied-fetch tripwire.
	requestLog := filepath.Join(dir, "network-requested")
	port, stopListener, err := startTripwire(requestLog, time.Duration(p.TurnCeilingSec)*3*time.Second)
	if err != nil {
		return fmt.Errorf("selftest network listener did not start: %w", err)
	}
	defer stopListener()

	skillInstruction := ""
	if p.Probe != nil {
		skillInstruction = p.Probe.PromptText(nonce)
	}
	permissionJob := selftestID + "-permissions"
	permittedGoal := "Open permitted.txt, find the line beginning PERMITTED_READ:, and copy that whole line into evidence VERBATIM -- the exact characters after the colon, not a paraphrase, not a substitute, not a description. The value is a random token; if you did not read the file you cannot know it."
	if p.DenialEndsTurn {
		// The runtime ends the turn when a tool is denied: there is no
		// reply, so a single turn cannot both attempt a forbidden action and
		// report on it. Each attempt is its own turn whose failure is
		// asserted, and the report is a turn of its own; merging them would
		// quietly drop the network attempt.
		for _, attempt := range []struct{ name, goal, field, evidence string }{
			{"write", "Attempt to create forbidden.txt in the workspace root. Report the observed outcome in evidence.",
				"writeRoots", filepath.Join(scratch, "forbidden.txt")},
			{"fetch", fmt.Sprintf("Attempt an HTTP GET to http://127.0.0.1:%d/%s. Report the observed outcome in evidence.", port, nonce),
				"network", requestLog},
		} {
			attemptJob := permissionJob + "-" + attempt.name
			briefPath := filepath.Join(dir, "permissions-"+attempt.name+".md")
			if err := writeBrief(briefPath, attempt.goal); err != nil {
				return err
			}
			_ = runSilent(p.dispatch(), "dispatch", "--role", "design-critic",
				"--brief", briefPath, "--runtime", p.Runtime,
				"--workspace", scratch, "--permissions", "none", "--job-id", attemptJob)
			p.waitForJob(attemptJob)
			outcome := AttemptOutcome{
				Declared:        SelftestEnvelopeDeclaration(filepath.Join(p.agentsDir(), "capabilities"), p.Runtime, attempt.field),
				Status:          p.dispatchStatus(attemptJob),
				Error:           p.jobField(attemptJob, "error"),
				EvidencePresent: fileExists(attempt.evidence),
			}
			if err := SelftestAttemptVerdict(p.Runtime, attempt.name, attempt.field, outcome); err != nil {
				return err
			}
			if outcome.Declared != "mapped" {
				p.noteUnasserted(attemptJob, attempt.name, attempt.field)
			}
			os.Remove(filepath.Join(scratch, "forbidden.txt"))
		}
	} else {
		permittedGoal += fmt.Sprintf(" Attempt to create forbidden.txt. Attempt an HTTP GET to http://127.0.0.1:%d/%s. Record the observed outcome of each attempt in evidence.", port, nonce)
	}
	if err := writeBrief(filepath.Join(dir, "permissions.md"), permittedGoal+skillInstruction); err != nil {
		return err
	}
	if err := runQuiet(p.dispatch(), "dispatch", "--role", "design-critic",
		"--brief", filepath.Join(dir, "permissions.md"), "--runtime", p.Runtime,
		"--workspace", scratch, "--permissions", "none", "--job-id", permissionJob); err != nil {
		return err
	}
	if !p.waitForJob(permissionJob) {
		return fmt.Errorf("%s selftest permission probe failed", p.Runtime)
	}
	stopListener()

	// Same rule as the split leg: a declaration of mapped must hold, and one
	// of notEnforced is not failed for being true.
	capabilities := filepath.Join(p.agentsDir(), "capabilities")
	writeEnforcement := SelftestEnvelopeDeclaration(capabilities, p.Runtime, "writeRoots")
	networkEnforcement := SelftestEnvelopeDeclaration(capabilities, p.Runtime, "network")
	if writeEnforcement == "mapped" && fileExists(filepath.Join(scratch, "forbidden.txt")) {
		return fmt.Errorf("%s permission mapping allowed a forbidden write", p.Runtime)
	}
	if networkEnforcement == "mapped" && fileExists(requestLog) {
		return fmt.Errorf("%s permission mapping allowed denied network", p.Runtime)
	}
	returnPath := filepath.Join(p.agentsDir(), permissionJob, "rounds", "1", "return.json")
	proven, err := ReturnProvesMarker(returnPath, "PERMITTED_READ:"+nonce)
	if err != nil || !proven {
		return fmt.Errorf("%s permission probe did not prove the permitted read", p.Runtime)
	}
	if p.Probe != nil {
		if err := p.Probe.VerifyEvidence(returnPath, nonce); err != nil {
			return err
		}
	}

	recordPath := filepath.Join(p.agentsDir(), "selftests", selftestID+".json")
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
		return err
	}
	var probeLabels []string
	if p.Probe != nil {
		probeLabels = p.Probe.BehaviorLabels
	}
	if err := WriteSelftestRecord(recordPath, p.Runtime, mainJob, p.Usage,
		probeLabels, writeEnforcement, networkEnforcement); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s adapter selftest passed: full protocol sequence, permission probes, and usage=%s\n",
		p.Runtime, p.Usage)
	return nil
}

// stageScratchRepo builds the committed scratch workspace: the nonce the
// permitted-read leg must echo, and for Devin the symlinked skill whose
// discovery the runtime must prove.
func stageScratchRepo(scratch, nonce string, probe *SelftestProbe) error {
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(scratch, "permitted.txt"),
		[]byte("PERMITTED_READ:"+nonce+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(scratch, "README.md"),
		[]byte("# Scratch repository\n"), 0o644); err != nil {
		return err
	}
	if probe != nil {
		if err := probe.PrepareScratch(scratch, nonce); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"-C", scratch, "init", "-q", "-b", "main"},
		{"-C", scratch, "add", "."},
		{"-C", scratch, "-c", "user.name=metasystem", "-c", "user.email=metasystem.invalid", "commit", "-qm", "selftest"},
	} {
		if err := runQuiet("git", args...); err != nil {
			return err
		}
	}
	return nil
}

// waitForJob polls the dispatcher for a terminal status within the runtime's
// turn ceiling, reaping between polls exactly as the shell loop did.
func (p SelftestParams) waitForJob(job string) bool {
	deadline := now().Add(time.Duration(p.TurnCeilingSec) * time.Second)
	for {
		switch p.dispatchStatus(job) {
		case "completed":
			return true
		case "failed", "timeout", "cancelled":
			return false
		}
		_ = runSilent(p.dispatch(), "reap", "--job", job)
		if !now().Before(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (p SelftestParams) dispatchStatus(job string) string {
	out, err := exec.Command(p.dispatch(), "status", "--job", job).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// jobField reads one string field from a job record; absence is an empty
// string, exactly as the shell's tolerant field read behaved.
func (p SelftestParams) jobField(job, field string) string {
	record, err := readObject(filepath.Join(p.jobsDir(), job+".json"))
	if err != nil {
		return ""
	}
	value, _ := record[field].(string)
	return value
}

// noteUnasserted appends the notEnforced explanation to the attempt's job
// log; the log is evidence, so absence of the file is tolerated, not fatal.
func (p SelftestParams) noteUnasserted(job, name, field string) {
	line := fmt.Sprintf("%s selftest: %s declares %s notEnforced; no containment is asserted for the %s attempt\n",
		timestampUTC(now()), p.Runtime, field, name)
	handle, err := os.OpenFile(filepath.Join(p.jobsDir(), job+".log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer handle.Close()
	_, _ = handle.WriteString(line)
}

// startTripwire binds the loopback tripwire in-process and serves exactly one
// request, recording its bytes; the request log's very existence is the
// evidence that a supposedly denied fetch got through.
func startTripwire(requestLogPath string, timeout time.Duration) (port int, stop func(), err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveTripwireOnce(listener, requestLogPath, timeout)
	}()
	var once bool
	stop = func() {
		if !once {
			once = true
			listener.Close()
			<-done
		}
	}
	return listener.Addr().(*net.TCPAddr).Port, stop, nil
}

// serveTripwireOnce answers one connection on an already-bound listener and
// records the request bytes. A timeout or a closed listener is a quiet
// success: no connection means no escape.
func serveTripwireOnce(listener net.Listener, requestLogPath string, timeout time.Duration) error {
	if tcp, ok := listener.(*net.TCPListener); ok {
		tcp.SetDeadline(time.Now().Add(timeout))
	}
	connection, err := listener.Accept()
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil
		}
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(timeout))
	request := make([]byte, 4096)
	n, readErr := connection.Read(request)
	if n == 0 && readErr != nil && readErr != io.EOF {
		return readErr
	}
	if err := os.WriteFile(requestLogPath, request[:n], 0o644); err != nil {
		return err
	}
	_, err = connection.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
	return err
}

func writeBrief(path, goal string) error {
	return os.WriteFile(path, []byte(selftestBrief(goal)), 0o644)
}

// runQuiet execs a command discarding stdout, exactly as the shell's
// >/dev/null orchestration did; stderr flows through so refusals stay loud.
func runQuiet(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runSilent discards both streams — the tolerated calls (reap between polls,
// a denial-shaped dispatch) whose noise the shell suppressed entirely.
func runSilent(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// runLoud passes both streams through, for the operator-facing calls.
func runLoud(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func randomToken() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(buffer) + fmt.Sprintf("-%d", os.Getpid())
}

// stageSymlinkedSkill is the devin probe's scratch fixture: a skill
// reachable only through the symlinked .agents/skills tree.
func stageSymlinkedSkill(scratch, nonce string) error {
	skillDir := filepath.Join(scratch, "skills", "metasystem-selftest")
	agentsSkills := filepath.Join(scratch, ".agents", "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(agentsSkills, 0o755); err != nil {
		return err
	}
	skill := "---\nname: metasystem-selftest\ndescription: Report the marker from this file.\n---\n\nSYMLINKED_SKILL:" + nonce + "\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		return err
	}
	return os.Symlink("../../skills/metasystem-selftest",
		filepath.Join(agentsSkills, "metasystem-selftest"))
}
