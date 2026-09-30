package batch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// DiagnosticRequest is one fresh, charged classification run. Tree is always
// derived from the durable batch record and NeverReuse must remain true.
type DiagnosticRequest struct {
	Tree, GoalID string
	Groups       []string
	Claim        Claim
	NeverReuse   bool
	// Fresh is the adapters' fresh-execution argv for a run whose purpose is
	// to execute, such as the second base run.
	Fresh []string
}

// DiagnosticResult is the evidence needed to classify one diagnostic tree:
// its red groups and every group's execution evidence.
type DiagnosticResult struct {
	AttemptID string
	Groups    []RedGroup
	Evidence  []GroupEvidence
	Sample    proofrun.LoadSample
}

// GroupEvidence is how one group of a diagnostic run ended.
type GroupEvidence struct {
	ID, Status, ExecutionIdentity, LogPath, LogDigest string
	NativeLaunched, CollectionComplete                bool
}

func (result DiagnosticResult) Green() bool { return len(result.Groups) == 0 }

// DiagnosticRefusal identifies a refusal before a diagnostic runner started.
type DiagnosticRefusal struct{ Status string }

func (refusal *DiagnosticRefusal) Error() string { return refusal.Status }

// RedSeams names the effects outside classification and tree derivation.
type RedSeams struct {
	Run func(DiagnosticRequest) (DiagnosticResult, error)
	// ConfirmFenced re-reads the live accepted ledger and binds a stop fence to
	// this exact joined, handed-over claim. A runner refusal alone is not proof.
	ConfirmFenced func(Unit) (string, bool, error)
	MintOpid      func() (string, error)
	Ledger        LedgerOwner
	UpdateNext    func(goalID, status string) error
	BaseCommit    string
	// Adapter names a red group's language adapter and whether the group is a
	// package-selection expansion, whose manifest is the whole module; a nil
	// adapter means the group's language facts are unknown.
	Adapter func(RedGroup) (language adapter.Adapter, expansion bool)
	// Sources is the retained verifier on the tip tree: per selected group,
	// the attempt that holds its pass.
	Sources func(Record) (map[string]string, error)
	// Location is where a known flake's allowance is counted and shown.
	Location *time.Location
	// LaneOwner names who registered the lane and whether that is a person:
	// the owner of a flake found in a batch of changes alone (U11b).
	LaneOwner func() (name string, person bool)
}

