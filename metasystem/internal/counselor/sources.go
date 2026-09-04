package counselor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func loadRecordSet(root string) RecordSet {
	records := RecordSet{}
	governed, invalidGoverned, governedLimitations := loadGovernedRuns(root)
	records.Runs = append(records.Runs, governedObservations(governed)...)
	tracked, trackedLimitations := loadTrackedRuns(root, governed, invalidGoverned)
	records.Runs = append(records.Runs, tracked...)
	records.SpendLimitations = append(records.SpendLimitations, governedLimitations...)
	records.SpendLimitations = append(records.SpendLimitations, trackedLimitations...)

	landings, landingLimitations := loadLandings(root)
	records.Landings = landings
	records.SpendLimitations = append(records.SpendLimitations, landingLimitations...)
	records.ActivityLimitations = append(records.ActivityLimitations, landingLimitations...)

	records.GoalEvents, trackedLimitations = loadGoalEvents(root)
	records.ActivityLimitations = append(records.ActivityLimitations, trackedLimitations...)
	registerEntries, registerLimitations := loadAcceptedRiskRegister(root)
	records.RegisterEntries = registerEntries
	records.RegisterLimitations = append(records.RegisterLimitations, registerLimitations...)
	return records
}

type acceptedRiskRegisterLine struct {
	SchemaVersion    int                                `json:"schemaVersion"`
	ID               string                             `json:"id"`
	RecordedAt       string                             `json:"recordedAt"`
	Kind             string                             `json:"kind"`
	Class            string                             `json:"class"`
	Title            string                             `json:"title"`
	AcceptanceStatus string                             `json:"acceptanceStatus"`
	AcceptanceReason string                             `json:"acceptanceReason"`
	SpecimenFacts    []acceptedRiskRegisterSpecimenFact `json:"specimenFacts"`
	ReviewLinks      []acceptedRiskRegisterReviewLink   `json:"reviewLinks"`
}

type acceptedRiskRegisterSpecimenFact struct {
	Fact      string                         `json:"fact"`
	Citations []acceptedRiskRegisterCitation `json:"citations"`
}

type acceptedRiskRegisterCitation struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Detail string `json:"detail"`
}

type acceptedRiskRegisterReviewLink struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Detail string `json:"detail"`
}

func loadAcceptedRiskRegister(root string) ([]RegisterEntry, []Limitation) {
	var entries []RegisterEntry
	var limitations []Limitation
	for _, source := range []string{acceptedRiskRegisterSource, "records/counselor/misclassification-register.jsonl"} {
		loaded, limited := loadAcceptedRiskRegisterFile(root, source)
		entries = append(entries, loaded...)
		limitations = append(limitations, limited...)
	}
	entries, duplicateLimitations := excludeDuplicateRegisterIDs(entries)
	limitations = append(limitations, duplicateLimitations...)
	sortRegisterEntries(entries)
	return entries, limitations
}

