package launch

// A repeated launch whose effect already holds (R-129-ui): Launched answers
// the launch that already joined the machine, and nothing else.

import (
	"os"
	"path/filepath"
	"testing"
)

func launchedBed(t *testing.T) (checkout, clone string) {
	t.Helper()
	checkout = t.TempDir()
	clone = filepath.Join(t.TempDir(), "agentic-tools-m1f")
	if err := os.Mkdir(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	return checkout, clone
}

func TestLaunchedAnswersAFinishedOrArmedLaunchWhoseCloneIsThere(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{OutcomeDone, OutcomeArmed} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			checkout, clone := launchedBed(t)
			if err := Save(checkout, Record{Launch: launchID, Machine: "m1f", Destination: clone, Outcome: outcome}); err != nil {
				t.Fatal(err)
			}
			record, launched, err := Launched(checkout, Request{Machine: "m1f"})
			if err != nil || !launched || record.Launch != launchID || record.Destination != clone {
				t.Fatalf("Launched = %+v, %v, %v", record, launched, err)
			}
			// The same destination asked for, spelled uncleanly, is the same.
			record, launched, err = Launched(checkout, Request{Machine: "m1f", Destination: clone + "/./"})
			if err != nil || !launched || record.Launch != launchID {
				t.Fatalf("Launched at destination = %+v, %v, %v", record, launched, err)
			}
		})
	}
}

func TestLaunchedIsALaunchToRunForAnythingElse(t *testing.T) {
	t.Parallel()
	checkout, clone := launchedBed(t)
	gone := filepath.Join(filepath.Dir(clone), "removed-clone")
	for _, record := range []Record{
		{Launch: launchID, Machine: "m1f", Destination: clone, Outcome: OutcomeFailed},
		{Launch: secondLaunch, Machine: "m1f", Destination: gone, Outcome: OutcomeDone},
		{Launch: "01M3BQ0000000000000000000C", Machine: "m1g", Destination: clone, Outcome: OutcomeDone},
		{Launch: "01M3BQ0000000000000000000D", Machine: "m1f", Destination: "", Outcome: OutcomeArmed},
		{Launch: "01M3BQ0000000000000000000E", Machine: "m1f", Destination: clone, Outcome: OutcomeRunning},
	} {
		if err := Save(checkout, record); err != nil {
			t.Fatal(err)
		}
	}
	for name, request := range map[string]Request{
		"no finished launch with its clone": {Machine: "m1f"},
		"another destination":               {Machine: "m1g", Destination: filepath.Join(filepath.Dir(clone), "elsewhere")},
		"a machine never launched":          {Machine: "m1z"},
	} {
		record, launched, err := Launched(checkout, request)
		if err != nil || launched || record.Launch != "" {
			t.Fatalf("%s: Launched = %+v, %v, %v", name, record, launched, err)
		}
	}
	// The other machine at its own clone is answered, so the filter is by machine.
	if record, launched, err := Launched(checkout, Request{Machine: "m1g", Destination: clone}); err != nil || !launched || record.Machine != "m1g" {
		t.Fatalf("m1g: Launched = %+v, %v, %v", record, launched, err)
	}
}

func TestLaunchedOnAHostThatLaunchedNothingIsALaunchToRun(t *testing.T) {
	t.Parallel()
	record, launched, err := Launched(t.TempDir(), Request{Machine: "m1f"})
	if err != nil || launched || record.Launch != "" {
		t.Fatalf("Launched = %+v, %v, %v", record, launched, err)
	}
}

func TestLaunchedCarriesAnUnreadableRecordDirectory(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Dir(checkout)), 0o755); err != nil {
		t.Fatal(err)
	}
	// A file where the directory should be cannot be listed.
	if err := os.WriteFile(Dir(checkout), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, launched, err := Launched(checkout, Request{Machine: "m1f"}); err == nil || launched {
		t.Fatalf("Launched = %v, %v, want the listing error", launched, err)
	}
}

func TestLaunchedResumeIsARepeatOnlyForADoneLaunchWhoseCloneIsThere(t *testing.T) {
	t.Parallel()
	checkout, clone := launchedBed(t)
	gone := filepath.Join(filepath.Dir(clone), "removed-clone")
	records := []Record{
		{Launch: launchID, Machine: "m1f", Destination: clone, Outcome: OutcomeDone},
		{Launch: secondLaunch, Machine: "m1f", Destination: clone, Outcome: OutcomeArmed},
		{Launch: "01M3BQ0000000000000000000C", Machine: "m1f", Destination: gone, Outcome: OutcomeDone},
	}
	for _, record := range records {
		if err := Save(checkout, record); err != nil {
			t.Fatal(err)
		}
	}
	for index, want := range []bool{true, false, false} {
		record, launched, err := Launched(checkout, Request{Machine: "m1f", Resume: records[index].Launch})
		if err != nil || launched != want || record.Launch != records[index].Launch {
			t.Fatalf("resume %s: Launched = %+v, %v, %v; want launched %v", records[index].Launch, record, launched, err, want)
		}
	}
	if _, launched, err := Launched(checkout, Request{Machine: "m1f", Resume: "01M3BQ0000000000000000000Z"}); err == nil || launched {
		t.Fatalf("resume of an unrecorded launch = %v, %v, want the load error", launched, err)
	}
	if _, launched, err := Launched(checkout, Request{Machine: "m1f", Resume: "../escape"}); err == nil || launched {
		t.Fatalf("resume of an invalid id = %v, %v, want a refusal", launched, err)
	}
}