// DiagnoseRed decides a red tip proof (D1): the failing groups run on the
// base; a base red runs once more, executed, and red twice is a trunk red
// that holds, red then green makes its identities known flakes. On a green
// base every member named by evidence is ejected at once; when nobody is
// named the known-flake predicate decides (unnamed). A diagnostic that cannot
// run leaves the batch diagnosing for the next tick. It never changes the
// record's unit order.
func DiagnoseRed(store Store, id, actor string, failing []RedGroup, prefixGoal string, at time.Time, seams RedSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateDiagnosing || record.Proof == nil {
		return fmt.Errorf("batch %s is not diagnosing a recorded test run", id)
	}
	joined := joinedUnits(record.Units)
	if len(joined) == 0 || seams.Run == nil {
		return fmt.Errorf("batch %s has no diagnostic runner or joined units", id)
	}
	// The diagnosis is charged as the tip proof was: to the last goal
	// member, or for a batch of changes to the lane (U11b).
	authority := ChargeUnit(joined)
	// runOn returns done when the diagnostic could not decide the red: the
	// fenced authority was ejected, or the batch stays diagnosing and err says
	// why when the runner failed outside an admission refusal.
	runOn := func(tree string, groups []RedGroup, fresh []string) (DiagnosticResult, bool, error) {
		result, runErr := seams.Run(DiagnosticRequest{Tree: tree, GoalID: authority.GoalID, Groups: redGroupIDs(groups),
			Claim: authority.Claim, NeverReuse: true, Fresh: fresh})
		if runErr == nil {
			return result, false, nil
		}
		var refusal *DiagnosticRefusal
		status := runErr.Error()
		if errors.As(runErr, &refusal) {
			status, runErr = refusal.Status, nil
			if seams.ConfirmFenced != nil {
				reason, fenced, confirmErr := seams.ConfirmFenced(authority)
				if confirmErr == nil && fenced {
					return DiagnosticResult{}, true, ReassembleSurvivorsWithReturns(store, id, actor, at,
						[]ReturnDecision{{GoalID: authority.GoalID, Outcome: UnitEjected, Reason: reason}})
				}
				if confirmErr != nil {
					status += "; live fence verification: " + confirmErr.Error()
				}
			}
		}
		if seams.UpdateNext != nil && !authority.IsChange() {
			next := "diagnostic unavailable: " + status + "; the landing batch stays diagnosing and runs it again at its next tick"
			if editErr := seams.UpdateNext(authority.GoalID, next); editErr != nil {
				return DiagnosticResult{}, true, errors.Join(runErr, editErr)
			}
		}
		if runErr != nil {
			runErr = fmt.Errorf("batch %s stays diagnosing: diagnostic unavailable: %w", id, runErr)
		}
		return DiagnosticResult{}, true, runErr
	}
	base, done, err := runOn(record.BaseTree, failing, nil)
	if done {
		return err
	}
	decide := redDecision{store: store, record: record, joined: joined, actor: actor, failing: failing, at: at, seams: seams, runOn: runOn}
	if !base.Green() {
		second, done, err := runOn(record.BaseTree, base.Groups, freshExecution(base.Groups, seams.Adapter))
		if done {
			return err
		}
		if !second.Green() {
			return holdAndRecordTrunkRed(store, record, actor, second, seams, at)
		}
		// Red then green on one base tree: main itself was intermittently red,
		// so its identities become known flakes and its stalls hang entries.
		if err := decide.recordMainEvidence(base, second); err != nil {
			return err
		}
		return decide.unnamed()
	}
	named := namedDiagnosticUnits(joined, failing, seams.Adapter)
	if prefixGoal != "" {
		named = map[string]bool{prefixGoal: true}
	}
	if len(named) == 0 {
		return decide.unnamed()
	}
	decisions := make([]ReturnDecision, 0, len(named))
	for _, unit := range joined {
		if named[unit.GoalID] {
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: diagnosticFailure(base, failing)})
		}
	}
	return ReassembleSurvivorsWithReturns(store, id, actor, at, decisions)
}

func redGroupIDs(groups []RedGroup) []string {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

// freshExecution is the union of the red groups' adapters' fresh-execution
// argv, each argument once, in group order.
func freshExecution(groups []RedGroup, languageOf func(RedGroup) (adapter.Adapter, bool)) []string {
	var fresh []string
	for _, group := range groups {
		if languageOf == nil {
			break
		}
		if language, _ := languageOf(group); language != nil {
			for _, argument := range language.FreshExecution() {
				if !slices.Contains(fresh, argument) {
					fresh = append(fresh, argument)
				}
			}
		}
	}
	return fresh
}

func joinedUnits(units []Unit) []Unit {
	return slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.State != UnitJoined })
}

// namedDiagnosticUnits names a member for a red group (R2) when the adapter's
// owner unit of any failure is in the member's recorded closure, or, for a
// group that is not a package-selection expansion, when a changed path of the
// member matches the group's declared input manifest.
func namedDiagnosticUnits(units []Unit, failing []RedGroup, languageOf func(RedGroup) (adapter.Adapter, bool)) map[string]bool {
	named := map[string]bool{}
	for _, group := range failing {
		var language adapter.Adapter
		expansion := false
		if languageOf != nil {
			language, expansion = languageOf(group)
		}
		for _, unit := range units {
			if language != nil && unit.Closure != nil && ownsAFailure(language, group, *unit.Closure) {
				named[unit.GoalID] = true
				continue
			}
			if expansion {
				continue
			}
			for _, changed := range unit.ChangedPaths {
				for _, input := range group.InputManifest {
					if diagnosticPathMatches(input, changed) {
						named[unit.GoalID] = true
					}
				}
			}
		}
	}
	return named
}

