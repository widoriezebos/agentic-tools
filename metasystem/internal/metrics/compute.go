package metrics

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func computeRows(w world, period Period, goalID string, limits thresholds) []metricRow {
	return []metricRow{
		computeOverhead(w, period, goalID, limits),
		computeStaleChecks(w, period, limits),
		computeRework(w, period, goalID, limits),
		computeFriction(w, period, goalID),
		computeWaiting(w, period, goalID, limits),
		computeDebt(w, period, goalID, limits),
		computeDelegates(w, period, goalID, limits),
		computeCollisions(w, period, goalID, limits),
		computeCost(w, period, goalID),
	}
}

func withUsable(base Coverage, usable int) Coverage {
	result := base
	result.Usable = usable
	return result
}

func metric(name, key, scope, action, owner string) metricRow {
	return metricRow{Key: key, Name: name, Scope: scope, Action: action, Owner: owner}
}

type attributionCounts struct {
	Attributed   int
	Total        int
	Unattributed int
	Rejected     int
}

func attributedCoverage(base Coverage, usable int, goalID string, counts attributionCounts) Coverage {
	coverage := withUsable(base, usable)
	coverage.Extra = fmt.Sprintf("goal=%s attributed=%d total=%d", goalID, counts.Attributed, counts.Total)
	return coverage
}

func unattributedBucket(source string, count int) detail {
	return detail{Text: fmt.Sprintf("attribution source=%s bucket=UNATTRIBUTED records=%d", source, count)}
}

func rejectedAttributionBucket(source string, count int) detail {
	return detail{Text: fmt.Sprintf("attribution source=%s bucket=REJECTED records=%d", source, count)}
}

func receiptAttribution(w world, period Period, goalID string) attributionCounts {
	var counts attributionCounts
	for _, receipt := range w.Receipts {
		if goalID == "" && !period.contains(receipt.At) {
			continue
		}
		counts.Total++
		if receipt.InvalidGoal || receipt.InvalidBuiltBy {
			counts.Rejected++
		} else if receipt.Fields["goal"] == goalID && goalID != "" {
			counts.Attributed++
		} else if receipt.Fields["goal"] == "" {
			counts.Unattributed++
		}
	}
	return counts
}

func jobAttribution(w world, period Period, goalID string) attributionCounts {
	var counts attributionCounts
	for _, job := range w.Jobs {
		if job.EndedAt.IsZero() || !dispatch.TerminalStatus(job.Status) {
			continue
		}
		if goalID == "" {
			if !period.contains(job.EndedAt) {
				continue
			}
		}
		counts.Total++
		if job.GoalID == goalID && goalID != "" {
			counts.Attributed++
		}
		if job.GoalID == "" {
			counts.Unattributed++
		}
	}
	return counts
}

func landingAttribution(w world, period Period, goalID string) attributionCounts {
	var counts attributionCounts
	for _, landing := range w.Landings {
		if goalID == "" {
			if !period.contains(landing.At) {
				continue
			}
		}
		counts.Total++
		if landing.Goals[goalID] && goalID != "" {
			counts.Attributed++
		}
		if len(landing.Goals) == 0 {
			counts.Unattributed++
		}
	}
	return counts
}

func critiqueAttribution(w world, goalID string) attributionCounts {
	counts := attributionCounts{Total: len(w.Critiques)}
	for _, chain := range w.Critiques {
		if chain.GoalID == goalID && goalID != "" {
			counts.Attributed++
		}
		if chain.GoalID == "" {
			counts.Unattributed++
		}
	}
	return counts
}

