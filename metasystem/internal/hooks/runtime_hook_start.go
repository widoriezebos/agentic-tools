package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// SessionStart owns its outcome before it asks the filesystem, Git or an
// owner for anything. Every termination goes through finish, which prints one
// fixed notice or one validated intentional response, so an ordinary runtime
// failure can always explain itself.

const startLastResort = `{"systemMessage":"Metasystem SessionStart could not produce its response: role context delivery is unconfirmed; a declared brain may be uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}`

const startBookkeepingNotice = "Metasystem SessionStart published its response but could not finish delivery bookkeeping. Repair supervision from the owning installation, then start a new session; context may repeat."

func startNoticeTemplate(cause, remedy string) string {
	return `{"systemMessage":"Metasystem SessionStart could not ` + cause + `: this session received no role context; if this checkout is a declared brain it is uninstructed. ` + remedy + ` Then start a new session."}`
}

// startNotice is one fixed SessionStart outcome and its exit status.
type startNotice struct {
	text   string
	status int
}

// StartOutcomeNotices is the SessionStart notice catalog: every fixed outcome
// the start boundary can publish. "interrupted" exits with the signal's
// status. The hook-start audit joins this catalog to its executed cases.
var StartOutcomeNotices = map[string]startNotice{
	"engine-missing":          {`{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh, then start a new session"}`, 0},
	"engine-skew":             {`{"systemMessage":"Metasystem engine does not answer path state-root: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}`, 0},
	"installation-directory":  {startNoticeTemplate("locate its installation directory", "Restore access to the installed hook and its parent directories."), 0},
	"checkout-identification": {startNoticeTemplate("identify the checkout and its primary installation", "Restore Git and access to the checkout and its primary metasystem installation."), 0},
	"resolved-directory":      {startNoticeTemplate("open the resolved installation directory", "Restore access to the installation directory returned by the engine."), 0},
	"installation-validation": {startNoticeTemplate("validate the metasystem installation", "Restore a complete, readable metasystem installation and rebuild bin/metasystem with scripts/agents/go-build.sh."), 0},
	"payload-storage":         {startNoticeTemplate("stage its input", "Restore writable temporary storage and free space."), 0},
	"boot-storage":            {startNoticeTemplate("stage brain context", "Restore writable temporary storage and free space."), 0},
	"start-preparation":       {startNoticeTemplate("prepare the session identity and context", "Restore the installed shell tools and rebuild bin/metasystem with scripts/agents/go-build.sh."), 0},
	"response-rendering":      {startLastResort, 0},
	"unexpected-termination":  {startLastResort, 0},
	"payload-read":            {startNoticeTemplate("read its session input", "Repair the SessionStart hook input and rebuild bin/metasystem with scripts/agents/go-build.sh."), 0},
	"pending-read":            {startNoticeTemplate("read pending steward incidents", "Restore access to the steward incident records and repair unreadable records."), 0},
	"holder-read":             {startNoticeTemplate("read checkout holder identity", "Restore readable checkout custody records and restart the owning runtime."), 0},
	"invocation-invalid":      {startNoticeTemplate("accept the runtime invocation", "Repair the installed hook registration."), 2},
	"runtime-unregistered":    {startNoticeTemplate("find the runtime in its registry", "Repair the installed hook registration."), 2},
	"runtime-registry":        {startNoticeTemplate("read the runtime registry", "Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations."), 0},
	"context-contract":        {startNoticeTemplate("read the runtime context contract", "Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations."), 0},
	"custody-unreadable":      {startNoticeTemplate("authenticate delegate custody", "Restore the recorded delegate custody and restart through its launcher."), 1},
	"process-identity":        {startNoticeTemplate("identify the owning runtime process", "Restart through the installed runtime launcher."), 0},
	"brain-boot":              {`{"systemMessage":"Metasystem brain boot failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}`, 0},
	"brain-timeout":           {`{"systemMessage":"Metasystem brain boot failed (timeout): this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}`, 0},
	"arming":                  {`{"systemMessage":"Metasystem supervision arming failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair supervision from the owning installation and run metasystem up there. Then start a new session."}`, 0},
	"wait-recovery":           {startNoticeTemplate("read durable wait recovery rows", "Repair supervision from the owning installation and run metasystem up there."), 0},
	"temporary-cleanup":       {startNoticeTemplate("remove its temporary files", "Restore temporary-directory access and remove leftover metasystem hook temporary files."), 0},
	"interrupted":             {`{"systemMessage":"Metasystem SessionStart could not finish because it was interrupted: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore the runtime session. Then start a new session."}`, 143},
}

