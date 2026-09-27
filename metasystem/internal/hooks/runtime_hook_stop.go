package hooks

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// lifecycle is one receipt, Stop worker or SessionEnd invocation: the
// session, runtime identity and checkout holder resolved once, then the
// event's own work.
type lifecycle struct {
	inv Invocation
	ops Ops

	scriptDir, harnessRoot, world, engine string
	repo, session, transcript             string
	sessionAbsent, stopHookActive         bool
	payload                               string
	stopStarted                           int64

	identity, identityPid, identityStarted string
	mainID, mainClass, mainHolder          string
	seatMachine, seatLineage, seatClaimEp  string

	conditions   []string
	stopFailure  string
	evidenceFail string

	workDir string
}

var causeNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// stopCauseCode is the stable condition code of a fixed diagnostic.
func stopCauseCode(diagnostic string) string {
	code := causeNonWord.ReplaceAllString(strings.ToLower(diagnostic), "-")
	code = strings.Trim(code, "-")
	return strings.TrimPrefix(code, "the-")
}

func (l *lifecycle) recordFailure(diagnostic, component string) {
	l.conditions = append(l.conditions, stopCauseCode(diagnostic)+"|"+component)
	if l.stopFailure == "" {
		l.stopFailure = diagnostic
	}
}

// intentionalSkip ends the event quietly; a Stop worker tells its deadline
// parent the skip was intentional.
func (l *lifecycle) intentionalSkip() {
	if l.inv.Event == "stop" && l.inv.env(stopDeadlineParentEnv) == strconv.Itoa(l.inv.Ppid) {
		_ = writeLine(l.inv.Stdout, internalSkipResult)
	}
	exitHook(0)
}

func (l *lifecycle) refuse(message string, status int) {
	_ = writeLine(l.inv.Stderr, "supervision hook refused: "+message)
	exitHook(status)
}

func runLifecycle(inv Invocation, ops Ops) int {
	l := &lifecycle{inv: inv, ops: ops}
	var ok bool
	if l.scriptDir, ok = physicalDirectory(scriptParent(inv.Script)); !ok {
		return 0
	}
	if l.harnessRoot, ok = physicalDirectory(l.scriptDir + "/../.."); !ok {
		return 0
	}
	if inv.Event == "stop" && inv.env(stopDeadlineParentEnv) != strconv.Itoa(inv.Ppid) {
		return runStopDeadlineParent(inv, ops, l.harnessRoot)
	}
	l.stopStarted = inv.Now().Unix()
	if started := inv.env(stopDeadlineStartedEnv); digits.MatchString(started) {
		if parsed, err := strconv.ParseInt(started, 10, 64); err == nil {
			l.stopStarted = parsed
		}
	}
	defer l.cleanupWork()
	l.prepare()
	switch inv.Event {
	case "receipt":
		l.classifyHolder()
		l.receipt()
	case "stop":
		l.stop()
	default:
		l.classifyHolder()
		l.end()
	}
	return 0
}

