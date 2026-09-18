package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func runProcDefaultSignals(args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "proc default-signals: a command is required after --")
		return 2
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc default-signals:", err)
		return 127
	}
	// Ignored signals stay ignored across exec, while signals with handlers
	// return to their default disposition. Install handlers so the command
	// can receive signals that the engine's caller ignored.
	handledSignals := make(chan os.Signal, 3)
	signal.Notify(handledSignals, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	if err := syscall.Exec(path, args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "proc default-signals: exec:", err)
		return 126
	}
	return 0
}
