package plain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Health is the lane's recorded evidence and measured ages; reading it acts on nothing.
type Health struct {
	Report              string         `json:"report,omitempty"`
	ReportPath          string         `json:"-"`
	Interventions       []Intervention `json:"interventions,omitempty"`
	observedAt          time.Time
	StateReason         string           `json:"state_reason,omitempty"`
	AdmissibleWaiting   []string         `json:"admissible_waiting,omitempty"`
	StalePendingMerge   string           `json:"stale_pending_merge,omitempty"`
	SealedBehindMain    bool             `json:"sealed_behind_main,omitempty"`
	State               string           `json:"state"`
	Goals               []GoalHealth     `json:"goals"`
	Batch               *BatchHealth     `json:"batch,omitempty"`
	Agent               AgentHealth      `json:"agent"`
	Paused              *PauseHealth     `json:"paused,omitempty"`
	Questions           []QuestionHealth `json:"questions"`
	Incidents           []IncidentHealth `json:"incidents"`
	Load                LoadHealth       `json:"load"`
	Engine              EngineHealth     `json:"engine"`
	Proofs              ProofHealth      `json:"proofs"`
	LastPush            string           `json:"last_push,omitempty"`
	AbandonedResolution string           `json:"abandoned_resolution,omitempty"`
	LastProof           *Result          `json:"last_proof,omitempty"`
}
type GoalHealth struct {
	Goal    string   `json:"goal"`
	State   string   `json:"state"`
	Minutes *float64 `json:"minutes"`
	Overdue bool     `json:"overdue"`
	Reason  string   `json:"reason"`
}
type BatchHealth struct {
	State    string         `json:"state"`
	ID       string         `json:"id"`
	Open     bool           `json:"open"`
	SealedAt string         `json:"sealed_at,omitempty"`
	Members  []GoalSHA      `json:"members"`
	Minutes  *float64       `json:"minutes"`
	Waiting  []BatchArrival `json:"waiting"`
}
type BatchArrival struct {
	Goal      string `json:"goal"`
	Boundary  string `json:"boundary"`
	Admission string `json:"admission"`
	Reason    string `json:"reason"`
}
type AgentHealth struct {
	Alive     bool     `json:"alive"`
	Verb      string   `json:"verb,omitempty"`
	Minutes   *float64 `json:"minutes"`
	WaitingOn string   `json:"waiting_on,omitempty"`
	Silent    bool     `json:"silent"`
}
type PauseHealth struct {
	Since   string   `json:"since"`
	Reason  string   `json:"reason"`
	Minutes *float64 `json:"minutes"`
}
type QuestionHealth struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Minutes  float64 `json:"minutes"`
	Premise  string  `json:"premise"`
	WaitsFor string  `json:"waits_for"`
}
type IncidentHealth struct {
	ID          string `json:"id"`
	Commit      string `json:"commit"`
	Test        string `json:"test"`
	Class       string `json:"class"`
	Attribution *Cause `json:"attribution,omitempty"`
	Repeat      string `json:"repeat"`
	MainMoved   bool   `json:"main_moved"`
	Unproven    bool   `json:"unproven"`
}
type LoadHealth struct {
	Now       hostload.Sample `json:"now"`
	Saturated bool            `json:"saturated"`
	AtStart   *float64        `json:"at_start"`
	AtEnd     *float64        `json:"at_end"`
}
type EngineHealth struct {
	Revision string `json:"revision"`
	Stale    bool   `json:"stale"`
	Known    bool   `json:"known"`
}
type ProofHealth struct {
	Durations []float64    `json:"durations"`
	Median    *float64     `json:"median"`
	Running   *float64     `json:"running"`
	Drift     bool         `json:"drift"`
	Samples   []LoadHealth `json:"samples"`
}
type Fired struct {
	Row     string `json:"row"`
	Subject string `json:"subject"`
	Minutes string `json:"minutes"`
	Act     string `json:"act"`
}

func healthLoad(start, end *hostload.Sample) LoadHealth {
	h := LoadHealth{}
	if start != nil && start.Available {
		h.AtStart = &start.Load1m
	}
	if end != nil && end.Available {
		h.AtEnd = &end.Load1m
	}
	return h
}
func measured(value *float64) string {
	if value == nil {
		return "unmeasured"
	}
	return fmt.Sprintf("%g", *value)
}

