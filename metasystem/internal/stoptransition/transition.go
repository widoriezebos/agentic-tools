// Package stoptransition owns the ordered, per-checkout stop transaction.
package stoptransition

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// Item is one currently live identity from a process family.
type Item struct {
	Key             string
	StatusLine      string
	FenceGeneration *int64
	Survivor        stopfence.Survivor
	// ObserveOnly marks an identity whose family reports it without signalling,
	// such as an untracked process or a run under adopted custody. Crashed-stop
	// re-inventory excludes it because arm must not treat it as a survivor.
	ObserveOnly bool
}

// Outcome is the family owner's truthful result after acting on one item.
type Outcome struct {
	Line            string
	AdditionalLines []string
	Complete        bool
	Survivor        stopfence.Survivor
	// Auxiliary holds independently identified results discovered while a
	// family stops one process. A supervision shutdown, for example, can stop
	// its owner successfully and then fail to publish bookkeeping for that
	// stop. Those are two things in the report and only the latter survives.
	Auxiliary []AuxiliaryOutcome
}

// AuxiliaryOutcome is one independently counted, processless result produced
// by the same family stop call as an ordinary process outcome.
type AuxiliaryOutcome struct {
	Key      string
	Line     string
	Complete bool
	Survivor stopfence.Survivor
}

// Family owns inventory and stopping for one ordered process family.
type Family interface {
	Name() string
	Inventory() ([]Item, error)
	Stop(Item) (Outcome, error)
}

// InventoryReadError preserves the one durable file that prevented a family
// from assembling its inventory. The transition records the file separately
// from the parse or read reason so a later arm refusal remains actionable.
type InventoryReadError struct {
	Path string
	Err  error
}

func (e *InventoryReadError) Error() string {
	if e.Err == nil {
		return "inventory record could not be read"
	}
	return e.Err.Error()
}

func (e *InventoryReadError) Unwrap() error { return e.Err }

// Report is the complete human page for one transition.
type Report struct {
	Lines    []string
	ExitCode int
}

type reportSlot struct {
	key      string
	lines    []string
	complete bool
	arrived  bool
}

// stopReportBuilder holds one replaceable place per identity. It changes only
// the in-memory report: every inventory and stop attempt still runs.
type stopReportBuilder struct {
	slots []reportSlot
	index map[string]int
}

func newStopReportBuilder(checkout string) *stopReportBuilder {
	return &stopReportBuilder{
		slots: []reportSlot{{key: "checkout", lines: []string{"checkout " + checkout}, complete: true}},
		index: map[string]int{"checkout": 0},
	}
}

func (b *stopReportBuilder) set(key string, lines []string, complete bool) {
	if index, ok := b.index[key]; ok {
		b.slots[index].lines = lines
		b.slots[index].complete = complete
		return
	}
	b.index[key] = len(b.slots)
	b.slots = append(b.slots, reportSlot{key: key, lines: lines, complete: complete})
}

func (b *stopReportBuilder) outcome(item Item, outcome Outcome, arrived bool) {
	key := "item:" + survivorIdentity(item.Survivor, item.Key)
	index, exists := b.index[key]
	if !exists {
		b.index[key] = len(b.slots)
		b.slots = append(b.slots, reportSlot{key: key, arrived: arrived})
		index = len(b.slots) - 1
	}
	slot := &b.slots[index]
	if slot.complete && outcome.Complete && outcomeSaysGone(outcome.Line) {
		return
	}
	line := outcome.Line
	if slot.arrived {
		line += " (arrived during stop)"
	}
	slot.lines = append([]string{line}, outcome.AdditionalLines...)
	slot.complete = outcome.Complete
}

func (b *stopReportBuilder) seen(item Item) bool {
	_, ok := b.index["item:"+survivorIdentity(item.Survivor, item.Key)]
	return ok
}

func recordAuxiliaryOutcomes(builder *stopReportBuilder, survivors *[]stopfence.Survivor, outcome Outcome) {
	for _, auxiliary := range outcome.Auxiliary {
		item := Item{Key: "auxiliary:" + auxiliary.Key, Survivor: auxiliary.Survivor}
		builder.outcome(item, Outcome{Line: auxiliary.Line, Complete: auxiliary.Complete, Survivor: auxiliary.Survivor}, false)
		if auxiliary.Complete {
			*survivors = removeSurvivor(*survivors, auxiliary.Survivor)
		} else {
			*survivors = appendUniqueSurvivor(*survivors, auxiliary.Survivor)
		}
	}
}

func (b *stopReportBuilder) report(exitCode int) Report {
	report := Report{ExitCode: exitCode}
	for _, slot := range b.slots {
		report.Lines = append(report.Lines, slot.lines...)
	}
	return report
}

func outcomeSaysGone(line string) bool {
	return strings.Contains(line, "already gone") || strings.Contains(line, "no longer running")
}

// Transition coordinates family owners without copying their signal ladders.
type Transition struct {
	Root       string
	Checkout   string
	ScaleMilli int
	Families   []Family
	Prober     identity.Prober
	Now        func() time.Time
	Sleep      func(time.Duration)
	Self       func() (identity.Ref, error)
	ArmFunc    func() ([]string, error)
	// FenceWrite and JobRecordRead expose the existing publication and record
	// readers to focused fault-injection tests. Production uses the durable
	// stop-fence writer and dispatch record reader directly.
	FenceWrite    func(stopfence.Record) (bool, error)
	JobRecordRead func(string) (map[string]any, error)
	// AfterStep observes completion of each numbered shutdown step. Once the
	// fence is closed, a returned error is reported and recorded without
	// ending the transaction.
	AfterStep func(int) error
}

func (t *Transition) writeFence(record stopfence.Record) (bool, error) {
	if t.FenceWrite != nil {
		return t.FenceWrite(record)
	}
	return stopfence.Publish(t.Root, record)
}

func (t *Transition) readJobRecord(path string) (map[string]any, error) {
	if t.JobRecordRead != nil {
		return t.JobRecordRead(path)
	}
	return dispatch.ReadRecordObject(path)
}

func (t *Transition) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

func (t *Transition) sleep(duration time.Duration) {
	if t.Sleep != nil {
		t.Sleep(duration)
		return
	}
	time.Sleep(duration)
}

