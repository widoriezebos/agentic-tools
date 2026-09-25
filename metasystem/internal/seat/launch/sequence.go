package launch

// The sequencer: the fleet-join design's steps, in order, as subprocesses of
// the owners that already know how to do them.
//
// Three rules shape every step below.
//
// One owner per step. The clone is git's, the build is the kit's one fenced
// build script's, the configuration is the validator's, the ledger is `goal
// fetch`'s, the enrollment is `steward arm`'s and the supervision is `up
// --recover-only --if-down`'s. Nothing here re-decides what any of them
// decides, and a refusal is recorded in that owner's own words: a gate fence
// that refuses the new clone's build is a sentence a human reads, not a code
// this package invented for it.
//
// A precondition is also a postcondition. Each step states what must be true
// for it to be worth running; a resume verifies the same statement for every
// step the record says is done, and redoes what does not hold. That is what
// makes `--resume` safe to press twice and what keeps it from trusting a
// record over the world.
//
// A scrubbed environment. Every subprocess runs without the launching
// process's METASYSTEM_* variables, so an inherited METASYSTEM_EVIDENCE_ROOT
// cannot outrank the configuration this verb just copied — the interface's
// own spawn inherits its environment, and configuration precedence puts the
// environment above the file.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Command is one subprocess: where it runs, what it runs, and the bound it
// runs under. Every one of them runs on an owned process group.
type Command struct {
	Dir    string
	Name   string
	Args   []string
	Budget time.Duration
}

// Runner runs one command under a scrubbed environment and answers what it
// printed. The error's message is the owner's own words, which is what the
// record keeps and the card shows.
type Runner interface {
	Run(Command) (string, error)
}

// Host is everything the sequencer asks about this host that is not a
// subprocess. It is an interface so the unit tests drive every precondition
// without a filesystem.
type Host interface {
	// Exists reports whether a path is there.
	Exists(path string) (bool, error)
	// Canonical resolves a path's symlinks.
	Canonical(path string) (string, error)
	// MakeDir creates a directory and reports whether this call created it.
	MakeDir(path string) (created bool, err error)
	// CopyLocalConf copies one seat's metasystem.conf.local to another as
	// bytes, rewriting evidence.root through the engine's own conf writer,
	// and publishes it atomically.
	CopyLocalConf(source, destination, evidenceRoot string) error
	// MakeManifest writes the adapter-declared local configuration paths
	// where `validate session-isolation` reads them.
	MakeManifest(path string) error
	// Stamp is the source commit an installed engine was built from.
	Stamp(binary string) (string, error)
	// Enrolled reports whether an installation carries a steward identity
	// whose enrolled digest is the binary now installed there. An identity
	// that names other bytes is not an enrollment of this engine, which is
	// why the digest and not the identity's presence is the question.
	Enrolled(installation string) bool
	// SupervisionAlive reports whether the supervision owner is alive at an
	// installation, from what the steward last recorded there.
	SupervisionAlive(installation string) bool
	// Free is the free bytes at a directory, and Size the bytes one tree
	// takes.
	Free(directory string) (uint64, error)
	Size(root string) (uint64, error)
}

// Presence is the seat transport, as the presence step uses it: one fetch
// namespace of this launch's own, and the deletion that ends it.
type Presence interface {
	// Look fetches the fleet's presence into the namespace and reports
	// whether a record for the machine is in it.
	Look(namespace, machine string) (bool, error)
	// Forget deletes the namespace.
	Forget(namespace string) error
}

// Clock is the sequencer's own time. The presence wait ticks on it, so a test
// drives three ticks without three of anything elapsing.
type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