func ownsAFailure(language adapter.Adapter, group RedGroup, closure adapter.Closure) bool {
	for _, failure := range group.Failures {
		owner, ok := language.OwnerUnit(adapter.Failure{Report: failure.Report, Classname: failure.Classname, Name: failure.Name,
			Status: failure.Status, Reason: failure.Reason}, closure)
		if ok && closure.Contains(owner) {
			return true
		}
	}
	return false
}

func diagnosticPathMatches(pattern, changed string) bool {
	value, literal, err := pathpattern.ManifestEntry(pattern)
	if err != nil {
		return true
	}
	value, changed = strings.TrimPrefix(value, "metasystem/"), strings.TrimPrefix(changed, "metasystem/")
	if literal {
		return value == changed
	}
	matched, err := pathpattern.MatchManifestEntry(value, changed)
	if err != nil {
		return true
	}
	return matched
}

func diagnosticFailure(result DiagnosticResult, fallback []RedGroup) string {
	groups := result.Groups
	if len(groups) == 0 {
		groups = fallback
	}
	ids, logs, failures := make([]string, 0, len(groups)), []string{}, []string{}
	for _, group := range groups {
		ids = append(ids, group.ID)
		if group.LogPath != "" {
			logs = append(logs, group.LogPath)
		}
		for _, failure := range group.Failures {
			if failure.Name != "" {
				failures = append(failures, failure.Name)
			}
		}
	}
	return fmt.Sprintf("attempt=%s groups=%s logs=%s failures=%s", result.AttemptID,
		strings.Join(ids, ","), strings.Join(logs, ","), strings.Join(failures, ","))
}

func holdAndRecordTrunkRed(store Store, record Record, actor string, result DiagnosticResult, seams RedSeams, at time.Time) error {
	if seams.MintOpid == nil || seams.Ledger == nil {
		return errLedgerOwnerUnbound
	}
	opid, err := seams.MintOpid()
	if err != nil {
		return err
	}
	red := TrunkRed{AttemptID: result.AttemptID, BaseCommit: seams.BaseCommit, BaseTree: record.BaseTree, Groups: result.Groups}
	if err := store.HoldTrunkRed(record.BatchID, red, opid, at, actor); err != nil {
		return err
	}
	_, err = store.WithLedgerOwner(seams.Ledger).EnsureTrunkRedRecorded(record.BatchID, seams.MintOpid, at, actor)
	return err
}

// redDecision carries one diagnosis through steps 1 and 3 of D1.
type redDecision struct {
	store   Store
	record  Record
	joined  []Unit
	actor   string
	failing []RedGroup
	at      time.Time
	seams   RedSeams
	runOn   func(string, []RedGroup, []string) (DiagnosticResult, bool, error)
}

func (d redDecision) ledger() (FlakeLedgerOwner, func() (string, error), error) {
	ledger, ok := d.seams.Ledger.(FlakeLedgerOwner)
	if !ok || d.seams.MintOpid == nil {
		return nil, nil, errLedgerOwnerUnbound
	}
	return ledger, d.seams.MintOpid, nil
}

