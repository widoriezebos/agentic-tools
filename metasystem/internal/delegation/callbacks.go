package delegation

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// callback is the router's internal entries: the callbacks runtime adapters
// make into the lifecycle (record compare-and-swap, handshake, custody,
// protocol error, repair claim, cancellation) and the dispatcher's own
// lease-held steps, each behind its control-plane authority for the supplied
// caller.
func (s *session) callback(command string, args []string) error {
	firstJob := func() (string, bool) {
		if len(args) >= 2 && args[0] == "--job" {
			return args[1], true
		}
		return "", false
	}
	switch command {
	case "__record-create":
		job, _ := firstJob()
		if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
			return err
		}
		return s.verbRecordFile("job record-create", args, func(job, source string) error {
			return s.l.ports.Records.Create(job, source)
		})
	case "__record-setup":
		job, _ := firstJob()
		if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
			return err
		}
		return s.verbRecordFile("job record-setup", args, func(job, source string) error {
			return s.l.ports.Records.Setup(job, source)
		})
	case "__record-cas":
		job, ok := firstJob()
		if !ok {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityRecordWriter, job); err != nil {
			return err
		}
		return s.verbRecordCAS(args)
	case "__critique-close":
		if len(args) < 2 || args[0] != "--root-job" {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityRecordWriter, args[1]); err != nil {
			return err
		}
		return s.verbCritiqueClose(args)
	case "__protocol-error":
		job, ok := firstJob()
		if !ok {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityAdapterWriter, job); err != nil {
			return err
		}
		return s.verbProtocolError(args)
	case "__repair-claim":
		job, ok := firstJob()
		if !ok {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityRecordWriter, job); err != nil {
			return err
		}
		return s.verbRepairClaim(args)
	case "__critique-register-advance", "__critique-read-admission", "__critique-exhaustion-advance", "__review-reference-reconcile":
		return s.callbackCritiqueMutation(strings.TrimPrefix(command, "__"), args)
	case "__launch":
		return s.callbackLaunch(args)
	case "__handshake-timeout":
		if len(args) != 2 || args[0] != "--job" {
			return exitWith(2)
		}
		return s.internalHandshakeTimeout(args[1])
	case "__reap-held":
		job, purpose, err := s.parseReapArgs(args)
		if err != nil {
			return err
		}
		return s.internalReapHeld(job, purpose)
	case "__handshake":
		job, ok := firstJob()
		if !ok {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityAdapterWriter, job); err != nil {
			return err
		}
		return s.callbackHandshake(args)
	case "__cancel-owned":
		if len(args) != 2 || args[0] != "--job" {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityHolderOnly, args[1]); err != nil {
			return err
		}
		return s.internalCancel(args[1])
	case "__breach-stop-goal":
		return s.callbackBreachStopGoal(args)
	case "__register-custody":
		job, ok := firstJob()
		if !ok {
			return exitWith(2)
		}
		if err := s.internalAuthority(AuthorityAdapterWriter, job); err != nil {
			return err
		}
		return s.callbackRegisterCustody(args)
	}
	return s.usageExit()
}

// flags is a verb's strict flag set over the session's stderr.
func (s *session) flags(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(s.stderr)
	return set
}

// verbRecordFile is job record-create / record-setup.
func (s *session) verbRecordFile(name string, args []string, write func(job, source string) error) error {
	set := s.flags(name)
	job := set.String("job", "", "job id")
	source := set.String("source", "", "record file")
	if set.Parse(args) != nil {
		return exitWith(2)
	}
	if *job == "" || *source == "" {
		s.eprintln(name + ": --root, --job, and --source are required")
		return exitWith(2)
	}
	return s.verbFailure(write(*job, *source))
}