// StartIntentionalOutcomes are the validated non-notice SessionStart
// outcomes: two skips and four published responses.
var StartIntentionalOutcomes = []string{
	"authenticated-delegate", "foreign-runtime", "context-ready", "screen-context-ready", "notices-ready", "healthy-no-context",
}

// StartEngineMissingNotice is the SessionStart notice the plumbing stub
// prints when no engine can run the hook.
func StartEngineMissingNotice() string { return StartOutcomeNotices["engine-missing"].text }

// bootstrapPayloadPrefix names the payload a bootstrap restart stages.
const bootstrapPayloadPrefix = "metasystem-hook-bootstrap."

const screenOnlyNotice = "this runtime has no session-context channel; the packet reached the screen only"

var (
	positiveInteger   = regexp.MustCompile(`^[1-9][0-9]*$`)
	digits            = regexp.MustCompile(`^[0-9]+$`)
	sha256Hex         = regexp.MustCompile(`^[0-9a-f]{64}$`)
	sessionShape      = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	contextFieldShape = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)+$`)
	contextEventShape = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,63}$`)
	delegateCustody   = regexp.MustCompile(`^\{"delegate":true,"jobId":"[a-z0-9][a-z0-9-]*","matchedPid":[1-9][0-9]*,"comparisonMode":"(darwin-microseconds|linux-ticks-boot-id|legacy-seconds)"\}$`)
)

type startRun struct {
	inv Invocation
	ops Ops

	signal    atomic.Int32
	finishing bool
	published bool

	armingStarted        bool
	armingRearmed        bool
	armingStatus         int
	waitRecoveryStatus   int
	deferredIdentityRead bool

	notices           string
	contextKind       string
	contextPayload    string
	contextField      string
	contextEvent      string
	contextBytes      int
	contextShapeReady bool
	preparedContext   string

	brainFailure        string
	brainDelivery       bool
	brainDeclarationSHA string
	brainDigestEmitted  string
	brainDigestCursor   string
	brainDigestPrefix   string
	brainDeadlineMS     int
	brainWait           time.Duration

	harnessRoot     string
	world           string
	engine          string
	repo            string
	session         string
	sessionAbsent   bool
	startSource     string
	payload         []byte
	identityPid     string
	identityStarted string
}

func runStart(inv Invocation, ops Ops) (status int) {
	s := &startRun{inv: inv, ops: ops, contextKind: "none", contextBytes: 2048}
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, ok := recovered.(hookExit); ok {
				panic(recovered)
			}
			// An unexpected failure is an unexpected termination: the one
			// owner still answers.
			s.finish("notice", "unexpected-termination")
		}
	}()
	s.main()
	s.finish("notice", "unexpected-termination")
	return 0
}

func signalStatus(received os.Signal) int {
	switch received {
	case syscall.SIGHUP:
		return 129
	case syscall.SIGINT:
		return 130
	default:
		return 143
	}
}

// pollSignals takes every delivered signal; the first one's status decides.
func (s *startRun) pollSignals() {
	if s.inv.Signals == nil {
		return
	}
	for {
		select {
		case received, ok := <-s.inv.Signals:
			if !ok {
				return
			}
			s.signal.CompareAndSwap(0, int32(signalStatus(received)))
		default:
			return
		}
	}
}

// checkpoint is where a pending signal ends the start: between owner calls,
// as the shell's traps ran after the command they interrupted.
func (s *startRun) checkpoint() {
	s.pollSignals()
	if s.signal.Load() == 0 || s.finishing {
		return
	}
	s.finish("notice", "interrupted")
}

func (s *startRun) collectNotice(line string) {
	if s.notices != "" {
		s.notices += "\n" + line
	} else {
		s.notices = line
	}
}

// startJSONEscape encodes text byte-wise for a JSON string: quote, backslash
// and control characters escaped, every other byte kept.
func startJSONEscape(input string) string {
	var out strings.Builder
	for index := 0; index < len(input); index++ {
		character := input[index]
		switch character {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if character < 32 {
				fmt.Fprintf(&out, `\u%04x`, character)
			} else {
				out.WriteByte(character)
			}
		}
	}
	return out.String()
}

// appendNoticeText appends fixed JSON-safe text to a systemMessage object.
func appendNoticeText(object, text string) string {
	return strings.TrimSuffix(object, `"}`) + `\n` + text + `"}`
}

