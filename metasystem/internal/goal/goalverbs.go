package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionstate"
)

// The verbs: every mutation runs the same spine — bounded flock,
// mission-seat refusal on the recorded active-mission fact, baseline
// discipline (CAS against the accepted bytes), the transition itself, then
// result validation THROUGH THE PARSER (one enforcement point for every
// bound and structural rule), and the ledger-then-baseline write order.
// The authority matrix (holder-only) runs at the command layer exactly
// like every other record-writer path; what arrives here is the distilled
// caller.

// Caller is the classified invoker, distilled at the command layer.
type Caller struct {
	// Class is the lease classification: HUMAN, MAIN, DELEGATE, ...
	Class string
	// Holder reports MAIN-lease holdership (already enforced by the
	// authority matrix; carried for messages).
	Holder bool
}

func (c Caller) origin() string {
	if c.Class == "HUMAN" {
		return OriginHuman
	}
	return OriginMain
}

// Store binds one checkout for verb execution. Prober and Now are seams
// for tests; nil means kernel and wall clock.
type Store struct {
	Root   string
	Prober identity.Prober
	Now    func() time.Time
}

func (s *Store) prober() identity.Prober {
	if s.Prober != nil {
		return s.Prober
	}
	return identity.KernelProber{}
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) nowISO() string {
	return s.now().UTC().Format("2006-01-02T15:04:05Z")
}

// Result is one verb's outcome for stdout.
type Result struct {
	Message string
	Dropped []string // prune's report
}

// baseline is plans/goals-accepted.json: the last accepted ledger's full
// bytes and digest. Full bytes twice over: reconcile's delta replay needs
// them, and deletion recovery restores from them.
type baseline struct {
	SchemaVersion int    `json:"schemaVersion"`
	Ledger        string `json:"ledger"`
	Sha256        string `json:"sha256"`
}

const lockDeadline = 2 * time.Second

