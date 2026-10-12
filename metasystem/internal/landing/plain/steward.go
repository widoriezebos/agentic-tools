package plain

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type Intervention struct {
	At, Row, Subject, What, Why, How, Outcome string
}

type StewardEffects struct {
	FlakeFix     func(goal.FlakeFact, goal.FlakeFix, time.Time) (goal.FlakeFact, error)
	UsageStore   *launch.Store
	Owner        func(string) (lane.OwnerProbe, error)
	Withdraw     func(string, channel.Question, ProveSeams) (bool, error)
	Trunk        func(string, string, ProveSeams) (string, error)
	Settle, Wake func(lane.Record) (string, error)
}

type StewardRun struct {
	Interventions []Intervention
	Line          string
}

func interventionsPath(install string) string {
	return filepath.Join(Dir(install), "interventions.jsonl")
}

// StewardStep serializes stewards; verbs and appends acquire the install lock separately.
func StewardStep(home string, now time.Time, seams ProveSeams) (run StewardRun, err error) {
	record, present, err := lane.Read(home)
	if err != nil || !present {
		return run, err
	}
	if err = withLock(record.Install, func() error { return nil }); err != nil {
		return run, err
	}
	held, err := lock.File(filepath.Join(Dir(record.Install), "steward-step.lock"), 0600, lock.Exclusive)
	if err != nil {
		return run, err
	}
	defer held.Release()
	seams.Now = func() time.Time { return now }
	seams.ReadOnly = true
	effects := StewardEffects{}
	if seams.Steward != nil {
		effects = *seams.Steward
	}
	if effects.Owner == nil {
		effects.Owner = func(string) (lane.OwnerProbe, error) {
			return lane.OwnerProbe{}, errors.New("the landing agent's process cannot be observed")
		}
	}
	view := lane.BuildView(lane.ViewSources{Home: home, Now: now, Owner: effects.Owner})
	status := ReadStatus(home, record, view, seams)
	if len(status.Problems) > 0 || status.Health == nil {
		return run, fmt.Errorf("the lane steward cannot read status: %s", strings.Join(status.Problems, "; "))
	}
	history, err := readLines[Intervention](interventionsPath(record.Install))
	if err != nil {
		return run, err
	}
	lines := []string{}
	proofRunning := status.RunningProof != nil && status.RunningProof.State == "running"
	person := status.Paused || view.Owner.StoppedBy != nil || proofRunning && status.RunningProof.Person != nil
	young := status.AgentProgress != nil && status.AgentProgress.Step != nil && status.AgentProgress.Minutes < 2
	main, err := checkoutGit(record.Root, seams).main()
	if err != nil {
		return run, err
	}
	for _, row := range status.Health.Rows() {
		act := row.Row == "stale-question" || row.Row == "unproven-incidents" || row.Row == "silent-agent" || row.Row == "suspected-contention" || stewardFlakeReady(status.Health, row)
		reason := "report only"
		if act {
			reason = ""
		}
		if act && (person || young || proofRunning && row.Row != "silent-agent") {
			reason = "bounded: a person's stop or proof, an agent step, or a proof is active"
		}
		if row.Row == "suspected-contention" && status.TrunkProof != nil && status.TrunkProof.Commit == main {
			reason = "bounded: current main already proved"
		}
		for _, previous := range history {
			at, parseErr := time.Parse(time.RFC3339Nano, previous.At)
			if act && previous.Row == row.Row && previous.Subject == row.Subject && (parseErr != nil || now.Sub(at) < time.Hour) {
				reason = "bounded: already acted within 60 minutes"
			}
		}
		if act && status.Stop != nil {
			reason = "bounded: a person's stop is open"
		}
		if reason != "" {
			lines = append(lines, row.Row+": "+row.Subject+"; "+reason)
			continue
		}
		entry := Intervention{At: now.UTC().Format(time.RFC3339Nano), Row: row.Row, Subject: row.Subject, What: row.Act, Why: row.Row + ": " + row.Subject, How: "metasystem landing prove --trunk"}
		seams.ReadOnly, seams.Person = false, nil
		var actErr error
		switch row.Row {
		case "confirmed-flake":
			entry, actErr = stewardFlake(status.Health, row, now, effects)
		case "silent-agent":
			entry.How = "settle the landing agent; keeper wake"
			entry.Outcome, actErr = stewardEffect(effects.Settle, record)
			if actErr == nil {
				entry.Outcome, actErr = stewardEffect(effects.Wake, record)
			}
		default:
			if row.Row == "stale-question" {
				entry.How = "withdraw the stale lane question; " + entry.How
				var question channel.Question
				question, actErr = channel.ReadQuestion(record.Install, row.Subject)
				withdraw := effects.Withdraw
				if withdraw == nil {
					withdraw = WithdrawStaleLaneQuestion
				}
				if actErr == nil {
					var withdrawn bool
					withdrawn, actErr = withdraw(record.Install, question, seams)
					if actErr == nil && !withdrawn {
						actErr = errors.New("the question no longer has a withdrawable premise")
					}
				}
			}
			if actErr == nil {
				trunk := effects.Trunk
				if trunk == nil {
					trunk = stewardTrunk
				}
				entry.Outcome, actErr = trunk(record.Install, record.Root, seams)
				proofRunning = actErr == nil
			}
		}
		if actErr != nil {
			entry.Outcome = "failed: " + actErr.Error()
		}
		appendErr := withLock(record.Install, func() error { return appendLine(interventionsPath(record.Install), entry) })
		err = errors.Join(err, actErr, appendErr)
		run.Interventions = append(run.Interventions, entry)
		history = append(history, entry)
		lines = append(lines, row.Row+": "+entry.Outcome)
		if appendErr != nil {
			break
		}
	}
	store := launch.Store{}
	if effects.UsageStore != nil {
		store = *effects.UsageStore
	}
	summary, reportErr := StewardReport(record.Install, record.Root, status, history, now, seams, store)
	err = errors.Join(err, reportErr)
	if summary != "" {
		lines = append(lines, summary)
	}
	run.Line = strings.Join(lines, "; ")
	return run, err
}

func stewardEffect(effect func(lane.Record) (string, error), record lane.Record) (string, error) {
	if effect == nil {
		return "", errors.New("the lane verb's effect is unavailable")
	}
	return effect(record)
}

func stewardTrunk(install, checkout string, seams ProveSeams) (string, error) {
	seams.Trunk, seams.Gate, seams.Impact = true, false, false
	if err := seams.fetchMain(checkout); err != nil {
		return "", err
	}
	running, _, err := Start(install, checkout, seams)
	return "proving main: " + running.Attempt, err
}