func loadAcceptedRiskRegisterFile(root, source string) ([]RegisterEntry, []Limitation) {
	path := filepath.Join(root, filepath.FromSlash(source))
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, []Limitation{acceptedRiskRegisterLimitation("Accepted-risk register evidence", "The register is absent at "+source+".", "Create the append-only register at "+source+" with one validated JSON object per line.")}
	}
	if err != nil {
		return nil, []Limitation{acceptedRiskRegisterLimitation("Accepted-risk register evidence", "The accepted-risk register could not be read, so accepted risks and near misses are absent.", "Restore readable durable register bytes at "+acceptedRiskRegisterSource+".")}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var entries []RegisterEntry
	malformed := 0
	incomplete := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, outcome := parseAcceptedRiskRegisterLine(line)
		switch outcome {
		case acceptedRiskLineValid:
			entries = append(entries, entry)
		case acceptedRiskLineMalformed:
			malformed++
		case acceptedRiskLineIncomplete:
			incomplete++
		}
	}
	var limitations []Limitation
	if err := scanner.Err(); err != nil {
		limitations = append(limitations, acceptedRiskRegisterLimitation("Accepted-risk register evidence", "The accepted-risk register reader stopped early: "+sanitizeEvidenceDetail(err.Error())+".", "Restore register lines below the scanner limit and keep the file readable."))
	}
	if malformed > 0 || incomplete > 0 {
		var details []string
		if malformed > 0 {
			details = append(details, pluralCount(malformed, "accepted-risk register line was", "accepted-risk register lines were")+" not exactly one JSON object with only accepted register fields and were excluded.")
		}
		if incomplete > 0 {
			details = append(details, pluralCount(incomplete, "accepted-risk register entry failed", "accepted-risk register entries failed")+" schemaVersion 1, non-empty identifier, RFC3339 recordedAt, accepted kind, non-empty class, title, acceptance status, acceptance reason, specimen fact, durable citation, or review link requirements and were excluded.")
		}
		limitations = append(limitations, acceptedRiskRegisterLimitation("Accepted-risk register shape", strings.Join(details, " "), "Rewrite malformed lines as single JSON objects and fill every required field instead of relying on memory or chat context."))
	}
	return entries, limitations
}

type acceptedRiskLineOutcome int

const (
	acceptedRiskLineValid acceptedRiskLineOutcome = iota
	acceptedRiskLineMalformed
	acceptedRiskLineIncomplete
)

func parseAcceptedRiskRegisterLine(line string) (RegisterEntry, acceptedRiskLineOutcome) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	var raw acceptedRiskRegisterLine
	if err := decoder.Decode(&raw); err != nil {
		return RegisterEntry{}, acceptedRiskLineMalformed
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RegisterEntry{}, acceptedRiskLineMalformed
	}
	entry, ok := buildAcceptedRiskRegisterEntry(raw)
	if !ok {
		return RegisterEntry{}, acceptedRiskLineIncomplete
	}
	return entry, acceptedRiskLineValid
}

func buildAcceptedRiskRegisterEntry(raw acceptedRiskRegisterLine) (RegisterEntry, bool) {
	if raw.SchemaVersion != 1 {
		return RegisterEntry{}, false
	}
	id := strings.TrimSpace(raw.ID)
	kind := RegisterEntryKind(strings.TrimSpace(raw.Kind))
	class := strings.TrimSpace(raw.Class)
	title := strings.TrimSpace(raw.Title)
	status := strings.TrimSpace(raw.AcceptanceStatus)
	reason := strings.TrimSpace(raw.AcceptanceReason)
	recordedAt, timeErr := time.Parse(time.RFC3339, strings.TrimSpace(raw.RecordedAt))
	if id == "" || class == "" || title == "" || status == "" || reason == "" || timeErr != nil {
		return RegisterEntry{}, false
	}
	if kind != RegisterAcceptedRisk && kind != RegisterNearMiss && kind != RegisterMisclassification {
		return RegisterEntry{}, false
	}
	if kind == RegisterMisclassification && status != "recorded" || kind != RegisterMisclassification && status == "recorded" {
		return RegisterEntry{}, false
	}
	facts, ok := buildRegisterSpecimenFacts(raw.SpecimenFacts)
	if !ok {
		return RegisterEntry{}, false
	}
	reviewLinks, ok := buildRegisterReviewLinks(raw.ReviewLinks)
	if !ok {
		return RegisterEntry{}, false
	}
	return RegisterEntry{
		ID: id, RecordedAt: recordedAt.UTC(), Kind: kind, Class: class, Title: title,
		AcceptanceStatus: status, AcceptanceReason: reason, SpecimenFacts: facts, ReviewLinks: reviewLinks,
	}, true
}

