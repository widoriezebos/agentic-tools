package launch

// What the sequencer does, step by step, with nothing real behind it.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryStepRunsItsOwnersCommandInOrder(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if record.Outcome != OutcomeDone {
		t.Fatalf("outcome = %q, want %q", record.Outcome, OutcomeDone)
	}
	want := []string{
		"git clone --quiet " + fromRoot + " " + destRoot,
		"git -C " + destRoot + " remote set-url origin " + originURL,
		"git -C " + fromRoot + " config --local --list -z",
		"git -C " + destRoot + " config goal.sync-remote origin",
		"git -C " + destRoot + " config goal.human.wido Wido <wido@example.invalid>",
		"git -C " + destRoot + " rev-parse HEAD",
		"git -C " + destRoot + " fetch --no-tags origin",
		"go-build.sh",
		"metasystem config get --key evidence.root --conf " + filepath.Join(fromRoot, install, "metasystem.conf"),
		"metasystem validate session-isolation --source-root " + fromRoot + " --destination-root " + destRoot +
			" --manifest " + filepath.Join(destInstall(), "artifacts", "agents", "ui", "local-config-paths") +
			" --harness-root " + filepath.Join(fromRoot, install),
		"metasystem config validate --conf " + filepath.Join(destInstall(), "metasystem.conf") + " --repo " + destRoot,
		"git -C " + destRoot + " config --get metasystem.goal.machine",
		"git -C " + destRoot + " config metasystem.goal.machine " + machineName,
		"metasystem goal fetch --root " + destRoot,
		"metasystem goal next --root " + destRoot,
		"metasystem steward arm --repo " + destInstall() + " --temporary-human-word " + humanWord + " --review-by " + reviewBy,
		"metasystem up --repo " + destInstall() + " --recover-only --if-down",
	}
	got := built.runner.keys()
	if len(got) != len(want) {
		t.Fatalf("commands =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for index, key := range want {
		if got[index] != key {
			t.Fatalf("command %d = %q, want %q", index, got[index], key)
		}
	}
	// user.name is this checkout's git identity and not a fleet endpoint or a
	// human of the ledger, so it is not one of the keys a machine inherits.
	if built.runner.ranCommand("git -C " + destRoot + " config user.name fixture") {
		t.Fatal("the clone was given a key that is not one of the fleet's")
	}
}

func TestTheBuildRunsTheKitsOwnBuildScriptUnderItsOwnBudget(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, command := range built.runner.ran {
		if filepath.Base(command.Name) != "go-build.sh" {
			continue
		}
		if command.Name != filepath.Join(destInstall(), "scripts", "agents", "go-build.sh") {
			t.Fatalf("build script = %q", command.Name)
		}
		if command.Dir != destInstall() {
			t.Fatalf("build ran in %q, want %q", command.Dir, destInstall())
		}
		if command.Budget != built.sequencer.BuildBudget {
			t.Fatalf("build budget = %v, want %v", command.Budget, built.sequencer.BuildBudget)
		}
		return
	}
	t.Fatal("the build step ran no build script")
}

func TestEveryStepRunsUnderABoundAndNoOtherShell(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, command := range built.runner.ran {
		if command.Budget <= 0 {
			t.Fatalf("%s runs under no bound", commandKey(command))
		}
		name := filepath.Base(command.Name)
		if name != "git" && name != "metasystem" && name != "go-build.sh" {
			t.Fatalf("the verb ran %q, which is neither git, the engine, nor the designated build owner", command.Name)
		}
	}
}

func TestTheScrubbedEnvironmentDropsEveryMetasystemVariable(t *testing.T) {
	t.Parallel()
	kept := Scrubbed([]string{"PATH=/usr/bin", "METASYSTEM_EVIDENCE_ROOT=/elsewhere", "HOME=/home/wido", "METASYSTEM_FIXTURE=1"})
	want := []string{"PATH=/usr/bin", "HOME=/home/wido"}
	if len(kept) != len(want) {
		t.Fatalf("environment = %v, want %v", kept, want)
	}
	for index, entry := range want {
		if kept[index] != entry {
			t.Fatalf("environment = %v, want %v", kept, want)
		}
	}
}

func TestAStepStopsAtTheFirstRefusalWithTheOwnersWords(t *testing.T) {
	t.Parallel()
	const fence = "go-build: refused: the gate fence holds this checkout"
	built := newWorld(request())
	built.runner.refused["go-build.sh"] = fence
	record, err := built.sequencer.Run(fresh())
	if err == nil || err.Error() != fence {
		t.Fatalf("error = %v, want the fence's own words", err)
	}
	if record.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q", record.Outcome, OutcomeFailed)
	}
	step, found := record.StepOf(StepEngine)
	if !found || step.Outcome != StepFailed || step.Words != fence {
		t.Fatalf("engine step = %+v, want the fence's own words verbatim", step)
	}
	if _, ran := record.StepOf(StepConfiguration); ran {
		t.Fatal("the sequence went on after a refusal")
	}
	if record.EndedAt == nil {
		t.Fatal("a failed launch has no end")
	}
}

