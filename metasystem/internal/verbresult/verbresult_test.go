package verbresult

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// A child for Run: this test binary re-executed, printing what its
// environment says to stdout and stderr and exiting with its status.
func init() {
	if os.Getenv("VERBRESULT_CHILD") == "1" {
		_, _ = os.Stderr.WriteString(os.Getenv("VERBRESULT_STDERR"))
		_, _ = os.Stdout.WriteString(os.Getenv("VERBRESULT_STDOUT"))
		status, _ := strconv.Atoi(os.Getenv("VERBRESULT_EXIT"))
		os.Exit(status)
	}
}

func child(t *testing.T, stdout, stderr string, status int) *exec.Cmd {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(self)
	command.Env = append(os.Environ(), "VERBRESULT_CHILD=1", "VERBRESULT_STDOUT="+stdout, "VERBRESULT_STDERR="+stderr,
		"VERBRESULT_EXIT="+strconv.Itoa(status))
	return command
}

func envelope(t *testing.T, result Result) string {
	t.Helper()
	var out bytes.Buffer
	if err := Write(&out, result); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// A refused child's envelope reads as its code, even when its stderr
// carries another code-shaped word first (the defect the four classifiers
// had: the first *_REFUSED word on the combined output won).
func TestRunReadsTheEnvelopeNotTheStderrWords(t *testing.T) {
	printed := envelope(t, Result{Verb: "internal test run", Outcome: Refused, Code: "GOAL_REVISION_MOVED", Summary: "the goal moved"})
	result, err := Run(child(t, printed, "note: X_REFUSED by an earlier step\n", 78), "internal test run")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != Refused || result.Code != "GOAL_REVISION_MOVED" || result.Exit != 78 {
		t.Fatalf("result = %+v", result)
	}
}

// Every unreadable stdout reads as unknown, with an error quoting stderr,
// and never as success.
func TestRunReadsAMissingOrBrokenEnvelopeAsUnknown(t *testing.T) {
	good := envelope(t, Result{Verb: "internal test run", Outcome: Confirmed, Summary: "ran"})
	cases := map[string]struct {
		stdout string
		status int
	}{
		"empty":           {"", 0},
		"truncated":       {good[:len(good)/2], 0},
		"trailing":        {good + "PROOF-RESULT {}\n", 0},
		"two envelopes":   {good + good, 0},
		"text":            {"TEST-PLAN mode=standard\n", 0},
		"other verb":      {envelope(t, Result{Verb: "test plan", Outcome: Confirmed}), 0},
		"confirmed but 1": {good, 1},
		"refused but 0":   {envelope(t, Result{Verb: "internal test run", Outcome: Refused, Code: "X_REFUSED"}), 0},
		"killed":          {good, -1},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			status := test.status
			if status < 0 {
				status = 0
			}
			command := child(t, test.stdout, "the child's own words\n", status)
			var result Result
			var err error
			if test.status < 0 {
				result, err = Read([]byte(test.stdout), "internal test run", -1, "the child's own words\n")
			} else {
				result, err = Run(command, "internal test run")
			}
			if err == nil || result.Outcome != Unknown {
				t.Fatalf("result = %+v, err = %v; want unknown with an error", result, err)
			}
			if !strings.Contains(err.Error(), "the child's own words") {
				t.Errorf("error %q does not quote stderr", err)
			}
		})
	}
}

// The writer maps an exit status and a coded error onto the envelope, and
// carries a refusal's data; Err gives the code back to errors.As.
func TestFromErrorCarriesCodeSummaryNextAndData(t *testing.T) {
	coded := &refusal.Coded{Code: "CANDIDATE_GOAL_REFUSED", Reason: errors.New("goal g is stopped"), Run: "metasystem goal show g"}
	result := FromError("internal test run", 78, stateError{coded, "fenced"}, nil)
	if result.Outcome != Refused || result.Code != "CANDIDATE_GOAL_REFUSED" || result.Summary != "goal g is stopped" ||
		result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem goal show g" || string(result.Data) != `{"state":"fenced"}` {
		t.Fatalf("result = %+v data=%s", result, result.Data)
	}
	back, err := Read([]byte(envelope(t, result)), "internal test run", 78, "")
	if err != nil {
		t.Fatal(err)
	}
	var carried *refusal.Coded
	if !errors.As(back.Err(), &carried) || carried.Code != "CANDIDATE_GOAL_REFUSED" {
		t.Fatalf("Err() lost the code: %v", back.Err())
	}
	if plain := FromError("internal test run", 1, errors.New("disk full"), nil); plain.Outcome != Failed || plain.Code != "" {
		t.Fatalf("plain = %+v", plain)
	}
}

type stateError struct {
	*refusal.Coded
	state string
}

func (e stateError) Unwrap() error   { return e.Coded }
func (e stateError) ResultData() any { return map[string]string{"state": e.state} }

// The exit/outcome pairs (R3): the writer's table always passes the
// reader's check.
func TestOutcomeForExitIsAlwaysPermitted(t *testing.T) {
	for _, status := range []int{0, 1, 2, 3, 75, 76, 77, 78, 124} {
		if outcome := OutcomeForExit(status); !Permitted(outcome, status) {
			t.Errorf("exit %d maps to %q, which the reader refuses", status, outcome)
		}
	}
	for outcome, status := range map[string]int{Confirmed: 76, Unchanged: 1, Refused: 0, Failed: 0, InProgress: 0, "": 0} {
		if Permitted(outcome, status) {
			t.Errorf("%q with exit %d is permitted", outcome, status)
		}
	}
}
