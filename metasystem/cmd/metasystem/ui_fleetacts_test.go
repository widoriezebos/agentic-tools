package main

// The fleet panel's Pause, Resume and Stop as the signed-in person
// (fleet-panel-ux-step2.md slice 2b, design D1; ruling R-142-ui), at the
// seams the server fills: each runs its public verb in this process with
// owners whose person and classify answer the session's human, so a verb that
// asks a person is answered by the session and not by the server's own
// launcher, which is machinery. Stop's admission reads this computer's
// machines as machine stop reads them and refuses the checkout serving the
// page before any stop runs (S2-01); a machine already stopped is admitted, so
// a second Stop is the verb's unchanged success with nothing written (S2-02).
//
// The verbs run on the beds their own tests use: the lane verb bed, and the
// machine bed whose processes are fixture items no signal reaches.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

// A signed-in Pause runs landing stop with the session's human as who paused
// the lane, which the pause file records; a second Pause is the verb's
// unchanged success.
func TestUIPauseRunsLandingStopForTheSessionsHuman(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 {
		t.Fatalf("set = %d %q %q", code, stdout, stderr)
	}
	acts := uiFleetActs{checkout: bed.cwd, owners: bed.owners}

	answered, err := acts.pause("Wido")

	if err != nil || answered.Outcome != intentConfirmed || !strings.HasPrefix(answered.Summary, "stopped the landing lane for Wido") {
		t.Fatalf("pause = %+v %v; want the lane stopped for Wido", answered, err)
	}
	if pause, paused := lane.ReadPause(bed.home); !paused || pause.By != "Wido" {
		t.Fatalf("the pause file = %+v (paused %v); want it written by Wido, the session's human", pause, paused)
	}
	again, err := acts.pause("Wido")
	if err != nil || again.Outcome != intentUnchanged || !strings.Contains(again.Summary, "already stopped by Wido") {
		t.Fatalf("pause again = %+v %v; want the verb's unchanged success", again, err)
	}
}

// A signed-in Resume runs landing start with the session's human as the
// person it asks for. At the server's own shell the same verb is refused,
// because the shell that started the server is no person; through the session
// the lane resumes and its pause file is gone. A resume of a running lane is
// the verb's unchanged success.
func TestUIResumeRunsLandingStartWithTheSessionAsThePerson(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 {
		t.Fatalf("set = %d %q %q", code, stdout, stderr)
	}
	if code, stdout, stderr := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 {
		t.Fatalf("stop = %d %q %q", code, stdout, stderr)
	}
	bed.person = errors.New("MetaSystem's own machinery started this shell")
	if code, _, stderr := bed.run(t, "landing", "start"); code == 0 || !strings.Contains(oneSpaced(stderr), "only a person may resume the landing lane") {
		t.Fatalf("start at the server's own shell = %d %q; want the person's refusal", code, stderr)
	}
	acts := uiFleetActs{checkout: bed.cwd, owners: bed.owners}

	answered, err := acts.resume("Wido")

	if err != nil || answered.Outcome != intentConfirmed || answered.Summary != "resumed the landing lane at "+bed.landingA {
		t.Fatalf("resume = %+v %v; want the lane resumed", answered, err)
	}
	if pause, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("the pause file is still there after a resume: %+v", pause)
	}
	again, err := acts.resume("Wido")
	if err != nil || again.Outcome != intentUnchanged || !strings.Contains(again.Summary, "is already running") {
		t.Fatalf("resume again = %+v %v; want the verb's unchanged success", again, err)
	}
}

// A signed-in Stop runs machine stop with the session classified as a
// person. Inside the server the verb's parent is the server's launcher, so at
// that shell the same stop is refused as machinery's; through the session the
// machine stops and its fence completes, and no other checkout is touched.
func TestUIStopClassifiesTheSessionAsAPerson(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.class = lease.ClassSupervision
	if code, result, _ := b.runJSON("machine", "stop", "agentic-tools-landing"); code != 1 || result.Outcome != intentRefused ||
		!strings.Contains(result.Summary, "MetaSystem's own machinery started this shell") {
		t.Fatalf("machine stop at the server's own shell = %d %+v; want machinery refused", code, result)
	}
	acts := uiFleetActs{checkout: b.this, owners: b.owners}

	answered, err := acts.stop("Wido", "agentic-tools-landing")

	if err != nil || answered.Outcome != intentConfirmed || answered.Summary != "stopped MetaSystem on agentic-tools-landing ("+b.landing+")" {
		t.Fatalf("stop = %+v %v; want the landing checkout stopped", answered, err)
	}
	if !stopfence.Completed(b.fence(b.landing)) || b.fence(b.this).State != stopfence.StateOpen || *b.stopped[b.this] != 0 {
		t.Fatalf("stop reached the wrong checkout: landing %+v this %+v", b.fence(b.landing), b.fence(b.this))
	}
}

