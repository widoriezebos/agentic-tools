package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

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
	var stdout, stderr bytes.Buffer
	run(command, landNowArgs(checkout), &stdout, &stderr)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return httpd.LandNowAnswer{}, errors.New("landing run answered nothing the interface can read: " + string(bytes.TrimSpace(stderr.Bytes())))
	}
	answered := httpd.LandNowAnswer{Outcome: result.Outcome, Summary: result.Summary}
	if result.Next != nil {
		// The next command exactly as the verb built it: its --repo names
		// this checkout so it works from any directory (Sol LN-01).
		answered.Next = &httpd.LandNowNext{Argv: result.Next.Argv, Reason: result.Next.Reason}
	}
	return answered, nil
}