func (t *Transition) prober() identity.Prober {
	if t.Prober != nil {
		return t.Prober
	}
	return identity.KernelProber{}
}

func (t *Transition) self() (identity.Ref, error) {
	if t.Self != nil {
		return t.Self()
	}
	exact, state, err := t.prober().Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return identity.Ref{}, fmt.Errorf("the transition process identity could not be proven: %v", err)
	}
	return exact.Ref(), nil
}

func (t *Transition) scale(base time.Duration) time.Duration {
	scale := t.ScaleMilli
	if scale < 1 {
		scale = 1000
	}
	duration := time.Duration((int64(base)*int64(scale) + 999) / 1000)
	if duration < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return duration
}

// Inventory reads each family in stop order.
func (t *Transition) Inventory() ([]Item, error) {
	for _, family := range t.Families {
		if beginner, ok := family.(interface{ BeginInventory() }); ok {
			beginner.BeginInventory()
		}
	}
	var inventory []Item
	for _, family := range t.Families {
		items, err := family.Inventory()
		if err != nil {
			return nil, fmt.Errorf("%s inventory: %w", family.Name(), err)
		}
		inventory = append(inventory, items...)
	}
	return inventory, nil
}

// Status prints one present-tense attempt of every family and one independent
// fence read without taking the transition lock or changing durable state.
func (t *Transition) Status() (Report, error) {
	for _, family := range t.Families {
		if beginner, ok := family.(interface{ BeginInventory() }); ok {
			beginner.BeginInventory()
		}
	}
	type observation struct {
		item    *Item
		failure string
	}
	var observed []observation
	readFailures := 0
	liveItems := 0
	for _, family := range t.Families {
		items, err := family.Inventory()
		if err != nil {
			path, reason := inventoryFailure(err)
			source := path
			if source == "" {
				source = "records"
			}
			observed = append(observed, observation{failure: fmt.Sprintf("inventory unreadable: %s %s: %s", family.Name(), source, reason)})
			readFailures++
			continue
		}
		for index := range items {
			item := items[index]
			observed = append(observed, observation{item: &item})
			liveItems++
		}
	}
	record, fenceErr := stopfence.Read(t.Root)
	if fenceErr != nil {
		readFailures++
	}

	report := Report{Lines: []string{"checkout " + t.Checkout}}
	matched := make([]bool, len(record.NotStopped))
	for _, entry := range observed {
		if entry.failure != "" {
			report.Lines = append(report.Lines, entry.failure)
			continue
		}
		line := entry.item.StatusLine
		for index, survivor := range record.NotStopped {
			if matched[index] || !sameSurvivorIdentity(entry.item.Survivor, survivor) {
				continue
			}
			line += "; unresolved from last stop: " + statusSurvivorExplanation(survivor, t.Checkout)
			matched[index] = true
			break
		}
		report.Lines = append(report.Lines, line)
	}
	if fenceErr == nil {
		for index, survivor := range record.NotStopped {
			if !matched[index] {
				report.Lines = append(report.Lines, "unresolved from last stop: "+statusSurvivorName(survivor)+": "+statusSurvivorExplanation(survivor, t.Checkout))
			}
		}
	}

	incompleteRecord := fenceErr == nil && record.State == stopfence.StateClosed && !stopfence.Completed(record)
	if readFailures == 0 && liveItems == 0 {
		if incompleteRecord {
			report.Lines = append(report.Lines, "no live processes found; the last stop is still unresolved")
		} else {
			report.Lines = append(report.Lines, "nothing is running")
		}
	}

	prefix := ""
	if readFailures > 0 && fenceErr == nil {
		prefix = "recorded fence: "
	}
	if fenceErr != nil {
		var unreadable *stopfence.RecordUnreadableError
		reason := fenceErr.Error()
		if errors.As(fenceErr, &unreadable) {
			reason = unreadable.Reason
		}
		report.Lines = append(report.Lines, fmt.Sprintf("fence record unreadable: %s; repair with metasystem arm --repo %s", reason, t.Checkout))
	} else if record.State == stopfence.StateClosed {
		switch {
		case stopfence.Completed(record):
			report.Lines = append(report.Lines, prefix+fmt.Sprintf("stopped since %s by %s pid %d; start again: metasystem arm --repo %s", record.ChangedAt, record.By.Verb, record.By.Pid, t.Checkout))
		case record.Phase == stopfence.PhaseStopping:
			report.Lines = append(report.Lines, prefix+fmt.Sprintf("stop unfinished since %s by %s pid %d; run: metasystem stop --repo %s", record.ChangedAt, record.By.Verb, record.By.Pid, t.Checkout))
		default:
			report.Lines = append(report.Lines, prefix+fmt.Sprintf("stop incomplete for %s since %s by %s pid %d; %d unresolved entries from the last stop, listed above; run: metasystem stop --repo %s", t.Checkout, record.ChangedAt, record.By.Verb, record.By.Pid, len(record.NotStopped), t.Checkout))
		}
	}
	if readFailures > 0 {
		report.ExitCode = 1
		report.Lines = append(report.Lines, fmt.Sprintf("status incomplete for %s; %d read failures, listed above; repair the named read failures, then run: metasystem status --repo %s", t.Checkout, readFailures, t.Checkout))
	}
	return report, nil
}

