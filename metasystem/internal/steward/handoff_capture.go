package steward

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

const (
	handoffClassMain     = "MAIN"
	handoffClassDelegate = "DELEGATE"
	handoffClassHuman    = "HUMAN"
)

var (
	mintHandoffNonce       = randomHandoffNonce
	handoffSourceAfterRead = func(string) {}
	handoffCandidateRead   = func(string) {}
	handoffStatePublished  = func(string) {}
	beforeHandoffPrepare   = func() {}
	planReferencePattern   = regexp.MustCompile(`plans/[A-Za-z0-9._/-]+\.md`)
)

// HandoffCaller is the lease-classified caller projection accepted by the
// steward boundary. Main facts come from its announcement. A delegate is
// rebound to the active continuation's job record before any state is made.
type HandoffCaller struct {
	Class, MainId, HolderMainId, JobId string
	Runtime, Session, Machine          string
	Ref                                identity.Ref
	Tag                                string
}

// HandoffCanceller identifies who asks to cancel. Human is present only for
// an attended-human act, whose proof and holder coordinates the steward checks.
type HandoffCanceller struct {
	Caller HandoffCaller
	Human  *HandoffHumanAct
}

// HandoffHumanAct carries the named human, terminal proof, and holder
// coordinates observed by the command at the cancellation boundary.
type HandoffHumanAct struct {
	By            string
	Proof         humanauthority.Proof
	HolderMainId  string
	HolderSession string
	ClaimEpoch    int64
}

type ScratchArg struct {
	Purpose  string
	Path     string
	Required bool
}

// HandoffRecord is the caller's complete record-time declaration. The note
// directory is resolved from runtime configuration before this boundary.
type HandoffRecord struct {
	Scratch       []ScratchArg
	Delegates     []HandoffDelegate
	NotePath      string
	NoteDirectory string
}

type HandoffRefusal struct {
	Code   string
	Detail string
}

func (e *HandoffRefusal) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + " " + e.Detail
}

type HandoffResult struct {
	Nonce       string
	StatePath   string
	StateDigest string
	IntentPath  string
	Intent      Intent
}

type ContextPruneResult struct {
	CallSessions int
	Handoffs     []string
}

type capturedSource struct {
	purpose  string
	source   string
	data     []byte
	required bool
	missing  bool
}

type capturedJob struct {
	state  HandoffOpenJob
	source capturedSource
}

type capturedHandoff struct {
	goal      *goal.GoalFile
	goalState string
	jobs      []capturedJob
	plans     []capturedSource
	scratch   []capturedSource
	landings  HandoffLastLandings
	messages  []HandoffMessage
	delegates []HandoffDelegate
	note      *HandoffNote
	authority HandoffCaller
}

func refusal(code, detail string) error { return &HandoffRefusal{Code: code, Detail: detail} }

func randomHandoffNonce() (string, error) {
	raw := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func handoffCanonicalRoot(stateRoot string) (string, error) {
	root, err := canonicalExistingPath(stateRoot)
	if err != nil {
		return "", fmt.Errorf("resolve handoff state root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("handoff state root is not a readable directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("handoff state root is not a readable directory: mode=%s", info.Mode())
	}
	return root, nil
}

func validateHandoffReceiptPath(root, receiptFile string) error {
	want := filepath.Join(root, "memory", "receipts.log")
	info, statErr := os.Lstat(receiptFile)
	var got string
	var err error
	if statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("handoff receipt file must be a regular file at %s", want)
		}
		got, err = canonicalExistingPath(receiptFile)
	} else if os.IsNotExist(statErr) {
		var parent string
		parent, err = canonicalExistingPath(filepath.Dir(receiptFile))
		got = filepath.Join(parent, filepath.Base(receiptFile))
	} else {
		err = statErr
	}
	if err != nil || got != want {
		return fmt.Errorf("handoff receipt file must be %s", want)
	}
	return nil
}

func decodeHandoffObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("JSON value is not an object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return object, nil
}

func handoffString(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}

func handoffInt64(object map[string]any, key string) (int64, bool) {
	value, present := object[key]
	if !present || value == nil {
		return 0, false
	}
	switch number := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseInt(number.String(), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func handoffIdentityFromObject(object map[string]any) (identity.Ref, bool) {
	pid, pidOK := handoffInt64(object, "pid")
	started, startedOK := handoffInt64(object, "pidStartedAt")
	if !pidOK || !startedOK || pid < 1 || started < 1 {
		return identity.Ref{}, false
	}
	ref := identity.Ref{Pid: pid, StartedAtSec: started}
	if value, ok := handoffInt64(object, "pidStartedAtExactMicro"); ok {
		ref.StartedAtUnixMicro = value
	}
	if value, ok := handoffInt64(object, "pidStartTicks"); ok {
		ref.StartTicks = value
	}
	ref.BootID = handoffString(object, "bootId")
	return ref, ref.Mode() != identity.CompareInvalid
}

func readStableSource(root, relative, purpose string, required bool) (capturedSource, error) {
	source := capturedSource{purpose: purpose, source: filepath.ToSlash(relative), required: required}
	if err := validateSourcePath(source.source); err != nil {
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=inside-root found=outside-root", source.source))
	}
	path := filepath.Join(root, filepath.FromSlash(source.source))
	before, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			source.missing = true
			return source, nil
		}
		found := "unreadable"
		if os.IsNotExist(err) {
			found = "missing"
		}
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=readable-regular found=%s", source.source, found))
	}
	if !before.Mode().IsRegular() {
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=regular found=%s", source.source, before.Mode()))
	}
	resolved, err := canonicalExistingPath(path)
	if err != nil {
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=inside-root found=unreadable", source.source))
	}
	if _, inside := relativePathInside(root, resolved); !inside || resolved != path {
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=inside-root found=symlink-escape", source.source))
	}
	first, err := os.ReadFile(path)
	if err != nil {
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=readable-regular found=unreadable", source.source))
	}
	handoffSourceAfterRead(path)
	second, err := os.ReadFile(path)
	after, statErr := os.Lstat(path)
	if err != nil || statErr != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || !bytes.Equal(first, second) {
		expected := testableDigest(first)
		found := "unreadable"
		if err == nil {
			found = testableDigest(second)
		}
		return source, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=%s found=%s", source.source, expected, found))
	}
	source.data = first
	return source, nil
}

func testableDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func activeDelegateCaller(root string, caller HandoffCaller) (HandoffCaller, error) {
	jobID, active, err := ConsumedActiveJob(root)
	if err != nil {
		return HandoffCaller{}, err
	}
	if !active || caller.JobId == "" || caller.JobId != jobID {
		return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "")
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", jobID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return HandoffCaller{}, fmt.Errorf("read active continuation job %s: %w", jobID, err)
	}
	object, err := decodeHandoffObject(data)
	if err != nil {
		return HandoffCaller{}, fmt.Errorf("decode active continuation job %s: %w", jobID, err)
	}
	ref, ok := handoffIdentityFromObject(object)
	runtime := handoffString(object, "runtime")
	session := handoffString(object, "sessionId")
	tag := handoffString(object, "instanceTag")
	mainID := handoffString(object, "mainId")
	if !ok || runtime == "" || session == "" || tag == "" || mainID == "" || handoffString(object, "jobId") != jobID || handoffString(object, "status") != "running" {
		return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "active continuation has no complete recorded custodian identity")
	}
	caller.Runtime, caller.Session, caller.Ref, caller.Tag, caller.MainId = runtime, session, ref, tag, mainID
	return caller, nil
}

func requireObservableHandoffRuntime(runtime string) error {
	declaration, registered := runtimes.Lookup(runtime)
	if !registered {
		return fmt.Errorf("handoff runtime %q is not registered", runtime)
	}
	if !declaration.MainObservable {
		return refusal("HANDOFF_UNOBSERVABLE", "runtime="+runtime)
	}
	return nil
}

func admitHandoffCaller(root string, caller HandoffCaller) (HandoffCaller, error) {
	switch caller.Class {
	case handoffClassMain:
		if err := requireObservableHandoffRuntime(caller.Runtime); err != nil {
			return HandoffCaller{}, err
		}
		if caller.MainId == "" || caller.MainId != caller.HolderMainId || caller.Machine == "" || caller.Runtime == "" || caller.Session == "" ||
			caller.Ref.Pid < 1 || caller.Ref.Mode() == identity.CompareInvalid || caller.Tag == "" {
			return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "")
		}
	case handoffClassDelegate:
		var err error
		caller, err = activeDelegateCaller(root, caller)
		if err != nil {
			return HandoffCaller{}, err
		}
		if caller.Machine == "" {
			return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "active continuation has no machine identity")
		}
	default:
		return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "")
	}
	if caller.Class == handoffClassDelegate {
		if err := requireObservableHandoffRuntime(caller.Runtime); err != nil {
			return HandoffCaller{}, err
		}
	}
	return caller, nil
}

// ResolveHandoffCaller completes the runtime identity of an admitted caller.
// Delegate commands need these recorded facts before they can resolve the
// runtime-owned note and transcript paths.
func ResolveHandoffCaller(stateRoot string, caller HandoffCaller) (HandoffCaller, error) {
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return HandoffCaller{}, err
	}
	return admitHandoffCaller(root, caller)
}

type handoffGoalSnapshot struct {
	claimed  []string
	landing  []string
	accepted map[string]*goal.GoalFile
}

type handoffGoalReader func(string, time.Time) (handoffGoalSnapshot, error)

func readHandoffGoalSnapshot(root string, now time.Time) (handoffGoalSnapshot, error) {
	work, err := goal.ReadClaimableBudgetedWork(root, now)
	if err != nil {
		return handoffGoalSnapshot{}, err
	}
	snapshot := handoffGoalSnapshot{claimed: work.Claimed, landing: work.Landing, accepted: make(map[string]*goal.GoalFile)}
	ids := work.Claimed
	if len(ids) == 0 {
		ids = work.Landing
	}
	if len(ids) == 1 {
		if file, ok := work.OwnedClaim(ids[0]); ok {
			snapshot.accepted[ids[0]] = file
		}
	}
	return snapshot, nil
}

func selectHandoffGoal(root string, now time.Time) (*goal.GoalFile, string, error) {
	return selectHandoffGoalWithReader(root, now, readHandoffGoalSnapshot)
}

func selectHandoffGoalWithReader(root string, now time.Time, readGoal handoffGoalReader) (*goal.GoalFile, string, error) {
	work, err := readGoal(root, now)
	if err != nil {
		return nil, "", err
	}
	ids, state := work.claimed, "claimed"
	if len(ids) == 0 {
		ids, state = work.landing, "landing"
	}
	if len(ids) == 0 {
		return nil, "", refusal("HANDOFF_NO_GOAL", "")
	}
	if len(ids) != 1 {
		return nil, "", fmt.Errorf("handoff found %d %s goals for this machine", len(ids), state)
	}
	file, ok := work.accepted[ids[0]]
	if !ok || file == nil || file.Claimed == nil {
		return nil, "", fmt.Errorf("handoff goal %s has no accepted claim record in its admission snapshot", ids[0])
	}
	return file, state, nil
}

func waiterInFlight(root, mainID string) error {
	entries, err := os.ReadDir(run.WaitersDir(root))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		relative := filepath.ToSlash(filepath.Join("artifacts", "agents", "waiters", entry.Name()))
		candidate, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		handoffCandidateRead(relative)
		var waiter run.Waiter
		if err := json.Unmarshal(candidate, &waiter); err != nil {
			return fmt.Errorf("handoff waiter record %s is malformed: %w", relative, err)
		}
		if waiter.SchemaVersion == 0 || waiter.MainId != mainID {
			continue
		}
		source, err := readStableSource(root, relative, "waiter record", true)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(source.data, &waiter); err != nil {
			return fmt.Errorf("handoff waiter record %s is malformed: %w", source.source, err)
		}
		if waiter.MainId != mainID {
			continue
		}
		if waiter.SchemaVersion != 2 {
			return fmt.Errorf("handoff waiter record %s has unsupported schema %d", source.source, waiter.SchemaVersion)
		}
		known := false
		for _, state := range run.WaiterStates {
			if state.Name == waiter.State {
				known = true
				if state.Class == run.WaiterStateInFlight {
					return refusal("HANDOFF_WAIT_IN_FLIGHT", "")
				}
				break
			}
		}
		if !known {
			return fmt.Errorf("handoff waiter record %s has unknown state %q", source.source, waiter.State)
		}
	}
	return nil
}