func buildRegisterSpecimenFacts(rawFacts []acceptedRiskRegisterSpecimenFact) ([]RegisterSpecimenFact, bool) {
	if len(rawFacts) == 0 {
		return nil, false
	}
	facts := make([]RegisterSpecimenFact, 0, len(rawFacts))
	for _, raw := range rawFacts {
		fact := strings.TrimSpace(raw.Fact)
		if fact == "" || len(raw.Citations) == 0 {
			return nil, false
		}
		citations := make([]RegisterCitation, 0, len(raw.Citations))
		for _, rawCitation := range raw.Citations {
			citation := RegisterCitation{
				Kind:   strings.TrimSpace(rawCitation.Kind),
				Target: strings.TrimSpace(rawCitation.Target),
				Detail: strings.TrimSpace(rawCitation.Detail),
			}
			if !validRegisterReferenceKind(citation.Kind) || citation.Target == "" {
				return nil, false
			}
			citations = append(citations, citation)
		}
		facts = append(facts, RegisterSpecimenFact{Fact: fact, Citations: citations})
	}
	return facts, true
}

func buildRegisterReviewLinks(rawLinks []acceptedRiskRegisterReviewLink) ([]RegisterReviewLink, bool) {
	if len(rawLinks) == 0 {
		return nil, false
	}
	links := make([]RegisterReviewLink, 0, len(rawLinks))
	for _, raw := range rawLinks {
		link := RegisterReviewLink{
			Kind:   strings.TrimSpace(raw.Kind),
			Target: strings.TrimSpace(raw.Target),
			Detail: strings.TrimSpace(raw.Detail),
		}
		if !validRegisterReferenceKind(link.Kind) || link.Target == "" {
			return nil, false
		}
		links = append(links, link)
	}
	return links, true
}

func validRegisterReferenceKind(kind string) bool {
	switch kind {
	case "commit", "goal", "ruling", "job", "record", "job-record":
		return true
	default:
		return false
	}
}

func excludeDuplicateRegisterIDs(entries []RegisterEntry) ([]RegisterEntry, []Limitation) {
	counts := make(map[string]int, len(entries))
	for _, entry := range entries {
		counts[entry.ID]++
	}
	duplicates := 0
	for _, count := range counts {
		if count > 1 {
			duplicates++
		}
	}
	if duplicates == 0 {
		return entries, nil
	}
	result := make([]RegisterEntry, 0, len(entries))
	for _, entry := range entries {
		if counts[entry.ID] == 1 {
			result = append(result, entry)
		}
	}
	return result, []Limitation{acceptedRiskRegisterLimitation("Accepted-risk register identity", pluralCount(duplicates, "accepted-risk register identifier appeared", "accepted-risk register identifiers appeared")+" on more than one valid line; every duplicated identifier was excluded rather than silently deduplicated.", "Keep each accepted-risk register identifier globally unique.")}
}

func sortRegisterEntries(entries []RegisterEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Class != entries[j].Class {
			return entries[i].Class < entries[j].Class
		}
		if !entries[i].RecordedAt.Equal(entries[j].RecordedAt) {
			return entries[i].RecordedAt.Before(entries[j].RecordedAt)
		}
		return entries[i].ID < entries[j].ID
	})
}

func sanitizeEvidenceDetail(detail string) string {
	detail = strings.NewReplacer("\r", " ", "\n", " ").Replace(detail)
	return strings.TrimSuffix(strings.TrimSpace(detail), ".")
}

func acceptedRiskRegisterLimitation(name, detail, enrichment string) Limitation {
	return evidenceLimitation(name, detail, enrichment)
}

type governedEvidence struct {
	Observation RunObservation
	Attempt     obligationstate.TerminalAttempt
}

