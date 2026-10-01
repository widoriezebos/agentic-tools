package main

// The landing agent's launcher (landing-lane-runtime-redesign §3, unit A-a):
// the steward of the host's registered lane checkout wakes one landing agent
// on demand, as the launch lane's landing kind on the seat's machinery. The
// kind, its lineage, its fence and the lane guard are internal/launch's; the
// wake decision is internal/landing/lane's; this file wires them to the
// launch manager, the lane installation's own settings and the outage mark.
// The agent's procedure is the landing-agent skill.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// landingLaneCheckout reads the host's registered lane for the launcher's
// lane guard: its checkout and its module root.
func landingLaneCheckout(home func() (string, error)) func() (launch.LaneCheckout, error) {
	return func() (launch.LaneCheckout, error) {
		laneHome, err := home()
		if err != nil {
			// A host with no home for the lane has no lane.
			return launch.LaneCheckout{}, nil
		}
		record, ok, incomplete, err := lane.ReadGuarded(laneHome)
		if err != nil || !ok {
			return launch.LaneCheckout{}, err
		}
		if incomplete != nil {
			// An older engine's record names no installation: the guard
			// holds on the checkout and its nested installation folder,
			// both, without probing either.
			return launch.LaneCheckout{Registered: true, Checkout: record.Root, Module: filepath.Join(record.Root, "metasystem"), Incomplete: incomplete}, nil
		}
		layout, err := record.Layout()
		if err != nil {
			return launch.LaneCheckout{}, err
		}
		return launch.LaneCheckout{Registered: true, Checkout: string(layout.Checkout), Module: string(layout.Install)}, nil
	}
}

// landingAgent starts, finds and reaps the landing agent through the launch
// manager the work verbs use.
type landingAgent struct {
	manager func() *launch.Manager
	// settings are the launch settings of the installation at a state root.
	settings func(stateRoot string) (launch.Settings, error)
	now      func() time.Time
	nonce    func() (string, error)
	// hold makes a started launch's agent the lane installation's holder;
	// nil is holdLaneInstallation.
	hold func(module string, record launch.Record) error
}

func newLandingAgent() landingAgent {
	return landingAgent{manager: func() *launch.Manager { return launchManager() }, settings: installationSettings, now: time.Now, nonce: landingNonce}
}

func landingNonce() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// running names a landing launch on this computer that has not ended; the
// manager's list reconciles a launch whose processes are gone.
func (a landingAgent) running() (string, bool, error) {
	records, err := a.manager().List()
	if err != nil {
		return "", false, err
	}
	for _, record := range records {
		if record.Kind == launch.LandingKind && !record.State.Terminal() {
			return record.ID, true, nil
		}
	}
	return "", false, nil
}

// landingAgentBrief is what the landing agent reads on stdin. Its procedure
// is the landing-agent skill (unit A-b); the brief names why it was woken
// and where the lane's state is.
var landingAgentBrief = func(root string, wake lane.Wake) string {
	return fmt.Sprintf(`# Landing agent

You are this computer's landing agent, in the landing lane %s. Follow the landing-agent skill.

The keeper woke you for: %s.
metasystem landing status --json shows the lane's state and its wake reasons; stop when none is left.
`, root, strings.Join(wake.Reasons, ", "))
}

