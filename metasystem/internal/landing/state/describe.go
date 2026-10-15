package state

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

type WordKey struct{ Section, Word string }
type Wording struct{ Phrase, Verb, Person, Agent string }
type Description struct {
	Headline, Summary, ProofHeadline, Verb, Person, Agent string
	Facts, Steward, Questions, Remedies                   []string
	HealthFacts                                           map[string][]string
}

// RenderInputs carries transient observations separately from the persisted state.
type RenderInputs struct {
	Now                            time.Time
	UnitsDone                      int
	UnitsTotal                     *int
	Minutes                        *float64
	ReadMinutes, CheckMinutes      float64
	Waiting                        bool
	UnreadHandIn                   string
	PID                            *int64
	Unready, Unread                bool
	MainCommits                    int
	WaitingReasons, RestartReasons []string
	HandIns                        map[int]HandInInput
	Questions                      map[string]QuestionInput
	Health                         map[string][]MeasuredFact
}
type HandInInput struct{ WaitNext, WaitFix bool }
type QuestionInput struct {
	FirstLine string
	Policy    bool
}
type MeasuredFact struct {
	Subject, Word, Reason string
	Minutes               *float64
}

// Words owns the language of each recorded state; an absent agent is described honestly.
var Words = map[WordKey]Wording{
	{"agent", ""}:                   {"the landing agent's state is unknown; examine the lane health", "none", "examine the lane health", "observe the lane"},
	{"lane", "running"}:             {"The landing lane's agent is at work", "none", "nothing to do", "continue the lane"},
	{"lane", "paused"}:              {"paused by {by}{reason}", "landing resume", "resume when the pause is no longer needed", "wait for the person"},
	{"lane", "stopped"}:             {"stopped by {by}{reason}; run: metasystem landing resume", "landing resume", "resume after examining the reason", "wait for the person"},
	{"agent", "idle"}:               {"The landing lane's agent is at work", "none", "nothing to do", "wait for the keeper"},
	{"agent", "ended"}:              {"idle; its landing agent starts when there is work", "none", "nothing to do", "wait for the keeper"},
	{"agent", "stepping"}:           {"The landing lane's agent is at work", "none", "nothing to do", "finish the step"},
	{"agent", "step-process-gone"}:  {"the landing agent's step process ended; run: metasystem landing resume", "landing resume", "resume the lane after examining the step", "settle the step"},
	{"batch.execution", "prepared"}: {"the batch is prepared; the keeper starts its checks", "none", "nothing to do", "start checks"},
	{"batch.execution", "running"}:  {"the batch is running; the keeper waits for its checks", "none", "nothing to do", "wait for checks"},
	{"batch.execution", "closed"}:   {"the batch is closed; the keeper selects the next work", "none", "nothing to do", "select work"},
	{"batch.admission", "open"}:     {"the batch is open; eligible hand-ins may join it", "none", "nothing to do", "admit eligible work"},
	{"batch.admission", "sealed"}:   {"the batch is sealed; later hand-ins wait for the next batch", "none", "nothing to do", "finish this batch"},
	{"hand_in", "waiting"}:          {"waiting for admission to the batch", "none", "nothing to do", "admit eligible work"},
	{"hand_in", "returned"}:         {"returned to its seat; repair the reason before handing it in again", "none", "examine the return reason", "wait for repaired work"},
	{"hand_in", "landed"}:           {"landed on main; the keeper selects the next work", "none", "nothing to do", "select work"},
	{"hand_in", "superseded"}:       {"superseded by a newer hand-in; the keeper uses that hand-in", "none", "nothing to do", "use the newer hand-in"},
	{"resolution", "resolving"}:     {"Resolving {conflicts} conflicts of {goal} ({paths})", "none", "nothing to do", "resolve the conflicts"},
	{"resolution", "resolved"}:      {"Resolved merge of {goal}, read pending", "none", "nothing to do", "read the resolution"},
	{"resolution", "building"}:      {"Fixing {units} of {goal} on {commit} (fix round {round})", "none", "nothing to do", "build the fix"},
	{"resolution", "running"}:       {"Fixing {units} of {goal} on {commit} (fix round {round})", "none", "nothing to do", "build the fix"},
	{"resolution", "reviewing"}:     {"Fixing {units} of {goal} on {commit} (fix round {round})", "none", "nothing to do", "read the fix"},
	{"resolution", "abandoned"}:     {"the resolution was abandoned: {abandoned}; run: metasystem landing resume", "landing resume", "resume on the current main", "rebuild the batch"},
	{"resolution", "done"}:          {"the fix is done; the keeper continues the lane", "none", "nothing to do", "continue the lane"},
	{"resolution", "closed"}:        {"the fix is closed; the keeper continues the lane", "none", "nothing to do", "continue the lane"},
	{"proof", "running"}:            {"proving ({done} of {total} units, {minutes} min); proving {subject} ({done} of {total} units, {minutes} min); {trunk}the agent waits for the keeper; nothing to do", "none", "nothing to do", "proving; the agent waits for the keeper"},
	{"proof", "died"}:               {"the lane's check died; run: metasystem landing resume", "landing resume", "resume the failed check", "repeat the proof"},
	{"proof", "pending"}:            {"the lane's check pending; run: metasystem landing resume", "landing resume", "resume the pending check", "admit the proof"},
	{"proof", "failed"}:             {"the lane's check failed; run: metasystem landing resume", "landing resume", "resume after examining the failure", "repeat the proof"},
	{"proof", "green"}:              {"the check is green; the keeper continues the lane", "none", "nothing to do", "continue the lane"},
	{"proof", "held"}:               {"the check is held; examine the reason before resuming", "landing resume", "resume after examining the reason", "wait for the hold to end"},
	{"proof", "skipped"}:            {"the check was skipped; examine its saved reason", "landing prove", "examine why the check did not run", "prove when the prerequisites hold"},
	{"proof", "cancelled"}:          {"the check was cancelled; the keeper continues the recorded decision", "none", "nothing to do", "continue the recorded decision"},
	{"proof", "red"}:                {"the check is red; examine its failing tests", "landing prove", "examine the failing tests", "classify the failed check"},
	{"main", "green"}:               {"main is proven green; the keeper continues the lane", "none", "nothing to do", "continue the lane"},
	{"main", "red"}:                 {"main is red", "landing prove", "land a fix on main, then metasystem landing prove --trunk", "wait for a fix"},
	{"main", "unproven"}:            {"main is unproven; prove it before relying on its checks", "landing prove", "prove main", "prove main"},
	{"question", "current"}:         {"Waiting for a person's answer to question %s %s", "none", "answer the question", "wait for the person"},
	{"question", "gone"}:            {"the question's premise is gone; withdraw it before asking again", "landing prove", "withdraw the stale question", "withdraw it and prove main"},
	{"question", "unknown"}:         {"the question's premise is unknown; examine its recorded facts", "none", "examine the question", "read the premise"},
	{"holds", "held"}:               {"held after {barren} runs that left the lane unchanged; a person runs metasystem landing resume", "landing run --goals", "choose the waiting goals to run", "wait for the person's selection"},
	{"holds", "pending"}:            {"the batch needs a person's selection; run: metasystem landing resume", "landing run --goals", "choose the waiting goals to run", "wait for the person's selection"},
	{"engine", "incident"}:          {"an engine incident at {incidents}; run: metasystem landing resume", "landing resume", "examine the engine incident before resuming", "wait for engine repair"},
}

