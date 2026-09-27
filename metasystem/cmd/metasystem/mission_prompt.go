package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The mission-prompt family assembles the byte-stable unattended mission
// host-turn prompt for one turn from the frozen authority and the mission's
// live control-plane data.

func runMissionPromptAssemble(args []string) int { return missionPromptAssembleTo(args, os.Stderr) }

// missionPromptAssembleTo is prompt-assemble with its diagnostics on stderr.
func missionPromptAssembleTo(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("mission prompt-assemble", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := pathFlag(flags, "repo", "", "repository root")
	missionID := flags.String("mission", "", "mission id")
	turn := flags.String("turn", "", "turn id")
	output := flags.String("output", "", "path to write the assembled prompt")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *missionID == "" || *turn == "" || *output == "" {
		fmt.Fprintln(stderr,
			"usage: metasystem internal mission prompt-assemble --repo <dir> --mission <id> --turn <turn-id> --output <file>")
		return 2
	}
	if err := mission.AssemblePrompt(*repo, *missionID, *turn, *output); err != nil {
		fmt.Fprintf(stderr, "mission prompt refused: %v\n", err)
		return 1
	}
	return 0
}