// Sequencer runs one launch. Everything it needs from the world is a field,
// so the unit tests are the sequencer with fakes in every one of them.
type Sequencer struct {
	Request Request
	// Installation is where the engine lives inside a checkout of this
	// repository, relative to the checkout root. It is read from the
	// launching seat's own layout, because the clone is a clone of it.
	Installation string
	// OriginURL is this checkout's ledger remote URL, which the clone's own
	// origin is set to: the clone comes from this checkout's objects and
	// then talks to the fleet's remote.
	OriginURL string
	Host      Host
	Runner    Runner
	Presence  Presence
	Clock     Clock
	// Write persists the record after every step. A launch that cannot write
	// its record is a launch nothing can report on, so the failure stops it.
	Write func(Record) error
	// Namespace is this launch's own presence fetch namespace, deleted when
	// the presence step ends.
	Namespace string
	// GitBudget bounds git and the network — the ledger fetch's own budget —
	// and BuildBudget the one build.
	GitBudget   time.Duration
	BuildBudget time.Duration
	// PresenceTick is one of the new machine's ticks and PresenceTicks how
	// many of them the wait allows. The tick is read from the configuration
	// this launch copied, so it is the new machine's cadence by construction.
	PresenceTick  time.Duration
	PresenceTicks int
}

// stepRun is what one step did: how it ended, and the words it ended with.
type stepRun struct {
	outcome string
	words   string
}

// Run runs the whole sequence against a record and answers the record as it
// finished.
//
// It stops at the first refusal: a launch whose clone failed has nothing to
// build, and a step that ran after one would be a step reporting on a machine
// that is not there. The error it answers is the refusal itself, so a caller
// exits with the last step's outcome.
func (s *Sequencer) Run(record Record) (Record, error) {
	record.SchemaVersion = SchemaVersion
	record.Machine = s.Request.Machine
	record.Destination = filepath.Clean(s.Request.Destination)
	record.Outcome = OutcomeRunning
	record.ReviewBy = s.Request.ReviewBy
	record.Next = Next{
		Session: "cd " + record.Destination + " && claude",
		Stop:    "metasystem stop --repo " + s.install(record.Destination),
	}
	if err := s.persist(&record); err != nil {
		return record, err
	}

	for _, name := range Steps {
		ran, err := s.step(name, &record)
		at := s.Clock.Now().UTC().Format(time.RFC3339)
		if err != nil {
			record.SetStep(Step{Step: name, Outcome: StepFailed, At: at, Words: err.Error()})
			record.Outcome = OutcomeFailed
			ended := at
			record.EndedAt = &ended
			_ = s.persist(&record)
			return record, err
		}
		record.SetStep(Step{Step: name, Outcome: ran.outcome, At: at, Words: ran.words})
		if ran.outcome == StepArmed {
			record.Outcome = OutcomeArmed
		}
		if err := s.persist(&record); err != nil {
			return record, err
		}
	}
	if record.Outcome == OutcomeRunning {
		record.Outcome = OutcomeDone
	}
	ended := s.Clock.Now().UTC().Format(time.RFC3339)
	record.EndedAt = &ended
	if err := s.persist(&record); err != nil {
		return record, err
	}
	return record, nil
}

func (s *Sequencer) persist(record *Record) error {
	if s.Write == nil {
		return nil
	}
	return s.Write(*record)
}

// step dispatches one step by name.
func (s *Sequencer) step(name string, record *Record) (stepRun, error) {
	switch name {
	case StepClone:
		return s.clone(record)
	case StepTracking:
		return s.tracking(record)
	case StepEngine:
		return s.engine(record)
	case StepConfiguration:
		return s.configuration(record)
	case StepNickname:
		return s.nickname(record)
	case StepLedger:
		return s.ledger(record)
	case StepEnrollment:
		return s.enrollment(record)
	case StepSupervision:
		return s.supervision(record)
	case StepPresence:
		return s.presence(record)
	}
	return stepRun{}, fmt.Errorf("unknown launch step %q", name)
}

/* --------------------------------------------------------------- 1 clone -- */

