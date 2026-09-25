package launch

// What the sequencer does, step by step, with nothing real behind it.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		// The remote and the keys are read before they are written, so the
		// same code repairs a half-made clone on a resume and writes a fresh
		// one here.
		"git -C " + destRoot + " remote get-url origin",
		"git -C " + destRoot + " remote set-url origin " + originURL,
		"git -C " + fromRoot + " config --local --list -z",
		"git -C " + destRoot + " config --local --list -z",
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
		// The wait is counted in the NEW machine's ticks, so its own cadence
		// is read from the clone rather than assumed from this seat's.
		"git -C " + destInstall() + " config --get metasystem.steward.tick-seconds",
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
	built.host.exists[filepath.Join(destInstall(), "artifacts", "agents", "ui", "local-config-paths")] = true
	built.host.enrolled = true
	built.host.supervision = true
	built.runner.said["git -C "+destRoot+" config --get metasystem.goal.machine"] = machineName + "\n"
	delete(built.runner.refused, "git -C "+destRoot+" config --get metasystem.goal.machine")
	// The clone this launch made already carries the fleet's remote and its
	// keys, which is what the clone step's postcondition asks about.
	built.runner.said["git -C "+destRoot+" remote get-url origin"] = originURL + "\n"
	built.runner.said["git -C "+destRoot+" config --local --list -z"] =
		built.runner.said["git -C "+fromRoot+" config --local --list -z"]

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

// A clone whose HEAD answers is not a finished step: the origin and the keys
// come after it, and a launch killed between them leaves both undone.
func TestAResumeRepairsACloneWhoseOriginAndKeysNeverLanded(t *testing.T) {
	t.Parallel()
	asked := request()
	asked.Resume = launchID
	built := newWorld(asked)
	built.host.exists[destRoot] = true
	built.host.exists[destBinary()] = true
	carried := fresh()
	carried.Created = Created{Destination: true}

	record, err := built.sequencer.Run(carried)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	step, _ := record.StepOf(StepClone)
	if step.Outcome != StepDone || !strings.Contains(step.Words, "origin") {
		t.Fatalf("clone = %+v, want the missing origin repaired", step)
	}
	if !built.runner.ranCommand("git -C " + destRoot + " remote set-url origin " + originURL) {
		t.Fatal("a clone with the wrong origin was left pointing at this checkout")
	}
	if !built.runner.ranCommand("git -C " + destRoot + " config goal.sync-remote origin") {
		t.Fatal("a clone with no endpoint keys was left without them")
	}
	if built.runner.ranCommand("git clone --quiet " + fromRoot + " " + destRoot) {
		t.Fatal("the repair cloned over the clone it was repairing")
	}
}

// The configuration step is four operations. A resume may skip only the one
// that would overwrite a human's edit; the checks run again, because a launch
// killed before them has never had its configuration checked at all.
func TestAResumeChecksTheConfigurationItDidNotCopyAgain(t *testing.T) {
	t.Parallel()
	asked := request()
	asked.Resume = launchID
	built := newWorld(asked)
	built.host.exists[destRoot] = true
	built.host.exists[destBinary()] = true
	built.host.exists[filepath.Join(destInstall(), "metasystem.conf.local")] = true
	carried := fresh()
	carried.Created = Created{Destination: true, EvidenceRoot: true}

	record, err := built.sequencer.Run(carried)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if step, _ := record.StepOf(StepConfiguration); step.Outcome != StepSkipped {
		t.Fatalf("configuration = %+v, want skipped", step)
	}
	if len(built.host.copied) != 0 {
		t.Fatal("a resume copied over the configuration a human may have edited")
	}
	if len(built.host.manifests) != 1 {
		t.Fatalf("wrote %d manifests, want the absent one written", len(built.host.manifests))
	}
	for _, key := range []string{
		"metasystem validate session-isolation --source-root " + fromRoot + " --destination-root " + destRoot +
			" --manifest " + filepath.Join(destInstall(), "artifacts", "agents", "ui", "local-config-paths") +
			" --harness-root " + filepath.Join(fromRoot, install),
		"metasystem config validate --conf " + filepath.Join(destInstall(), "metasystem.conf") + " --repo " + destRoot,
	} {
		if !built.runner.ranCommand(key) {
			t.Fatalf("a resume did not run %q", key)
		}
	}
}

