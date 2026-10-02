// Command fixtureapp is the application the launch contract's tests drive.
// It answers a health URL after a configurable delay, can write a readiness
// line to its log, can be told to ignore TERM, and can leave a descendant
// behind that outlives a TERM to its own parent.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "", "address to listen on (default: METASYSTEM_APP_ADDRESS)")
	readyAfter := flag.Duration("ready-after", 0, "answer 200 only after this long")
	darkFile := flag.String("dark-file", "", "stop answering 200 while this file exists, while staying alive")
	readyFile := flag.String("ready-file", "", "answer 200 only once this file exists")
	listenAfter := flag.Duration("listen-after", 0, "start listening only after this long")
	readyLine := flag.String("ready-line", "", "write this line to the log once ready")
	ignoreTerm := flag.Bool("ignore-term", false, "ignore SIGTERM")
	noListen := flag.Bool("no-listen", false, "listen on nothing at all")
	spawnDescendant := flag.Bool("spawn-descendant", false, "spawn a descendant that ignores TERM and wait in the foreground")
	exitAfter := flag.Duration("exit-after", 0, "exit by itself after this long")
	exitCode := flag.Int("exit-code", 0, "the status to exit with")
	exitNow := flag.Bool("exit-now", false, "exit before anything is ready")
	live := flag.String("live-file", "", "touch this file while alive and remove it on exit")
	flag.Parse()

	if *exitNow {
		fmt.Println("fixtureapp: exiting before readiness")
		os.Exit(*exitCode)
	}
	if *ignoreTerm {
		signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)
	}
	if *live != "" {
		_ = os.WriteFile(*live, []byte(strconv.Itoa(os.Getpid())), 0o644)
		defer os.Remove(*live)
	}
	if *spawnDescendant {
		self, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "fixtureapp:", err)
			os.Exit(1)
		}
		args := []string{"--ignore-term", "--no-listen"}
		if *live != "" {
			args = append(args, "--live-file", *live+".descendant")
		}
		descendant := exec.Command(self, args...)
		descendant.Stdout, descendant.Stderr = os.Stdout, os.Stderr
		if err := descendant.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "fixtureapp:", err)
			os.Exit(1)
		}
		fmt.Println("fixtureapp: descendant", descendant.Process.Pid)
	}

	address := *listen
	if address == "" {
		address = os.Getenv("METASYSTEM_APP_ADDRESS")
	}
	started := time.Now()
	if !*noListen && address != "" {
		time.Sleep(*listenAfter)
		listener, err := net.Listen("tcp", address)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fixtureapp: cannot listen:", err)
			os.Exit(1)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/-/health", func(w http.ResponseWriter, r *http.Request) {
			if time.Since(started) < *readyAfter {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if *readyFile != "" {
				if _, err := os.Stat(*readyFile); err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
			}
			if *darkFile != "" {
				if _, err := os.Stat(*darkFile); err == nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			fmt.Fprintln(w, "ok")
		})
		server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() { _ = server.Serve(listener) }()
		fmt.Println("fixtureapp: listening on", listener.Addr().String())
	}
	if *readyLine != "" {
		time.Sleep(*readyAfter)
		fmt.Println(*readyLine)
	}
	if *exitAfter > 0 {
		time.Sleep(*exitAfter)
		fmt.Println("fixtureapp: exiting by itself")
		if *live != "" {
			_ = os.Remove(*live)
		}
		os.Exit(*exitCode)
	}
	if *ignoreTerm {
		select {}
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	fmt.Println("fixtureapp: stopping")
}
