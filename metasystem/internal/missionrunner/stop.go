package missionrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

const (
	ItemRunner = "mission-runner"
	ItemTurn   = "mission-turn"

	TerminationAlreadyGone = "already-gone"
	TerminationTerm        = "term"
	TerminationKill        = "kill"
)

// Item is one live mission process established from its durable record and
// its tag-proven kernel identity.
type Item struct {
	Kind            string
	Root            string
	MissionID       string
	TurnID          string
	Runtime         string
	Status          string
	RecordPath      string
	FenceGeneration int64
	Pid             int64
	PidStartedAt    int64
	PidStartTicks   int64
	BootID          string
	Pgid            int64
	Tag             string
	Liveness        string
}

func (i Item) ref() identity.Ref {
	return identity.Ref{Pid: i.Pid, StartedAtSec: i.PidStartedAt, StartTicks: i.PidStartTicks, BootID: i.BootID}
}

// StopOptions carries the one fact the caller learns while stopping the
// preceding runner: whether the runner itself concluded the host turn.
type StopOptions struct {
	RunnerConcluded bool
}

// StopOutcome is the typed process result consumed by the aggregate stop
// report.
type StopOutcome struct {
	Item     Item
	Result   string
	Signal   string
	Reason   string
	ByRunner bool
}

// Inventory lists live runners first and independently lists every owned,
// non-terminal host turn. A turn remains visible even when its runner record
// is dead or missing.
func Inventory(root string) ([]Item, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return nil, err
	}
	items := []Item{}
	runnerPaths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "missions", "runners", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(runnerPaths)
	for _, path := range runnerPaths {
		doc, readErr := readJSONDoc(path)
		if readErr != nil {
			return nil, fmt.Errorf("mission runner record is unreadable: %s: %w", path, readErr)
		}
		if valueString(doc["status"]) != "running" {
			continue
		}
		item, ok := itemFromRecord(ItemRunner, root, strings.TrimSuffix(filepath.Base(path), ".json"), "", path, doc)
		if !ok {
			return nil, fmt.Errorf("mission runner record has an invalid process identity: %s", path)
		}
		liveness := runnerItemLiveness(item, authorization)
		item.Liveness = liveness.String()
		if liveness == identity.Unknown {
			return nil, fmt.Errorf("mission runner identity is uninspectable: %s", path)
		}
		if liveness == identity.Alive {
			items = append(items, item)
		}
	}
	turnPaths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "missions", "*", "turns", "*", "turn.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(turnPaths)
	for _, path := range turnPaths {
		doc, readErr := readJSONDoc(path)
		if readErr != nil {
			return nil, fmt.Errorf("mission turn record is unreadable: %s: %w", path, readErr)
		}
		if !staleTurnOpen(doc) {
			continue
		}
		if pgid, ok := jsonInt(doc["pgid"]); !ok || pgid < 2 || valueString(doc["instanceTag"]) == "" {
			// A pending turn has no host process yet and is therefore not a
			// process inventory item.
			continue
		}
		missionID := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		turnID := filepath.Base(filepath.Dir(path))
		item, ok := itemFromRecord(ItemTurn, root, missionID, turnID, path, doc)
		if !ok || item.Pgid < 2 || item.Tag == "" {
			return nil, fmt.Errorf("mission turn record has an invalid process identity: %s", path)
		}
		if !groupAlive(int(item.Pgid)) {
			continue
		}
		grant := fixtureauth.GroupOwnershipGrant{}
		if item.Runtime == "fake" {
			grant = authorization.GroupOwnership()
		}
		switch groupOwnership(int(item.Pgid), item.Tag, grant) {
		case janitor.GroupOwned:
			item.Liveness = identity.Alive.String()
			items = append(items, item)
		case janitor.GroupIndeterminate:
			return nil, fmt.Errorf("mission turn group identity is uninspectable: %s", path)
		}
	}
	return items, nil
}