// start stages the brief in the lane installation and starts the landing
// kind in the lane checkout on that installation's own settings (the
// roster keys of its metasystem.conf.local), bound to its fence.
func (a landingAgent) start(root string, wake lane.Wake) (string, error) {
	module := batch.ModuleRoot(root)
	settings, err := a.settings(module)
	if err != nil {
		return "", err
	}
	nonce, err := a.nonce()
	if err != nil {
		return "", err
	}
	id := "landing-" + nonce
	dir := filepath.Join(module, "artifacts", "agents", "landing-agent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	brief := filepath.Join(dir, id+".brief.md")
	file, err := os.OpenFile(brief, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.WriteString(landingAgentBrief(root, wake))
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return "", err
	}
	manager := *a.manager()
	manager.Settings, manager.SettingsError = settings, nil
	spec := launch.StartSpec{ID: id, Kind: launch.LandingKind, WorkingDirectory: root, FenceRoot: module, Brief: brief, Tag: nonce}
	record, err := manager.Start(spec)
	if err != nil {
		// A start that did not happen leaves no brief behind.
		return "", errors.Join(err, os.Remove(brief))
	}
	// The launch, not the agent, makes the agent the lane installation's
	// holder (2026-10-01): its SessionStart hook may skip arming (an engine
	// rebuilding), and an agent that is not the holder can prove, push and
	// return nothing. Start returns once the child exists, before its model
	// has answered once, so the lease is held before its first tool call.
	hold := a.hold
	if hold == nil {
		hold = holdLaneInstallation
	}
	if err := hold(module, record); err != nil {
		// An agent that cannot act on the lane is not left running.
		_, cancelErr := manager.Cancel(record.ID)
		return "", errors.Join(fmt.Errorf("landing agent %s could not take the lane installation's lease, so it was stopped: %w", record.ID, err), cancelErr)
	}
	return record.ID, nil
}

// holdLaneInstallation announces a started landing launch's agent process
// at the lane installation under the landing-agent lineage and takes the
// installation's lease for it, as session start does: a dead holder's lease
// passes to it. A live holder that is another process keeps the lease, and
// that is an error: the agent could not prove it is the lane's agent.
func holdLaneInstallation(module string, record launch.Record) error {
	child := record.Child
	if child == nil {
		return errors.New("the launch recorded no agent process")
	}
	ticks, boot := child.StartTicks, child.BootID
	if ticks == 0 || boot == "" {
		ticks, boot = 0, ""
	}
	if _, err := lease.AnnounceWithPair(module, record.ID, child.Pid, child.StartedAtSec, ticks, boot,
		fmt.Sprintf("claude:%d", child.Pid), "claude", launch.LandingOwnerLineage); err != nil {
		return err
	}
	holder, err := lease.CurrentHolder(module)
	if err != nil {
		return err
	}
	if holder.Pid != child.Pid || holder.OwnerLineage != launch.LandingOwnerLineage {
		return fmt.Errorf("the lane installation is held by the live session %s (pid %d)", holder.OwnerLineage, holder.Pid)
	}
	return nil
}

// cancel stops a landing launch that started as the lane was paused.
func (a landingAgent) cancel(id string) error {
	_, err := a.manager().Cancel(id)
	return err
}

// reapOutage feeds the lane installation's outage mark from an ended
// landing launch that the provider limited or overloaded, as a seat's reap
// does; the mark then holds the next start until it lapses.
func (a landingAgent) reapOutage(id string) error {
	dir, err := a.manager().Store.StateDir(id)
	if err != nil {
		return err
	}
	record, err := a.manager().Store.Read(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	class, evidence, ok := outage.ClassifyProviderResult(filepath.Join(dir, "result.json"))
	if !ok {
		class, evidence, ok = outage.ClassifyLogs(filepath.Join(dir, "stderr.log"), filepath.Join(dir, "exec.log"))
	}
	if !ok {
		return nil
	}
	_, err = outage.Record(batch.ModuleRoot(record.WorkingDirectory), class, evidence, launch.LandingOwnerLineage, a.now())
	return err
}

// newLandingAgentKeeper is the keeper's landing-agent step for the steward
// of self (simple lane §1, rail 2): it starts the agent when the lane's
// queue is not empty, the lane is not paused and none is alive. Its one
// hold is a standing provider outage at the lane installation, and each
// ended launch is reaped for its outage.
func newLandingAgentKeeper(self, home string, agent landingAgent) lane.AgentKeeper {
	return lane.AgentKeeper{Home: home, Now: agent.now, Self: self,
		Holds: []func(string) (string, error){
			func(root string) (string, error) {
				if mark, standing := outage.StandingAt(batch.ModuleRoot(root), agent.now()); standing {
					return fmt.Sprintf("the model provider is limited or overloaded (%s since %s); it starts when the provider recovers", mark.LastClass, lane.LocalText(mark.Since)), nil
				}
				return "", nil
			},
		},
		Running: agent.running, Start: agent.start, Reap: []func(string) error{agent.reapOutage}, Cancel: agent.cancel}
}

// probe reads whether a landing agent runs on this computer, its process
// and since when: what the lane's view shows as the lane's runner.
func (a landingAgent) probe(string) (lane.OwnerProbe, error) {
	records, err := a.manager().List()
	if err != nil {
		return lane.OwnerProbe{}, err
	}
	for _, record := range records {
		if record.Kind != launch.LandingKind || record.State.Terminal() {
			continue
		}
		probe := lane.OwnerProbe{Alive: true}
		if record.Child != nil {
			probe.PID = record.Child.Pid
		}
		if at, err := time.Parse(time.RFC3339, record.StartedAt); err == nil {
			probe.Since = at
		}
		return probe, nil
	}
	return lane.OwnerProbe{}, nil
}

// landingAgentStep is the landing agent's keeper step for the steward of
// repo: nil without a lane home.
func landingAgentStep(repo string) func() string {
	home, err := batchowner.LandingLaneHome()
	if err != nil {
		return nil
	}
	return newLandingAgentKeeper(repo, home, newLandingAgent()).Step
}