func captureHandoffJobs(root, heldGoal string) ([]capturedJob, error) {
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var jobs []capturedJob
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".tmp.json") {
			continue
		}
		relative := filepath.ToSlash(filepath.Join("artifacts", "agents", "jobs", entry.Name()))
		candidate, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		handoffCandidateRead(relative)
		object, err := decodeHandoffObject(candidate)
		if err != nil {
			return nil, fmt.Errorf("decode job record %s: %w", entry.Name(), err)
		}
		record := dispatch.JobRecordOf(object)
		if record.GoalID() != heldGoal {
			continue
		}
		source, err := readStableSource(root, relative, "open job record", true)
		if err != nil {
			return nil, fmt.Errorf("capture job record %s: %w", entry.Name(), err)
		}
		object, err = decodeHandoffObject(source.data)
		if err != nil {
			return nil, fmt.Errorf("decode job record %s: %w", entry.Name(), err)
		}
		record = dispatch.JobRecordOf(object)
		if record.GoalID() != heldGoal {
			continue
		}
		if record.Status() == "pending" || record.Status() == "pending-setup" {
			id := record.JobID()
			if id == "" {
				id = strings.TrimSuffix(entry.Name(), ".json")
			}
			return nil, refusal("HANDOFF_LAUNCH_IN_FLIGHT", "job="+id)
		}
		if record.Status() != "running" {
			continue
		}
		if record.JobID() == "" || record.Role() == "" || record.Phase() == "" {
			return nil, fmt.Errorf("running job record %s lacks jobId, role or phase", entry.Name())
		}
		jobs = append(jobs, capturedJob{state: HandoffOpenJob{
			ID: record.JobID(), Role: record.Role(), Status: record.Status(), Phase: record.Phase(),
		}, source: source})
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].state.ID < jobs[j].state.ID })
	return jobs, nil
}