// recordMainEvidence opens or promotes a known flake per identified failure
// of main's red, owned by the approver of the member whose closure holds the
// test's owner unit, else the batch's last member's approver; a stall on the
// base opens a hang entry.
func (d redDecision) recordMainEvidence(red, green DiagnosticResult) error {
	ledger, mint, err := d.ledger()
	if err != nil {
		return err
	}
	byOwner, owners := map[string][]RedGroup{}, []Unit{}
	for _, group := range red.Groups {
		if stalled(group) {
			if err := d.recordHang(ledger, mint, red, group, ""); err != nil {
				return err
			}
			continue
		}
		for _, failure := range d.identified(group) {
			owner := flakeOwner(d.joined, d.language(group), failure)
			if _, seen := byOwner[owner.GoalID]; !seen {
				owners = append(owners, owner)
			}
			one := group
			one.Failures = []Failure{failure}
			byOwner[owner.GoalID] = append(byOwner[owner.GoalID], one)
		}
	}
	for _, owner := range owners {
		opid, err := mint()
		if err != nil {
			return err
		}
		approver := owner.Approver
		if owner.IsChange() {
			// A batch of changes alone: the flake is the lane's registrar's,
			// when that is a person; else it stays pending and the batch goes
			// on by its classification, never looping on a refused promotion.
			name, person := "", false
			if d.seams.LaneOwner != nil {
				name, person = d.seams.LaneOwner()
			}
			if !person {
				if _, err := ledger.RecordPending(opid, d.sighting(red, green, byOwner[owner.GoalID], "")); err != nil {
					return err
				}
				continue
			}
			approver = name
		}
		if _, err := ledger.Promote(opid, Promotion{FlakeSighting: d.sighting(red, green, byOwner[owner.GoalID], ""),
			Owner: approver, OwnerMachine: owner.Claim.Machine, Location: d.seams.Location}); err != nil {
			return err
		}
	}
	return nil
}

func flakeOwner(joined []Unit, language adapter.Adapter, failure Failure) Unit {
	for _, unit := range joined {
		if !unit.IsChange() && language != nil && unit.Closure != nil && ownsAFailure(language, RedGroup{Failures: []Failure{failure}}, *unit.Closure) {
			return unit
		}
	}
	// A change has no approver: its flake is owned by the charge member's.
	return ChargeUnit(joined)
}

func (d redDecision) sighting(red, green DiagnosticResult, groups []RedGroup, tipTree string) FlakeSighting {
	sighting := FlakeSighting{BatchID: d.record.BatchID, BaseCommit: d.seams.BaseCommit, BaseTree: d.record.BaseTree, TipTree: tipTree,
		RedAttempt: red.AttemptID, Groups: groups, RedSample: red.Sample, GreenAttempt: green.AttemptID, GreenSample: green.Sample, SeenAt: d.at}
	if len(green.Evidence) > 0 {
		sighting.GreenLogPath, sighting.GreenLogDigest = green.Evidence[0].LogPath, green.Evidence[0].LogDigest
	}
	return sighting
}

func (d redDecision) recordHang(ledger FlakeLedgerOwner, mint func() (string, error), run DiagnosticResult, group RedGroup, tipTree string) error {
	opid, err := mint()
	if err != nil {
		return err
	}
	_, err = ledger.RecordHang(opid, HangSighting{BatchID: d.record.BatchID, AttemptID: run.AttemptID, BaseCommit: d.seams.BaseCommit,
		BaseTree: d.record.BaseTree, TipTree: tipTree, Group: group, Evidence: hangEvidence(group), Sample: run.Sample, SeenAt: d.at})
	return err
}

// stalled reports whether the watchdog stalled the group: a hang, never a
// flake. It is the result's typed stall, never the words of its reason.
func stalled(group RedGroup) bool {
	return group.Stall != nil
}

// hangEvidence is the watchdog's typed stall and the group's silence
// figures; the last started test is the one without a terminal.
func hangEvidence(group RedGroup) HangEvidence {
	evidence := HangEvidence{Dump: "dump: unavailable (not requested)", LongestSilentSeconds: group.LongestSilentSeconds,
		LongestZeroCPUSeconds: group.LongestZeroCPUSeconds}
	if group.Stall != nil {
		evidence.Section, evidence.EvidenceDir = group.Stall.Section, group.Stall.EvidenceDir
		if group.Stall.Dump != "" {
			evidence.Dump = group.Stall.Dump
		}
	}
	for _, missing := range group.Missing {
		if missing.Status == "missing-terminal" {
			evidence.LastStartedTest = missing.Name
		}
	}
	return evidence
}

