package main

// `metasystem seat launch`: one machine of this fleet joins on this host.
//
// This file is the edge and nothing else. It parses the flags, gathers the
// facts the preflight judges against, takes the host lock, builds the
// sequencer out of the real host, the real runner, the real transport and the
// wall clock, and prints what the record says. Every decision belongs to
// internal/seat/launch, and every step belongs to the owner it runs.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// seatLaunchPresenceTicks is how many of the new machine's ticks the presence
// step waits before it reports armed rather than done.
const seatLaunchPresenceTicks = 3

// seatLaunchBuildBudget bounds the one build. It is the fenced build script's
// own worst case on a cold module cache and not a guess at a fast one.
const seatLaunchBuildBudget = 10 * time.Minute

func runSeatLaunch(args []string) int {
	flags := flag.NewFlagSet("seat launch", flag.ContinueOnError)
	machine := flags.String("machine", "", "nickname the new machine carries and publishes presence under")
	from := pathFlag(flags, "from", ".", "the checkout this machine is cloned from (default: the current directory)")
	destination := pathFlag(flags, "destination", "", "where the clone lands (default: beside this checkout, named for the remote's repository and the nickname)")
	word := flags.String("temporary-human-word", "", "the human's own authorization, verbatim; enrolls the machine TEMPORARILY with the word recorded on its identity")
	reviewBy := flags.String("review-by", "", "the human's own re-approval date (required with --temporary-human-word)")
	resume := flags.String("resume", "", "continue the launch with this id: verify each done step and redo what does not hold")
	recordPath := pathFlag(flags, "record", "", "the record file this launch writes (default: one per launch under artifacts/agents/ui/launches)")
	asJSON := flags.Bool("json", false, "print the record as JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := humanauthority.ValidateTemporaryWordPair(*word, *reviewBy); err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 2
	}

	request := launch.Request{
		Machine: *machine, From: *from, Destination: *destination,
		Word: *word, ReviewBy: *reviewBy, Resume: *resume,
	}
	record, path, err := seatLaunchRecord(request, *recordPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	// A resume takes the machine and the destination from the record it
	// names: they are what this launch created, and a resume that took them
	// from a flag could finish one launch into another's directory.
	if request.Resuming() {
		request.Machine, request.Destination = record.Machine, record.Destination
		if request.ReviewBy == "" {
			request.ReviewBy = record.ReviewBy
		}
	}
	if request.Machine == "" {
		fmt.Fprintln(os.Stderr, "seat launch: --machine is required")
		return 2
	}
	if request.Destination == "" {
		proposed, err := seatLaunchDestination(request.From, request.Machine)
		if err != nil {
			fmt.Fprintln(os.Stderr, "seat launch:", err)
			return 1
		}
		request.Destination = proposed
	}

	facts, err := seatLaunchFacts(request, record)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	if err := launch.Preflight(request, facts); err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}

	held, err := launch.Take(launch.LockPath(), request.Machine, record.Launch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	defer func() { _ = held.Release() }()

	namespace, err := launch.NewNamespace()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	transport, err := seat.NewGit(request.From)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	installation, err := seatLaunchInstallation(request.From)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", err)
		return 1
	}
	sequencer := &launch.Sequencer{
		Request: request, Installation: installation,
		OriginURL: seatLaunchOrigin(request.From),
		Host:      launch.OSHost{}, Runner: launch.OSRunner{},
		Presence: launch.SeatPresence{Transport: transport}, Clock: launch.Wall{},
		Write:     func(written launch.Record) error { return launch.SaveAt(path, written, request.From) },
		Namespace: namespace,
		// git and the network take the ledger fetch's own budget; the build
		// takes its own, because a cold module cache is minutes and not
		// seconds.
		GitBudget: seat.TransportBudget, BuildBudget: seatLaunchBuildBudget,
		// The new machine's cadence is this seat's: the configuration that
		// sets it is the configuration this launch copies.
		PresenceTick:  time.Duration(steward.TickSeconds(filepath.Join(request.From, installation))) * time.Second,
		PresenceTicks: seatLaunchPresenceTicks,
	}
	record.Process = launch.Identify(os.Getpid())
	record.StartedAt = time.Now().UTC().Format(time.RFC3339)
	finished, runErr := sequencer.Run(record)
	if *asJSON {
		encoded, err := json.MarshalIndent(finished, "", "  ")
		if err == nil {
			os.Stdout.Write(append(encoded, '\n'))
		}
	} else {
		fmt.Print(seatLaunchReport(finished))
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "seat launch:", runErr)
		return 1
	}
	return 0
}

