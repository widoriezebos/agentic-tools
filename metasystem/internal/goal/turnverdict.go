package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// The turn verdict: one verb, one structured decision. The scanner fills ScanResult (the
// verdict's input contract — internal/report imports this package, never
// the reverse), the verdict decides, and the one capped, pruned, flocked
// state file is the ONLY Stop-state on disk.

// Item is one classified scanner fact. Busy items carry the bounded
// display detail the hook renders verbatim (≤200 bytes at construction).
type Item struct {
	Kind              string `json:"kind"` // job | mission | gate | plan
	Id                string `json:"id"`
	Detail            string `json:"detail"`
	FullDetail        string `json:"fullDetail"`
	LineDigest        string `json:"lineDigest"`
	SourcePath        string `json:"sourcePath"`
	OwnerMainId       string `json:"ownerMainId"`
	RequestedAction   string `json:"requestedAction"`
	PreviouslyRefused bool   `json:"previouslyRefused"`
	HumanRequired     bool   `json:"humanRequired"`
}

// ScanResult is the verdict's input contract, produced by the scanner.
type ScanResult struct {
	Open             []Item   `json:"open"`
	TemplateUnfilled []Item   `json:"templateUnfilled"`
	OpenWorkWarnings []string `json:"openWorkWarnings"`
	WaitingOnHuman   []Item   `json:"waitingOnHuman"`
	StalePlans       []Item   `json:"stalePlans"`
	Busy             []Item   `json:"busy"`
	Questions        []Item   `json:"questions"`
	Drafts           []Item   `json:"drafts"`
	// Unreadable lists every input the scanner could not read — plans,
	// records, markers, enumeration failures, indeterminate runner
	// liveness. Non-empty Unreadable vetoes BOTH the all-clear and any
	// goal block: nothing can be asserted over unread inputs.
	Unreadable []string `json:"unreadable"`
	// Jobs and Runs are the monitor facility's typed facts:
	// the unwatched rule, the run warnings, and the green cursor
	// consume these, never the display-shaped Busy items.
	Jobs []JobFact `json:"jobs"`
	Runs []RunFact `json:"runs"`
	// RunUnreadable is the run readers' own failure channel — surfaced
	// OUTSIDE the ladder so Busy can never hide it, and it freezes the
	// green cursor (the cursor rides only proven-green scans).
	RunUnreadable []string `json:"runUnreadable"`
}

// JobFact is one delegate job's monitor-relevant slice.
type JobFact struct {
	Id           string `json:"id"`
	MainId       string `json:"mainId"`
	StartedAt    string `json:"startedAt"`
	Status       string `json:"status"`
	WaiterLive   bool   `json:"waiterLive"`
	Title        string `json:"title"`
	Role         string `json:"role"`
	GoalId       string `json:"goalId"`
	SourcePath   string `json:"sourcePath"`
	SourceDigest string `json:"sourceDigest"`
	Ownership    string `json:"ownership"`
}

// RunFact is one run record's monitor-relevant slice.
type RunFact struct {
	Id            string `json:"id"`
	MainId        string `json:"mainId"`
	Generation    int    `json:"generation"`
	Nonce         string `json:"nonce"`
	Status        string `json:"status"`
	ProbeState    string `json:"probeState"` // alive | dead | unknown | ""
	TerminalSeq   int64  `json:"terminalSeq"`
	Supervised    bool   `json:"supervised"`
	WaiterLive    bool   `json:"waiterLive"`
	Acked         bool   `json:"acked"`
	Hung          bool   `json:"hung"`
	ExpectGreen   string `json:"expectGreen"`
	ExpectRed     string `json:"expectRed"`
	ExpectHung    string `json:"expectHung"`
	ExpectUnknown string `json:"expectUnknown"`
	Title         string `json:"title"`
	Role          string `json:"role"`
	GoalId        string `json:"goalId"`
	StartedAt     string `json:"startedAt"`
	SourcePath    string `json:"sourcePath"`
	SourceDigest  string `json:"sourceDigest"`
	Ownership     string `json:"ownership"`
}

// OpenWorkSignature keys the open-work block-once slot: the sorted open
// items' details, hashed.
func (r ScanResult) OpenWorkSignature() string {
	lines := make([]string, 0, len(r.Open))
	for _, item := range r.Open {
		if !item.PreviouslyRefused {
			line := item.LineDigest
			if line == "" {
				line = item.Detail
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	return sha256Hex([]byte(strings.Join(lines, "\n")))
}

// GoalFacts is the exported goal read surface; the turn verdict is its
// one in-tree composer.
type GoalFacts struct {
	Id       string `json:"id"`
	Intent   string `json:"intent"`
	NextStep string `json:"nextStep"`
	Revision string `json:"revision"`
}

// Verdict is the verb's structured decision.
type Verdict struct {
	SchemaVersion     int        `json:"schemaVersion"`
	Class             string     `json:"class"`
	CauseCode         string     `json:"causeCode,omitempty"`
	Component         string     `json:"component,omitempty"`
	ShouldBlock       bool       `json:"shouldBlock"`
	BlockSource       *string    `json:"blockSource"` // "open-work" | "goal" | "idle-backlog" | "uncertainty" | null
	OpenWork          []string   `json:"openWork"`
	OpenWorkSignature string     `json:"openWorkSignature"`
	Goal              *GoalFacts `json:"goal"`
	LedgerStatus      string     `json:"ledgerStatus"`
	Diagnostics       []string   `json:"diagnostics"`
	Display           string     `json:"display"`
	// FailClosed is true only when the verb could not compute a verdict and
	// answers fail-closed; the hook renders that as its fixed degraded message.
	FailClosed bool `json:"failClosed,omitempty"`
	// SurfaceWatchdog answers the hook's --watchdog-surfaced digest: true
	// exactly once per new digest per session, decided under the flock.
	SurfaceWatchdog bool `json:"surfaceWatchdog"`
	// IdleRefusal tells the hook that a blocking response belongs to the
	// counted idle-backlog branch, whose text must not use the open-work
	// block-once preface.
	IdleRefusal bool `json:"idleRefusal"`
	CountSpent  bool `json:"countSpent"`
	// BrainStatusDue asks the Stop hook to carry the visibility line. Posting
	// is plumbing after this verdict and never changes the stop decision.
	BrainStatusDue bool              `json:"brainStatusDue"`
	Facts          *TurnVerdictFacts `json:"-"`
	escalation     *TurnEscalationFacts
}

// The Stop-state file: capped, pruned, flocked — the caps bound Stop
// latency and storage by construction.
const (
	maxSessions       = 128
	maxGoalRevisions  = 64
	maxFreeDigests    = 16
	sessionRetainDays = 30
)

type sessionState struct {
	LastTouched          string   `json:"lastTouched"`
	OpenWorkSignature    string   `json:"openWorkSignature"`
	BlockedGoalRevisions []string `json:"blockedGoalRevisions"`
	BlockedFreeDigests   []string `json:"blockedFreeDigests"`
	BlockedQueueDigests  []string `json:"blockedQueueDigests,omitempty"`
	ObservedQueueDigest  string   `json:"observedQueueDigest,omitempty"`
	WatchdogSurfaced     *string  `json:"watchdogSurfaced"`
	// The monitor facility's two additive slots: the
	// unwatched-work block-once digests and the green cursor riding the
	// terminal sequence's total order.
	BlockedUnwatchedDigests []string `json:"blockedUnwatchedDigests,omitempty"`
	GreenCursor             int64    `json:"greenCursor,omitempty"`
	IdleBlockDigest         string   `json:"idleBlockDigest,omitempty"`
	IdleBlocks              int      `json:"idleBlocks,omitempty"`
}

// TurnVerdictOptions carries facts established at the Stop hook boundary.
// The actor is the checkout holder's enrolled machine plus the lineage from
// the holder's announced main process.
type TurnVerdictOptions struct {
	StopHookActive   bool
	SessionAbsent    bool
	SeatActor        Actor
	SeatClaimEpoch   int64
	SeatActorProblem string
	HandoffRecorded  func(session string) (nonce string, live bool, err error)
}

type registeredWait struct {
	row          run.Waiter
	goalID       string
	coveredJobID string
	coveredRunID string
	coveredRun   run.WaiterTarget
	humanAct     bool
	landing      bool
}

type registeredWaits []registeredWait

type authenticatedWatches struct {
	registered  registeredWaits
	humanRuns   map[string]bool
	humanCaller bool
}

func goalIDOf(facts *GoalFacts) string {
	if facts == nil {
		return ""
	}
	return facts.Id
}

func (waits registeredWaits) matchingSignature(signature string) registeredWaits {
	matching := make(registeredWaits, 0, len(waits))
	for _, wait := range waits {
		if wait.row.OpenWorkSignature == signature {
			matching = append(matching, wait)
		}
	}
	return matching
}

func (waits registeredWaits) suppressOpenWork(signature, currentGoalID string, waitingOnHuman []Item) bool {
	for _, wait := range waits.matchingSignature(signature) {
		if !wait.humanAct || wait.goalID == currentGoalID && waitingOnHumanForGoal(waitingOnHuman, wait.goalID) {
			return true
		}
	}
	return false
}

func waitingOnHumanForGoal(items []Item, goalID string) bool {
	for _, item := range items {
		// Plan-stream conditions belong to the checkout's current goal. A
		// typed goal condition, when supplied, must name the covered goal.
		if item.Kind != "goal" || item.Id == goalID {
			return true
		}
	}
	return false
}

func (waits registeredWaits) hasWorkInFlight() bool {
	for _, wait := range waits {
		if !wait.humanAct {
			return true
		}
	}
	return false
}

func (waits registeredWaits) watchesJob(id string) bool {
	for _, wait := range waits {
		if !wait.humanAct && wait.coveredJobID == id {
			return true
		}
	}
	return false
}

func (waits registeredWaits) watchesRun(fact RunFact) bool {
	for _, wait := range waits {
		if !wait.humanAct && wait.coveredRunID == fact.Id &&
			wait.coveredRun.Generation == fact.Generation && wait.coveredRun.LaunchNonce == fact.Nonce {
			return true
		}
	}
	return false
}

func runWatchKey(fact RunFact) string {
	return fmt.Sprintf("%s.g%d.%s", fact.Id, fact.Generation, fact.Nonce)
}

func (watches authenticatedWatches) watchesJob(id string) bool {
	return watches.registered.watchesJob(id)
}

func (watches authenticatedWatches) watchesRun(fact RunFact) bool {
	return watches.registered.watchesRun(fact) || watches.humanRuns[runWatchKey(fact)]
}

func (waits registeredWaits) lines() []string {
	lines := make([]string, 0, len(waits))
	for _, wait := range waits {
		target := wait.row.Kind + " " + wait.row.TargetID
		if wait.landing {
			target = "landing for goal " + wait.goalID
		} else if wait.humanAct {
			target = "human act for goal " + wait.goalID
		}
		lines = append(lines, fmt.Sprintf("WAITING: registered wait %s covers %s until %s", wait.row.WaitID, target, wait.row.Deadline))
	}
	return lines
}

var turnVerdictBootClock = identity.BootClock

// IdleEscalationEvent is the package-neutral handoff from the goal verdict to
// the steward. The command layer supplies the steward-backed callbacks so the
// goal package does not acquire a reverse dependency on the steward package.
type IdleEscalationEvent struct {
	SessionID      string
	MainID         string
	GoalID         string
	BacklogDigest  string
	Refusal        int
	StopHookActive bool
	ClaimActor     Actor
	ClaimNeeded    bool
	SeatClaimEpoch int64
	ClaimMade      bool
	ClaimDetail    string
	IntentID       string
	IntentPrepared bool
	IntentDetail   string
}

type verdictState struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Sessions      map[string]*sessionState `json:"sessions"`
}

func statePath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "turn-verdict-state.json")
}