func (s *startRun) buildResponse(reason string) (string, bool) {
	switch reason {
	case "context-ready":
		if !(s.preparedContext != "" && s.contextKind == "channel" && s.contextShapeReady && s.contextField != "" &&
			s.contextEvent != "" && s.contextPayload != "") {
			return "", false
		}
	case "screen-context-ready":
		if !(s.preparedContext == "" && s.contextKind == "screen" && s.notices != "") {
			return "", false
		}
	case "notices-ready":
		if !(s.preparedContext == "" && s.contextKind == "none" && s.notices != "") {
			return "", false
		}
	case "healthy-no-context":
		if !(s.preparedContext == "" && s.contextKind == "none" && s.notices == "") {
			return "", false
		}
		return "{}", true
	default:
		return "", false
	}
	response := s.preparedContext
	if s.notices != "" {
		escaped := startJSONEscape(s.notices)
		if s.preparedContext != "" {
			response = `{"systemMessage":"` + escaped + `",` + strings.TrimPrefix(s.preparedContext, "{")
		} else {
			response = `{"systemMessage":"` + escaped + `"}`
		}
	}
	return response, response != ""
}

// emergency is the last resort when the outcome owner itself cannot finish.
func (s *startRun) emergency() {
	if s.published {
		_ = writeLine(s.inv.Stderr, startBookkeepingNotice)
		exitHook(1)
	}
	if writeLine(s.inv.Stdout, startLastResort) == nil {
		exitHook(0)
	}
	_ = writeLine(s.inv.Stderr, startLastResort)
	exitHook(74)
}

// finish is the one owner of every SessionStart termination.
func (s *startRun) finish(family, key string) {
	if s.finishing {
		s.emergency()
	}
	s.finishing = true
	if s.published {
		_ = writeLine(s.inv.Stderr, startBookkeepingNotice)
		exitHook(1)
	}
	response, status, skipLine := "", 0, ""
	switch family {
	case "notice":
		notice, ok := StartOutcomeNotices[key]
		if !ok {
			s.emergency()
		}
		response, status = notice.text, notice.status
		if key == "interrupted" {
			status = s.consumeSignal()
		}
		if (key == "brain-boot" || key == "brain-timeout") && s.notices != "" {
			response = appendNoticeText(response, startJSONEscape(s.notices))
		}
		if key == "arming" && s.armingRearmed {
			response = appendNoticeText(response, "Metasystem re-armed the rebuilt engine: the engine reported a completed re-arm before the later failure.")
		}
		if s.armingStarted && response != startLastResort {
			response = appendNoticeText(response, "Supervision may have been partly initialized.")
		}
	case "intentional":
		switch key {
		case "authenticated-delegate":
			skipLine = "Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context."
		case "foreign-runtime":
			skipLine = "Metasystem SessionStart intentionally skipped: another runtime owns this process."
		case "context-ready", "screen-context-ready", "notices-ready", "healthy-no-context":
			built, ok := s.buildResponse(key)
			if !ok {
				s.emergency()
			}
			response = built
		default:
			s.emergency()
		}
	default:
		s.emergency()
	}

	// A signal that arrived while the outcome was being decided replaces it.
	s.pollSignals()
	if pending := int(s.signal.Swap(0)); pending != 0 {
		response = StartOutcomeNotices["interrupted"].text
		status = pending
		family, key, skipLine = "notice", "interrupted", ""
		if s.armingStarted {
			response = appendNoticeText(response, "Supervision may have been partly initialized.")
		}
	}

	if skipLine != "" {
		if writeLine(s.inv.Stderr, skipLine) != nil {
			if writeLine(s.inv.Stdout, startLastResort) == nil {
				exitHook(74)
			}
			s.emergency()
		}
		s.published = true
		exitHook(0)
	}

	if writeLine(s.inv.Stdout, response) != nil {
		_ = writeLine(s.inv.Stderr, startLastResort)
		exitHook(74)
	}
	s.published = true

	if family == "intentional" && s.brainDelivery {
		cursor, prefix := "", ""
		if s.brainDigestEmitted == "true" {
			cursor, prefix = s.brainDigestCursor, s.brainDigestPrefix
		}
		if s.ops.BrainStartDelivered(s.repo, s.repo, s.brainDeclarationSHA, cursor, prefix) != 0 {
			_ = writeLine(s.inv.Stderr, startBookkeepingNotice)
			status = 1
		}
	}
	s.pollSignals()
	if s.signal.Load() != 0 {
		// A signal after publication cannot change the published response;
		// its bookkeeping is unconfirmed.
		if status == 0 {
			_ = writeLine(s.inv.Stderr, startBookkeepingNotice)
		}
		status = 1
	}
	exitHook(status)
}

