package main

// The landing agent's launcher (landing-lane-runtime-redesign §3, unit A-a):
// the steward of the host's registered lane checkout wakes one landing agent
// on demand, as the launch lane's landing kind on the seat's machinery. The
// kind, its lineage, its fence and the lane guard are internal/launch's; the
// wake decision is internal/landing/lane's; this file wires them to the
// launch manager, the lane installation's own settings and the outage mark.
// The agent's brief and tool gate are unit A-b's; its budgets are K-g's.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/agentgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
	// gateSettings writes the Claude settings file that holds the landing
	// agent's tool gate for launch id of store in the lane checkout root, whose
	// installation is module, and returns its path, which the launch records
	// as "settings" (the launcher refuses a landing launch without it). The
	// gate is unit A-b's (internal/landing/agentgate). nil passes none.
	gateSettings func(store launch.Store, id, root, module string) (string, error)
}

func newLandingAgent() landingAgent {
	return landingAgent{manager: func() *launch.Manager { return launchManager() }, settings: installationSettings, now: time.Now, nonce: landingNonce,
		gateSettings: landingToolGate}
}

// landingToolGate writes launch id's tool gate (unit A-b) in the
// launch's own state directory, outside the lane checkout root the agent
// may edit: a PreToolUse hook that runs the lane installation's engine in
// the lane checkout and denies when it cannot.
func landingToolGate(store launch.Store, id, root, module string) (string, error) {
	dir, err := store.StateDir(id)
	if err != nil {
		return "", err
	}
	return agentgate.WriteClaudeSettings(dir, filepath.Join(module, "bin", "metasystem"), root)
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
	if a.gateSettings != nil {
		path, err := a.gateSettings(manager.Store, id, root, module)
		if err != nil {
			return "", errors.Join(err, os.Remove(brief))
		}
		if path != "" {
			data, _ := json.Marshal(path)
			spec.AdapterData = map[string]json.RawMessage{"settings": data}
		}
	}
	record, err := manager.Start(spec)
	if err != nil {
		// A start that did not happen leaves no brief behind.
		return "", errors.Join(err, os.Remove(brief))
	}
	return record.ID, nil
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
// of self. Its holds: a standing provider outage at the lane installation,
// and the lane's batch owner still running (a landing agent never runs
// beside it). The budget, usage and custody gates join here with K-g and
// K-f.
func newLandingAgentKeeper(self, home string, agent landingAgent) lane.AgentKeeper {
	return lane.AgentKeeper{Home: home, Now: agent.now, Self: self,
		Sources: lane.WakeSources{Validation: lane.ValidationDue, Finalization: func(string) (bool, error) { return cadence.FinalizationPending(home) }},
		Holds: []func(string) (string, error){
			func(root string) (string, error) {
				if mark, standing := outage.StandingAt(batch.ModuleRoot(root), agent.now()); standing {
					return fmt.Sprintf("the model provider is limited or overloaded (%s since %s); it starts when the provider recovers", mark.LastClass, lane.LocalText(mark.Since)), nil
				}
				return "", nil
			},
			func(root string) (string, error) {
				probe, err := batchowner.LandingLaneOwnerProbe(root)
				if err != nil {
					return "", err
				}
				if probe.Alive {
					return fmt.Sprintf("the lane's batch owner (pid %d) still runs, and a landing agent never runs beside it", probe.PID), nil
				}
				// The owner's keeper asked for an owner this minute: it may
				// be starting, and the agent never starts into that race.
				state := lane.ReadKeeper(home)
				last, err := time.Parse(time.RFC3339, state.LastLaunch)
				if err == nil && agent.now().Sub(last) < lane.BaseInterval || batchowner.OwnerLaunchedWithin(root, agent.now(), batchowner.OwnerStartWindow) {
					return "the lane's batch owner is starting, and a landing agent never runs beside it", nil
				}
				return "", nil
			},
		},
		Running: agent.running, Start: agent.start, Reap: []func(string) error{agent.reapOutage}, Cancel: agent.cancel,
		Exclusive: batchowner.WithOwnerEnsureLock}
}

// liveOrStarting names a landing agent that runs, or whose start the
// keeper claimed and has not finished: no batch owner starts beside either.
func (a landingAgent) liveOrStarting(home func() (string, error)) func() (string, bool, error) {
	return func() (string, bool, error) {
		if id, running, err := a.running(); err != nil || running {
			return id, running, err
		}
		laneHome, err := home()
		if err != nil {
			// No home for the lane: whether an agent is starting is unknown,
			// and unknown holds.
			return "", false, fmt.Errorf("this computer's landing lane home can't be found, so no batch owner starts: %w", err)
		}
		if _, starting, err := lane.AgentStarting(laneHome, a.now()); err != nil || starting {
			return "a landing agent that is starting", starting, err
		}
		return "", false, nil
	}
}

// landingLaneSteps is the steward's lane step: the owner's keeper, then the
// landing agent's, each line printed when it says something.
func landingLaneSteps(steps ...func() string) func() string {
	var live []func() string
	for _, step := range steps {
		if step != nil {
			live = append(live, step)
		}
	}
	if len(live) == 0 {
		return nil
	}
	return func() string {
		var lines []string
		for _, step := range live {
			if line := step(); line != "" {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
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