// Stop closes process creation, joins older creators, and stops every family in order.
func (t *Transition) Stop() (Report, error) {
	self, err := t.self()
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	held, err := t.acquire("stop", self)
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	defer held.Release()

	previous, err := stopfence.Read(t.Root)
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	generation := previous.Generation + 1
	carriedRemote := remoteSurvivors(previous.NotStopped)
	record := stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopping,
		Generation: generation, ChangedAt: t.now().UTC().Format(time.RFC3339),
		By:       stopfence.Actor{Verb: "stop", Process: stopfence.ProcessFromRef(self)},
		Checkout: t.Checkout, NotStopped: append([]stopfence.Survivor(nil), carriedRemote...),
	}
	if _, err := t.writeFence(record); err != nil {
		return Report{ExitCode: 1}, err
	}
	builder := newStopReportBuilder(t.Checkout)
	survivors := append([]stopfence.Survivor(nil), carriedRemote...)
	for _, survivor := range t.waitForCreators(generation, record.ChangedAt, builder) {
		survivors = appendUniqueSurvivor(survivors, survivor)
	}

	items := t.inventoryDuringStop(builder, &survivors)
	for _, family := range t.Families {
		if family.Name() == "untracked" {
			continue
		}
		prefix := family.Name() + ":"
		for _, item := range items {
			if !strings.HasPrefix(item.Key, prefix) {
				continue
			}
			outcome, stopErr := family.Stop(item)
			if stopErr != nil {
				outcome = Outcome{Line: "NOT STOPPED " + item.StatusLine + ": " + stopErr.Error() + "; did: left it listed in the fence record", Survivor: item.Survivor}
			}
			builder.outcome(item, outcome, false)
			if !outcome.Complete {
				survivors = appendUniqueSurvivor(survivors, survivorFor(item, outcome))
			} else {
				survivors = removeSurvivor(survivors, item.Survivor)
			}
			recordAuxiliaryOutcomes(builder, &survivors, outcome)
		}
		t.observeFamilySteps(family.Name(), builder, &survivors)
	}

	for pass := 0; pass < 3; pass++ {
		late := t.inventoryDuringStop(builder, &survivors)
		eligible := late[:0]
		for _, item := range late {
			if item.ObserveOnly || strings.HasPrefix(item.Key, "untracked:") {
				continue
			}
			if item.FenceGeneration == nil || *item.FenceGeneration <= generation {
				eligible = append(eligible, item)
			}
		}
		if len(eligible) == 0 {
			break
		}
		for _, item := range eligible {
			arrived := !builder.seen(item)
			outcome, stopErr := t.stopItem(item)
			if stopErr != nil {
				outcome = Outcome{Line: "NOT STOPPED " + item.StatusLine + ": " + stopErr.Error() + "; did: left it listed in the fence record", Survivor: item.Survivor}
			}
			builder.outcome(item, outcome, arrived)
			if !outcome.Complete {
				survivor := survivorFor(item, outcome)
				if arrived && survivor.Reason == "" {
					survivor.Reason = "arrived during stop and remained unresolved"
				}
				survivors = appendUniqueSurvivor(survivors, survivor)
			} else {
				survivors = removeSurvivor(survivors, item.Survivor)
			}
			recordAuxiliaryOutcomes(builder, &survivors, outcome)
		}
	}
	if err := t.afterStep(8); err != nil {
		recordObserverFailure(8, err, builder, &survivors)
	}

	finalItems := t.inventoryDuringStop(builder, &survivors)
	for _, item := range finalItems {
		arrived := !builder.seen(item)
		outcome, stopErr := t.stopItem(item)
		if stopErr != nil {
			outcome = Outcome{Line: "NOT STOPPED " + item.StatusLine + ": " + stopErr.Error() + "; did: left it listed in the fence record", Survivor: item.Survivor}
		}
		builder.outcome(item, outcome, arrived)
		if !outcome.Complete {
			survivors = appendUniqueSurvivor(survivors, survivorFor(item, outcome))
		} else {
			survivors = removeSurvivor(survivors, item.Survivor)
		}
		recordAuxiliaryOutcomes(builder, &survivors, outcome)
	}
	if err := t.afterStep(9); err != nil {
		recordObserverFailure(9, err, builder, &survivors)
	}

	if len(items) == 0 && len(builder.slots) == 1 && len(survivors) == 0 {
		builder.set("empty", []string{"nothing is running"}, true)
	}
	record.NotStopped = survivors
	record.ChangedAt = t.now().UTC().Format(time.RFC3339)
	if len(survivors) == 0 {
		record.Phase = stopfence.PhaseStopped
	} else {
		record.Phase = stopfence.PhaseStopIncomplete
	}
	durable, writeErr := t.writeFence(record)
	if writeErr != nil {
		path := stopfence.TransitionPath(t.Root)
		builder.set("fence-publication", []string{fmt.Sprintf("NOT STOPPED fence record %s: final result could not be saved: %v; did: left the last published fence closed", path, writeErr)}, false)
		report := builder.report(1)
		count := len(survivors) + 1
		report.Lines = append(report.Lines, fmt.Sprintf("stop incomplete for %s; %d not stopped, listed above; final result not saved in %s; fence remains closed; repair the named write failure, then run: metasystem stop --repo %s", t.Checkout, count, path, t.Checkout))
		return report, nil
	}
	if !durable {
		builder.set("fence-durability", []string{fmt.Sprintf("fence record %s: final result published; durability unconfirmed", stopfence.TransitionPath(t.Root))}, true)
	}
	report := builder.report(0)
	appendStopClosingLine(&report, t.Checkout, len(survivors))
	return report, nil
}

func appendStopClosingLine(report *Report, checkout string, notStopped int) {
	if notStopped == 0 {
		report.ExitCode = 0
		report.Lines = append(report.Lines, fmt.Sprintf("stopped %s; start again: metasystem arm --repo %s", checkout, checkout))
		return
	}
	report.ExitCode = 1
	report.Lines = append(report.Lines, fmt.Sprintf("stop incomplete for %s; %d not stopped, listed above; run: metasystem stop --repo %s", checkout, notStopped, checkout))
}

// inventoryDuringStop keeps each family independent after the fence is
// closed: one unreadable family is a vocal NOT STOPPED result, not an exit
// that prevents later owners from winding down their processes.
func (t *Transition) inventoryDuringStop(report *stopReportBuilder, survivors *[]stopfence.Survivor) []Item {
	for _, family := range t.Families {
		if beginner, ok := family.(interface{ BeginInventory() }); ok {
			beginner.BeginInventory()
		}
	}
	var inventory []Item
	for _, family := range t.Families {
		items, err := family.Inventory()
		if err != nil {
			path, reason := inventoryFailure(err)
			if family.Name() == "job" && t.recordRemoteInventoryFailure(path, reason, report, survivors) {
				continue
			}
			candidate := stopfence.Survivor{Component: "family-inventory", ID: family.Name(), Path: path, Reason: reason}
			*survivors = removeFamilyInventorySurvivors(*survivors, family.Name())
			*survivors = appendUniqueSurvivor(*survivors, candidate)
			report.set(inventoryFailureKey(family.Name(), path), []string{inventoryFailureLine(family.Name(), path, reason)}, false)
			continue
		}
		for _, cleared := range familyInventorySurvivors(*survivors, family.Name()) {
			line := family.Name() + " records were read on retry"
			if cleared.Path != "" {
				line = fmt.Sprintf("%s inventory %s: records were read on retry", family.Name(), cleared.Path)
			}
			report.set(inventoryFailureKey(family.Name(), cleared.Path), []string{line}, true)
		}
		*survivors = removeFamilyInventorySurvivors(*survivors, family.Name())
		inventory = append(inventory, items...)
	}
	t.reconcileRemoteSurvivors(inventory, report, survivors)
	return inventory
}

