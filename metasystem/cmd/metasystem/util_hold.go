package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// runUtilHold parks the process until it is told to stop. Fixtures use it as
// a stand-in child whose whole job is to exist: the --tag value rides in the
// process's command line, so the census and reaper can recognize the process
// by scanning the process table for the tag. On SIGTERM or SIGINT the process
// acknowledges the orderly stop — writing "stopped" to --stopped-file when
// one is given — and exits 0, so a supervisor can distinguish a child that
// saw its termination signal from one that was killed outright.
func runUtilHold(args []string) int {
	flags := flag.NewFlagSet("util hold", flag.ContinueOnError)
	tag := flags.String("tag", "", "instance tag carried in this process's command line")
	stoppedFile := flags.String("stopped-file", "", "file that receives \"stopped\" on an orderly stop")
	if flags.Parse(args) != nil {
		return 2
	}
	if *tag == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem util hold --tag TAG [--stopped-file FILE]")
		return 2
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(signals)
	<-signals

	if *stoppedFile != "" {
		if err := os.WriteFile(*stoppedFile, []byte("stopped\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}