// consumeSignal takes the pending signal's status (143 when none).
func (s *startRun) consumeSignal() int {
	if status := int(s.signal.Swap(0)); status != 0 {
		return status
	}
	return 143
}

// deferBrainFailure records a brain failure; preparation continues without
// context and the failure becomes the published outcome.
func (s *startRun) deferBrainFailure(key string) {
	s.brainFailure = key
	s.contextKind = "none"
	s.contextPayload = ""
}

func (s *startRun) finishPrepared() {
	switch {
	case s.brainFailure == "brain-boot":
		s.finish("notice", "brain-boot")
	case s.brainFailure == "brain-timeout":
		s.finish("notice", "brain-timeout")
	case s.contextKind == "channel":
		s.finish("intentional", "context-ready")
	case s.contextKind == "screen":
		s.finish("intentional", "screen-context-ready")
	case s.notices != "":
		s.finish("intentional", "notices-ready")
	default:
		s.finish("intentional", "healthy-no-context")
	}
}

func (s *startRun) main() {
	inv, ops := s.inv, s.ops
	if staged := inv.env(runtimeHookBootstrappedEnv); strings.HasPrefix(filepath.Base(staged), bootstrapPayloadPrefix) &&
		filepath.Dir(staged) == filepath.Clean(inv.TempDir) {
		defer os.Remove(staged)
	}
	if !runtimeNamePattern.MatchString(inv.Runtime) {
		s.finish("notice", "invocation-invalid")
	}

	stateHint := inv.env("METASYSTEM_HOOK_DELEGATE_STATE_ROOT")
	installationHint := inv.env("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")
	jobHint := inv.env("METASYSTEM_HOOK_DELEGATE_JOB")
	if stateHint+installationHint+jobHint != "" {
		if stateHint == "" || installationHint == "" || jobHint == "" || !inv.IsExecutable(installationHint+"/bin/metasystem") {
			s.finish("notice", "custody-unreadable")
		}
		if s.delegateCustody(stateHint, installationHint, jobHint) {
			s.finish("intentional", "authenticated-delegate")
		}
		s.finish("notice", "custody-unreadable")
	}

	scriptDir, ok := physicalDirectory(scriptParent(inv.Script))
	if !ok {
		s.finish("notice", "installation-directory")
	}
	s.harnessRoot, ok = physicalDirectory(scriptDir + "/../..")
	if !ok {
		s.finish("notice", "installation-directory")
	}
	s.checkpoint()
	s.world, ok = worldInstallation(ops, s.harnessRoot)
	s.checkpoint()
	if !ok || s.world == "" {
		s.finish("notice", "checkout-identification")
	}
	canonical := s.world + "/bin/metasystem"
	s.engine = inv.env("METASYSTEM_BIN")
	if s.engine == "" {
		s.engine = canonical
	}
	if !inv.IsExecutable(canonical) || !inv.IsExecutable(s.engine) {
		s.finish("notice", "engine-missing")
	}
	registered, status := ops.RuntimeNames()
	s.checkpoint()
	registered = trimNewlines(registered)
	if status != 0 || registered == "" {
		s.finish("notice", "runtime-registry")
	}
	if !containsLine(registered, inv.Runtime) {
		// Refusing the start does not end the session: a real session whose
		// SessionStart exits non-zero keeps running and still fires its tool
		// hook, which leaves a call untouched when the cache is unreadable.
		// Write the cache before refusing so that session stays gated.
		s.writeEngineCache()
		s.finish("notice", "runtime-unregistered")
	}

	payload, err := io.ReadAll(inv.Stdin)
	s.checkpoint()
	if err != nil {
		s.finish("notice", "payload-storage")
	}
	s.payload = payload
	repoResult, status := ops.StateRoot(s.world)
	s.checkpoint()
	repoResult = trimNewlines(repoResult)
	if status == 1 {
		s.finish("notice", "installation-validation")
	} else if status != 0 || !oneLine(repoResult) {
		s.finish("notice", "engine-skew")
	}
	s.repo, ok = physicalDirectory(repoResult)
	if !ok || s.repo == "" {
		s.finish("notice", "resolved-directory")
	}

	probe, status := jsonStrip(string(payload), "__metasystem_shape_probe")
	if status != 0 {
		s.finish("notice", "payload-read")
	}
	empty := ""
	session, status := jsonGet(probe, "session_id", true, &empty)
	if status != 0 {
		s.finish("notice", "payload-read")
	}
	if strings.Contains(probe, "\n  \"source\":") {
		source, status := jsonGet(probe, "source", true, nil)
		if status != 0 || source == "" {
			s.finish("notice", "payload-read")
		}
		s.startSource = source
	}
	if session == "" {
		s.sessionAbsent = true
		session = "session-" + strconv.Itoa(inv.Ppid)
	}
	if !sessionShape.MatchString(session) {
		session = sha256Text(session)
	}
	s.session = session

	if s.delegateCustody(s.repo, s.world, "") {
		s.finish("intentional", "authenticated-delegate")
	}

	s.resolveIdentity()

	// The cache is written only once the session is known to belong to this
	// runtime: a session another runtime owns leaves no file under
	// artifacts/agents.
	s.writeEngineCache()

	s.bootstrapEngine()

	declaration, status := ops.StartContext(inv.Runtime)
	s.checkpoint()
	declaration = trimNewlines(declaration)
	if status == 1 {
		declaration = ""
	} else if status != 0 {
		s.finish("notice", "context-contract")
	}
	if declaration != "" {
		field, event, bytes, sources := parseStartContext(declaration)
		parsedBytes, bytesErr := strconv.Atoi(bytes)
		if !(contextFieldShape.MatchString(field) && contextEventShape.MatchString(event) &&
			digits.MatchString(bytes) && bytesErr == nil && parsedBytes >= 2048 && sources != "") {
			s.finish("notice", "context-contract")
		}
		s.contextField, s.contextEvent, s.contextBytes = field, event, parsedBytes
	}

	pending, status := ops.StewardPending(s.repo)
	s.checkpoint()
	if status != 0 {
		s.finish("notice", "pending-read")
	}
	if pending = trimNewlines(pending); pending != "" {
		s.collectNotice("Steward incidents pending: " + pending)
	}

	s.brainDeadlineMS = 5000
	if value := inv.env("METASYSTEM_BRAIN_BOOT_DEADLINE_MS"); positiveInteger.MatchString(value) {
		if parsed, err := strconv.Atoi(value); err == nil {
			s.brainDeadlineMS = parsed
		}
	}
	s.brainWait = time.Duration(s.brainDeadlineMS/1000+3) * time.Second
	s.prepareBrain()
	s.prepareContextObject()

	if s.deferredIdentityRead {
		s.collectNotice("Metasystem supervision could not identify the immediate " + inv.Runtime + " agent process; arming was refused.")
	} else {
		s.armingStarted = true
		var output capturedOutput
		request := UpRequest{
			Runtime: inv.Runtime, MetasystemRoot: s.world, Repo: s.repo, Session: s.session,
			Pid: s.identityPid, StartTime: s.identityStarted, Tag: inv.Runtime + ":" + s.identityPid,
			StartSource: s.startSource, CallerPid: inv.Pid,
		}
		if s.sessionAbsent {
			request.NoRuntimeSession = true
		} else {
			request.RuntimeSession = s.session
		}
		s.armingStatus = ops.Up(request, &output, &output)
		s.checkpoint()
		captured := trimNewlines(output.String())
		if strings.Contains(captured, " re-armed=") {
			s.armingRearmed = true
		}
		aggregate := captured[strings.LastIndex(captured, "\n")+1:]
		if s.armingStatus != 0 {
			s.collectNotice("Metasystem supervision arming failed: " + aggregate)
			if strings.Contains(aggregate, " re-armed=") {
				s.collectNotice("Metasystem re-armed the rebuilt engine: " + aggregate)
			}
		} else if strings.Contains(aggregate, " re-armed=") {
			s.collectNotice("Metasystem re-armed the rebuilt engine: " + aggregate)
		}
	}
	// A revived session may already hold the checkout when process discovery
	// or arming is unavailable. The session-start owner matches this session
	// against the holder's announced session before returning wait rows.
	var waiting capturedOutput
	s.waitRecoveryStatus = ops.SessionStart(s.repo, s.session, &waiting, &waiting)
	s.checkpoint()
	lines := trimNewlines(waiting.String())
	switch {
	case s.waitRecoveryStatus == 0:
		if lines != "" {
			s.collectNotice(lines)
		}
	case s.waitRecoveryStatus != 64:
		s.collectNotice("Metasystem could not read durable wait recovery rows for this session: " + strings.ReplaceAll(lines, "\n", "; "))
	}
	s.finishPrepared()
}