func capturePlanReferences(root, nextStep string) ([]capturedSource, error) {
	seen := map[string]bool{}
	var sources []capturedSource
	for _, candidate := range planReferencePattern.FindAllString(nextStep, -1) {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if err := validateSourcePath(candidate); err != nil {
			return nil, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=inside-root found=outside-root", candidate))
		}
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(candidate))); os.IsNotExist(err) {
			continue
		}
		source, err := readStableSource(root, candidate, "named plan page", true)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func captureScratch(root string, scratch []ScratchArg) ([]capturedSource, error) {
	seen := map[string]bool{}
	sources := make([]capturedSource, 0, len(scratch))
	for _, arg := range scratch {
		if strings.TrimSpace(arg.Purpose) == "" || strings.ContainsAny(arg.Purpose, "\r\n") {
			return nil, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=nonempty-purpose found=invalid", arg.Path))
		}
		key := arg.Purpose + "\x00" + arg.Path
		if seen[key] {
			return nil, refusal("HANDOFF_REFERENCE", fmt.Sprintf("path=%s expected=unique-reference found=duplicate", arg.Path))
		}
		seen[key] = true
		source, err := readStableSource(root, arg.Path, arg.Purpose, arg.Required)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func newestHandoffLandings(file *goal.GoalFile) HandoffLastLandings {
	result := HandoffLastLandings{}
	for index := len(file.History) - 1; index >= 0 && len(result.History) < maxHandoffLandings; index-- {
		row := file.History[index]
		if row.Verb == "land-ready" || row.Verb == "release" {
			result.History = append(result.History, HandoffLanding{At: row.At, OpID: row.Opid, Verb: row.Verb, Actor: row.Actor})
		}
	}
	return result
}

func captureHandoffReceipts(root, goalID string, landings *HandoffLastLandings) error {
	source, err := readStableSource(root, "memory/receipts.log", "goal receipts", false)
	if err != nil {
		return err
	}
	if source.missing {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(string(source.data), "\n"), "\n")
	for index := len(lines) - 1; index >= 0 && len(landings.Receipts) < maxHandoffLandings; index-- {
		fields := strings.Split(lines[index], "|")
		if len(fields) < 3 || fields[2] != "RECEIPT" {
			continue
		}
		values := map[string]string{}
		for _, field := range fields[3:] {
			key, value, ok := strings.Cut(field, "=")
			if ok {
				values[key] = value
			}
		}
		if values["goal"] != goalID {
			continue
		}
		landings.Receipts = append(landings.Receipts, HandoffReceipt{
			Receipt: fmt.Sprintf("memory/receipts.log:%d", index+1), Type: values["type"],
			Outcome: values["outcome"], Note: values["note"],
		})
	}
	return nil
}

func captureHandoffMessages(root string) ([]HandoffMessage, error) {
	pending, err := PendingNotifications(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Nonce < pending[j].Nonce })
	messages := make([]HandoffMessage, 0, len(pending))
	for _, notification := range pending {
		if notification.Nonce == "" || notification.Message == "" {
			return nil, fmt.Errorf("pending notification is missing its nonce or message")
		}
		messages = append(messages, HandoffMessage{Nonce: notification.Nonce, Message: notification.Message, DeliveryStatus: "pending"})
	}
	return messages, nil
}

func captureHandoff(root string, caller HandoffCaller, record HandoffRecord, now time.Time) (capturedHandoff, error) {
	return captureHandoffWithReader(root, caller, record, now, readHandoffGoalSnapshot)
}

func captureHandoffWithReader(root string, caller HandoffCaller, record HandoffRecord, now time.Time, readGoal handoffGoalReader) (capturedHandoff, error) {
	authority, err := admitHandoffCaller(root, caller)
	if err != nil {
		return capturedHandoff{}, err
	}
	file, state, err := selectHandoffGoalWithReader(root, now, readGoal)
	if err != nil {
		return capturedHandoff{}, err
	}
	if file.Claimed == nil {
		return capturedHandoff{}, fmt.Errorf("handoff goal %s lost its accepted claimant", file.Id)
	}
	if authority.Machine != file.Claimed.Machine {
		return capturedHandoff{}, refusal("HANDOFF_NOT_HOLDER", fmt.Sprintf("machine=%s claimant=%s", authority.Machine, file.Claimed.Machine))
	}
	if err := waiterInFlight(root, authority.MainId); err != nil {
		return capturedHandoff{}, err
	}
	jobs, err := captureHandoffJobs(root, file.Id)
	if err != nil {
		return capturedHandoff{}, err
	}
	plans, err := capturePlanReferences(root, file.NextStep)
	if err != nil {
		return capturedHandoff{}, err
	}
	scratchSources, err := captureScratch(root, record.Scratch)
	if err != nil {
		return capturedHandoff{}, err
	}
	landings := newestHandoffLandings(file)
	if err := captureHandoffReceipts(root, file.Id, &landings); err != nil {
		return capturedHandoff{}, err
	}
	messages, err := captureHandoffMessages(root)
	if err != nil {
		return capturedHandoff{}, err
	}
	return capturedHandoff{goal: file, goalState: state, jobs: jobs, plans: plans, scratch: scratchSources,
		landings: landings, messages: messages, delegates: record.Delegates, authority: authority}, nil
}

func captureLessonsNote(record HandoffRecord, authority HandoffCaller, now time.Time, previousDigest string) (*HandoffNote, error) {
	if record.NotePath == "" {
		return nil, refusal("HANDOFF_NOTE_MISSING", "")
	}
	notePath, err := filepath.Abs(record.NotePath)
	if err != nil {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	notePath = filepath.Clean(notePath)
	directory := filepath.Clean(record.NoteDirectory)
	directoryInfo, directoryErr := os.Lstat(directory)
	if directoryErr != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	noteInfo, noteErr := os.Lstat(notePath)
	if noteErr != nil || !noteInfo.Mode().IsRegular() || noteInfo.Mode()&os.ModeSymlink != 0 {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	canonicalDirectory, directoryErr := canonicalExistingPath(directory)
	canonicalNote, noteErr := canonicalExistingPath(notePath)
	if directoryErr != nil || noteErr != nil {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	if _, inside := relativePathInside(canonicalDirectory, canonicalNote); !inside || canonicalNote == canonicalDirectory {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	data, err := os.ReadFile(canonicalNote)
	if err != nil {
		return nil, refusal("HANDOFF_NOTE_OUTSIDE_MEMORY", "path="+record.NotePath)
	}
	for _, delegate := range record.Delegates {
		if !bytes.Contains(data, []byte(delegate.Output)) {
			return nil, refusal("HANDOFF_NOTE_LACKS_OUTPUT", "path="+record.NotePath)
		}
	}
	digest := testableDigest(data)
	modified := noteInfo.ModTime().UTC()
	modifiedSecond := modified.Truncate(time.Second)
	sessionStarted := time.Unix(authority.Ref.StartedAtSec, 0).UTC()
	stale := modifiedSecond.Before(sessionStarted) ||
		(modifiedSecond.Equal(sessionStarted) && previousDigest != "" && previousDigest == digest) ||
		!modified.Before(now)
	if stale {
		return nil, refusal("HANDOFF_NOTE_STALE", fmt.Sprintf("note=%s modified=%s session=%s", record.NotePath,
			modifiedSecond.Format(time.RFC3339), sessionStarted.Format(time.RFC3339)))
	}
	return &HandoffNote{Path: canonicalNote, ModifiedAt: modified, SHA256: digest}, nil
}

func handoffNoteDigest(root string, intent *Intent) (string, error) {
	if intent == nil || intent.Handoff == nil {
		return "", nil
	}
	data, err := os.ReadFile(intent.Handoff.StatePath)
	if err != nil {
		return "", err
	}
	var state HandoffState
	if err := decodeStrictHandoffJSON(data, &state); err != nil {
		return "", err
	}
	if state.LessonsNote == nil {
		return "", nil
	}
	return state.LessonsNote.SHA256, nil
}

func ensureHandoffParent(root string) (string, error) {
	parent := filepath.Join(root, "artifacts", "agents", "context", "handoffs")
	if err := ensureCanonicalDirectory(root, parent); err != nil {
		return "", err
	}
	return parent, nil
}

func createHandoffDirectory(root string) (string, string, error) {
	parent, err := ensureHandoffParent(root)
	if err != nil {
		return "", "", err
	}
	for {
		nonce, err := mintHandoffNonce()
		if err != nil {
			return "", "", fmt.Errorf("mint handoff nonce: %w", err)
		}
		if !handoffNoncePattern.MatchString(nonce) {
			return "", "", fmt.Errorf("minted handoff nonce %q is invalid", nonce)
		}
		inUse, err := handoffNonceInUse(root, nonce)
		if err != nil {
			return "", "", err
		}
		if inUse {
			continue
		}
		dir := filepath.Join(parent, nonce)
		if err := os.Mkdir(dir, 0o755); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", "", err
		}
		if err := syncHandoffDir(parent); err != nil {
			cleanupErr := os.Remove(dir)
			if cleanupErr == nil {
				cleanupErr = syncHandoffDir(parent)
			}
			if cleanupErr == nil {
				return "", "", err
			}
			return nonce, dir, errors.Join(err, fmt.Errorf("clean failed handoff directory %s: %w", nonce, cleanupErr))
		}
		return nonce, dir, nil
	}
}

func writeExclusiveHandoffFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return syncHandoffDir(filepath.Dir(path))
}

func stagedHandoffReference(root, dir, nonce, category string, index int, source capturedSource, write bool) (HandoffReference, error) {
	if source.missing {
		return HandoffReference{CompositionReference: dispatch.CompositionReference{Purpose: source.purpose},
			Required: false, SourcePath: source.source, Status: "missing"}, nil
	}
	name := fmt.Sprintf("%s-%06d.copy", category, index)
	path := filepath.Join(dir, "references", name)
	if write {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return HandoffReference{}, err
		}
		if err := writeExclusiveHandoffFile(path, source.data); err != nil {
			return HandoffReference{}, err
		}
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return HandoffReference{}, err
	}
	reference := HandoffReference{CompositionReference: dispatch.CompositionReference{
		Slot: fmt.Sprintf("%s-%06d", category, index), Purpose: source.purpose,
		Path: filepath.ToSlash(relative), OpenPath: path, Digest: testableDigest(source.data),
		Bytes: len(source.data), Lifetime: "immutable",
	}, Required: source.required, SourcePath: source.source}
	return reference, nil
}

func splitHandoffStateLists(state *HandoffState, manifest *HandoffManifest) {
	if len(state.OpenJobs) > maxHandoffOpenJobs {
		manifest.OpenJobs = append(manifest.OpenJobs, state.OpenJobs[maxHandoffOpenJobs:]...)
		state.OpenJobs = state.OpenJobs[:maxHandoffOpenJobs]
	}
	if len(state.Scratch) > maxHandoffScratch {
		manifest.Scratch = append(manifest.Scratch, state.Scratch[maxHandoffScratch:]...)
		state.Scratch = state.Scratch[:maxHandoffScratch]
	}
	if len(state.MessagesOwed) > maxHandoffMessages {
		manifest.MessagesOwed = append(manifest.MessagesOwed, state.MessagesOwed[maxHandoffMessages:]...)
		state.MessagesOwed = state.MessagesOwed[:maxHandoffMessages]
	}
	if len(state.OpenWork) > maxHandoffOpenWork {
		manifest.OpenWork = append(manifest.OpenWork, state.OpenWork[maxHandoffOpenWork:]...)
		state.OpenWork = state.OpenWork[:maxHandoffOpenWork]
	}
	if len(state.Delegates) > maxHandoffDelegates {
		manifest.Delegates = append(manifest.Delegates, state.Delegates[maxHandoffDelegates:]...)
		state.Delegates = state.Delegates[:maxHandoffDelegates]
	}
}

func marshalHandoffJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func manifestHasContent(manifest HandoffManifest) bool {
	return len(manifest.OpenJobs) > 0 || len(manifest.Scratch) > 0 || len(manifest.MessagesOwed) > 0 ||
		len(manifest.OpenWork) > 0 || len(manifest.Delegates) > 0 ||
		len(manifest.LastLandings.History) > 0 || len(manifest.LastLandings.Receipts) > 0
}

func fitHandoffState(root, dir string, state HandoffState) (HandoffState, HandoffManifest, []byte, []byte, error) {
	manifest := HandoffManifest{SchemaVersion: HandoffSchemaVersion}
	splitHandoffStateLists(&state, &manifest)
	moveOrder := []func(){
		func() { manifest.OpenJobs = append(state.OpenJobs, manifest.OpenJobs...); state.OpenJobs = nil },
		func() { manifest.OpenWork = append(state.OpenWork, manifest.OpenWork...); state.OpenWork = nil },
		func() { manifest.Delegates = append(state.Delegates, manifest.Delegates...); state.Delegates = nil },
		func() { manifest.Scratch = append(state.Scratch, manifest.Scratch...); state.Scratch = nil },
		func() {
			manifest.MessagesOwed = append(state.MessagesOwed, manifest.MessagesOwed...)
			state.MessagesOwed = nil
		},
		func() { manifest.LastLandings = state.LastLandings; state.LastLandings = HandoffLastLandings{} },
	}
	for moved := 0; ; moved++ {
		var manifestData []byte
		if manifestHasContent(manifest) {
			var err error
			manifestData, err = marshalHandoffJSON(manifest)
			if err != nil {
				return HandoffState{}, HandoffManifest{}, nil, nil, err
			}
			manifestPath := filepath.Join(dir, "manifest.json")
			relative, err := filepath.Rel(root, manifestPath)
			if err != nil {
				return HandoffState{}, HandoffManifest{}, nil, nil, err
			}
			state.Manifest = &dispatch.CompositionReference{Slot: "manifest", Purpose: "handoff overflow",
				Path: filepath.ToSlash(relative), OpenPath: manifestPath, Digest: testableDigest(manifestData),
				Bytes: len(manifestData), Lifetime: "immutable"}
		} else {
			state.Manifest = nil
		}
		stateData, err := marshalHandoffJSON(state)
		if err != nil {
			return HandoffState{}, HandoffManifest{}, nil, nil, err
		}
		if len(stateData) < output.MaxInlineBytes {
			return state, manifest, stateData, manifestData, nil
		}
		if moved == len(moveOrder) {
			return HandoffState{}, HandoffManifest{}, nil, nil, refusal("HANDOFF_STATE_TOO_LARGE", fmt.Sprintf("bytes=%d limit=%d", len(stateData), output.MaxInlineBytes-1))
		}
		moveOrder[moved]()
	}
}

func buildHandoffState(root, nonce, dir string, capture capturedHandoff, now time.Time, write bool) (HandoffBinding, error) {
	jobStates := make([]HandoffOpenJob, 0, len(capture.jobs))
	for index, job := range capture.jobs {
		reference, err := stagedHandoffReference(root, dir, nonce, "job", index, job.source, write)
		if err != nil {
			return HandoffBinding{}, err
		}
		job.state.Record = reference
		jobStates = append(jobStates, job.state)
	}
	planReferences := make([]HandoffReference, 0, len(capture.plans))
	for index, source := range capture.plans {
		reference, err := stagedHandoffReference(root, dir, nonce, "plan", index, source, write)
		if err != nil {
			return HandoffBinding{}, err
		}
		planReferences = append(planReferences, reference)
	}
	scratchReferences := make([]HandoffReference, 0, len(capture.scratch))
	for index, source := range capture.scratch {
		reference, err := stagedHandoffReference(root, dir, nonce, "scratch", index, source, write)
		if err != nil {
			return HandoffBinding{}, err
		}
		scratchReferences = append(scratchReferences, reference)
	}
	claim := capture.goal.Claimed
	state := HandoffState{
		SchemaVersion: HandoffSchemaVersion, WrittenAt: now,
		Seat: HandoffSeat{Machine: capture.authority.Machine, Runtime: capture.authority.Runtime,
			Session: capture.authority.Session, NormalizedSession: goal.NormalizeSession(capture.authority.Session),
			MainID: capture.authority.MainId, Identity: capture.authority.Ref, Tag: capture.authority.Tag,
			JobID: capture.authority.JobId},
		HeldGoal: HandoffHeldGoal{ID: capture.goal.Id, State: capture.goalState, AcceptedRevision: capture.goal.Revision,
			Claimant: HandoffClaimant{Machine: claim.Machine, Lineage: claim.Lineage, At: claim.At, Revision: claim.Revision}},
		NextStep: HandoffNextStep{Text: capture.goal.NextStep, References: planReferences},
		OpenJobs: jobStates, LastLandings: capture.landings, Scratch: scratchReferences,
		MessagesOwed: capture.messages, Delegates: capture.delegates, LessonsNote: capture.note,
		Engine: captureHandoffEngine(root), Disposable: HandoffDisposable,
	}
	state, _, stateData, manifestData, err := fitHandoffState(root, dir, state)
	if err != nil {
		return HandoffBinding{}, err
	}
	if write && len(manifestData) > 0 {
		if err := writeExclusiveHandoffFile(filepath.Join(dir, "manifest.json"), manifestData); err != nil {
			return HandoffBinding{}, err
		}
		if _, err := validateManifestReference(root, dir, *state.Manifest); err != nil {
			return HandoffBinding{}, err
		}
	}
	statePath := filepath.Join(dir, "state.json")
	binding := HandoffBinding{StatePath: statePath, StateDigest: testableDigest(stateData), Runtime: capture.authority.Runtime,
		Session: goal.NormalizeSession(capture.authority.Session), MainId: capture.authority.MainId,
		Predecessor: capture.authority.Ref, PredecessorTag: capture.authority.Tag,
		PredecessorJob: capture.authority.JobId, RecordedAt: now}
	if write {
		if err := writeExclusiveHandoffFile(statePath, stateData); err != nil {
			return HandoffBinding{}, err
		}
		if err := syncHandoffDir(dir); err != nil {
			return HandoffBinding{}, err
		}
		handoffStatePublished(statePath)
		if _, err := verifyBoundHandoffState(root, nonce, capture.goal.Id, binding); err != nil {
			return HandoffBinding{}, err
		}
	}
	return binding, nil
}

func captureHandoffEngine(root string) *HandoffEngine {
	path, err := os.Executable()
	if err != nil {
		return nil
	}
	engine := &HandoffEngine{Path: path}
	if data, err := os.ReadFile(path); err == nil {
		engine.SHA256 = testableDigest(data)
	}
	if installed, err := VerifyIdentity(RepoIdentityPath(root), root); err == nil {
		engine.InstallGen = installed.Generation
	}
	return engine
}

func liveHandoffIntents(intents []Intent) ([]Intent, error) {
	var handoffs []Intent
	for _, intent := range intents {
		if intent.Reason != seatHandoffReason && intent.Handoff == nil {
			continue
		}
		if intent.Reason != seatHandoffReason || intent.Handoff == nil {
			return nil, fmt.Errorf("intent %s has an incomplete seatHandoff authorization", intent.Nonce)
		}
		handoffs = append(handoffs, intent)
	}
	return handoffs, nil
}

func readLiveHandoffIntents(root string) ([]Intent, error) {
	entries, err := os.ReadDir(intentsDir(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var intents []Intent
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(intentsDir(root), entry.Name()))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var intent Intent
		if err := json.Unmarshal(data, &intent); err != nil {
			return nil, fmt.Errorf("intent %s malformed: %w", entry.Name(), err)
		}
		intents = append(intents, intent)
	}
	return liveHandoffIntents(intents)
}

// Handoff captures a bounded immutable continuation state and publishes one
// replacement authorization. Every refusal before directory creation leaves
// no nonce state or intent.
func Handoff(stateRoot string, caller HandoffCaller, record HandoffRecord, now time.Time, receiptFile string) (HandoffResult, error) {
	return handoffWithGoalReader(stateRoot, caller, record, now, receiptFile, readHandoffGoalSnapshot)
}

func handoffWithGoalReader(stateRoot string, caller HandoffCaller, record HandoffRecord, now time.Time, receiptFile string, readGoal handoffGoalReader) (HandoffResult, error) {
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return HandoffResult{}, err
	}
	if now.IsZero() {
		return HandoffResult{}, fmt.Errorf("handoff requires one nonzero clock observation")
	}
	if err := validateHandoffReceiptPath(root, receiptFile); err != nil {
		return HandoffResult{}, err
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		return HandoffResult{}, err
	}
	defer arbitration.Release()
	capture, err := captureHandoffWithReader(root, caller, record, now, readGoal)
	if err != nil {
		return HandoffResult{}, err
	}
	roster, err := dispatch.ResolveRoster(dispatch.RosterParams{ConfPath: filepath.Join(root, "metasystem.conf"), Role: continuationRole, Mode: "build"})
	if err != nil {
		return HandoffResult{}, fmt.Errorf("steward continuation roster could not be resolved: %w", err)
	}
	intents, err := LiveIntents(root)
	if err != nil {
		return HandoffResult{}, err
	}
	handoffs, err := liveHandoffIntents(intents)
	if err != nil {
		return HandoffResult{}, err
	}
	var predecessor *Intent
	for index := range handoffs {
		if _, err := verifyBoundHandoffState(root, handoffs[index].Nonce, handoffs[index].Goal, *handoffs[index].Handoff); err != nil {
			return HandoffResult{}, fmt.Errorf("live handoff %s is invalid and cannot be superseded: %w", handoffs[index].Nonce, err)
		}
		if handoffs[index].Handoff.Session != goal.NormalizeSession(capture.authority.Session) {
			return HandoffResult{}, refusal("HANDOFF_OTHER_PENDING", "nonce="+handoffs[index].Nonce)
		}
		if predecessor != nil {
			return HandoffResult{}, fmt.Errorf("more than one live handoff already names session %s", handoffs[index].Handoff.Session)
		}
		predecessor = &handoffs[index]
	}
	previousDigest, err := handoffNoteDigest(root, predecessor)
	if err != nil {
		return HandoffResult{}, fmt.Errorf("read previous handoff note: %w", err)
	}
	capture.note, err = captureLessonsNote(record, capture.authority, now, previousDigest)
	if err != nil {
		return HandoffResult{}, err
	}
	const previewNonce = "0000000000000000"
	if _, err := buildHandoffState(root, previewNonce, HandoffDir(root, previewNonce), capture, now, false); err != nil {
		return HandoffResult{}, err
	}
	nonce, dir, err := createHandoffDirectory(root)
	if err != nil {
		result := HandoffResult{Nonce: nonce}
		if dir != "" {
			result.StatePath = filepath.Join(dir, "state.json")
		}
		return result, err
	}
	binding, err := buildHandoffState(root, nonce, dir, capture, now, true)
	result := HandoffResult{Nonce: nonce, StatePath: filepath.Join(dir, "state.json"), StateDigest: binding.StateDigest}
	if err != nil {
		cleanupErr := os.RemoveAll(dir)
		if cleanupErr == nil {
			cleanupErr = syncHandoffDir(filepath.Dir(dir))
		}
		if cleanupErr == nil {
			return HandoffResult{}, err
		}
		return result, errors.Join(err, fmt.Errorf("clean failed handoff publication %s: %w", nonce, cleanupErr))
	}
	intent, err := StageHandoffIntent(root, nonce, capture.goal.Id, "steward-"+nonce, roster.Runtime, roster.Model, binding)
	result.Intent = intent
	if err != nil {
		return result, err
	}
	if predecessor != nil {
		if err := cancelHandoffUnderLock(root, predecessor.Nonce, "superseded by "+nonce); err != nil {
			_, statErr := os.Lstat(filepath.Join(intentsDir(root), predecessor.Nonce+".json"))
			if os.IsNotExist(statErr) {
				return result, fmt.Errorf("handoff %s was staged and predecessor %s is no longer live, but its cancellation did not finish durably: %w", nonce, predecessor.Nonce, err)
			}
			if statErr == nil {
				return result, fmt.Errorf("handoff %s was staged but predecessor %s remains live because supersession failed: %w", nonce, predecessor.Nonce, err)
			}
			return result, fmt.Errorf("handoff %s was staged but predecessor %s liveness became unreadable during failed supersession (%v): %w", nonce, predecessor.Nonce, statErr, err)
		}
	}
	beforeHandoffPrepare()
	if err := prepareIntentUnderLock(root, receiptFile, intent); err != nil {
		if data, readErr := os.ReadFile(filepath.Join(cancelledDir(root), nonce+".json")); readErr == nil {
			_ = json.Unmarshal(data, &result.Intent)
		}
		if predecessor != nil {
			return result, fmt.Errorf("handoff %s superseded %s, but the replacement intent is not live: %w", nonce, predecessor.Nonce, err)
		}
		return result, err
	}
	prepared, err := readExactLiveIntent(root, nonce)
	if err != nil {
		return result, fmt.Errorf("handoff %s prepared but its live record cannot be read back: %w", nonce, err)
	}
	result.IntentPath = filepath.Join(intentsDir(root), nonce+".json")
	result.Intent = prepared
	return result, nil
}

func readExactLiveIntent(root, nonce string) (Intent, error) {
	data, err := os.ReadFile(filepath.Join(intentsDir(root), nonce+".json"))
	if err != nil {
		return Intent{}, err
	}
	var intent Intent
	if err := json.Unmarshal(data, &intent); err != nil {
		return Intent{}, fmt.Errorf("intent %s malformed: %w", nonce, err)
	}
	return intent, nil
}

func cancelHandoffUnderLock(root, nonce, reason string) error {
	if !handoffNoncePattern.MatchString(nonce) || reason == "" {
		return fmt.Errorf("cancel handoff requires a valid nonce and reason")
	}
	intent, err := readExactLiveIntent(root, nonce)
	if err != nil {
		return err
	}
	if intent.Nonce != nonce || intent.Reason != seatHandoffReason || intent.Handoff == nil {
		return fmt.Errorf("intent %s is not a live bound handoff", nonce)
	}
	return CancelIntent(root, nonce, reason)
}

func handoffHumanRefusal(nonce string, canceller HandoffCanceller, token string) error {
	class := canceller.Caller.Class
	if class == "" {
		class = "none"
	}
	return refusal("HANDOFF_HUMAN_UNPROVEN", fmt.Sprintf("nonce=%s by=%s caller=%s human=%s", nonce, canceller.Human.By, class, token))
}

func handoffBindingNamesAuthority(binding HandoffBinding, authority HandoffCaller) bool {
	return binding.Runtime == authority.Runtime &&
		binding.Session == goal.NormalizeSession(authority.Session) &&
		binding.MainId == authority.MainId &&
		binding.Predecessor == authority.Ref &&
		binding.PredecessorTag == authority.Tag &&
		binding.PredecessorJob == authority.JobId
}

func handoffCancelReason(stateRoot, root, nonce string, intent Intent, canceller HandoffCanceller) (string, error) {
	if canceller.Human == nil {
		authority, err := admitHandoffCaller(root, canceller.Caller)
		if err != nil {
			return "", err
		}
		if handoffBindingNamesAuthority(*intent.Handoff, authority) {
			return "cancelled by the seat", nil
		}
		return "", refusal("HANDOFF_OTHER_SESSION", fmt.Sprintf("nonce=%s session=%s caller=%s human=none", nonce, intent.Handoff.Session, authority.Class))
	}
	act := canceller.Human
	if canceller.Caller.Class != handoffClassHuman {
		return "", handoffHumanRefusal(nonce, canceller, "unattempted")
	}
	if !act.Proof.TerminalValidFor(stateRoot) {
		token := "unobserved"
		if act.Proof.Valid() {
			token = "other-root"
		} else if act.Proof.Outcome != "" && act.Proof.Outcome != humanauthority.OutcomeProven {
			token = act.Proof.Outcome
		}
		return "", handoffHumanRefusal(nonce, canceller, token)
	}
	lease, err := goal.ReadHolderLease(root)
	if err != nil {
		return "", fmt.Errorf("handoff %s cannot be cancelled by a human act: the checkout holder lease cannot be proved: %w", nonce, err)
	}
	if lease.HolderMainId != act.HolderMainId || lease.ClaimEpoch != act.ClaimEpoch {
		return "", fmt.Errorf("handoff %s cannot be cancelled by a human act: the supplied holder coordinates do not match the current checkout lease", nonce)
	}
	if intent.Handoff.MainId != lease.HolderMainId {
		return "", fmt.Errorf("handoff %s cannot be cancelled by a human act: it was recorded by %s and the current holder is %s", nonce, intent.Handoff.MainId, lease.HolderMainId)
	}
	by, err := goal.ValidateHumanName(act.By)
	if err != nil {
		return "", fmt.Errorf("handoff %s cannot be cancelled by a human act: %w", nonce, err)
	}
	p := act.Proof.InvokerRef
	ref := identity.Ref{Pid: p.PID, StartedAtSec: p.PIDStartedAt, StartTicks: p.StartTicks, BootID: p.BootID}
	if ref.StartedAtSec < 1 || ref.Mode() == identity.CompareInvalid {
		return "", fmt.Errorf("handoff %s cannot be cancelled by a human act: the attended-human process identity is invalid", nonce)
	}
	session := "none"
	if act.HolderSession != "" {
		session = goal.NormalizeSession(act.HolderSession)
	}
	boot := ref.BootID
	if boot == "" {
		boot = "none"
	}
	return fmt.Sprintf("cancelled by human by=%s holder=%s epoch=%d session=%s human-pid=%d human-started=%d human-ticks=%d human-boot=%s",
		by, lease.HolderMainId, lease.ClaimEpoch, session, p.PID, p.PIDStartedAt, p.StartTicks, boot), nil
}

// CancelHandoff cancels only a live, bound handoff. Consumption is a terminal
// authorization transition and cannot be relabelled as a seat cancellation.
func CancelHandoff(stateRoot, nonce string, canceller HandoffCanceller) error {
	if !handoffNoncePattern.MatchString(nonce) {
		return refusal("HANDOFF_NOT_LIVE", "nonce="+nonce)
	}
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return err
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer arbitration.Release()
	intent, err := readExactLiveIntent(root, nonce)
	if err != nil {
		if os.IsNotExist(err) {
			return refusal("HANDOFF_NOT_LIVE", "nonce="+nonce)
		}
		return err
	}
	if intent.Reason != seatHandoffReason || intent.Handoff == nil {
		return refusal("HANDOFF_NOT_LIVE", "nonce="+nonce)
	}
	reason, err := handoffCancelReason(stateRoot, root, nonce, intent, canceller)
	if err != nil {
		return err
	}
	if err := cancelHandoffUnderLock(root, nonce, reason); err != nil {
		_, statErr := os.Lstat(filepath.Join(intentsDir(root), nonce+".json"))
		switch {
		case statErr == nil:
			return fmt.Errorf("handoff %s remains live because cancellation did not finish: %w", nonce, err)
		case os.IsNotExist(statErr):
			return fmt.Errorf("handoff %s is no longer live, but cancellation did not finish durably: %w", nonce, err)
		default:
			return fmt.Errorf("handoff %s cancellation failed and liveness is unreadable (%v): %w", nonce, statErr, err)
		}
	}
	return nil
}

// LiveHandoffForSession reads only intact live launch authority. The default
// no-expiry policy needs no wall-clock sample; its single rule still owns the
// admission decision for a future policy change.
func LiveHandoffForSession(stateRoot, session string) (string, bool, error) {
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return "", false, err
	}
	handoffs, err := readLiveHandoffIntents(root)
	if err != nil {
		return "", false, err
	}
	normalized := goal.NormalizeSession(session)
	var found string
	for _, intent := range handoffs {
		if _, err := verifyBoundHandoffState(root, intent.Nonce, intent.Goal, *intent.Handoff); err != nil {
			return "", false, err
		}
		if handoffExpiryRule(intent.Handoff.RecordedAt, time.Time{}) || intent.Handoff.Session != normalized {
			continue
		}
		if found != "" {
			return "", false, fmt.Errorf("more than one live handoff names session %s", normalized)
		}
		found = intent.Nonce
	}
	return found, found != "", nil
}