// The record says this launch made the evidence root before anything after it
// can fail, so a retry meets a directory its own record accounts for.
func TestTheEvidenceRootIsRecordedBeforeTheRestOfTheStepCanFail(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.runner.refused["metasystem config validate --conf "+
		filepath.Join(destInstall(), "metasystem.conf")+" --repo "+destRoot] = "the configuration is invalid"
	record, err := built.sequencer.Run(fresh())
	if err == nil {
		t.Fatal("the configuration step did not fail")
	}
	if !record.Created.EvidenceRoot {
		t.Fatal("the record does not say this launch created the evidence root")
	}
	for _, written := range built.written {
		if written.Created.EvidenceRoot {
			return
		}
	}
	t.Fatal("no write of the record carried the evidence root this launch created")
}

// A clone that already carries somebody else's nickname is not this launch's
// to rename.
func TestAClonesOwnNicknameIsNeverOverwritten(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.runner.said["git -C "+destRoot+" config --get metasystem.goal.machine"] = "m1c\n"
	delete(built.runner.refused, "git -C "+destRoot+" config --get metasystem.goal.machine")

	_, err := built.sequencer.Run(fresh())
	refusal := refusalOf(t, err)
	if refusal.Code != CodeNicknameTaken {
		t.Fatalf("code = %s, want %s", refusal.Code, CodeNicknameTaken)
	}
	if built.runner.ranCommand("git -C " + destRoot + " config metasystem.goal.machine " + machineName) {
		t.Fatal("a live machine's nickname was taken away from it")
	}
}

// Supervision is read from the owner lock, and the step proves it after it
// runs rather than assuming that up's return means a live owner.
func TestSupervisionIsProvedAfterTheRecoveryCommand(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.host.supervisionAfterUp = false
	_, err := built.sequencer.Run(fresh())
	refusal := refusalOf(t, err)
	if refusal.Code != CodeSupervisionDown {
		t.Fatalf("code = %s, want %s", refusal.Code, CodeSupervisionDown)
	}
	if !built.runner.ranCommand("metasystem up --repo " + destInstall() + " --recover-only --if-down") {
		t.Fatal("the recovery command never ran")
	}
}

func TestSupervisionAlreadyUpSkipsTheRecoveryCommand(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.host.supervision = true
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if step, _ := record.StepOf(StepSupervision); step.Outcome != StepSkipped {
		t.Fatalf("supervision = %+v, want skipped", step)
	}
	if built.runner.ranCommand("metasystem up --repo " + destInstall() + " --recover-only --if-down") {
		t.Fatal("the recovery command ran although the owner lock was already alive")
	}
}

// A record under the nickname is not enough: it has to be this machine's,
// which the repository identity and the generation say and a name does not.
func TestPresenceUnderTheNicknameIsNotConfirmedWithoutTheIdentity(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.presence.foreign = true
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if record.Outcome != OutcomeArmed {
		t.Fatalf("outcome = %q, want %q: another machine's record must not confirm this one", record.Outcome, OutcomeArmed)
	}
}

// The wait is counted in the new machine's own ticks, read from the clone
// after its configuration was written.
func TestThePresenceWaitTicksOnTheClonesOwnCadence(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.presence.appearsAt = -1
	built.runner.said["git -C "+destInstall()+" config --get metasystem.steward.tick-seconds"] = "120\n"

	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, waited := range built.clock.waited {
		if waited != 2*time.Minute {
			t.Fatalf("waited %v, want the clone's own 120 seconds", built.clock.waited)
		}
	}
	if len(built.clock.waited) != 3 {
		t.Fatalf("waited %v, want three of them", built.clock.waited)
	}
}

func TestThePresenceWaitFallsBackToThisSeatsCadence(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.presence.appearsAt = -1
	// A clone that names no cadence: git answers with an exit code.
	built.runner.refused["git -C "+destInstall()+" config --get metasystem.steward.tick-seconds"] = ""

	if _, err := built.sequencer.Run(fresh()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(built.clock.waited) == 0 || built.clock.waited[0] != built.sequencer.PresenceTick {
		t.Fatalf("waited %v, want this seat's own tick", built.clock.waited)
	}
}

// The cadence key travels with the endpoint keys, so the clone has one of its
// own to be waited on.
func TestTheCadenceKeyIsOneOfTheKeysAMachineInherits(t *testing.T) {
	t.Parallel()
	listed := "metasystem.steward.tick-seconds\n120\x00core.bare\nfalse\x00"
	keys := copied(listed)
	if len(keys) != 1 || keys[0].name != "metasystem.steward.tick-seconds" || keys[0].value != "120" {
		t.Fatalf("copied = %+v, want the cadence and nothing that is not the fleet's", keys)
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
	// One write before anything runs, one for the destination this launch is
	// about to create, one for the evidence root it creates, one after each
	// of the nine steps, and one at the end. The two mid-step writes are the
	// point: a launch killed after making either must come back to a record
	// that says it made it.
	if len(built.written) != 13 {
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