func appendOutcomeLines(report *Report, outcome Outcome, suffix string) {
	report.Lines = append(report.Lines, outcome.Line+suffix)
	report.Lines = append(report.Lines, outcome.AdditionalLines...)
}

func (t *Transition) observeFamilySteps(family string, report *stopReportBuilder, survivors *[]stopfence.Survivor) {
	steps, known := map[string][]int{
		"mission":     {1},
		"job":         {2},
		"proof-run":   {3},
		"run":         {4},
		"steward":     {5},
		"supervision": {6, 7},
	}[family]
	if !known {
		err := &UnknownFamilyError{Family: family}
		survivor := stopfence.Survivor{Component: "family", ID: family, Reason: err.Error()}
		*survivors = appendUniqueSurvivor(*survivors, survivor)
		report.set("family-step:"+family, []string{fmt.Sprintf("NOT STOPPED %s shutdown step: %v; did: continued to the next stop step", family, err)}, false)
		return
	}
	for _, step := range steps {
		if err := t.afterStep(step); err != nil {
			recordObserverFailure(step, err, report, survivors)
		}
	}
}

func recordObserverFailure(step int, err error, report *stopReportBuilder, survivors *[]stopfence.Survivor) {
	survivor := stopfence.Survivor{Component: "observer", ID: "step-" + strconv.Itoa(step), Reason: err.Error()}
	*survivors = appendUniqueSurvivor(*survivors, survivor)
	report.set("observer:step-"+strconv.Itoa(step), []string{fmt.Sprintf("NOT STOPPED observer after shutdown step %d: %v; did: continued to the next stop step", step, err)}, false)
}

func (t *Transition) afterStep(step int) error {
	if t.AfterStep == nil {
		return nil
	}
	return t.AfterStep(step)
}

func (t *Transition) stopItem(item Item) (Outcome, error) {
	for _, family := range t.Families {
		if strings.HasPrefix(item.Key, family.Name()+":") {
			return family.Stop(item)
		}
	}
	return Outcome{}, fmt.Errorf("no family owns %s", item.Key)

}

func (t *Transition) waitForCreators(generation int64, fenceChangedAt string, report *stopReportBuilder) []stopfence.Survivor {
	deadline := t.now().Add(t.scale(10 * time.Second))
	closedAt, _ := time.Parse(time.RFC3339, fenceChangedAt)
	var survivors []stopfence.Survivor
	for {
		claims, problems, err := stopfence.InspectClaims(t.Root, generation)
		if err != nil {
			report.set("creator-inventory", []string{"NOT STOPPED creation-claim inventory: " + err.Error() + "; did: continued to process inventory"}, false)
			return appendUniqueSurvivor(survivors, stopfence.Survivor{Component: "family-inventory", ID: "creation-claim", Reason: err.Error()})
		}
		for _, problem := range problems {
			survivors = appendUniqueSurvivor(survivors, stopfence.Survivor{Component: "creator-claim", ID: problem.Path, Reason: problem.Err.Error()})
			line := ""
			if removeErr := stopfence.RemoveClaimFile(t.Root, problem.Path); removeErr != nil {
				line = fmt.Sprintf("NOT STOPPED creator claim %s: %v; did: removal failed: %v", problem.Path, problem.Err, removeErr)
			} else {
				line = fmt.Sprintf("NOT STOPPED creator claim %s: %v; did: removed %s", problem.Path, problem.Err, problem.Path)
			}
			report.set("creator-claim:"+problem.Path, []string{line}, false)
		}
		var alive, unknown []stopfence.CreationClaim
		for _, claim := range claims {
			switch stopfence.CreatorLiveness(claim, t.prober()) {
			case identity.Dead:
				if err := stopfence.RemoveClaimFile(t.Root, claim.Path); err != nil {
					report.set("creator-claim:"+claim.Path, []string{fmt.Sprintf("NOT STOPPED creator %s claim %s: dead claim removal failed: %v", claim.Verb, claim.Path, err)}, false)
					survivors = appendUniqueSurvivor(survivors, stopfence.Survivor{Component: "creator-claim", ID: claim.Path, Tag: claim.Verb, Reason: "dead claim removal failed: " + err.Error()})
				}
			case identity.Alive:
				alive = append(alive, claim)
			default:
				unknown = append(unknown, claim)
			}
		}
		if len(alive) == 0 && len(unknown) == 0 {
			return survivors
		}
		if !t.now().Before(deadline) {
			sort.Slice(alive, func(i, j int) bool { return alive[i].Path < alive[j].Path })
			for _, claim := range alive {
				waited := t.scale(10 * time.Second)
				line := fmt.Sprintf("NOT STOPPED creator %s pid %d started %d: still creating after %s; did: nothing", claim.Verb, claim.Creator.Pid, claim.Creator.PidStartedAt, waited)
				report.set(fmt.Sprintf("creator:%s:%d:%d", claim.Verb, claim.Creator.Pid, claim.Creator.PidStartedAt), []string{line}, false)
				survivors = appendUniqueSurvivor(survivors, stopfence.Survivor{Component: "creator", ID: claim.Verb, Pid: claim.Creator.Pid, PidStartedAt: claim.Creator.PidStartedAt, Reason: "still creating"})
			}
			sort.Slice(unknown, func(i, j int) bool { return unknown[i].Path < unknown[j].Path })
			for _, claim := range unknown {
				line := fmt.Sprintf("NOT STOPPED creator %s claim %s: liveness unknown", claim.Verb, claim.Path)
				openedAt, openedErr := time.Parse(time.RFC3339, claim.OpenedAt)
				if openedErr == nil && !closedAt.IsZero() && openedAt.Before(closedAt) {
					if err := stopfence.RemoveClaimFile(t.Root, claim.Path); err != nil {
						line += fmt.Sprintf("; did: removal failed: %v", err)
					} else {
						line += "; did: removed creation claim " + claim.Path
					}
				}
				report.set("creator-claim:"+claim.Path, []string{line}, false)
				survivors = appendUniqueSurvivor(survivors, stopfence.Survivor{Component: "creator", ID: claim.Path, Tag: claim.Verb, Pid: claim.Creator.Pid, PidStartedAt: claim.Creator.PidStartedAt, Reason: "liveness unknown"})
			}
			return survivors
		}
		t.sleep(t.scale(500 * time.Millisecond))
	}
}