// verbRecordCAS is job record-cas: the lost compare's observation on stdout.
func (s *session) verbRecordCAS(args []string) error {
	set := s.flags("job record-cas")
	job := set.String("job", "", "job id")
	expect := set.String("expect", "", "expected status")
	status := set.String("status", "", "target status")
	patch := set.String("patch", "", "patch file")
	if set.Parse(args) != nil {
		return exitWith(2)
	}
	if *job == "" || *expect == "" || *status == "" || *patch == "" {
		s.eprintln("job record-cas: --root, --job, --expect, --status, and --patch are required")
		return exitWith(2)
	}
	observed, err := s.l.ports.Records.CAS(*job, *expect, *status, *patch)
	if observed != "" {
		s.println(observed)
	}
	return s.verbFailure(err)
}

func (s *session) verbCritiqueClose(args []string) error {
	if repeated := repeatedFlag(args); repeated != "" {
		s.eprintf("job critique-close: flag --%s repeated; authority-bearing flags parse strictly\n", repeated)
		return exitWith(2)
	}
	set := s.flags("job critique-close")
	rootJob := set.String("root-job", "", "critic root")
	runnerClosed := set.Bool("runner-closed", false, "mark the runner closed")
	if set.Parse(args) != nil || set.NArg() != 0 || *rootJob == "" {
		s.eprintln("job critique-close: --root-job is required")
		return exitWith(2)
	}
	return s.verbFailure(dispatch.CritiqueChainClose(s.root, *rootJob, *runnerClosed))
}

func (s *session) verbProtocolError(args []string) error {
	set := s.flags("job record-protocol-error")
	job := set.String("job", "", "job id")
	expect := set.String("expect", "", "expected status")
	violation := set.String("violation", "", "violation text")
	violationFile := set.String("violation-file", "", "violation file")
	if set.Parse(args) != nil {
		return exitWith(2)
	}
	if *job == "" || *expect == "" {
		s.eprintln("job record-protocol-error: --root, --job, and --expect are required")
		return exitWith(2)
	}
	return s.verbFailure(dispatch.RecordProtocolError(s.root, *job, *expect, *violation, *violationFile))
}

// verbRepairClaim claims the round's one paid repair: 0 won, 3 lost (the
// observation on stdout), 1 mechanical.
func (s *session) verbRepairClaim(args []string) error {
	set := s.flags("job repair-claim")
	job := set.String("job", "", "job id")
	if set.Parse(args) != nil {
		return exitWith(2)
	}
	if *job == "" {
		s.eprintln("job repair-claim: --root and --job are required")
		return exitWith(2)
	}
	observed, err := dispatch.RepairClaim(s.root, *job)
	if observed != "" {
		s.println(observed)
	}
	return s.verbFailure(err)
}

// repeatedFlag is refuseRepeatedFlags: the name of the first repeated flag.
func repeatedFlag(args []string) string {
	seen := map[string]bool{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if index := strings.IndexByte(name, '='); index >= 0 {
			name = name[:index]
		}
		if name == "" {
			continue
		}
		if seen[name] {
			return name
		}
		seen[name] = true
	}
	return ""
}

// callbackCritiqueMutation is internal_critique_mutation: holder-only
// authority for the chain root the mutation names, then the owner.
func (s *session) callbackCritiqueMutation(verb string, args []string) error {
	rootJob := ""
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--root-job" {
			rootJob = args[index+1]
			break
		}
	}
	if !validID(rootJob) {
		return exitWith(2)
	}
	if err := s.internalAuthority(AuthorityHolderOnly, rootJob); err != nil {
		return err
	}
	return s.critiqueVerb(verb, args)
}

