package plain

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
)

// commandTail retains only the command's last 64 KiB as it copies its output.
// A report larger than the tail cannot authorize a repeat with a partial count.
type commandTail struct {
	output io.Writer
	tail   []byte
}

func (w *commandTail) Write(p []byte) (int, error) {
	n, err := w.output.Write(p)
	const limit = 64 * 1024
	p = p[:n]
	if len(p) >= limit {
		w.tail = append(w.tail[:0], p[len(p)-limit:]...)
	} else {
		if extra := len(w.tail) + len(p) - limit; extra > 0 {
			w.tail = w.tail[extra:]
		}
		w.tail = append(w.tail, p...)
	}
	return n, err
}

type checkReport struct {
	kind   string
	failed []FailedUnit
	load   float64
}

// FailedChecks reads test identities only from a complete command report.
func FailedChecks(data []byte) []FailedUnit { return readReport(data).failed }

func readReport(tail []byte) checkReport {
	lines := strings.Split(strings.TrimSuffix(string(tail), "\n"), "\n")
	last := strings.Split(strings.TrimSuffix(lines[len(lines)-1], "\r"), "\t")
	if len(last) == 2 && last[0] == "LANDING-NOT-RUN" {
		return checkReport{kind: "not-run"}
	}
	checked := len(lines) - 1
	// The engine can render its result after the command's final report.
	// Any later protocol line leaves that report incomplete.
	for checked >= 0 && !strings.HasPrefix(lines[checked], "LANDING-") {
		checked--
	}
	if checked < 0 {
		return checkReport{}
	}
	last = strings.Split(strings.TrimSuffix(lines[checked], "\r"), "\t")
	if len(last) != 2 || last[0] != "LANDING-CHECKED" {
		return checkReport{}
	}
	count, err := strconv.Atoi(last[1])
	if err != nil || count < 0 {
		return checkReport{}
	}
	report := checkReport{kind: "complete"}
	for i := checked - 1; i >= 0; i-- {
		fields := strings.Split(strings.TrimSuffix(lines[i], "\r"), "\t")
		switch fields[0] {
		case "LANDING-FAILED":
			if len(fields) != 3 || fields[1] == "" {
				return checkReport{}
			}
			report.failed = append([]FailedUnit{{Unit: fields[1], Tests: strings.Fields(fields[2])}}, report.failed...)
		case "LANDING-LOAD":
			if len(fields) != 2 {
				return checkReport{}
			}
			report.load, err = strconv.ParseFloat(fields[1], 64)
			if err != nil || report.load < 0 || math.IsNaN(report.load) || math.IsInf(report.load, 0) {
				return checkReport{}
			}
		default:
			if count != len(report.failed) {
				return checkReport{}
			}
			return report
		}
	}
	if count != len(report.failed) {
		return checkReport{}
	}
	return report
}

func recordFlakes(seams ProveSeams, red, green Result, kind string, repeats []Running) Result {
	var reasons []string
	green.Flakes = nil
	for i, unit := range red.Failed {
		if seams.RecordFlake == nil && green.Result == Green {
			green.Cause = &Cause{Kind: "flake", Tests: failingTests(red.Failed), Evidence: red.Log}
			green.Result, green.Reason = Red, "the failed tests could not be recorded"
			green.Repeat, green.Failed = "started", red.Failed
			return green
		}
		repeat := repeats[0]
		if kind == "alone" {
			repeat = repeats[i]
		}
		where := "tip"
		if repeat.Trunk {
			where = "main"
		}
		f := FlakeRecord{FailedUnit: unit, Commit: red.Commit, Tree: red.Tree, Attempt: red.Attempt, Log: red.Log, Load: red.Load,
			Where: where, Repeat: kind, RepeatAttempt: repeat.Attempt, RepeatLog: repeat.Log}
		var err error
		f.Outputs, err = repoproof.TestEvidence(f.Log, f.Unit, f.Tests, "fail")
		if err == nil {
			expected := ""
			if green.Result == Green {
				expected = "pass"
			}
			f.RepeatOutputs, err = repoproof.TestEvidence(f.RepeatLog, f.Unit, f.Tests, expected)
		}
		green.Flakes = append(green.Flakes, f)

		var recorded FlakeRecorded
		if err == nil && green.Result == Green {
			recorded, err = seams.RecordFlake(f)
		}
		if err != nil {
			if green.Result == Green {
				green.Cause = &Cause{Kind: "flake", Tests: failingTests(red.Failed), Evidence: red.Log}
			}
			green.Result, green.Reason = Red, "the failed tests could not be recorded: "+err.Error()
			green.Repeat, green.Failed = "started", red.Failed
			return green
		}
		if green.Result != Green {
			continue
		}
		how := "alone"
		if kind == "whole" {
			how = "in a whole check"
		}
		seen := "once"
		if recorded.Seen != 1 {
			seen = fmt.Sprintf("%d times", recorded.Seen)
		}
		reasons = append(reasons, fmt.Sprintf("%s failed once and passed when run again %s; seen %s; goal %s fixes it", unit.Unit, how, seen, recorded.Goal))
	}
	if green.Result == Green {
		green.FlakePublished = true
		green.Reason = strings.Join(reasons, "; ")
		if green.Scope == "full" {
			green.FullTree, green.FullAt = green.Tree, green.At
		}
	}
	return green
}