// withLock runs fn under the goal ledger's bounded flock. A wedged lock
// refuses in 2 seconds — the installed Stop hook runs a 5-second budget,
// so the verdict degrades rather than hangs.
func (s *Store) withLock(fn func() (Result, error)) (Result, error) {
	path := filepath.Join(s.Root, "artifacts", "agents", "goal.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("goal lock cannot be opened: %w", err)
	}
	defer f.Close()
	deadline := time.Now().Add(lockDeadline)
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return Result{}, fmt.Errorf("goal lock is busy; refusing after %s", lockDeadline)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

// refuseMissionSeat is the recorded-fact seat check (GOAL-21): mutation
// refuses while any mission in this checkout is actively driven, and
// refuses fail-closed when liveness is indeterminate — an unverifiable
// runner cannot authorize rewriting intent.
func (s *Store) refuseMissionSeat() error {
	survey := missionstate.Survey(s.Root, s.prober())
	if active := survey.ActiveMissions(); len(active) > 0 {
		return fmt.Errorf("goal mutation while a mission is active is refused; the mission's intent is the contract — conclude or park mission %s first", active[0].MissionId)
	}
	if survey.Indeterminate() {
		return fmt.Errorf("mission liveness is unknown (%s); refusing goal mutation fail-closed", strings.Join(survey.Unreadable, "; "))
	}
	return nil
}

// ledgerState is what the mutation spine reads before a transition.
type ledgerState struct {
	ledger      *Ledger
	ledgerBytes []byte
	base        *baseline
}

// readState reads the ledger/baseline pair without judging it.
func (s *Store) readState() (ledgerState, error) {
	var state ledgerState
	data, err := os.ReadFile(LedgerPath(s.Root))
	switch {
	case err == nil:
		state.ledgerBytes = data
	case !os.IsNotExist(err):
		return state, err
	}
	raw, err := os.ReadFile(BaselinePath(s.Root))
	switch {
	case err == nil:
		var b baseline
		if err := json.Unmarshal(raw, &b); err != nil {
			return state, fmt.Errorf("goals-accepted.json is unreadable: %v", err)
		}
		state.base = &b
	case !os.IsNotExist(err):
		return state, err
	}
	return state, nil
}

// mutationLedger applies the baseline discipline for a MUTATING verb and
// returns the trusted parsed ledger. genesisOK verbs (open, declare-free)
// may run with neither file present — they create the pair.
func (s *Store) mutationLedger(state ledgerState, genesisOK bool) (*Ledger, error) {
	switch {
	case state.ledgerBytes == nil && state.base == nil:
		if genesisOK {
			return &Ledger{}, nil
		}
		return nil, fmt.Errorf("no goal ledger; `goal open` starts one")
	case state.ledgerBytes != nil && state.base == nil:
		return nil, fmt.Errorf("no accepted baseline; run `goal reconcile` first")
	case state.ledgerBytes == nil && state.base != nil:
		return nil, fmt.Errorf("goals.md was deleted after adoption; run `goal reconcile` to restore from baseline")
	}
	if sha256Hex(state.ledgerBytes) != state.base.Sha256 || string(state.ledgerBytes) != state.base.Ledger {
		return nil, fmt.Errorf("the ledger differs from the accepted baseline (manual edit or interrupted write); run `goal reconcile`")
	}
	ledger, problems := Parse(state.ledgerBytes)
	if len(problems) > 0 {
		return nil, fmt.Errorf("the accepted ledger is malformed (%s); run `goal reconcile`", problems[0])
	}
	return ledger, nil
}

// publish writes the transitioned ledger through the parser (the single
// enforcement point) in ledger-then-baseline order. A crash between the
// two writes leaves a digest mismatch — a named degraded state reconcile
// repairs, never silence.
func (s *Store) publish(ledger *Ledger) error {
	bytes := Serialize(ledger)
	if _, problems := Parse(bytes); len(problems) > 0 {
		return fmt.Errorf("refusing to write an illegal ledger: %s", problems[0])
	}
	if err := atomicWrite(LedgerPath(s.Root), bytes); err != nil {
		return err
	}
	return s.writeBaseline(bytes)
}

func (s *Store) writeBaseline(bytes []byte) error {
	base := baseline{SchemaVersion: 1, Ledger: string(bytes), Sha256: sha256Hex(bytes)}
	encoded, err := json.MarshalIndent(base, "", " ")
	if err != nil {
		return err
	}
	return atomicWrite(BaselinePath(s.Root), append(encoded, '\n'))
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".goal-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// mutate is the whole spine shared by every mutating verb.
func (s *Store) mutate(genesisOK bool, transition func(*Ledger) (Result, error)) (Result, error) {
	return s.withLock(func() (Result, error) {
		if err := s.refuseMissionSeat(); err != nil {
			return Result{}, err
		}
		state, err := s.readState()
		if err != nil {
			return Result{}, err
		}
		ledger, err := s.mutationLedger(state, genesisOK)
		if err != nil {
			return Result{}, err
		}
		result, err := transition(ledger)
		if err != nil {
			return Result{}, err
		}
		if err := s.publish(ledger); err != nil {
			return Result{}, err
		}
		return result, nil
	})
}

// sectionOf finds the id's section, or "" when absent.
func sectionOf(ledger *Ledger, id string) string {
	if ledger.Current != nil && ledger.Current.Id == id {
		return "current"
	}
	for _, g := range ledger.Queued {
		if g.Id == id {
			return "queued"
		}
	}
	for _, g := range ledger.Parked {
		if g.Id == id {
			return "parked"
		}
	}
	for _, g := range ledger.Done {
		if g.Id == id {
			return "done"
		}
	}
	return ""
}

func takeQueued(ledger *Ledger, id string) (Goal, bool) {
	for i, g := range ledger.Queued {
		if g.Id == id {
			ledger.Queued = append(ledger.Queued[:i:i], ledger.Queued[i+1:]...)
			return g, true
		}
	}
	return Goal{}, false
}

func takeParked(ledger *Ledger, id string) (Goal, bool) {
	for i, g := range ledger.Parked {
		if g.Id == id {
			ledger.Parked = append(ledger.Parked[:i:i], ledger.Parked[i+1:]...)
			return g, true
		}
	}
	return Goal{}, false
}

func takeDone(ledger *Ledger, id string) (Goal, bool) {
	for i, g := range ledger.Done {
		if g.Id == id {
			ledger.Done = append(ledger.Done[:i:i], ledger.Done[i+1:]...)
			return g, true
		}
	}
	return Goal{}, false
}

// dropFree removes a standing declaration in the same atomic write:
// declaring intent supersedes declared absence.
func dropFree(ledger *Ledger) {
	ledger.Free = nil
}

// Open declares a new goal. With no Current goal it becomes Current — the
// one-command program start (GOAL-20); otherwise it queues.
func (s *Store) Open(caller Caller, id, intent, next string) (Result, error) {
	return s.mutate(true, func(ledger *Ledger) (Result, error) {
		if section := sectionOf(ledger, id); section != "" {
			return Result{}, fmt.Errorf("goal %q already exists (%s); ids are unique across all sections", id, section)
		}
		goal := Goal{Id: id, Intent: intent, Origin: caller.origin(), NextStep: next}
		dropFree(ledger)
		if ledger.Current == nil {
			ledger.Current = &goal
			return Result{Message: fmt.Sprintf("opened %s as the Current goal", id)}, nil
		}
		ledger.Queued = append(ledger.Queued, goal)
		return Result{Message: fmt.Sprintf("opened %s into the queue (current: %s)", id, ledger.Current.Id)}, nil
	})
}

// SetNext rewrites the Current goal's step, nothing else.
func (s *Store) SetNext(caller Caller, step string) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		if ledger.Current == nil {
			return Result{}, fmt.Errorf("no Current goal to set a step on")
		}
		if ledger.Current.NextStep == step {
			return Result{}, fmt.Errorf("the Current goal's step already reads exactly that")
		}
		ledger.Current.NextStep = step
		return Result{Message: fmt.Sprintf("next step on %s updated", ledger.Current.Id)}, nil
	})
}