func itemFromRecord(kind, root, missionID, turnID, path string, doc map[string]any) (Item, bool) {
	pid, pidOK := jsonInt(doc["pid"])
	started, startedOK := jsonInt(doc["pidStartedAt"])
	pgid, pgidOK := jsonInt(doc["pgid"])
	tag, tagOK := doc["instanceTag"].(string)
	if !pidOK || !startedOK || !pgidOK || !tagOK || pid < 1 || pgid < 1 || tag == "" {
		return Item{}, false
	}
	item := Item{Kind: kind, Root: root, MissionID: missionID, TurnID: turnID,
		Runtime: valueString(doc["runtime"]), Status: valueString(doc["status"]), RecordPath: path,
		Pid: pid, PidStartedAt: started, Pgid: pgid, Tag: tag}
	item.PidStartTicks, _ = jsonInt(doc["pidStartTicks"])
	item.BootID, _ = doc["bootId"].(string)
	item.FenceGeneration, _ = jsonInt(doc["fenceGeneration"])
	if item.ref().Mode() == identity.CompareInvalid {
		return Item{}, false
	}
	return item, true
}

func runnerItemLiveness(item Item, authorization *fixtureauth.Authorization) identity.Liveness {
	state := identity.AliveRef(identity.KernelProber{}, item.ref())
	if state != identity.Alive {
		return state
	}
	actual, err := unix.Getpgid(int(item.Pid))
	if err != nil || int64(actual) != item.Pgid {
		return identity.Dead
	}
	grant := fixtureauth.GroupOwnershipGrant{}
	if authorization != nil && item.Runtime == "fake" {
		grant = authorization.GroupOwnership()
	}
	switch groupOwnership(int(item.Pgid), item.Tag, grant) {
	case janitor.GroupOwned:
		return identity.Alive
	case janitor.GroupIndeterminate:
		return identity.Unknown
	default:
		return identity.Dead
	}
}

var stopSignal = func(pid int, signal syscall.Signal) error { return unix.Kill(pid, signal) }

// Stop ends one inventoried mission process. It never signals without the
// record identity and positioned tag proof represented by the item.
func Stop(item Item, options StopOptions) (StopOutcome, error) {
	switch item.Kind {
	case ItemRunner:
		return stopRunner(item)
	case ItemTurn:
		return stopTurn(item, options.RunnerConcluded)
	default:
		return StopOutcome{}, fmt.Errorf("unknown mission stop item %q", item.Kind)
	}
}

func stopRunner(item Item) (StopOutcome, error) {
	outcome := StopOutcome{Item: item, Signal: TerminationAlreadyGone, Result: "already-gone"}
	authorization, err := fixtureauth.New(item.Root)
	if err != nil {
		return outcome, err
	}
	state := runnerItemLiveness(item, authorization)
	if state == identity.Unknown {
		outcome.Result, outcome.Reason = "not-stopped", "runner identity is uninspectable"
		return outcome, nil
	}
	if state == identity.Dead {
		return outcome, finishDeadRunner(item)
	}
	requester, requesterState, requesterErr := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if requesterErr != nil || requesterState != identity.Alive {
		return outcome, fmt.Errorf("cannot establish stop requester identity: %v", requesterErr)
	}
	intent := map[string]any{
		"schemaVersion": 1, "missionId": item.MissionID, "requestedAt": nowISO(),
		"runner": processFields(item), "requester": refFields(requester.Ref()),
	}
	intentPath := filepath.Join(item.Root, "artifacts", "agents", "missions", item.MissionID, "stop-intent.json")
	if err := atomicWriteJSON(intentPath, intent); err != nil {
		return outcome, err
	}
	if err := stopSignal(-int(item.Pgid), syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return outcome, err
	}
	termWait, err := ScaledWait(10)
	if err != nil {
		return outcome, err
	}
	if waitRunnerGone(item, authorization, termWait) {
		outcome.Result, outcome.Signal, outcome.Reason = "stopped", TerminationTerm, "runner-concluded"
		return outcome, nil
	}
	if runnerItemLiveness(item, authorization) == identity.Alive {
		if err := stopSignal(-int(item.Pgid), syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return outcome, err
		}
	}
	killWait, err := ScaledWait(1)
	if err != nil {
		return outcome, err
	}
	if !waitRunnerGone(item, authorization, killWait) {
		outcome.Result, outcome.Signal, outcome.Reason = "not-stopped", TerminationKill, "runner survived SIGKILL"
		return outcome, nil
	}
	outcome.Result, outcome.Signal, outcome.Reason = "stopped", TerminationKill, "TERM ignored"
	return outcome, finishDeadRunner(item)
}