// OpenFence opens a stopped checkout under the transition lock without
// starting components.
func (t *Transition) OpenFence(verb string) (int64, error) {
	self, err := t.self()
	if err != nil {
		return 0, err
	}
	held, err := t.acquire(verb, self)
	if err != nil {
		return 0, err
	}
	defer held.Release()
	return t.openFenceHeld(verb, self)
}

func (t *Transition) openFenceHeld(verb string, self identity.Ref) (int64, error) {
	record, err := stopfence.Read(t.Root)
	if err != nil {
		return 0, err
	}
	if record.State == stopfence.StateClosed && record.Phase == stopfence.PhaseStopping {
		if identity.AliveRef(t.prober(), record.By.Ref()) != identity.Dead {
			return 0, &StopInProgressError{Pid: record.By.Pid}
		}
		record.NotStopped = t.crashedStopSurvivors(record.NotStopped)
		if len(record.NotStopped) == 0 {
			record.Phase = stopfence.PhaseStopped
		} else {
			record.Phase = stopfence.PhaseStopIncomplete
		}
		record.By = stopfence.Actor{Verb: verb, Process: stopfence.ProcessFromRef(self)}
		if _, err := t.writeFence(record); err != nil {
			return 0, &FencePublicationError{Path: stopfence.TransitionPath(t.Root), Err: err}
		}
	}
	if record.Phase == stopfence.PhaseStopIncomplete && len(record.NotStopped) > 0 {
		stillLive := record.NotStopped[:0]
		for _, survivor := range record.NotStopped {
			if survivor.Component == "family-inventory" {
				// There is no process identity to probe. Only another stop can
				// prove the family readable and replace this recorded uncertainty.
				stillLive = append(stillLive, survivor)
				continue
			}
			alive, probeErr := t.survivorAlive(survivor)
			if probeErr != nil {
				return 0, probeErr
			}
			if alive {
				stillLive = append(stillLive, survivor)
			}
		}
		record.NotStopped = stillLive
		if len(stillLive) > 0 {
			survivor := stillLive[0]
			if survivor.Component == "family-inventory" {
				return 0, &UnreadableFamilySurvivorError{Family: survivor.ID, Path: survivor.Path, Reason: survivor.Reason}
			}
			if survivor.Component == "job" && survivor.ID != "" && survivor.MachineID != "" {
				return 0, &RemoteJobSurvivorError{JobID: survivor.ID, MachineID: survivor.MachineID}
			}
			if survivor.Component == "creator" && survivor.Reason == "liveness unknown" {
				return 0, &CreatorClaimSurvivorError{Verb: survivor.Tag, Path: survivor.ID, Pid: survivor.Pid}
			}
			if survivor.Pid < 1 || survivor.PidStartedAt < 1 {
				return 0, &RecordedReasonSurvivorError{Survivor: survivor}
			}
			return 0, &LocalSurvivorError{Survivor: survivor}
		}
	}
	record.State = stopfence.StateOpen
	record.Phase = stopfence.PhaseArmed
	record.Generation++
	record.ChangedAt = t.now().UTC().Format(time.RFC3339)
	record.By = stopfence.Actor{Verb: verb, Process: stopfence.ProcessFromRef(self)}
	record.Checkout = t.Checkout
	record.NotStopped = []stopfence.Survivor{}
	if _, err := t.writeFence(record); err != nil {
		return 0, err
	}
	return record.Generation, nil
}

// StopInProgressError identifies the status remedy for a live transition
// owner without baking command rendering into this package.
type StopInProgressError struct{ Pid int64 }

func (e *StopInProgressError) Error() string {
	return fmt.Sprintf("a checkout transition by pid %d holds the checkout", e.Pid)
}

// FencePublicationError names an unpublished crashed-stop re-inventory so arm
// can direct the person to repair the write and run stop instead of opening.
type FencePublicationError struct {
	Path string
	Err  error
}

func (e *FencePublicationError) Error() string {
	return fmt.Sprintf("cannot save the crashed-stop re-inventory in %s: %v; repair the named write failure", e.Path, e.Err)
}

// UnknownFamilyError keeps a newly introduced family vocal after the fence is
// closed until the stop order gives it an explicit numbered step.
type UnknownFamilyError struct{ Family string }

func (e *UnknownFamilyError) Error() string {
	return fmt.Sprintf("family %s has no numbered shutdown step", e.Family)
}

// LocalSurvivorError preserves the process coordinates required by arm's
// explicit second-stop remedy.
type LocalSurvivorError struct{ Survivor stopfence.Survivor }

func (e *LocalSurvivorError) Error() string {
	return fmt.Sprintf("%s pid %d started %d survived the last stop", e.Survivor.Component, e.Survivor.Pid, e.Survivor.PidStartedAt)
}

// UnprobeableLocalSurvivorError reports an identity whose current liveness
// could not be established without upgrading uncertainty into "alive".
type UnprobeableLocalSurvivorError struct{ Survivor stopfence.Survivor }

func (e *UnprobeableLocalSurvivorError) Error() string {
	return fmt.Sprintf("cannot establish whether %s pid %d started %d is still alive", e.Survivor.Component, e.Survivor.Pid, e.Survivor.PidStartedAt)
}

// CreatorClaimSurvivorError names the engine-owned claim whose creator could
// not be classified alive or dead.
type CreatorClaimSurvivorError struct {
	Verb string
	Path string
	Pid  int64
}

func (e *CreatorClaimSurvivorError) Error() string {
	return fmt.Sprintf("creator %s claim %s has unknown liveness", e.Verb, e.Path)
}

// UnreadableFamilySurvivorError identifies a whole process family whose
// records stop could not read and therefore could not act on.
type UnreadableFamilySurvivorError struct {
	Family string
	Path   string
	Reason string
}

