package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Result names every report file published by one invocation. Target is the
// command's primary file; Paths also includes the detailed weekly tier and
// any missing goal reports produced by the period sweep.
type Result struct {
	Target string
	Paths  []string
}

// WriteError names the exact report whose pre-publication write failed.
type WriteError struct {
	Target string
	Err    error
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("cannot write metrics report %s: %v", e.Target, e.Err)
}

func (e *WriteError) Unwrap() error { return e.Err }

var writeReport = func(path, content, anchor string) error {
	_, err := atomicfile.WriteText(path, content, anchor)
	return err
}

// Report computes and atomically publishes one period or per-goal report.
func Report(opts Options) (Result, error) {
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("metrics report: cannot resolve checkout root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	opts.Root = absRoot
	if opts.GoalID != "" && !goalToken.MatchString(opts.GoalID) {
		return Result{}, fmt.Errorf("metrics report: invalid goal id %q", opts.GoalID)
	}
	period, err := resolvePeriod(opts)
	if err != nil {
		return Result{}, err
	}
	w, err := loadWorld(absRoot)
	if err != nil {
		return Result{}, err
	}
	if opts.GoalID != "" {
		if _, exists := w.Goals[opts.GoalID]; !exists {
			target := filepath.Join(absRoot, "artifacts", "agents", "metrics", "goal-"+opts.GoalID+".md")
			result := Result{Target: target}
			if err := publish(target, renderUnavailableGoalReport(w, period, opts.GoalID), absRoot); err != nil {
				return result, err
			}
			result.Paths = append(result.Paths, target)
			return result, nil
		}
	}
	limits := loadThresholds(absRoot)
	rows := computeRows(w, period, opts.GoalID, limits)

	if opts.GoalID != "" {
		target := filepath.Join(absRoot, "artifacts", "agents", "metrics", "goal-"+opts.GoalID+".md")
		result := Result{Target: target}
		if err := publish(target, renderReport(w, period, opts.GoalID, rows, false), absRoot); err != nil {
			return result, err
		}
		result.Paths = append(result.Paths, target)
		return result, nil
	}

	result := Result{}
	if period.CalendarWeek {
		detailed := detailedPeriodPath(absRoot, period)
		if err := publish(detailed, renderReport(w, period, "", rows, false), absRoot); err != nil {
			return Result{Target: detailed}, err
		}
		result.Paths = append(result.Paths, detailed)
		target := filepath.Join(absRoot, "plans", "metrics", w.Machine, period.Week+".md")
		result.Target = target
		if err := publish(target, renderReport(w, period, "", rows, true), absRoot); err != nil {
			return result, err
		}
		result.Paths = append(result.Paths, target)
	} else {
		target := detailedPeriodPath(absRoot, period)
		result.Target = target
		if err := publish(target, renderReport(w, period, "", rows, false), absRoot); err != nil {
			return result, err
		}
		result.Paths = append(result.Paths, target)
	}

	for _, record := range selectedGoals(w, period, "") {
		target := filepath.Join(absRoot, "artifacts", "agents", "metrics", "goal-"+record.File.Id+".md")
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return result, &WriteError{Target: target, Err: err}
		}
		goalPeriod := Period{Instant: period.Instant}
		goalRows := computeRows(w, goalPeriod, record.File.Id, limits)
		if err := publish(target, renderReport(w, goalPeriod, record.File.Id, goalRows, false), absRoot); err != nil {
			return result, err
		}
		result.Paths = append(result.Paths, target)
	}
	return result, nil
}

func renderUnavailableGoalReport(w world, period Period, goalID string) string {
	coverage := withUsable(w.GoalCoverage, 0)
	coverage.Missing = 1
	coverage.Extra = "goal=" + goalID + " status=missing-goal"
	var b strings.Builder
	b.WriteString("# Metasystem metrics report\n\n")
	b.WriteString("report_kind=goal\n")
	b.WriteString("goal=" + goalID + "\n")
	b.WriteString("report_status=UNAVAILABLE\n")
	b.WriteString("report_instant=" + period.Instant.Format(time.RFC3339) + "\n")
	b.WriteString("machine=" + w.Machine + "\n")
	b.WriteString("source_identity=" + w.Identity.String() + "\n")
	b.WriteString("tier=detailed-this-machine\n")
	b.WriteString("coverage=" + coverage.String() + "\n")
	b.WriteString("detail=requested goal " + goalID + " is absent from the accepted goal ledger\n")
	return b.String()
}

func publish(target, content, anchor string) error {
	if err := writeReport(target, content, anchor); err != nil {
		return &WriteError{Target: target, Err: err}
	}
	return nil
}

func renderReport(w world, period Period, goalID string, rows []metricRow, compact bool) string {
	var b strings.Builder
	b.WriteString("# Metasystem metrics report\n\n")
	if goalID == "" {
		b.WriteString("report_kind=period\n")
		b.WriteString("period_start=" + period.Start.Format(time.RFC3339) + "\n")
		b.WriteString("period_end=" + period.End.Format(time.RFC3339) + "\n")
	} else {
		b.WriteString("report_kind=goal\n")
		b.WriteString("goal=" + goalID + "\n")
	}
	b.WriteString("report_instant=" + period.Instant.Format(time.RFC3339) + "\n")
	b.WriteString("machine=" + w.Machine + "\n")
	b.WriteString("source_identity=" + w.Identity.String() + "\n")
	if compact {
		b.WriteString("tier=compact-tracked\n")
	} else {
		b.WriteString("tier=detailed-this-machine\n")
	}
	for _, row := range rows {
		b.WriteString("\nmetric=" + row.Key + "\n")
		b.WriteString("name=" + row.Name + "\n")
		b.WriteString("scope=" + row.Scope + "\n")
		b.WriteString("value=" + row.Value + "\n")
		for _, coverage := range row.Coverage {
			b.WriteString("coverage=" + coverage.String() + "\n")
		}
		for _, threshold := range row.Thresholds {
			b.WriteString("threshold=" + threshold + "\n")
		}
		b.WriteString("action=" + row.Action + "\n")
		b.WriteString("owner=" + row.Owner + "\n")
		for _, item := range row.Details {
			if compact && item.MachineOnly {
				continue
			}
			b.WriteString("detail=" + cleanLine(item.Text) + "\n")
		}
	}
	return b.String()
}

// GoalReportTarget returns the absolute path the goal-done fast path names in
// a warning when report publication fails before Report can return a result.
func GoalReportTarget(root, id string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return filepath.Join(abs, "artifacts", "agents", "metrics", "goal-"+id+".md")
}

// ConcludedInWindow exposes the sweep predicate for focused callers and tests.
func ConcludedInWindow(file *goal.GoalFile, start, end time.Time) bool {
	done, ok := historyTime(file, "done")
	return ok && !done.Before(start) && done.Before(end)
}