// prepare resolves the installation, session, runtime identity and holder
// every lifecycle event uses.
func (l *lifecycle) prepare() {
	inv, ops := l.inv, l.ops
	// A job-bound adapter supplies evidence coordinates before launching any
	// provider child. The coordinates never authorize a skip: this live
	// ancestry must match that exact job's recorded custody.
	stateHint := inv.env("METASYSTEM_HOOK_DELEGATE_STATE_ROOT")
	installationHint := inv.env("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")
	jobHint := inv.env("METASYSTEM_HOOK_DELEGATE_JOB")
	if stateHint+installationHint+jobHint != "" {
		if stateHint == "" || installationHint == "" || jobHint == "" || !inv.IsExecutable(installationHint+"/bin/metasystem") {
			l.refuse("delegate context hint is incomplete or its engine is unavailable", 1)
		}
		result, status := ops.HookDelegate(stateHint, installationHint, jobHint, inv.Ppid)
		if status == 0 && strings.Contains(result, `"delegate":true`) {
			l.intentionalSkip()
		}
		l.refuse("supplied delegate context did not authenticate this process ancestry", 1)
	}
	// Worktree mapping locates the installation before the engine check. The
	// installation's own engine proves its provenance even when an override
	// runs the turn.
	var ok bool
	if l.world, ok = worldInstallation(ops, l.harnessRoot); !ok {
		exitHook(0)
	}
	canonical := l.world + "/bin/metasystem"
	l.engine = inv.env("METASYSTEM_BIN")
	if l.engine == "" {
		l.engine = canonical
	}
	if !inv.IsExecutable(canonical) || !inv.IsExecutable(l.engine) {
		if inv.Event == "stop" {
			_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "engine-missing"))
		}
		exitHook(0)
	}
	registered, status := ops.RuntimeNames()
	if status != 0 {
		l.refuse("runtime registry query failed (exit "+strconv.Itoa(status)+")", status)
	}
	if !containsLine(trimNewlines(registered), inv.Runtime) {
		l.refuse("runtime '"+inv.Runtime+"' is not registered", 2)
	}
	if inv.Event == "stop" {
		directory, err := os.MkdirTemp(inv.TempDir, "metasystem-stop-presentation.")
		if err != nil {
			_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "staging-failed"))
			exitHook(0)
		}
		l.workDir = directory
	}
	payload, err := readAllString(inv.Stdin)
	if err != nil {
		exitHook(1)
	}
	l.payload = payload
	if inv.Event == "stop" {
		// Missing optional fields keep their fallbacks, but an unreadable
		// payload is never mistaken for an empty one at turn end.
		empty := ""
		if _, status := jsonGet(payload, "__metasystem_shape_probe", false, &empty); status != 0 {
			exitHook(1)
		}
	}
	repo, status := ops.StateRoot(l.world)
	repo = trimNewlines(repo)
	if status == 1 {
		exitHook(0)
	} else if status != 0 || !oneLine(repo) {
		if inv.Event == "stop" {
			_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "engine-skew"))
		}
		exitHook(0)
	}
	if l.repo, ok = physicalDirectory(repo); !ok {
		exitHook(0)
	}
	l.session = jsonValue(payload, "session_id")
	empty := ""
	if transcript, status := jsonGet(payload, "transcript_path", true, &empty); status == 0 {
		l.transcript = trimNewlines(transcript)
	}
	if l.session == "" {
		l.sessionAbsent = true
		l.session = "session-" + strconv.Itoa(inv.Ppid)
	}
	if inv.Event == "stop" && jsonValue(payload, "stop_hook_active") == "true" {
		l.stopHookActive = true
	}
	// Session hygiene happens once at this boundary: the runtime's string is
	// untrusted input; anything outside the safe shape becomes its sha256.
	if !sessionShape.MatchString(l.session) {
		l.session = sha256Text(l.session)
	}

	// Runtime signatures are anchored on the executable, so an intermediate
	// shell does not impersonate the runtime merely because its arguments
	// name this hook. Start at the immediate parent.
	local, status := ops.HookDelegate(l.repo, l.world, "", inv.Ppid)
	if status == 0 && strings.Contains(local, `"delegate":true`) {
		l.intentionalSkip()
	} else if status != 0 && status != 3 {
		l.refuse("local delegate custody evidence was unreadable", 1)
	}

	var identity string
	if inv.Runtime == "fake" {
		// The synthetic fixture runtime is outside the adoptable-host
		// registry; real provider hooks discover foreign hosts through every
		// adoptable host.
		identity, status = ops.FindAncestor(l.world, inv.Ppid, "fake", false)
	} else {
		identity, status = ops.FindAncestor(l.world, inv.Ppid, "", true)
	}
	if status != 0 {
		identity = ""
	}
	identity = trimNewlines(identity)
	if identity != "" {
		runtime := jsonValue(identity, "runtime")
		if runtime == "" {
			l.recordFailure("the runtime identity was unreadable", "runtime-identity")
			identity = ""
		} else if runtime != inv.Runtime {
			l.intentionalSkip()
		}
		if identity != "" {
			l.identityPid = jsonValue(identity, "pid")
			l.identityStarted = jsonValue(identity, "pidStartedAt")
			if !(positiveInteger.MatchString(l.identityPid) && positiveInteger.MatchString(l.identityStarted)) {
				l.recordFailure("the runtime identity was unreadable", "runtime-identity")
				identity, l.identityPid, l.identityStarted = "", "", ""
			}
		}
	} else {
		// Recorded fallback: a hook may run in a harness or runtime wrapper
		// whose authenticated main was announced explicitly. Classification
		// returns that exact announcement; an unannounced process gains
		// nothing here.
		view, status := ops.Classify(l.repo, l.world, inv.Ppid)
		view = trimNewlines(view)
		class := jsonValue(view, "class")
		if status != 0 || class == "" {
			l.recordFailure("the fallback runtime identity could not be classified", "runtime-identity")
		} else if class == "MAIN" {
			runtime := jsonValue(view, "announcement.runtime")
			if runtime == "" {
				l.recordFailure("the fallback runtime identity was unreadable", "runtime-identity")
			} else if runtime != inv.Runtime {
				l.intentionalSkip()
			} else {
				l.identityPid = jsonValue(view, "announcement.pid")
				l.identityStarted = jsonValue(view, "announcement.pidStartedAt")
				if positiveInteger.MatchString(l.identityPid) && positiveInteger.MatchString(l.identityStarted) {
					identity = "recorded-main"
				}
				if identity == "" {
					l.recordFailure("the fallback runtime identity was unreadable", "runtime-identity")
				}
			}
		}
	}
	l.identity = identity
}

func readAllString(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	return string(data), err
}

// classifyHolder reads the checkout holder's classification for the
// identified runtime process: the main, its holdership and the seat's
// presentation coordinates.
func (l *lifecycle) classifyHolder() {
	if l.identityPid == "" {
		return
	}
	pid, _ := strconv.Atoi(l.identityPid)
	view, status := l.ops.Classify(l.repo, l.world, pid)
	view = trimNewlines(view)
	if status != 0 || view == "" {
		l.recordFailure("the checkout holder could not be classified", "checkout-holder")
		return
	}
	l.mainID = jsonValue(view, "mainId")
	l.mainClass = jsonValue(view, "class")
	l.mainHolder = jsonValue(view, "holder")
	l.seatClaimEp = jsonValueDefault(view, "claimEpoch", "0")
	l.seatLineage = jsonValueDefault(view, "announcement.ownerLineage", "")
	if l.seatLineage == "" {
		l.seatLineage = l.mainID
	}
	if machine, err := l.ops.Git("-C", l.repo, "config", "--get", "metasystem.goal.machine"); err == nil {
		l.seatMachine = trimNewlines(machine)
	}
	if l.mainClass == "" || (l.mainHolder != "true" && l.mainHolder != "false") {
		l.recordFailure("the checkout holder classification was unreadable", "checkout-holder")
	}
}

// surface prints one non-start notice as a system message object.
func (l *lifecycle) surface(message string) string {
	rendered := jsonObject("systemMessage=" + message)
	if rendered == "" || jsonValue(rendered, "systemMessage") == "" {
		exitHook(1)
	}
	return rendered
}

func (l *lifecycle) receipt() {
	_, _, status := l.ops.ReceiptCheck(l.harnessRoot)
	if status == 1 {
		_ = writeLine(l.inv.Stdout, l.surface("Metasystem retro due: run metasystem receipt status for details, then skills/retro."))
	} else if status != 0 {
		_ = writeLine(l.inv.Stdout, l.surface("Metasystem receipt check errored; run metasystem receipt status to see why."))
	}
	exitHook(0)
}

