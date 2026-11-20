package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The supervisor starts the real process group, retains its child and measures
// the adapter result. Two shells stand in for the provider wrapper and main.
// The handoff command is their descendant and classifies its actual parent.
func recordWrappedSeatHandoff(t *testing.T, b *workBed, seat steward.SeatRecord, now time.Time) {
	t.Helper()
	root := b.root()
	executable, err := os.Executable()
	contextMust(t, err)
	contextMust(t, os.Symlink(executable, filepath.Join(root, "capture-command")))
	wrapper := filepath.Join(t.TempDir(), "claude")
	const mainScript = `./capture-command -test.run='^TestMachineReviveCleanEnd$' capture-wrapped-seat "$1" >&2
exit $?
`
	contextMust(t, os.WriteFile(filepath.Join(root, "main.sh"), []byte(mainScript), 0600))
	at := now.Add(-90 * time.Second).Format(time.RFC3339Nano)
	script := "#!/bin/sh\n/bin/sh main.sh " + at + " || exit 1\nprintf '%s\\n' '{\"is_error\":false,\"session_id\":\"wrapped-session\",\"result\":\"Handoff recorded.\"}'\n"
	contextMust(t, testexec.WriteFile(wrapper, []byte(script), 0700))
	projects := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(projects, "wrapped-session.jsonl"), nil, 0600)
	manager := *b.manager
	manager.Prober = identity.KernelProber{}
	manager.Processes = launch.OSProcesses{Prober: manager.Prober}
	manager.Adapters = map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: wrapper, ProjectsRoot: projects}}
	metadata, err := json.Marshal(map[string]string{"brief": filepath.Join(root, b.brief("seat.md", "Hand off the next step.")), "model": "fixture", "fenceRoot": root})
	contextMust(t, err)
	start := launch.Record{ID: seat.LaunchID, Kind: "seat", Adapter: "claude-headless", State: launch.Starting, WorkingDirectory: root, StartedAt: seat.StartedAt}
	contextMust(t, json.Unmarshal(metadata, &start.AdapterData))
	contextMust(t, manager.Store.Create(start))
	record, err := manager.Supervise(start.ID)
	if err != nil || record.State != launch.Completed {
		dir, _ := manager.Store.StateDir(start.ID)
		log, _ := os.ReadFile(filepath.Join(dir, "stderr.log"))
		t.Fatalf("wrapped launch: %s %s %v\n%s", record.State, record.Reason, err, log)
	}
	intents, err := steward.LiveIntents(root)
	contextMust(t, err)
	if len(intents) != 1 || intents[0].Handoff == nil || record.Child == nil || record.Child.Pid == intents[0].Handoff.Predecessor.Pid {
		t.Fatal("the launch and handoff did not retain distinct wrapper and main identities")
	}
}

func captureWrappedSeat(t *testing.T, atText string) {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, atText)
	contextMust(t, err)
	root, err := os.Getwd()
	contextMust(t, err)
	main, live, err := (identity.KernelProber{}).ReadStart(int64(os.Getppid()))
	if err != nil || live != identity.Alive {
		t.Fatalf("read announced main: %s %v", live, err)
	}
	_, err = lease.AnnounceWithPair(root, "initial-session", main.Pid, main.StartedAt.Unix(), main.StartTicks, main.BootID, "claude-main", "claude", steward.SeatLineage)
	contextMust(t, err)
	holder, err := lease.CurrentHolder(root)
	contextMust(t, err)
	contextMust(t, lease.AssociateSession(root, holder.MainId, "wrapped-session", "start", "compact"))
	installation, err := stateroot.ParseInstallation(root)
	contextMust(t, err)
	caller, err := contextHandoffCallerWithMachine(installation, root, goal.ResolveMachine)
	contextMust(t, err)
	memory := filepath.Join(root, "memory")
	note := filepath.Join(memory, "lessons.md")
	writeContextHandoffNote(t, note, "Collect the result and continue.\n", at.Add(-time.Second))
	_, err = steward.HandoffWithWorkReader(root, caller, steward.HandoffRecord{NotePath: note, NoteDirectory: memory}, at,
		filepath.Join(memory, "receipts.log"), goal.ReadClaimableBudgetedWork)
	contextMust(t, err)
}