// Promote moves a queued goal to Current.
func (s *Store) Promote(caller Caller, id string) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		switch sectionOf(ledger, id) {
		case "":
			return Result{}, fmt.Errorf("no goal %q to promote", id)
		case "current":
			return Result{}, fmt.Errorf("goal %q is already Current", id)
		case "parked", "done":
			return Result{}, fmt.Errorf("goal %q is %s; only queued goals promote", id, sectionOf(ledger, id))
		}
		if ledger.Current != nil {
			return Result{}, fmt.Errorf("goal %q is already Current; park or conclude it first", ledger.Current.Id)
		}
		goal, _ := takeQueued(ledger, id)
		dropFree(ledger)
		ledger.Current = &goal
		return Result{Message: fmt.Sprintf("promoted %s to Current", id)}, nil
	})
}

// successor applies --then/--and-none after the Current goal leaves:
// promote the named queued goal, or declare goal-free (refusing while
// queued goals stand).
func (s *Store) successor(ledger *Ledger, caller Caller, thenId string, andNone bool) error {
	switch {
	case thenId != "" && andNone:
		return fmt.Errorf("--then and --and-none are mutually exclusive")
	case thenId != "":
		goal, ok := takeQueued(ledger, thenId)
		if !ok {
			return fmt.Errorf("--then %s names no queued goal", thenId)
		}
		ledger.Current = &goal
		return nil
	case andNone:
		if len(ledger.Queued) > 0 {
			var ids []string
			for _, g := range ledger.Queued {
				ids = append(ids, g.Id)
			}
			return fmt.Errorf("--and-none refused: the queue holds %s; promote or park first — declared absence with a standing queue is a contradiction", strings.Join(ids, ", "))
		}
		digest, err := ScanDigest(s.Root)
		if err != nil {
			return fmt.Errorf("cannot compute the plans-stream digest: %v", err)
		}
		ledger.Free = &Free{Declared: s.nowISO(), Origin: caller.origin(), Digest: digest}
		return nil
	default:
		return fmt.Errorf("leaving zero Current goals requires --then <queued-id> or --and-none")
	}
}

// humanGate is the advisory reservation on human-origin conclusions (D66
// constraint 3): done/park on a human-opened goal needs a HUMAN caller.
func humanGate(goal Goal, caller Caller, verb string) error {
	if goal.Origin == OriginHuman && caller.Class != "HUMAN" {
		return fmt.Errorf("goal %s was opened by the human; %s is human-reserved (advisory grade)", goal.Id, verb)
	}
	return nil
}