func (l *lifecycle) tag() string {
	return "metasystem-main-" + l.inv.Runtime + "-" + l.ops.Slug(l.session)
}

func (l *lifecycle) runtimeSession() (string, bool) {
	if l.sessionAbsent {
		return "", true
	}
	return l.session, false
}

// end retires the session: its unused Stop authorization first, then its
// announcement. The second visibility channel runs before every exit.
func (l *lifecycle) end() {
	inv, ops := l.inv, l.ops
	pending, status := ops.StewardPending(l.repo)
	if status == 0 {
		if pending = trimNewlines(pending); pending != "" {
			_ = writeLine(inv.Stdout, l.surface("Steward incidents pending: "+pending))
		}
	}
	if ops.SessionEnd(l.repo, l.session) != 0 {
		_ = writeLine(inv.Stdout, l.surface("Metasystem could not durably retire this session's unused stop authorization; later stops must treat it as unsafe."))
	}
	if l.identity == "" {
		_ = writeLine(inv.Stdout, l.surface("Metasystem supervision could not identify the immediate "+inv.Runtime+" agent process; arming was refused."))
		exitHook(0)
	}
	session, absent := l.runtimeSession()
	var discard capturedOutput
	ops.Up(UpRequest{
		Runtime: inv.Runtime, MetasystemRoot: l.world, Repo: l.repo, Session: l.session,
		Pid: l.identityPid, StartTime: l.identityStarted, Tag: l.tag(),
		RuntimeSession: session, NoRuntimeSession: absent, Retire: true, CallerPid: inv.Pid,
	}, &discard, &discard)
	exitHook(0)
}

func (l *lifecycle) workFile(name string) string { return filepath.Join(l.workDir, name) }

func (l *lifecycle) cleanupWork() {
	if l.workDir != "" {
		_ = os.RemoveAll(l.workDir)
	}
}

func nowStamp(now time.Time) string { return now.UTC().Format("2006-01-02T15:04:05Z") }

// stopRun is the Stop worker's turn-end state.
type stopRun struct {
	*lifecycle
	generation, attemptSeq        string
	upRC                          int
	upNotice, upFailure           string
	upFailureResult               string
	healthLine, checkinTail       string
	digestMessage                 string
	digestCursor, digestPrefix    string
	receiptRC                     int
	extras                        string
	protocolMessage               string
	protocolCounts                string
	supervisionDir                string
	hookLogFailure                string
	refusalRecord                 string
	verdictFile, factsFile        string
	completionFile, presentation  string
	deliveryReady                 bool
	response                      string
	stopElapsedSec                int64
	armingStderr, armingCapture   string
	healthCapture, digestCapture  string
	receiptCapture, receiptStderr string
	// phases is the Stop worker's own cost trace, one name=milliseconds
	// entry per owner call, written to hooks.log beside the verdict line so
	// a slow Stop names the owner that spent its budget.
	phases []string
}

// evidenceGCInterval bounds how often a Stop runs evidence collection.
const evidenceGCInterval = 15 * time.Minute

// readEvidenceGCStamp reads when a Stop last completed evidence collection.
// An absent or unreadable stamp means collection is due.
func readEvidenceGCStamp(path string) (time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return last, err == nil
}

// timed runs one owner call and records its elapsed milliseconds.
func (s *stopRun) timed(name string, call func()) {
	if s.inv.Monotonic == nil {
		call()
		return
	}
	started := s.inv.Monotonic()
	call()
	s.phases = append(s.phases, name+"="+strconv.FormatInt(int64((s.inv.Monotonic()-started)/time.Millisecond), 10)+"ms")
}

func appendLine(base, line string) string {
	if base == "" {
		return line
	}
	return base + "\n" + line
}

// lastLine is `tail -1` of an output as command substitution reads it.
func lastLine(output string) string {
	output = strings.TrimSuffix(output, "\n")
	return output[strings.LastIndex(output, "\n")+1:]
}

func (l *lifecycle) stop() {
	s := &stopRun{lifecycle: l, protocolCounts: "{}"}
	inv, ops := l.inv, l.ops
	// Only after runtime and delegate context are decided may Stop publish
	// its attempt evidence. The parent treats no other empty or malformed
	// result as a skip, preserving the fail-closed deadline behavior.
	turnKey := sha256Text(l.session + "\n" + l.payload)
	var attempt string
	var status int
	s.timed("attempt", func() { attempt, status = ops.HookAttempt(l.repo, inv.Pid, turnKey) })
	attempt = trimNewlines(attempt)
	if status != 0 || attempt == "" {
		l.evidenceFail = "HEALTH unknown — hook-freshness=unknown (attempt evidence could not be recorded)"
		l.recordFailure("attempt evidence could not be recorded", "turn-evidence")
	} else {
		s.generation = jsonValue(attempt, "generation")
		s.attemptSeq = jsonValue(attempt, "attemptSeq")
		if !(positiveInteger.MatchString(s.generation) && positiveInteger.MatchString(s.attemptSeq)) {
			l.evidenceFail = "HEALTH unknown — hook-freshness=unknown (attempt evidence was unreadable)"
			l.recordFailure("attempt evidence was unreadable", "turn-evidence")
		}
	}
	s.timed("classify", l.classifyHolder)
	s.arm()
	s.timed("protocol", s.check)
	s.decide()
}

