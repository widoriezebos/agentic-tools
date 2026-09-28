// Command fixturesupervisor is a real, detached supervisor process for the
// launch contract's tests: it is the same applaunch.Supervise the engine
// runs, in a process of its own session and group, so that the tests can
// interrupt it where a person's kill would land.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

func main() {
	stateRoot := flag.String("state-root", "", "the seat's state root")
	key := flag.String("key", applaunch.StandingKey, "the run's key")
	contractPath := flag.String("contract", "", "the launch contract")
	projectRoot := flag.String("project-root", "", "the tree the contract's commands run in")
	address := flag.String("address", "", "the address this run listens on")
	readyFile := flag.String("ready-file", "", "write the readiness answer here")
	readyFD := flag.Int("ready-fd", -1, "write the readiness answer to this descriptor")
	dieAfterSpawn := flag.Bool("die-after-spawn", false, "die between the spawn and the child's ref write")
	flag.Parse()

	contract, err := applaunch.Load(*contractPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixturesupervisor:", err)
		os.Exit(2)
	}
	var pipe *os.File
	if *readyFD >= 0 {
		pipe = os.NewFile(uintptr(*readyFD), "readiness")
	}
	report := func(line string) {
		if *readyFile != "" {
			_ = os.WriteFile(*readyFile, []byte(line+"\n"), 0o644)
		}
		if pipe != nil {
			fmt.Fprintln(pipe, line)
			_ = pipe.Close()
			pipe = nil
		}
		fmt.Println(line)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	options := applaunch.SuperviseOptions{
		Context:     ctx,
		StateRoot:   *stateRoot,
		Contract:    contract,
		ProjectRoot: *projectRoot,
		Environment: os.Environ(),
		Seed:        applaunch.Record{Key: *key, Address: *address, StateRoot: *stateRoot},
		Ready:       func(a string) { report("ready " + a) },
		Failed:      func(m string) { report("failed " + m) },
	}
	if *dieAfterSpawn {
		options.AfterSpawn = func() error { os.Exit(9); return nil }
	}
	if err := applaunch.Supervise(options); err != nil {
		fmt.Fprintln(os.Stderr, "fixturesupervisor:", err)
		os.Exit(1)
	}
}