func waitRunnerGone(item Item, authorization *fixtureauth.Authorization, limit time.Duration) bool {
	poll, err := Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 50)
	if err != nil {
		poll = 50 * time.Millisecond
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if runnerItemLiveness(item, authorization) == identity.Dead {
			return true
		}
		time.Sleep(poll)
	}
	return runnerItemLiveness(item, authorization) == identity.Dead
}

func stopTurn(item Item, byRunner bool) (StopOutcome, error) {
	outcome := StopOutcome{Item: item, ByRunner: byRunner}
	if item.Liveness == identity.Unknown.String() {
		outcome.Result, outcome.Reason = "not-stopped", "host group identity is uninspectable"
		return outcome, nil
	}
	if current, err := readJSONDoc(item.RecordPath); err == nil {
		if termination := valueString(current["hostTermination"]); termination != "" && !staleTurnOpen(current) {
			outcome.Result, outcome.Signal = "stopped", termination
			return outcome, nil
		}
	}
	engine := NewEngine(item.Root, item.MissionID)
	termination, err := engine.terminateGroup(int(item.Pgid), item.Tag, item.Runtime == "fake")
	if err != nil {
		outcome.Result, outcome.Signal, outcome.Reason = "not-stopped", termination, err.Error()
		return outcome, nil
	}
	outcome.Signal = termination
	if groupAlive(int(item.Pgid)) && groupHasSubstantiveMember(int(item.Pgid)) {
		outcome.Result, outcome.Reason = "not-stopped", "host group death is unproven"
		return outcome, nil
	}
	outcome.Result = "stopped"
	if _, err := patchTurn(item.RecordPath, map[string]any{
		"status": "failed", "outcome": "failed", "error": "turn-lost",
		"detail": "stopped by metasystem stop", "hostTermination": termination,
		"endedAt": nowISO(), "hostEndedAt": nowISO(),
	}); err != nil {
		return outcome, err
	}
	return outcome, nil
}

func finishDeadRunner(item Item) error {
	engine := NewEngine(item.Root, item.MissionID)
	turns, err := Inventory(item.Root)
	if err != nil {
		return err
	}
	for _, turn := range turns {
		if turn.Kind == ItemTurn && turn.MissionID == item.MissionID {
			if _, err := stopTurn(turn, false); err != nil {
				return err
			}
		}
	}
	record, err := readDocLabeled(item.RecordPath, "mission runner record", 3)
	if err == nil && sameRecordIdentity(record, item) {
		record["status"] = "stopped"
		record["error"] = nil
		record["endedAt"] = nowISO()
		if err := atomicWriteJSON(item.RecordPath, record); err != nil {
			return err
		}
	}
	if err := engine.releaseDeadRunnerLease(item); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(item.Root, "artifacts", "agents", "missions", item.MissionID, "stop-intent.json"))
	return nil
}

func (e *Engine) releaseDeadRunnerLease(item Item) error {
	dir := e.missionDir()
	marker := filepath.Join(dir, "lease.d")
	leasePath := filepath.Join(dir, "lease.json")
	release, err := lease.LockBounded(filepath.Join(dir, "lease.lock"), "mission lease")
	if err != nil {
		return err
	}
	defer release()
	if pathExists(leasePath) {
		doc, err := readDocLabeled(leasePath, "mission lease", 3)
		if err != nil {
			return err
		}
		pid, _ := jsonInt(doc["pid"])
		tag, _ := doc["instanceTag"].(string)
		if pid != item.Pid || tag != item.Tag {
			return fmt.Errorf("mission lease belongs to a different runner")
		}
	}
	return clearLeaseFilesLocked(marker, leasePath, pathExists(marker))
}