func governedObservations(evidence map[string]governedEvidence) []RunObservation {
	ids := make([]string, 0, len(evidence))
	for id := range evidence {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	observations := make([]RunObservation, 0, len(ids))
	for _, id := range ids {
		observations = append(observations, evidence[id].Observation)
	}
	return observations
}

func loadGovernedRuns(root string) (map[string]governedEvidence, map[string]bool, []Limitation) {
	paths, err := filepath.Glob(filepath.Join(obligationstate.Dir(root), "*.json"))
	if err != nil {
		return map[string]governedEvidence{}, map[string]bool{}, []Limitation{evidenceLimitation("Governed-run evidence", "The governed-obligation directory could not be listed, so governed spend is absent.", "Make the governed-obligation directory readable and retain its validated state files.")}
	}
	sort.Strings(paths)
	candidates := map[string][]governedEvidence{}
	rejected := 0
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			rejected++
			continue
		}
		var header obligationstate.State
		if json.Unmarshal(data, &header) != nil || header.GoalID == "" || header.GoalRevision == 0 || header.ObligationRevision == 0 {
			rejected++
			continue
		}
		if filepath.Clean(obligationstate.Path(root, header.GoalID, header.GoalRevision, header.ObligationRevision)) != filepath.Clean(path) {
			rejected++
			continue
		}
		state, found, loadErr := obligationstate.Load(root, header.GoalID, header.GoalRevision, header.ObligationRevision)
		if loadErr != nil || !found {
			rejected++
			continue
		}
		for _, attempt := range state.Attempts {
			outcome, ok := asOutcome(attempt.Status)
			ended, timeErr := time.Parse(time.RFC3339, attempt.EndedAt)
			if !ok || timeErr != nil || attempt.ObservedCostMinutes > uint64(math.MaxInt64/int64(time.Minute)) {
				rejected++
				continue
			}
			candidates[attempt.RunID] = append(candidates[attempt.RunID], governedEvidence{
				Observation: RunObservation{
					ID: attempt.RunID, Kind: RunGoverned, CompletedAt: ended.UTC(),
					Duration: time.Duration(attempt.ObservedCostMinutes) * time.Minute, Outcome: outcome,
				},
				Attempt: attempt,
			})
		}
	}

	result := map[string]governedEvidence{}
	invalid := map[string]bool{}
	duplicates := 0
	for id, attempts := range candidates {
		if len(attempts) != 1 {
			duplicates++
			invalid[id] = true
			continue
		}
		result[id] = attempts[0]
	}
	var limitations []Limitation
	if rejected > 0 {
		limitations = append(limitations, evidenceLimitation("Governed-run evidence", fmt.Sprintf("%d governed-obligation records or attempts were unreadable and were excluded from spend.", rejected), "Repair the durable obligation state so its strict reader accepts every record."))
	}
	if duplicates > 0 {
		limitations = append(limitations, evidenceLimitation("Governed-run identity", fmt.Sprintf("%d run identifiers appeared in more than one governed-obligation state and were excluded rather than silently deduplicated.", duplicates), "Restore one authoritative governed-obligation owner for each run identifier."))
	}
	return result, invalid, limitations
}

