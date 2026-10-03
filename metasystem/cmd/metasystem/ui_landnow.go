package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// landNowVerb is THE SEAM for the landing lane card's Land now (goal
// fleet-card-can-land-now): the public verb the interface runs, in this
// process exactly as the terminal runs it, with --json and this checkout's
// --repo, reading back its one-result envelope (intentResult). It is found by
// name in the verb table, so an engine built with landing run runs it and one
// built before it answers in words that it cannot; wiring the real verb is
// this constant and, should its flags differ, landNowArgs.
const landNowVerb = "landing run"

// landNowArgs is the argument vector landing run is run with.
func landNowArgs(checkout string) []string {
	return []string{"--json", "--repo", checkout}
}

// landNowRun is Land now as the server performs it.
func landNowRun(roots lifecycle.Roots) (httpd.LandNowAnswer, error) {
	owners := defaultIntentOwners()
	return landNowRunWith(roots.Checkout, findIntentCommand, func(command intentCommand, args []string, stdout, stderr io.Writer) {
		runIntentIn(command, args, stdout, stderr, roots.Checkout, owners)
	})
}

func landNowRunWith(checkout string, find func(string) (intentCommand, bool), run func(intentCommand, []string, io.Writer, io.Writer)) (httpd.LandNowAnswer, error) {
	command, found := find(landNowVerb)
	if !found {
		return httpd.LandNowAnswer{Outcome: intentRefused,
			Summary: "this engine has no metasystem landing run yet, so Land now cannot start the landing agent; nothing was started",
			Next:    &httpd.LandNowNext{Argv: []string{}, Reason: "the lane's keeper starts it on its next tick when there is work"}}, nil
	}
	return verbAnswer(landNowVerb, command, landNowArgs(checkout), run)
}

// verbAnswer runs one public verb's command with args through run and reads
// back its one-result envelope (intentResult) as the page shows it: the
// outcome, line 1 and line 2. It is the one runner Land now and the fleet
// panel's three acts share.
func verbAnswer(verb string, command intentCommand, args []string, run func(intentCommand, []string, io.Writer, io.Writer)) (httpd.LandNowAnswer, error) {
	var stdout, stderr bytes.Buffer
	run(command, args, &stdout, &stderr)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return httpd.LandNowAnswer{}, errors.New(verb + " answered nothing the interface can read: " + string(bytes.TrimSpace(stderr.Bytes())))
	}
	answered := httpd.LandNowAnswer{Outcome: result.Outcome, Summary: result.Summary}
	if result.Next != nil {
		// The next command exactly as the verb built it: its --repo names
		// this checkout so it works from any directory (Sol LN-01).
		answered.Next = &httpd.LandNowNext{Argv: result.Next.Argv, Reason: result.Next.Reason}
	}
	return answered, nil
}

// The fleet panel's three acts as the signed-in person
// (fleet-panel-ux-step2.md slice 2b, design D1; ruling R-142-ui admits the
// session for these three verbs from that panel and for nothing else).
const (
	lanePauseVerb   = "landing stop"
	laneResumeVerb  = "landing start"
	machineStopVerb = "machine stop"
)

// sessionOwners are owners whose two person seams answer the signed-in
// human (design D1): the landing verbs' person names them, and the process
// verbs' classify answers the human class, as act.SignedIn classifies a
// session ("the thing that was proven is that a human answered a one-time
// code"). Nothing else changes: no verb body, no flag, and nothing is
// serialized, because a proof parsed from JSON has no authority, so neither a
// subprocess nor a token could carry it. The route admitted the human with
// act.SignedIn before any of this runs.
func sessionOwners(owners intentOwners, human string) intentOwners {
	owners.landing.person = func(string) (string, error) { return human, nil }
	owners.processes.process.classify = func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassHuman}, nil
	}
	return owners
}

// uiFleetActs are the fleet panel's Pause, Resume and Stop for the checkout
// this server serves, each its public verb run once in this process with
// --json and that checkout's --repo, as Land now runs landing run. owners are
// a fresh set per act: production's defaults, or a test bed's.
type uiFleetActs struct {
	checkout string
	owners   func() intentOwners
}

// run runs verb for human under sessionOwners.
func (acts uiFleetActs) run(human, verb string, args ...string) (httpd.LandNowAnswer, error) {
	return acts.runWith(sessionOwners(acts.owners(), human), verb, args...)
}

// runWith runs verb under owners, once, in this process.
func (acts uiFleetActs) runWith(owners intentOwners, verb string, args ...string) (httpd.LandNowAnswer, error) {
	command, found := findIntentCommand(verb)
	if !found {
		return httpd.LandNowAnswer{}, errors.New("this engine has no metasystem " + verb + ", so the interface cannot run it")
	}
	return verbAnswer(verb, command, append([]string{"--json", "--repo", acts.checkout}, args...), func(command intentCommand, args []string, stdout, stderr io.Writer) {
		runIntentIn(command, args, stdout, stderr, acts.checkout, owners)
	})
}

// pause is landing stop with the session's human as who paused the lane,
// which the pause file records and the panel shows ("paused by Wido").
func (acts uiFleetActs) pause(human string) (httpd.LandNowAnswer, error) {
	return acts.run(human, lanePauseVerb, "--by", human)
}

// resume is landing start with the session's human as the person it asks.
// It records nobody, at a terminal as from here.
func (acts uiFleetActs) resume(human string) (httpd.LandNowAnswer, error) {
	return acts.run(human, laneResumeVerb)
}

// stop is machine stop of one machine with the session classified as a
// person. The name comes after "--", so a name can never be read as a flag;
// the route admitted it first (admitStop). The verb reads the name again, so
// the checkout serving this page is refused once more where the stop runs:
// the person seam answers no person for it (R-142-ui).
func (acts uiFleetActs) stop(human, machine string) (httpd.LandNowAnswer, error) {
	owners := sessionOwners(acts.owners(), human)
	person := owners.processes.process.classify
	owners.processes.process.classify = func(repo, root string, pid int64) (lease.Classification, error) {
		if acts.serving(repo) || acts.serving(root) {
			return lease.Classification{}, errors.New("the machine " + machine + " names now is the one serving this page, which is stopped at a terminal you opened yourself")
		}
		return person(repo, root, pid)
	}
	return acts.runWith(owners, machineStopVerb, "--", machine)
}

// serving reports whether a path the stop's person check is asked about is
// the checkout serving this page or lies inside it. The check is asked about
// the stop's state root and installation, which in the self-hosted layout are
// <checkout>/metasystem and not the checkout, so equality alone would miss
// it. Both are compared as given and with their links resolved.
func (acts uiFleetActs) serving(path string) bool {
	return machinePathWithin(filepath.Clean(path), filepath.Clean(acts.checkout)) ||
		machinePathWithin(realpath.Resolve(path), realpath.Resolve(acts.checkout))
}