// clone is the machine itself: a full clone of this checkout's objects, with
// its origin pointed at the fleet's remote and this checkout's endpoint and
// human keys copied onto it.
//
// It comes from this checkout rather than from the remote because that is
// fast, offline and prompts for no credential; the tracking step after it is
// what makes the clone read the fleet's own tip.
func (s *Sequencer) clone(record *Record) (stepRun, error) {
	destination := record.Destination
	present, err := s.Host.Exists(destination)
	if err != nil {
		return stepRun{}, err
	}
	if present && record.Created.Destination {
		// The postcondition, verified rather than believed: a directory this
		// launch made is a clone only if git can name its HEAD.
		head, err := s.git(destination, "rev-parse", "HEAD")
		if err == nil {
			record.ClonedCommit = strings.TrimSpace(head)
			return stepRun{outcome: StepSkipped, words: "the clone this launch made is at " + record.ClonedCommit}, nil
		}
	}
	if err := s.room(destination); err != nil {
		return stepRun{}, err
	}
	if _, err := s.run(Command{Name: "git", Args: []string{"clone", "--quiet", s.Request.From, destination}, Budget: s.GitBudget}); err != nil {
		return stepRun{}, err
	}
	record.Created.Destination = true
	if err := s.persist(record); err != nil {
		return stepRun{}, err
	}
	if _, err := s.git(destination, "remote", "set-url", "origin", s.OriginURL); err != nil {
		return stepRun{}, err
	}
	if err := s.copyKeys(destination); err != nil {
		return stepRun{}, err
	}
	head, err := s.git(destination, "rev-parse", "HEAD")
	if err != nil {
		return stepRun{}, err
	}
	record.ClonedCommit = strings.TrimSpace(head)
	return stepRun{outcome: StepDone}, nil
}

// room refuses a clone that would not fit: free space at the destination's
// parent against twice the size of the checkout being cloned.
//
// Twice, because a clone is the working tree and the objects, and a launch
// that filled the disk halfway through would leave a directory nothing can
// finish and a host nothing else can write to either.
func (s *Sequencer) room(destination string) error {
	size, err := s.Host.Size(s.Request.From)
	if err != nil {
		return err
	}
	free, err := s.Host.Free(filepath.Dir(destination))
	if err != nil {
		return err
	}
	if free < 2*size {
		return refuse(CodeDiskShort,
			"%s has %d MB free and a clone of %s needs about %d MB",
			filepath.Dir(destination), free/(1<<20), s.Request.From, (2*size)/(1<<20))
	}
	return nil
}

// endpointKeys are the git configuration keys a machine of this fleet needs
// and a fresh clone does not inherit: where the ledger is, which branch it
// lands on, which ref the steward treats as landed, how this host notifies,
// and who the humans are.
//
// They are copied rather than defaulted because each of them is this fleet's
// own answer, and a machine that guessed one would talk to a different ledger
// or answer to a different human.
var endpointKeys = []string{
	"goal.sync-remote",
	"goal.sync-branch",
	"metasystem.steward.landing-ref",
	"metasystem.steward.notify-command",
}

// copyKeys reads this checkout's own local configuration and sets the keys a
// machine needs on the clone.
func (s *Sequencer) copyKeys(destination string) error {
	listed, err := s.git(s.Request.From, "config", "--local", "--list", "-z")
	if err != nil {
		return err
	}
	for _, key := range copied(listed) {
		if _, err := s.git(destination, "config", key.name, key.value); err != nil {
			return err
		}
	}
	return nil
}

type configKey struct{ name, value string }

// copied picks the keys a machine needs out of one `git config --list -z`
// reading. NUL separates the entries and a newline separates each entry's key
// from its value, so a value with a line break in it — a notify command is
// the one that has them — survives the reading whole.
func copied(listed string) []configKey {
	keys := []configKey{}
	for _, entry := range strings.Split(listed, "\x00") {
		name, value, found := strings.Cut(entry, "\n")
		if !found {
			continue
		}
		wanted := strings.HasPrefix(name, "goal.human.")
		for _, known := range endpointKeys {
			wanted = wanted || name == known
		}
		if wanted {
			keys = append(keys, configKey{name: name, value: value})
		}
	}
	return keys
}

/* ------------------------------------------------------------ 2 tracking -- */

// tracking gives the clone its own remote-tracking refs.
//
// It runs every time, because the ledger fetch of step 6 deliberately does
// not refresh them: the accepted-ref advance fetches into its own operation
// namespace, so refs/remotes/origin/main would never exist on a clone that
// only ever ran `goal fetch`.
func (s *Sequencer) tracking(record *Record) (stepRun, error) {
	if _, err := s.git(record.Destination, "fetch", "--no-tags", "origin"); err != nil {
		return stepRun{}, err
	}
	return stepRun{outcome: StepDone}, nil
}