// The checkout serving the page is refused before any stop runs, named by its
// nickname and by its path: in words that say a terminal stops it, with that
// command as line 2. No fence changes and no process is touched.
func TestUIStopRefusesTheServingCheckout(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	acts := uiFleetActs{checkout: b.this, owners: b.owners}

	for _, name := range []string{"m1e", b.this} {
		refused, err := acts.admitStop(name)

		if err != nil || refused == nil || refused.Outcome != intentRefused || !strings.HasPrefix(refused.Summary, "m1e serves this page") ||
			refused.Next == nil || !slices.Equal(refused.Next.Argv, []string{"metasystem", "machine", "stop", "m1e"}) || refused.Next.Reason == "" {
			t.Fatalf("admission of %s = %+v %v; want the serving checkout refused, with the terminal's command", name, refused, err)
		}
	}
	if fence := b.fence(b.this); fence.State != stopfence.StateOpen || fence.Generation != 0 {
		t.Fatalf("a refused stop changed this checkout's fence: %+v", fence)
	}
	for checkout, stops := range b.stopped {
		if *stops != 0 {
			t.Fatalf("a refused stop touched %d processes of %s", *stops, checkout)
		}
	}
}

// The serving checkout is refused again where the stop runs, not only at
// admission: a name admitted for another checkout that names this one by the
// time the verb reads it again (a nickname changed in between) stops nothing.
func TestUIStopRefusesTheServingCheckoutWhereTheStopRuns(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	acts := uiFleetActs{checkout: b.this, owners: b.owners}

	answered, err := acts.stop("Wido", "m1e")

	if err != nil || answered.Outcome != intentRefused || !strings.Contains(answered.Summary, "nothing was changed") {
		t.Fatalf("stop of the serving checkout at run time = %+v %v; want refused", answered, err)
	}
	if fence := b.fence(b.this); fence.State != stopfence.StateOpen || fence.Generation != 0 {
		t.Fatalf("the serving checkout's fence changed: %+v", fence)
	}
	for checkout, stops := range b.stopped {
		if *stops != 0 {
			t.Fatalf("a refused stop touched %d processes of %s", *stops, checkout)
		}
	}
}

// selfHosted gives the bed's serving checkout the self-hosted shape, as this
// repository has it: its installation is <checkout>/metasystem, a template,
// so a stop's state root and installation are that folder and not the
// checkout. It answers the installation.
func (b *machineBed) selfHosted() string {
	b.t.Helper()
	installation := filepath.Join(b.this, "metasystem")
	for _, gone := range []string{filepath.Join(b.this, "metasystem.conf"), filepath.Join(b.this, "bin")} {
		if err := os.RemoveAll(gone); err != nil {
			b.t.Fatal(err)
		}
	}
	for file, content := range map[string]string{
		filepath.Join(installation, "metasystem.conf"):   "metasystem.template=true\n",
		filepath.Join(installation, "bin", "metasystem"): "fixture\n",
	} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			b.t.Fatal(err)
		}
		if err := testexec.WriteFile(file, []byte(content), 0o755); err != nil {
			b.t.Fatal(err)
		}
	}
	return installation
}