var safeSession = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// NormalizeSession is the one hygiene rule, applied at the hook boundary
// and defensively here: anything not matching the safe shape becomes its
// sha256 hex.
func NormalizeSession(id string) string {
	if safeSession.MatchString(id) {
		return id
	}
	return sha256Hex([]byte(id))
}

// TurnVerdict decides one turn end. watchdogDigest is the hook's
// --watchdog-surfaced value ("" = no watchdog findings this turn, which
// clears the stored digest); mainId is the CALLER's main identity for
// the unwatched-work rule (empty for humans and unidentified callers).
func (s *Store) TurnVerdict(scan ScanResult, sessionId, watchdogDigest, mainId string, option ...TurnVerdictOptions) (Verdict, error) {
	sessionId = NormalizeSession(sessionId)
	options := TurnVerdictOptions{}
	if len(option) > 0 {
		options = option[0]
	}
	verdict := Verdict{SchemaVersion: 1, Class: "seat-actionable", LedgerStatus: "ok"}
	resolvedRoot, err := ResolveStateRoot(s.Root)
	if err != nil {
		return infrastructureVerdict("state-root", err), nil
	}
	store := *s
	store.Root = resolvedRoot
	s = &store
	brainState := brain.Read(s.Root, ExistingLedgerIdentity(s.Root))
	brainSeat := brainState.State == brain.Declared || brainState.State == brain.Corrupt
	if !brainSeat {
		scan = withoutBrainOnlyScanEffects(scan)
	}
	if closed, fence, fenceErr := stopfence.Closed(s.Root); fenceErr != nil {
		return infrastructureVerdict("goal-fence", fenceErr), nil
	} else if closed {
		description, descriptionErr := stopfence.ClosedDescription(fence, fence.Checkout)
		command, commandErr := stopfence.ClosedCommand(fence, fence.Checkout)
		if descriptionErr != nil {
			return infrastructureVerdict("goal-fence", descriptionErr), nil
		}
		if commandErr != nil {
			return infrastructureVerdict("goal-fence", commandErr), nil
		}
		stopped := Verdict{
			SchemaVersion: 1,
			Class:         "seat-actionable",
			ShouldBlock:   false,
			LedgerStatus:  "stopped",
			Display:       description + "; run: " + command,
		}
		// A closed checkout is still a completed judgment. Freeze its known
		// stop state for presentation instead of making the hook report an
		// auxiliary facts failure. Goal work remains unknown because the stop
		// fence deliberately returns before reading the ledger.
		stamp := &sessionState{LastTouched: s.nowISO()}
		stopped.Facts = freezeTurnVerdictFacts(s.Root, sessionId, mainId, scan,
			ClaimableBudgetedWork{}, false, nil, stopped, stopped.Display, stamp, options, false, authenticatedWatches{})
		stopped.Facts.Actions = []TurnAction{}
		return stopped, nil
	}
	if options.HandoffRecorded != nil {
		nonce, live, handoffErr := options.HandoffRecorded(sessionId)
		if handoffErr != nil {
			return infrastructureVerdict("handoff-record", handoffErr), nil
		}
		if live {
			display := "handoff recorded: " + nonce + "; end this session"
			allowed := Verdict{
				SchemaVersion: 1,
				Class:         "seat-actionable",
				ShouldBlock:   false,
				LedgerStatus:  "ok",
				Display:       display,
			}
			stamp := &sessionState{LastTouched: s.nowISO()}
			allowed.Facts = freezeTurnVerdictFacts(s.Root, sessionId, mainId, scan,
				ClaimableBudgetedWork{}, false, nil, allowed, display, stamp, options, false, authenticatedWatches{})
			allowed.Facts.Actions = []TurnAction{}
			return allowed, nil
		}
	}

	result, err := s.withLock(func() (Result, error) {
		_, humanAuthorized, markerDetail, err := s.inspectSessionStop(sessionId, mainId)
		if err != nil {
			return Result{}, infrastructureFailure{"session-stop-marker", err}
		}
		var work ClaimableBudgetedWork
		var workErr error
		workRead := !humanAuthorized && !brainSeat
		if workRead {
			work, workErr = readClaimableBudgetedWork(s.Root, s.now(), s.prober())
		}
		waits := registeredWaits{}
		if workRead && workErr == nil && len(scan.Unreadable) == 0 && len(scan.RunUnreadable) == 0 {
			waits = s.registeredWaits(work, sessionId, mainId, options.SessionAbsent)
		}
		watches := s.authenticatedWatches(scan, mainId, waits)
		state, err := s.loadVerdictState()
		if err != nil {
			return Result{}, infrastructureFailure{"verdict-state", err}
		}
		session := state.touch(sessionId, s.nowISO())

		brainLines := s.brainSummary(scan, brainState)
		runLines := s.decideRuns(&verdict, scan, session, mainId, watches)
		s.decide(&verdict, scan, session, &work, brainSeat, waits)
		if options.SessionAbsent {
			detail := "registered waits were not read because the Stop supplied no session"
			verdict.Diagnostics = append(verdict.Diagnostics, detail)
			verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
		}
		if markerDetail != "" {
			verdict.Diagnostics = append(verdict.Diagnostics, markerDetail)
			verdict.Display = strings.TrimSpace(verdict.Display + "\n" + markerDetail)
		}
		if !humanAuthorized && !brainSeat {
			s.enforceIdleBacklogWithWaits(&verdict, &work, workErr, session, sessionId, mainId, options, waits)
		}
		greens := s.decideGreens(scan, session)
		fullDisplay := composeDisplay(append(append([]string{}, brainLines...), runLines.lines...), verdict.Display, greens)
		verdict.SurfaceWatchdog = session.watchdog(watchdogDigest)
		verdictPersistenceFailed := false
		if brainState.State == brain.Declared {
			verdict.BrainStatusDue = brain.StatusDue(s.Root, s.now())
			if err := brainStatusWriter(s.Root, *brainState.Record, s.now()); err != nil {
				failure := infrastructureFailure{"status-write", err}
				if !verdict.ShouldBlock {
					return Result{}, failure
				}
				verdictPersistenceFailed = true
				detail := failure.Error() + "; the observed refusal remains blocking"
				verdict.Diagnostics = append(verdict.Diagnostics, detail)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
				fullDisplay = composeDisplay(append(append([]string{}, brainLines...), runLines.lines...), verdict.Display, greens)
			}
		}

		verdictStateSaved := true
		if err := s.saveVerdictState(state); err != nil {
			verdictStateSaved = false
			if verdict.ShouldBlock {
				detail := infrastructureFailure{"verdict-state", err}.Error() + "; the observed refusal remains blocking"
				if verdict.IdleRefusal {
					verdict.CountSpent = false
					detail = "the idle refusal count could not be spent because the turn verdict state could not be written: " + err.Error()
				}
				verdict.Diagnostics = append(verdict.Diagnostics, detail)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
				fullDisplay = composeDisplay(append(append([]string{}, brainLines...), runLines.lines...), verdict.Display, greens)
			} else {
				return Result{}, infrastructureFailure{"verdict-state", err}
			}
		}
		humanStopConsumed := false
		if humanAuthorized && verdictStateSaved && !verdictPersistenceFailed {
			marker, consumed, consumeDetail, err := consumeSessionStopForVerdict(s, sessionId, mainId)
			if err != nil && !verdict.ShouldBlock {
				// Nothing was decided, so the failed consume is the only
				// finding: an infrastructure allowance, the marker unspent.
				return Result{}, infrastructureFailure{"session-stop-marker", err}
			}
			if err != nil {
				// A decided refusal survives a consume that could not be
				// proved, exactly as it survives a lost consume race: the
				// marker stays unspent for a stop that can prove its use,
				// and no failure invents an authorization.
				detail := infrastructureFailure{"session-stop-marker", err}.Error() + "; the observed refusal remains blocking and the authorization stays unspent"
				verdict.Diagnostics = append(verdict.Diagnostics, detail)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
				fullDisplay = strings.TrimSpace(fullDisplay + "\n" + detail)
			} else if !consumed {
				if consumeDetail == "" {
					consumeDetail = "the authorization changed before its single use could be recorded"
				}
				verdict.Diagnostics = append(verdict.Diagnostics, consumeDetail)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + consumeDetail)
				fullDisplay = strings.TrimSpace(fullDisplay + "\n" + consumeDetail)
			} else {
				humanStopConsumed = true
				if consumeDetail != "" {
					verdict.Diagnostics = append(verdict.Diagnostics, consumeDetail)
					verdict.Display = strings.TrimSpace(verdict.Display + "\n" + consumeDetail)
					fullDisplay = strings.TrimSpace(fullDisplay + "\n" + consumeDetail)
				}
				verdict.ShouldBlock = false
				verdict.BlockSource = nil
				sessionStopLine := "SESSION STOP authorized once by " + marker.By +
					fmt.Sprintf(" for holder %s at lease epoch %d", marker.HolderMainId, marker.ClaimEpoch)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + sessionStopLine)
				fullDisplay = strings.TrimSpace(fullDisplay + "\n" + sessionStopLine)
			}
		}
		artifactPath := turnVerdictArtifactPath(s.Root, sessionId)
		fileLine := "Full turn verdict: " + artifactPath
		if durable, writeErr := atomicfile.WriteText(artifactPath, fullDisplay, s.Root); writeErr != nil {
			detail := "full turn verdict could not be written: " + writeErr.Error()
			verdict.Diagnostics = append(verdict.Diagnostics, detail)
			fileLine = "Full turn verdict could not be written to " + artifactPath
		} else if !durable {
			detail := "full turn verdict was written but its crash durability is unknown"
			verdict.Diagnostics = append(verdict.Diagnostics, detail)
			fileLine = "Full turn verdict was written to " + artifactPath + ", but its crash durability is unknown"
		}
		verdict.Display = renderTurnVerdict(verdict, brainLines, runLines, greens, fileLine)
		verdict.Facts = freezeTurnVerdictFacts(s.Root, sessionId, mainId, scan, work, workRead, workErr, verdict, fullDisplay, session, options, humanStopConsumed, watches)
		return Result{}, nil
	})
	_ = result
	if err != nil {
		if failure, ok := err.(infrastructureFailure); ok {
			return infrastructureVerdict(failure.component, failure.err), nil
		}
		return infrastructureVerdict("verdict-state-lock", err), nil
	}
	return verdict, nil
}