/* -------------------------------------------------------------- 3 engine -- */

// engine builds the new machine's own engine through the kit's one fenced,
// stamped build script, which is the designated build owner named in
// docs/architecture.md.
func (s *Sequencer) engine(record *Record) (stepRun, error) {
	binary := s.binary(record.Destination)
	present, err := s.Host.Exists(binary)
	if err != nil {
		return stepRun{}, err
	}
	if present {
		stamp, err := s.Host.Stamp(binary)
		if err == nil && stamp != "" && stamp == record.ClonedCommit {
			record.BuiltStamp = stamp
			return stepRun{outcome: StepSkipped, words: "the engine at " + binary + " is stamped " + stamp}, nil
		}
	}
	install := s.install(record.Destination)
	if _, err := s.run(Command{
		Dir:    install,
		Name:   filepath.Join(install, "scripts", "agents", "go-build.sh"),
		Budget: s.BuildBudget,
	}); err != nil {
		return stepRun{}, err
	}
	stamp, err := s.Host.Stamp(binary)
	if err != nil {
		return stepRun{}, err
	}
	record.BuiltStamp = stamp
	return stepRun{outcome: StepDone}, nil
}

/* ------------------------------------------------------- 4 configuration -- */

// configuration gives the machine this seat's roster and secrets, its own
// evidence root, and the adapters' local files, and then has the new engine
// validate the configuration it will run under.
//
// The roster is copied as bytes and never parsed here. A machine is a seat of
// this fleet or it is nothing, and the fleet-join design's hand-edited
// template would stop a button halfway; what this verb rewrites is the one
// key it knows is wrong for a second machine, the evidence root, and a human
// edits the copy afterwards if the machine should differ in any other way.
func (s *Sequencer) configuration(record *Record) (stepRun, error) {
	destinationInstall := s.install(record.Destination)
	local := filepath.Join(destinationInstall, "metasystem.conf.local")
	present, err := s.Host.Exists(local)
	if err != nil {
		return stepRun{}, err
	}
	if present {
		return stepRun{outcome: StepSkipped, words: local + " is already written"}, nil
	}
	root, err := s.evidenceRoot(record)
	if err != nil {
		return stepRun{}, err
	}
	sourceInstall := s.install(s.Request.From)
	if err := s.Host.CopyLocalConf(filepath.Join(sourceInstall, "metasystem.conf.local"), local, root); err != nil {
		return stepRun{}, err
	}
	manifest := filepath.Join(destinationInstall, "artifacts", "agents", "ui", "local-config-paths")
	if err := s.Host.MakeManifest(manifest); err != nil {
		return stepRun{}, err
	}
	if _, err := s.run(Command{
		Dir: sourceInstall, Name: s.binary(s.Request.From), Budget: s.GitBudget,
		Args: []string{"validate", "session-isolation",
			"--source-root", s.Request.From, "--destination-root", record.Destination,
			"--manifest", manifest, "--harness-root", sourceInstall},
	}); err != nil {
		return stepRun{}, err
	}
	if _, err := s.run(Command{
		Dir: destinationInstall, Name: s.binary(record.Destination), Budget: s.GitBudget,
		Args: []string{"config", "validate",
			"--conf", filepath.Join(destinationInstall, "metasystem.conf"),
			"--repo", record.Destination},
	}); err != nil {
		return stepRun{}, err
	}
	return stepRun{outcome: StepDone, words: "the roster and the local configuration were copied from this seat; edit " + local + " if this machine should differ"}, nil
}