// critiqueVerb runs one critique register owner with its verb's flags.
func (s *session) critiqueVerb(verb string, args []string) error {
	name := "job " + verb
	if repeated := repeatedFlag(args); repeated != "" {
		s.eprintf("%s: flag --%s repeated; authority-bearing flags parse strictly\n", name, repeated)
		return exitWith(2)
	}
	set := s.flags(name)
	switch verb {
	case "critique-register-advance":
		rootJob := set.String("root-job", "", "")
		roundJob := set.String("round-job", "", "")
		if set.Parse(args) != nil {
			return exitWith(2)
		}
		if *rootJob == "" || *roundJob == "" {
			s.eprintln("job critique-register-advance: --repo, --root-job, and --round-job are required")
			return exitWith(2)
		}
		outcome, err := dispatch.CritiqueRegisterAdvance(s.root, *rootJob, *roundJob)
		if err != nil {
			return s.verbFailure(err)
		}
		s.println(outcome)
		return nil
	case "critique-read-admission":
		role := set.String("role", "", "")
		rootJob := set.String("root-job", "", "")
		round := set.Int64("round", 0, "")
		goalID := set.String("goal", "", "")
		subjectFile := set.String("subject-file", "", "")
		resultFile := set.String("result", "", "")
		if set.Parse(args) != nil || set.NArg() != 0 {
			return exitWith(2)
		}
		if *role == "" || *rootJob == "" || *subjectFile == "" || *resultFile == "" {
			s.eprintln("job critique-read-admission: --repo, --role, --root-job, --round, --subject-file, and --result are required")
			return exitWith(2)
		}
		return s.critiqueReadAdmission(*role, *rootJob, *goalID, *round, *subjectFile, *resultFile)
	case "critique-exhaustion-advance":
		rootJob := set.String("root-job", "", "")
		role := set.String("role", "", "")
		message := set.String("message", "", "")
		successor := set.String("successor", "", "")
		if set.Parse(args) != nil {
			return exitWith(2)
		}
		if *rootJob == "" || *role == "" || *message == "" || *successor == "" {
			s.eprintln("job critique-exhaustion-advance: --repo, --root-job, --role, --message, and --successor are required")
			return exitWith(2)
		}
		action, err := dispatch.CritiqueExhaustionAdvance(s.root, *rootJob, *role, *message, *successor)
		if err != nil {
			return s.verbFailure(err)
		}
		s.println(action)
		return nil
	case "review-reference-reconcile":
		rootJob := set.String("root-job", "", "")
		evidenceJob := set.String("evidence-job", "", "")
		if set.Parse(args) != nil {
			return exitWith(2)
		}
		if set.NArg() != 0 || *rootJob == "" || *evidenceJob == "" {
			s.eprintln("job review-reference-reconcile: --repo, --root-job, and --evidence-job are required")
			return exitWith(2)
		}
		return s.verbFailure(dispatch.ReconcileReviewReference(s.root, *rootJob, *evidenceJob))
	}
	return exitWith(2)
}

// critiqueReadAdmission admits a computed critic read before its job is
// published, writing the structured result either way.
func (s *session) critiqueReadAdmission(role, rootJob, goalID string, round int64, subjectFile, resultFile string) error {
	result := dispatch.ReadAdmissionResult{}
	var admissionErr error
	data, err := os.ReadFile(subjectFile)
	if err != nil {
		admissionErr = fmt.Errorf("read critique subject from %s: %w", subjectFile, err)
	} else {
		var raw any
		if err := json.Unmarshal(data, &raw); err != nil {
			admissionErr = fmt.Errorf("decode critique subject from %s: %w", subjectFile, err)
		} else if subject, present, err := readsubject.DecodeReadSubject(raw); err != nil {
			admissionErr = fmt.Errorf("decode critique subject from %s: %w", subjectFile, err)
		} else if !present {
			admissionErr = fmt.Errorf("critique subject file %s contains no subject", subjectFile)
		} else {
			result, admissionErr = dispatch.CritiqueReadAdmissionForGoal(s.root, role, rootJob, goalID, round, subject)
		}
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err == nil {
		err = os.WriteFile(resultFile, append(encoded, '\n'), 0o600)
	}
	if err != nil {
		return s.verbFailure(fmt.Errorf("write critique read admission result to %s: %w", resultFile, err))
	}
	return s.verbFailure(admissionErr)
}

func (s *session) callbackLaunch(args []string) error {
	var runtime, verb, job, tag, capability string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return exitWith(2)
		}
		switch args[index] {
		case "--runtime":
			runtime = args[index+1]
		case "--verb":
			verb = args[index+1]
		case "--job":
			job = args[index+1]
		case "--tag":
			tag = args[index+1]
		case "--launch-capability":
			capability = args[index+1]
		default:
			return exitWith(2)
		}
	}
	if runtime == "" || (verb != "dispatch" && verb != "follow-up") || job == "" || tag == "" || capability == "" {
		return exitWith(2)
	}
	return s.internalLaunch(runtime, verb, job, tag, capability)
}