// withoutBrainOnlyScanEffects preserves the scanner's Questions and Drafts
// population on a node while preventing a brain-only draft projection failure
// from changing that node's existing turn verdict. Job classification remains
// the scanner's decision so an open chain retains trunk's STILL WORKING branch.
func withoutBrainOnlyScanEffects(scan ScanResult) ScanResult {
	unreadable := make([]string, 0, len(scan.Unreadable))
	for _, detail := range scan.Unreadable {
		if strings.HasPrefix(detail, "draft scan: ") || strings.HasPrefix(detail, "question scan: ") {
			continue
		}
		unreadable = append(unreadable, detail)
	}
	scan.Unreadable = unreadable
	return scan
}

// registeredWaits returns only pending waits owned by this live holder
// session and still joined to the holder's claimed goal and source
// incarnation. A row that cannot prove every coordinate is absent from the
// decision; waiter failures never grant permission to stop.
func (s *Store) registeredWaits(work ClaimableBudgetedWork, sessionID, mainID string, sessionAbsent bool) registeredWaits {
	if sessionAbsent {
		return nil
	}
	lease, lineage, ok := s.registeredWaitOwner(sessionID, mainID)
	if !ok {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(run.WaitersDir(s.Root), "*.json"))
	if len(paths) == 0 {
		return nil
	}
	bootID, bootElapsed, err := turnVerdictBootClock()
	if err != nil || bootID == "" {
		return nil
	}
	claimed := make(map[string]bool, len(work.Claimed)+len(work.Landing))
	for _, id := range work.Claimed {
		claimed[id] = true
	}
	for _, id := range work.Landing {
		claimed[id] = true
	}
	waits := make(registeredWaits, 0, len(paths))
	for _, path := range paths {
		row, ok := registeredWaitAtOwnerPath(s.Root, path)
		if !ok || !s.registeredWaitEligible(row, sessionID, mainID, lineage, lease.ClaimEpoch, bootID, bootElapsed) {
			continue
		}
		wait, ok := s.registeredWaitSource(work, claimed, row, bootElapsed)
		if ok {
			waits = append(waits, wait)
		}
	}
	return waits
}

func (s *Store) authenticatedWatches(scan ScanResult, mainID string, waits registeredWaits) authenticatedWatches {
	watches := authenticatedWatches{registered: waits, humanCaller: mainID == ""}
	if mainID != "" {
		return watches
	}
	watches.humanRuns = map[string]bool{}
	for _, fact := range scan.Runs {
		if fact.MainId == "" && run.AuthenticatedHumanRunWaiter(s.Root, s.prober(), fact.Id,
			run.WaiterTarget{Generation: fact.Generation, LaunchNonce: fact.Nonce}) {
			watches.humanRuns[runWatchKey(fact)] = true
		}
	}
	return watches
}

func registeredWaitAtOwnerPath(root, path string) (run.Waiter, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return run.Waiter{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return run.Waiter{}, false
	}
	var row run.Waiter
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&row) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		filepath.Clean(path) != filepath.Clean(run.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)) {
		return run.Waiter{}, false
	}
	return row, true
}

func (s *Store) registeredWaitOwner(sessionID, mainID string) (sessionStopLease, string, bool) {
	if sessionID == "" || mainID == "" {
		return sessionStopLease{}, "", false
	}
	lease, err := s.currentSessionStopLease()
	if err != nil || lease.HolderMainId != mainID || identity.AliveRef(s.prober(), identity.Ref{
		Pid: lease.Pid, StartedAtSec: lease.PidStartedAt, StartTicks: lease.PidStartTicks, BootID: lease.BootID,
	}) != identity.Alive {
		return sessionStopLease{}, "", false
	}
	paths, err := filepath.Glob(filepath.Join(s.Root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		return sessionStopLease{}, "", false
	}
	lineage := ""
	for _, path := range paths {
		if !sessionStopAnnouncementName.MatchString(filepath.Base(path)) {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return sessionStopLease{}, "", false
		}
		var announcement sessionStopAnnouncement
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&announcement) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return sessionStopLease{}, "", false
		}
		if announcement.MainId != mainID {
			continue
		}
		if lineage != "" || announcement.Pid != lease.Pid || announcement.PidStartedAt != lease.PidStartedAt ||
			announcement.PidStartTicks != lease.PidStartTicks || announcement.BootID != lease.BootID {
			return sessionStopLease{}, "", false
		}
		lineage = announcement.OwnerLineage
		if lineage == "" {
			lineage = mainID
		}
	}
	if lineage == "" {
		return sessionStopLease{}, "", false
	}
	return lease, lineage, true
}