func TestTheWordNeverReachesTheRecord(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, written := range append(built.written, record) {
		if strings.Contains(recordText(t, written), humanWord) {
			t.Fatal("the human's authorization was written into the record")
		}
	}
	if !built.runner.ranCommand("metasystem steward arm --repo " + destInstall() +
		" --temporary-human-word " + humanWord + " --review-by " + reviewBy) {
		t.Fatal("the word did not reach the arming verb's argument list")
	}
}

func TestAResumeSkipsEveryStepWhosePostconditionHolds(t *testing.T) {
	t.Parallel()
	asked := request()
	asked.Resume = launchID
	built := newWorld(asked)
	built.host.exists[destRoot] = true
	built.host.exists[destBinary()] = true
	built.host.exists[filepath.Join(destInstall(), "metasystem.conf.local")] = true
	built.host.enrolled = true
	built.host.supervision = true
	built.runner.said["git -C "+destRoot+" config --get metasystem.goal.machine"] = machineName + "\n"
	delete(built.runner.refused, "git -C "+destRoot+" config --get metasystem.goal.machine")

	carried := fresh()
	carried.Created = Created{Destination: true, Nickname: true, EvidenceRoot: true}
	record, err := built.sequencer.Run(carried)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, name := range []string{StepClone, StepEngine, StepConfiguration, StepNickname, StepEnrollment, StepSupervision} {
		step, found := record.StepOf(name)
		if !found || step.Outcome != StepSkipped {
			t.Fatalf("%s = %+v, want skipped", name, step)
		}
	}
	// The two steps whose precondition is "always" run again, and so does the
	// presence observation: a resume asks the world, it does not trust a file.
	for _, name := range []string{StepTracking, StepLedger, StepPresence} {
		step, _ := record.StepOf(name)
		if step.Outcome != StepDone {
			t.Fatalf("%s = %+v, want done", name, step)
		}
	}
	if built.runner.ranCommand("git clone --quiet " + fromRoot + " " + destRoot) {
		t.Fatal("a resume cloned over the clone it made")
	}
	if built.runner.ranCommand("go-build.sh") {
		t.Fatal("a resume rebuilt an engine already stamped at this clone's HEAD")
	}
}

func TestAResumeRedoesTheStepWhosePostconditionDoesNotHold(t *testing.T) {
	t.Parallel()
	asked := request()
	asked.Resume = launchID
	built := newWorld(asked)
	built.host.exists[destRoot] = true
	built.host.exists[destBinary()] = true
	// The engine is there and was built from another commit, which is the
	// postcondition failing rather than the file being absent.
	built.host.stamp[destBinary()] = "0000000000000000000000000000000000000000"
	carried := fresh()
	carried.Created = Created{Destination: true}
	record, err := built.sequencer.Run(carried)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if step, _ := record.StepOf(StepEngine); step.Outcome != StepDone {
		t.Fatalf("engine = %+v, want it rebuilt", step)
	}
	if !built.runner.ranCommand("go-build.sh") {
		t.Fatal("a stamp that is not this clone's HEAD did not rebuild the engine")
	}
}

func TestAResumeThatHasToArmAgainAsksForTheWord(t *testing.T) {
	t.Parallel()
	asked := request()
	asked.Resume, asked.Word, asked.ReviewBy = launchID, "", ""
	built := newWorld(asked)
	built.host.exists[destRoot] = true
	built.host.exists[destBinary()] = true
	built.host.exists[filepath.Join(destInstall(), "metasystem.conf.local")] = true
	carried := fresh()
	carried.Created = Created{Destination: true, Nickname: true, EvidenceRoot: true}
	record, err := built.sequencer.Run(carried)
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeWordRequired {
		t.Fatalf("error = %v, want %s", err, CodeWordRequired)
	}
	if record.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q", record.Outcome, OutcomeFailed)
	}
}

func TestTheEvidenceRootIsTheSiblingOfThisSeatsEffectiveRoot(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(built.host.copied) != 1 {
		t.Fatalf("copied %d configurations, want one", len(built.host.copied))
	}
	copied := built.host.copied[0]
	if copied.source != filepath.Join(fromRoot, install, "metasystem.conf.local") {
		t.Fatalf("copied from %q", copied.source)
	}
	if copied.destRoot != filepath.Join(destInstall(), "metasystem.conf.local") {
		t.Fatalf("copied to %q", copied.destRoot)
	}
	if copied.evidenceRoot != "/w/evidence/m1f" {
		t.Fatalf("evidence root = %q, want the sibling of this seat's own", copied.evidenceRoot)
	}
}

func TestTheEvidenceRootRefusesThisSeatsOwnRoot(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	// A seat whose effective root is already named for this machineName would
	// have the new machine write its evidence into this seat's.
	built.runner.said["metasystem config get --key evidence.root --conf "+filepath.Join(fromRoot, install, "metasystem.conf")] = "/w/evidence/m1f\n"
	_, err := built.sequencer.Run(fresh())
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeEvidenceRootUnsafe {
		t.Fatalf("error = %v, want %s", err, CodeEvidenceRootUnsafe)
	}
}