func loadTrackedRuns(root string, governed map[string]governedEvidence, invalidGoverned map[string]bool) ([]RunObservation, []Limitation) {
	records, unreadable := (&run.Store{Root: root}).List()
	observations := make([]RunObservation, 0, len(records))
	active := 0
	missingGoverned := 0
	contradictions := 0
	launchFailuresWithoutEnd := 0
	malformedEndStamps := 0
	for index := range records {
		record := &records[index]
		if !run.Terminal(record.Status) {
			active++
			continue
		}
		started, startedErr := time.Parse(time.RFC3339, record.StartedAt)
		outcome, outcomeOK := asOutcome(record.Status)
		ended, endState := parseRunEnd(record)
		if endState == runEndMalformed {
			malformedEndStamps++
		}
		launchFailedWithoutEnd := startedErr == nil && record.Status == run.StatusLaunchFailed && endState == runEndAbsent
		if launchFailedWithoutEnd {
			ended = started.UTC()
		}
		if startedErr != nil || !outcomeOK || (!launchFailedWithoutEnd && endState != runEndParsed) || ended.Before(started) {
			unreadable = append(unreadable, run.RecordPath(root, record.RunId)+": terminal timing or outcome is unusable")
			continue
		}
		if durable, ok := governed[record.RunId]; ok {
			if terminalContradiction(record, durable.Attempt) {
				contradictions++
			}
			continue
		}
		if invalidGoverned[record.RunId] {
			continue
		}
		if launchFailedWithoutEnd {
			launchFailuresWithoutEnd++
		}
		if record.Governed != nil {
			missingGoverned++
			duration := ended.Sub(started)
			if record.Governed.ObservedCostMinutes != nil && *record.Governed.ObservedCostMinutes <= uint64(math.MaxInt64/int64(time.Minute)) {
				duration = time.Duration(*record.Governed.ObservedCostMinutes) * time.Minute
			}
			observations = append(observations, RunObservation{ID: record.RunId, Kind: RunGoverned, CompletedAt: ended, Duration: duration, Outcome: outcome})
			continue
		}
		observations = append(observations, RunObservation{ID: record.RunId, Kind: RunTracked, CompletedAt: ended, Duration: ended.Sub(started), Outcome: outcome})
	}
	var limitations []Limitation
	if len(unreadable) > 0 {
		limitations = append(limitations, evidenceLimitation("Tracked-run evidence", fmt.Sprintf("%d retained run records were unreadable and were excluded from spend.", len(unreadable)), "Repair or restore the retained run records so the run store validates them."))
	}
	if malformedEndStamps > 0 {
		detail := fmt.Sprintf("%d terminal run records carried unparsable endedAt timestamps and were excluded instead of being treated as missing end timestamps.", malformedEndStamps)
		if malformedEndStamps == 1 {
			detail = "1 terminal run record carried an unparsable endedAt timestamp and was excluded instead of being treated as a missing end timestamp."
		}
		limitations = append(limitations, evidenceLimitation("Run-end timestamp shape", detail, "Rewrite endedAt as RFC3339, or omit it only for launch-failed records whose failure timestamp is truly absent."))
	}
	if active > 0 {
		limitations = append(limitations, evidenceLimitation("Active-run boundary", fmt.Sprintf("%d active runs have no completed duration or terminal outcome and are excluded until they conclude.", active), "Record a distinct in-progress spend observation if partial-run cost is needed."))
	}
	if missingGoverned > 0 {
		limitations = append(limitations, evidenceLimitation("Governed-run durability", fmt.Sprintf("%d terminal governed runs lacked their durable obligation state; their retained run-record cost is included as fallback evidence.", missingGoverned), "Restore the matching governed-obligation terminal attempts."))
	}
	if contradictions > 0 {
		limitations = append(limitations, evidenceLimitation("Governed-run reconciliation", fmt.Sprintf("%d retained run records contradicted their durable governed-obligation owner; only the obligation-state facts were counted.", contradictions), "Reconcile the retained run copy with the authoritative terminal attempt."))
	}
	if launchFailuresWithoutEnd > 0 {
		limitations = append(limitations, evidenceLimitation("Launch-failed timing", pluralCount(launchFailuresWithoutEnd, "valid launch-failed run record carried", "valid launch-failed run records carried")+" no end timestamp; each outcome is counted in its start-time window with zero completed duration.", "Record the failure timestamp as endedAt on every launch-failed run."))
	}
	return observations, limitations
}

type runEndState int

const (
	runEndParsed runEndState = iota
	runEndAbsent
	runEndMalformed
)

func parseRunEnd(record *run.Record) (time.Time, runEndState) {
	if record.EndedAt == nil {
		return time.Time{}, runEndAbsent
	}
	stamp, err := time.Parse(time.RFC3339, *record.EndedAt)
	if err != nil {
		return time.Time{}, runEndMalformed
	}
	return stamp.UTC(), runEndParsed
}

func terminalContradiction(record *run.Record, attempt obligationstate.TerminalAttempt) bool {
	if record.Governed == nil || record.EndedAt == nil || record.Governed.ObservedCostMinutes == nil {
		return true
	}
	return record.Status != attempt.Status || record.StartedAt != attempt.StartedAt || *record.EndedAt != attempt.EndedAt ||
		*record.Governed.ObservedCostMinutes != attempt.ObservedCostMinutes
}