func buildHealth(status Status, batch *Batch, pause lane.Pause, abandoned string, results []Result, queue []Line, install, checkout string, seams ProveSeams, git laneGit) (*Health, error) {
	h := &Health{State: "idle", Goals: []GoalHealth{}, Questions: []QuestionHealth{}, Incidents: []IncidentHealth{}, AbandonedResolution: abandoned, LastProof: status.LastProof}
	now := seams.now()
	h.observedAt = now
	h.ReportPath = filepath.Join(Dir(install), "steward-report.md")
	report, err := readStewardReport(install)
	if err != nil {
		return nil, err
	}
	h.Report = report.Summary
	interventions, err := readLines[Intervention](interventionsPath(install))
	if err != nil {
		return nil, err
	}
	h.Interventions = interventions[max(0, len(interventions)-5):]
	stamp := now.Format(time.RFC3339Nano)
	switch status.Owner.State {
	case lane.OwnerStopped:
		h.State = "stopped"
	case lane.OwnerHeld:
		h.State = "held"
		h.StateReason = strings.TrimPrefix(status.View.AgentSummary(false), "held ")
	case lane.OwnerRunning:
		h.State = "agent running"
	}
	if status.RunningFix != nil && status.RunningFix.State == "resolving" {
		h.State = "resolving"
	}
	if (status.Paused || status.Stop != nil) && h.State != "stopped" && h.State != "held" {
		h.State = "paused"
	}
	if status.RunningProof != nil && status.RunningProof.State == "running" {
		h.State = "proving"
	}
	for _, e := range status.Queue {
		if e.State != StateWaiting && e.State != StateReturned && e.State != StateLanded {
			continue
		}
		end := stamp
		if e.State == StateLanded {
			end = e.LandedAt
		} else if e.State == StateReturned {
			end = e.ReturnedAt
		}
		minutes := elapsedMinutes(e.At, end)
		if e.State == StateLanded {
			landed, err := time.Parse(time.RFC3339Nano, end)
			if minutes == nil || err != nil || landed.Before(now.Add(-landedWindow)) || landed.After(now) {
				continue
			}
		}
		reason := e.Reason
		if reason == "" && e.State == StateWaiting {
			reason = "waiting for admission to the batch"
			if batch != nil && batch.State != BatchClosed {
				if slices.Contains(batch.Members, GoalSHA{Goal: e.Goal, SHA: e.SHA}) {
					reason = "selected in this batch"
				} else if !batch.Open {
					reason = "waiting for the next batch"
				} else if status.RunningFix != nil {
					reason = "a fix round is open"
				}
			}
		}
		state := e.State
		if status.RunningProof != nil && status.RunningProof.State == "running" && slices.Contains(status.RunningProof.Goals, e.Goal) {
			state = "proving"
		}
		h.Goals = append(h.Goals, GoalHealth{Goal: e.Goal, State: state, Minutes: minutes, Overdue: e.State != StateLanded && minutes != nil && *minutes > 60, Reason: reason})
	}
	if h.State == "held" && status.Owner.Barren > 0 {
		entries := slices.Clone(status.Queue)
		slices.SortStableFunc(entries, func(a, b Entry) int {
			aTime, aErr := time.Parse(time.RFC3339Nano, a.At)
			bTime, bErr := time.Parse(time.RFC3339Nano, b.At)
			if aErr != nil || bErr != nil {
				return 0
			}
			return aTime.Compare(bTime)
		})
		for _, e := range entries {
			if e.State == StateWaiting && !e.Held && e.SHA != "" {
				h.AdmissibleWaiting = append(h.AdmissibleWaiting, e.Goal)
			}
		}
	}
	if h.State == "stopped" && strings.Contains(status.Summary, "rebuild") {
		merge, err := seams.git(checkout, "rev-parse", "--verify", "--quiet", "MERGE_HEAD^{commit}")
		var exit *exec.ExitError
		if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
			return nil, err
		}
		if err == nil {
			h.StalePendingMerge = merge
		}
	}
	h.SealedBehindMain = batch != nil && batch.State != BatchClosed && !batch.Open && batch.SealedAt != "" && status.MainMove.Kind == "code"
	if status.Paused {
		h.Paused = &PauseHealth{Since: pause.At, Reason: pause.Reason, Minutes: elapsedMinutes(pause.At, stamp)}
	} else if status.Stop != nil {
		h.Paused = &PauseHealth{Since: status.Stop.At, Reason: status.Stop.Evidence, Minutes: elapsedMinutes(status.Stop.At, stamp)}
	}
	h.Agent.Alive = status.AgentAlive
	if p := status.AgentProgress; p != nil {
		h.Agent.Minutes = elapsedMinutes(p.Since, stamp)
		if p.LastStep != nil {
			h.Agent.Verb = p.LastStep.Verb
		}
		if p.Step != nil && !p.ProcessGone {
			h.Agent.WaitingOn = p.Step.Verb + " since " + lane.LocalText(p.Step.StartedAt)
		}
	}
	if r := status.RunningProof; r != nil && r.State == "running" {
		h.Agent.WaitingOn = "proof " + r.Attempt + " since " + lane.LocalText(r.Since)
		h.Proofs.Running = elapsedMinutes(r.Since, stamp)
	}
	if f := status.RunningFix; f != nil {
		h.Agent.WaitingOn = "fix/resolution " + f.Attempt + " (" + f.State + ") since " + lane.LocalText(f.ReadStartedAt)
	}
	if r := status.RunningRegeneration; r != nil && r.State == "running" {
		h.Agent.WaitingOn = "regeneration " + r.Goal + " since " + lane.LocalText(r.Since)
	}
	h.Agent.Silent = h.Agent.Alive && h.Agent.Minutes != nil && *h.Agent.Minutes > 20 && h.Agent.WaitingOn == ""
	main, err := git.main()
	if err != nil {
		return nil, err
	}
	questions, problems := channel.WalkQuestions(install)
	if len(problems) > 0 {
		return nil, fmt.Errorf("lane questions: %v", problems)
	}
	for _, q := range questions {
		if q.Goal != "" || q.About != "lane" || q.State != "open" {
			continue
		}
		machine := seams.Machine
		if machine == nil {
			machine = goal.ResolveMachine
		}
		name, err := machine(install)
		if err != nil {
			return nil, err
		}
		if q.Machine != name {
			continue
		}
		premise, err := LaneQuestionPremise(install, q, seams)
		if err != nil {
			return nil, err
		}
		h.Questions = append(h.Questions, QuestionHealth{ID: q.ID, Kind: q.Kind, Minutes: now.Sub(q.OpenedAt).Minutes(), Premise: premise, WaitsFor: LaneQuestionHeadline(q)})
	}
	incidents, err := seams.incidents(install, checkout, main)
	if err != nil {
		return nil, err
	}
	proven := slices.ContainsFunc(results, func(r Result) bool { return r.Trunk && r.Commit == main && (r.Result == Green || r.Result == Red) })
	for _, incident := range incidents {
		if incident.Closed != nil {
			continue
		}
		fact := IncidentHealth{ID: incident.ID, Test: strings.TrimSpace(incident.TestUnit + " " + incident.TestName), Class: incident.EntryClass(), Repeat: "none"}
		if len(incident.Sightings) > 0 {
			sight := incident.Sightings[len(incident.Sightings)-1]
			fact.Commit = sight.BaseCommit
			for _, r := range slices.Backward(results) {
				if r.Attempt == sight.Attempt {
					fact.Commit = r.Commit
					fact.Attribution = r.Cause
					fact.Repeat = healthRepeat(r)
					break
				}
			}
		}
		fact.MainMoved = fact.Commit != "" && main != fact.Commit
		fact.Unproven = fact.MainMoved && !proven
		h.Incidents = append(h.Incidents, fact)
	}
	if batch != nil {
		h.Batch = &BatchHealth{ID: batch.ID, State: batch.State, Open: batch.Open, SealedAt: batch.SealedAt, Members: batch.Members, Minutes: elapsedMinutes(batch.CreatedAt, stamp), Waiting: []BatchArrival{}}
		for _, e := range status.Queue {
			if e.State != StateWaiting {
				continue
			}
			arrival := BatchArrival{Goal: e.Goal, Boundary: "unknown", Admission: "unknown", Reason: "the seal or hand-in time is unrecorded"}
			at, atErr := time.Parse(time.RFC3339Nano, e.At)
			sealed, sealErr := time.Parse(time.RFC3339Nano, batch.SealedAt)
			if atErr == nil && sealErr == nil {
				arrival.Boundary = "handed in before the seal"
				if !at.Before(sealed) {
					arrival.Boundary = "arrived after the boundary"
				}
				arrival.Admission, arrival.Reason = batchAdmission(*batch, e)
				if at.Before(sealed) {
					arrival.Admission, arrival.Reason = admissionAtSeal(*batch, e, queue, incidents, sealed)
				}
			}

			h.Batch.Waiting = append(h.Batch.Waiting, arrival)
		}
	}
	load := seams.Load
	if load == nil {
		load = hostload.Read
	}
	h.Load.Now = load(now)
	h.Load.Saturated = h.Load.Now.Saturated()
	if r := status.LastProof; r != nil {
		samples := healthLoad(r.LoadStart, r.LoadEnd)
		h.Load.AtStart, h.Load.AtEnd = samples.AtStart, samples.AtEnd
	}
	scope := "full"
	if running, recorded, _, err := ReadRunning(install, seams); err != nil {
		return nil, err
	} else if recorded {
		if running.Impact {
			scope = "impact"
		}
		if running.Admission != nil {
			scope = running.Admission.Scope
		}
	}
	for _, r := range comparableProofs(results, scope) {
		h.Proofs.Durations = append(h.Proofs.Durations, *r.Minutes)
		h.Proofs.Samples = append(h.Proofs.Samples, healthLoad(r.LoadStart, r.LoadEnd))
	}
	if len(h.Proofs.Durations) > 0 {
		sorted := slices.Clone(h.Proofs.Durations)
		slices.Sort(sorted)
		median := (sorted[(len(sorted)-1)/2] + sorted[len(sorted)/2]) / 2
		h.Proofs.Median = &median
		h.Proofs.Drift = h.Proofs.Running != nil && *h.Proofs.Running > 1.5*median
	}
	file, err := os.Open(filepath.Join(install, "bin", "metasystem"))
	if err == nil {
		h.Engine.Revision, err = enginebuild.ReadStamp(file)
		err = errors.Join(err, file.Close())
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if revision := strings.TrimSuffix(strings.TrimPrefix(h.Engine.Revision, "dev-"), "-dirty"); revision != "" && revision != "unknown" && revision != "dev" {
		if _, err := seams.git(checkout, "cat-file", "-e", revision+"^{commit}"); err == nil {
			h.Engine.Known = true
			_, err := seams.git(checkout, "diff", "--quiet", revision, main, "--", relativeLanePath(install, checkout, "internal/landing/"), relativeLanePath(install, checkout, "internal/gaterun/"), relativeLanePath(install, checkout, "cmd/metasystem/intent_landing*.go"))
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				h.Engine.Stale = true
			} else if err != nil {
				return nil, err
			}
		}
	}
	if status.LastPush != nil {
		h.LastPush = status.LastPush.At
	}
	return h, nil
}
func relativeLanePath(install, checkout, path string) string {
	relative, _ := filepath.Rel(checkout, filepath.Join(install, path))
	return filepath.ToSlash(relative)
}
func healthRepeat(r Result) string {
	if r.RepeatComplete && len(r.FlakeRepeats) > 0 {
		return Green
	}
	passed := false
	for _, f := range r.Flakes {
		if len(f.Tests) == 0 {
			continue
		}
		for _, test := range f.Tests {
			output, exists := f.RepeatOutputs[test]
			if exists && output.Outcome == "fail" {
				return Red
			}
			if !exists || output.Outcome != "pass" {
				return "none"
			}
			passed = true
		}
	}
	if passed {
		return Green
	}
	return "none"
}

