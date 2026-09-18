package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func runLaunchPackCheck(args []string) int {
	flags := flag.NewFlagSet("launch pack-check", flag.ContinueOnError)
	kind := flags.String("kind", "", "launch kind")
	brief := flags.String("brief", "", "brief file")
	directory := flags.String("dir", ".", "working directory")
	if flags.Parse(args) != nil {
		return 2
	}
	if (*kind != "design" && *kind != "read") || *brief == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch pack-check --kind <design|read> --brief <file> [--dir <directory>]")
		return 2
	}
	ranges, err := launchManager().CheckPack(launch.StartSpec{Kind: *kind, Brief: *brief, WorkingDirectory: *directory})
	if err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Printf("pack=ok kind=%s ranges=%d\n", *kind, ranges)
	return 0
}