func (d redDecision) language(group RedGroup) adapter.Adapter {
	if d.seams.Adapter == nil {
		return nil
	}
	language, _ := d.seams.Adapter(group)
	return language
}

// identified is the group's failed tests whose identity the adapter accepts.
func (d redDecision) identified(group RedGroup) []Failure {
	language, identified := d.language(group), []Failure{}
	for _, failure := range group.Failures {
		if language == nil || failure.Status != "failed" {
			continue
		}
		if _, ok := language.Identity(adapter.Failure{Report: failure.Report, Classname: failure.Classname, Name: failure.Name,
			Status: failure.Status, Reason: failure.Reason}); ok {
			identified = append(identified, failure)
		}
	}
	return identified
}

// predicate is D1's five clauses over every red group: the known flakes a
// composed landing would use, or the text of each group's failing clause.
func (d redDecision) predicate(known []OpenEntry) (uses []FlakeUse, clauses []string) {
	for _, group := range d.failing {
		identified := d.identified(group)
		var clause string
		switch format := strings.SplitN(group.Adapter, ":", 2); {
		case group.Status != "failed":
			clause = fmt.Sprintf("group %s is red without a failing test identity (status %s: %s)", group.ID, group.Status, group.NotRunReason)
		case !group.CollectionComplete || len(group.Missing)+len(group.Unexpected) > 0:
			clause = fmt.Sprintf("group %s is red without complete evidence (collection complete %t, %d missing, %d unexpected)",
				group.ID, group.CollectionComplete, len(group.Missing), len(group.Unexpected))
		case group.Adapter == "" || format[0] == "section" || format[0] == "command" && group.Adapter != "command:junit-xml":
			clause = fmt.Sprintf("group %s's adapter %q reports no test identities", group.ID, group.Adapter)
		case len(group.Failures) == 0 || len(identified) != len(group.Failures):
			clause = fmt.Sprintf("group %s is red without a failing test identity for each failure (%d of %d identified)",
				group.ID, len(identified), len(group.Failures))
		}
		for _, failure := range identified {
			if clause != "" {
				break
			}
			identity := FlakeID(group.ID, failure)
			index := slices.IndexFunc(known, func(entry OpenEntry) bool { return entry.Identity == identity && entry.Class == ClassKnownFlake })
			switch {
			case index < 0:
				clause = fmt.Sprintf("%s (group %s) is not a known flake", failure.Name, group.ID)
			case !known[index].CarriesLanding(d.at):
				clause = fmt.Sprintf("known flake %s's landing allowance expired on %s (owner %s); it blocks every batch until fixed at its cause",
					known[index].ID, known[index].AllowanceUntil.In(d.location()).Format("Mon 2 Jan 15:04 MST"), known[index].Owner)
			default:
				uses = append(uses, FlakeUse{Identity: identity, EntryID: known[index].ID, AllowanceUntil: known[index].AllowanceUntil})
			}
		}
		if clause != "" {
			clauses = append(clauses, clause+"; nothing lands on it")
		}
	}
	return uses, clauses
}

func (d redDecision) location() *time.Location {
	if d.seams.Location == nil {
		return time.Local
	}
	return d.seams.Location
}