// arm drives supervision for this turn and captures every fact the Stop
// presentation reads: arming, health, narrator digest and receipt cadence.
func (s *stopRun) arm() {
	inv, ops := s.inv, s.ops
	session, absent := s.runtimeSession()
	var upOut strings.Builder
	var upErr strings.Builder
	upStarted := time.Duration(0)
	if inv.Monotonic != nil {
		upStarted = inv.Monotonic()
	}
	if s.identityPid != "" {
		s.upRC = ops.Up(UpRequest{
			Runtime: inv.Runtime, MetasystemRoot: s.world, Repo: s.repo, Session: s.session,
			Pid: s.identityPid, StartTime: s.identityStarted, Tag: s.tag(),
			RuntimeSession: session, NoRuntimeSession: absent, CallerPid: inv.Pid,
		}, &upOut, &upErr)
	} else {
		// A Stop with no session identity still drives the restricted verify
		// and recovery path. It gains no announcement or lease authority.
		s.upRC = ops.Up(UpRequest{
			Runtime: inv.Runtime, MetasystemRoot: s.world, Repo: s.repo, RecoverOnly: true, IfDown: true,
			RuntimeSession: session, NoRuntimeSession: absent, CallerPid: inv.Pid,
		}, &upOut, &upErr)
	}
	if inv.Monotonic != nil {
		s.phases = append(s.phases, "up="+strconv.FormatInt(int64((inv.Monotonic()-upStarted)/time.Millisecond), 10)+"ms")
	}
	s.armingStderr = s.workFile("arming.stderr")
	_ = os.WriteFile(s.armingStderr, []byte(upErr.String()), 0o600)
	upOutput := trimNewlines(upOut.String())
	aggregate := lastLine(upOutput)
	if strings.Contains(aggregate, " re-armed=") {
		s.upNotice = "Metasystem re-armed the rebuilt engine: " + aggregate
	} else if strings.HasPrefix(aggregate, "up outcome=stopped") {
		component := ""
		for _, line := range strings.Split(upOutput, "\n") {
			if match := stoppedComponent.FindStringSubmatch(line); match != nil {
				component = match[1]
			}
		}
		remedy := ""
		if match := stoppedRemedy.FindStringSubmatch(aggregate); match != nil {
			remedy = match[1]
		}
		if strings.HasPrefix(component, "stop incomplete ") || strings.HasPrefix(component, "stop unfinished ") {
			s.upNotice = "Metasystem " + component + "; run: " + remedy + "."
		} else {
			s.upNotice = "Metasystem is stopped for this checkout; start again with " + remedy + "."
		}
	}
	if s.upRC != 0 {
		var components []string
		for _, line := range strings.Split(upOutput, "\n") {
			if strings.HasPrefix(line, "component=") {
				components = append(components, line)
			}
		}
		result := strings.Join(components, "\n")
		if result != "" {
			result += "\n"
		}
		result += aggregate
		if diagnostic := trimNewlines(upErr.String()); diagnostic != "" {
			result = appendLine(result, diagnostic)
		}
		s.upFailureResult = result
		s.upFailure = "Metasystem supervision arming failed:\n" + result
		s.recordFailure("supervision arming failed", "supervision-arming")
	}
	s.armingCapture = s.workFile("arming.txt")
	if os.WriteFile(s.armingCapture, []byte(upOutput), 0o600) != nil {
		s.recordFailure("the supervision arming result could not be captured", "supervision-arming")
	}

	s.healthCapture = s.workFile("health.json")
	var health string
	var healthRC int
	s.timed("health", func() { health, healthRC = ops.HealthPreview(s.repo, s.world) })
	_ = os.WriteFile(s.healthCapture, []byte(health), 0o600)
	s.healthLine = jsonValue(health, "line")
	if healthRC > 2 || s.healthLine == "" {
		s.healthLine = "HEALTH unknown — hook-freshness=unknown (the health engine returned no verdict)"
		s.recordFailure("the health engine returned no verdict", "health")
	}

	var digest string
	var digestRC int
	s.timed("digest", func() { digest, digestRC = ops.DigestPending(s.repo) })
	digest = trimNewlines(digest)
	if digestRC == 0 {
		message, messageRC := jsonValueStatus(digest, "message")
		cursor, cursorRC := jsonValueStatus(digest, "cursor")
		prefix, prefixRC := jsonValueStatus(digest, "prefixSha256")
		s.digestMessage, s.digestCursor, s.digestPrefix = message, cursor, prefix
		if messageRC != 0 || cursorRC != 0 || prefixRC != 0 {
			s.digestMessage = "NARRATOR DIGEST unavailable: the digest state was unreadable"
			s.recordFailure("the narrator digest state was unreadable", "narrator")
		}
	} else {
		s.digestMessage = "NARRATOR DIGEST unavailable: " + strings.ReplaceAll(digest, "\n", " ")
		s.recordFailure("the narrator digest could not be read", "narrator")
	}
	s.checkinTail = s.healthLine
	if s.upNotice != "" {
		s.checkinTail = s.upNotice + "\n" + s.checkinTail
	}
	if s.digestMessage != "" {
		s.checkinTail += "\n" + s.digestMessage
	}
	s.digestCapture = s.workFile("digest.txt")
	if os.WriteFile(s.digestCapture, []byte(s.digestMessage), 0o600) != nil {
		s.recordFailure("the narrator digest could not be captured", "narrator")
	}
	s.receiptCapture = s.workFile("receipt.txt")
	s.receiptStderr = s.workFile("receipt.stderr")
	var receiptOut, receiptErr string
	var receiptRC int
	s.timed("receipt", func() { receiptOut, receiptErr, receiptRC = ops.ReceiptCheck(s.harnessRoot) })
	s.receiptRC = receiptRC
	_ = os.WriteFile(s.receiptCapture, []byte(receiptOut), 0o600)
	_ = os.WriteFile(s.receiptStderr, []byte(receiptErr), 0o600)
}