func (s *Store) registeredWaitEligible(row run.Waiter, sessionID, mainID, lineage string, claimEpoch int64, bootID string, bootElapsed time.Duration) bool {
	if row.SchemaVersion != 2 || row.State != "pending" || row.Delivery != "blocking" || row.Result != nil ||
		!run.ValidWaitID(row.WaitID) || !run.ValidWaitID(row.Nonce) || row.MainId != mainID ||
		row.Session != sessionID || row.RuntimeSession != sessionID ||
		(row.OwnerLineage != lineage && row.OwnerLineage != mainID) ||
		row.OwnerDigest != run.OwnerDigest(mainID) || row.ClaimEpoch == nil || *row.ClaimEpoch != claimEpoch ||
		row.Kind != row.Selector.Kind || row.TargetID != row.Selector.TargetID || row.Target == (run.WaiterTarget{}) ||
		(row.OpenWorkSignature != "" && !sessionStopDigest.MatchString(row.OpenWorkSignature)) ||
		run.ValidateWaitSelector(row.Selector) != nil {
		return false
	}
	if row.Kind == "goal" {
		if row.GoalID == "" || row.GoalID != row.Selector.GoalID {
			return false
		}
	} else if row.GoalID != "" {
		return false
	}
	if identity.AliveRef(s.prober(), identity.Ref{
		Pid: row.Pid, StartedAtSec: row.PidStartedAt, StartedAtUnixMicro: row.PidStartedAtMicro,
		StartTicks: row.PidStartTicks, BootID: row.BootID,
	}) != identity.Alive {
		return false
	}
	registeredAt, registeredErr := time.Parse(time.RFC3339Nano, row.RegisteredAt)
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, row.Deadline)
	lastObserved, observedErr := time.Parse(time.RFC3339Nano, row.LastObservedAt)
	now := s.now().UTC()
	if registeredErr != nil || deadlineErr != nil || observedErr != nil || registeredAt.After(lastObserved) ||
		lastObserved.After(now) || now.Sub(lastObserved) > 30*time.Second || !now.Before(deadline) ||
		!deadline.After(registeredAt) || deadline.Sub(registeredAt) > 24*time.Hour || row.RemainingNanos <= 0 {
		return false
	}
	bootNanos := bootElapsed.Nanoseconds()
	if row.DeadlineBootID != bootID ||
		row.LastObservedBootNanos <= 0 || row.LastObservedBootNanos > bootNanos ||
		bootNanos-row.LastObservedBootNanos > (30*time.Second).Nanoseconds() || row.BootDeadlineNanos <= bootNanos ||
		row.BootDeadlineNanos-row.LastObservedBootNanos > (24*time.Hour).Nanoseconds() {
		return false
	}
	return true
}

func (s *Store) registeredWaitSource(work ClaimableBudgetedWork, claimed map[string]bool, row run.Waiter, bootElapsed time.Duration) (registeredWait, bool) {
	wait := registeredWait{row: row}
	switch row.Kind {
	case "job":
		goalID, _, ok := s.pendingWaitJob(row)
		if !ok || !claimed[goalID] {
			return registeredWait{}, false
		}
		wait.goalID, wait.coveredJobID = goalID, row.TargetID
		return wait, true
	case "run":
		goalID, current, ok := s.pendingWaitRun(row.TargetID, row.Target)
		if !ok || !claimed[goalID] {
			return registeredWait{}, false
		}
		wait.goalID, wait.coveredRunID, wait.coveredRun = goalID, row.TargetID, current
		return wait, true
	case "attempt":
		goalID, runID, current, ok := s.pendingWaitAttempt(row)
		if !ok || !claimed[goalID] {
			return registeredWait{}, false
		}
		wait.goalID, wait.coveredRunID, wait.coveredRun = goalID, runID, current
		return wait, true
	case "goal":
		if !claimed[row.Selector.GoalID] {
			return registeredWait{}, false
		}
		if row.Selector.Event == "landing" && !contains(work.Landing, row.Selector.GoalID) {
			return registeredWait{}, false
		}
		deadline, _ := time.Parse(time.RFC3339Nano, row.Deadline)
		budget := min(10*time.Second, deadline.Sub(s.now().UTC()))
		budget = min(budget, time.Duration(row.BootDeadlineNanos)-bootElapsed)
		budget = min(budget, time.Duration(row.RemainingNanos))
		if budget <= 0 {
			return registeredWait{}, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		observation, err := ObserveLedgerForWait(ctx, s.Root, row.Selector, row.Target, row.LastCheckedTip, row.OwnerLineage)
		cancel()
		if err != nil || observation.Temporary || !observation.Pending || observation.Incarnation != row.Target {
			return registeredWait{}, false
		}
		wait.goalID = row.Selector.GoalID
		wait.humanAct = row.Selector.Event == "human-act"
		wait.landing = row.Selector.Event == "landing"
		return wait, true
	default:
		return registeredWait{}, false
	}
}

func (s *Store) pendingWaitJob(row run.Waiter) (string, run.WaiterTarget, bool) {
	data, err := os.ReadFile(filepath.Join(s.Root, "artifacts", "agents", "jobs", row.TargetID+".json"))
	if err != nil {
		return "", run.WaiterTarget{}, false
	}
	var record struct {
		JobID       string `json:"jobId"`
		OperationID string `json:"operationId"`
		Round       int64  `json:"round"`
		Status      string `json:"status"`
		StartedAt   string `json:"startedAt"`
		GoalID      string `json:"goalId"`
	}
	if json.Unmarshal(data, &record) != nil || (record.JobID != "" && record.JobID != row.TargetID) ||
		record.OperationID == "" || record.GoalID == "" {
		return "", run.WaiterTarget{}, false
	}
	if record.Status == "pending-setup" {
		current := run.WaiterTarget{OperationID: record.OperationID}
		return record.GoalID, current, row.Target == current
	}
	if record.Status != "pending" && record.Status != "running" {
		return "", run.WaiterTarget{}, false
	}
	preRunning := row.Target.OperationID == record.OperationID && row.Target.Round == 0 && row.Target.StartedAt == "" && record.Status != "running"
	if !preRunning && (record.Round < 1 || record.StartedAt == "") {
		return "", run.WaiterTarget{}, false
	}
	current := run.WaiterTarget{OperationID: record.OperationID, Round: record.Round, StartedAt: record.StartedAt}
	if preRunning {
		current = row.Target
	}
	if current == row.Target || (row.Target.OperationID == current.OperationID && row.Target.Round == 0 &&
		row.Target.StartedAt == "" && current.Round > 0 && current.StartedAt != "" && record.Status == "running") {
		return record.GoalID, current, true
	}
	return "", run.WaiterTarget{}, false
}

func (s *Store) pendingWaitRun(id string, target run.WaiterTarget) (string, run.WaiterTarget, bool) {
	record, err := (&run.Store{Root: s.Root}).Read(id)
	if err != nil || record == nil || record.GoalId == "" ||
		(record.Status != run.StatusLaunching && record.Status != run.StatusRunning && record.Status != run.StatusDraining) {
		return "", run.WaiterTarget{}, false
	}
	current := run.WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce}
	return record.GoalId, current, current == target
}

func (s *Store) pendingWaitAttempt(row run.Waiter) (string, string, run.WaiterTarget, bool) {
	attempt, err := proofrun.ReadAttempt(s.Root, row.TargetID)
	if err != nil || attempt.Terminal != nil {
		return "", "", run.WaiterTarget{}, false
	}
	if current := (run.WaiterTarget{ProofDigest: attempt.ProofIdentity.IdentityDigest}); current != row.Target {
		return "", "", run.WaiterTarget{}, false
	}
	if attempt.ReservationOwner == nil {
		return attempt.GoalID, "", run.WaiterTarget{}, true
	}
	owner := attempt.ReservationOwner
	current := run.WaiterTarget{Generation: owner.RunGeneration, LaunchNonce: owner.LaunchNonce}
	goalID, observed, ok := s.pendingWaitRun(owner.RunID, current)
	if !ok || goalID != attempt.GoalID {
		return "", "", run.WaiterTarget{}, false
	}
	return attempt.GoalID, owner.RunID, observed, true
}

func (s *Store) brainSummary(scan ScanResult, state brain.ReadResult) []string {
	if state.State == brain.Undeclared {
		return nil
	}
	var claimed, held, approved []string
	machine, _ := ResolveMachine(s.Root)
	if endpoint, err := ResolveEndpoint(s.Root); err == nil {
		if projection, projectErr := Project(endpoint, false, s.now()); projectErr == nil && projection.Tree != nil {
			for id, item := range projection.Tree.Live {
				if item.Claimed != nil {
					claimed = append(claimed, id)
					if item.Claimed.Machine == machine {
						held = append(held, id)
					}
				} else if item.Approved != nil {
					approved = append(approved, id)
				}
			}
		}
	}
	sort.Strings(claimed)
	sort.Strings(held)
	sort.Strings(approved)
	questionIDs := itemIDs(scan.Questions)
	draftIDs := itemIDs(scan.Drafts)
	line := fmt.Sprintf("BRAIN SEAT: nodes hold %d claims (%s); %d approved goals await a node; %d asks await Wido (%s); %d drafts await approval (%s)",
		len(claimed), displayIDs(claimed), len(approved), len(questionIDs), displayIDs(questionIDs), len(draftIDs), displayIDs(draftIDs))
	lines := []string{line}
	if state.State == brain.Corrupt {
		lines = append(lines, brain.RemedialRefusal(state.Reason, s.Root))
	}
	if len(held) > 0 {
		lines = append(lines, "HELD HERE: "+strings.Join(held, ", ")+"; release them to a node")
	}
	return lines
}