// unnamed is step 3: a stall is a hang entry; the known-flake predicate
// decides between the composed landing (3a) and every member's return (3b);
// the one classification run executes the red groups on the identical tip
// when every red group has an identified failure.
func (d redDecision) unnamed() error {
	ledger, mint, err := d.ledger()
	if err != nil {
		return err
	}
	tip := DiagnosticResult{AttemptID: d.record.Proof.AttemptID, Groups: d.failing, Sample: d.record.Proof.Sample}
	returned := func(text string, run DiagnosticResult) error {
		return returnEveryMember(d.store, d.record, d.actor, d.at, text+"; "+diagnosticFailure(run, d.failing))
	}
	identifiedGroups, classifiable := []RedGroup{}, true
	for _, group := range d.failing {
		if stalled(group) {
			if err := d.recordHang(ledger, mint, tip, group, d.record.TipTree); err != nil {
				return err
			}
		}
		one := group
		one.Failures = d.identified(group)
		classifiable = classifiable && len(one.Failures) > 0
		identifiedGroups = append(identifiedGroups, one)
	}
	known, err := ledger.OpenByClass(ClassKnownFlake)
	if err != nil {
		return err
	}
	uses, clauses := d.predicate(known)
	if !classifiable {
		return returned(strings.Join(clauses, "; "), tip)
	}
	if len(clauses) == 0 {
		// Nobody was named, but a member whose closure is unknown was never
		// asked: it may own the red. A known flake does not carry it; it
		// returns, and the others re-prove without it.
		if unknown := d.unknownClosures(tip); len(unknown) > 0 {
			return ReassembleSurvivorsWithReturns(d.store, d.record.BatchID, d.actor, d.at, unknown)
		}
	}
	run, done, err := d.runOn(d.record.TipTree, d.failing, freshExecution(d.failing, d.seams.Adapter))
	if done {
		return err
	}
	if why := d.classificationRed(run); why != "" {
		for _, group := range run.Groups {
			if stalled(group) {
				if err := d.recordHang(ledger, mint, run, group, d.record.TipTree); err != nil {
					return err
				}
			}
		}
		if len(run.Groups) > 0 {
			why += ": the members' changes break it together, since it passes on main"
		}
		return returned(why+"; nothing lands on it", run)
	}
	opid, err := mint()
	if err != nil {
		return err
	}
	if len(clauses) > 0 {
		refs, err := ledger.RecordPending(opid, d.sighting(tip, run, identifiedGroups, d.record.TipTree))
		if err != nil {
			return err
		}
		ids := []string{}
		for _, ref := range refs {
			ids = append(ids, ref.ID)
		}
		return returned(fmt.Sprintf("the red failed on the batch tip, passed when run again there, and passes on main; it is a pending flake (entry %s), "+
			"not a known one, so nothing lands on it (%s). Reproduce it on your tree, then land again with metasystem work land; a flake becomes known only from evidence on main",
			strings.Join(ids, ","), strings.Join(clauses, "; ")), run)
	}
	composed := d.record
	composed.Proof = new(Proof)
	*composed.Proof = *d.record.Proof
	composed.Proof.Flakes = uses
	if d.seams.Sources == nil {
		return fmt.Errorf("batch %s stays diagnosing: no kept verifier can put its test run together", d.record.BatchID)
	}
	resolved, err := d.seams.Sources(composed)
	if err != nil {
		return d.verifierUnavailable(err, run)
	}
	sources, err := ResolveSources(*composed.Proof, resolved)
	if err != nil {
		return returned(err.Error()+"; nothing lands on it", run)
	}
	// The sighting follows the compare-and-swap: a lost swap leaves none for
	// a landing that never happened. Its opid is on the record, and the
	// register journals by opid, so publishing it again is idempotent.
	if err := d.store.Update(d.record.BatchID, func(current *Record) error {
		if current.State != StateDiagnosing || current.Proof == nil || current.Proof.AttemptID != d.record.Proof.AttemptID {
			return fmt.Errorf("%s: the batch changed before its combined test run was recorded", codeProofInputMoved)
		}
		current.Proof.Status, current.Proof.Failure, current.Proof.Sources, current.Proof.Flakes = "green", "", sources, uses
		current.Transition(StateLanding, d.at, "diagnose", d.actor, "composed on known flakes; classification attempt "+run.AttemptID+"; sighting op "+opid)
		return nil
	}); err != nil {
		return err
	}
	if _, err := ledger.RecordPending(opid, d.sighting(tip, run, identifiedGroups, d.record.TipTree)); err != nil {
		return fmt.Errorf("batch %s lands composed but its flake sighting (op %s) was not recorded: %w", d.record.BatchID, opid, err)
	}
	return nil
}