func (h Health) Rows() []Fired {
	rows := []Fired{}
	fire := func(row, subject string, minutes *float64, act string) {
		rows = append(rows, Fired{Row: row, Subject: subject, Minutes: measured(minutes), Act: act})
	}
	if len(h.AdmissibleWaiting) > 0 {
		fire("barren-hold", strings.Join(h.AdmissibleWaiting, ","), nil, "metasystem landing run --goals "+strings.Join(h.AdmissibleWaiting, ","))
	}
	if h.StalePendingMerge != "" {
		fire("stale-pending-merge", h.StalePendingMerge, nil, "abort it (the lane rebuilds on the current main)")
	}
	if h.SealedBehindMain {
		fire("sealed-batch-behind-main", h.Batch.ID, h.Batch.Minutes, "landing restart")
	}
	for _, q := range h.Questions {
		if q.Premise == "gone" {
			fire("stale-question", q.ID, &q.Minutes, "withdraw it, prove main, re-ask only if red")
		}
	}
	for _, i := range h.Incidents {
		if i.Unproven {
			fire("unproven-incidents", i.ID, nil, "prove main before asking anything")
		}
	}
	if r := h.LastProof; r != nil {
		saturated := r.LoadStart != nil && r.LoadStart.Saturated() || r.LoadEnd != nil && r.LoadEnd.Saturated()
		if r.Result == Red && saturated && len(r.FlakeRepeats) == 0 && len(r.Flakes) == 0 && !r.RepeatComplete {
			fire("suspected-contention", r.Commit, r.Minutes, "repeat by name before classifying; the recorded attribution stands")
		}
		if healthRepeat(*r) == Green {
			fire("confirmed-flake", r.Attempt, r.Minutes, "a flake: register it, no incident")
		}
	}
	if h.Agent.Silent {
		waiting := 0
		for _, g := range h.Goals {
			if g.State == StateWaiting {
				waiting++
			}
		}
		subject := "with nothing to do"
		if waiting > 0 {
			subject = fmt.Sprintf("with %d goals waiting", waiting)
			if waiting == 1 {
				subject = "with 1 goal waiting"
			}
		}
		fire("silent-agent", subject, h.Agent.Minutes, "end the silent landing agent session and wake the keeper")
	}
	if h.Engine.Stale {
		fire("stale-engine", h.Engine.Revision, nil, "redeploy the engine from main (deploy now); the steward acts on it once landing-deploys-the-engine rebuilds the install's engine")
	}
	if b := h.Batch; b != nil && b.State != BatchClosed && !b.Open && len(b.Members) == 1 {
		for _, a := range b.Waiting {
			if a.Boundary == "handed in before the seal" && a.Admission == "admissible" {
				fire("batch-of-one", a.Goal, b.Minutes, "a goal admissible before the seal was left out")
			}
		}
	}
	if h.Proofs.Drift {
		fire("proof-drift", "running proof", h.Proofs.Running, fmt.Sprintf("%s min against a median of %s; load at start %s/end %s", measured(h.Proofs.Running), measured(h.Proofs.Median), measured(h.Load.AtStart), measured(h.Load.AtEnd)))
	}
	if h.AbandonedResolution != "" {
		fire("abandoned-resolution", h.AbandonedResolution, nil, "rebuild the merge on the current main")
	}
	for _, g := range h.Goals {
		if g.State != StateLanded && g.Overdue {
			fire("overdue-goal", g.Goal, g.Minutes, fmt.Sprintf("%s: %s min; %s", g.Goal, measured(g.Minutes), g.Reason))
		}
	}
	if p := h.Paused; p != nil && p.Minutes != nil && *p.Minutes > 30 {
		fire("paused", p.Reason, p.Minutes, lane.LocalText(p.Since)+"; "+p.Reason)
	}
	for _, q := range h.Questions {
		if q.Minutes > 30 {
			fire("question-open", q.ID, &q.Minutes, q.WaitsFor)
		}
	}
	return rows
}
func (h Health) Line() (line string) {
	defer func() {
		if h.Report != "" {
			line += "; " + h.Report
		}
		if len(h.Interventions) > 0 {
			last := h.Interventions[len(h.Interventions)-1]
			line += fmt.Sprintf("; steward: %s %s min ago", last.What, measured(elapsedMinutes(last.At, h.observedAt.Format(time.RFC3339Nano))))
		}
	}()
	state := h.State
	if h.State == "held" && h.StateReason != "" {
		state += " " + h.StateReason
	}
	rows := h.Rows()
	if len(rows) == 0 {
		return "lane " + state + "; nothing fires"
	}
	names := []string{}
	for _, r := range rows {
		names = append(names, r.Row)
	}
	oldest := 0.
	for _, g := range h.Goals {
		if g.Minutes != nil && *g.Minutes > oldest {
			oldest = *g.Minutes
		}
	}
	load := "unmeasured"
	if h.Load.Now.Available {
		load = fmt.Sprintf("%g", h.Load.Now.Load1m)
	}
	return fmt.Sprintf("lane %s; %d rows fire: %s; %d goals, oldest %g min; load %s", state, len(rows), strings.Join(names, ", "), len(h.Goals), oldest, load)
}
func (h Health) Text() string {
	var out strings.Builder
	fmt.Fprintln(&out, h.Line())
	fmt.Fprintln(&out, "report: "+h.ReportPath)
	fmt.Fprintf(&out, "goals: %d\n", len(h.Goals))
	for _, g := range h.Goals {
		fmt.Fprintf(&out, "  %s: %s, %s min; %s\n", g.Goal, g.State, measured(g.Minutes), g.Reason)
	}
	fmt.Fprintln(&out, "batch:")
	if b := h.Batch; b != nil {
		fmt.Fprintf(&out, "  %s: open %t, sealed %s, %d members, %s min\n", b.ID, b.Open, lane.LocalText(b.SealedAt), len(b.Members), measured(b.Minutes))
		for _, a := range b.Waiting {
			fmt.Fprintf(&out, "  %s: %s; %s; %s\n", a.Goal, a.Boundary, a.Admission, a.Reason)
		}
	} else {
		fmt.Fprintln(&out, "  none")
	}
	fmt.Fprintf(&out, "agent: alive %t; last step %s, %s min; waiting on %s\n", h.Agent.Alive, h.Agent.Verb, measured(h.Agent.Minutes), h.Agent.WaitingOn)
	if p := h.Paused; p != nil {
		fmt.Fprintf(&out, "paused: since %s, %s min; %s\n", lane.LocalText(p.Since), measured(p.Minutes), p.Reason)
	}
	fmt.Fprintln(&out, "questions:")
	for _, q := range h.Questions {
		fmt.Fprintf(&out, "  %s: %s, %g min, premise %s; %s\n", q.ID, q.Kind, q.Minutes, q.Premise, q.WaitsFor)
	}
	fmt.Fprintln(&out, "incidents:")
	for _, i := range h.Incidents {
		fmt.Fprintf(&out, "  %s: commit %s, %s, class %s, attribution %v, repeat %s, main moved %t, unproven %t\n", i.ID, i.Commit, i.Test, i.Class, i.Attribution, i.Repeat, i.MainMoved, i.Unproven)
	}
	known := "unknown"
	if h.Engine.Known {
		known = h.Engine.Revision
	}
	fmt.Fprintf(&out, "engine: %s; stale %t\nproofs: %v min; median %s; running %s min\n", known, h.Engine.Stale, h.Proofs.Durations, measured(h.Proofs.Median), measured(h.Proofs.Running))
	for index, sample := range h.Proofs.Samples {
		fmt.Fprintf(&out, "  proof %d: load at start %s/end %s\n", index+1, measured(sample.AtStart), measured(sample.AtEnd))
	}
	current := "unmeasured"
	if h.Load.Now.Available {
		current = fmt.Sprintf("%g", h.Load.Now.Load1m)
	}
	fmt.Fprintf(&out, "load: %s; saturated %t; at start %s/end %s\nlast push: %s\n", current, h.Load.Saturated, measured(h.Load.AtStart), measured(h.Load.AtEnd), lane.LocalText(h.LastPush))
	for _, r := range h.Rows() {
		fmt.Fprintf(&out, "%s: %s (%s min): %s\n", r.Row, r.Subject, r.Minutes, r.Act)
	}
	fmt.Fprintln(&out, "interventions:")
	for _, act := range h.Interventions {
		fmt.Fprintf(&out, "  %s: %s; %s; %s; %s\n", lane.LocalText(act.At), act.What, act.Why, act.How, act.Outcome)
	}
	return strings.TrimSpace(out.String())
}

