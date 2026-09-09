package goal

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TurnVerdictDisplayRuneLimit keeps one Stop refusal close to one screen.
const TurnVerdictDisplayRuneLimit = 4000

type runWarningClass int

const (
	runLooksHung runWarningClass = iota
	runEndedUnknown
	runWentRed
	runLivenessUnknown
	runUnsupervised
)

type runWarning struct {
	class runWarningClass
	id    string
	line  string
}

type runDisplayLines struct {
	lines      []string
	warnings   []runWarning
	unreadable []string
	actionable []string
}

func turnVerdictArtifactPath(root, sessionID string) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", sessionID+".txt")
}

func renderTurnVerdict(verdict Verdict, brainLines []string, runs runDisplayLines, greens []string, fileLine string) string {
	ladder := nonemptyLines(verdict.Display)
	verdictLine := "NOTHING LEFT TO WORK ON"
	var actionable []string
	if len(ladder) > 0 {
		verdictLine = ladder[0]
		actionable = append(actionable, ladder[1:]...)
	}
	if len(verdict.OpenWork) > 0 && strings.HasPrefix(verdictLine, "OPEN WORK (") {
		verdictLine = fmt.Sprintf("OPEN WORK (%d)", len(verdict.OpenWork))
		actionable = append(append([]string{}, verdict.OpenWork...), ladder[1:]...)
	}
	if verdict.BlockSource != nil && *verdict.BlockSource == "unwatched-work" && len(runs.actionable) > 0 {
		underlyingVerdict := verdictLine
		underlyingActions := actionable
		verdictLine = runs.actionable[0]
		actionable = append([]string{underlyingVerdict}, underlyingActions...)
		actionable = append(actionable, runs.actionable[1:]...)
	} else {
		actionable = append(actionable, runs.actionable...)
	}

	remainder := append([]string{}, brainLines...)
	remainder = append(remainder, summarizeRunWarnings(runs)...)
	remainder = append(remainder, summarizeGreens(greens)...)
	return boundedVerdictLines(verdictLine, actionable, remainder, fileLine)
}

func nonemptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func summarizeRunWarnings(runs runDisplayLines) []string {
	classes := []runWarningClass{runLooksHung, runEndedUnknown, runWentRed, runLivenessUnknown, runUnsupervised}
	var lines []string
	for _, class := range classes {
		var matching []runWarning
		for _, warning := range runs.warnings {
			if warning.class == class {
				matching = append(matching, warning)
			}
		}
		if len(matching) == 1 {
			lines = append(lines, matching[0].line)
		} else if len(matching) > 1 {
			lines = append(lines, runWarningSummary(class, len(matching), matching[0].id))
		}
	}
	if len(runs.unreadable) == 1 {
		lines = append(lines, runs.unreadable[0])
	} else if len(runs.unreadable) > 1 {
		lines = append(lines, fmt.Sprintf("%d run records unreadable; first %s", len(runs.unreadable), runs.unreadable[0]))
	}
	return lines
}

func runWarningSummary(class runWarningClass, count int, oldest string) string {
	switch class {
	case runLooksHung:
		return fmt.Sprintf("%d runs look hung, oldest %s", count, oldest)
	case runEndedUnknown:
		return fmt.Sprintf("%d runs ended ended-unknown or launch-failed, oldest %s", count, oldest)
	case runWentRed:
		return fmt.Sprintf("%d runs went red, oldest %s", count, oldest)
	case runLivenessUnknown:
		return fmt.Sprintf("%d runs of unknown liveness, oldest %s", count, oldest)
	default:
		return fmt.Sprintf("%d runs unsupervised, oldest %s", count, oldest)
	}
}

func summarizeGreens(greens []string) []string {
	if len(greens) == 0 {
		return nil
	}
	withoutContinuation := 0
	var withContinuation []string
	for _, line := range greens {
		if strings.HasSuffix(line, "the run record says: no continuation recorded") {
			withoutContinuation++
		} else {
			withContinuation = append(withContinuation, line)
		}
	}
	lines := []string{fmt.Sprintf("%d runs finished green without a recorded continuation; %d with one", withoutContinuation, len(withContinuation))}
	if len(withContinuation) > 3 {
		lines[0] += fmt.Sprintf("; %d more in the full turn verdict file", len(withContinuation)-3)
	}
	lines = append(lines, withContinuation[:min(3, len(withContinuation))]...)
	return lines
}

func boundedVerdictLines(verdictLine string, actionable, remainder []string, fileLine string) string {
	compose := func(actions, rest []string, notice string) string {
		lines := []string{verdictLine}
		lines = append(lines, actions...)
		if notice != "" {
			lines = append(lines, notice)
		}
		lines = append(lines, rest...)
		lines = append(lines, fileLine)
		return strings.Join(lines, "\n")
	}
	if full := compose(actionable, remainder, ""); len([]rune(full)) <= TurnVerdictDisplayRuneLimit {
		return full
	}

	kept := append([]string{}, actionable...)
	for len(kept) > 0 {
		dropped := len(actionable) - len(kept)
		notice := ""
		if dropped > 0 {
			notice = fmt.Sprintf("%d actionable items trimmed; see the full turn verdict file below", dropped)
		}
		if candidate := compose(kept, remainder, notice); len([]rune(candidate)) <= TurnVerdictDisplayRuneLimit {
			return candidate
		}
		kept = kept[:len(kept)-1]
	}
	notice := fmt.Sprintf("%d actionable items trimmed; see the full turn verdict file below", len(actionable))
	for len(remainder) > 0 {
		if candidate := compose(nil, remainder, notice); len([]rune(candidate)) <= TurnVerdictDisplayRuneLimit {
			return candidate
		}
		remainder = remainder[:len(remainder)-1]
	}
	return compose(nil, nil, notice)
}