// internalLaunch is the __launch entry as a call: holder-only authority for
// the job, then the adapter launch.
func (s *session) internalLaunch(runtime, verb, job, tag, capability string) error {
	if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
		return err
	}
	return s.launchAdapter(runtime, verb, job, tag, capability)
}

// internalRecordSetup is the __record-setup entry as a call.
func (s *session) internalRecordSetup(job, source string) error {
	if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
		return err
	}
	return s.verbFailure(s.l.ports.Records.Setup(job, source))
}

// internalRecordCAS is the __record-cas entry as a call.
func (s *session) internalRecordCAS(job, expect, status, patch string) error {
	_, err := s.casEntry(job, expect, status, patch)
	return err
}

// internalReapHeld is the __reap-held entry as a call: holder-only
// authority, then the lease-reentrant reap.
func (s *session) internalReapHeld(job, purpose string) error {
	if err := s.internalAuthority(AuthorityHolderOnly, ""); err != nil {
		return err
	}
	s.leaseReentry = true
	defer func() { s.leaseReentry = false }()
	args := []string{}
	if job != "" {
		args = append(args, "--job", job)
	}
	if purpose != "" {
		args = append(args, "--purpose", purpose)
	}
	return s.reapJobs(args)
}

// callbackHandshake is internal_handshake: evaluate the adapter's reported
// session into its record patch and move the record from pending.
func (s *session) callbackHandshake(args []string) error {
	var job, session, turn, model, effective, signal string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return exitWith(2)
		}
		value := args[index+1]
		switch args[index] {
		case "--job":
			job = value
		case "--session":
			session = value
		case "--turn":
			turn = value
		case "--model":
			model = value
		case "--effective":
			effective = value
		case "--signal":
			signal = value
		default:
			return exitWith(2)
		}
	}
	if !validID(job) || !isFile(effective) || !isFile(s.recordPath(job)) {
		return exitWith(2)
	}
	if signal != "true" && signal != "false" {
		s.eprintf("invalid value %q for flag -signal: must be true or false\n", signal)
		return exitWith(1)
	}
	patch, err := s.mustTemp(s.recordLocks, "handshake-patch")
	if err != nil {
		return err
	}
	defer removeQuietly(patch)
	if err := dispatch.HandshakeEval(s.recordPath(job), effective, session, turn, model, signal == "true", patch); err != nil {
		_ = s.verbFailure(err)
		return exitWith(1)
	}
	target, ok := field(patch, "target")
	if !ok {
		return exitWith(1)
	}
	bodyText, ok := field(patch, "patch")
	if !ok {
		return exitWith(1)
	}
	body, err := s.mustTemp(s.recordLocks, "handshake-body")
	if err != nil {
		return err
	}
	if err := os.WriteFile(body, []byte(bodyText+"\n"), 0o600); err != nil {
		return exitWith(1)
	}
	if _, err := s.recordCAS(job, "pending", target, body); err != nil {
		return err
	}
	if target != "running" {
		return exitWith(1)
	}
	return nil
}