func sameRecordIdentity(record map[string]any, item Item) bool {
	pid, _ := jsonInt(record["pid"])
	started, _ := jsonInt(record["pidStartedAt"])
	tag, _ := record["instanceTag"].(string)
	return pid == item.Pid && started == item.PidStartedAt && tag == item.Tag
}

func processFields(item Item) map[string]any {
	return map[string]any{"pid": item.Pid, "pidStartedAt": item.PidStartedAt,
		"pidStartTicks": item.PidStartTicks, "bootId": item.BootID, "pgid": item.Pgid, "instanceTag": item.Tag}
}

func refFields(ref identity.Ref) map[string]any {
	return map[string]any{"pid": ref.Pid, "pidStartedAt": ref.StartedAtSec,
		"pidStartedAtMicro": ref.StartedAtUnixMicro, "pidStartTicks": ref.StartTicks, "bootId": ref.BootID}
}

type runnerStoppedError struct{}

func (e *runnerStoppedError) Error() string { return "stopped by metasystem stop" }

func (e *Engine) checkStopNotification(turnID any) error {
	if e.stopNotifications == nil {
		return nil
	}
	select {
	case <-e.stopNotifications:
	default:
		return nil
	}
	recordPath, _, _ := e.runnerPaths()
	record, err := readDocLabeled(recordPath, "mission runner record", 3)
	if err != nil {
		return failf(3, "mission runner terminated without a readable stop intent")
	}
	intentPath := filepath.Join(e.missionDir(), "stop-intent.json")
	intent, err := readDocLabeled(intentPath, "mission stop intent", 3)
	if err != nil || intent["missionId"] != e.Mission || !intentMatchesRunner(intent, record) {
		return failf(3, "mission runner terminated without a matching stop intent")
	}
	termination := TerminationAlreadyGone
	if turn := e.activeHost; turn != nil && turn.pid > 0 && turn.process != nil && !turn.process.exited() {
		termination, err = e.terminateGroup(turn.pid, turn.tag, turn.fakeRuntime)
		if err != nil {
			return err
		}
	} else if id, ok := turnID.(string); ok && id != "" {
		path := filepath.Join(e.missionDir(), "turns", id, "turn.json")
		turn, readErr := readJSONDoc(path)
		if readErr == nil && staleTurnOpen(turn) {
			pgid, pgidOK := jsonInt(turn["pgid"])
			tag, tagOK := turn["instanceTag"].(string)
			if pgidOK && tagOK {
				termination, err = e.terminateGroup(int(pgid), tag, turn["runtime"] == "fake")
				if err != nil {
					return err
				}
			}
		}
	}
	if id, ok := turnID.(string); ok && id != "" {
		path := filepath.Join(e.missionDir(), "turns", id, "turn.json")
		if pathExists(path) {
			if _, err := patchTurn(path, map[string]any{
				"status": "failed", "outcome": "failed", "error": "turn-lost",
				"detail": "stopped by metasystem stop", "hostTermination": termination,
				"endedAt": nowISO(), "hostEndedAt": nowISO(),
			}); err != nil {
				return err
			}
		}
	}
	_ = os.Remove(intentPath)
	return &runnerStoppedError{}
}

func intentMatchesRunner(intent, record map[string]any) bool {
	runner, ok := intent["runner"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"pid", "pidStartedAt", "pidStartTicks", "bootId", "pgid", "instanceTag"} {
		if valueString(runner[key]) != valueString(record[key]) {
			return false
		}
	}
	return true
}

func currentProcessRef() (identity.Ref, error) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return identity.Ref{}, fmt.Errorf("cannot establish creator identity: %v", err)
	}
	return exact.Ref(), nil
}

func fenceRefusal(root string, record stopfence.Record) error {
	description, err := stopfence.ClosedDescription(record, root)
	if err != nil {
		return err
	}
	command, err := stopfence.ClosedCommand(record, root)
	if err != nil {
		return err
	}
	return failf(3, "%s\nat an agent-free terminal, run: %s", description, command)
}

func rearmedCreationError(root, thing string) error {
	return failf(3, "the checkout %s was stopped and armed again while %s started; %s has been ended; the caller may retry", root, thing, thing)
}
