package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// runTestReport summarizes an existing result without selecting or running
// any checks. The caller supplies the observed-cost threshold for its cohort.
func runTestReport(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("test status", stdout, stderr)
	resultPath := flags.String("result", "", "retained TestResult JSON path")
	expensiveMS := flags.Int64("expensive-ms", 0, "positive observed group-duration threshold in milliseconds")
	jsonOutput := flags.Bool("json", false, "print the cost summary as JSON")
	verbose := flags.Bool("verbose", false, "every group, not only the expensive ones")
	refuse := func(code int, line1 string, hint textui.Hint) int {
		page := passthroughPage(stderr, "", *verbose)
		page.Refusal(line1, hint)
		printPage(stderr, page)
		return code
	}
	if flags.Parse(args) != nil || *resultPath == "" || *expensiveMS <= 0 || flags.NArg() != 0 {
		return refuse(2, "a result's cost needs the result file and a positive threshold; nothing was read",
			textui.Hint{Argv: []string{"metasystem", "test", "status", "--result", "FILE", "--expensive-ms", "5000"}, Reason: "FILE is a recorded test result"})
	}
	result, err := testrun.ReadWorkerResult(*resultPath)
	if err != nil {
		return refuse(1, "the result cannot be read: "+err.Error(), textui.Hint{Reason: "name a result a test run recorded"})
	}
	report, err := proofrun.SummarizeTestResultCost(result, *expensiveMS)
	if err != nil {
		return refuse(1, "the result's cost cannot be summarized: "+err.Error(), textui.Hint{Reason: "name a result a test run recorded"})
	}
	if *jsonOutput {
		writeJSONLine(stdout, stderr, report)
		return 0
	}
	page := passthroughPage(stdout, "", *verbose)
	layTestCost(page, report)
	printPage(stdout, page)
	return 0
}

// layTestCost is a recorded result's measured cost: the groups run and
// reused, the observed time against the threshold, and the expensive
// groups by name (every group with --verbose).
func layTestCost(page *textui.Page, report proofrun.TestCostSummary) {
	ran, reused := report.NativeTestGroups+report.NativeBuildGroups, report.ReusedTestGroups+report.ReusedBuildGroups
	facts := []string{fmt.Sprintf("%d ran · %d reused", ran, reused)}
	if report.TotalDurationMS != nil {
		facts = append(facts, testMillis(*report.TotalDurationMS)+" in all")
	}
	page.Headline("Tree "+textui.SHA(report.CandidateTree)+": "+textui.Count(len(report.Groups), "test group", "test groups"), facts...)
	threshold := testMillis(report.ExpensiveThresholdMS)
	section := page.Section("Expensive", "observed over "+threshold)
	if report.NativeExpensiveGroups+report.ReusedExpensiveGroups == 0 {
		section.Text("none")
	}
	table := section.Table(textui.Column{}, textui.Column{Right: true}, textui.Column{Flex: true})
	for _, group := range report.Groups {
		expensive := group.Expensive != nil && *group.Expensive
		if !expensive && !page.Verbose() {
			continue
		}
		observed := "not observed"
		if group.ObservedDurationMS != nil {
			observed = testMillis(*group.ObservedDurationMS)
		}
		table.Row(textui.Plain(group.ID), textui.Plain(observed), textui.Dim(group.Status))
	}
	if report.UnknownExpenseGroups > 0 {
		section.Text(textui.Count(report.UnknownExpenseGroups, "group has", "groups have") + " no observed time")
	}
}

// testMillis is a measured time: milliseconds under a second, else as
// textui spells durations.
func testMillis(ms int64) string {
	if ms < 1000 {
		return strconv.FormatInt(ms, 10) + "ms"
	}
	return textui.Duration(time.Duration(ms) * time.Millisecond)
}
