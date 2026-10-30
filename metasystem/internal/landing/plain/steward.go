package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type Intervention struct {
	Person                                    *ActProvenance `json:",omitempty"`
	At, Row, Subject, What, Why, How, Outcome string
	Shadow                                    string
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

// RecordIntervention appends under the same installation lock as lane acts.
func RecordIntervention(install string, entry Intervention) error {
	return withLock(install, func() error { return appendLine(interventionsPath(install), entry) })
}

// StewardStep serializes stewards; verbs and appends acquire the install lock separately.
func StewardStep(home string, now time.Time, seams ProveSeams) (StewardRun, error) {
	for try := 0; try < 3; try++ {
		run, err := stewardStep(home, now, seams)
		var changed *Reregistered
		if !errors.As(err, &changed) {
			return run, err
		}
	}
	return StewardRun{}, errors.New("the landing lane registration kept changing during the steward read")
}
func stewardStep(home string, now time.Time, seams ProveSeams) (run StewardRun, err error) {
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
	seams.AgentProbe = effects.Owner
	build := func(readonly ProveSeams) lane.View {
		return lane.BuildView(lane.ViewSources{Home: home, Now: now, Owner: readonly.AgentProbe, Fingerprint: KeeperFingerprint, Wake: KeeperWake(home, readonly)})
	}
	status := ObserveStatus(home, record, seams, build)
	view := status.View
	if status.ObservationError != nil {
		return run, status.ObservationError
	}
	if len(status.Problems) > 0 || status.Health == nil {
		return run, fmt.Errorf("the lane steward cannot read status: %s", strings.Join(status.Problems, "; "))
	}
	if err = closeGoneLoopStops(record.Install, status, now); err != nil {
		return run, err
	}
	status = ObserveStatus(home, record, seams, build)
	if status.ObservationError != nil {
		return run, status.ObservationError
	}
	if len(status.Problems) > 0 || status.Health == nil {
		return run, fmt.Errorf("the lane steward cannot read status: %s", strings.Join(status.Problems, "; "))
	}
	history, err := readLines[Intervention](interventionsPath(record.Install))
	if err != nil {
		return run, err
	}
	lines := []string{}
	proofRunning := status.RunningProof != nil && status.RunningProof.State == "running"
	person := landingstate.StopOwner(landingstate.StopFacts{Paused: status.Paused, StoppedBy: view.Owner.StoppedBy != nil, PersonProof: proofRunning && status.RunningProof.Person != nil}).Word == landingstate.Person
	young := status.AgentProgress != nil && status.AgentProgress.Step != nil && status.AgentProgress.Minutes < 2
	main, err := checkoutGit(record.Root, seams).main()
	if err != nil {
		return run, err
	}
	hold, contentionHeld, holdErr := CurrentHold(record.Install, seams)
	if holdErr != nil {
		return run, holdErr
	}
	rows := status.Health.Rows()
	for _, row := range rows {
		act := row.Row == "cannot-heal" || row.Row == "own-red-held" || row.Row == "person-act-no-effect" || row.Row == "stale-question" || row.Row == "unproven-incidents" || row.Row == "silent-agent" || row.Row == "suspected-contention" || stewardFlakeReady(status.Health, row)
		reason := "report only"
		if act {
			reason = ""
		}
		if act && (person || young || proofRunning && row.Row != "silent-agent" && row.Row != "cannot-heal") {
			reason = "bounded: a person's stop or proof, an agent step, or a proof is active"
		}
		if row.Row == "suspected-contention" && status.TrunkProof != nil && status.TrunkProof.Commit == main {
			reason = "bounded: current main already proved"
		}
		if contentionHeld && (row.Row == "stale-question" || row.Row == "unproven-incidents" || row.Row == "suspected-contention") {
			reason = landingstate.Phrase("bounded-the-proof-start-is-held-beside-builds") + hold.Until.Local().Format("15:04 MST")
		}
		var attempts []Intervention
		var defect *Intervention
		for i := range history {
			p := &history[i]
			if p.Row == row.Row {
				attempts, defect = append(attempts, *p), nil
			}
			if p.Row == "steward-act-failed" && p.Subject == row.Row {
				defect = p
			}
		}
		retry := false
		if act && reason == "" && len(attempts) >= 2 {
			last := attempts[len(attempts)-2:]
			if strings.HasPrefix(last[0].Outcome, "failed: ") && last[0].Outcome == last[1].Outcome {
				if defect != nil {
					q, readErr := channel.ReadQuestion(record.Install, strings.TrimPrefix(defect.Outcome, "asked "))
					if readErr != nil {
						return run, errors.Join(err, readErr)
					}
					retry = q.State != "open"
				} else {
					tried := []string{}
					for _, p := range last {
						tried = append(tried, p.Subject+": "+p.How+" ("+p.What+") → "+p.Outcome)
					}
					outcome, askErr := stewardAsk(record, seams, now, "steward-act-failed", row.Row, "", landingstate.Phrase("the-landing-lane-s-steward-stopped-acting-on")+row.Row+": its last two acts failed the same way", "the steward does not act on "+row.Row+" again until you act or the outcome changes; this question is withdrawn when "+row.Row+" no longer fires", strings.Join(tried, "; "))
					if askErr != nil {
						return run, errors.Join(err, askErr)
					}
					entry := Intervention{At: now.UTC().Format(time.RFC3339Nano), Row: "steward-act-failed", Subject: row.Row, What: last[1].Outcome, Why: "D11: two acts of " + row.Row + " failed the same way", How: "stop acting on the row; one question", Outcome: outcome}
					if appendErr := RecordIntervention(record.Install, entry); appendErr != nil {
						return run, errors.Join(err, appendErr)
					}
					history, run.Interventions = append(history, entry), append(run.Interventions, entry)
				}
				if !retry {
					reason = "bounded: its last two acts failed the same way"
				}
			}
		}
		for _, previous := range history {
			at, parseErr := time.Parse(time.RFC3339Nano, previous.At)
			if reason == "" && row.Row == "cannot-heal" && previous.Row == row.Row && previous.Subject == row.Subject && (parseErr != nil || now.Sub(at) < landingstate.SubmissionWindow) {
				reason = "bounded: asked once for this situation"
			}
			if act && reason == "" && !retry && !strings.HasPrefix(previous.Outcome, "held: ") && previous.Row == row.Row && previous.Subject == row.Subject && (parseErr != nil || now.Sub(at) < time.Hour) {
				reason = "bounded: already acted within 60 minutes"
			}
		}
		if reason != "" {
			lines = append(lines, row.Row+": "+row.Subject+"; "+reason)
			continue
		}
		entry := Intervention{At: now.UTC().Format(time.RFC3339Nano), Row: row.Row, Subject: row.Subject, What: row.Act, Why: row.Row + ": " + row.Subject, How: "metasystem landing prove --trunk"}
		seams.ReadOnly, seams.Person = false, nil
		var actErr error
		switch row.Row {
		case "cannot-heal":
			underlying, subject, _ := strings.Cut(row.Subject, " ")
			decision := "The landing lane cannot heal this: goal " + subject + " has waited in the lane more than 120 minutes"
			if underlying == "silent-agent" {
				decision = "The landing lane cannot heal this: the landing agent has been silent more than 80 minutes"
			}
			tried := "nothing: the row has no act of its own"
			for _, p := range history {
				if p.Row == underlying && p.Subject == subject {
					tried = p.Subject + ": " + p.How + " (" + p.What + ") → " + p.Outcome
				}
			}
			entry.How = "one question to the person"
			entry.Outcome, actErr = stewardAsk(record, seams, now, "cannot-heal", underlying, subject, decision, "the lane keeps reporting it; this question is withdrawn when the situation ends", tried)
		case "own-red-held":
			red := status.Health.OwnRed[slices.IndexFunc(status.Health.OwnRed, func(r OwnRed) bool { return r.Goal == row.Subject })]
			entry.How = "metasystem landing return " + red.Goal + " (as the lane, cause own)"
			_, _, actErr = ReturnProven(record.Install, red.Goal, "own", red.Replay, false, "lane steward", now, seams)
			entry.Outcome = "returned " + red.Goal
		case "person-act-no-effect":
			a := status.Health.PersonActs[0]
			entry.Row, entry.Subject = "person-refused", a.Verb+" "+a.At
			if slices.ContainsFunc(history, func(p Intervention) bool { return p.Row == entry.Row && p.Subject == entry.Subject }) {
				continue
			}
			entry.What = fmt.Sprintf("took no effect in %s min: no agent step, no proof start, the hold is back", measured(a.Minutes))
			entry.Why, entry.How = "person-act-no-effect", "recorded as a defect; no second wake"
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
				proofRunning = actErr == nil && entry.Outcome != stewardAlreadyProven
				if actErr == nil && !proofRunning && row.Row != "stale-question" {
					lines = append(lines, row.Row+": "+entry.Outcome)
					continue
				}
			}
		}
		if actErr != nil {
			entry.Outcome = "failed: " + actErr.Error()
		}
		if shadow := status.Shadow; shadow != nil {
			entry.Shadow = strings.Join(landingstate.Describe(*shadow, landingstate.RenderInputs{}).Steward, "; ")
		}
		appendErr := RecordIntervention(record.Install, entry)
		err = errors.Join(err, actErr, appendErr)
		run.Interventions = append(run.Interventions, entry)
		history = append(history, entry)
		lines = append(lines, row.Row+": "+entry.Outcome)
		if appendErr != nil {
			break
		}
	}
	questions, _ := channel.WalkQuestions(record.Install)
	for _, q := range questions {
		if q.State != "open" || len(q.Facts) < 2 || !strings.HasPrefix(q.Facts[1], "lane steward: ") {
			continue
		}
		if seams.Machine == nil {
			return run, errors.Join(err, errors.New("the lane computer's name cannot be read"))
		}
		machine, readErr := seams.Machine(record.Install)
		if readErr != nil {
			return run, errors.Join(err, readErr)
		}
		if q.Machine != machine {
			continue
		}
		var situation stewardQuestion
		if json.Unmarshal([]byte(strings.TrimPrefix(q.Facts[1], "lane steward: ")), &situation) != nil || situation.Row == "" {
			lines = append(lines, "steward question "+q.ID+" unreadable")
			continue
		}
		if slices.ContainsFunc(rows, func(r Fired) bool {
			return r.Row == situation.Row && (situation.Subject == "" || r.Subject == situation.Subject)
		}) {
			continue
		}
		because := "the situation ended: " + situation.Row + " no longer fires"
		withdrawErr := withLock(record.Install, func() error {
			return transitionWrite(record.Install, "questions", "withdraw", now, "", func() error {
				_, err := channel.Withdraw(record.Install, q.ID, because, nil, channel.DestinationConfig{})
				return err
			}, seams)
		})
		err = errors.Join(err, withdrawErr)
		if withdrawErr == nil {
			lines = append(lines, "steward question "+q.ID+": "+because)
		}
	}
	store := launch.Store{}
	if effects.UsageStore != nil {
		store = *effects.UsageStore
	}
	summary, reportErr := StewardReport(record.Install, record.Root, status, history, now, seams, store)
	err = errors.Join(err, reportErr, writeStewardLine(record.Install, *status.Health, now))
	if summary != "" {
		lines = append(lines, summary)
	}
	run.Line = strings.Join(lines, "; ")
	return run, err
}