// Park moves a goal to Parked. Parking the Current goal requires a
// successor decision, exactly like done.
func (s *Store) Park(caller Caller, id, because, thenId string, andNone bool) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		switch sectionOf(ledger, id) {
		case "":
			return Result{}, fmt.Errorf("no goal %q to park", id)
		case "parked":
			return Result{}, fmt.Errorf("goal %q is already parked", id)
		case "done":
			return Result{}, fmt.Errorf("goal %q is done; reopen it instead", id)
		case "queued":
			goal, _ := takeQueued(ledger, id)
			if err := humanGate(goal, caller, "park"); err != nil {
				return Result{}, err
			}
			goal.Parked = because
			ledger.Parked = append(ledger.Parked, goal)
			return Result{Message: fmt.Sprintf("parked %s", id)}, nil
		}
		goal := *ledger.Current
		if err := humanGate(goal, caller, "park"); err != nil {
			return Result{}, err
		}
		ledger.Current = nil
		if err := s.successor(ledger, caller, thenId, andNone); err != nil {
			return Result{}, err
		}
		goal.Parked = because
		ledger.Parked = append(ledger.Parked, goal)
		return Result{Message: fmt.Sprintf("parked %s", id)}, nil
	})
}

// Unpark returns a parked goal to the queue.
func (s *Store) Unpark(caller Caller, id string) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		switch sectionOf(ledger, id) {
		case "":
			return Result{}, fmt.Errorf("no goal %q to unpark", id)
		case "queued":
			return Result{}, fmt.Errorf("goal %q is already queued", id)
		case "current", "done":
			return Result{}, fmt.Errorf("goal %q is %s; only parked goals unpark", id, sectionOf(ledger, id))
		}
		goal, _ := takeParked(ledger, id)
		goal.Parked = ""
		dropFree(ledger)
		ledger.Queued = append(ledger.Queued, goal)
		return Result{Message: fmt.Sprintf("unparked %s into the queue", id)}, nil
	})
}

// Done concludes the Current goal, with the same successor decision as
// park.
func (s *Store) Done(caller Caller, id, concluded, thenId string, andNone bool) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		switch sectionOf(ledger, id) {
		case "":
			return Result{}, fmt.Errorf("no goal %q to conclude", id)
		case "done":
			return Result{}, fmt.Errorf("goal %q is already done", id)
		case "queued", "parked":
			return Result{}, fmt.Errorf("goal %q is %s; only the Current goal concludes", id, sectionOf(ledger, id))
		}
		goal := *ledger.Current
		if err := humanGate(goal, caller, "done"); err != nil {
			return Result{}, err
		}
		ledger.Current = nil
		if err := s.successor(ledger, caller, thenId, andNone); err != nil {
			return Result{}, err
		}
		goal.NextStep = ""
		goal.Evidence = nil
		goal.Conclude = concluded
		ledger.Done = append(ledger.Done, goal)
		return Result{Message: fmt.Sprintf("concluded %s", id)}, nil
	})
}

// Reopen returns a done goal to the queue, Origin preserved; Done blocks
// carry no step, so the caller supplies one.
func (s *Store) Reopen(caller Caller, id, next string) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		switch sectionOf(ledger, id) {
		case "":
			return Result{}, fmt.Errorf("no goal %q to reopen", id)
		case "current", "queued", "parked":
			return Result{}, fmt.Errorf("goal %q is %s, not done", id, sectionOf(ledger, id))
		}
		goal, _ := takeDone(ledger, id)
		goal.Conclude = ""
		goal.NextStep = next
		dropFree(ledger)
		ledger.Queued = append(ledger.Queued, goal)
		return Result{Message: fmt.Sprintf("reopened %s into the queue (origin %s preserved)", id, goal.Origin)}, nil
	})
}

// DeclareFree declares (or renews — the one deliberate idempotence
// exception) the absence of intent over the current plans-stream world.
func (s *Store) DeclareFree(caller Caller) (Result, error) {
	return s.mutate(true, func(ledger *Ledger) (Result, error) {
		if ledger.Current != nil || len(ledger.Queued) > 0 {
			return Result{}, fmt.Errorf("goal-free is legal only at zero Current and zero Queued goals")
		}
		digest, err := ScanDigest(s.Root)
		if err != nil {
			return Result{}, fmt.Errorf("cannot compute the plans-stream digest: %v", err)
		}
		renewed := ledger.Free != nil
		ledger.Free = &Free{Declared: s.nowISO(), Origin: caller.origin(), Digest: digest}
		if renewed {
			return Result{Message: "goal-free declaration renewed over the current plans-stream world"}, nil
		}
		return Result{Message: "goal-free declared over the current plans-stream world"}, nil
	})
}

