package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

func runUIStart(args []string) int   { return runUI("start", args) }
func runUIServe(args []string) int   { return runUI("serve", args) }
func runUIStatus(args []string) int  { return runUI("status", args) }
func runUIStop(args []string) int    { return runUI("stop", args) }
func runUIRestart(args []string) int { return runUI("restart", args) }

func runUI(verb string, args []string) int {
	flags := flag.NewFlagSet("ui "+verb, flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout path (default: the checkout that contains the installation)")
	root := flags.String("metasystem-root", "", "metasystem installation")
	var listen string
	var waitSeconds int64 = 15
	readyFD := -1
	if verb == "start" || verb == "serve" || verb == "restart" {
		flags.StringVar(&listen, "listen", "", "loopback IP and port")
	}
	if verb == "stop" || verb == "restart" {
		flags.Int64Var(&waitSeconds, "wait-seconds", 15, "seconds to wait for the server to stop")
	}
	if verb == "serve" {
		flags.IntVar(&readyFD, "ready-fd", -1, "readiness pipe descriptor")
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || waitSeconds < 0 || waitSeconds > int64((1<<63-1)/time.Second) || readyFD < -1 {
		fmt.Fprintln(os.Stderr, "invalid arguments for ui "+verb)
		return 2
	}
	var ready *os.File
	if readyFD >= 0 {
		ready = os.NewFile(uintptr(readyFD), "interface readiness")
		defer ready.Close()
	}
	// A launched child reports its refusal to the launcher, which prints it
	// unchanged; a human verb prints the same line itself. serve keeps its own
	// output on standard error, where it logs.
	refuse := func(line string) int {
		if ready != nil {
			fmt.Fprintln(ready, "failed "+line)
			_ = ready.Close()
			ready = nil
		}
		if verb == "serve" {
			fmt.Fprintln(os.Stderr, line)
		} else {
			fmt.Println(line)
		}
		return 1
	}
	metasystemRoot, err := upMetasystemRoot(*root)
	if err != nil {
		return refuse(err.Error())
	}
	roots, err := lifecycle.ResolveRoots(*repo, metasystemRoot)
	if err != nil {
		return refuse(err.Error())
	}
	if verb == "start" || verb == "serve" || verb == "restart" {
		listenSet := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				listenSet = true
			}
		})
		listen, err = config.UIListen(filepath.Join(roots.Installation, "metasystem.conf"), listen, listenSet)
		if err != nil {
			return refuse(err.Error())
		}
		listen, err = lifecycle.ValidateListen(listen)
		if err != nil {
			return refuse(err.Error())
		}
	}
	prober := identity.KernelProber{}
	stop := lifecycle.StopOptions{Prober: prober, Wait: time.Duration(waitSeconds) * time.Second}
	start := func() lifecycle.Result {
		executable, err := os.Executable()
		if err != nil {
			return lifecycle.Result{Lines: []string{"cannot launch the interface server: " + err.Error()}, Code: 1}
		}
		return lifecycle.StartResult(lifecycle.LaunchSpec{
			Executable: executable,
			Args:       lifecycle.ServeArgs(roots.Checkout, roots.Installation, listen),
			Dir:        roots.Checkout,
			LogPath:    filepath.Join(lifecycle.Dir(roots.StateRoot), "server.log"),
		}, lifecycle.ExecSpawn, 0)
	}
	switch verb {
	case "start":
		return printUIResult(start())
	case "status":
		return printUIResult(lifecycle.StatusResult(roots.StateRoot, prober, nil))
	case "stop":
		return printUIResult(lifecycle.StopResult(roots.StateRoot, stop))
	case "restart":
		return printUIResult(lifecycle.RestartResult(roots.StateRoot, stop, start))
	case "serve":
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		subject, err := config.UISubject(filepath.Join(roots.Installation, "metasystem.conf"))
		if err != nil {
			return refuse(err.Error())
		}
		bundleDigest, bundle := interfaceBundle()
		err = lifecycle.Serve(ctx, lifecycle.Options{
			Roots: roots, Listen: listen, EngineBuild: supervise.BuildStamp, Prober: prober,
			NewHandler: func(bound net.Addr, rec lifecycle.Record) http.Handler {
				return httpd.New(httpd.Info{Checkout: rec.Checkout, StartedAt: rec.StartedAt, EngineBuild: rec.EngineBuild, ExecutableDigest: rec.ExecutableDigest, BundleDigest: bundleDigest,
					Describe: func() (workspace.Workspace, error) {
						return workspace.Describe(
							workspace.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot},
							workspace.Record{EngineBuild: rec.EngineBuild, StartedAt: rec.StartedAt, ExecutableDigest: rec.ExecutableDigest},
							subject,
						)
					},
					Project: func() (project.Thread, error) {
						return project.ReadThread(projectRoots(roots), time.Now().UTC())
					},
					Document: func(id string) (project.Document, error) {
						return project.Read(projectRoots(roots), id, time.Now().UTC())
					},
				}, bound, bundle)
			},
			Ready: func(address string) {
				if ready != nil {
					fmt.Fprintln(ready, "ready "+address)
					_ = ready.Close()
					ready = nil
				}
				fmt.Fprintf(os.Stderr, "interface running at http://%s (pid %d)\n", address, os.Getpid())
			},
		})
		if err != nil {
			return refuse(lifecycle.ServeFailure(roots.StateRoot, err))
		}
		return 0
	}
	return 2
}

// interfaceBundle reports the digest of the source the embedded bundle was
// built from and the bundle itself. The manifest is the publish point: without
// one this executable carries no bundle, so the handler receives none and says
// so rather than serving half a build.
func interfaceBundle() (string, fs.FS) {
	manifest, err := web.ReadManifest()
	if err != nil {
		return "", nil
	}
	return manifest.SourceDigest, web.Dist()
}

func printUIResult(result lifecycle.Result) int {
	for _, line := range result.Lines {
		fmt.Println(line)
	}
	return result.Code
}

// projectRoots carries the lifecycle's three roots to the reader that declares
// its own triple, which is the conversion this wiring exists for.
func projectRoots(roots lifecycle.Roots) project.Roots {
	return project.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot}
}
