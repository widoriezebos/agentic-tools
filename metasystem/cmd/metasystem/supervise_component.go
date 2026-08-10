package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// runSuperviseComponent is a supervised component (watcher or reaper)
// in its minimal Phase 0 form: it rewrites its heartbeat file every
// interval and lives until signalled. The owner observes it by that
// heartbeat plus kernel liveness. The component's real work — running
// the census, applying reaper verdicts — arrives in Phase 0b/1 as
// those pieces port; this is the lifecycle skeleton the owner-alone
// fixtures need to drive the running binary.
func runSuperviseComponent(args []string) int {
	flags := flag.NewFlagSet("supervise component", flag.ContinueOnError)
	component := flags.String("component", "", "watcher | reaper")
	tag := flags.String("tag", "", "component instance tag")
	heartbeat := flags.String("heartbeat", "", "heartbeat file path")
	intervalSec := flags.Int("interval", 60, "heartbeat interval seconds")
	// crashOnStart reproduces the pure crash-loop shape (D-2): a
	// component that dies on startup WITHOUT ever beating, so it never
	// reads Healthy and the breaker advances monotonically to N. A
	// component that beat even once would reset the breaker (the
	// four-fail-then-reset pattern), which is a different, non-terminal
	// shape. Fixture-only.
	crashOnStart := flags.Bool("crash-on-start", false, "exit immediately without beating (fixture-only)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *component == "" || *tag == "" || *heartbeat == "" {
		fmt.Fprintln(os.Stderr, "supervise component: --component, --tag, --heartbeat required")
		return 2
	}
	if *crashOnStart {
		fmt.Fprintln(os.Stderr, "supervise component: crash-on-start (fixture)")
		return 1
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	write := func() {
		record := map[string]any{
			"pid":             os.Getpid(),
			"pidStartedAt":    0,
			"instanceTag":     *tag,
			"observedAtEpoch": time.Now().Unix(),
			"loadedCapMin":    *intervalSec,
			"engine":          "go",
		}
		line, err := json.Marshal(record)
		if err != nil {
			return
		}
		temporary, err := os.CreateTemp(filepath.Dir(*heartbeat), ".hb-*")
		if err != nil {
			return
		}
		defer os.Remove(temporary.Name())
		if _, err := temporary.Write(line); err != nil {
			temporary.Close()
			return
		}
		temporary.Close()
		_ = os.Rename(temporary.Name(), *heartbeat)
	}

	write() // beat once immediately so the owner sees liveness fast
	ticker := time.NewTicker(time.Duration(*intervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return 0
		case <-ticker.C:
			write()
		}
	}
}