// capturedOutput collects the combined output of one owner call.
type capturedOutput struct{ strings.Builder }

func containsLine(lines, want string) bool {
	for _, line := range strings.Split(lines, "\n") {
		if line == want {
			return true
		}
	}
	return false
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// delegateCustody asks the custody owner whether this hook's caller descends
// from exact delegate-job custody. Unreadable custody ends the start.
func (s *startRun) delegateCustody(root, metasystemRoot, job string) bool {
	result, status := s.ops.HookDelegate(root, metasystemRoot, job, s.inv.Ppid)
	s.checkpoint()
	if status == 3 {
		return false
	}
	if status != 0 || !delegateCustody.MatchString(trimNewlines(result)) {
		s.finish("notice", "custody-unreadable")
	}
	return true
}

// resolveIdentity finds the runtime process this session belongs to: the
// runtime-signature ancestor, else the exact announced main the lease
// classifies. Another runtime's process ends the start as a skip; an
// unreadable identity defers to the holder notice and skips arming.
func (s *startRun) resolveIdentity() {
	inv, ops := s.inv, s.ops
	var identity string
	var status int
	if inv.Runtime == "fake" {
		identity, status = ops.FindAncestor(s.world, inv.Ppid, "fake", false)
	} else {
		identity, status = ops.FindAncestor(s.world, inv.Ppid, "", true)
	}
	s.checkpoint()
	if status != 0 {
		s.deferredIdentityRead = true
		identity = ""
	}
	identity = trimNewlines(identity)
	if identity != "" {
		runtime, status := jsonGet(identity, "runtime", true, nil)
		if status != 0 {
			s.deferredIdentityRead = true
			runtime = ""
		}
		if !s.deferredIdentityRead && runtime != inv.Runtime {
			s.finish("intentional", "foreign-runtime")
		}
		if !s.deferredIdentityRead {
			s.identityPid = s.identityField(identity, "pid")
			s.identityStarted = s.identityField(identity, "pidStartedAt")
		}
		if !(positiveInteger.MatchString(s.identityPid) && positiveInteger.MatchString(s.identityStarted)) {
			s.deferredIdentityRead = true
			s.identityPid, s.identityStarted = "", ""
		}
		return
	}
	processIdentityFailed := s.deferredIdentityRead
	s.deferredIdentityRead = false
	view, status := ops.Classify(s.repo, s.world, inv.Ppid)
	s.checkpoint()
	view = trimNewlines(view)
	if status != 0 || view == "" {
		s.deferredIdentityRead = true
		view = ""
	}
	class := ""
	if !s.deferredIdentityRead {
		var status int
		if class, status = jsonGet(view, "class", true, nil); status != 0 {
			s.deferredIdentityRead = true
		}
	}
	if !s.deferredIdentityRead && class != "MAIN" {
		s.deferredIdentityRead = true
	}
	parentRuntime := ""
	if !s.deferredIdentityRead {
		var status int
		if parentRuntime, status = jsonGet(view, "announcement.runtime", true, nil); status != 0 {
			s.deferredIdentityRead = true
		}
	}
	if !s.deferredIdentityRead {
		if parentRuntime != inv.Runtime {
			s.finish("intentional", "foreign-runtime")
		}
		s.identityPid = s.identityField(view, "announcement.pid")
		s.identityStarted = s.identityField(view, "announcement.pidStartedAt")
	}
	if !s.deferredIdentityRead && !(positiveInteger.MatchString(s.identityPid) && positiveInteger.MatchString(s.identityStarted)) {
		s.deferredIdentityRead = true
	}
	if processIdentityFailed {
		s.deferredIdentityRead = true
	}
}

func (s *startRun) identityField(value, field string) string {
	out, status := jsonValueStatus(value, field)
	if status != 0 {
		s.deferredIdentityRead = true
		return ""
	}
	return out
}

// writeEngineCache records the engine and installation Claude's tool gate
// execs; best effort on both sides, replaced by rename.
func (s *startRun) writeEngineCache() {
	directory := filepath.Join(s.harnessRoot, "artifacts", "agents", "context")
	if os.MkdirAll(directory, 0o755) != nil {
		return
	}
	temporary, err := os.CreateTemp(directory, ".engine-path.")
	if err != nil {
		return
	}
	name := temporary.Name()
	_, writeErr := temporary.WriteString(s.engine + "\n" + s.world + "\n")
	closeErr := temporary.Close()
	if writeErr != nil || closeErr != nil || os.Rename(name, filepath.Join(directory, "engine-path")) != nil {
		_ = os.Remove(name)
	}
}

// bootstrapEngine is the hook side of a generation cutover: when the
// installation's enrolled engine was built from sources the checkout's
// landed tree has moved past, it rebuilds the engine and restarts this start
// on the rebuilt engine, whose arming re-enrolls it. A retained plan the
// rebuilt engine could not run holds the rearm until that plan finishes.
func (s *startRun) bootstrapEngine() {
	if s.inv.env(runtimeHookBootstrappedEnv) != "" || s.inv.Exec == nil {
		return
	}
	behind, err := s.ops.EngineBehind(s.world, s.repo)
	s.checkpoint()
	if err != nil || !behind {
		return
	}
	held, err := s.ops.UnmigratableRetainedPlans(s.repo)
	s.checkpoint()
	if err != nil {
		s.collectNotice("Metasystem could not check retained plans before rebuilding the engine behind this checkout's sources: " + err.Error())
		return
	}
	if len(held) > 0 {
		s.collectNotice("Metasystem kept the enrolled engine: retained plans " + strings.Join(held, ", ") + " need it; the engine is rebuilt once they finish.")
		return
	}
	if err := s.ops.RebuildEngine(s.world); err != nil {
		s.checkpoint()
		s.collectNotice("Metasystem could not rebuild the engine behind this checkout's sources: " + err.Error())
		return
	}
	s.checkpoint()
	// The restarted start reads this start's payload, staged for the shell
	// that execs the stub; the restarted hook removes the staged file.
	staged, err := os.CreateTemp(s.inv.TempDir, bootstrapPayloadPrefix)
	if err == nil {
		_, err = staged.Write(s.payload)
		if closeErr := staged.Close(); err == nil {
			err = closeErr
		}
	}
	var shell string
	if err == nil {
		shell, err = exec.LookPath("bash")
	}
	if err == nil {
		env := append(s.inv.Environ(), runtimeHookBootstrappedEnv+"="+staged.Name())
		err = s.inv.Exec(shell, []string{"bash", "-c", `exec bash "$0" "$1" start <"$2"`, s.inv.Script, s.inv.Runtime, staged.Name()}, env)
		_ = os.Remove(staged.Name())
	}
	s.collectNotice("Metasystem rebuilt the engine but could not restart SessionStart on it: " + fmt.Sprint(err))
}

// parseStartContext splits `field=F event=E bytes=B sources=S` the way the
// hook always read it.
func parseStartContext(declaration string) (field, event, bytes, sources string) {
	cutBefore := func(value, marker string) string {
		if index := strings.Index(value, marker); index >= 0 {
			return value[:index]
		}
		return value
	}
	after := func(value, marker string) string {
		if index := strings.Index(value, marker); index >= 0 {
			return value[index+len(marker):]
		}
		return value
	}
	field = cutBefore(strings.TrimPrefix(declaration, "field="), " event=")
	event = cutBefore(after(declaration, " event="), " bytes=")
	bytes = cutBefore(after(declaration, " bytes="), " sources=")
	sources = after(declaration, " sources=")
	return field, event, bytes, sources
}

// prepareBrain composes the read-only brain packet within its deadline and
// validates it field by field; any failure defers to the brain notice and
// preparation continues to arming and wait recovery.
func (s *startRun) prepareBrain() {
	type bootResult struct {
		stdout, stderr string
		status         int
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan bootResult, 1)
	go func() {
		stdout, stderr, status := s.ops.BrainBoot(ctx, s.repo, s.repo, s.contextBytes, s.brainDeadlineMS)
		done <- bootResult{stdout, stderr, status}
	}()
	after := s.inv.After
	if after == nil {
		after = time.After
	}
	var result bootResult
	timeout := after(s.brainWait)
	for waiting := true; waiting; {
		select {
		case result = <-done:
			waiting = false
		case received := <-s.inv.Signals:
			// The boot wait polls, so a signal ends the start while it waits.
			s.signal.CompareAndSwap(0, int32(signalStatus(received)))
			s.checkpoint()
		case <-timeout:
			cancel()
			s.checkpoint()
			s.deferBrainFailure("brain-timeout")
			return
		}
	}
	s.checkpoint()
	if result.status != 0 || result.stdout == "" {
		if result.stderr != "" {
			if _, err := io.WriteString(s.inv.Stderr, result.stderr); err != nil {
				_ = writeLine(s.inv.Stderr, "Metasystem SessionStart could not relay the brain boot diagnostic.")
			}
		}
		s.deferBrainFailure("brain-boot")
		return
	}
	out := result.stdout
	required := func(field string) (string, bool) {
		value, status := jsonGet(out, field, false, nil)
		return trimNewlines(value), status == 0
	}
	shell := func(field string) (string, bool) {
		value, status := jsonGet(out, field, true, nil)
		return value, status == 0
	}
	optional := func(field string) (string, bool) {
		value, status := jsonGet(out, field, true, nil)
		if status == 3 {
			return "", true
		}
		return value, status == 0
	}
	declared, ok := required("declared")
	if !ok {
		s.deferBrainFailure("brain-boot")
		return
	}
	if declared == "false" {
		unknown, status := jsonStrip(out, "declared")
		if status != 0 || unknown != "{}" {
			s.deferBrainFailure("brain-boot")
			return
		}
		s.contextKind = "none"
		return
	}
	if declared != "true" {
		s.deferBrainFailure("brain-boot")
		return
	}
	unknown, status := jsonStrip(out, "declared", "state", "payload", "bytes", "sections", "digestEmitted",
		"digestCursor", "digestPrefixSha256", "declarationSha256")
	if status != 0 || unknown != "{}" {
		s.deferBrainFailure("brain-boot")
		return
	}
	state, ok1 := shell("state")
	bytes, ok2 := required("bytes")
	emitted, ok3 := required("digestEmitted")
	cursor, ok4 := required("digestCursor")
	prefix, ok5 := optional("digestPrefixSha256")
	declarationSHA, ok6 := optional("declarationSha256")
	payload, ok7 := shell("payload")
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) || payload == "" {
		s.deferBrainFailure("brain-boot")
		return
	}
	s.brainDigestEmitted, s.brainDigestCursor, s.brainDigestPrefix = emitted, cursor, prefix
	s.brainDeclarationSHA = declarationSHA
	sectionsValid := true
	for _, section := range []string{"asks", "held", "fleet", "digest"} {
		sectionState, ok := shell("sections." + section)
		if !ok {
			s.deferBrainFailure("brain-boot")
			return
		}
		switch sectionState {
		case "complete", "cut", "skipped", "error":
		default:
			sectionsValid = false
		}
	}
	if (state != "declared" && state != "corrupt") || !digits.MatchString(bytes) ||
		(emitted != "true" && emitted != "false") || !digits.MatchString(cursor) || !sectionsValid {
		s.deferBrainFailure("brain-boot")
		return
	}
	if emitted == "true" && !sha256Hex.MatchString(prefix) {
		s.deferBrainFailure("brain-boot")
		return
	}
	if state == "declared" {
		if !sha256Hex.MatchString(declarationSHA) {
			s.deferBrainFailure("brain-boot")
			return
		}
		s.brainDelivery = true
	} else if declarationSHA != "" {
		s.deferBrainFailure("brain-boot")
		return
	}
	s.contextPayload = payload
	if s.contextField != "" {
		s.contextKind = "channel"
		return
	}
	s.contextKind = "screen"
	s.notices = screenOnlyNotice
	s.collectNotice(payload)
	if len(s.notices) > 2048 {
		s.notices = s.notices[:2048]
	}
	s.contextPayload = ""
}

