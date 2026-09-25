package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// `ui tools`: the interface's own read tools, served over stdio.
//
// It is the one verb nothing types. The interface server hands it to the
// Project Partner's runtime at session setup — a stdio tool server named in
// `mcpServers` — and the runtime starts this process, talks the Model Context
// Protocol to it, and closes it when the session ends. So this is the engine
// answering the Partner from the same readers the pages are composed from,
// rather than a second account of the workspace living inside an agent.
//
// It writes nothing, reads only what the published operations read, and holds
// no state between calls beyond the readers themselves.

func runUITools(args []string) int {
	flags := flag.NewFlagSet("ui tools", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout the tools read (default: the checkout that contains the installation)")
	installation := flags.String("metasystem-root", "", "metasystem installation")
	// Which interface server run started this tool server. The fleet tool
	// reads the presence-fetch metadata that run leaves behind, and that file
	// outlives the run that wrote it; without this name the tool could cite a
	// previous server's success while the page, which keeps its fetch state
	// only in memory, had fallen back to the tick's copy.
	presenceRun := flags.String("presence-run", "", "the interface server run whose presence fetches this tool may cite")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "invalid arguments for ui tools")
		return 2
	}
	metasystemRoot, err := upMetasystemRoot(*installation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	roots, err := lifecycle.ResolveRoots(*root, metasystemRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if err := uitools.Serve(os.Stdin, os.Stdout, toolReaders(roots, *presenceRun)); err != nil {
		fmt.Fprintln(os.Stderr, "the interface's tool server stopped reading: "+err.Error())
		return 1
	}
	return 0
}

// toolReaders is the published operations' own readers, over one checkout. They
// are the interface server's readers, built here again rather than passed
// across a process boundary: this process is the engine, and the engine reads
// the ledger and the checkout the same way wherever it runs.
func toolReaders(roots lifecycle.Roots, presenceRun string) uitools.Readers {
	now := func() time.Time { return time.Now().UTC() }
	ledger := snapshot.New(roots.StateRoot, time.Now)
	journal := steward.NotificationJournalPath(roots.Checkout)
	pane := func() (project.Pane, error) { return project.ReadPane(projectRoots(roots), now()) }
	notices := func(limit int, before string) ([]notifications.Notice, error) {
		return notifications.Page(journal, limit, before)
	}
	return uitools.Readers{
		Now:     now,
		Observe: ledger.Observe,
		// What this interface is made of, and what this kit's own words mean.
		// Both are composed here, in the engine, from the owners that hold
		// them: the bundle this executable carries, the configuration this
		// seat resolves, the resolver's homes, and the kit's own documents.
		Interface: describeInterface(roots),
		Kit:       uitools.Kit{Root: roots.Installation, Commands: commandCatalogue},
		Document: func(id string) (project.Document, error) {
			return project.Read(projectRoots(roots), id, now())
		},
		Project: pane,
		// The fleet, from the copy the interface server's own fetch owner
		// left in this checkout and the metadata it writes after every
		// attempt. This process owns no fetcher: it is started by the
		// Partner's runtime and lives for one session, and a second fetcher
		// beside the server's would be a second minute rule over one
		// namespace.
		Fleet: func() (fleet.Page, error) {
			state, _, err := fleet.LoadMetadata(roots.Checkout)
			// Only the run that started this tool server may be cited. A
			// file from another run describes fetches no live server has
			// made, into a namespace this run has not refreshed, so it is
			// read as no metadata at all and the tool falls back to the
			// tick's copy exactly as the page does.
			mine := err == nil && state.OfRun(presenceRun)
			copied := fleet.Copy{}
			switch {
			case err != nil:
				copied.Problem = "the interface's presence fetch metadata could not be read: " + err.Error()
			case mine:
				copied = fleet.Copy{
					AttemptedAt: state.AttemptedAt, SucceededAt: state.SucceededAt,
					FailedAt: state.FailedAt, Problem: state.Problem,
				}
			case state.Run != "":
				copied.Problem = "the interface's presence fetch metadata was written by another server run"
			}
			observed := ledger.Observe()
			board := backlog.Board{}
			if observed.State == snapshot.StateRead && observed.Tree != nil {
				board = backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
			}
			return fleetReader(roots,
				func() bool { return mine && state.SucceededAt != "" },
				func() fleet.Copy { return copied },
			)(observed, board, now())
		},
		Notices: notices,
		// The landing page, composed here the way the interface server composes
		// it for a turn asked from Overview: over a first visit's day, and
		// recording no visit. Reading the page is the human's visit; a tool
		// call is not, and moving their marker would shorten the window their
		// next visit compares against.
		Overview: func() (overview.Page, error) {
			read, err := pane()
			if err != nil {
				return overview.Page{}, err
			}
			held, err := notices(notifications.DefaultLimit, "")
			if err != nil {
				held = []notifications.Notice{}
			}
			observed := ledger.Observe()
			board := backlog.Board{}
			if observed.State == snapshot.StateRead && observed.Tree != nil {
				board = backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
			}
			at := now()
			return overview.Compose(overview.Inputs{
				Project: read,
				Rows:    board.Rows,
				Closed:  board.Closed,
				Counts:  board.Counts,
				Ledger: overview.Ledger{
					Freshness: snapshot.Freshness(observed.Fetch.Outcome),
					AtTip:     observed.State == snapshot.StateRead,
					Statement: observed.Message,
				},
				Journal: held,
				Since:   at.Add(-24 * time.Hour),
				First:   true,
			}, at), nil
		},
	}
}