type stewardQuestion struct {
	Ask     string `json:"ask"`
	Row     string `json:"row"`
	Subject string `json:"subject"`
}

func stewardAsk(record lane.Record, seams ProveSeams, now time.Time, ask, row, subject, decision, ifNothing, tried string) (string, error) {
	if seams.Machine == nil {
		return "", errors.New("the lane computer's name cannot be read")
	}
	machine, err := seams.Machine(record.Install)
	if err != nil {
		return "", err
	}
	facts, err := json.Marshal(stewardQuestion{Ask: ask, Row: row, Subject: subject})
	if err != nil {
		return "", err
	}
	var q channel.Question
	err = withLock(record.Install, func() error {
		return transitionWrite(record.Install, "questions", "ask", now, "", func() error {
			q, _, err = channel.AskOrFind(channel.AskRequest{RepoRoot: record.Install, About: "lane", Kind: "other", Machine: machine, Lineage: lane.AgentLineage, Now: now, Plain: &channel.Plain{Decision: decision, IfNothing: ifNothing}, Facts: []string{decision, "lane steward: " + string(facts), "tried: " + tried}})
			return err
		}, seams)
	})
	return "asked " + q.ID, err
}

func stewardEffect(effect func(lane.Record) (string, error), record lane.Record) (string, error) {
	if effect == nil {
		return "", errors.New("the lane verb's effect is unavailable")
	}
	return effect(record)
}

const stewardAlreadyProven = "main already has applicable proof"

func stewardTrunk(install, checkout string, seams ProveSeams) (string, error) {
	seams.Trunk, seams.Gate, seams.Impact = true, false, false
	if err := seams.fetchMain(checkout); err != nil {
		return "", err
	}
	commit, tree, err := seams.subject(checkout)
	if err != nil {
		return "", err
	}
	if _, ok := applicableMainProof(install, checkout, commit, tree, seams, true); ok {
		return stewardAlreadyProven, nil
	}
	running, already, err := Start(install, checkout, seams)
	var refusal *Refusal
	if errors.As(err, &refusal) && refusal.Code == "LANE_PROOF_CONTENTION" {
		return "held: " + refusal.Reason, nil
	}
	if err == nil && already && running.Reused != nil {
		return stewardAlreadyProven, nil
	}
	return landingstate.Phrase("proving-main-2") + running.Attempt, err
}