func itemIDs(items []Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	sort.Strings(ids)
	return ids
}

func displayIDs(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

type infrastructureFailure struct {
	component string
	err       error
}

func (failure infrastructureFailure) Error() string {
	return infrastructureProducerPrefixes[failure.component] + ": " + failure.err.Error()
}

var infrastructureProducerPrefixes = map[string]string{
	"state-root": "state root", "goal-fence": "goal fence", "verdict-state": "turn verdict state",
	"verdict-state-lock": "turn verdict state lock", "status-write": "status write", "session-stop-marker": "session stop marker",
	"handoff-record": "handoff record",
}

func infrastructureVerdict(component string, err error) Verdict {
	prefix := infrastructureProducerPrefixes[component]
	detail := prefix + ": " + err.Error()
	return Verdict{
		SchemaVersion: 1, Class: "infrastructure", CauseCode: slugStopCause(prefix), Component: component,
		ShouldBlock: false, LedgerStatus: "degraded", Diagnostics: []string{detail}, Display: detail,
	}
}

func slugStopCause(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	dash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			dash = false
		} else if out.Len() > 0 && !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	result := strings.Trim(out.String(), "-")
	return strings.TrimPrefix(result, "the-")
}

// enforceIdleBacklog owns the agent-path causal invariant. Without a valid
// attended-human authorization, a failed fresh read and claimable unattended
// backlog both block the stop.
const unreadableIdleBacklogDigest = "ledger-unreadable"

func (s *Store) enforceIdleBacklog(verdict *Verdict, work *ClaimableBudgetedWork, workErr error, session *sessionState, sessionID, mainID string, options TurnVerdictOptions) {
	s.enforceIdleBacklogWithWaits(verdict, work, workErr, session, sessionID, mainID, options, nil)
}

func (s *Store) enforceIdleBacklogWithWaits(verdict *Verdict, work *ClaimableBudgetedWork, workErr error, session *sessionState, sessionID, mainID string, options TurnVerdictOptions, waits registeredWaits) {
	blockedBeforeIdle := verdict.ShouldBlock
	if workErr != nil {
		verdict.Class = "idle-with-backlog"
		verdict.IdleRefusal = true
		verdict.CountSpent = true
		if session.IdleBlockDigest != unreadableIdleBacklogDigest {
			session.IdleBlockDigest = unreadableIdleBacklogDigest
			if options.StopHookActive && session.IdleBlocks > 0 {
				session.IdleBlocks++
			} else {
				session.IdleBlocks = 1
			}
		} else if options.StopHookActive {
			session.IdleBlocks++
		}
		if session.IdleBlocks >= 3 {
			s.escalateIdleBacklog(verdict, session, sessionID, mainID, "", false, blockedBeforeIdle, options,
				"the fresh canonical ledger read failed, so no continuation goal could be identified: "+workErr.Error())
			return
		}
		verdict.ShouldBlock = true
		source := "uncertainty"
		verdict.BlockSource = &source
		detail := "IDLE WITH BACKLOG cannot be ruled out: the fresh canonical ledger read failed: " + workErr.Error()
		detail += fmt.Sprintf("; refusal %d of 3 for the unreadable ledger; stop_hook_active=%t; at 3 the refusal is recorded, the human idle alarm is raised, and the turn ends", session.IdleBlocks, options.StopHookActive)
		verdict.Diagnostics = append(verdict.Diagnostics, detail)
		verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
		return
	}
	if work == nil {
		return
	}
	if len(work.Refused) > 0 {
		detail := fmt.Sprintf("CLAIM WOULD REFUSE: %s: %s", work.Refused[0].GoalID, work.Refused[0].Cause)
		if remaining := len(work.Refused) - 1; remaining > 0 {
			detail += fmt.Sprintf(" (and %d more)", remaining)
		}
		verdict.Diagnostics = append(verdict.Diagnostics, detail)
		verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
	}
	digest := idleBacklogDigest(*work)
	if len(work.Claimable) == 0 || work.HasDelegateJobInFlight() || waits.hasWorkInFlight() {
		session.IdleBlockDigest = digest
		session.IdleBlocks = 0
		return
	}
	verdict.IdleRefusal = true
	verdict.Class = "idle-with-backlog"
	verdict.CountSpent = true
	if digest == session.IdleBlockDigest {
		session.IdleBlocks++
	} else {
		session.IdleBlockDigest = digest
		session.IdleBlocks = 1
	}
	if session.IdleBlocks >= 3 {
		goalID := work.Claimable[0]
		claimNeeded := true
		// A breach-stopped goal is waiting on a human and must not keep the
		// machine from taking the next item; Next excludes it from Claimed.
		if len(work.Claimed) > 0 {
			goalID = work.Claimed[0]
			claimNeeded = false
		}
		s.escalateIdleBacklog(verdict, session, sessionID, mainID, goalID, claimNeeded, blockedBeforeIdle, options, "")
		return
	}
	verdict.ShouldBlock = true
	source := "idle-backlog"
	verdict.BlockSource = &source
	countText := fmt.Sprintf("refusal %d of 3 for this unchanged backlog; at 3 request steward continuation and allow Stop unless another branch blocks", session.IdleBlocks)
	verdict.Display = strings.TrimSpace(verdict.Display + "\n" + fmt.Sprintf(
		"IDLE WITH BACKLOG: %d claimable goals await a live claim or job: %s; %s; stop_hook_active=%t; an attended human may run `metasystem session stop --by <name>`",
		len(work.Claimable), idleBacklogNames(*work), countText, options.StopHookActive))
}

func idleBacklogNames(work ClaimableBudgetedWork) string {
	limit := min(5, len(work.Claimable))
	names := append([]string(nil), work.Claimable[:limit]...)
	display := strings.Join(names, ", ")
	if remaining := len(work.Claimable) - len(names); remaining > 0 {
		display += fmt.Sprintf(" and %d more (metasystem goal list names them all)", remaining)
	}
	return display
}

func idleBacklogDigest(work ClaimableBudgetedWork) string {
	claimable := append([]string(nil), work.Claimable...)
	claimed := append([]string(nil), work.Claimed...)
	jobs := append([]string(nil), work.NonTerminalJobs...)
	sort.Strings(claimable)
	sort.Strings(claimed)
	sort.Strings(jobs)
	parts := []string{
		"claimable\n" + strings.Join(claimable, "\n"),
		"claimed\n" + strings.Join(claimed, "\n"),
		"non-terminal-jobs\n" + strings.Join(jobs, "\n"),
	}
	return sha256Hex([]byte(strings.Join(parts, "\n--\n")))
}