func admissionAtSeal(batch Batch, entry Entry, lines []Line, incidents []goal.TrunkRedEntry, sealed time.Time) (string, string) {
	var open []goal.TrunkRedEntry
	for _, incident := range incidents {
		opened, err := time.Parse(time.RFC3339Nano, incident.Opened)
		if err != nil {
			return "unknown", "the records cannot establish which incidents were open at the seal"
		}
		if incident.Closed != nil {
			closed, err := time.Parse(time.RFC3339Nano, incident.Closed.At)
			if err != nil || closed.Before(opened) {
				return "unknown", "the records cannot establish which incidents were open at the seal"
			}
			if !closed.After(sealed) {
				continue
			}
		}
		if !opened.After(sealed) {
			incident.Closed = nil
			open = append(open, incident)
		}
	}
	var snapshot []Line
	for _, line := range lines {
		at, err := time.Parse(time.RFC3339Nano, line.At)
		if err != nil {
			if line.Goal == entry.Goal && line.SHA == entry.SHA {
				return "unknown", "the queue cannot establish eligibility at the seal"
			}
			continue
		}
		if !at.After(sealed) {
			snapshot = append(snapshot, line)
		}
	}
	for _, recorded := range slices.Backward(HoldEntries(entriesOf(snapshot), open)) {
		if recorded.Goal == entry.Goal && recorded.SHA == entry.SHA {
			return batchAdmission(batch, recorded)
		}
	}
	return "unknown", "eligibility at the seal is unrecorded"
}
