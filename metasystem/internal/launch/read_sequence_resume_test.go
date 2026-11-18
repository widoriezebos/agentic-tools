package launch

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func TestResumingReadPreparesOutputsOnlyForANewLaunch(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		state UnitStepState
	}{
		{"running", StepRunning},
		{"passed", StepPassed},
		{"starting with a launch", StepStarting},
		{"pending without a launch", StepPending},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := baseUnitFixture(t)
			key := "resume" + filepath.Base(filepath.Dir(t.TempDir()))
			owner := diskstore.Owner{Kind: diskstore.OwnerUnit, Ref: key}
			directory, err := diskstore.CreateTempStore(diskstore.UnitReadFindingsName(key), diskstore.UnitReadFindingsClass, owner)
			if err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, "report.md")
			before := []byte("read findings\n\x00retained bytes\n")
			if err := os.WriteFile(output, before, 0o600); err != nil {
				t.Fatal(err)
			}
			diff := writeLaunchFile(t, "read.diff", sampleDiff())
			round := UnitRound{Directory: t.TempDir(), Steps: []UnitStep{{Name: "read", State: test.state, Mode: "whole"}}}
			const launchID = "resumed-read"
			if test.state != StepPending {
				record := seed(t, fixture.manager, launchID, Running)
				if _, err := fixture.manager.Store.Update(record.ID, func(record *Record) error {
					record.Kind = "read"
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				round.Steps[0].LaunchID = launchID
				if test.state != StepStarting {
					round.Steps[0].LaunchIDs = []string{launchID}
				}
			}
			sequence := readSequence{serial: true, diff: diff,
				spec: StartSpec{WorkingDirectory: fixture.worktree, Brief: brief(t), Outputs: []string{output}},
				driver: stepDriver{manager: fixture.manager, round: &round, unit: true,
					launchID: func(int) string { return launchID },
					save:     func() error { return nil },
					snapshot: func(string) (repositorySnapshot, error) { return repositorySnapshot{}, nil },
					start: func(spec StartSpec) (Record, error) {
						if _, err := os.Stat(output); !os.IsNotExist(err) {
							t.Fatalf("new launch found stale output: %v", err)
						}
						if info, err := os.Stat(directory); err != nil || !info.IsDir() {
							t.Fatalf("new launch's output directory was not prepared: %v", err)
						}
						return fixture.manager.Start(spec)
					},
				}}
			_, capped, err := sequence.run(0, fixture.manager.Now())
			if err != nil {
				t.Fatal(err)
			}
			wantStarts := 0
			if test.state == StepPending {
				wantStarts = 1
			} else {
				after, err := os.ReadFile(output)
				if err != nil || !bytes.Equal(before, after) {
					t.Errorf("resuming changed declared output: before=%q after=%q err=%v", before, after, err)
				}
			}
			if wantCapped := test.state == StepRunning || test.state == StepStarting; capped != wantCapped {
				t.Errorf("resume capped=%v, want %v", capped, wantCapped)
			}
			launches, err := fixture.manager.Store.List()
			if err != nil || len(launches) != 1 || launches[0].ID != launchID || len(fixture.starter.ids) != wantStarts || !slices.Equal(round.Steps[0].LaunchIDs, []string{launchID}) {
				t.Fatalf("resume launches=%+v starts=%v step=%+v err=%v", launches, fixture.starter.ids, round.Steps[0], err)
			}
		})
	}
}