func (s *Store) escalateIdleBacklog(verdict *Verdict, session *sessionState, sessionID, mainID, goalID string, claimNeeded, blockedBeforeIdle bool, options TurnVerdictOptions, unavailable string) {
	escalationRepair := unavailable != ""
	if unavailable == "" && s.ResolveIdleSeat != nil {
		actor, epoch, err := s.ResolveIdleSeat()
		if err != nil {
			options.SeatActorProblem = err.Error()
			escalationRepair = true
		} else {
			options.SeatActor = actor
			options.SeatClaimEpoch = epoch
		}
	}
	event := IdleEscalationEvent{
		SessionID: sessionID, MainID: mainID, GoalID: goalID,
		BacklogDigest: session.IdleBlockDigest, Refusal: session.IdleBlocks,
		StopHookActive: options.StopHookActive, ClaimActor: options.SeatActor,
		ClaimNeeded: claimNeeded, SeatClaimEpoch: options.SeatClaimEpoch,
	}
	if unavailable != "" {
		event.ClaimDetail = unavailable
	} else if options.SeatActorProblem != "" {
		event.ClaimDetail = options.SeatActorProblem
	} else if options.SeatActor.Machine == "" || options.SeatActor.Lineage == "" || options.SeatClaimEpoch < 1 {
		event.ClaimDetail = "the announced seat actor or its positive checkout lease epoch was unavailable"
	} else if claimNeeded {
		event.ClaimDetail = "claim deferred to the steward tick"
	} else {
		event.ClaimDetail = "goal is already claimed by this machine; no steward claim is needed"
	}

	if unavailable == "" && options.SeatActorProblem == "" &&
		options.SeatActor.Machine != "" && options.SeatActor.Lineage != "" && options.SeatClaimEpoch > 0 {
		if s.PrepareIdleContinuation == nil {
			event.IntentDetail = "the steward continuation preparation seam is unavailable"
			escalationRepair = true
		} else if nonce, err := s.PrepareIdleContinuation(event); err != nil {
			event.IntentDetail = err.Error()
			escalationRepair = true
		} else {
			event.IntentID = nonce
			event.IntentPrepared = true
			event.IntentDetail = "prepared steward continuation intent " + nonce
		}
	} else {
		event.IntentDetail = "no steward continuation intent was prepared because its goal and seat actor could not be established"
		escalationRepair = true
	}

	incidentDetail := ""
	incidentID := ""
	if s.RecordIdleIncident == nil {
		incidentDetail = "the steward incident recorder is unavailable"
		escalationRepair = true
	} else if id, err := s.RecordIdleIncident(event); err != nil {
		incidentDetail = "the steward incident could not be recorded: " + err.Error()
		escalationRepair = true
	} else {
		incidentID = id
		incidentDetail = "recorded steward alert episode " + id
	}
	alarmDetail := "the human idle alarm was not raised because the steward intent was prepared"
	if !event.IntentPrepared {
		if s.RaiseIdleAlarm == nil {
			alarmDetail = "the human idle alarm could not be queued because its steward seam is unavailable"
			escalationRepair = true
		} else if err := s.RaiseIdleAlarm(event); err != nil {
			alarmDetail = "the human idle alarm could not be queued: " + err.Error()
			escalationRepair = true
		} else {
			alarmDetail = "queued the steward's existing human idle alarm because a steward intent could not be prepared"
		}
	}

	if !blockedBeforeIdle {
		verdict.ShouldBlock = false
		verdict.BlockSource = nil
	} else {
		verdict.IdleRefusal = false
	}
	detail := fmt.Sprintf("IDLE WITH BACKLOG: refusal %d reached the bound of 3 for this unchanged backlog; ", session.IdleBlocks)
	if event.IntentPrepared && claimNeeded {
		detail += fmt.Sprintf("selected goal %s and deferred its claim as %s to the steward tick; ", goalID, options.SeatActor.historyActor())
	} else if event.IntentPrepared {
		detail += fmt.Sprintf("selected this machine's held goal %s as %s without another claim attempt; ", goalID, options.SeatActor.historyActor())
	} else if goalID == "" {
		detail += "could not identify a continuation goal: " + event.ClaimDetail + "; "
	} else {
		detail += fmt.Sprintf("could not hand goal %s to the steward: %s; ", goalID, event.ClaimDetail)
	}
	if event.IntentPrepared {
		detail += "prepared steward continuation intent " + event.IntentID + "; "
	} else {
		detail += "could not prepare a steward continuation: " + event.IntentDetail + "; "
	}
	detail += incidentDetail + "; " + alarmDetail
	if blockedBeforeIdle {
		detail += "; another turn-verdict branch remains blocking this stop"
	} else {
		detail += "; the turn will end"
	}
	detail += fmt.Sprintf("; stop_hook_active=%t", options.StopHookActive)
	verdict.escalation = &TurnEscalationFacts{
		IntentId:          event.IntentID,
		IncidentId:        incidentID,
		AlarmDetail:       alarmDetail,
		Detail:            detail,
		IntentPrepared:    event.IntentPrepared,
		SupervisionRepair: escalationRepair,
	}
	verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
}

// decideRuns applies the monitor facility's rules OUTSIDE the ladder:
// run warnings always surface, and unwatched work blocks once — BEFORE
// Busy can suppress anything (a watched active run is Busy; an unwatched
// one blocks despite being busy, which is the point).
func (s *Store) decideRuns(verdict *Verdict, scan ScanResult, session *sessionState, mainId string, watches authenticatedWatches) runDisplayLines {
	var warnings runDisplayLines
	warn := func(class runWarningClass, id, format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		warnings.lines = append(warnings.lines, line)
		warnings.warnings = append(warnings.warnings, runWarning{class: class, id: id, line: line})
	}
	continuation := func(text string) string {
		if text == "" {
			return "no continuation recorded"
		}
		return text
	}
	for _, runFact := range scan.Runs {
		switch {
		case runFact.Status == "red" && !runFact.Acked:
			warn(runWentRed, runFact.Id, "run %s went red; the run record says: %s", runFact.Id, continuation(runFact.ExpectRed))
		case (runFact.Status == "ended-unknown" || runFact.Status == "launch-failed") && !runFact.Acked:
			warn(runEndedUnknown, runFact.Id, "run %s ended %s; the run record says: %s", runFact.Id, runFact.Status, continuation(runFact.ExpectUnknown))
		case runFact.Hung:
			warn(runLooksHung, runFact.Id, "run %s looks hung; the run record says: %s", runFact.Id, continuation(runFact.ExpectHung))
		case runFact.ProbeState == "unknown":
			warn(runLivenessUnknown, runFact.Id, "run %s liveness unknown", runFact.Id)
		case (runFact.Status == "launching" || runFact.Status == "running" || runFact.Status == "draining") && !runFact.Supervised:
			warn(runUnsupervised, runFact.Id, "supervision is not scanning run %s", runFact.Id)
		}
	}
	warnings.lines = append(warnings.lines, scan.RunUnreadable...)
	warnings.unreadable = append(warnings.unreadable, scan.RunUnreadable...)
	verdict.Diagnostics = append(verdict.Diagnostics, scan.RunUnreadable...)

	// The unwatched-work block: lifecycle-tagged keys.
	var unwatchedTags, unwatchedIds []string
	for _, job := range scan.Jobs {
		if mainId != "" && job.MainId == mainId &&
			(job.Status == "pending" || job.Status == "running") && !watches.watchesJob(job.Id) {
			unwatchedTags = append(unwatchedTags, "job:"+job.Id+"@"+job.StartedAt)
			unwatchedIds = append(unwatchedIds, job.Id)
		}
	}
	for _, runFact := range scan.Runs {
		inFlight := runFact.Status == "launching" || runFact.Status == "running" || runFact.Status == "draining"
		// A main owns its own runs; a HUMAN caller (empty mainId) owns
		// human-launched runs (null coordinates) — the waiter side already
		// keys humans on the OS user id.
		owned := runFact.MainId == mainId
		if owned && inFlight && !watches.watchesRun(runFact) {
			unwatchedTags = append(unwatchedTags, fmt.Sprintf("run:%s.g%d.%s", runFact.Id, runFact.Generation, runFact.Nonce))
			unwatchedIds = append(unwatchedIds, runFact.Id)
		}
	}
	if len(unwatchedTags) > 0 {
		sort.Strings(unwatchedTags)
		digest := sha256Hex([]byte(strings.Join(unwatchedTags, "\n")))
		if !contains(session.BlockedUnwatchedDigests, digest) {
			session.BlockedUnwatchedDigests = appendCapped(session.BlockedUnwatchedDigests, digest, maxFreeDigests)
			verdict.ShouldBlock = true
			source := "unwatched-work"
			verdict.BlockSource = &source
			warnings.actionable = append(warnings.actionable, fmt.Sprintf(
				"work you launched is unwatched: %s; arm the printed watch command or conclude the runs",
				strings.Join(unwatchedIds, ", ")))
			warnings.lines = append(warnings.lines, warnings.actionable[len(warnings.actionable)-1])
		}
	}
	return warnings
}

// decideGreens surfaces green terminals exactly once per session on the
// terminal sequence's total order. The greens are RE-READ FROM DISK
// inside this verdict's flock: the scanner's
// ScanResult predates the lock and a stale snapshot could advance the
// cursor past a green it never saw. Any unreadable run record freezes
// the cursor entirely. Crafted scan facts still drive the
// warning paths; the CURSOR trusts only the fresh read.
func (s *Store) decideGreens(scan ScanResult, session *sessionState) []string {
	records, unreadable := (&run.Store{Root: s.Root}).List()
	if len(unreadable) > 0 || len(scan.RunUnreadable) > 0 {
		return nil
	}
	type green struct {
		seq  int64
		line string
	}
	var greens []green
	seen := map[string]bool{}
	for _, record := range records {
		if record.Status != "green" || record.TerminalSeq == nil || *record.TerminalSeq <= session.GreenCursor {
			continue
		}
		text := record.Expect.Green
		if text == "" {
			text = "no continuation recorded"
		}
		greens = append(greens, green{*record.TerminalSeq, fmt.Sprintf(
			"run %s finished green; the run record says: %s", record.RunId, text)})
		seen[record.RunId] = true
	}
	// Crafted scan facts (tests, and any future non-record source) join
	// when the disk read did not already carry them.
	for _, runFact := range scan.Runs {
		if runFact.Status != "green" || runFact.TerminalSeq <= session.GreenCursor || seen[runFact.Id] {
			continue
		}
		text := runFact.ExpectGreen
		if text == "" {
			text = "no continuation recorded"
		}
		greens = append(greens, green{runFact.TerminalSeq, fmt.Sprintf(
			"run %s finished green; the run record says: %s", runFact.Id, text)})
	}
	sort.Slice(greens, func(i, j int) bool { return greens[i].seq < greens[j].seq })
	var lines []string
	for _, g := range greens {
		lines = append(lines, g.line)
		session.GreenCursor = g.seq
	}
	return lines
}

// composeDisplay joins the run prefix, the ladder's display, and the
// green lines, dropping empties.
func composeDisplay(prefix []string, ladder string, greens []string) string {
	var parts []string
	parts = append(parts, prefix...)
	if ladder != "" {
		parts = append(parts, ladder)
	}
	parts = append(parts, greens...)
	return strings.Join(parts, "\n")
}

