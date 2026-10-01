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
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/agentgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
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
	// The session's id is fixed before it starts, so its transcript is found
	// and its usage reconciled however it ends (K10).
	session, err := landingSessionID()
	if err != nil {
		return "", errors.Join(err, os.Remove(brief))
	}
	sessionData, _ := json.Marshal(session)
	spec := launch.StartSpec{ID: id, Kind: launch.LandingKind, WorkingDirectory: root, FenceRoot: module, Brief: brief, Tag: nonce,
		AdapterData: map[string]json.RawMessage{"sessionID": sessionData}}
	if a.gateSettings != nil {
		path, err := a.gateSettings(manager.Store, id, root, module)
		if err != nil {
			return "", errors.Join(err, os.Remove(brief))
		}
		if path != "" {
			data, _ := json.Marshal(path)
			spec.AdapterData["settings"] = data
		}
	}
	record, err := manager.Start(spec)
	if err != nil {
		// A start that did not happen leaves no brief behind.
		return "", errors.Join(err, os.Remove(brief))
	}
	return record.ID, nil
}

// landingSessionID is a fresh random (version 4) UUID for a landing
// session's --session-id.
func landingSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	text := hex.EncodeToString(raw)
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32], nil
}

// reapUsage reconciles an ended landing launch's usage into the lane's
// budget store, once by its id (K10): from the launch's own measure, else
// from the session's transcript; usage that can't be read stops the lane
// for a person. It runs as a keeper Reap, under the lane flock.
func (a landingAgent) reapUsage(home string) func(string) error {
	return func(id string) error {
		end := lane.LaunchEnd{Launch: id, State: "gone"}
		record, err := a.manager().Store.Read(id)
		switch {
		case errors.Is(err, os.ErrNotExist):
			end.Usage.Why = "its launch record is gone"
		case err != nil:
			return err
		default:
			usage, err := a.manager().Usage(id)
			if err != nil {
				return err
			}
			end.State, end.Completed = string(record.State), record.State == launch.Completed
			end.Usage = lane.Usage{Known: usage.Known, Tokens: usage.Tokens, Source: usage.Source, Why: usage.Why}
		}
		return lane.RecordLaunchEndHeld(home, end, a.now())
	}
}

// landingDailyCeiling reads launch.landing.daily.tokens of the lane
// installation at module.
func landingDailyCeiling(module string) (int64, error) {
	value, _, err := config.Get(config.GetParams{Key: launch.LandingDailyTokensKey, ConfPath: filepath.Join(module, "metasystem.conf"), LookupEnv: launchLookupEnv})
	if err != nil {
		return 0, err
	}
	ceiling, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || ceiling < 0 {
		return 0, fmt.Errorf("%s is %q, not a token count", launch.LandingDailyTokensKey, value)
	}
	return ceiling, nil
}

// laneInstallation is the installation the registered lane at home
// recorded for its checkout root.
func laneInstallation(home, root string) (string, error) {
	record, ok, err := lane.Read(home)
	if err != nil {
		return "", err
	}
	if !ok || record.Root != root {
		return "", fmt.Errorf("the landing lane at %s is no longer registered", root)
	}
	layout, err := record.Layout()
	return string(layout.Install), err
}

// The alert owners of the lane's stop-loss and of its watch.
const (
	laneStopLossOwner = "lane:stoploss"
	laneWatchOwner    = "lane:watch"
)

// openLaneAlert opens the alert of a lane hit at the lane installation,
// through the steward's one opening path (OpenAlert); deliver nil is the
// steward's own notifier.
func openLaneAlert(home string, now func() time.Time, deliver func(repoRoot, message string) error) func(root string, hit lane.Hit) error {
	return func(root string, hit lane.Hit) error {
		install, err := laneInstallation(home, root)
		if err != nil {
			return err
		}
		since, err := time.Parse(time.RFC3339, hit.At)
		if err != nil {
			since = now()
		}
		owner := laneStopLossOwner
		if hit.Kind == lane.HitWatch {
			owner = laneWatchOwner
		}
		_, _, err = steward.OpenAlert(install, steward.AlertOpening{Owner: owner, Work: hit.Work, Since: since, Message: hit.Message, Now: now(), Deliver: deliver,
			Evidence: []steward.AlertEvidence{{Record: lane.StopLossPath(home), At: hit.At, Fact: hit.Kind}}})
		return err
	}
}

// landingCustodyGrace is how long the keeper gives the landing work of a
// session it cancelled to end after SIGTERM.
const landingCustodyGrace = 5 * time.Second

// settleLandingCustody ends the landing work a cancelled session left
// running and reads custody settled (K9).
func settleLandingCustody(home string) func(root string) error {
	return func(root string) error {
		install, err := laneInstallation(home, root)
		if err != nil {
			return err
		}
		settlement, err := custody.Stop(home, laneprobe.Production(home, install, true), nil, landingCustodyGrace, time.Sleep)
		if err != nil {
			return err
		}
		if !settlement.Settled(false) {
			return errors.New(strings.Join(append(settlement.Live, settlement.Unknown...), "; "))
		}
		return nil
	}
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
// the budgets (usage not known, the daily ceiling) and custody that can't be
// read. Each ended launch is reaped for its outage and its usage; a session
// past its deadline is cancelled and its work settled; a hit's alert opens
// through the steward's OpenAlert.
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
			// The budgets (K10): usage not known, or the daily ceiling.
			func(root string) (string, error) {
				install, err := laneInstallation(home, root)
				if err != nil {
					return "", err
				}
				ceiling, err := landingDailyCeiling(install)
				if err != nil {
					return "", err
				}
				return lane.StopLossHoldHeld(home, agent.now(), ceiling)
			},
			// Custody (K9): landing work whose state can't be read holds the
			// next session until a person goes past it.
			func(root string) (string, error) {
				install, err := laneInstallation(home, root)
				if err != nil {
					return "", err
				}
				settlement, err := custody.Settle(home, laneprobe.Production(home, install, true))
				if err != nil {
					return "", err
				}
				if len(settlement.Unknown) > 0 {
					return "whether earlier landing work still runs can't be read (" + strings.Join(settlement.Unknown, "; ") + "); a person checks it with metasystem landing status --verbose", nil
				}
				return "", nil
			},
		},
		Running: agent.running, Start: agent.start, Reap: []func(string) error{agent.reapOutage, agent.reapUsage(home)}, Cancel: agent.cancel,
		Settle: settleLandingCustody(home), Alert: openLaneAlert(home, agent.now, nil)}
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

// landingLaneSteps is the steward's lane step: the landing agent's keeper,
// then the lane watch, each line printed when it says something.
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

// landingLaneWatchStep is the lane-only detector for the steward of repo
// (K10): nil without a lane home.
func landingLaneWatchStep(repo string) func() string {
	home, err := batchowner.LandingLaneHome()
	if err != nil {
		return nil
	}
	now := func() time.Time { return time.Now().UTC() }
	return lane.LaneWatch{Home: home, Now: now, Self: repo, Alert: openLaneAlert(home, now, nil)}.Step
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
