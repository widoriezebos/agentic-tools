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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
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
	// proofEffects are the lane Git and process boundaries; zero uses the host.
	proofEffects plain.ProveSeams
	home         string
	manager      func() *launch.Manager
	// settings are the launch settings of the installation at a state root.
	settings func(stateRoot string) (launch.Settings, error)
	now      func() time.Time
	nonce    func() (string, error)
	// machine names this computer as the lane installation's goal ledger
	// knows it: the machine a lane question records.
	machine func(stateRoot string) (string, error)
}

func newLandingAgent() landingAgent {
	return landingAgent{manager: func() *launch.Manager { return launchManager() }, settings: installationSettings, now: time.Now, nonce: landingNonce, machine: goal.ResolveMachine}
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
Your recorded batch identity is %s. Read status before each step; continue through a pause only when status admits this batch.
For a question about the checkout being off main, include "The checkout is not on main." in its facts. For a question about main's commit, include "Main is at commit <full SHA>." in its facts, replacing <full SHA> with the commit. The keeper withdraws your question when that premise is gone.
Read its recorded batch and merge only those goal/commit pairs, in their recorded order. New waiting lines belong to the next selection. A prepared batch with a person selector needs its recorded person selection before it is admitted.
`, root, strings.Join(wake.Reasons, ", "), wake.BatchID)
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
	return record.ID, nil
}

// cancel stops a landing launch whose admission changed while it started.
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
	if !ok && record.State != launch.Completed && (record.ExitCode == nil || *record.ExitCode != 0) {
		return nil
	}
	seen, err := time.Parse(time.RFC3339Nano, record.FinishedAt)
	if err != nil {
		return err
	}
	_, err = outage.Observe(a.home, record.Adapter, launchModel(record), class, evidence, record.ID, seen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "provider evidence was not recorded:", err)
	}
	return nil
}

// questionHold holds the start while a question about the lane asked on
// this computer is open: the agent that asked ended its turn to wait for
// the person, and the next one reads the answer with question show. A lane
// question from another computer holds nothing here.
func (a landingAgent) questionHold(module string) (string, error) {
	machineFn := a.machine
	if machineFn == nil {
		machineFn = goal.ResolveMachine
	}
	open, _ := channel.WalkQuestions(module)
	var lane []channel.Question
	for _, q := range open {
		if satisfied, err := plain.PolicyQuestionSatisfied(module, q); err != nil {
			return "", err
		} else if satisfied {
			continue
		}
		if q.Goal == "" && q.About == "lane" && (q.State == "open" || q.State != "closed" && channel.LaneStopCommand(q) != "") {
			lane = append(lane, q)
		}
	}
	if len(lane) == 0 {
		return "", nil
	}
	machine, err := machineFn(module)
	if err != nil {
		return "", fmt.Errorf("a question about the lane is open and this computer's name cannot be read: %w", err)
	}
	for _, q := range lane {
		if q.Machine == machine {
			if withdrawn, err := plain.WithdrawStaleLaneQuestion(module, q, a.proofEffects); err != nil {
				return "", err
			} else if withdrawn {
				continue
			}
			if command := channel.LaneStopCommand(q); command != "" {
				return "the lane waits for a person's act\nrun: " + command, nil
			}
			return plain.LaneQuestionHeadline(q), nil
		}
	}
	return "", nil
}

// newLandingAgentKeeper is the keeper's landing-agent step for the steward
// of self (simple lane §1, rail 2): it starts the agent when the plain
// lane's queue holds work (queued, proof-finished), its current fences
// admit that selection and none is alive. Its holds are a proof that runs, a standing provider
// outage at the lane installation and an open question the landing agent
// asked about the lane, and each ended launch is reaped for its outage.
func newLandingAgentKeeper(self, home string, agent landingAgent) lane.AgentKeeper {
	agent.home = home
	machine := agent.machine
	if machine == nil {
		machine = goal.ResolveMachine
	}
	seams := agent.proofEffects
	seams.Now = agent.now
	seams.TimerHeld = func() bool { _, paused := lane.ReadPause(home); return paused || helm.Active(self).Active }
	seams.Lane = func() (lane.Record, error) {
		record, present, err := lane.Read(home)
		if err == nil && !present {
			err = errors.New("the landing lane is no longer registered")
		}
		return record, err
	}
	seams.Policy = func(key string) (plain.PolicyValue, error) {
		record, err := seams.Lane()
		if err != nil {
			return plain.PolicyValue{}, err
		}
		inv, err := landingDesignInvocation(record.Install, io.Discard)
		if err != nil {
			return plain.PolicyValue{}, err
		}
		return inv.laneBatchSeams(home, record, seams).Policy(key)
	}
	return lane.AgentKeeper{Home: home, Now: agent.now, Self: self, Sources: plain.KeeperWake(home, seams),
		AdmitWake: func(root string, wake lane.Wake) (lane.Wake, error) {
			record, err := seams.Lane()
			if err != nil {
				return wake, err
			}
			return plain.DrainWake(record.Install, record.Root, wake, seams)
		},
		Helmed:       func(root string) bool { return helm.Active(root).Active },
		Continuation: func(record lane.Record) string { return plain.PersonBatchContinuation(record.Install, record, home) },
		Prepare: func(record lane.Record) error {
			selection := seams
			selection.AgentRunning = func() (bool, error) { _, running, err := agent.running(); return running, err }
			_, err := plain.SelectBatch(record.Install, record.Root, record, selection)
			return err
		},
		Observe: func(record lane.Record) error {
			effects := agent.proofEffects
			effects.Now = agent.now
			_, drainErr := plain.AdvanceDrain(record.Install, record.Root, effects)
			questionErr := plain.SyncPolicyQuestion(record.Install, machine, agent.now(), seams)
			if questionErr != nil {
				questionErr = fmt.Errorf("synchronize the lane's stop question in %s: %w; repair that question source and observe the lane again", plain.Dir(record.Install), questionErr)
			}
			if drainErr != nil || questionErr != nil {
				return &lane.ObservationError{Progress: drainErr, StopQuestion: questionErr}
			}
			return nil
		},
		PersonSelection: func(record lane.Record) bool {
			_, err := plain.RecordedPersonBatch(record.Install, record, "")
			return err == nil
		},
		ProviderHold: func(root string) (string, error) {
			settings, err := agent.settings(batch.ModuleRoot(root))
			if err != nil {
				return "", err
			}
			providers, err := outage.ReadProviders(home)
			mark, standing := providers.Standing(settings.LandingRuntime, agent.now())
			if err != nil {
				return "", err
			}
			if standing {
				return fmt.Sprintf("the model provider is limited or overloaded (%s since %s); it starts when the provider recovers", mark.LastClass, lane.LocalText(mark.Since)), nil
			}
			return "", nil
		},
		Holds: []func(string) (string, error){
			func(string) (string, error) {
				record, _, err := lane.Read(home)
				if err != nil {
					return "", err
				}
				return plain.ProofHold(record.Install, agent.proofEffects)
			},
			func(string) (string, error) {
				record, _, err := lane.Read(home)
				if err != nil {
					return "", err
				}
				return plain.KeeperDrainHold(record.Install)
			},

			func(string) (string, error) {
				record, _, err := lane.Read(home)
				if err != nil {
					return "", err
				}
				return agent.questionHold(record.Install)
			},
		},
		Running: agent.running, Start: agent.start, Reap: []func(string) error{agent.reapOutage}, Cancel: agent.cancel,
		Fingerprint: plain.KeeperFingerprint,
		BarrenStop: func(record lane.Record, state lane.AgentState) error {
			return plain.RecordBarrenStop(record.Install, state, lane.AgentStatePath(home), agent.now(), plain.ProveSeams{Lane: func() (lane.Record, error) { return record, nil }})
		},
		Waiting: func(install, checkout string) (int, error) {
			waiting, err := plain.Pending(install, checkout, seams)
			return len(waiting), err
		}}
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
func landingAgentStep(repo string) func() lane.AgentRun {
	home, err := batchowner.LandingLaneHome()
	if err != nil {
		return nil
	}
	return newLandingAgentKeeper(repo, home, newLandingAgent()).Run
}

func launchModel(record launch.Record) string {
	var model string
	_ = json.Unmarshal(record.AdapterData["model"], &model)
	return model
}