// FencedClaimLines renders breach-stopped claims without treating them as
// work this machine can continue before the human resumes them.
func FencedClaimLines(files []*GoalFile) []string {
	lines := make([]string, 0, len(files))
	for _, file := range files {
		// A breach-stopped goal is waiting on a human and must not keep the
		// machine from taking the next item.
		if !file.IsFencedClaim() {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"FENCED %s: breach-stopped by %s (%s); only goal resume, a human act, clears it; the queue is open",
			file.Id, file.StopFence.StopID, file.StopFence.Reason,
		))
	}
	return lines
}

// LandingClaimLines renders claims waiting to land (goal land-ready) as live
// work for the landing, never as idleness and never as a claim the machine
// must continue before the next item. Past its elapsed box the wait is
// printed as overdue; the box's elapsed fence does not close on it.
func LandingClaimLines(files []*GoalFile, now time.Time) []string {
	lines := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsLandingClaim() {
			continue
		}
		if LandingOverdue(file, now) {
			lines = append(lines, fmt.Sprintf(
				"LANDING OVERDUE %s: land-ready since %s and past its elapsed box; land it; the queue is open",
				file.Id, file.Landing.At,
			))
			continue
		}
		lines = append(lines, fmt.Sprintf("LANDING %s: land-ready since %s; the queue is open", file.Id, file.Landing.At))
	}
	return lines
}

// LandingOverdue reports a landing claim whose episode clock, less its idle
// seconds, has passed the budget's elapsed limit. It reads the claim's own
// episode start; a consumed discharge that advanced dispatch's start only
// makes this earlier, never later, so it is a floor on the wait.
func LandingOverdue(file *GoalFile, now time.Time) bool {
	if !file.IsLandingClaim() || file.Budget == nil {
		return false
	}
	startText := file.Claimed.EpisodeAt
	if file.Claimed.EpisodeRevision == 0 {
		startText = file.Claimed.At
	}
	start, err := time.Parse(time.RFC3339, startText)
	if err != nil {
		return false
	}
	elapsed := now.Sub(start) - time.Duration(file.Claimed.IdleSeconds)*time.Second
	return elapsed >= file.Budget.ElapsedDuration()
}

// OnlyFencedClaim reports the one held claim that cannot be live work until
// the human resumes it.
func (w ClaimableBudgetedWork) OnlyFencedClaim() (*GoalFile, bool) {
	if len(w.Claimed) != 0 || len(w.fencedClaims) != 1 || !w.fencedClaims[0].IsFencedClaim() {
		return nil, false
	}
	return w.fencedClaims[0], true
}

// decide is the precedence ladder from the design, in order.
func (s *Store) decide(verdict *Verdict, scan ScanResult, session *sessionState, work *ClaimableBudgetedWork, brainSeat bool, waits registeredWaits) {
	for _, item := range scan.Open {
		verdict.OpenWork = append(verdict.OpenWork, item.Detail)
	}
	verdict.OpenWorkSignature = scan.OpenWorkSignature()
	verdict.Diagnostics = append(verdict.Diagnostics, scan.Unreadable...)

	facts, status, statusLine := s.goalFacts()
	verdict.LedgerStatus = status
	verdict.Goal = facts

	// An unwatched-work block from decideRuns holds the turn already;
	// the ladder still composes its display but must not overwrite the
	// block source or re-block.
	alreadyBlocked := verdict.ShouldBlock

	var display []string
	for _, warning := range scan.OpenWorkWarnings {
		display = append(display, warning)
		verdict.Diagnostics = append(verdict.Diagnostics, warning)
	}
	for _, item := range scan.TemplateUnfilled {
		display = append(display, item.Detail)
		verdict.Diagnostics = append(verdict.Diagnostics, item.Detail)
	}
	blockGoal := func(reason string) {
		if alreadyBlocked {
			display = append(display, reason)
			return
		}
		verdict.ShouldBlock = true
		source := "goal"
		verdict.BlockSource = &source
		display = append(display, reason)
	}

	switch {
	case len(scan.Busy) > 0:
		// An active checkout needs no prodding: no goal clause, no block.
		var names []string
		for _, item := range scan.Busy {
			names = append(names, item.Detail)
		}
		display = append(display, "STILL WORKING: "+strings.Join(names, "; "))

	case len(scan.Open) > 0 && waits.suppressOpenWork(verdict.OpenWorkSignature, goalIDOf(facts), scan.WaitingOnHuman):
		display = append(display, fmt.Sprintf("OPEN WORK (%d): %s", len(scan.Open), strings.Join(verdict.OpenWork, "; ")))

	case len(scan.Open) > 0:
		// Open work blocks once per durable plan-line marker. The session
		// signature protects direct callers that supply unmarked scan facts;
		// marked plan lines do not receive another refusal in any session.
		if verdict.OpenWorkSignature != "" && session.OpenWorkSignature != verdict.OpenWorkSignature {
			session.OpenWorkSignature = verdict.OpenWorkSignature
			if !alreadyBlocked {
				verdict.ShouldBlock = true
				source := "open-work"
				verdict.BlockSource = &source
			}
		}
		display = append(display, fmt.Sprintf("OPEN WORK (%d): %s", len(scan.Open), strings.Join(verdict.OpenWork, "; ")))

	case len(scan.WaitingOnHuman) > 0:
		// A human-blocked checkout must not be handed a contradictory
		// imperative: the wait is reported, the goal clause suppressed.
		var names []string
		for _, item := range scan.WaitingOnHuman {
			names = append(names, item.Detail)
		}
		display = append(display, "WAITING ON THE HUMAN: "+strings.Join(names, "; "))

	case len(scan.Unreadable) > 0:
		// Unreadable vetoes BOTH outcomes: no all-clear over unread
		// inputs, no goal prod over files that may hold open work.
		display = append(display, fmt.Sprintf("%d inputs unreadable: %s", len(scan.Unreadable), strings.Join(scan.Unreadable, "; ")))

	default:
		if brainSeat {
			break
		}
		// The scanner reports nothing at all: the goal has the floor.
		switch status {
		case "ok":
			if work != nil && len(work.Claimable) > 0 && waits.hasWorkInFlight() {
				detail := "WORK IN FLIGHT: a registered wait is joined to the claimed goal"
				if work != nil && len(work.Claimable) > 0 {
					detail += "; claimable shared backlog also includes " + strings.Join(work.Claimable, ", ")
				}
				display = append(display, detail)
				break
			}
			if work != nil && work.HasDelegateJobInFlight() && len(work.Claimable) > 0 {
				display = append(display, "WORK IN FLIGHT: a non-terminal delegate job is joined to a live process; claimable shared backlog also includes "+strings.Join(work.Claimable, ", "))
				break
			}
			if !contains(session.BlockedGoalRevisions, facts.Revision) {
				session.BlockedGoalRevisions = appendCapped(session.BlockedGoalRevisions, facts.Revision, maxGoalRevisions)
				blockGoal("open work is done; the goal file names the next step: " + facts.NextStep)
			} else {
				display = append(display, "NOTHING LEFT TO WORK ON; the current goal is "+facts.Id+" ("+facts.NextStep+")")
			}
			if first, digest := s.queuedFrontier(); digest != "" {
				if session.ObservedQueueDigest == "" {
					session.ObservedQueueDigest = digest
				} else if digest != session.ObservedQueueDigest {
					session.ObservedQueueDigest = digest
					queueNow := "is now empty"
					if first != "" {
						queueNow = "now starts with " + first
					}
					blockGoal(fmt.Sprintf("the shared goal queue changed while %s remains claimed here; it %s", facts.Id, queueNow))
				}
			}
			if work != nil {
				display = append(display, FencedClaimLines(work.fencedClaims)...)
				display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
			}
		case "queued-only":
			first, _ := s.queuedFrontier()
			if first == "" {
				display = append(display, "no goal is claimed here and the queue is empty; a person opens the next goal (`goal open --origin human`); a seat opens only the blocker of its claimed goal (`--blocks`, R-93-m1e)")
				if work != nil {
					display = append(display, FencedClaimLines(work.fencedClaims)...)
					display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
				}
				break
			}
			display = append(display, "no current goal; the queue holds "+first)
			if work != nil {
				display = append(display, FencedClaimLines(work.fencedClaims)...)
				display = append(display, LandingClaimLines(work.landingClaims, s.now())...)
			}
		case "goal-free":
			fresh, digest, declared := s.freeState()
			if fresh {
				display = append(display, "NOTHING LEFT TO WORK ON; goal-free declared "+declared)
			} else if !contains(session.BlockedFreeDigests, digest) {
				session.BlockedFreeDigests = appendCapped(session.BlockedFreeDigests, digest, maxFreeDigests)
				blockGoal("the goal-free declaration predates new work; declare a goal or renew with `goal declare-free`")
			} else {
				display = append(display, "the goal-free declaration is stale (already surfaced); renew with `goal declare-free`")
			}
		case "absent":
			// Advisory, never degraded: the pre-adoption installation.
			display = append(display, "NOTHING LEFT TO WORK ON; no goal ledger; `goal open` starts one")
		default: // degraded
			display = append(display, "goal ledger degraded: "+statusLine+" — the all-clear is withheld")
		}
	}

	if status == "degraded" && !verdict.ShouldBlock && len(scan.Busy) == 0 && (len(scan.Open) > 0 || len(scan.WaitingOnHuman) > 0) {
		display = append(display, "goal ledger degraded: "+statusLine)
	}
	display = append(display, waits.lines()...)
	verdict.Display = strings.Join(display, "\n")
}