func handoffIntentForVerification(root, nonce string) (Intent, error) {
	live, liveErr := readExactLiveIntent(root, nonce)
	if liveErr == nil {
		return live, nil
	}
	if !os.IsNotExist(liveErr) {
		return Intent{}, liveErr
	}
	consumed, consumedErr := ConsumedIntent(root, nonce)
	if consumedErr == nil {
		return consumed, nil
	}
	if errors.Is(consumedErr, os.ErrNotExist) {
		return Intent{}, refusal("HANDOFF_NOT_FOUND", "nonce="+nonce)
	}
	return Intent{}, consumedErr
}

// VerifyHandoffState verifies the immutable state for an exact live or
// consumed authorization, including reaped consumed records.
func VerifyHandoffState(stateRoot, nonce string) (string, error) {
	if !handoffNoncePattern.MatchString(nonce) {
		return "", refusal("HANDOFF_NOT_FOUND", "nonce="+nonce)
	}
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return "", err
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		return "", err
	}
	defer arbitration.Release()
	intent, err := handoffIntentForVerification(root, nonce)
	if err != nil {
		return "", err
	}
	if intent.Nonce != nonce || intent.Reason != seatHandoffReason || intent.Handoff == nil {
		return "", refusal("HANDOFF_NOT_FOUND", "nonce="+nonce)
	}
	found, verifyErr := verifyBoundHandoffState(root, nonce, intent.Goal, *intent.Handoff)
	if verifyErr != nil {
		if found == "" {
			found = "unreadable"
		}
		return found, refusal("HANDOFF_STATE_MISMATCH", fmt.Sprintf("expected=%s found=%s detail=%s", intent.Handoff.StateDigest, found, verifyErr))
	}
	return found, nil
}