func TestTheEvidenceRootRefusesADirectoryThatResolvesElsewhere(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.host.canonical["/w/evidence/m1f"] = "/somewhere/else"
	_, err := built.sequencer.Run(fresh())
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeEvidenceRootUnsafe {
		t.Fatalf("error = %v, want %s", err, CodeEvidenceRootUnsafe)
	}
	if !strings.Contains(refusal.Message, "/somewhere/else") {
		t.Fatalf("refusal = %q, want it to name where the link points", refusal.Message)
	}
}

func TestTheEvidenceRootRefusesADirectoryThisLaunchDidNotCreate(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.host.present["/w/evidence/m1f"] = true
	_, err := built.sequencer.Run(fresh())
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeEvidenceRootUnsafe {
		t.Fatalf("error = %v, want %s", err, CodeEvidenceRootUnsafe)
	}
}

func TestACloneIsRefusedWhenTheDiskWouldNotHoldIt(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.host.size = 4 << 30
	built.host.free = 6 << 30
	_, err := built.sequencer.Run(fresh())
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeDiskShort {
		t.Fatalf("error = %v, want %s", err, CodeDiskShort)
	}
	if built.runner.ranCommand("git clone --quiet " + fromRoot + " " + destRoot) {
		t.Fatal("the clone ran although the disk would not hold it")
	}
}

func TestPresenceIsReadThroughTheSeatTransportOnTheInjectedClock(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.presence.appearsAt = 2
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	step, _ := record.StepOf(StepPresence)
	if step.Outcome != StepDone {
		t.Fatalf("presence = %+v, want done", step)
	}
	if built.presence.looks != 3 {
		t.Fatalf("looked %d times, want the first look and two ticks", built.presence.looks)
	}
	if len(built.clock.waited) != 2 || built.clock.waited[0] != built.sequencer.PresenceTick {
		t.Fatalf("waited %v, want two of the new machine's ticks", built.clock.waited)
	}
	if len(built.presence.forgotten) != 1 || built.presence.forgotten[0] != built.sequencer.Namespace {
		t.Fatalf("forgot %v, want this launch's own namespace", built.presence.forgotten)
	}
}

func TestAMachineThatPublishesNothingIsArmedAndNotFailed(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.presence.appearsAt = -1
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if record.Outcome != OutcomeArmed {
		t.Fatalf("outcome = %q, want %q", record.Outcome, OutcomeArmed)
	}
	step, _ := record.StepOf(StepPresence)
	if step.Outcome != StepArmed {
		t.Fatalf("presence = %+v, want armed", step)
	}
	if !strings.Contains(step.Words, "within three ticks") || !strings.Contains(step.Words, destRoot) {
		t.Fatalf("presence words = %q, want the deadline and where to read the health", step.Words)
	}
	if built.presence.looks != 4 {
		t.Fatalf("looked %d times, want the first look and three ticks", built.presence.looks)
	}
}

func TestTheRecordCarriesWhatTheMachineIsAndWhatToDoWithIt(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if record.Machine != machineName || record.Destination != destRoot {
		t.Fatalf("record = %+v", record)
	}
	if record.ClonedCommit != headCommit || record.BuiltStamp != headCommit {
		t.Fatalf("cloned %q built %q, want this clone's HEAD", record.ClonedCommit, record.BuiltStamp)
	}
	if record.Orientation != "g1-s44 is claimable here" {
		t.Fatalf("orientation = %q", record.Orientation)
	}
	if record.ReviewBy != reviewBy {
		t.Fatalf("reviewBy = %q", record.ReviewBy)
	}
	if record.Next.Session != "cd "+destRoot+" && claude" {
		t.Fatalf("session command = %q", record.Next.Session)
	}
	if record.Next.Stop != "metasystem stop --repo "+destInstall() {
		t.Fatalf("stop command = %q", record.Next.Stop)
	}
	if !record.Created.Destination || !record.Created.Nickname || !record.Created.EvidenceRoot {
		t.Fatalf("created = %+v, want everything this launch made", record.Created)
	}
}

func TestTheRecordIsRewrittenAfterEveryStep(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	// One write before anything runs, one for the destRoot this launch is
	// about to create, one after each of the nine steps, and one at the end.
	if len(built.written) != 12 {
		t.Fatalf("wrote the record %d times", len(built.written))
	}
	if len(built.written[0].Steps) != 0 || built.written[0].Outcome != OutcomeRunning {
		t.Fatalf("the first write = %+v, want a running launch with no step", built.written[0])
	}
}

func recordText(t *testing.T, record Record) string {
	t.Helper()
	var built strings.Builder
	built.WriteString(record.Machine + record.Destination + record.Orientation + record.ReviewBy)
	built.WriteString(record.Next.Session + record.Next.Stop)
	for _, step := range record.Steps {
		built.WriteString(step.Step + step.Outcome + step.Words)
	}
	return built.String()
}