func asOutcome(status string) (RunOutcome, bool) {
	switch status {
	case run.StatusGreen:
		return OutcomeGreen, true
	case run.StatusRed:
		return OutcomeRed, true
	case run.StatusEndedUnknown:
		return OutcomeEndedUnknown, true
	case run.StatusLaunchFailed:
		return OutcomeLaunchFailed, true
	default:
		return "", false
	}
}

func loadGoalEvents(root string) ([]GoalEventObservation, []Limitation) {
	files, err := goal.ReadCommitGoals(root, "HEAD")
	if err != nil {
		return nil, []Limitation{evidenceLimitation("Goal-history evidence", "The current branch's goal-history files could not be read, so goal events are absent.", "Restore readable committed goal-history files on the current branch.")}
	}
	tree, problems := goal.ParseTreeFiles(files)
	type eventCandidate struct {
		at       time.Time
		verb     string
		class    GoalVerbClass
		mapped   bool
		conflict bool
	}
	candidates := map[string]eventCandidate{}
	addHistory := func(history []goal.HistoryLine) {
		for _, line := range history {
			stamp, parseErr := time.Parse(time.RFC3339, line.At)
			if parseErr != nil {
				problems = append(problems, goal.Problem("history timestamp is invalid"))
				continue
			}
			class, mapped := goalVerbClass(line.Verb)
			candidate := eventCandidate{at: stamp.UTC(), verb: line.Verb, class: class, mapped: mapped}
			if previous, exists := candidates[line.Opid]; exists {
				if previous.at.Equal(candidate.at) && previous.verb == candidate.verb {
					continue
				}
				previous.conflict = true
				candidates[line.Opid] = previous
				continue
			}
			candidates[line.Opid] = candidate
		}
	}
	if tree != nil {
		if tree.Root != nil {
			addHistory(tree.Root.History)
		}
		for _, file := range tree.Live {
			addHistory(file.History)
		}
		for _, file := range tree.Done {
			addHistory(file.History)
		}
	}
	events := make([]GoalEventObservation, 0, len(candidates))
	unmapped := 0
	conflicts := 0
	for operationID, candidate := range candidates {
		switch {
		case candidate.conflict:
			conflicts++
		case !candidate.mapped:
			unmapped++
		default:
			events = append(events, GoalEventObservation{OperationID: operationID, At: candidate.at, Class: candidate.class})
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].At.Equal(events[j].At) {
			return events[i].OperationID < events[j].OperationID
		}
		return events[i].At.Before(events[j].At)
	})
	var limitations []Limitation
	if len(problems) > 0 {
		limitations = append(limitations, evidenceLimitation("Goal-history evidence", fmt.Sprintf("The committed goal tree reported %d structural problems; only history entries from parsed records were considered.", len(problems)), "Repair the committed goal tree until its strict tree parser reports no problems."))
	}
	if unmapped > 0 {
		limitations = append(limitations, evidenceLimitation("Goal-event exclusions", fmt.Sprintf("%d unique retained goal operations use verbs outside the requested exact mapping and were excluded from the ratio.", unmapped), "Record a closed activity class on every goal operation."))
	}
	if conflicts > 0 {
		limitations = append(limitations, evidenceLimitation("Goal-event identity", fmt.Sprintf("%d operation identifiers carried conflicting verb or timestamp facts and were excluded rather than silently resolved.", conflicts), "Repair each operation identifier to one verb and timestamp across the goal tree."))
	}
	return events, limitations
}

func goalVerbClass(verb string) (GoalVerbClass, bool) {
	switch verb {
	case "open":
		return GoalOpen, true
	case "edit":
		return GoalEdit, true
	case "claim":
		return GoalClaim, true
	case "set-budget":
		return GoalBudget, true
	case "done":
		return GoalDone, true
	default:
		return "", false
	}
}