// Prune drops Done blocks beyond the newest ten and reports every dropped
// block: the ledger is not an audit log, and anything worth keeping goes
// to the decisions documents.
func (s *Store) Prune(caller Caller) (Result, error) {
	return s.mutate(false, func(ledger *Ledger) (Result, error) {
		if len(ledger.Done) <= DoneKept {
			return Result{}, fmt.Errorf("nothing to prune: %d done goals, %d kept", len(ledger.Done), DoneKept)
		}
		cut := len(ledger.Done) - DoneKept
		var dropped []string
		for _, goal := range ledger.Done[:cut] {
			dropped = append(dropped, fmt.Sprintf("%s — %s (concluded: %s)", goal.Id, goal.Intent, goal.Conclude))
		}
		ledger.Done = ledger.Done[cut:]
		return Result{
			Message: fmt.Sprintf("pruned %d done goals; the decisions documents remain the audit surface", cut),
			Dropped: dropped,
		}, nil
	})
}

// Reconcile is the single authority path for bytes the verbs did not
// write: genesis adoption of an unbaselined ledger, restoration of a
// deleted ledger from the baseline, and the block-level delta replay for
// manual edits — applying every rule the direct verbs apply.
func (s *Store) Reconcile(caller Caller) (Result, error) {
	return s.withLock(func() (Result, error) {
		if err := s.refuseMissionSeat(); err != nil {
			return Result{}, err
		}
		state, err := s.readState()
		if err != nil {
			return Result{}, err
		}
		switch {
		case state.ledgerBytes == nil && state.base == nil:
			return Result{}, fmt.Errorf("nothing to reconcile: no ledger and no baseline; `goal open` starts one")
		case state.ledgerBytes == nil && state.base != nil:
			// Post-adoption deletion: restore the accepted full bytes
			// (there are no candidate bytes, so there is no delta to
			// replay). Wanting the ledger gone has a legal path — the
			// verbs; rm is not it.
			if err := atomicWrite(LedgerPath(s.Root), []byte(state.base.Ledger)); err != nil {
				return Result{}, err
			}
			return Result{Message: "restored from baseline"}, nil
		case state.base == nil:
			// Genesis: adopt the ledger as the first accepted state after
			// the full grammar check. The parser's legality matrix IS the
			// verb-reachable state set (every parse-legal ledger is
			// producible from empty by a verb sequence), so a strict parse
			// is the genesis replay.
			if _, problems := Parse(state.ledgerBytes); len(problems) > 0 {
				return Result{}, fmt.Errorf("genesis reconcile refused: %s", problems[0])
			}
			if err := s.writeBaseline(state.ledgerBytes); err != nil {
				return Result{}, err
			}
			return Result{Message: "adopted the ledger as the first accepted state"}, nil
		}
		if string(state.ledgerBytes) == state.base.Ledger {
			return Result{Message: "already reconciled; the ledger matches the accepted baseline"}, nil
		}
		accepted, acceptedProblems := Parse([]byte(state.base.Ledger))
		if len(acceptedProblems) > 0 {
			// The baseline itself is malformed — accept the edited ledger
			// as genesis if it parses; the baseline was never a legal
			// authority.
			if _, problems := Parse(state.ledgerBytes); len(problems) > 0 {
				return Result{}, fmt.Errorf("both the ledger and the baseline are malformed; repair goals.md by hand and reconcile again: %s", problems[0])
			}
			if err := s.writeBaseline(state.ledgerBytes); err != nil {
				return Result{}, err
			}
			return Result{Message: "the stored baseline was malformed; adopted the ledger as the accepted state"}, nil
		}
		edited, problems := Parse(state.ledgerBytes)
		if len(problems) > 0 {
			return Result{}, fmt.Errorf("reconcile refused: the edited ledger is malformed: %s", problems[0])
		}
		if err := replayAuthority(accepted, edited, caller); err != nil {
			return Result{}, fmt.Errorf("reconcile refused: %v", err)
		}
		if err := s.writeBaseline(state.ledgerBytes); err != nil {
			return Result{}, err
		}
		return Result{Message: "reconciled: the edit maps to legal transitions and is accepted"}, nil
	})
}