func (e *UnreadableFamilySurvivorError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("cannot probe the %s survivor because %s could not be read during the last stop: %s; repair that named record before running stop", e.Family, e.Path, e.Reason)
	}
	return fmt.Sprintf("%s records could not be read during the last stop: %s", e.Family, e.Reason)
}

// RecordedReasonSurvivorError preserves a durable NOT STOPPED reason that
// has no process identity to probe, such as an unreadable claim removed by
// the preceding stop. A second stop is the only operation that can clear it.
type RecordedReasonSurvivorError struct{ Survivor stopfence.Survivor }

func (e *RecordedReasonSurvivorError) Error() string {
	name := strings.TrimSpace(e.Survivor.Component + " " + e.Survivor.ID)
	return fmt.Sprintf("cannot establish whether %s is resolved because the last stop recorded: %s", name, e.Survivor.Reason)
}

// RemoteJobSurvivorError identifies the remote cancellation required before
// the local checkout can be armed.
type RemoteJobSurvivorError struct {
	JobID     string
	MachineID string
}

func (e *RemoteJobSurvivorError) Error() string {
	return fmt.Sprintf("job %s is owned by machine %s and is not terminal", e.JobID, e.MachineID)
}

// Remedy names the machine and command that can conclude the surviving job.
func (e *RemoteJobSurvivorError) Remedy() string {
	return fmt.Sprintf("cancel it from %s with metasystem delegate --cancel %s", e.MachineID, e.JobID)
}

// RemoteJobEvidenceError distinguishes absent evidence from a record that was
// present but unreadable or did not match the recorded job and machine.
type RemoteJobEvidenceError struct {
	JobID     string
	MachineID string
	Path      string
	Kind      string
	Reason    string
}

func (e *RemoteJobEvidenceError) Error() string {
	switch e.Kind {
	case "missing":
		return fmt.Sprintf("cannot establish whether job %s on machine %s is terminal because its record %s is missing; restore that job's record from %s; if the job is still open there, cancel it there with metasystem delegate --cancel %s and restore the resulting terminal record", e.JobID, e.MachineID, e.Path, e.MachineID, e.JobID)
	case "mismatched":
		return fmt.Sprintf("cannot establish whether job %s on machine %s is terminal because record %s does not match that job and machine: %s; restore the matching record from %s", e.JobID, e.MachineID, e.Path, e.Reason, e.MachineID)
	default:
		return fmt.Sprintf("cannot establish whether job %s on machine %s is terminal because record %s could not be read: %s; repair that named record before running stop", e.JobID, e.MachineID, e.Path, e.Reason)
	}
}

func (e *RemoteJobEvidenceError) stopLine() string {
	return "NOT STOPPED " + e.Error() + "; did: left the remote job in the fence record"
}

func (t *Transition) remoteJobEvidence(survivor stopfence.Survivor) (terminal bool, evidenceErr *RemoteJobEvidenceError) {
	path := filepath.Join(t.Root, "artifacts", "agents", "jobs", survivor.ID+".json")
	if survivor.ID == "" || filepath.Base(survivor.ID) != survivor.ID {
		return false, &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "mismatched", Reason: "the recorded job id is invalid"}
	}
	record, err := t.readJobRecord(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "missing"}
	}
	if err != nil {
		if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
			return false, &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "missing"}
		}
		return false, &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "unreadable", Reason: err.Error()}
	}
	lens := dispatch.JobRecordOf(record)
	if lens.JobID() != survivor.ID || lens.MachineID() != survivor.MachineID {
		return false, &RemoteJobEvidenceError{
			JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "mismatched",
			Reason: fmt.Sprintf("record names job %q on machine %q", lens.JobID(), lens.MachineID()),
		}
	}
	return dispatch.TerminalStatus(lens.Status()), nil
}

func (t *Transition) survivorAlive(survivor stopfence.Survivor) (bool, error) {
	if survivor.MachineID != "" {
		// A remote job cannot be probed from this host. Its family record must
		// become terminal before arm may treat it as ended.
		if survivor.Component == "job" {
			terminal, evidenceErr := t.remoteJobEvidence(survivor)
			if evidenceErr != nil {
				return true, evidenceErr
			}
			return !terminal, nil
		}
		return true, nil
	}
	if survivor.Pid < 1 || survivor.PidStartedAt < 1 {
		return true, nil
	}
	state := identity.AliveRef(t.prober(), identity.Ref{Pid: survivor.Pid, StartedAtSec: survivor.PidStartedAt})
	switch state {
	case identity.Dead:
		return false, nil
	case identity.Alive:
		return true, nil
	default:
		if survivor.Component == "creator" && survivor.Reason == "liveness unknown" {
			return true, &CreatorClaimSurvivorError{Verb: survivor.Tag, Path: survivor.ID, Pid: survivor.Pid}
		}
		return true, &UnprobeableLocalSurvivorError{Survivor: survivor}
	}
}

// Arm opens the fence and then starts the process rings. A launch failure leaves it open.
func (t *Transition) Arm() (Report, error) {
	self, err := t.self()
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	held, err := t.acquire("arm", self)
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	defer held.Release()
	generation, replacement, err := t.armFenceHeld(self)
	if err != nil {
		return Report{ExitCode: 1}, err
	}
	report := Report{Lines: []string{"checkout " + t.Checkout}}
	if replacement != "" {
		report.Lines = append(report.Lines, replacement)
	}
	if t.ArmFunc != nil {
		lines, armErr := t.ArmFunc()
		report.Lines = append(report.Lines, lines...)
		if armErr != nil {
			report.ExitCode = 1
			return report, armErr
		}
	}
	report.Lines = append(report.Lines, fmt.Sprintf("armed %s generation %d", t.Checkout, generation))
	return report, nil
}

func (t *Transition) acquire(verb string, self identity.Ref) (*lock.Lock, error) {
	held, err := stopfence.Acquire(t.Root, verb, self, t.ScaleMilli)
	var holder *lock.HolderError
	if errors.As(err, &holder) && holder.State == lock.Alive {
		return nil, &StopInProgressError{Pid: holder.Holder.Pid}
	}
	return held, err
}