// evidenceRoot is the new machine's evidence root: the sibling of this seat's
// EFFECTIVE root, named for the nickname.
//
// The effective root is read through the engine rather than from the file,
// because the file is not the last word on it — an environment variable
// outranks it — and a launch that wrote a sibling of the wrong root would
// point the new machine's evidence at this seat's.
func (s *Sequencer) evidenceRoot(record *Record) (string, error) {
	sourceInstall := s.install(s.Request.From)
	printed, err := s.run(Command{
		Dir: sourceInstall, Name: s.binary(s.Request.From), Budget: s.GitBudget,
		Args: []string{"config", "get", "--key", "evidence.root",
			"--conf", filepath.Join(sourceInstall, "metasystem.conf")},
	})
	if err != nil {
		return "", err
	}
	mine := strings.TrimSpace(printed)
	if mine == "" {
		return "", refuse(CodeEvidenceRootUnsafe, "this seat's engine names no evidence root")
	}
	canonical, err := s.Host.Canonical(mine)
	if err != nil {
		return "", refuse(CodeEvidenceRootUnsafe, "this seat's evidence root %s could not be resolved: %v", mine, err)
	}
	sibling := filepath.Join(filepath.Dir(canonical), s.Request.Machine)
	if sibling == canonical {
		return "", refuse(CodeEvidenceRootUnsafe,
			"the evidence root for %s would be this seat's own root %s", s.Request.Machine, canonical)
	}
	created, err := s.Host.MakeDir(sibling)
	if err != nil {
		return "", refuse(CodeEvidenceRootUnsafe, "%s could not be created: %v", sibling, err)
	}
	if !created && !record.Created.EvidenceRoot {
		return "", refuse(CodeEvidenceRootUnsafe,
			"%s is already there and this launch did not create it", sibling)
	}
	// A directory that resolves somewhere else is a symlink pointing out of
	// the evidence tree, which would put this machine's evidence wherever it
	// points — including into this seat's own root.
	resolved, err := s.Host.Canonical(sibling)
	if err != nil || resolved != sibling {
		return "", refuse(CodeEvidenceRootUnsafe,
			"%s resolves to %s; an evidence root that is a link elsewhere is refused", sibling, resolved)
	}
	record.Created.EvidenceRoot = true
	return sibling, nil
}

/* ------------------------------------------------------------ 5 nickname -- */

// nickname is what makes the clone a machine: the one git key every other
// seat reads it by.
func (s *Sequencer) nickname(record *Record) (stepRun, error) {
	held, err := s.git(record.Destination, "config", "--get", "metasystem.goal.machine")
	if err == nil && strings.TrimSpace(held) == s.Request.Machine {
		record.Created.Nickname = true
		return stepRun{outcome: StepSkipped, words: record.Destination + " already carries " + s.Request.Machine}, nil
	}
	if _, err := s.git(record.Destination, "config", "metasystem.goal.machine", s.Request.Machine); err != nil {
		return stepRun{}, err
	}
	record.Created.Nickname = true
	return stepRun{outcome: StepDone}, nil
}

/* -------------------------------------------------------------- 6 ledger -- */

// ledger gives the machine the fleet's accepted tip and one orientation line.
//
// The fetch runs every time: a fresh clone validates the canonical tree and
// creates its accepted ref, and a resumed launch wants the tip as it stands
// now rather than as it stood when it was interrupted.
func (s *Sequencer) ledger(record *Record) (stepRun, error) {
	if _, err := s.run(Command{
		Dir: record.Destination, Name: s.binary(record.Destination), Budget: s.GitBudget,
		Args: []string{"goal", "fetch", "--root", record.Destination},
	}); err != nil {
		return stepRun{}, err
	}
	// The orientation is a reading and not a gate: a machine that joined a
	// ledger with nothing claimable is a machine that joined.
	printed, err := s.run(Command{
		Dir: record.Destination, Name: s.binary(record.Destination), Budget: s.GitBudget,
		Args: []string{"goal", "next", "--root", record.Destination},
	})
	if err == nil {
		record.Orientation = firstLine(printed)
	}
	return stepRun{outcome: StepDone}, nil
}

/* ---------------------------------------------------------- 7 enrollment -- */