func loadLandings(root string) ([]LandingObservation, []Limitation) {
	workspace := gittree.Workspace{Dir: root}
	top, err := workspace.TopLevel()
	if err != nil {
		return nil, []Limitation{gitEvidenceUnavailable(gitResolutionFailure("repository top-level resolution", err))}
	}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, []Limitation{gitEvidenceUnavailable(gitResolutionFailure("checkout prefix resolution", err))}
	}
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return nil, []Limitation{gitEvidenceUnavailable(gitResolutionFailure("current branch tip resolution", err))}
	}
	if unborn {
		return nil, nil
	}
	args := []string{"-C", top, "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false",
		"log", "--topo-order", "--reverse", "--format=%x1e%H%x1f%cI%x1f%(trailers:key=Goal-Transaction,valueonly,separator=%x1d)", "--numstat", "--no-renames", head, "--"}
	if prefix != "" {
		args = append(args, strings.TrimSuffix(prefix, "/"))
	}
	command := exec.Command("git", args...)
	command.Env = gittree.ScrubbedEnviron()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := boundedexec.Run(command, boundedexec.Timeout(filepath.Join(root, "metasystem.conf"), boundedexec.Local), "counselor git history"); err != nil {
		return nil, []Limitation{gitEvidenceUnavailable("the bounded Git history read failed: " + err.Error())}
	}
	landings, rejected := parseGitLog(stdout.String(), prefix)
	if rejected == 0 {
		return landings, nil
	}
	return landings, []Limitation{evidenceLimitation("Git landing evidence", fmt.Sprintf("%d current-branch commit or numstat records were malformed and were excluded from landing facts.", rejected), "Repair the Git history metadata or provide a durable typed landing record.")}
}

func parseGitLog(log, prefix string) ([]LandingObservation, int) {
	var landings []LandingObservation
	rejected := 0
	for _, rawBlock := range strings.Split(log, "\x1e") {
		block := strings.TrimLeft(rawBlock, "\r\n")
		if strings.TrimSpace(block) == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		header := strings.Split(lines[0], "\x1f")
		if len(header) != 3 {
			rejected++
			continue
		}
		stamp, err := time.Parse(time.RFC3339, strings.TrimSpace(header[1]))
		if err != nil {
			rejected++
			continue
		}
		goalOperationID := strings.TrimSpace(header[2])
		if strings.Contains(goalOperationID, "\x1d") {
			rejected++
			continue
		}
		landing := LandingObservation{Commit: strings.TrimSpace(header[0]), CompletedAt: stamp.UTC(), GoalOperationID: goalOperationID}
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) < 3 {
				rejected++
				continue
			}
			path := filepath.ToSlash(fields[len(fields)-1])
			if prefix != "" {
				if !strings.HasPrefix(path, prefix) {
					rejected++
					continue
				}
				path = strings.TrimPrefix(path, prefix)
			}
			landing.Files++
			landing.Paths = append(landing.Paths, path)
			insertions, parseErr := strconv.Atoi(fields[0])
			if parseErr != nil {
				if fields[0] == "-" {
					landing.BinaryFiles++
				} else {
					rejected++
				}
				continue
			}
			if insertions < 0 || landing.Insertions > math.MaxInt-insertions {
				rejected++
				continue
			}
			landing.Insertions += insertions
		}
		landings = append(landings, landing)
	}
	return landings, rejected
}

func gitResolutionFailure(scope string, err error) string {
	detail := sanitizeEvidenceDetail(err.Error())
	if strings.Contains(detail, "not a git repository") {
		return scope + " failed because the supplied root is not inside a Git worktree"
	}
	if detail == "" {
		return scope + " failed"
	}
	return scope + " failed: " + detail
}

func gitEvidenceUnavailable(reason string) Limitation {
	reason = strings.NewReplacer("\r", " ", "\n", " ").Replace(reason)
	reason = strings.TrimSuffix(strings.TrimSpace(reason), ".")
	return evidenceLimitation("Git landing evidence", "The current branch's Git history could not be read, so landing counts, sizes, and path classes are absent; "+reason+".", "Restore readable current-branch Git history or provide a durable typed landing record.")
}

func evidenceLimitation(name, detail, enrichment string) Limitation {
	return Limitation{Name: name, Detail: detail, Enrichment: enrichment}
}