func (t *Transition) crashedStopSurvivors(previous []stopfence.Survivor) []stopfence.Survivor {
	for _, family := range t.Families {
		if beginner, ok := family.(interface{ BeginInventory() }); ok {
			beginner.BeginInventory()
		}
	}
	survivors := remoteSurvivors(previous)
	var inventory []Item
	for _, family := range t.Families {
		items, err := family.Inventory()
		if err != nil {
			path, reason := inventoryFailure(err)
			if family.Name() == "job" {
				matched := false
				for _, survivor := range remoteSurvivors(survivors) {
					want := filepath.Join(t.Root, "artifacts", "agents", "jobs", survivor.ID+".json")
					if filepath.Clean(path) != filepath.Clean(want) {
						continue
					}
					matched = true
					evidenceErr := &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "unreadable", Reason: reason}
					survivor.Path = path
					survivor.Reason = evidenceErr.Error()
					survivors = appendUniqueSurvivor(survivors, survivor)
				}
				if matched {
					continue
				}
			}
			survivors = appendUniqueSurvivor(survivors, stopfence.Survivor{
				Component: "family-inventory", ID: family.Name(), Path: path, Reason: reason,
			})
			continue
		}
		for _, item := range items {
			inventory = append(inventory, item)
			if item.ObserveOnly || strings.HasPrefix(item.Key, "untracked:") {
				continue
			}
			survivor := item.Survivor
			if survivor.Component == "job" && survivor.MachineID != "" {
				survivor.Reason = fmt.Sprintf("job remains non-terminal on machine %s; cancel it from %s with metasystem delegate --cancel %s, then restore its terminal record", survivor.MachineID, survivor.MachineID, survivor.ID)
			}
			survivors = appendUniqueSurvivor(survivors, survivor)
		}
	}
	for _, survivor := range remoteSurvivors(survivors) {
		represented := false
		for _, item := range inventory {
			if sameSurvivorIdentity(item.Survivor, survivor) {
				represented = true
				break
			}
		}
		if represented {
			continue
		}
		terminal, evidenceErr := t.remoteJobEvidence(survivor)
		if terminal {
			survivors = removeSurvivor(survivors, survivor)
			continue
		}
		if evidenceErr != nil {
			survivor.Path = evidenceErr.Path
			survivor.Reason = evidenceErr.Error()
		} else {
			survivor.Reason = fmt.Sprintf("job remains non-terminal on machine %s; cancel it from %s with metasystem delegate --cancel %s, then restore its terminal record", survivor.MachineID, survivor.MachineID, survivor.ID)
		}
		survivors = appendUniqueSurvivor(survivors, survivor)
	}
	return survivors
}

func (t *Transition) armFenceHeld(self identity.Ref) (int64, string, error) {
	generation, err := t.openFenceHeld("arm", self)
	if err == nil {
		return generation, "", nil
	}
	var unreadable *stopfence.RecordUnreadableError
	if !errors.As(err, &unreadable) {
		return 0, "", err
	}
	highest := t.highestDurableGeneration(unreadable.HighestGeneration)
	record := stopfence.Record{
		State: stopfence.StateOpen, Phase: stopfence.PhaseArmed,
		Generation: highest + 1,
		ChangedAt:  t.now().UTC().Format(time.RFC3339),
		By:         stopfence.Actor{Verb: "arm", Process: stopfence.ProcessFromRef(self)},
		Checkout:   t.Checkout, NotStopped: []stopfence.Survivor{},
	}
	if _, writeErr := t.writeFence(record); writeErr != nil {
		return 0, "", writeErr
	}
	return record.Generation, "fence record replaced (generation counter recovered; was unreadable: " + unreadable.Reason + ")", nil
}

func (t *Transition) highestDurableGeneration(highest int64) int64 {
	if owner, err := supervise.ReadArmingOwner(t.Root); err == nil && owner.FenceGeneration > highest {
		highest = owner.FenceGeneration
	}
	if state, err := supervise.ReadPublishedGeneration(t.Root); err == nil && state.Generation > highest {
		highest = state.Generation
	}
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return highest
	}
	frames, err := registry.ReadFrames(registryPath)
	if err != nil {
		return highest
	}
	for _, frame := range frames {
		if frame.Record == nil {
			continue
		}
		checkout, _ := frame.Record["checkoutPath"].(string)
		if !sameDurableCheckout(checkout, t.Checkout) {
			continue
		}
		if generation, ok := durableGeneration(frame.Record["generation"]); ok && generation > highest {
			highest = generation
		}
	}
	return highest
}

func sameDurableCheckout(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftPhysical, leftErr := filepath.EvalSymlinks(left)
	rightPhysical, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && leftPhysical == rightPhysical
}

func durableGeneration(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		generation := int64(typed)
		return generation, typed >= 0 && float64(generation) == typed
	case int64:
		return typed, typed >= 0
	case int:
		return int64(typed), typed >= 0
	default:
		return 0, false
	}
}

func remoteSurvivors(all []stopfence.Survivor) []stopfence.Survivor {
	var kept []stopfence.Survivor
	for _, survivor := range all {
		if survivor.Component == "job" && survivor.ID != "" && survivor.MachineID != "" {
			kept = appendUniqueSurvivor(kept, survivor)
		}
	}
	return kept
}

func (t *Transition) reconcileRemoteSurvivors(items []Item, report *stopReportBuilder, survivors *[]stopfence.Survivor) {
	for _, survivor := range remoteSurvivors(*survivors) {
		represented := false
		for _, item := range items {
			if sameSurvivorIdentity(item.Survivor, survivor) {
				represented = true
				break
			}
		}
		if represented {
			continue
		}
		terminal, evidenceErr := t.remoteJobEvidence(survivor)
		item := Item{Key: "job:" + survivor.ID, StatusLine: fmt.Sprintf("job %s machine %s", survivor.ID, survivor.MachineID), Survivor: survivor}
		if terminal {
			report.outcome(item, Outcome{Line: fmt.Sprintf("job %s machine %s: terminal record read", survivor.ID, survivor.MachineID), Complete: true}, false)
			*survivors = removeSurvivor(*survivors, survivor)
			continue
		}
		if evidenceErr != nil {
			survivor.Path = evidenceErr.Path
			survivor.Reason = evidenceErr.Error()
			*survivors = appendUniqueSurvivor(*survivors, survivor)
			item.Survivor = survivor
			report.outcome(item, Outcome{Line: evidenceErr.stopLine(), Complete: false, Survivor: survivor}, false)
			continue
		}
		survivor.Reason = fmt.Sprintf("job remains non-terminal on machine %s; cancel it from %s with metasystem delegate --cancel %s, then restore its terminal record", survivor.MachineID, survivor.MachineID, survivor.ID)
		*survivors = appendUniqueSurvivor(*survivors, survivor)
		item.Survivor = survivor
		report.outcome(item, Outcome{Line: "NOT STOPPED " + item.StatusLine + ": " + survivor.Reason + "; did: left it in the fence record", Complete: false, Survivor: survivor}, false)
	}
}

