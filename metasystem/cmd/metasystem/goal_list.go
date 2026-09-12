package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const goalListSummaryMaxBytes = 64 * 1024

type goalListOutput struct {
	JSON, History, Done, Pretty bool
}

func goalDisplayRecord(file *goal.GoalFile, history bool) *goal.GoalFile {
	if history {
		return file
	}
	// Projection records remain intact for validation and other readers.
	copy := *file
	copy.History = []goal.HistoryLine{}
	return &copy
}

func printGoalListJSON(value any, pretty bool) int {
	encoder := json.NewEncoder(os.Stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// syncedListStates are the buckets a synced ledger has, in the order the
// summary lists them; the legacy ledger has its current goal instead of
// claimed and approved. "open" is not a state of either.
var syncedListStates = []string{goal.StateClaimed, goal.StateApproved, goal.StateQueued, goal.StateParked}
var legacyListStates = []string{"current", goal.StateQueued, goal.StateParked}

func goalListSummary(grouped map[string][]*goal.GoalFile, states []string, tip string, notices []string, includeDone bool, horizon goal.ApprovalHorizon) string {
	if includeDone {
		states = append(append([]string{}, states...), goal.StateDone)
	}
	remaining := 0
	for _, state := range states {
		remaining += len(grouped[state])
	}
	var summary strings.Builder
	for _, state := range states {
		fmt.Fprintf(&summary, "%s=%d ", state, len(grouped[state]))
	}
	if !includeDone {
		fmt.Fprintf(&summary, "done=%d ", len(grouped[goal.StateDone]))
	}
	fmt.Fprintf(&summary, "tip=%s\n", tip)
	footer := func() string {
		return fmt.Sprintf("... %d more; run with --json > file for the records\n", remaining)
	}
	appendLine := func(line string, more bool) bool {
		// Reserve the omission count before accepting a complete line, so
		// even a large backlog leaves a usable path to its missing records.
		reserve := 0
		if more {
			reserve = len(footer())
		}
		if summary.Len()+len(line)+reserve > goalListSummaryMaxBytes {
			return false
		}
		summary.WriteString(line)
		return true
	}
	for i, notice := range notices {
		if !appendLine("! "+strings.Join(strings.Fields(notice), " ")+"\n", remaining > 0 || i+1 < len(notices)) {
			summary.WriteString(footer())
			return summary.String()
		}
	}
	for _, state := range states {
		files := make(map[string]*goal.GoalFile, len(grouped[state]))
		for _, file := range grouped[state] {
			files[file.Id] = file
		}
		for _, id := range goal.OrderedOpenGoalIDs(files) {
			file := files[id]
			pin, claim := "-", "-"
			if file.Pinned != "" {
				pin = file.Pinned
			}
			if file.Claimed != nil && file.Claimed.Machine != "" {
				claim = file.Claimed.Machine
			}
			line := fmt.Sprintf("%d:%d %s tier %d %s pin=%s claim=%s%s :: %s\n",
				file.Priority, file.Sequence, file.State, file.Tier, file.Id, pin, claim, goalListMarkers(file, horizon), goalNextSentence(file.NextStep))
			if !appendLine(line, remaining > 1) {
				summary.WriteString(footer())
				return summary.String()
			}
			remaining--
		}
	}
	return summary.String()
}

// goalListMarkers carries the facts the table view used to print beside a
// goal, so the summary loses none of them: a landing in progress, why a
// goal is parked, and whether a relayed approval still stands.
func goalListMarkers(file *goal.GoalFile, horizon goal.ApprovalHorizon) string {
	var markers []string
	if file.Landing != nil {
		markers = append(markers, "landing-since="+file.Landing.At)
	}
	if file.Parked != nil && file.Parked.Because != "" {
		markers = append(markers, "parked="+goalCutRunes(strings.Join(strings.Fields(file.Parked.Because), " "), 80))
	}
	if file.Approved != nil && file.Approved.Authority == goal.ApprovalAuthorityRelayed {
		if expired, why := file.ApprovalExpired(horizon); expired {
			markers = append(markers, "relayed=EXPIRED:"+strings.ReplaceAll(why, " ", "_"))
		} else {
			markers = append(markers, "relayed=review-by:"+file.Approved.ReviewBy)
		}
	}
	if len(markers) == 0 {
		return ""
	}
	return " " + strings.Join(markers, " ")
}

// goalNextSentence is the next step's first sentence, cut at 120 runes with
// a visible mark, control characters dropped: this text reaches a terminal
// and a seat's context raw, where the JSON escaped it.
func goalNextSentence(next string) string {
	var cleaned []rune
	for _, char := range strings.Join(strings.Fields(next), " ") {
		if !unicode.IsControl(char) {
			cleaned = append(cleaned, char)
		}
	}
	text := cleaned
	for i, char := range text {
		if strings.ContainsRune(".!?。！？", char) && (i+1 == len(text) || text[i+1] == ' ') {
			text = text[:i+1]
			break
		}
	}
	return goalCutRunes(string(text), 120)
}

// goalCutRunes cuts text at limit runes and marks the cut, so a reader can
// tell a whole sentence from a partial one.
func goalCutRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-3]) + "..."
}

func legacyGoalGroups(ledger *goal.Ledger) map[string][]*goal.GoalFile {
	grouped := map[string][]*goal.GoalFile{}
	if ledger == nil {
		return grouped
	}
	legacy := map[string][]goal.Goal{
		goal.StateQueued: ledger.Queued, goal.StateParked: ledger.Parked, goal.StateDone: ledger.Done,
	}
	if ledger.Current != nil {
		legacy["current"] = []goal.Goal{*ledger.Current}
	}
	for state, files := range legacy {
		for _, file := range files {
			grouped[state] = append(grouped[state], &goal.GoalFile{Id: file.Id, State: state, NextStep: file.NextStep})
		}
	}
	return grouped
}