var (
	stoppedComponent = regexp.MustCompile(`^component=stopped outcome=standing detail="(.*)"$`)
	stoppedRemedy    = regexp.MustCompile(`^up outcome=stopped remedy="(.*)"$`)
)

// check reads the holder protocol state and opens the hook evidence trail.
func (s *stopRun) check() {
	ops := s.ops
	s.refusalRecord = filepath.Join(s.repo, "artifacts", "agents", "supervision", "stop-refusals", ops.Slug(s.session)+".json")
	if s.mainID != "" {
		growth, status := ops.ProtocolGrowth(s.repo, s.mainID)
		growth = trimNewlines(growth)
		if status != 0 {
			s.recordFailure("the holder protocol state could not be read", "holder-protocol")
		} else if growth != "" {
			message, messageRC := jsonValueStatus(growth, "message")
			counts, countsRC := jsonValueStatus(growth, "counts")
			s.protocolMessage, s.protocolCounts = message, counts
			if messageRC != 0 || countsRC != 0 {
				s.recordFailure("the holder protocol state was unreadable", "holder-protocol")
			}
		}
	}
	// The evidence trail sits beside the rest of the supervision state. One
	// hook-log line per infrastructure condition, in every outcome.
	s.supervisionDir = filepath.Join(s.repo, "artifacts", "agents", "supervision")
	_ = os.MkdirAll(s.supervisionDir, 0o755)
}