func formatFloat(value float64) string {
	if math.Abs(value) < 0.0000005 {
		value = 0
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func formatHours(duration time.Duration) string {
	return formatFloat(duration.Hours())
}

func historyTime(file *goal.GoalFile, verb string) (time.Time, bool) {
	for index := len(file.History) - 1; index >= 0; index-- {
		if file.History[index].Verb != verb {
			continue
		}
		stamp, err := time.Parse(time.RFC3339, file.History[index].At)
		if err == nil {
			return stamp.UTC(), true
		}
	}
	return time.Time{}, false
}

func goalBounds(record goalRecord) (time.Time, time.Time, bool) {
	opened, err := time.Parse(time.RFC3339, record.File.OpenedAt)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	done, ok := historyTime(record.File, "done")
	return opened.UTC(), done, ok
}

func selectedGoals(w world, period Period, goalID string) []goalRecord {
	if goalID != "" {
		if record, ok := w.Goals[goalID]; ok {
			return []goalRecord{record}
		}
		return nil
	}
	var selected []goalRecord
	ids := make([]string, 0, len(w.Goals))
	for id := range w.Goals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := w.Goals[id]
		if record.File.State != goal.StateDone {
			continue
		}
		if done, ok := historyTime(record.File, "done"); ok && period.contains(done) {
			selected = append(selected, record)
		}
	}
	return selected
}

func computeOverhead(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Overhead ratio", "overhead_ratio", "this-machine", "draft naming the goal and whether process spend or review density is high or low", "coordinator")
	goals := selectedGoals(w, period, goalID)
	var values []string
	var spendJudgment, densityJudgment *float64
	goalCoverage := withUsable(w.GoalCoverage, len(goals))
	row.Coverage = append(row.Coverage, goalCoverage)
	for _, record := range goals {
		id := record.File.Id
		_, _, complete := goalBounds(record)
		if !complete || record.File.State != goal.StateDone {
			values = append(values, id+" unavailable (goal is not concluded with a done history row)")
			continue
		}
		var wall time.Duration
		timedJobs := 0
		criticRounds := map[string]int{}
		for _, job := range w.Jobs {
			if job.GoalID != id || job.EndedAt.IsZero() || !dispatch.TerminalStatus(job.Status) {
				continue
			}
			if job.TimingError == "" {
				timedJobs++
				wall += job.EndedAt.Sub(job.StartedAt)
			}
			if job.Role == "design-critic" || job.Role == "code-critic" {
				root := emptyAs(job.RootJob, job.JobID)
				if job.Round > criticRounds[root] {
					criticRounds[root] = job.Round
				}
			}
		}
		jobCounts := jobAttribution(w, period, id)
		jobCoverage := attributedCoverage(w.JobCoverage, timedJobs, id, jobCounts)
		row.Coverage = append(row.Coverage, jobCoverage)
		row.Details = append(row.Details, unattributedBucket("jobs", jobCounts.Unattributed))

		critiqueUsable := 0
		for _, chain := range w.Critiques {
			if chain.GoalID != id {
				continue
			}
			critiqueUsable++
			if chain.Rounds > criticRounds[chain.Name] {
				criticRounds[chain.Name] = chain.Rounds
			}
		}
		critiqueCounts := critiqueAttribution(w, id)
		critiqueCoverage := attributedCoverage(w.CritiqueCoverage, critiqueUsable, id, critiqueCounts)
		rounds := 0
		for _, count := range criticRounds {
			if count > 1 {
				rounds += count - 1
			}
		}
		corrections := 0
		receiptUsable := 0
		receiptCounts := receiptAttribution(w, period, id)
		for _, receipt := range w.Receipts {
			if !receiptInScope(receipt, period, id) {
				continue
			}
			value, err := strconv.Atoi(receipt.Fields["corrections"])
			if err == nil && value >= 0 {
				corrections += value
				receiptUsable++
			}
		}
		payload := 0
		shared := 0
		landingUsable := 0
		for _, commit := range w.Landings {
			if !commit.Goals[id] {
				continue
			}
			landingUsable++
			payload += commit.ChangedLines
			if commit.Shared {
				shared++
			}
		}
		landingCounts := landingAttribution(w, period, id)
		receiptCoverage := attributedCoverage(w.ReceiptCoverage, receiptUsable, id, receiptCounts)
		landingCoverage := attributedCoverage(w.LandingCoverage, landingUsable, id, landingCounts)
		row.Coverage = append(row.Coverage, landingCoverage, receiptCoverage, critiqueCoverage)
		row.Details = append(row.Details,
			unattributedBucket("landings", landingCounts.Unattributed),
			unattributedBucket("receipts", receiptCounts.Unattributed),
			unattributedBucket("critique-chains", critiqueCounts.Unattributed),
		)
		if receiptCounts.Rejected > 0 {
			row.Details = append(row.Details, rejectedAttributionBucket("receipts", receiptCounts.Rejected))
		}
		budget := record.File.Budget
		parts := []string{id}
		if budget == nil {
			row.Details = append(row.Details, detail{Text: "no structured elapsed budget: goal=" + id})
		}
		if timedJobs == 0 {
			parts = append(parts, "wall_hours=unavailable", "spend=unavailable (no timed attributed jobs)")
			row.Details = append(row.Details, detail{Text: "no timed attributed jobs: goal=" + id})
		} else if budget != nil {
			parts = append(parts, "wall_hours="+formatHours(wall))
			spend := wall.Hours() / budget.ElapsedDuration().Hours()
			parts = append(parts, "spend="+formatFloat(spend))
			if spendJudgment == nil || spend < limits.Spend.Min || spend > limits.Spend.Max {
				copy := spend
				spendJudgment = &copy
			}
		} else {
			parts = append(parts, "wall_hours="+formatHours(wall), "spend=unavailable")
		}
		if payload == 0 {
			parts = append(parts, "density=unavailable (no landed lines)")
		} else {
			density := float64(rounds+corrections) * 100 / float64(payload)
			parts = append(parts, "density="+formatFloat(density), fmt.Sprintf("critique_rounds=%d corrections=%d landed_lines=%d", rounds, corrections, payload))
			if densityJudgment == nil || density < limits.Density.Min || density > limits.Density.Max {
				copy := density
				densityJudgment = &copy
			}
		}
		if shared > 0 {
			parts = append(parts, fmt.Sprintf("shared_commits=%d", shared))
		}
		values = append(values, strings.Join(parts, " "))
	}
	for _, chain := range w.Critiques {
		if chain.GoalID == "" {
			row.Details = append(row.Details, detail{Text: "critique chain unattributed: " + chain.Name, MachineOnly: true})
		}
	}
	if len(values) == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	if len(goals) == 0 {
		jobCounts := jobAttribution(w, period, "")
		landingCounts := landingAttribution(w, period, "")
		receiptCounts := receiptAttribution(w, period, "")
		critiqueCounts := critiqueAttribution(w, "")
		row.Coverage = append(row.Coverage,
			withUsable(w.JobCoverage, 0), withUsable(w.LandingCoverage, 0),
			withUsable(w.ReceiptCoverage, 0), withUsable(w.CritiqueCoverage, 0),
		)
		row.Details = append(row.Details,
			unattributedBucket("jobs", jobCounts.Unattributed),
			unattributedBucket("landings", landingCounts.Unattributed),
			unattributedBucket("receipts", receiptCounts.Unattributed),
			unattributedBucket("critique-chains", critiqueCounts.Unattributed),
		)
		if receiptCounts.Rejected > 0 {
			row.Details = append(row.Details, rejectedAttributionBucket("receipts", receiptCounts.Rejected))
		}
	}
	row.Thresholds = []string{limits.Spend.judgment("spend", spendJudgment), limits.Density.judgment("density", densityJudgment)}
	row.Details = append(row.Details, joinedDetails(row.Coverage...)...)
	return row
}

func computeStaleChecks(w world, period Period, limits thresholds) metricRow {
	row := metric("Stale checks", "stale_checks", "this-machine", "run the stale proof surface or drop it from green's meaning", "steward")
	latest := map[string]proofRecord{}
	variants := map[string]bool{}
	usable := 0
	for _, proof := range w.Proofs {
		if proof.At.After(period.Instant) {
			continue
		}
		usable++
		if proof.Verdict != "green" {
			variants[proof.Surface+"="+proof.Verdict] = true
			continue
		}
		if standing, ok := latest[proof.Surface]; !ok || proof.At.After(standing.At) {
			latest[proof.Surface] = proof
		}
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	crossed := false
	var values []string
	for _, surface := range keys {
		proof := latest[surface]
		age := period.Instant.Sub(proof.At).Hours() / 24
		if age < 0 {
			age = 0
		}
		values = append(values, surface+" days_since_green="+formatFloat(age))
		if limits.Stale.Invalid == "" && age > float64(limits.Stale.Value) {
			crossed = true
		}
		if proof.Fallback {
			row.Details = append(row.Details, detail{Text: "milestone-battery uses labelled envelope mtime fallback: " + proof.Path, MachineOnly: true})
		}
	}
	for _, variant := range sortedStrings(variants) {
		row.Details = append(row.Details, detail{Text: "proof verdict variant: " + variant})
	}
	if len(values) == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	coverage := withUsable(w.ProofCoverage, usable)
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{limits.Stale.judgment("days since green", crossed, len(values) > 0)}
	row.Details = append(row.Details, detail{Text: "no per-leg run-history record exists"})
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}

func receiptInScope(receipt *receiptRecord, period Period, goalID string) bool {
	if receipt.InvalidGoal || receipt.InvalidBuiltBy {
		return false
	}
	if goalID != "" {
		return receipt.Fields["goal"] == goalID
	}
	return period.contains(receipt.At)
}

func journalInScope(record journalRecord, period Period, goalID string) bool {
	if goalID == "" {
		return period.contains(record.TerminalAt)
	}
	return containsTarget(record.Targets, goalID)
}

func computeRework(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Rework rate", "rework_rate", "fleet-synced", "fix the brief or the diagnosis", "coordinator")
	coverage := w.ReceiptCoverage
	items, corrected, maximum := 0, 0, 0
	for _, receipt := range w.Receipts {
		if !receiptInScope(receipt, period, goalID) {
			continue
		}
		value, err := strconv.Atoi(receipt.Fields["corrections"])
		if err != nil || value < 0 {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d invalid effective corrections", receipt.Line))
			continue
		}
		items++
		if value > 0 {
			corrected++
		}
		if value > maximum {
			maximum = value
		}
	}
	coverage.Usable = items
	var share *float64
	if items == 0 {
		row.Value = "unavailable"
	} else {
		value := float64(corrected) / float64(items)
		share = &value
		row.Value = fmt.Sprintf("corrected_items=%d receipted_items=%d share=%s max_corrections=%d", corrected, items, formatFloat(value), maximum)
	}
	counts := receiptAttribution(w, period, goalID)
	if goalID != "" {
		coverage = attributedCoverage(coverage, items, goalID, counts)
	}
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{
		limits.ReworkItem.judgment("corrections per item", maximum > int(limits.ReworkItem.Value), items > 0),
		limits.ReworkRate.judgment("corrected item share", share),
	}
	row.Details = append(row.Details, unattributedBucket("receipts", counts.Unattributed))
	if counts.Rejected > 0 {
		row.Details = append(row.Details, rejectedAttributionBucket("receipts", counts.Rejected))
	}
	row.Details = append(row.Details, detail{Text: "critique rounds are this-machine detail and are not summed into rework", MachineOnly: true})
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}

func computeFriction(w world, period Period, goalID string) metricRow {
	row := metric("Friction rate", "friction_rate", "this-machine context-only", "classify refusals at retro and build the missing surface when a class repeats", "coordinator today; counselor when built")
	type counts struct{ rejected, terminal int }
	byVerb := map[string]counts{}
	usable := 0
	for _, record := range w.Journals {
		if !journalInScope(record, period, goalID) {
			continue
		}
		usable++
		count := byVerb[record.Verb]
		count.terminal++
		if record.Outcome == "rejected" {
			count.rejected++
			row.Details = append(row.Details, detail{Text: fmt.Sprintf("journal=%s verb=%s evidence=%s", filepath.Base(record.Path), record.Verb, cleanLine(record.Evidence)), MachineOnly: true})
		}
		byVerb[record.Verb] = count
	}
	verbs := make([]string, 0, len(byVerb))
	for verb := range byVerb {
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	var values []string
	for _, verb := range verbs {
		count := byVerb[verb]
		values = append(values, fmt.Sprintf("verb=%s rejected=%d terminal=%d rate=%s", verb, count.rejected, count.terminal, formatFloat(float64(count.rejected)/float64(count.terminal))))
	}
	if len(values) == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	coverage := withUsable(w.JournalCoverage, usable)
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{"context-only"}
	row.Details = append(row.Details, detail{Text: "classification is a human read; no classification surface exists"})
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}

func concludingEpoch(file *goal.GoalFile) (claim time.Time, done time.Time, epochs int, ok bool) {
	doneIndex := -1
	for index := len(file.History) - 1; index >= 0; index-- {
		if file.History[index].Verb == "done" {
			doneIndex = index
			break
		}
	}
	if doneIndex < 0 {
		return time.Time{}, time.Time{}, 0, false
	}
	done, err := time.Parse(time.RFC3339, file.History[doneIndex].At)
	if err != nil {
		return time.Time{}, time.Time{}, 0, false
	}
	claimIndex := -1
	for index := 0; index < doneIndex; index++ {
		if file.History[index].Verb == "claim" || file.History[index].Verb == "steal" {
			epochs++
			claimIndex = index
		}
	}
	if claimIndex < 0 {
		return time.Time{}, done.UTC(), epochs, false
	}
	claim, err = time.Parse(time.RFC3339, file.History[claimIndex].At)
	if err != nil {
		return time.Time{}, done.UTC(), epochs, false
	}
	return claim.UTC(), done.UTC(), epochs, true
}

func computeWaiting(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Time waiting on checks", "time_waiting_on_checks", "fleet-synced", "draft naming the slowest recorded whole-battery proof surface", "coordinator")
	goals := selectedGoals(w, period, goalID)
	usableGoals, usableLandings := 0, 0
	var values []string
	var judgment *float64
	for _, record := range goals {
		claim, done, epochs, ok := concludingEpoch(record.File)
		if !ok {
			row.Details = append(row.Details, detail{Text: fmt.Sprintf("lifecycle incomplete: goal=%s epochs=%d", record.File.Id, epochs)})
			continue
		}
		var last time.Time
		for _, commit := range w.Landings {
			if !commit.Goals[record.File.Id] || commit.At.Before(claim) || commit.At.After(done) {
				continue
			}
			usableLandings++
			if commit.At.After(last) {
				last = commit.At
			}
		}
		if last.IsZero() {
			row.Details = append(row.Details, detail{Text: fmt.Sprintf("lifecycle incomplete: goal=%s epochs=%d no attributable landing", record.File.Id, epochs)})
			continue
		}
		usableGoals++
		building := last.Sub(claim)
		proving := done.Sub(last)
		total := done.Sub(claim)
		if total == 0 {
			values = append(values, fmt.Sprintf("%s building_hours=%s proving_hours=%s waiting_share=unavailable (zero-duration lifecycle) epochs=%d", record.File.Id, formatHours(building), formatHours(proving), epochs))
			row.Details = append(row.Details, detail{Text: "degenerate lifecycle: goal=" + record.File.Id + " claim, landing, and done share one timestamp"})
			continue
		}
		share := proving.Seconds() / total.Seconds()
		values = append(values, fmt.Sprintf("%s building_hours=%s proving_hours=%s waiting_share=%s epochs=%d", record.File.Id, formatHours(building), formatHours(proving), formatFloat(share), epochs))
		var batteryWall time.Duration
		batteryRuns := 0
		for _, proof := range w.Proofs {
			if proof.Surface != "milestone-battery" || proof.StartedAt.IsZero() || proof.At.Before(claim) || proof.At.After(done) {
				continue
			}
			batteryRuns++
			batteryWall += proof.At.Sub(proof.StartedAt)
		}
		if batteryRuns == 0 {
			row.Details = append(row.Details, detail{Text: "goal=" + record.File.Id + " whole-battery wall time unavailable", MachineOnly: true})
		} else {
			row.Details = append(row.Details, detail{Text: fmt.Sprintf("goal=%s whole_battery_wall_hours=%s records=%d", record.File.Id, formatHours(batteryWall), batteryRuns), MachineOnly: true})
		}
		if judgment == nil || share > limits.Waiting.Value {
			copy := share
			judgment = &copy
		}
	}
	if len(values) == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	goalCoverage := withUsable(w.GoalCoverage, usableGoals)
	landingCoverage := withUsable(w.LandingCoverage, usableLandings)
	landingCounts := landingAttribution(w, period, goalID)
	if goalID != "" {
		landingCoverage = attributedCoverage(landingCoverage, usableLandings, goalID, landingCounts)
		row.Details = append(row.Details, unattributedBucket("landings", landingCounts.Unattributed))
	}
	row.Coverage = []Coverage{goalCoverage, landingCoverage}
	row.Thresholds = []string{limits.Waiting.judgment("proving share", judgment)}
	proofCoverage := withUsable(w.ProofCoverage, 0)
	for _, proof := range w.Proofs {
		if proof.Surface == "milestone-battery" && !proof.StartedAt.IsZero() {
			proofCoverage.Usable++
		}
	}
	row.Details = append(row.Details, detail{Text: "whole-battery detail " + proofCoverage.String(), MachineOnly: true})
	row.Details = append(row.Details, joinedDetails(goalCoverage, landingCoverage)...)
	return row
}

func computeDebt(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Debt age", "debt_age", "fleet-synced", "own each aging item or close it", "coordinator")
	ids := make([]string, 0, len(w.Goals))
	for id := range w.Goals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	usable := 0
	crossed := false
	var values []string
	for _, id := range ids {
		if goalID != "" && id != goalID {
			continue
		}
		file := w.Goals[id].File
		if file.State == goal.StateDone {
			continue
		}
		usable++
		var anchor time.Time
		kind := ""
		switch {
		case file.State == goal.StateParked && file.Parked != nil:
			anchor, _ = time.Parse(time.RFC3339, file.Parked.At)
			kind = "parked"
		case file.State == goal.StateQueued:
			anchor, _ = time.Parse(time.RFC3339, file.OpenedAt)
			kind = "queued opened-at anchor"
		}
		if kind == "" || anchor.IsZero() || anchor.After(period.Instant) {
			continue
		}
		age := period.Instant.Sub(anchor).Hours() / 24
		values = append(values, fmt.Sprintf("%s kind=%s age_days=%s", id, kind, formatFloat(age)))
		if limits.Debt.Invalid == "" && age > float64(limits.Debt.Value) {
			crossed = true
		}
	}
	if usable == 0 {
		row.Value = "unavailable"
	} else if len(values) == 0 {
		row.Value = "items=0"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	coverage := withUsable(w.GoalCoverage, usable)
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{limits.Debt.judgment("debt age days", crossed, len(values) > 0)}
	row.Details = append(row.Details, detail{Text: "no residue register exists"})
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}

func computeDelegates(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Built by delegates", "built_by_delegates", "fleet-synced", "draft asking why the coordinator built the item or why its builder went unrecorded", "coordinator")
	usable, delegated, recorded, mixed, unrecorded := 0, 0, 0, 0, 0
	coverage := w.ReceiptCoverage
	for _, receipt := range w.Receipts {
		if !receiptInScope(receipt, period, goalID) {
			continue
		}
		switch receipt.Fields["built_by"] {
		case "delegate":
			delegated++
			recorded++
		case "coordinator":
			recorded++
		case "mixed":
			mixed++
			recorded++
		case "":
			unrecorded++
		default:
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d invalid built_by", receipt.Line))
			continue
		}
		usable++
	}
	coverage.Usable = usable
	var share *float64
	if usable == 0 {
		row.Value = "unavailable"
	} else if recorded == 0 {
		row.Value = fmt.Sprintf("unavailable (builder unrecorded) unrecorded=%d", unrecorded)
	} else {
		value := float64(delegated) / float64(recorded)
		share = &value
		row.Value = fmt.Sprintf("delegate_items=%d builder_recorded_items=%d share=%s mixed=%d unrecorded=%d", delegated, recorded, formatFloat(value), mixed, unrecorded)
	}
	counts := receiptAttribution(w, period, goalID)
	if goalID != "" {
		coverage = attributedCoverage(coverage, usable, goalID, counts)
	}
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{limits.Delegates.judgment("delegate-built share", share)}
	if unrecorded > 0 {
		row.Details = append(row.Details, detail{Text: fmt.Sprintf("builder unrecorded: items=%d", unrecorded)})
	}
	row.Details = append(row.Details, unattributedBucket("receipts", counts.Unattributed))
	if counts.Rejected > 0 {
		row.Details = append(row.Details, rejectedAttributionBucket("receipts", counts.Rejected))
	}
	row.Details = append(row.Details, detail{Text: "landed bytes stay deferred until builder provenance accumulates"})
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}

func computeCollisions(w world, period Period, goalID string, limits thresholds) metricRow {
	row := metric("Cross-machine collisions", "cross_machine_collisions", "mixed scopes, separated", "draft naming the goal, verb, and collision class", "coordinator")
	trueEvents, displaced, steals, historyUsable := 0, 0, 0, 0
	seen := map[string]bool{}
	for id, record := range w.Goals {
		if goalID != "" && id != goalID {
			continue
		}
		for _, history := range record.File.History {
			stamp, err := time.Parse(time.RFC3339, history.At)
			if err != nil || (goalID == "" && !period.contains(stamp.UTC())) {
				continue
			}
			historyUsable++
			if history.Displaced == "" && history.Verb != "steal" {
				continue
			}
			key := history.Opid + "|" + id
			if seen[key] {
				continue
			}
			seen[key] = true
			trueEvents++
			if history.Displaced != "" {
				displaced++
			}
			if history.Verb == "steal" {
				steals++
			}
		}
	}
	journalUsable := 0
	for _, record := range w.Journals {
		if !journalInScope(record, period, goalID) {
			continue
		}
		switch {
		case record.Outcome == "lost":
			journalUsable++
			row.Details = append(row.Details, detail{Text: "contested transaction: " + filepath.Base(record.Path) + " (counterpart unidentifiable — same-machine and cross-machine look alike)", MachineOnly: true})
		case record.Outcome == "confirmed" && record.Attempts > 1:
			journalUsable++
			row.Details = append(row.Details, detail{Text: fmt.Sprintf("contention context: %s attempts=%d confirmed", filepath.Base(record.Path), record.Attempts), MachineOnly: true})
		}
	}
	goalHistoryCoverage := Coverage{Source: "goal-history", Found: w.GoalCoverage.Found, Usable: historyUsable, Rejected: w.GoalCoverage.Rejected, Missing: w.GoalCoverage.Missing, Details: w.GoalCoverage.Details}
	journalCoverage := withUsable(w.JournalCoverage, journalUsable)
	transportCoverage := Coverage{Source: "transport-push-failures", Missing: 1}
	if historyUsable == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = fmt.Sprintf("true_cross_machine_events=%d displaced=%d steals=%d", trueEvents, displaced, steals)
	}
	row.Coverage = []Coverage{goalHistoryCoverage, journalCoverage, transportCoverage}
	row.Thresholds = []string{limits.Collisions.judgment("true cross-machine events", trueEvents > int(limits.Collisions.Value), historyUsable > 0)}
	row.Details = append(row.Details, detail{Text: "no transport-failure record exists"})
	row.Details = append(row.Details, joinedDetails(goalHistoryCoverage, journalCoverage)...)
	return row
}

func jobInScope(job jobRecord, period Period, goalID string) bool {
	if job.EndedAt.IsZero() || !dispatch.TerminalStatus(job.Status) {
		return false
	}
	if goalID == "" {
		return period.contains(job.EndedAt)
	}
	return job.GoalID == goalID
}

func computeCost(w world, period Period, goalID string) metricRow {
	row := metric("Cost per result", "cost_per_result", "this-machine context-only", "read the dimensioned trend at retro", "coordinator")
	tokenNames := []string{"inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"}
	tokens := map[string]float64{}
	tokenCoverage := map[string]int{}
	costs := map[string]float64{}
	costCoverage := map[string]int{}
	units := map[string]float64{}
	unitCoverage := map[string]int{}
	jobs, timedJobs := 0, 0
	var wall time.Duration
	for _, job := range w.Jobs {
		if !jobInScope(job, period, goalID) {
			continue
		}
		jobs++
		if job.TimingError == "" {
			timedJobs++
			wall += job.EndedAt.Sub(job.StartedAt)
		}
		if job.Usage == nil {
			continue
		}
		runtimeName := emptyAs(job.Runtime, "unknown")
		for _, dimension := range tokenNames {
			if value, ok := number(job.Usage[dimension]); ok {
				key := runtimeName + "," + dimension
				tokens[key] += value
				tokenCoverage[key]++
			}
		}
		if cost, ok := job.Usage["cost"].(map[string]any); ok {
			amount, amountOK := number(cost["amount"])
			currency, currencyOK := cost["currency"].(string)
			if amountOK && currencyOK && currency != "" {
				costs[currency] += amount
				costCoverage[currency]++
			}
		}
		if unit, ok := job.Usage["providerUnits"].(map[string]any); ok {
			value, valueOK := number(unit["value"])
			name, nameOK := unit["name"].(string)
			if valueOK && nameOK && name != "" {
				key := runtimeName + "," + name
				units[key] += value
				unitCoverage[key]++
			}
		}
	}
	results := 0
	if goalID != "" {
		if record, ok := w.Goals[goalID]; ok && record.File.State == goal.StateDone {
			results = 1
		}
	} else {
		results = len(selectedGoals(w, period, ""))
	}
	var values []string
	if jobs > 0 {
		if timedJobs == 0 {
			values = append(values, "wall_hours=unavailable")
		} else {
			values = append(values, "wall_hours="+formatHours(wall))
		}
		values = append(values, fmt.Sprintf("results=%d", results))
	}
	appendGroups := func(label string, sums map[string]float64, counts map[string]int) {
		keys := make([]string, 0, len(sums))
		for key := range sums {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			part := fmt.Sprintf("%s[%s]=%s records=%d", label, key, formatFloat(sums[key]), counts[key])
			if results > 0 {
				part += " per_result=" + formatFloat(sums[key]/float64(results))
			}
			values = append(values, part)
		}
	}
	appendGroups("tokens", tokens, tokenCoverage)
	appendGroups("cost", costs, costCoverage)
	appendGroups("provider_units", units, unitCoverage)
	if jobs == 0 {
		row.Value = "unavailable"
	} else {
		row.Value = strings.Join(values, "; ")
	}
	coverage := withUsable(w.JobCoverage, timedJobs)
	jobCounts := jobAttribution(w, period, goalID)
	if goalID != "" {
		coverage = attributedCoverage(coverage, timedJobs, goalID, jobCounts)
		row.Details = append(row.Details, unattributedBucket("jobs", jobCounts.Unattributed))
	}
	row.Coverage = []Coverage{coverage}
	row.Thresholds = []string{"context-only"}
	costPresent := 0
	for _, count := range costCoverage {
		costPresent += count
	}
	if costPresent < jobs {
		row.Details = append(row.Details, detail{Text: fmt.Sprintf("partial cost coverage: cost_present=%d jobs=%d", costPresent, jobs)})
	}
	if jobs > 0 && timedJobs == 0 {
		row.Details = append(row.Details, detail{Text: "wall-clock unavailable: no in-scope jobs carry both timestamps"})
	}
	row.Details = append(row.Details, joinedDetails(coverage)...)
	return row
}
