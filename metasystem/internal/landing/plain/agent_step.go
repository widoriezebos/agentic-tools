package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// AgentStep records the command process, including its identity across PID reuse.
type AgentStep struct {
	Verb      string   `json:"verb"`
	Args      []string `json:"args"`
	StartedAt string   `json:"started_at"`
	PID       int64    `json:"pid"`
	Process   string   `json:"process,omitempty"`
}

type AgentProgress struct {
	Source      string     `json:"-"`
	LastStep    *AgentStep `json:"last_step,omitempty"`
	Step        *AgentStep `json:"step,omitempty"`
	Since       string     `json:"since,omitempty"`
	Minutes     int        `json:"minutes"`
	ProcessGone bool       `json:"process_gone,omitempty"`
	Line        string     `json:"line"`
	Finding     string     `json:"finding,omitempty"`
}

type agentIdle struct {
	EndedAt string     `json:"ended_at"`
	Step    *AgentStep `json:"step,omitempty"`
}

// BeginAgentStep retains switches only: free text, paths and option values never enter status.
func BeginAgentStep(install, verb string, args []string, now time.Time, effects ...ProveSeams) (func(time.Time) error, error) {
	step := AgentStep{Verb: verb, Args: []string{}, StartedAt: now.UTC().Format(time.RFC3339), PID: int64(os.Getpid())}
	for _, arg := range args {
		switch arg {
		case "--gate", "--trunk", "--classify", "--wait", "--as-seat":
			step.Args = append(step.Args, arg)
		}
	}
	exact, live, err := identity.KernelProber{}.Probe(step.PID)
	if err != nil || live != identity.Alive {
		return func(time.Time) error { return nil }, fmt.Errorf("identify landing step process: %v", err)
	}
	step.Process, err = identity.EncodeRef(exact.Ref())
	if err != nil {
		return func(time.Time) error { return nil }, err
	}
	finish := func(ended time.Time) error {
		return withLock(install, func() error {
			var current AgentStep
			if err := readAgentRecord(install, "agent-step.json", &current); err != nil {
				return err
			}
			// A later command owns its own completion; this exit must not erase it.
			if current.PID != step.PID || current.StartedAt != step.StartedAt {
				return nil
			}
			return transitionWrite(install, "agent", verb, ended, "", func() error {
				if err := writeAgentRecord(install, "agent-idle.json", agentIdle{EndedAt: ended.UTC().Format(time.RFC3339), Step: &step}); err != nil {
					return err
				}
				return os.Remove(filepath.Join(Dir(install), "agent-step.json"))
			}, effects...)
		})
	}
	if err := withLock(install, func() error {
		return transitionWrite(install, "agent", verb, now, "", func() error { return writeAgentRecord(install, "agent-step.json", step) }, effects...)
	}); err != nil {
		return finish, err
	}
	return finish, nil
}

func writeAgentRecord(install, name string, record any) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(filepath.Join(Dir(install), name), append(data, '\n'), 0600, "")
	return err
}

func readAgentRecord(install, name string, record any) error {
	data, err := os.ReadFile(filepath.Join(Dir(install), name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, record)
}

func agentProgress(install string, owner lane.OwnerView, seams ProveSeams, queue ...Entry) (*AgentProgress, error) {
	progress := &AgentProgress{}
	if owner.Since != nil {
		progress.Since = *owner.Since
		progress.Source = "landing-agent-keeper.json"
	}
	var step AgentStep
	err := readAgentRecord(install, "agent-step.json", &step)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		at, parseErr := time.Parse(time.RFC3339, step.StartedAt)
		if parseErr != nil || step.Verb == "" || step.PID <= 0 {
			return nil, errors.New("the agent step has no valid verb, start time or process")
		}
		progress.Step, progress.Since = &step, step.StartedAt
		progress.Source = "agent-step.json"
		progress.LastStep = &step
		progress.Minutes = int(seams.now().Sub(at) / time.Minute)
		running := Running{Pid: step.PID, Process: step.Process}
		live := seams.alive(running)
		// Older records carry only the required PID.
		if step.Process == "" && seams.Alive == nil {
			_, state, _ := identity.KernelProber{}.Probe(step.PID)
			live = state != identity.Dead
		}
		progress.ProcessGone = !live
		words := strings.Join(append([]string{step.Verb}, step.Args...), " ")
		progress.Line = fmt.Sprintf("agent: %s since %s (%d min)", words, at.In(time.Local).Format("15:04"), progress.Minutes)
		if !live {
			progress.Line += ", process gone"
		}
		return progress, nil
	}
	var idle agentIdle
	err = readAgentRecord(install, "agent-idle.json", &idle)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && idle.EndedAt > progress.Since {
		progress.Since = idle.EndedAt
		progress.Source = "agent-idle.json"
	}
	progress.LastStep = idle.Step
	// A person's return is recorded activity even though it runs outside the agent session.
	for _, entry := range queue {
		if entry.ReturnedAt > progress.Since {
			progress.Since = entry.ReturnedAt
			progress.Source = "queue.jsonl"
		}
	}
	if progress.Since == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339, progress.Since)
	if err != nil {
		return nil, err
	}
	progress.Minutes = int(seams.now().Sub(at) / time.Minute)
	progress.Line = "agent: idle since " + at.In(time.Local).Format("15:04")
	if owner.State == lane.OwnerRunning {
		progress.Line = fmt.Sprintf(landingstate.Phrase("agent-running-no-step-for-d-min"), progress.Minutes)
	}
	return progress, nil
}

// Empty-batch idle is a finding: landing stop pauses admission but does not end a session.
func (status *Status) agentIdleFinding(install string, seams ProveSeams) error {
	p := status.AgentProgress
	if p == nil || p.Step != nil || !status.AgentAlive || status.RunningProof != nil && status.RunningProof.State == "running" || status.RunningFix != nil || status.RunningRegeneration != nil || len(status.Problems) != 0 {
		return nil
	}
	for _, entry := range status.Queue {
		if entry.State == StateWaiting || entry.State == "running" {
			return nil
		}
	}
	raw, _, err := config.Get(config.GetParams{Key: "landing.agent-idle-minutes", ConfPath: filepath.Join(install, "metasystem.conf")})
	if errors.Is(err, os.ErrNotExist) {
		raw, _ = config.CompiledDefault("landing.agent-idle-minutes")
		err = nil
	}
	if err != nil {
		return err
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		return fmt.Errorf("landing.agent-idle-minutes must be a nonnegative minute count: %q", raw)
	}
	if p.Minutes > limit {
		p.Finding = fmt.Sprintf("agent turn is idle %d min after the batch emptied; landing stop does not end the session", p.Minutes)
	}
	return nil
}