// callbackRegisterCustody is internal_register_custody: append a live custody
// process under the record's locks, its observed start second a binding
// cross-check.
func (s *session) callbackRegisterCustody(args []string) error {
	var job, pidText string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return exitWith(2)
		}
		switch args[index] {
		case "--job":
			job = args[index+1]
		case "--pid":
			pidText = args[index+1]
		default:
			return exitWith(2)
		}
	}
	if !validID(job) || !positiveInteger(pidText) || !isFile(s.recordPath(job)) {
		return exitWith(2)
	}
	pid, _ := strconv.ParseInt(pidText, 10, 64)
	started, err := s.l.ports.Process.StartedAt(pid)
	if err != nil {
		return exitWith(1)
	}
	processes, err := s.l.ports.Process.ClaimProcesses()
	if err != nil {
		return s.verbFailure(err)
	}
	exact, state, probeErr := processes.Reader.ReadStart(pid)
	if probeErr != nil || state != identity.Alive {
		s.eprintf("job custody-add: pid %d identity unreadable or not alive\n", pid)
		return exitWith(1)
	}
	if exact.StartedAt.Unix() != started {
		s.eprintf("job custody-add: pid %d start %d does not match the recorded %d\n", pid, exact.StartedAt.Unix(), started)
		return exitWith(1)
	}
	return s.verbFailure(dispatch.CustodyAdd(s.root, job, pid, processes.Reader))
}

// callbackBreachStopGoal is internal_breach_stop_goal: the stop custodian
// closes one exact breached revision and completes its cancellation pass.
func (s *session) callbackBreachStopGoal(args []string) error {
	var goalID, revisionText string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return exitWith(2)
		}
		switch args[index] {
		case "--goal":
			goalID = args[index+1]
		case "--revision":
			revisionText = args[index+1]
		default:
			return exitWith(2)
		}
	}
	if !validID(goalID) || !positiveInteger(revisionText) {
		return exitWith(2)
	}
	revision, _ := strconv.ParseUint(revisionText, 10, 64)
	return s.BreachStopGoal(goalID, revision)
}

// BreachStopGoal is the stop-custodian gate, then the breach stop of one
// exact revision and its cancellation pass; it prints the stop line.
func (s *session) BreachStopGoal(goalID string, revision uint64) error {
	if err := s.internalAuthority(AuthorityStopCustodian, ""); err != nil {
		return err
	}
	batch, err := s.breachStop(goalID, revision)
	if err != nil {
		return s.die(1, fmt.Sprintf("breach-stop could not close %s revision %d", goalID, revision))
	}
	if err := s.breachStopRun(batch.StopID); err != nil {
		return err
	}
	s.printf("stop=%s state=COMPLETE\n", batch.StopID)
	return nil
}

// internalHandshakeTimeout is internal_handshake_timeout: the dispatcher's
// verdict on its own handshake wait, recorded before the stalled group is
// wound down, standing down when a session landed late.
func (s *session) internalHandshakeTimeout(job string) error {
	if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
		return err
	}
	record := s.recordPath(job)
	logPath := filepath.Join(s.jobs, job+".log")
	note := func(format string, args ...any) {
		_ = appendFile(logPath, []byte(s.nowISO()+" "+fmt.Sprintf(format, args...)+"\n"))
	}
	status := fieldOr(record, "status")
	note("handshake-timeout entered status=%s", status)
	if status != "pending" && status != "running" {
		return nil
	}
	// Stand down before killing anything if a session already landed: the
	// wait was won, just late.
	if session := fieldOr(record, "sessionId"); session != "" && session != "null" {
		note("handshake-timeout stood down; session %s landed before wind-down", session)
		return nil
	}
	// Record the verdict before killing the group, retrying a lost compare
	// (an adapter moves the record pending to running in this window).
	recorded := false
	for attempt := 0; attempt < 3; attempt++ {
		status = fieldOr(record, "status")
		if status != "pending" && status != "running" {
			note("handshake-timeout stood down; record is already %s", status)
			return nil
		}
		if session := fieldOr(record, "sessionId"); session != "" && session != "null" {
			note("handshake-timeout stood down; session %s landed while it was being written", session)
			return nil
		}
		patch, err := s.mustTemp(s.recordLocks, "handshake")
		if err != nil {
			return err
		}
		if err := writePatch(patch, `{"error":"handshake_timeout","phase":"handshake"}`); err != nil {
			return exitWith(1)
		}
		if _, err := s.recordCAS(job, status, "failed", patch); err == nil {
			note("handshake-timeout recorded from %s", status)
			recorded = true
			break
		}
	}
	if !recorded {
		note("handshake-timeout lost three compares; the record kept changing")
		return exitWith(1)
	}
	// The verdict stands; cleaning up the stalled group is best effort.
	if !s.windDownGroup(record) {
		note("handshake-timeout recorded, but the group did not wind down cleanly")
	}
	return nil
}