func WordsText() string {
	lines := make([]string, 0, len(Words))
	for k, w := range Words {
		lines = append(lines, strings.Join([]string{k.Section, k.Word, w.Phrase, w.Verb}, "\t"))
	}
	sort.Strings(lines)
	return "section\tword\tphrase\tverb\n" + strings.Join(lines, "\n")
}
func SinceText(s Since, word string) string {
	if s.At.IsZero() {
		return "time unrecorded"
	}
	at := s.At.Local().Format("15:04")
	if s.Observed {
		source := strings.TrimSuffix(s.Source, ".json")
		if source == "" {
			source = "the record"
		}
		if word == "" {
			word = "transition"
		}
		if source == "batch" {
			source = "the batch record"
		}
		return "seen since " + at + " (" + source + " carries no " + word + " time)"
	}
	return "since " + at
}

// Describe reads only its argument. Observation clocks never become running durations.
func Describe(r Record, inputs RenderInputs) Description {
	key := WordKey{"agent", r.Agent.Word}
	for _, c := range []struct {
		match bool
		key   WordKey
	}{
		{r.Lane.Word == "paused" || r.Lane.Word == "stopped", WordKey{"lane", r.Lane.Word}},
		{r.Holds.Barren >= 2, WordKey{"holds", "held"}},
		{r.Holds.SelectionPending, WordKey{"holds", "pending"}},
		{r.Main.Word == "red", WordKey{"main", "red"}},
		{r.Resolution.Word != "" && r.Resolution.Word != "done" && r.Resolution.Word != "closed", WordKey{"resolution", r.Resolution.Word}},
		{r.Proof.Word == "running" || r.Proof.Word == "died" || r.Proof.Word == "pending" || r.Proof.Word == "failed", WordKey{"proof", r.Proof.Word}},
		{r.Main.CheckNotRun && r.Proof.Word != "running" && (r.Resolution.Word == "" || r.Resolution.Word == "done" || r.Resolution.Word == "closed"), WordKey{"main", "unproven"}},
		{len(r.Engine.Incidents) > 0, WordKey{"engine", "incident"}},
	} {
		if c.match {
			key = c.key
		}
	}
	w, ok := Words[key]
	if !ok {
		w = Wording{"the lane state is unknown; examine the lane health", "none", "examine the lane health", "observe the lane"}
	}
	d := Description{Headline: w.Phrase, Verb: w.Verb, Person: w.Person, Agent: w.Agent}
	f := r.Resolution
	by, reason := r.Lane.By, r.Lane.Reason
	if by == "" {
		by = "the person"
	}
	if reason != "" {
		reason = " (" + reason + ")"
	}
	commit := f.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	total, minutes := "counting", "unknown"
	if inputs.UnitsTotal != nil {
		total = fmt.Sprint(*inputs.UnitsTotal)
	}
	if inputs.Minutes != nil && !r.Proof.Since.Observed {
		minutes = fmt.Sprintf("%.1f", *inputs.Minutes)
	}
	incidents := []string{}
	for _, i := range r.Engine.Incidents {
		incidents = append(incidents, i.At.Local().Format("15:04")+" in landing "+i.Verb+": "+i.Panic)
	}
	d.Headline = strings.NewReplacer("{by}", by, "{reason}", reason, "{barren}", fmt.Sprint(r.Holds.Barren), "{incidents}", strings.Join(incidents, "; "), "{conflicts}", fmt.Sprint(len(f.Paths)), "{goal}", f.Goal, "{paths}", strings.Join(f.Paths, ", "), "{units}", strings.Join(f.Units, ", "), "{commit}", commit, "{round}", fmt.Sprint(max(1, f.Round)), "{abandoned}", f.Reason, "{done}", fmt.Sprint(inputs.UnitsDone), "{total}", total, "{minutes}", minutes).Replace(w.Phrase)
	if key.Section == "proof" && key.Word == Running {
		subject := "main"
		if !r.Proof.Trunk && len(r.Proof.Members) > 0 {
			names := []string{}
			for _, member := range r.Proof.Members {
				names = append(names, member.Goal)
			}
			subject = fmt.Sprintf("the batch of %d — %s", len(names), strings.Join(names, ", "))
		}
		trunk := ""
		if subject == "main" {
			trunk = "the lane proves main; "
		}
		d.Headline = strings.NewReplacer("{subject}", subject, "{trunk}", trunk).Replace(d.Headline)
	}
	if key.Section == "resolution" {
		if f.ReadPending {
			d.Headline = fmt.Sprintf("reading the resolution of %s (%g min)", f.Goal, inputs.ReadMinutes)
		}
		if f.CheckResult != "" && !f.ReadPending {
			d.Headline += fmt.Sprintf("; last check %s in %.2f minutes", f.CheckResult, inputs.CheckMinutes)
		}
		if f.Word != "abandoned" {
			d.Headline += "; nothing to do"
		}
		if f.LaunchError != "" {
			d.Headline = "launch of the resolution failed at " + f.LaunchAt.Local().Format("2006-01-02 15:04 MST") + ": " + f.LaunchError
			d.Verb, d.Person = "landing resume", "examine the failed launch before resuming"
		}
	}
	if key.Section == "main" {
		if r.Main.CheckNotRun {
			d.Headline = "main's last check could not run (" + r.Main.Reason + ")"
			if r.Main.RetryAllowed {
				next := "at its next wake"
				if !r.Main.RetryAt.IsZero() {
					next = "at " + r.Main.RetryAt.Local().Format("2006-01-02 15:04 MST")
				}
				d.Headline += "; the lane will check main again " + next
				d.Verb, d.Person, d.Agent = "none", "nothing to do", "check main again"
			} else {
				d.Headline += "; run: metasystem landing prove --trunk"
				d.Person = "examine why the check could not run, then metasystem landing prove --trunk"
				d.Agent = "wait for the person's check"
			}
		} else {
			d.Headline = redMain(r.Main, r.HandIns)
		}
	}
	if r.Main.Word != Red && r.Main.Flakes != "" && key.Section != "proof" && key.Section != "resolution" {
		d.Headline = "main has unstable checks: " + r.Main.Flakes + "; run: metasystem landing resume"
		if r.Main.FlakePublished {
			d.Headline = "main is proven with unstable checks: " + r.Main.Flakes + "; run: metasystem landing resume"
		}
		d.Verb, d.Person = "landing resume", "resume after examining the unstable checks"
	}
	if r.Stop != nil && key.Section != "proof" && key.Section != "resolution" && !r.Main.CheckNotRun {
		d.Headline = stopText(*r.Stop)
		d.Verb = "landing resume"
		d.Person = Remedy(r.Stop.Command)
	}
	for i, f := range []Fact{r.Lane.Fact, {r.Agent.Word, r.Agent.Since}, r.Batch.Execution, r.Batch.Admission, r.Resolution.Fact, r.Proof.Fact, r.Main.Fact} {
		if f.Word != "" || !f.Since.At.IsZero() {
			d.Facts = append(d.Facts, []string{"lane", "agent", "batch.execution", "batch.admission", "resolution", "proof", "main"}[i]+" "+f.Word+" "+SinceText(f.Since, f.Word))
		}
	}
	for _, h := range r.HandIns {
		d.Facts = append(d.Facts, h.Goal+": "+h.Word+" "+SinceText(h.Since, h.Word)+"; "+h.Reason)
	}
	if r.Holds.Barren > 0 || r.Holds.SelectionPending {
		d.Facts = append(d.Facts, fmt.Sprintf("holds: %d unchanged runs; %s", r.Holds.Barren, SinceText(r.Holds.Since, "hold")))
	}
	for _, q := range r.Questions {
		since := "since " + q.Since.At.Local().Format("2006-01-02 15:04 MST")
		if q.Since.Observed || q.Since.At.IsZero() {
			since = SinceText(q.Since, "question")
		}
		line := fmt.Sprintf(Words[WordKey{"question", "current"}].Phrase, q.ID, since)
		first := inputs.Questions[q.ID].FirstLine
		if !inputs.Questions[q.ID].Policy {
			if at := strings.IndexAny(first, ".?!"); at >= 0 {
				first = first[:at+1]
			}
		}
		if first != "" {
			line += "; " + first
		}
		if !inputs.Questions[q.ID].Policy {
			line += "; metasystem question show channel:" + q.ID + "; withdraw: metasystem question withdraw channel:" + q.ID + " --reason TEXT"
		}
		d.Questions = append(d.Questions, line)
	}
	if key.Section == "agent" {
		if inputs.Waiting && (key.Word == "idle" || key.Word == "ended") {
			d.Headline = "idle; its agent starts within one tick (15 s)"
		}
		if inputs.PID != nil && key.Word == "stepping" {
			d.Headline += fmt.Sprintf(" (pid %d)", *inputs.PID)
		}
		if inputs.Unready {
			d.Headline = "The landing lane can't run its agent; run: metasystem landing resume"
			if inputs.Unread {
				d.Headline = "the landing agent's state is unknown; run: metasystem landing resume"
			}
			d.Verb, d.Person = "landing resume", "resume after examining the reason"
		}
	}
	if inputs.UnreadHandIn != "" {
		d.Headline = inputs.UnreadHandIn + "; run: metasystem landing resume"
		d.Verb, d.Person = "landing resume", "examine the unread hand-in before resuming"
	}
	if key.Section == "proof" || key.Section == "resolution" || r.Main.Word == "red" || r.Main.Flakes != "" {
		d.ProofHeadline = d.Headline
	}
	d.Summary = d.Headline
	if len(d.Questions) > 0 {
		d.Summary = d.Questions[0]
		if r.Main.Word == "red" {
			d.Summary = d.Headline + ". " + d.Summary
		}
	}
	if inputs.UnreadHandIn != "" {
		d.Summary = d.Headline
	}
	if !r.Batch.SealedAt.IsZero() {
		d.Summary += fmt.Sprintf("; batch sealed %s: %d members", r.Batch.SealedAt.Local().Format("15:04"), len(r.Batch.Members))
	}
	for _, reason := range inputs.WaitingReasons {
		d.Summary += "; " + reason
	}
	for _, reason := range inputs.RestartReasons {
		d.Summary += "; " + reason
	}
	if r.Batch.MainMove == "records-only" {
		d.Summary += fmt.Sprintf("; main gained %d record commits since the batch base; the batch stands", inputs.MainCommits)
	}
	if r.Batch.MainMove == "code" {
		d.Summary += "; main moved: code change"
		if !strings.Contains(d.Summary, "metasystem landing ") && r.Proof.Word != "running" {
			d.Summary += "; run: metasystem landing resume"
		}
	}
	for _, h := range r.HandIns {
		if inputs.HandIns[h.Ordinal].WaitNext {
			d.Summary += "; " + h.Goal + " waits for the next batch"
		}
		if inputs.HandIns[h.Ordinal].WaitFix {
			d.Summary += "; " + h.Goal + " waits: a fix round is open"
		}
	}
	if strings.Contains(d.Summary, "; nothing to do") {
		d.Summary = strings.ReplaceAll(d.Summary, "; nothing to do", "") + "; nothing to do"
	}
	if r.LastObservation != nil {
		d.Facts = append(d.Facts, "last observation failed at "+r.LastObservation.At.Local().Format("15:04")+": "+strings.Join(r.LastObservation.Problems, "; "))
	}
	d.HealthFacts = map[string][]string{}
	for section, facts := range inputs.Health {
		for _, fact := range facts {
			minutes := "unmeasured"
			if fact.Minutes != nil {
				minutes = fmt.Sprintf("%g", *fact.Minutes)
			}
			line := fmt.Sprintf("%s: %s (%s min)", fact.Subject, fact.Word, minutes)
			if fact.Reason != "" {
				line += "; " + fact.Reason
			}
			d.HealthFacts[section] = append(d.HealthFacts[section], line)
		}
	}
	d.Steward = append([]string{d.Headline}, d.Facts...)
	if d.Verb != "none" {
		d.Remedies = append(d.Remedies, "metasystem "+d.Verb)
	}
	if r.RemedyCommand != "" {
		d.Remedies = []string{Remedy(r.RemedyCommand)}
	}
	return d
}
func redMain(m Main, entries []HandIn) string {
	text := Words[WordKey{"main", "red"}].Phrase
	incidents := strings.Join(m.OpenIncidents, ", ")
	if incidents != "" {
		text += " (incident " + incidents + ")"
	}
	text += ": " + m.Reason + "; " + Words[WordKey{"main", "red"}].Person
	for _, e := range entries {
		if e.Word == "waiting" && incidents != "" {
			text += "; or metasystem landing land-anyway " + e.Goal + " --reason TEXT (" + LandAnywayImpact(e.Goal, incidents) + ")"
		}
	}
	return text
}
func LandAnywayImpact(goal, incident string) string {
	return "lets " + goal + " land while main's incident " + incident + " is open; undo: metasystem landing give-back " + goal + " --reason TEXT"
}
func Remedy(command string) string {
	words := strings.Fields(command)
	if len(words) < 3 || words[0] != "metasystem" {
		return command
	}
	if words[1] == "landing" {
		if words[2] == "restart" || words[2] == "run" && slices.ContainsFunc(words[3:], func(w string) bool { return w == "--goals" || strings.HasPrefix(w, "--goals=") }) {
			return "metasystem landing resume"
		}
		if words[2] == "return" && len(words) > 3 && !strings.HasPrefix(words[3], "--") {
			return "metasystem landing give-back " + words[3] + " --reason TEXT"
		}
	}
	if words[1] == "work" && words[2] == "land" && len(words) > 3 && !strings.HasPrefix(words[3], "--") {
		for i, w := range words[4:] {
			if w == "--exception=GOAL_LAND_TRUNK_RED" || w == "--exception" && i+5 < len(words) && words[i+5] == "GOAL_LAND_TRUNK_RED" {
				return "metasystem landing land-anyway " + words[3] + " --reason TEXT"
			}
		}
	}
	return command
}
func stopText(s Stop) string {
	why := s.Subject + " needs a person's decision"
	for _, fact := range []string{strings.TrimSuffix("; cause: "+s.Kind, "; cause: "), strings.TrimSuffix("; "+s.Name, "; ")} {
		why += fact
	}
	if s.Kind == "main" && s.Name != "" {
		entries := []HandIn{}
		for _, g := range slices.DeleteFunc(strings.Split(s.Subject, ", "), func(g string) bool { return g == "main" || g == "lane" }) {
			entries = append(entries, HandIn{Goal: g, Fact: Fact{Word: "waiting"}})
		}
		why = redMain(Main{Fact: Fact{Word: "red"}, OpenIncidents: []string{s.Name}}, entries)
		if len(s.Tests) > 0 {
			why += "; failing tests: " + strings.Join(s.Tests, ", ")
		}
		return "the lane stopped: " + why
	}
	if len(s.Tests) > 0 {
		why += "; failing tests: " + strings.Join(s.Tests, ", ")
	}
	command := Remedy(s.Command)
	if strings.Contains(command, "landing prove --classify ") {
		why = strings.Replace(why, s.Subject+" needs a person's decision", "classify the failed check", 1)
	}
	return "the lane stopped: " + why + "; run: " + command
}

