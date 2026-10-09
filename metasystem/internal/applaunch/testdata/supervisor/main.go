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
	"time"

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
	noReadyDeadline := flag.Bool("no-ready-deadline", false, "wait for readiness or the application's exit, with no deadline")
	listenFD := flag.Int("listen-fd", -1, "inherited application listener")
	flag.Parse()

	contract, err := applaunch.Load(*contractPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixturesupervisor:", err)
		os.Exit(2)
	}
	var pipe *os.File
	if *readyFD >= 0 {
		pipe = applaunch.ReadinessPipe(*readyFD)
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
	if *listenFD >= 0 {
		listener := os.NewFile(uintptr(*listenFD), "application listener")
		options.Spawn = func(spec applaunch.ChildSpec) (applaunch.Child, error) {
			spec.Argv = append(spec.Argv, "--listen-fd", "3")
			spec.ExtraFiles = []*os.File{listener}
			child, err := applaunch.ExecChild(spec)
			_ = listener.Close()
			return child, err
		}
	}
	if *noReadyDeadline {
		// Readiness is the application's own signal or its exit; a test's
		// verdict never rests on a clock a loaded host outruns.
		options.ReadyDeadline = func(time.Duration) <-chan time.Time { return nil }
	}
	if *dieAfterSpawn {
		// A person's kill lands here: no deferred act, no ended record.
		options.AfterSpawn = func() error {
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			select {}
		}
	}
	if err := applaunch.Supervise(options); err != nil {
		fmt.Fprintln(os.Stderr, "fixturesupervisor:", err)
		os.Exit(1)
	}
}