// guardFixtureWait is checkout_execution_guard_fixture_wait: the process
// fixture's control record drives guard participation only (a ready file, a
// release file, an optional detached member, an optional nested dispatch).
func (s *session) guardFixtureWait() error {
	control := s.env.GuardFixture
	if !isFile(control) {
		s.eprintln("checkout execution guard fixture: control record is absent")
		return exitWith(2)
	}
	ready, ok1 := field(control, "ready")
	release, ok2 := field(control, "release")
	capText, ok3 := field(control, "capSec")
	if !ok1 || !ok2 || !ok3 {
		return exitWith(2)
	}
	if !filepath.IsAbs(ready) || !filepath.IsAbs(release) || !positiveInteger(capText) {
		s.eprintln("checkout execution guard fixture: invalid control record")
		return exitWith(2)
	}
	if err := touch(ready); err != nil {
		return exitWith(2)
	}
	detachReady := fieldOr(control, "detachReady")
	detachRelease := fieldOr(control, "detachRelease")
	if detachReady+detachRelease != "" {
		if !filepath.IsAbs(detachReady) || !filepath.IsAbs(detachRelease) {
			s.eprintln("checkout execution guard fixture: detached controls are incomplete")
			return exitWith(2)
		}
		scripts := filepath.Join(s.root, "scripts", "agents")
		_, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{
			Argv: []string{filepath.Join(scripts, "checkout-execution-guard.sh"), "run-member", "--root", s.guardRoot, "--engine", s.ms, "--",
				filepath.Join(scripts, "checkout-execution-guard-fixtures.sh"), "__wait-only", detachReady, detachRelease, capText},
			Dir: s.root, GuardRoot: s.guardRoot, GuardOwner: "dispatch fixture detached member",
		})
		if err != nil {
			s.eprintln(err.Error())
			return exitWith(1)
		}
		return nil
	}
	childControl := fieldOr(control, "childControl")
	childBrief := fieldOr(control, "childBrief")
	var child *exec.Cmd
	if childControl+childBrief != "" {
		if !isFile(childControl) || !isFile(childBrief) {
			s.eprintln("checkout execution guard fixture: nested dispatch controls are incomplete")
			return exitWith(2)
		}
		child = exec.Command(s.ms, "internal", "delegate", "--role", "implementer", "--brief", childBrief,
			"--goal", "none-explicit", "--destructive-reach", "DESIGN-BEARING", "--op", fmt.Sprintf("checkout-guard-nested-%d", os.Getpid()))
		child.Env = append(os.Environ(), "METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="+childControl,
			"METASYSTEM_BIN="+s.ms, "METASYSTEM_DELEGATE_ROOT="+s.root)
		child.Stdout, child.Stderr = &s.stdout, s.stderr
		if err := child.Start(); err != nil {
			s.eprintln(err.Error())
			return exitWith(1)
		}
	}
	seconds, _ := strconv.ParseInt(capText, 10, 64)
	clock := s.l.ports.Clock
	deadline := clock.Now().Add(time.Duration(seconds) * time.Second)
	for !exists(release) {
		if !clock.Now().Before(deadline) {
			s.eprintf("checkout execution guard fixture: release wait expired after %ss\n", capText)
			return exitWith(1)
		}
		clock.Sleep(50 * time.Millisecond)
	}
	if child != nil {
		if err := child.Wait(); err != nil {
			return exitWith(exitCodeOf(err))
		}
	}
	return nil
}