// Phrases owns the lane's person-facing state messages outside the recorded word rows.
var Phrases = map[string]string{
	"landing-reservation-held":                                           "the landing agent's start is held: %s (reservation %s); %s",
	"goal-s-cannot-be-proven-held-repair-the-base":                       "goal %s cannot be proven held; repair the base tree, then take it over with metasystem goal claim %s --take-over --reason TEXT",
	"carry-debt-is-unpaid-word-s-landed-as-s":                            "carry debt is unpaid: word %s landed as %s without a carried row; run metasystem work land %s --using-exception %s",
	"prepared-for-carried-delivery-under-exception-s-code-s":             "prepared for carried delivery under exception %s code %s; not yet landed",
	"held-unreadable-message":                                            "held: unreadable: ",
	"held-warning-s-s-s":                                                 "held: warning %s: %s: %s",
	"agent-landings-are-goal-work-name-the-held-goal":                    "agent landings are goal work: name the held goal with --goal, or the ledger must be declared Goal-free",
	"the-goal-records-couldn-t-be-fetched-so-nothing":                    "the goal records couldn't be fetched, so nothing was landed",
	"the-goal-records-came-back-incomplete-so-nothing-was":               "the goal records came back incomplete, so nothing was landed",
	"goal-fetch-returned-no-accepted-ledger-tip":                         "goal fetch returned no accepted ledger tip: ",
	"can-t-be-read-from-the-goal-records-so":                             " can't be read from the goal records, so nothing was landed",
	"reads-incomplete-in-the-goal-records-so-nothing-was":                " reads incomplete in the goal records, so nothing was landed",
	"carried-landing-stopped-at-rebase-conflict":                         "carried landing stopped at rebase conflict",
	"so-nothing-was-landed":                                              ", so nothing was landed",
	"the-goal-couldn-t-be-reserved-for-this-exception":                   "the goal couldn't be reserved for this exception landing, so nothing was landed",
	"the-goal-s-reservation-for-this-landing-came-back":                  "the goal's reservation for this landing came back incomplete, so nothing was landed",
	"reservation-returned-an-incomplete-row":                             "reservation returned an incomplete row: ",
	"the-files-of-the-unfinished-exception-commit-can-t":                 "the files of the unfinished exception commit can't be read, so nothing was landed",
	"the-unfinished-exception-commit-conflicts-with-main-so-nothing":     "the unfinished exception commit conflicts with main, so nothing was landed",
	"origin-couldn-t-be-fetched-so-nothing-was-landed":                   "origin couldn't be fetched, so nothing was landed",
	"carried-intent-returned-no-entry":                                   "carried intent returned no entry",
	"goal-held-at-the-rebased-base":                                      "goal held at the rebased base",
	"main-moved-while-this-landed-so-nothing-was-pushed":                 "main moved while this landed, so nothing was pushed",
	"already-landed-as":                                                  "already landed as ",
	"has-expired-so-nothing-was-landed":                                  " has expired, so nothing was landed",
	"wasn-t-recorded-by-a-person-at-an-enrolled":                         " wasn't recorded by a person at an enrolled terminal, so nothing was landed",
	"is-in-a-state-this-landing-doesn-t-know":                            " is in a state this landing doesn't know, so nothing was landed",
	"the-checkout-is-held-under-a-claim-and-an":                          "the checkout is held under a claim, and an agent commit names its owner lineage",
	"metasystem-conf-names-no-test-contract-testing-contract-so":         "metasystem.conf names no test contract (testing.contract), so nothing can be landed",
	"the-running-metasystem-program-can-t-be-read-so":                    "the running metasystem program can't be read, so nothing was committed",
	"the-change-includes-files-the-running-system-writes-which":          "the change includes files the running system writes, which are never landed",
	"test-verify-returned-success-evidence-with-a-failing-process":       "test verify returned success evidence with a failing process status",
	"test-verify-returned-an-unreadable-missing-groups-list":             "test verify returned an unreadable missing-groups list",
	"test-verify-returned-an-unreadable-failing-groups-list":             "test verify returned an unreadable failing-groups list",
	"after-resolving-it-then-push-transport-too":                         "after resolving it; then push transport too",
	"after-resolving-the-transport-remote":                               "after resolving the transport remote",
	"the-landing-stopped-at":                                             "the landing stopped at ",
	"fix-what-stopped-it-verbose-shows-the-step-s":                       "fix what stopped it (--verbose shows the step's log), then repeat this command",
	"this-checkout-s-role-couldn-t-be-read-so":                           "this checkout's role couldn't be read, so nothing was landed",
	"the-commit-message-couldn-t-be-saved-for-the":                       "the commit message couldn't be saved for the commit, so nothing was landed",
	"the-commit-message-file-can-t-be-read-so":                           "the commit message file can't be read, so nothing was landed",
	"a-landing-needs-its-commit-message-so-nothing-was":                  "a landing needs its commit message, so nothing was landed",
	"staged-lands-what-is-staged-so-it-takes-no":                         "--staged lands what is staged, so it takes no --path, so nothing was landed",
	"nothing-to-land-no-path-and-no-staged-so":                           "nothing to land: no --path and no --staged, so nothing was landed",
	"tests-and-test-receipt-don-t-go-together-so":                        "--tests and --test-receipt don't go together, so nothing was landed",
	"test-receipt-goes-with-chain-or-direct-fix-tier":                    "--test-receipt goes with --chain or --direct-fix tier-1, so nothing was landed",
	"an-exception-landing-names-its-goal-so-nothing-was":                 "an exception landing names its goal, so nothing was landed",
	"recertification-goes-with-chain-so-nothing-was-landed":              "--recertification goes with --chain, so nothing was landed",
	"recertification-combines-only-with-direct-fix-register-carriage-so": "--recertification combines only with --direct-fix register-carriage, so nothing was landed",
	"a-recertified-landing-needs-a-fresh-test-receipt-for":               "a recertified landing needs a fresh test receipt for this exact change, so nothing was landed",
	"direct-fix-tier-needs-a-goal-root-job-and":                          "--direct-fix tier-1 needs a goal, --root-job, and --test-receipt or --tests, so nothing was landed",
	"root-job-and-tests-go-only-with-direct-fix":                         "--root-job and --tests go only with --direct-fix tier-1, so nothing was landed",
	"needs-its-full-test-run-s-receipt-so-nothing":                       " needs its full test run's receipt, so nothing was landed",
	"this-checkout-isn-t-on-a-branch-so-nothing":                         "this checkout isn't on a branch, so nothing was landed",
	"the-receipt-record-couldn-t-be-read-so-nothing":                     "the receipt record couldn't be read, so nothing was landed",
	"the-change-deletes-the-receipt-record-so-nothing-was":               "the change deletes the receipt record, so nothing was landed",
	"the-change-touches-code-but-adds-no-receipt-line":                   "the change touches code but adds no receipt line for it, so nothing was landed",
	"so-nothing-was-landed-2":                                            "), so nothing was landed",
	"the-test-receipt-is-for-other-files-than-the":                       "the test receipt is for other files than the staged change, so nothing was landed",
	"recertification-takes-the-record-s-path-inside-the-repository":      "--recertification takes the record's path inside the repository, not an absolute path, so nothing was landed",
	"stopped":                                                          " stopped (",
	"main-moved-while-it-was-being-landed":                             "main moved while it was being landed",
	"the-landing-agent-s-started-at-s-as-the":                          "the landing agent %s started at %s as the lane was paused by %s, and could not be stopped: %v; run: metasystem work stop %s",
	"the-landing-agent-s-changed-its-registered-lane-or":               "the landing agent %s changed its registered lane or selected work while starting and could not be stopped: %v; run: metasystem work stop %s",
	"the-landing-agent-s-started-at-s-as-the-2":                        "the landing agent %s started at %s as the lane was paused by %s, and was stopped again",
	"was-stopped-because-its-registered-lane-or-selected-work":         " was stopped because its registered lane or selected work changed while it started",
	"the-landing-agent-at-s-is-not-started-the":                        "the landing agent at %s is not started: the lane is paused by %s; metasystem landing resume resumes it",
	"the-landing-agent-s-is-running-at-s":                              "the landing agent %s is running at %s",
	"the-folder-that-held-it-can-t-be-reached":                         "the folder that held it can't be reached: %w",
	"so-the-lane-s-members-are-not-known-and":                          "), so the lane's members are not known and nothing was returned",
	"whether-the-lane-changed-after-its-held-runs-is":                  "whether the lane changed after its held runs is unknown: ",
	"nothing-was-landed":                                               "; nothing was landed",
	"runtime-path-s-cannot-be-landed":                                  "runtime path %s cannot be landed",
	"goal-item-s-is-not-held-by-s":                                     "goal item %s is not held by %s",
	"record-s-requires-a-held-goal":                                    "record %s requires a held goal",
	"existing-record-s-requires-a-held-goal":                           "existing record %s requires a held goal",
	"agent-running-no-step-for-d-min":                                  "agent: running, no step for %d min",
	"the-candidate-contains-an-unselected-superseded-returned-or-held": "the candidate contains an unselected, superseded, returned or held hand-in: ",
	"the-candidate-contains-a-superseded-returned-or-held-selected":    "the candidate contains a superseded, returned or held selected hand-in: ",
	"was-not-returned-the-lane-s-last-check-found":                     " was not returned: the lane's last check found ",
	"the-person-returned-the-goal-as":                                  "; the person returned the goal as ",
	"waited-for-the-running-proof":                                     "waited for the running proof",
	"the-lane-is-resolving":                                            "the lane is resolving ",
	"the-restart-could-not-be-made-durable-the-running":                "the restart could not be made durable; the running check continues",
	"more-than-one-lane-fix-is-running":                                "more than one lane fix is running",
	"is-already-returned":                                              " is already returned: ",
	"the-person-returned-the-goal-as-unclassified":                     "; the person returned the goal as unclassified",
	"returned-to-the-seat-rebuild-the-merge-without-this":              "returned to the seat; rebuild the merge without this goal",
	"returned": "returned ",
	"superseded-by-a-newer-hand-in-nothing-returned": "superseded by a newer hand-in; nothing returned",
	"held":                                   "held ",
	"agent-running":                          "agent running",
	"running-proof":                          "running proof",
	"engine-s-stale-t-proofs-v-min-median-s": "engine: %s; stale %t\nproofs: %v min; median %s; running %s min\n",
	"those-failures-stay-open-and-later-goals-are-still": "; those failures stay open and later goals are still held by them; undo with `landing give-back ",
	"proving-it-again":                                            "; proving it again",
	"the-lane-s-check-stopped-before-it-ended":                    "the lane's check stopped before it ended",
	"the-running-check-cannot-be-decoded-w":                       "the running check cannot be decoded: %w",
	"start-proving-tree-s-w":                                      "start proving tree %s: %w",
	"start-proving-tree-s-s-retry-s":                              "start proving tree %s: %s; retry: %s",
	"the-proving-command-s-log-could-not-be-made":                 "the proving command's log could not be made: %w",
	"the-proving-command-exited-d":                                "the proving command exited %d",
	"the-proving-command-ended-w":                                 "the proving command ended: %w",
	"the-running-landing-check-can-t-be-read-w":                   "the running landing check can't be read: %w",
	"the-landing-lane-is-stopped":                                 "the landing lane is stopped",
	"has-landed":                                                  " has landed",
	"the-cancellation-does-not-match-the-running-check":           "the cancellation does not match the running check",
	"the-check-s-processes-have-not-stopped-the-batch":            "the check's processes have not stopped; the batch stays sealed",
	"the-running-check-s-elapsed-time-cannot-be-read":             "the running check's elapsed time cannot be read",
	"the-fix-of-main-is-proving":                                  "the fix of main is proving",
	"nothing-to-resume-the-lane-is-running-batch":                 "nothing to resume: the lane is running batch ",
	"the-running-proof-continues":                                 "the running proof continues",
	"proving-main":                                                "proving main",
	"the-lane-s-pause-record-can-t-be-read":                       "the lane's pause record can't be read, so the lane reads as stopped",
	"the-running-regeneration":                                    "the running regeneration",
	"the-running-proof":                                           "the running proof",
	"what-the-running-proof-holds":                                "what the running proof holds",
	"when-the-queued-work-landed":                                 "when the queued work landed",
	"the-running-proof-log":                                       "the running proof log",
	"landed-with-lane-fix-d":                                      "landed with lane fix %d",
	"proving-main-2":                                              "proving main: ",
	"hour-to-s-d-landed-d-returned-d-red":                         "hour to %s: %d landed, %d returned, %d red (goal %d, main %d, flake %d, unknown %d), oldest goal %g min, %d acts, %d overdue records",
	"closes-when-its-subjects-landed-returned-or-were-superseded": "closes when its subjects landed, returned or were superseded, or its proof attempt is no longer current",
	"closes-when-done-superseded-by-another-proof-or-its":         "closes when done, superseded by another proof, or its merge ownership is settled; an abandoned fix waits until its subject landed, returned or was superseded",
	"leaves-the-views-after-the-day-window-when-landed":           "leaves the views after the day window when landed or returned; superseded submissions leave immediately; queue history stays",
	"the-goal-is-held":                                            "the goal is held: ",
	"d-waiting-d-returned-in-the-queue":                           "%d waiting, %d returned in the queue",
	"held-2":                                                      " (held)",
	"proving-at":                                                  "proving at ",
	"stopped-and":                                                 " stopped, and ",
	"it-stays-stopped":                                            "; it stays stopped",
	"is-already-running":                                          " is already running",
	"a-landing-agent-is-already-running-at":                       "a landing agent is already running at ",
	"the-landing-lane-is-stopped-by":                              "the landing lane is stopped by ",
	"performs-the-person-act-for-this-stopped-subject":            "performs the person act for this stopped subject",
	"the-landing-lane-was-stopped-before-this-operation-started":  "the landing lane was stopped before this operation started",
	"the-landing-lane-couldn-t-be-stopped":                        "the landing lane couldn't be stopped",
	"the-landing-lane-could-not-be-stopped":                       "the landing lane could not be stopped: ",
	"the-lane-is-paused-but-its-person-s-act":                     "the lane is paused, but its person's act could not be recorded",
	"the-running-regeneration-could-not-be-stopped":               "the running regeneration could not be stopped",
	"the-lane-is-paused-but-its-continuation-could-not":           "the lane is paused, but its continuation could not be revoked",
	"the-lane-is-paused-but-its-agent-could-not":                  "the lane is paused, but its agent could not be reached",
	"the-lane-is-paused-but-its-agent-has-not":                    "the lane is paused, but its agent has not ended",
	"stopped-the-landing-lane-for":                                "stopped the landing lane for ",
	"the-landing-lane-is-already-stopped-by":                      "the landing lane is already stopped by ",
	"no-landing-agent-was-running":                                "no landing agent was running",
	"the-lane-s-engine-deploy-stopped":                            "the lane's engine deploy stopped: ",
	"is-already-returned-2":                                       " is already returned:",
	"proving":                                                     "proving ",
	"already-proving":                                             "already proving ",
	"nothing-was-pushed-whether-head-contains-returned-goal":      "nothing was pushed: whether HEAD contains returned goal ",
	"head-still-contains-returned-goal":                           "HEAD still contains returned goal ",
	"could-not-be-returned":                                       " could not be returned: ",
	"was-returned-to-its-seat-nothing-was-pushed":                 " was returned to its seat; nothing was pushed",
	"the-landed-cards-could-not-be-read-from-the":                 "the landed cards could not be read from the queue: ",
	"the-landed-card-for-s-was-not-written-s":                     "the landed card for %s was not written: %s",
	"the-landed-cards-were-not-written":                           "the landed cards were not written: ",
	"nothing-was-pushed-whether-head-contains-held-goal":          "nothing was pushed: whether HEAD contains held goal ",
	"head-contains-held-goal":                                     "HEAD contains held goal ",
	"rebuild-the-batch-by-merging-only-waiting-goals-that":        "rebuild the batch by merging only waiting goals that are not held, then metasystem landing prove",
	"resolving-d-conflicts-of-s":                                  "Resolving %d conflicts of %s",
	"returned-s-s":                                                "returned %s: %s",
	"landing-return-requires-one-goal-and-cause-own-main":         "landing return requires one goal and --cause own, main, other, flake, environment, or unclassified; nothing was returned",
	"waits-in-the-landing-lane-so-nothing-was-returned":           " waits in the landing lane, so nothing was returned",
	"resolution-read-jobs-could-not-be-proved-stopped-s":          "resolution read jobs could not be proved stopped: %s",
	"a-person-stopped-the-lane":                                   "a person stopped the lane",
	"may-land-can-t-be-read-now-so-nothing":                       " may land can't be read now, so nothing was landed",
	"resolving-conflicts-of":                                      "Resolving conflicts of ",
	"the-resolution-read-could-not-be-returned":                   "the resolution read could not be returned",
	"s-at-s-landed-on-main-through-the-landing":                   "%s at %s landed on main through the landing lane; the goal stays open until done",
	"s-at-s-was-returned-s":                                       "%s at %s was returned: %s",
	"shows-whether-it-waits-landed-or-was-returned":               "shows whether it waits, landed or was returned",
	"the-returned-card-for":                                       "the returned card for ",
	"the-landing-stopped-exit":                                    "the landing stopped (exit ",
	"held-unreadable-2":                                           "held: unreadable:",
	"held-nothing-to-push":                                        "held: nothing to push",
	"held-goal-free-ledger":                                       "held: goal-free ledger",
	"held-ok-d-commit-s-above-s":                                  "held: ok %d commit(s) above %s\n",
	"held-refused-s-s-s":                                          "held refused: %s: %s: %s\n",
	"held-unreadable-unknown-verdict":                             "held: unreadable: unknown verdict",
}

func Phrase(key string) string { return Phrases[key] }
