package plain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// RawFacts retains the records belonging to one status observation.
type RawFacts struct {
	EngineIncidents         []state.EngineIncident
	Err                     error
	Now                     time.Time
	Previous                *state.Record
	Stops                   []Stop
	Pause                   lane.Pause
	Resume                  lane.Pause
	Agent                   lane.AgentState
	AgentLive               *bool
	Batch                   *Batch
	Queue                   []Line
	Ordinals                []int
	Results                 []Result
	Running                 Running
	Fix                     Fix
	Main, Engine, Abandoned string
	Incidents               []goal.TrunkRedEntry
	QuestionHealth          []QuestionHealth
	Anchors                 Anchors
	AllQuestions            []channel.Question
	Questions               []state.Question
	QuestionInputs          map[string]state.QuestionInput
}

// ShadowOf projects only the supplied observation; it never reads or acts.
func ShadowOf(observation Status, raw RawFacts) state.Record {
	since := func(stamp, source string) state.Since {
		if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			return state.Since{At: at.UTC()}
		}
		return state.Since{At: raw.Now.UTC(), Observed: true, Source: source}
	}
	next := state.Record{}
	if stop := observation.Stop; stop != nil {
		next.Stop = stopDescription(*stop)
	}
	next.Lane.Word = "running"
	if observation.Paused {
		next.Lane.Word = "paused"
	}
	if observation.Root == nil || raw.Pause.Unreadable() || observation.Stop != nil {
		next.Lane.Word = "stopped"
	}
	next.Lane.By, next.Lane.Reason = raw.Pause.By, raw.Pause.Reason
	next.Lane.Since = since(raw.Pause.At, "landing-lane.json")
	if next.Lane.Word == "running" {
		next.Lane.By, next.Lane.Reason = raw.Resume.By, ""
		next.Lane.Since = since(raw.Resume.At, "landing-lane-resumed.json")
		if raw.Resume.At == "" {
			next.Lane.Since = since("", "landing-lane.json")
		}
	}
	agentAlive := observation.AgentAlive
	if raw.AgentLive != nil {
		agentAlive = *raw.AgentLive
	}
	next.Agent.Session = raw.Agent.Launch
	next.Agent.Word = "ended"
	if agentAlive {
		next.Agent.Word = "idle"
	}
	if progress := observation.AgentProgress; progress != nil {
		source := progress.Source
		if source == "" {
			source = "landing-agent-keeper.json"
		}
		next.Agent.Since = since(progress.Since, source)
		if step := progress.Step; step != nil && !progress.ProcessGone {
			next.Agent.Step = state.Step{Verb: step.Verb, Args: append([]string(nil), step.Args...), StartedAt: step.StartedAt}
			if agentAlive {
				next.Agent.Word = "stepping"
			}
			next.Agent.Since = since(step.StartedAt, "agent-step.json")
		}
		if progress.Step != nil && progress.ProcessGone && agentAlive {
			next.Agent.Word = "step-process-gone"
			next.Agent.Since = since("", "agent-step.json")
		}
	} else {
		stamp := raw.Agent.StartedAt
		if next.Agent.Word == "ended" {
			stamp = raw.Agent.ReapedAt
		}
		next.Agent.Since = since(stamp, "landing-agent-keeper.json")
	}
	if !agentAlive {
		next.Agent.Since = since(raw.Agent.ReapedAt, "landing-agent-keeper.json")
	}
	if batch := raw.Batch; batch != nil && !(batch.State == BatchClosed && len(batch.Members) == 0) {
		next.Batch.ID, next.Batch.Base = batch.ID, batch.Base
		next.Batch.SealedAt, _ = time.Parse(time.RFC3339Nano, batch.SealedAt)
		next.Batch.MainMove = observation.MainMove.Kind
		next.Batch.Execution.Word = batch.State
		preparedAt := batch.CreatedAt
		latest := func(stamp string) {
			at, err := time.Parse(time.RFC3339Nano, stamp)
			previous, _ := time.Parse(time.RFC3339Nano, preparedAt)
			if err == nil && at.After(previous) {
				preparedAt = stamp
			}
		}
		for _, restart := range batch.Restarts {
			latest(restart.At)
		}
		for _, action := range batch.Actions {
			if !action.Pending && (action.Kind == "reopen" || action.Kind == "restart") {
				latest(action.At)
			}
		}
		stamp := ""
		if batch.State == BatchPrepared {
			stamp = preparedAt
		}
		if batch.State == BatchRunning {
			stamp = batch.RunningAt
		}
		if batch.State == BatchClosed {
			stamp = batch.ClosedAt
		}
		next.Batch.Execution.Since = since(stamp, "batch.json")
		next.Batch.Admission.Word = "sealed"
		stamp = batch.SealedAt
		if batch.Open {
			next.Batch.Admission.Word, stamp = "open", preparedAt
		}
		next.Batch.Admission.Since = since(stamp, "batch.json")
		for _, member := range batch.Members {
			next.Batch.Members = append(next.Batch.Members, state.Member{Goal: member.Goal, SHA: member.SHA})
		}
	}
	for i, entry := range observation.Queue {
		stamp := entry.LandedAt
		if entry.State == StateReturned {
			stamp = entry.ReturnedAt
		}
		at, _ := time.Parse(time.RFC3339Nano, stamp)
		if lifecycleExpired("submission", entry.State, at, raw.Now) {
			continue
		}
		one := state.HandIn{Goal: entry.Goal, SHA: entry.SHA, Ordinal: raw.Ordinals[i], Reason: entry.Reason}
		one.Word = entry.State
		one.At, _ = time.Parse(time.RFC3339Nano, entry.At)
		stamp, source := entry.At, "queue.jsonl"
		switch entry.State {
		case StateReturned:
			stamp = entry.ReturnedAt
		case StateLanded:
			stamp, source = entry.LandedAt, "pushes.jsonl"
			if stamp == "" {
				source = "queue.jsonl"
			}
		case StateSuperseded:
			stamp = ""
		}
		one.Since = since(stamp, source)
		next.HandIns = append(next.HandIns, one)
	}
	if fix := raw.Fix; fix.State != "" {
		next.Resolution = resolutionDescription(&fix)
		stamp := fix.At
		if fix.State == "reviewing" {
			stamp = fix.ReadStartedAt
		}
		if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			next.Resolution.Since = state.Since{At: at.UTC()}
		}
	}
	if proof := observation.RunningProof; proof != nil {
		next.Proof.Attempt, next.Proof.Word = proof.Attempt, proof.State
		next.Proof.Trunk = raw.Running.Trunk
		for _, member := range raw.Running.BatchMembers {
			next.Proof.Members = append(next.Proof.Members, state.Member{Goal: member.Goal, SHA: member.SHA})
		}
		if len(next.Proof.Members) == 0 && raw.Batch != nil && raw.Running.BatchID == raw.Batch.ID {
			next.Proof.Members = append([]state.Member(nil), next.Batch.Members...)
		}
		next.Proof.Scope = "full"
		if raw.Running.Gate {
			next.Proof.Scope = "gate"
		} else if raw.Running.Impact {
			next.Proof.Scope = "impact"
		}
		if raw.Running.Admission != nil {
			next.Proof.Scope = raw.Running.Admission.Scope
		}
		startedAt := ""
		if proof.State == "running" {
			startedAt = proof.Since
		}
		if proof.State != "running" {
			next.Proof.Since = since("", "running.json")
		} else if at, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
			next.Proof.Since = state.Since{At: at.UTC()}
		}
	} else if proof := observation.LastProof; proof != nil {
		next.Proof.Attempt, next.Proof.Word, next.Proof.Scope, next.Proof.LoadAtStart = proof.Attempt, proof.Result, proof.Scope, proof.LoadStart
		next.Proof.Since = since(proof.At, "results.jsonl")
	}
	next.Main.Commit, next.Main.Word = raw.Main, "unproven"
	stamp, source := "", "refs/remotes/origin/main"
	for _, proof := range raw.Results {
		if proof.Trunk && proof.Commit == raw.Main && (proof.Result == Green || proof.Result == Red) {
			next.Main.Word, stamp, source = proof.Result, proof.At, "results.jsonl"
			next.Main.Reason = strings.Join(failingTests(proof.Failed), ", ")
			if next.Main.Reason == "" {
				next.Main.Reason = proof.Reason
			}
			next.Main.Flakes, next.Main.FlakePublished = "", false
			if proof.Result == Green {
				next.Main.Flakes, next.Main.FlakePublished = flakeNames(proof.Flakes), proof.FlakePublished
			}
		}
	}
	for _, incident := range raw.Incidents {
		if incident.Closed == nil {
			next.Main.OpenIncidents = append(next.Main.OpenIncidents, incident.ID)
		}
	}
	if push := observation.LastPush; push != nil && push.Commit == raw.Main {
		stamp, source = push.At, "pushes.jsonl"
	}
	next.Main.Since = since(stamp, source)
	if p := observation.LastProof; next.Main.Word != Red && p != nil && p.Result == Green && len(p.Flakes) > 0 && strings.Contains(p.Reason, "flake on main:") {
		next.Main.Flakes = flakeNames(p.Flakes)
	}
	next.Questions = append([]state.Question(nil), raw.Questions...)
	next.Holds.Barren, next.Holds.SelectionPending = raw.Agent.Barren, observation.BatchPolicy != nil && observation.BatchPolicy.Value == "person" && raw.Batch == nil
	holdAt := ""
	for _, stop := range raw.Stops {
		if stop.Loop == "lane-return" && stop.Subject == "lane" && stop.Attempt == raw.Agent.Barren {
			holdAt = stop.At
			break
		}
	}
	holdSource := "landing-agent-keeper.json"
	if holdAt != "" {
		holdSource = "stops.jsonl"
	}
	next.Holds.Since = since(holdAt, holdSource)
	next.Engine.Stamp = raw.Engine
	next.Engine.Incidents = raw.EngineIncidents
	return state.Carry(raw.Previous, next)
}