func (t *Transition) recordRemoteInventoryFailure(path, reason string, report *stopReportBuilder, survivors *[]stopfence.Survivor) bool {
	matched := false
	for _, survivor := range remoteSurvivors(*survivors) {
		want := filepath.Join(t.Root, "artifacts", "agents", "jobs", survivor.ID+".json")
		if filepath.Clean(path) != filepath.Clean(want) {
			continue
		}
		matched = true
		evidenceErr := &RemoteJobEvidenceError{JobID: survivor.ID, MachineID: survivor.MachineID, Path: path, Kind: "unreadable", Reason: reason}
		survivor.Path = path
		survivor.Reason = evidenceErr.Error()
		*survivors = appendUniqueSurvivor(*survivors, survivor)
		item := Item{Key: "job:" + survivor.ID, Survivor: survivor}
		report.outcome(item, Outcome{Line: evidenceErr.stopLine(), Complete: false, Survivor: survivor}, false)
	}
	return matched
}

func statusSurvivorName(survivor stopfence.Survivor) string {
	switch {
	case survivor.Component == "family-inventory" && survivor.Path != "":
		return survivor.ID + " " + survivor.Path
	case survivor.Component == "job" && survivor.MachineID != "":
		return fmt.Sprintf("job %s machine %s", survivor.ID, survivor.MachineID)
	case survivor.Pid > 0:
		name := strings.TrimSpace(survivor.Component + " " + survivor.ID)
		if survivor.PidStartedAt > 0 {
			return fmt.Sprintf("%s pid %d started %d", name, survivor.Pid, survivor.PidStartedAt)
		}
		return fmt.Sprintf("%s pid %d", name, survivor.Pid)
	case survivor.Path != "":
		return strings.TrimSpace(survivor.Component + " " + survivor.Path)
	default:
		return strings.TrimSpace(survivor.Component + " " + survivor.ID)
	}
}

func statusSurvivorExplanation(survivor stopfence.Survivor, checkout string) string {
	reason := survivor.Reason
	if reason == "" {
		reason = "unresolved by the last stop"
	}
	if survivor.Component == "job" && survivor.MachineID != "" && !strings.Contains(reason, "metasystem delegate --cancel") {
		reason += fmt.Sprintf("; cancel it from %s with metasystem delegate --cancel %s and restore its terminal record", survivor.MachineID, survivor.ID)
	}
	if survivor.Component == "family-inventory" && survivor.Path != "" && !strings.Contains(reason, "repair") {
		reason += "; repair the named record, then run: metasystem stop --repo " + checkout
	}
	return reason
}

func survivorIdentity(survivor stopfence.Survivor, fallback string) string {
	if survivor.Component == "" {
		return fallback
	}
	if survivor.Component == "job" && survivor.MachineID != "" {
		return strings.Join([]string{survivor.Component, survivor.ID, survivor.MachineID}, "|")
	}
	if survivor.Pid > 0 || survivor.PidStartedAt > 0 {
		return fmt.Sprintf("%s|%s|%s|%d|%d|%s", survivor.Component, survivor.ID, survivor.MachineID, survivor.Pid, survivor.PidStartedAt, survivor.Tag)
	}
	if survivor.Path != "" {
		return strings.Join([]string{survivor.Component, survivor.ID, survivor.Path}, "|")
	}
	return strings.Join([]string{survivor.Component, survivor.ID, survivor.MachineID, survivor.Tag}, "|")
}

func sameSurvivorIdentity(left, right stopfence.Survivor) bool {
	if left.Component == "" || right.Component == "" {
		return false
	}
	return survivorIdentity(left, "") == survivorIdentity(right, "")
}

func inventoryFailure(err error) (string, string) {
	var readErr *InventoryReadError
	if errors.As(err, &readErr) {
		return readErr.Path, readErr.Error()
	}
	return "", err.Error()
}

func inventoryFailureKey(family, path string) string {
	return "inventory:" + family + ":" + path
}

func familyInventorySurvivors(all []stopfence.Survivor, family string) []stopfence.Survivor {
	var found []stopfence.Survivor
	for _, survivor := range all {
		if survivor.Component == "family-inventory" && survivor.ID == family {
			found = append(found, survivor)
		}
	}
	return found
}

func inventoryFailureLine(family, path, reason string) string {
	if path != "" {
		return fmt.Sprintf("NOT STOPPED %s inventory %s: %s; did: continued to the next stop step", family, path, reason)
	}
	return fmt.Sprintf("NOT STOPPED %s inventory: %s; did: continued to the next stop step", family, reason)
}

func removeFamilyInventorySurvivors(all []stopfence.Survivor, family string) []stopfence.Survivor {
	kept := all[:0]
	for _, survivor := range all {
		if survivor.Component == "family-inventory" && survivor.ID == family {
			continue
		}
		kept = append(kept, survivor)
	}
	return kept
}

func survivorFor(item Item, outcome Outcome) stopfence.Survivor {
	if outcome.Survivor.Component != "" {
		return outcome.Survivor
	}
	return item.Survivor
}

func appendUniqueSurvivor(all []stopfence.Survivor, candidate stopfence.Survivor) []stopfence.Survivor {
	for index, survivor := range all {
		if sameSurvivorIdentity(survivor, candidate) {
			all[index] = candidate
			return all
		}
	}
	return append(all, candidate)
}

func removeSurvivor(all []stopfence.Survivor, candidate stopfence.Survivor) []stopfence.Survivor {
	kept := all[:0]
	for _, survivor := range all {
		if sameSurvivorIdentity(survivor, candidate) {
			continue
		}
		kept = append(kept, survivor)
	}
	return kept
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