// seatLaunchRecord is the record this run writes and where it writes it: the
// one a resume names, the one the interface already wrote, or a fresh one.
func seatLaunchRecord(request launch.Request, named string) (launch.Record, string, error) {
	if request.Resuming() {
		path := named
		if path == "" {
			resolved, err := launch.Path(request.From, request.Resume)
			if err != nil {
				return launch.Record{}, "", err
			}
			path = resolved
		}
		record, err := launch.LoadAt(path)
		if err != nil {
			return launch.Record{}, "", err
		}
		if record.Launch != request.Resume {
			return launch.Record{}, "", fmt.Errorf("the record at %s is launch %s and not %s", path, record.Launch, request.Resume)
		}
		return record, path, nil
	}
	if named != "" {
		// The interface writes the record before it spawns this verb, so a
		// named file that is already there is this launch's own id.
		if record, err := launch.LoadAt(named); err == nil && record.Launch != "" {
			return record, named, nil
		}
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return launch.Record{}, "", err
	}
	path := named
	if path == "" {
		resolved, err := launch.Path(request.From, id)
		if err != nil {
			return launch.Record{}, "", err
		}
		path = resolved
	}
	return launch.Record{SchemaVersion: launch.SchemaVersion, Launch: id, Outcome: launch.OutcomeRunning}, path, nil
}

// seatLaunchFacts is the world the preflight judges against: this seat's own
// nickname, every nickname already spoken for, and what a resumed launch
// created.
func seatLaunchFacts(request launch.Request, record launch.Record) (launch.Facts, error) {
	this, _ := seat.Machine(request.From)
	taken := map[string]bool{}
	transport, err := seat.NewGit(request.From)
	if err != nil {
		return launch.Facts{}, err
	}
	// The tick's canonical copy, read in place and never fetched: the
	// question is which nicknames this fleet already knows, and a fetch here
	// would move a ref the steward owns.
	namespace := seat.TickNamespace
	if transport.Local {
		namespace = seat.UINamespace
	}
	if copied, err := transport.Read(namespace); err == nil {
		for name := range copied.Records {
			taken[name] = true
		}
	}
	claims, _ := seat.Claims(request.From)
	for name := range claims {
		taken[name] = true
	}
	for _, name := range launch.Siblings(filepath.Dir(filepath.Clean(request.From)), launch.RepositoryName(seatLaunchOrigin(request.From))) {
		taken[name] = true
	}
	names := make([]string, 0, len(taken))
	for name := range taken {
		names = append(names, name)
	}
	facts := launch.Facts{This: this, Taken: names}
	if request.Resuming() {
		facts.Created = record.Created
	}
	return facts, nil
}

// seatLaunchDestination is where a machine lands when nobody says: beside
// this checkout, named for the ledger remote's own repository.
func seatLaunchDestination(from, machine string) (string, error) {
	repository := launch.RepositoryName(seatLaunchOrigin(from))
	if repository == "" {
		return "", fmt.Errorf("this checkout's origin names no repository, so there is no default destination; pass --destination")
	}
	return launch.DefaultDestination(from, repository, machine), nil
}

// seatLaunchOrigin is this checkout's origin URL, or "" where it has none.
func seatLaunchOrigin(from string) string {
	out, err := goalBranchGit(from, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// seatLaunchInstallation is where the engine lives inside this checkout,
// relative to its root: the directory that carries metasystem.conf. The clone
// is a clone of this checkout, so the same relative path names the engine in
// it.
func seatLaunchInstallation(from string) (string, error) {
	for _, relative := range []string{"metasystem", "."} {
		if _, err := os.Stat(filepath.Join(from, relative, "metasystem.conf")); err == nil {
			return relative, nil
		}
	}
	return "", fmt.Errorf("no metasystem.conf under %s, so this is not a checkout a machine can be cloned from", from)
}

// seatLaunchReport is the record in words, for a human who ran the verb at a
// terminal: one line per step, then what the machine is and what to do next.
func seatLaunchReport(record launch.Record) string {
	var built strings.Builder
	built.WriteString(record.Machine + " " + record.Outcome + " at " + record.Destination + "\n")
	for _, step := range record.Steps {
		line := "  " + step.Step + ": " + step.Outcome
		if step.Words != "" {
			line += " — " + step.Words
		}
		built.WriteString(line + "\n")
	}
	if record.Orientation != "" {
		built.WriteString("  next work: " + record.Orientation + "\n")
	}
	if record.Outcome == launch.OutcomeDone || record.Outcome == launch.OutcomeArmed {
		built.WriteString("  start a session: " + record.Next.Session + "\n")
		built.WriteString("  stop the machine: " + record.Next.Stop + "\n")
	}
	return built.String()
}