// RenderInputsOf projects transient values without publishing them.
func RenderInputsOf(observation Status, raw RawFacts) state.RenderInputs {
	inputs := state.RenderInputs{Now: raw.Now, Unready: observation.Owner.State == lane.OwnerUnready, Unread: observation.Owner.Unread != "", PID: observation.Owner.PID, HandIns: map[int]state.HandInInput{}, Questions: raw.QuestionInputs}
	if wake := observation.View.Wake; wake != nil {
		for _, problem := range wake.Unread {
			if strings.HasPrefix(problem, "hand-in ") {
				inputs.UnreadHandIn = strings.SplitN(problem, "\n", 2)[0]
				break
			}
		}
	}
	if proof := observation.RunningProof; proof != nil {
		inputs.UnitsDone, inputs.UnitsTotal, inputs.Minutes = proof.UnitsDone, proof.UnitsTotal, proof.Minutes
	}
	inputs.ReadMinutes, inputs.CheckMinutes = raw.Fix.ReadMinutes, raw.Fix.CheckMinutes
	if batch := raw.Batch; batch != nil {
		inputs.MainCommits = observation.MainMove.Count
		inputs.WaitingReasons = append([]string(nil), batch.Waiting...)
		if len(batch.Restarts) > 0 {
			inputs.RestartReasons = []string{batch.Restarts[len(batch.Restarts)-1].Reason}
		}
	}
	for i, entry := range observation.Queue {
		one := state.HandInInput{}
		if b := raw.Batch; b != nil && b.State != BatchClosed && entry.State == StateWaiting {
			selected, sameGoal := false, false
			for _, m := range b.Members {
				selected = selected || m.Goal == entry.Goal && m.SHA == entry.SHA
				sameGoal = sameGoal || m.Goal == entry.Goal
			}
			if !entry.Held && !selected {
				one.WaitNext = !b.Open || sameGoal
				one.WaitFix = !one.WaitNext && (observation.RunningFix != nil || raw.Running.Checkpoint && raw.Running.BatchID == b.ID)
			}
		}
		inputs.HandIns[raw.Ordinals[i]] = one
	}
	return inputs
}