// goalFacts reads the goal world and classifies it for the verdict,
// routed on the checkout's world: a converted checkout judges from the
// synced projection by this machine's enrolled nickname, a legacy
// checkout from the single file. The vocabulary is shared — ok,
// queued-only, goal-free, absent, degraded — so every verdict rule
// downstream is world-neutral.
func (s *Store) goalFacts() (*GoalFacts, string, string) {
	if NewWorld(s.Root) {
		return s.convertedGoalFacts()
	}
	return s.legacyGoalFacts()
}

// convertedGoalFacts maps the synced world onto the verdict's
// vocabulary: this machine's claimed goal plays the Current role, the
// root record's declaration plays goal-free, and a queue nobody here
// claimed plays queued-only.
func (s *Store) convertedGoalFacts() (*GoalFacts, string, string) {
	machine, err := ResolveMachine(s.Root)
	if err != nil {
		return nil, "degraded", err.Error()
	}
	endpoint, err := ResolveEndpoint(s.Root)
	if err != nil {
		return nil, "degraded", err.Error()
	}
	proj, err := Project(endpoint, false, s.now())
	if err != nil {
		return nil, "degraded", err.Error()
	}
	if proj.Tree == nil {
		return nil, "degraded", "the accepted tree is unreadable"
	}
	if f := currentClaimOf(proj.Tree, machine); f != nil {
		return &GoalFacts{
			Id:       f.Id,
			Intent:   f.Intent,
			NextStep: f.NextStep,
			Revision: fmt.Sprintf("%s@%d", f.Id, f.Revision),
		}, "ok", ""
	}
	if proj.Tree.Root != nil && proj.Tree.Root.Free != nil {
		return nil, "goal-free", ""
	}
	return nil, "queued-only", ""
}

func (s *Store) legacyGoalFacts() (*GoalFacts, string, string) {
	ledger, problems, err := s.ReadLedger()
	if err != nil {
		return nil, "degraded", err.Error()
	}
	if ledger == nil {
		if s.BaselinePresent() {
			return nil, "degraded", "goals.md was deleted after adoption; run `goal reconcile` to restore from baseline"
		}
		return nil, "absent", ""
	}
	if len(problems) > 0 {
		return nil, "degraded", string(problems[0])
	}
	if !s.BaselineMatches() {
		return nil, "degraded", "the ledger differs from the accepted baseline; run `goal reconcile`"
	}
	switch {
	case ledger.Current != nil:
		return &GoalFacts{
			Id:       ledger.Current.Id,
			Intent:   ledger.Current.Intent,
			NextStep: ledger.Current.NextStep,
			Revision: ledger.Revision(),
		}, "ok", ""
	case ledger.Free != nil:
		return nil, "goal-free", ""
	default:
		return nil, "queued-only", ""
	}
}

// queuedFrontier names the first awaiting or approved goal and a stable
// digest of that whole backlog frontier.
func (s *Store) queuedFrontier() (first, digest string) {
	if NewWorld(s.Root) {
		endpoint, err := ResolveEndpoint(s.Root)
		if err != nil {
			return "", ""
		}
		proj, err := Project(endpoint, false, s.now())
		if err != nil || proj.Tree == nil {
			return "", ""
		}
		type row struct {
			id  string
			rev uint64
		}
		var rows []row
		for _, id := range OrderedOpenGoalIDs(proj.Tree.Live) {
			f := proj.Tree.Live[id]
			if f.State == StateQueued || f.State == StateApproved {
				rows = append(rows, row{id, f.Revision})
			}
		}
		if len(rows) == 0 {
			return "", sha256Hex(nil)
		}
		var lines []string
		for _, r := range rows {
			lines = append(lines, fmt.Sprintf("%s@%d", r.id, r.rev))
		}
		return rows[0].id, sha256Hex([]byte(strings.Join(lines, "\n")))
	}
	ledger, _, _ := s.ReadLedger()
	if ledger == nil {
		return "", ""
	}
	if len(ledger.Queued) == 0 {
		return "", sha256Hex(nil)
	}
	return ledger.Queued[0].Id, ledger.QueuedDigest()
}

// freeState reads the goal-free declaration's freshness, routed on the
// world: both worlds compare the recorded digest against the current
// plans-stream scan, because a declaration is only as good as the world
// it described.
func (s *Store) freeState() (fresh bool, digest, declared string) {
	if NewWorld(s.Root) {
		endpoint, err := ResolveEndpoint(s.Root)
		if err != nil {
			return false, "", ""
		}
		proj, err := Project(endpoint, false, s.now())
		if err != nil || proj.Tree == nil || proj.Tree.Root == nil || proj.Tree.Root.Free == nil {
			return false, "", ""
		}
		scan, err := ScanDigest(s.Root)
		if err != nil {
			return false, scan, proj.Tree.Root.Free.Declared
		}
		return scan == proj.Tree.Root.Free.Digest, scan, proj.Tree.Root.Free.Declared
	}
	ledger, _, _ := s.ReadLedger()
	fresh, digest = s.freeIsFresh(ledger)
	declared = ""
	if ledger != nil && ledger.Free != nil {
		declared = ledger.Free.Declared
	}
	return fresh, digest, declared
}

// freeIsFresh recomputes the plans-stream digest against the declaration.
func (s *Store) freeIsFresh(ledger *Ledger) (bool, string) {
	if ledger == nil || ledger.Free == nil {
		return false, ""
	}
	digest, err := ScanDigest(s.Root)
	if err != nil {
		return false, digest
	}
	return digest == ledger.Free.Digest, digest
}

// watchdog runs the exactly-once surface protocol on one session's slot.
func (st *sessionState) watchdog(digest string) bool {
	if digest == "" {
		st.WatchdogSurfaced = nil
		return false
	}
	if st.WatchdogSurfaced != nil && *st.WatchdogSurfaced == digest {
		return false
	}
	st.WatchdogSurfaced = &digest
	return true
}

// loadVerdictState reads the state file. Absence is the initial empty state;
// unreadable or malformed bytes are uncertainty and must block the Stop.
func (s *Store) loadVerdictState() (*verdictState, error) {
	state := &verdictState{SchemaVersion: 1, Sessions: map[string]*sessionState{}}
	data, err := os.ReadFile(statePath(s.Root))
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("turn verdict state is malformed: %w", err)
	}
	if state.SchemaVersion != 1 || state.Sessions == nil {
		return nil, fmt.Errorf("turn verdict state has an invalid schema")
	}
	return state, nil
}

// touch returns the session's entry, pruning expired sessions and
// evicting oldest-lastTouched beyond the map cap.
func (state *verdictState) touch(sessionId, now string) *sessionState {
	cutoff := isoDaysBefore(now, sessionRetainDays)
	for id, session := range state.Sessions {
		if session.LastTouched < cutoff {
			delete(state.Sessions, id)
		}
	}
	session, ok := state.Sessions[sessionId]
	if !ok {
		session = &sessionState{}
		state.Sessions[sessionId] = session
	}
	session.LastTouched = now
	for len(state.Sessions) > maxSessions {
		oldestId, oldest := "", ""
		for id, entry := range state.Sessions {
			if id == sessionId {
				continue
			}
			if oldest == "" || entry.LastTouched < oldest {
				oldest, oldestId = entry.LastTouched, id
			}
		}
		if oldestId == "" {
			break
		}
		delete(state.Sessions, oldestId)
	}
	return session
}

func (s *Store) saveVerdictState(state *verdictState) error {
	data, err := json.MarshalIndent(state, "", " ")
	if err != nil {
		return err
	}
	return verdictStateWriter(statePath(s.Root), append(data, '\n'))
}

var verdictStateWriter = atomicWrite

var brainStatusWriter = brain.WriteStatus

var consumeSessionStopForVerdict = func(store *Store, sessionID, mainID string) (SessionStop, bool, string, error) {
	return store.consumeSessionStop(sessionID, mainID)
}

// isoDaysBefore subtracts days from an ISO stamp lexically-safely.
func isoDaysBefore(iso string, days int) string {
	t, err := parseISO(iso)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -days).UTC().Format("2006-01-02T15:04:05Z")
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// appendCapped appends FIFO-evicting beyond the cap.
func appendCapped(list []string, v string, cap int) []string {
	list = append(list, v)
	if len(list) > cap {
		list = list[len(list)-cap:]
	}
	return list
}

func parseISO(iso string) (t time.Time, err error) {
	return time.Parse("2006-01-02T15:04:05Z", iso)
}