func (s *stopRun) appendHookLog(line string) error {
	file, err := os.OpenFile(filepath.Join(s.supervisionDir, "hooks.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = file.WriteString(line)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

func (s *stopRun) appendStopCondition(class, code, component, outcome string) {
	generation := s.generation
	if generation == "" {
		generation = "-"
	}
	deadlineEnd := s.stopStarted + 60
	line := "stop-condition " + class + " " + code + " " + component + " " + generation + " " + strconv.FormatInt(deadlineEnd, 10) + " " + outcome + "\n"
	if s.appendHookLog(line) != nil {
		s.hookLogFailure = "the infrastructure stop condition could not be appended to the hook log"
	}
}

func (s *stopRun) appendConditions() {
	for _, condition := range s.conditions {
		code, component, _ := strings.Cut(condition, "|")
		s.appendStopCondition("infrastructure", code, component, "degraded-allow")
	}
}

// decide is the turn end: the advisor allowance, or the one structured
// verdict, presented and emitted once.
func (s *stopRun) decide() {
	inv, ops := s.inv, s.ops
	// "Advisor" is a positive finding: an announced main of this checkout
	// that is not the one holding it. An unidentified caller is not an
	// advisor.
	if s.mainClass == "MAIN" && s.mainHolder != "true" {
		message := "OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run metasystem session isolate."
		if s.upFailure != "" {
			message += "\n" + s.upFailure
		}
		if s.evidenceFail != "" {
			message += "\n" + s.evidenceFail
		}
		if s.protocolMessage != "" {
			message += "\n" + s.protocolMessage
		}
		s.appendConditions()
		if s.hookLogFailure != "" {
			message += "\n" + s.hookLogFailure
		}
		s.extras = message
		s.present("false", true)
		s.emit(s.response)
		s.advanceProtocol()
		exitHook(0)
	}
	if s.identityPid != "" {
		pid, _ := strconv.Atoi(s.identityPid)
		renewRC := 0
		s.timed("lease", func() { renewRC = ops.RenewLease(s.repo, pid) })
		if renewRC != 0 {
			s.recordFailure("the checkout holder lease could not be renewed", "holder-lease")
		}
	}

	// The watchdog path calls the verdict like every other path: the
	// report's text stays hook-side, its digest rides to the verdict, and
	// the verdict's surfaceWatchdog answer decides exactly-once surfacing.
	var watchdogText string
	var watchdogRC int
	s.timed("watchdog", func() { watchdogText, watchdogRC = ops.WatchdogReport(s.repo) })
	watchdogText = trimNewlines(watchdogText)
	if watchdogRC != 0 {
		s.recordFailure("the supervision watchdog state could not be read", "watchdog")
	}
	watchdogDigest := ""
	if watchdogText != "" {
		watchdogDigest = sha256Text(watchdogText)
	}

	// Leave evidence that this ran: without it a hook that fired and found
	// nothing cannot be told from one that never fired.
	// Evidence collection keeps a 90-minute grace, so it runs at most once
	// per evidenceGCInterval rather than on every turn end: on seat m1e it
	// walked 132,000 files for 4-5 s of every Stop. A skipped turn still
	// leaves its line.
	stamp := filepath.Join(s.supervisionDir, "evidence-gc.last")
	now := inv.Now().UTC()
	if last, ok := readEvidenceGCStamp(stamp); ok && !now.Before(last) && now.Sub(last) < evidenceGCInterval {
		_ = s.appendHookLog(nowStamp(now) + " evidence-gc skipped: last ran " + last.Format(time.RFC3339) + "; next after " + last.Add(evidenceGCInterval).Format(time.RFC3339) + "\n")
	} else {
		gcRC := 1
		if log, err := os.OpenFile(filepath.Join(s.supervisionDir, "hooks.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			s.timed("evidence-gc", func() { gcRC = ops.EvidenceGC(s.world, log) })
			_ = log.Close()
		}
		if gcRC != 0 {
			s.recordFailure("the hook evidence state could not be maintained", "hook-evidence")
		} else {
			_ = os.WriteFile(stamp, []byte(now.Format(time.RFC3339)+"\n"), 0o644)
		}
	}

	// One structured decision: the verdict owns open work, the goal clause,
	// precedence, block-once state and the all-clear. A nonzero status is
	// an I/O failure, and this hook answers it with its fixed degraded
	// allowance, never an all-clear it cannot vouch for.
	s.verdictFile = s.workFile("verdict.json")
	s.factsFile = s.workFile("facts.json")
	s.completionFile = s.workFile("completion.json")
	verdictReadable := false
	degradedLine := ""
	factsFailure, completionFailure := "", ""
	var shouldBlock, display, surfaceWatchdog, brainStatusDue, verdictClass, countSpent, verdict string
	var stdout, stderr string
	var status int
	s.timed("verdict", func() {
		stdout, stderr, status = ops.TurnVerdict(TurnVerdictRequest{
			Root: s.repo, Session: s.session, SessionAbsent: s.sessionAbsent, Watchdog: watchdogDigest,
			MainID: s.mainID, StopHookActive: s.stopHookActive, Transcript: s.transcript, Runtime: inv.Runtime,
			FactsFile: s.factsFile, CompletionFile: s.completionFile,
		})
	})
	if status == 0 {
		verdict = trimNewlines(stdout)
		_ = os.WriteFile(s.verdictFile, []byte(verdict+"\n"), 0o600)
		if !nonEmptyFile(s.factsFile) {
			factsFailure = lastLine(stderr)
			s.recordFailure("the frozen judgment facts were unavailable", "judgment-facts")
		}
		if !nonEmptyFile(s.completionFile) {
			completionFailure = lastLine(stderr)
			s.recordFailure("the completion observation was unavailable", "completion-observation")
		}
		var rc1, rc2, rc3, rc4, rc5 int
		var idleRefusal string
		shouldBlock, rc1 = jsonValueStatus(verdict, "shouldBlock")
		display, rc2 = jsonValueStatus(verdict, "display")
		surfaceWatchdog, rc3 = jsonValueStatus(verdict, "surfaceWatchdog")
		idleRefusal, rc4 = jsonValueStatus(verdict, "idleRefusal")
		verdictClass = "seat-actionable"
		if value, rc := jsonValueStatus(verdict, "class"); rc == 0 {
			verdictClass = value
		}
		countSpent = "true"
		if value, rc := jsonValueStatus(verdict, "countSpent"); rc == 0 {
			countSpent = value
		}
		brainStatusDue, rc5 = jsonValueStatus(verdict, "brainStatusDue")
		boolean := func(value string) bool { return value == "true" || value == "false" }
		if rc1 != 0 || rc2 != 0 || rc3 != 0 || rc4 != 0 || rc5 != 0 || display == "" ||
			!boolean(shouldBlock) || !boolean(surfaceWatchdog) || !boolean(idleRefusal) ||
			!boolean(brainStatusDue) || !boolean(countSpent) ||
			(verdictClass != "infrastructure" && verdictClass != "seat-actionable" && verdictClass != "idle-with-backlog") {
			degradedLine = "the turn verdict was unreadable"
		} else {
			verdictReadable = true
		}
	} else {
		degradedLine = lastLine(stderr)
	}

	s.appendConditions()
	if verdictReadable && verdictClass == "infrastructure" {
		cause := jsonValue(verdict, "causeCode")
		if cause == "" {
			cause = "turn-verdict-unavailable"
		}
		component := jsonValue(verdict, "component")
		if component == "" {
			component = "verdict-state"
		}
		s.appendStopCondition("infrastructure", cause, component, "degraded-allow")
		// The verdict's own state could not be read or written: the notice
		// says so in fixed words, names the owner and carries the detail.
		display = "turn-verdict degraded: stopping is allowed on degraded infrastructure; the steward owns repair. Cause: " + cause + ". Component: " + component + ".\n" + display
	}
	if verdictReadable && verdictClass == "idle-with-backlog" && shouldBlock == "true" && countSpent == "false" {
		s.appendStopCondition("idle-with-backlog", "idle-refusal-count-not-spent", "verdict-state", "refused-uncounted")
	}
	if !verdictReadable {
		s.appendStopCondition("infrastructure", "turn-verdict-unavailable", "verdict-state", "degraded-allow")
	}

	if verdictReadable {
		_ = s.appendHookLog(nowStamp(inv.Now()) + " stop verdict block=" + shouldBlock + "\n")
		if len(s.phases) > 0 {
			_ = s.appendHookLog(nowStamp(inv.Now()) + " stop phases " + strings.Join(s.phases, " ") + "\n")
		}
		brainPostFailure := ""
		if brainStatusDue == "true" {
			statusPath := filepath.Join(s.repo, "artifacts", "agents", "brain-status.json")
			before := fileJSONValueDefault(statusPath, "lastPostedAt")
			var discard, postErr strings.Builder
			postRC := ops.ChannelStatusPost(s.repo, &discard, &postErr)
			after := fileJSONValueDefault(statusPath, "lastPostedAt")
			if postRC != 0 {
				reason := lastLine(postErr.String())
				if reason == "" {
					reason = "channel status exited " + strconv.Itoa(postRC)
				}
				brainPostFailure = "the brain's status line was not published: " + reason
			} else if after == "" || after == before {
				brainPostFailure = "the brain's status line was not published: no channel provider is configured"
			}
		}
		s.extras = ""
		for _, line := range []string{s.upNotice, s.evidenceFail, factsFailure, completionFailure} {
			if line != "" {
				s.extras = appendLine(s.extras, line)
			}
		}
		if surfaceWatchdog == "true" && watchdogText != "" {
			s.extras = appendLine(s.extras, watchdogText)
		}
		for _, line := range []string{s.protocolMessage, brainPostFailure, s.hookLogFailure} {
			if line != "" {
				s.extras = appendLine(s.extras, line)
			}
		}
		if s.stopFailure != "" {
			// This records the stable refusal cause and occurrence; the
			// shared presenter below owns the public response.
			s.composeFailedStop(s.stopFailure, display, shouldBlock == "true", s.extras)
		}
		s.present(shouldBlock, false)
	} else {
		_ = s.appendHookLog(nowStamp(inv.Now()) + " stop verdict unavailable\n")
		if degradedLine == "" {
			degradedLine = "no diagnostic"
		}
		message := "turn-verdict unavailable: stopping is allowed on degraded infrastructure; the steward owns repair. " + degradedLine
		for _, line := range []string{s.upFailure, s.evidenceFail, s.protocolMessage, s.hookLogFailure} {
			if line != "" {
				message += "\n" + line
			}
		}
		s.surface(message + "\n" + s.checkinTail)
		s.fallback(false)
	}
	s.emit(s.response)
	s.advanceProtocol()
	exitHook(0)
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// fileJSONValueDefault is `json get --file F --field K --default ”`.
func fileJSONValueDefault(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return jsonValueDefault(string(data), field, "")
}

func (s *stopRun) advanceProtocol() {
	if !s.deliveryReady || s.mainID == "" || s.identityPid == "" || s.protocolMessage == "" {
		return
	}
	pid, _ := strconv.Atoi(s.identityPid)
	s.ops.ProtocolAdvance(s.repo, s.mainID, pid, s.protocolCounts)
}

func (s *stopRun) fallback(retainedBlock bool) {
	if retainedBlock {
		s.response = mustDegradedStopForm("blocked", "bare")
	} else {
		s.response = mustDegradedStopForm("allowed", "bare")
	}
	s.deliveryReady = false
}

// present composes, publishes and maps the Stop presentation; any failure
// keeps the retained verdict's control through the bare degraded form.
func (s *stopRun) present(retainedBlock string, advisor bool) {
	s.deliveryReady = false
	block := retainedBlock == "true"
	if !advisor && !nonEmptyFile(s.verdictFile) {
		s.fallback(block)
		return
	}
	token, err := s.ops.TokenHex(16)
	if err != nil || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(token) {
		s.fallback(block)
		return
	}
	input := s.workFile("presentation-input.json")
	s.presentation = s.workFile("presentation.json")
	mapped := s.workFile("provider-output.json")
	failures := s.workFile("failures.txt")
	notices := s.workFile("notices.txt")
	_ = os.WriteFile(failures, []byte(s.stopFailure), 0o600)
	_ = os.WriteFile(notices, []byte(s.extras), 0o600)
	request := StopInputRequest{
		Root: s.repo, Runtime: s.inv.Runtime, Session: s.session, Attempt: token, MainID: s.mainID,
		Machine: s.seatMachine, Lineage: s.seatLineage, ClaimEpoch: s.seatClaimEp, Advisor: advisor,
		HealthFile: s.healthCapture, DigestFile: s.digestCapture, DigestCursorPrefix: s.digestPrefix,
		ReceiptFile: s.receiptCapture, ReceiptStderrFile: s.receiptStderr, ReceiptExit: s.receiptRC,
		ArmingFile: s.armingCapture, ArmingStderrFile: s.armingStderr, ArmingExit: s.upRC,
		NoticeFile: notices, FailureFile: failures, OutputFile: input,
	}
	if !advisor {
		request.VerdictFile, request.FactsFile, request.CompletionFile = s.verdictFile, s.factsFile, s.completionFile
	}
	log := &hookLogWriter{s}
	if s.ops.StopInput(request, log) != 0 {
		s.fallback(block)
		return
	}
	if s.ops.StopPresent(s.repo, input, s.presentation, log) != 0 {
		s.fallback(block)
		return
	}
	if s.ops.StopOutput(s.inv.Runtime, s.presentation, mapped, log) != 0 {
		s.fallback(block)
		return
	}
	data, _ := os.ReadFile(mapped)
	s.response = trimNewlines(string(data))
	s.deliveryReady = true
}

// hookLogWriter appends an owner's diagnostics to the hook log.
type hookLogWriter struct{ s *stopRun }

func (w *hookLogWriter) Write(data []byte) (int, error) {
	_ = w.s.appendHookLog(string(data))
	return len(data), nil
}

func (s *stopRun) completeAttempt(request HookCompletion) int {
	request.Repo, request.Generation, request.Attempt = s.repo, s.generation, s.attemptSeq
	request.ElapsedSec, request.HasElapsed = s.stopElapsedSec, true
	status := s.ops.HookComplete(request)
	if status == 2 {
		request.HasElapsed = false
		status = s.ops.HookComplete(request)
	}
	return status
}

// emit publishes the one Stop response and records its delivery.
func (s *stopRun) emit(response string) {
	inv, ops := s.inv, s.ops
	s.stopElapsedSec = inv.Now().Unix() - s.stopStarted
	if s.stopElapsedSec < 0 {
		s.stopElapsedSec = 0
	}
	decision := jsonValue(response, "decision")
	if decision == "" {
		decision = "allow"
	}
	_ = os.MkdirAll(s.supervisionDir, 0o755)
	_ = s.appendHookLog(nowStamp(inv.Now()) + " stop response decision=" + decision + " elapsed=" + strconv.FormatInt(s.stopElapsedSec, 10) + "s\n")
	responseFile, err := os.CreateTemp(inv.TempDir, "metasystem-supervision-response.")
	if err != nil {
		_ = writeLine(inv.Stdout, response)
		s.completeAttempt(HookCompletion{Result: "ERROR", Outcome: "PAYLOAD_STAGE_FAILED"})
		return
	}
	name := responseFile.Name()
	defer os.Remove(name)
	_, writeErr := responseFile.WriteString(response + "\n")
	closeErr := responseFile.Close()
	if writeErr != nil || closeErr != nil {
		_ = writeLine(inv.Stdout, response)
		s.completeAttempt(HookCompletion{Result: "ERROR", Outcome: "PAYLOAD_STAGE_FAILED"})
		return
	}
	if writeLine(inv.Stdout, response) != nil {
		s.completeAttempt(HookCompletion{Result: "ERROR", Outcome: "EMISSION_FAILED", HealthLine: s.healthLine, PayloadFile: name})
		return
	}
	if s.deliveryReady && s.digestMessage != "" && digits.MatchString(s.digestCursor) && sha256Hex.MatchString(s.digestPrefix) {
		if ops.DigestAdvance(s.repo, s.digestCursor, s.digestPrefix) != 0 {
			_ = writeLine(inv.Stderr, "supervision hook: emitted the narrator digest but could not advance its check-in cursor")
		}
	}
	if !s.deliveryReady {
		// Presentation failure is component evidence; the retained verdict
		// still decided whether this Stop blocked.
		s.completeAttempt(HookCompletion{Result: "ERROR", Outcome: "PRESENTATION_UNAVAILABLE", PayloadFile: name})
		return
	}
	presentation, _ := os.ReadFile(s.presentation)
	if s.completeAttempt(HookCompletion{
		Result: "OK", Outcome: "EMITTED", HealthLine: s.healthLine, PayloadFile: name, Installation: s.repo,
		ReportID: jsonValue(string(presentation), "report.id"), ReportAlias: jsonValue(string(presentation), "report.alias"),
		ReportPath: jsonValue(string(presentation), "report.path"), ReportSHA256: jsonValue(string(presentation), "report.sha256"),
	}) != 0 {
		_ = writeLine(inv.Stderr, "supervision hook: emitted the health line but could not record completion")
	}
}

// stopBlock renders an unrecorded refusal; a malformed rendering ends the
// worker, which its deadline parent answers with the fixed allowance.
func (s *stopRun) stopBlock(systemMessage, reason string) string {
	rendered, status := s.ops.StopBlock(StopBlockRequest{SystemMessage: systemMessage, Detail: reason})
	rendered = trimNewlines(rendered)
	if status != 0 || jsonValue(rendered, "decision") != "block" || jsonValue(rendered, "reason") == "" {
		exitHook(1)
	}
	return rendered
}

// composeFailedStop records the infrastructure refusal and carries its
// repeated-failure notice into the extras the presenter shows.
func (s *stopRun) composeFailedStop(cause, verdictDisplay string, verdictShouldBlock bool, verdictExtras string) {
	remedy := "A human or steward must restore supervision outside this seat, then retry."
	if cause == "supervision arming failed" && s.upFailureResult != "" {
		remedy = s.upFailureResult
	}
	failureDetail := verdictDisplay + "\n\nInfrastructure condition, not a refusal (the steward owns repair): " + cause
	refusalSystemMessage := verdictExtras
	if refusalSystemMessage != "" {
		refusalSystemMessage += "\n"
	}
	refusalSystemMessage += s.checkinTail
	reportTail := refusalSystemMessage
	if s.upFailureResult != "" {
		reportTail = appendLine(reportTail, s.upFailureResult)
	}
	surfaceWith := func(first string) {
		s.extras = appendLine(s.extras, first)
		if verdictShouldBlock {
			message := first
			if verdictExtras != "" {
				message += "\n" + verdictExtras
			}
			if s.checkinTail != "" {
				message += "\n" + s.checkinTail
			}
			s.response = s.stopBlock(message, verdictDisplay)
		} else {
			message := first + "\n" + verdictDisplay
			if verdictExtras != "" {
				message += "\n" + verdictExtras
			}
			if s.checkinTail != "" {
				message += "\n" + s.checkinTail
			}
			s.response = s.surface(message)
		}
	}
	response, status := s.ops.StopBlock(StopBlockRequest{
		Class: "infrastructure", SystemMessage: refusalSystemMessage, RefusalRecord: s.refusalRecord,
		Session: s.session, Cause: cause, Remedy: remedy, ArmingResult: s.upFailureResult, Detail: failureDetail,
	})
	response = trimNewlines(response)
	if status != 0 || response == "" {
		surfaceWith("Metasystem stop-refusal record failure: the record could not be read or atomically updated. Stopping is allowed so record failure cannot recreate the refusal loop.\nCause: " + cause + "\nRemedy: " + remedy)
		return
	}
	s.response = response
	if jsonValue(response, "decision") == "block" {
		return
	}
	message, messageRC := jsonValueStatus(response, "systemMessage")
	if messageRC != 0 || message == "" {
		surfaceWith("Metasystem stop-refusal record failure: the record returned an unreadable repeated-failure response. Stopping is allowed so record failure cannot recreate the refusal loop.\nCause: " + cause + "\nRemedy: " + remedy)
		return
	}
	// The refusal appends the supplied system message to its repeated-failure
	// notice. Split that known suffix so the repeated notice can precede the
	// ordinary verdict envelope without duplicating extras or health.
	repeated := message
	if reportTail != "" {
		suffix := "\n" + reportTail
		if strings.HasSuffix(message, suffix) {
			repeated = strings.TrimSuffix(message, suffix)
		}
	}
	if repeated != "" {
		s.extras = appendLine(s.extras, repeated)
	}
	if verdictShouldBlock {
		blocking := repeated
		if verdictExtras != "" {
			blocking += "\n" + verdictExtras
		}
		if s.checkinTail != "" {
			blocking += "\n" + s.checkinTail
		}
		s.response = s.stopBlock(blocking, verdictDisplay)
	} else {
		allowed := repeated + "\n" + verdictDisplay
		if verdictExtras != "" {
			allowed += "\n" + verdictExtras
		}
		if s.checkinTail != "" {
			allowed += "\n" + s.checkinTail
		}
		s.response = s.surface(allowed)
	}
	_ = failureDetail
}