func shadowEngine(install string) (string, error) {
	file, err := os.Open(filepath.Join(install, "bin", "metasystem"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	stamp, err := enginebuild.ReadStamp(file)
	return stamp, errors.Join(err, file.Close())
}

// RecordResumeFinding retains advisory failure separately from operational records.
func RecordResumeFinding(install string, finding string) error {
	return withLock(install, func() error {
		return writeAgentRecord(install, "resume-record-finding.json", finding)
	})
}

// RefreshShadow publishes only complete, coherent observations.
func RefreshShadow(home string, now time.Time, by string, seams ProveSeams) (changed []string, refreshErr error) {
	record, exists, err := lane.Read(home)
	if err != nil || !exists {
		return nil, err
	}
	info, err := os.Stat(record.Install)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("the lane installation is not a directory")
	}
	defer func() {
		if refreshErr != nil {
			_ = writeAgentRecord(record.Install, "shadow-refresh.json", refreshErr.Error())
		} else {
			_ = os.Remove(filepath.Join(Dir(record.Install), "shadow-refresh.json"))
		}
	}()
	seams.Now = func() time.Time { return now }
	for try := 0; try < 3; try++ {
		observation, err := captureObservation(home, &record, seams, nil)
		var changedRegistration *Reregistered
		if errors.As(err, &changedRegistration) {
			var present bool
			record, present, err = lane.Read(home)
			if err != nil || !present {
				return nil, err
			}
			if _, err := os.Stat(record.Install); err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !observation.Coherent {
			continue
		}
		if len(observation.Status.Problems) > 0 {
			problems := observation.Status.Problems
			return nil, errors.Join(errors.New(strings.Join(problems, "; ")), state.NoteObservation(Dir(record.Install), now, problems))
		}
		if seams.BeforeShadowPublish != nil {
			seams.BeforeShadowPublish()
		}
		changed, err = state.Replace(Dir(record.Install), now, state.Carry(observation.Captured.Shadow, ShadowOf(observation.Status, observation.Status.Raw)), by, observation.Captured.Version, state.Fingerprint{
			Value:   observation.Captured.Fingerprint,
			Current: func() string { return observationFingerprint(home, seams) },
		})
		var stale *state.Stale
		if errors.As(err, &stale) {
			continue
		}
		return changed, err
	}
	return nil, state.NoteUnstable(Dir(record.Install), now)
}