// unknownClosures returns each joined member without a recorded closure: no
// language adapter recognised its checkout at join, or the closure failed
// there. Such a member can be neither named nor cleared by owner unit.
func (d redDecision) unknownClosures(tip DiagnosticResult) []ReturnDecision {
	var decisions []ReturnDecision
	for _, unit := range d.joined {
		if unit.Closure == nil {
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: fmt.Sprintf(
				"BATCH_MEMBER_CLOSURE_UNKNOWN: %s has no recorded closure (no language adapter recognised its checkout at join, or its closure failed there), "+
					"so the lane cannot rule out that it owns this red; a known flake does not carry it and nothing of it lands; %s",
				unit.GoalID, diagnosticFailure(tip, d.failing))})
		}
	}
	return decisions
}

// MaxComposedVerifierAttempts bounds how often the composed path runs the
// retained verifier for one tip proof before every member returns: each
// failed try is counted on the record, so an owner restart keeps the count.
const MaxComposedVerifierAttempts = 3

// verifierUnavailable counts one failed retained verification of this tip
// proof on the record and leaves the batch diagnosing for the next tick; the
// last allowed failure returns every member with the classification log and
// the verifier's error instead.
func (d redDecision) verifierUnavailable(cause error, run DiagnosticResult) error {
	prefix := "attempt=" + d.record.Proof.AttemptID + " "
	tries := 1
	for _, entry := range d.record.History {
		if entry.Verb == "verifier-unavailable" && strings.HasPrefix(entry.Detail, prefix) {
			tries++
		}
	}
	if tries >= MaxComposedVerifierAttempts {
		return returnEveryMember(d.store, d.record, d.actor, d.at, fmt.Sprintf("BATCH_VERIFIER_UNAVAILABLE: the retained verifier failed %d times composing the known-flake landing (%v); nothing lands on it; %s",
			tries, cause, diagnosticFailure(run, d.failing)))
	}
	err := d.store.Update(d.record.BatchID, func(current *Record) error {
		if current.State != StateDiagnosing || current.Proof == nil || current.Proof.AttemptID != d.record.Proof.AttemptID {
			return fmt.Errorf("%s: batch changed before its verifier failure was counted", codeProofInputMoved)
		}
		current.Transition(StateDiagnosing, d.at, "verifier-unavailable", d.actor, fmt.Sprintf("%stry %d of %d: %v", prefix, tries, MaxComposedVerifierAttempts, cause))
		return nil
	})
	return errors.Join(fmt.Errorf("batch %s stays diagnosing: retained verifier try %d of %d: %w", d.record.BatchID, tries, MaxComposedVerifierAttempts, cause), err)
}

// classificationRed judges the classification run in reverse: it is green
// only when every requested group passed, executed, with complete collection,
// at the execution identity the tip plan recorded.
func (d redDecision) classificationRed(run DiagnosticResult) string {
	if len(run.Groups) > 0 {
		return fmt.Sprintf("the red failed twice on the batch tip (%s)", strings.Join(redGroupIDs(run.Groups), ","))
	}
	for _, group := range d.failing {
		index := slices.IndexFunc(run.Evidence, func(evidence GroupEvidence) bool { return evidence.ID == group.ID })
		want := d.record.Proof.GroupIdentities[group.ID]
		if index < 0 || run.Evidence[index].Status != "passed" || !run.Evidence[index].NativeLaunched || !run.Evidence[index].CollectionComplete ||
			want == "" || run.Evidence[index].ExecutionIdentity != want {
			return fmt.Sprintf("the classification run of group %s was not an executed, complete pass at the tip plan's execution identity", group.ID)
		}
	}
	return ""
}