// The refusal where the stop runs holds in the self-hosted layout too, where
// the person check is asked about <checkout>/metasystem rather than the
// checkout (R-142-ui, Sol's read of 13e15a879): the serving checkout named
// at run time stops nothing, and another machine of this computer still
// stops through the same guard.
func TestUIStopRefusesTheSelfHostedServingCheckoutWhereTheStopRuns(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	installation := b.selfHosted()
	resolver := stateroot.NewResolver(b.top, noExecutable)
	layout, err := resolver.ResolveLayout(b.this)
	if err != nil || layout.GitRoot != b.this || layout.InstallationRoot != installation {
		t.Fatalf("the bed's serving checkout = %+v %v; want the self-hosted layout, installation %s", layout, err, installation)
	}
	if root, err := resolver.RootForInstallation(installation); err != nil || root != installation {
		t.Fatalf("its state root = %q %v; want the installation itself", root, err)
	}
	acts := uiFleetActs{checkout: b.this, owners: b.owners}

	answered, err := acts.stop("Wido", "m1e")

	if err != nil || answered.Outcome != intentRefused || !strings.Contains(answered.Summary, "nothing was changed") {
		t.Fatalf("stop of the self-hosted serving checkout at run time = %+v %v; want refused", answered, err)
	}
	for _, at := range []string{b.this, installation} {
		if fence := b.fence(at); fence.State != stopfence.StateOpen || fence.Generation != 0 {
			t.Fatalf("the serving checkout's fence at %s changed: %+v", at, fence)
		}
	}
	if *b.stopped[b.this] != 0 {
		t.Fatalf("a refused stop touched %d processes of the serving checkout", *b.stopped[b.this])
	}

	other, err := acts.stop("Wido", "agentic-tools-landing")

	if err != nil || other.Outcome != intentConfirmed || !stopfence.Completed(b.fence(b.landing)) {
		t.Fatalf("stop of another machine beside a self-hosted serving checkout = %+v %v; want it stopped", other, err)
	}
}

// Admission is machine stop's own reading: a machine of this computer is
// admitted whether it runs or is already stopped, and a name that is no
// machine here, or one that runs on another computer, is the verb's own
// refusal.
func TestUIStopAdmitsWhatMachineStopReads(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	acts := uiFleetActs{checkout: b.this, owners: b.owners}

	for _, name := range []string{"agentic-tools-landing", "m1x"} {
		if refused, err := acts.admitStop(name); err != nil || refused != nil {
			t.Fatalf("admission of %s = %+v %v; want it admitted", name, refused, err)
		}
	}
	for name, said := range map[string]string{"m9z": "no machine m9z on this computer or in the fleet", "m2a": "m2a runs on another computer"} {
		refused, err := acts.admitStop(name)
		if err != nil || refused == nil || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, said) || refused.Next == nil {
			t.Fatalf("admission of %s = %+v %v; want the verb's refusal %q", name, refused, err, said)
		}
	}
}

// Two signed-in Stops of one machine, the way two tabs press it: the first is
// confirmed and its fence completed; the second is admitted, because a machine
// stopped is still this computer's, and is the verb's unchanged success that
// writes nothing, as witnessMachineStopRepeat checks a repeat at a terminal.
func TestUIStopTwiceIsTheVerbsUnchangedWithNoNewFence(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.class = lease.ClassSupervision
	acts := uiFleetActs{checkout: b.this, owners: b.owners}
	press := func() httpd.LandNowAnswer {
		t.Helper()
		if refused, err := acts.admitStop("agentic-tools-landing"); err != nil || refused != nil {
			t.Fatalf("admission = %+v %v; want it admitted", refused, err)
		}
		answered, err := acts.stop("Wido", "agentic-tools-landing")
		if err != nil {
			t.Fatalf("stop = %v", err)
		}
		return answered
	}

	first := press()
	if first.Outcome != intentConfirmed || !stopfence.Completed(b.fence(b.landing)) {
		t.Fatalf("first stop = %+v, fence %+v; want it confirmed and completed", first, b.fence(b.landing))
	}
	fenced := b.fence(b.landing)
	before := idemTreeDigest(t, filepath.Dir(b.this))

	second := press()

	if second.Outcome != intentUnchanged || !strings.HasPrefix(second.Summary, "MetaSystem is already stopped on agentic-tools-landing") {
		t.Fatalf("second stop = %+v; want the verb's unchanged success", second)
	}
	idemSameTree(t, "a second signed-in stop", before, idemTreeDigest(t, filepath.Dir(b.this)))
	if b.fence(b.landing).Generation != fenced.Generation {
		t.Fatalf("the second stop wrote a new fence generation: %d, was %d", b.fence(b.landing).Generation, fenced.Generation)
	}
}
