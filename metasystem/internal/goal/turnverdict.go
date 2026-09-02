package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// The turn verdict: one verb, one structured decision. The scanner fills ScanResult (the
// verdict's input contract — internal/report imports this package, never
// the reverse), the verdict decides, and the one capped, pruned, flocked
// state file is the ONLY Stop-state on disk.

// Item is one classified scanner fact. Busy items carry the bounded
// display detail the hook renders verbatim (≤200 bytes at construction).
type Item struct {
	Kind   string // job | mission | gate | plan
	Id     string
	Detail string
}

// ScanResult is the verdict's input contract, produced by the scanner.
type ScanResult struct {
	Open           []Item
	WaitingOnHuman []Item
	StalePlans     []Item
	Busy           []Item
	// Unreadable lists every input the scanner could not read — plans,
	// records, markers, enumeration failures, indeterminate runner
	// liveness. Non-empty Unreadable vetoes BOTH the all-clear and any
	// goal block: nothing can be asserted over unread inputs.
	Unreadable []string
	// Jobs and Runs are the monitor facility's typed facts:
	// the unwatched rule, the run warnings, and the green cursor
	// consume these, never the display-shaped Busy items.
	Jobs []JobFact
	Runs []RunFact
	// RunUnreadable is the run readers' own failure channel — surfaced
	// OUTSIDE the ladder so Busy can never hide it, and it freezes the
	// green cursor (the cursor rides only proven-green scans).
	RunUnreadable []string
}

// JobFact is one delegate job's monitor-relevant slice.
type JobFact struct {
	Id         string
	MainId     string
	StartedAt  string
	Status     string
	WaiterLive bool
}

// RunFact is one run record's monitor-relevant slice.
type RunFact struct {
	Id                                                string
	MainId                                            string
	Generation                                        int
	Nonce                                             string
	Status                                            string
	ProbeState                                        string // alive | dead | unknown | ""
	TerminalSeq                                       int64
	Supervised                                        bool
	WaiterLive                                        bool
	Acked                                             bool
	Hung                                              bool
	ExpectGreen, ExpectRed, ExpectHung, ExpectUnknown string
}