// replayAuthority maps the block-level delta between the accepted and
// edited ledgers to verb transitions and applies every rule the direct
// verbs apply. A manual edit can never reach a state the equivalent verb
// sequence would have refused.
func replayAuthority(accepted, edited *Ledger, caller Caller) error {
	acceptedGoals := goalIndex(accepted)
	editedGoals := goalIndex(edited)

	for id, before := range acceptedGoals {
		after, present := editedGoals[id]
		if !present {
			if before.section != "done" {
				return fmt.Errorf("goal %s was deleted from %s; no verb deletes standing goals (prune drops only done blocks)", id, before.section)
			}
			continue // prune
		}
		if before.goal.Origin != after.goal.Origin {
			return fmt.Errorf("goal %s: origin may never be rewritten (%s -> %s)", id, before.goal.Origin, after.goal.Origin)
		}
		if err := replayTransition(id, before, after, caller); err != nil {
			return err
		}
	}
	// New ids are opens — always legal; the parser already enforced their
	// shape and the ledger's structural legality.
	return nil
}

type placedGoal struct {
	section string
	goal    Goal
}

func goalIndex(ledger *Ledger) map[string]placedGoal {
	out := map[string]placedGoal{}
	if ledger.Current != nil {
		out[ledger.Current.Id] = placedGoal{"current", *ledger.Current}
	}
	for _, g := range ledger.Queued {
		out[g.Id] = placedGoal{"queued", g}
	}
	for _, g := range ledger.Parked {
		out[g.Id] = placedGoal{"parked", g}
	}
	for _, g := range ledger.Done {
		out[g.Id] = placedGoal{"done", g}
	}
	return out
}

// replayTransition checks one goal's movement against the table.
func replayTransition(id string, before, after placedGoal, caller Caller) error {
	if before.section == after.section {
		// Intra-section edits: the Current step edit is set-next; any
		// other field rewrite has no producing verb.
		if before.section == "current" {
			same := before.goal.Intent == after.goal.Intent &&
				equalLines(before.goal.Evidence, after.goal.Evidence)
			if !same {
				return fmt.Errorf("goal %s: only the Current goal's next step is editable in place (set-next)", id)
			}
			return nil
		}
		if beforeBytes, afterBytes := fmt.Sprint(before.goal), fmt.Sprint(after.goal); beforeBytes != afterBytes {
			return fmt.Errorf("goal %s: no verb edits a %s goal in place", id, before.section)
		}
		return nil
	}
	legal := map[[2]string]bool{
		{"queued", "current"}: true, // promote
		{"queued", "parked"}:  true, // park
		{"current", "parked"}: true, // park (successor legality is the parser's zero-current rule)
		{"current", "done"}:   true, // done
		{"parked", "queued"}:  true, // unpark
		{"done", "queued"}:    true, // reopen
	}
	if !legal[[2]string{before.section, after.section}] {
		return fmt.Errorf("goal %s: %s -> %s is not a legal transition", id, before.section, after.section)
	}
	if after.section == "done" || after.section == "parked" {
		verb := "done"
		if after.section == "parked" {
			verb = "park"
		}
		if err := humanGate(before.goal, caller, verb); err != nil {
			return err
		}
	}
	return nil
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ReadLedger is the read path: parse whatever stands, with problems as
// degraded facts, never mutating anything. Read verbs and the verdict ride
// this.
func (s *Store) ReadLedger() (*Ledger, []Problem, error) {
	data, err := os.ReadFile(LedgerPath(s.Root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	ledger, problems := Parse(data)
	return ledger, problems, nil
}

// BaselinePresent reports whether adoption's accepted state exists —
// the fact that splits pre-adoption absence from post-adoption deletion.
func (s *Store) BaselinePresent() bool {
	_, err := os.Stat(BaselinePath(s.Root))
	return err == nil
}

// BaselineMatches reports whether the ledger bytes match the accepted
// baseline (the degraded check for the verdict's read side).
func (s *Store) BaselineMatches() bool {
	state, err := s.readState()
	if err != nil || state.base == nil || state.ledgerBytes == nil {
		return false
	}
	return string(state.ledgerBytes) == state.base.Ledger && sha256Hex(state.ledgerBytes) == state.base.Sha256
}