// prepareContextObject renders the runtime's context field as one nested
// object and proves the rendering returns exactly the packet and event.
func (s *startRun) prepareContextObject() {
	s.preparedContext = ""
	s.contextShapeReady = false
	if s.contextPayload == "" || s.contextField == "" {
		return
	}
	if s.contextKind != "channel" || s.contextEvent == "" {
		s.finish("notice", "response-rendering")
	}
	segments := strings.Split(s.contextField, ".")
	leaf := jsonObject(segments[len(segments)-1] + "=" + s.contextPayload)
	if leaf == "" {
		s.finish("notice", "response-rendering")
	}
	event := jsonObject("hookEventName=" + s.contextEvent)
	if event == "" {
		s.finish("notice", "response-rendering")
	}
	leaf = strings.TrimSuffix(leaf, "}") + "," + strings.TrimPrefix(event, "{")
	for index := len(segments) - 2; index >= 0; index-- {
		leaf = `{"` + segments[index] + `":` + leaf + `}`
	}
	probe, status := jsonGet(leaf, s.contextField, true, nil)
	if status != 0 || probe != s.contextPayload {
		s.finish("notice", "response-rendering")
	}
	eventField := strings.Join(segments[:len(segments)-1], ".") + ".hookEventName"
	eventProbe, status := jsonGet(leaf, eventField, true, nil)
	if status != 0 || eventProbe != s.contextEvent {
		s.finish("notice", "response-rendering")
	}
	s.preparedContext = leaf
	s.contextShapeReady = true
}