// OpenWorkSignature keys the open-work block-once slot: the sorted open
// items' details, hashed.
func (r ScanResult) OpenWorkSignature() string {
	if len(r.Open) == 0 {
		return ""
	}
	lines := make([]string, 0, len(r.Open))
	for _, item := range r.Open {
		lines = append(lines, item.Detail)
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
	ShouldBlock       bool       `json:"shouldBlock"`
	BlockSource       *string    `json:"blockSource"` // "open-work" | "goal" | "idle-backlog" | "uncertainty" | null
	OpenWork          []string   `json:"openWork"`
	OpenWorkSignature string     `json:"openWorkSignature"`
	Goal              *GoalFacts `json:"goal"`
	LedgerStatus      string     `json:"ledgerStatus"`
	Diagnostics       []string   `json:"diagnostics"`
	Display           string     `json:"display"`
	// SurfaceWatchdog answers the hook's --watchdog-surfaced digest: true
	// exactly once per new digest per session, decided under the flock.
	SurfaceWatchdog bool `json:"surfaceWatchdog"`
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
func (s *Store) TurnVerdict(scan ScanResult, sessionId, watchdogDigest, mainId string) (Verdict, error) {
	sessionId = NormalizeSession(sessionId)
	verdict := Verdict{SchemaVersion: 1, LedgerStatus: "ok"}
	resolvedRoot, err := ResolveStateRoot(s.Root)
	if err != nil {
		return failClosedTurnVerdict(err), nil
	}
	s.Root = resolvedRoot

	result, err := s.withLock(func() (Result, error) {
		_, humanAuthorized, markerDetail, err := s.inspectSessionStop(sessionId, mainId)
		if err != nil {
			return Result{}, err
		}
		var work ClaimableBudgetedWork
		var workErr error
		if !humanAuthorized {
			work, workErr = readClaimableBudgetedWork(s.Root, s.now(), s.prober())
		}
		state, err := s.loadVerdictState()
		if err != nil {
			return Result{}, err
		}
		session := state.touch(sessionId, s.nowISO())

		prefix := s.decideRuns(&verdict, scan, session, mainId)
		s.decide(&verdict, scan, session, &work)
		if markerDetail != "" {
			verdict.Diagnostics = append(verdict.Diagnostics, markerDetail)
			verdict.Display = strings.TrimSpace(verdict.Display + "\n" + markerDetail)
		}
		if !humanAuthorized {
			s.enforceIdleBacklog(&verdict, &work, workErr)
		}
		greens := s.decideGreens(scan, session)
		verdict.Display = composeDisplay(prefix, verdict.Display, greens)
		verdict.SurfaceWatchdog = session.watchdog(watchdogDigest)

		if err := s.saveVerdictState(state); err != nil {
			return Result{}, err
		}
		if humanAuthorized {
			marker, consumed, consumeDetail, err := s.consumeSessionStop(sessionId, mainId)
			if err != nil {
				return Result{}, err
			}
			if !consumed {
				if consumeDetail == "" {
					consumeDetail = "the authorization changed before its single use could be recorded"
				}
				return Result{}, fmt.Errorf("session stop authorization could not be completed: %s", consumeDetail)
			}
			if consumeDetail != "" {
				verdict.Diagnostics = append(verdict.Diagnostics, consumeDetail)
				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + consumeDetail)
			}
			verdict.ShouldBlock = false
			verdict.BlockSource = nil
			verdict.Display = strings.TrimSpace(verdict.Display + "\nSESSION STOP authorized once by " + marker.By +
				fmt.Sprintf(" for holder %s at lease epoch %d", marker.HolderMainId, marker.ClaimEpoch))
		}
		return Result{}, nil
	})
	_ = result
	if err != nil {
		return failClosedTurnVerdict(err), nil
	}
	return verdict, nil
}

func failClosedTurnVerdict(err error) Verdict {
	source := "uncertainty"
	detail := "cannot prove that stopping is safe: " + err.Error()
	return Verdict{
		SchemaVersion: 1, ShouldBlock: true, BlockSource: &source,
		LedgerStatus: "degraded", Diagnostics: []string{detail}, Display: detail,
	}
}

// enforceIdleBacklog owns the agent-path causal invariant. Without a valid
// attended-human authorization, a failed fresh read and claimable unattended
// backlog both block the stop.
func (s *Store) enforceIdleBacklog(verdict *Verdict, work *ClaimableBudgetedWork, workErr error) {
	if workErr != nil {
		verdict.ShouldBlock = true
		source := "uncertainty"
		verdict.BlockSource = &source
		detail := "IDLE WITH BACKLOG cannot be ruled out: the fresh canonical ledger read failed: " + workErr.Error()
		verdict.Diagnostics = append(verdict.Diagnostics, detail)
		verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
		return
	}
	if work == nil || len(work.Claimable) == 0 || work.HasDelegateJobInFlight() {
		return
	}
	verdict.ShouldBlock = true
	source := "idle-backlog"
	verdict.BlockSource = &source
	verdict.Display = strings.TrimSpace(verdict.Display + "\n" + fmt.Sprintf(
		"IDLE WITH BACKLOG: claimable goals await a live claim or job: %s; claim or dispatch one, or an attended human may run `metasystem session stop --by <name>`",
		strings.Join(work.Claimable, ", ")))
}

// decideRuns applies the monitor facility's rules OUTSIDE the ladder:
// run warnings always surface, and unwatched work blocks once — BEFORE
// Busy can suppress anything (a watched active run is Busy; an unwatched
// one blocks despite being busy, which is the point).
func (s *Store) decideRuns(verdict *Verdict, scan ScanResult, session *sessionState, mainId string) []string {
	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
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
			warn("run %s went red; the run record says: %s", runFact.Id, continuation(runFact.ExpectRed))
		case (runFact.Status == "ended-unknown" || runFact.Status == "launch-failed") && !runFact.Acked:
			warn("run %s ended %s; the run record says: %s", runFact.Id, runFact.Status, continuation(runFact.ExpectUnknown))
		case runFact.Hung:
			warn("run %s looks hung; the run record says: %s", runFact.Id, continuation(runFact.ExpectHung))
		case runFact.ProbeState == "unknown":
			warn("run %s liveness unknown", runFact.Id)
		case (runFact.Status == "launching" || runFact.Status == "running" || runFact.Status == "draining") && !runFact.Supervised:
			warn("supervision is not scanning run %s", runFact.Id)
		}
	}
	warnings = append(warnings, scan.RunUnreadable...)
	verdict.Diagnostics = append(verdict.Diagnostics, scan.RunUnreadable...)

	// The unwatched-work block: lifecycle-tagged keys.
	var unwatchedTags, unwatchedIds []string
	for _, job := range scan.Jobs {
		if mainId != "" && job.MainId == mainId &&
			(job.Status == "pending" || job.Status == "running") && !job.WaiterLive {
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
		if owned && inFlight && !runFact.WaiterLive {
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
			warnings = append(warnings, fmt.Sprintf(
				"work you launched is unwatched: %s; arm the printed watch command or conclude the runs",
				strings.Join(unwatchedIds, ", ")))
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

// decide is the precedence ladder from the design, in order.
func (s *Store) decide(verdict *Verdict, scan ScanResult, session *sessionState, work *ClaimableBudgetedWork) {
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

	case len(scan.Open) > 0:
		// Open work blocks first — once per signature.
		if session.OpenWorkSignature != verdict.OpenWorkSignature {
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
		// The scanner reports nothing at all: the goal has the floor.
		switch status {
		case "ok":
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
		case "queued-only":
			first, _ := s.queuedFrontier()
			if first == "" {
				display = append(display, "no goal is claimed here and the queue is empty; `goal open` starts one")
				break
			}
			display = append(display, "no current goal; the queue holds "+first)
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
	for id, f := range proj.Tree.Live {
		if f.State == "claimed" && f.Claimed != nil && f.Claimed.Machine == machine {
			return &GoalFacts{
				Id:       id,
				Intent:   f.Intent,
				NextStep: f.NextStep,
				Revision: fmt.Sprintf("%s@%d", id, f.Revision),
			}, "ok", ""
		}
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

// queuedFrontier names the first queued goal and a stable digest of the
// queue, routed on the world: the digest keys the once-per-change block
// the session records, so it must change exactly when the queue does.
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
			id     string
			rev    uint64
			opened string
		}
		var rows []row
		for id, f := range proj.Tree.Live {
			if f.State == "queued" {
				rows = append(rows, row{id, f.Revision, f.OpenedAt})
			}
		}
		if len(rows) == 0 {
			return "", sha256Hex(nil)
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].opened != rows[j].opened {
				return rows[i].opened < rows[j].opened
			}
			return rows[i].id < rows[j].id
		})
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
	return atomicWrite(statePath(s.Root), append(data, '\n'))
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