// enrollment is the human's own act, carried: `steward arm` with the word
// they typed and the review date they chose.
//
// Arming mints the identity and starts the runner, so there is no second
// "start the steward" step. The word is passed as an argument and is never
// written into the record.
func (s *Sequencer) enrollment(record *Record) (stepRun, error) {
	install := s.install(record.Destination)
	if s.Host.Enrolled(install) {
		return stepRun{outcome: StepSkipped, words: install + " already carries an identity enrolled for the engine installed there"}, nil
	}
	args := []string{"steward", "arm", "--repo", install}
	if s.Request.Word != "" {
		args = append(args, "--temporary-human-word", s.Request.Word, "--review-by", s.Request.ReviewBy)
	} else if s.Request.Resuming() {
		// The record never held the word, by design, so a resume that has to
		// arm again asks for it rather than enrolling under something this
		// verb made up.
		return stepRun{}, refuse(CodeWordRequired,
			"this launch has to enroll %s and the record never held your authorization; give the word and the review date again", record.Machine)
	}
	if _, err := s.run(Command{Dir: install, Name: s.binary(record.Destination), Args: args, Budget: s.GitBudget}); err != nil {
		return stepRun{}, err
	}
	return stepRun{outcome: StepDone, words: "temporary enrollment, review due " + s.Request.ReviewBy}, nil
}

/* --------------------------------------------------------- 8 supervision -- */

// supervision starts what arming did not: the machinery-only form of `up`.
//
// Ordinary `up` announces a session and wants a runtime's ancestry, which a
// process this verb detached does not have. `--recover-only --if-down`
// authenticates the enrolled binary and starts the rings that are missing,
// announcing nothing.
func (s *Sequencer) supervision(record *Record) (stepRun, error) {
	install := s.install(record.Destination)
	if s.Host.SupervisionAlive(install) {
		return stepRun{outcome: StepSkipped, words: "supervision is already up at " + install}, nil
	}
	if _, err := s.run(Command{
		Dir: install, Name: s.binary(record.Destination), Budget: s.GitBudget,
		Args: []string{"up", "--repo", install, "--recover-only", "--if-down"},
	}); err != nil {
		return stepRun{}, err
	}
	return stepRun{outcome: StepDone}, nil
}

/* ------------------------------------------------------------ 9 presence -- */

// presence is the one confirmation that the machine joined the fleet rather
// than merely started: another seat can read its presence record.
//
// The verb reads it itself, through the seat transport, in a fetch namespace
// of its own. It does not consult the interface's copy: that fetcher runs
// only while a browser is open and starts at most once a minute, so a launch
// watched from a closed laptop would report a live machine as silent.
//
// Not seeing one is `armed` and not a failure. The machine is enrolled and
// its runner is up; what is missing is the confirmation, and the words send
// the human to the new machine's own health line.
func (s *Sequencer) presence(record *Record) (stepRun, error) {
	defer func() { _ = s.Presence.Forget(s.Namespace) }()
	// The nickname was refused if any presence ref already named it, so a
	// record under it now is this launch's own and there is no earlier
	// generation to tell it from.
	for look := 0; ; look++ {
		found, err := s.Presence.Look(s.Namespace, record.Machine)
		if err == nil && found {
			return stepRun{outcome: StepDone, words: record.Machine + " published presence"}, nil
		}
		if look >= s.PresenceTicks {
			return stepRun{outcome: StepArmed, words: "armed; presence not confirmed within " +
				words(s.PresenceTicks) + " ticks: read " + record.Destination + "'s health"}, nil
		}
		<-s.Clock.After(s.PresenceTick)
	}
}

/* ------------------------------------------------------------- the seams -- */

// install is the engine's own root inside a checkout of this repository.
func (s *Sequencer) install(checkout string) string {
	if s.Installation == "" || s.Installation == "." {
		return filepath.Clean(checkout)
	}
	return filepath.Join(checkout, s.Installation)
}

// binary is the engine installed in one checkout.
func (s *Sequencer) binary(checkout string) string {
	return filepath.Join(s.install(checkout), "bin", "metasystem")
}

// git runs one git invocation in a repository, under the network budget.
func (s *Sequencer) git(root string, args ...string) (string, error) {
	return s.run(Command{Name: "git", Args: append([]string{"-C", root}, args...), Budget: s.GitBudget})
}

func (s *Sequencer) run(command Command) (string, error) {
	return s.Runner.Run(command)
}

func firstLine(printed string) string {
	for _, line := range strings.Split(printed, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// words spells the small numbers the presence step says out loud, so the card
// reads "within three ticks" rather than "within 3 ticks".
func words(count int) string {
	switch count {
	case 1:
		return "one"
	case 2:
		return "two"
	case 3:
		return "three"
	}
	return fmt.Sprintf("%d", count)
}
