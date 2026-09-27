package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

const buildStampFlag = "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="

var positiveInteger = regexp.MustCompile(`^[1-9][0-9]*$`)

type buildOptions struct {
	out      string
	trimpath bool
}

func parseBuildArgs(args []string) (buildOptions, string) {
	var options buildOptions
	for len(args) > 0 {
		switch args[0] {
		case "--out":
			if len(args) < 2 || args[1] == "" {
				return options, "go-build: --out needs a path"
			}
			options.out = args[1]
			args = args[2:]
		case "--trimpath":
			options.trimpath = true
			args = args[1:]
		default:
			return options, "go-build: unknown argument: " + args[0]
		}
	}
	return options, ""
}

// runBuild compiles ./cmd/metasystem. With --out it is the PROOF build: the
// engine goes to PATH and bin/metasystem stays untouched, because a
// supervision-armed checkout fingerprints the live binary and a commit-time
// swap under an armed watch is exactly what the fingerprint refuses.
// Without it, it stages beside bin/metasystem and renames over it, under the
// gate fence. CGO is pinned off so link portability is deliberate, and the
// commit stamp makes operational artifacts self-attest.
func runBuild(ctx context.Context, args []string, root string, d deps) int {
	options, usage := parseBuildArgs(args)
	if usage != "" {
		fmt.Fprintln(d.stderr, usage)
		return 2
	}

	workers := d.getenv("METASYSTEM_TEST_WORKERS")
	if workers == "" {
		workers = "1"
	}
	if !positiveInteger.MatchString(workers) {
		fmt.Fprintln(d.stderr, "go-build: METASYSTEM_TEST_WORKERS must be a positive integer")
		return 1
	}

	if d.lookGo() != nil {
		fmt.Fprintln(d.stderr, "go-build: no go toolchain on PATH; the engine cannot be built")
		return 1
	}

	if options.out == "" && d.getenv("METASYSTEM_ALLOW_CONCURRENT_GATE") != "1" && executableFile(filepath.Join(root, "bin", "metasystem")) {
		holders := d.fence(root, d.selfPid)
		for _, holder := range holders {
			fmt.Fprintf(d.stderr, "gate %s is running as pid %d\n", holder.Gate, holder.Pid)
		}
		if len(holders) > 0 {
			fmt.Fprintln(d.stderr, "go-build: a live gate run owns this checkout; rebuilding now would swap its binary mid-run (METASYSTEM_ALLOW_CONCURRENT_GATE=1 overrides)")
			return 1
		}
	}

	stamp, err := buildStamp(ctx, root, d)
	if err != nil {
		fmt.Fprintf(d.stderr, "go-build: cannot classify the working tree against the compiled ENGINE policy: %v\n", err)
		return 1
	}

	env := append(d.environ(), "GOMAXPROCS="+workers, "CGO_ENABLED=0")
	ldflags := buildStampFlag + stamp
	if options.out != "" {
		goArgs := []string{"build", "-p=" + workers, "-buildvcs=false"}
		if options.trimpath {
			goArgs = append(goArgs, "-trimpath")
		}
		goArgs = append(goArgs, "-ldflags", ldflags, "-o", options.out, "./cmd/metasystem")
		if d.goTool(ctx, root, env, goArgs, d.stdout, d.stderr) != nil {
			fmt.Fprintln(d.stderr, "go-build: build failed")
			return 1
		}
		fmt.Fprintf(d.stdout, "go-build: proof engine @ %s (CGO_ENABLED=0); bin/metasystem untouched\n", stamp)
		return 0
	}

	// Build beside the target and rename over it: go build refuses to
	// overwrite a non-object file (exactly the stale or foreign case this
	// build exists to replace), and the atomic rename never leaves a
	// half-written binary where a live process might exec it.
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		fmt.Fprintf(d.stderr, "go-build: cannot create bin: %v\n", err)
		return 1
	}
	staging := filepath.Join("bin", ".metasystem.build."+strconv.FormatInt(d.selfPid, 10))
	defer func() { _ = os.Remove(filepath.Join(root, staging)) }()
	goArgs := []string{"build", "-p=" + workers, "-buildvcs=false", "-ldflags", ldflags, "-o", staging, "./cmd/metasystem"}
	if d.goTool(ctx, root, env, goArgs, d.stdout, d.stderr) != nil {
		fmt.Fprintln(d.stderr, "go-build: build failed")
		return 1
	}
	if err := os.Rename(filepath.Join(root, staging), filepath.Join(root, "bin", "metasystem")); err != nil {
		fmt.Fprintf(d.stderr, "go-build: cannot install bin/metasystem: %v\n", err)
		return 1
	}
	fmt.Fprintf(d.stdout, "go-build: bin/metasystem @ %s (CGO_ENABLED=0)\n", stamp)
	return 0
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

// buildStamp is the enclosing commit only while the compiled ENGINE surface
// matches HEAD. A changed or untracked engine path makes it a development
// stamp, which automatic enrollment refuses. METASYSTEM_BUILD_STAMP (the
// witness path's judged engine-input digest) overrides it. VCS stamping is
// pinned off either way: the explicit stamp is the attestation.
func buildStamp(ctx context.Context, root string, d deps) (string, error) {
	if override := d.getenv("METASYSTEM_BUILD_STAMP"); override != "" {
		return override, nil
	}
	head, err := d.git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "unknown", nil
	}
	commit := strings.TrimSpace(string(head))
	if commit == "" {
		return "unknown", nil
	}
	prefixOutput, err := d.git(ctx, root, "rev-parse", "--show-prefix")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-prefix: %w", err)
	}
	prefix := strings.TrimSuffix(strings.TrimRight(string(prefixOutput), "\r\n"), "/")
	changed, err := d.git(ctx, root, "diff", "--name-only", "--no-renames", "-z", "HEAD", "--")
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	untracked, err := d.git(ctx, root, "ls-files", "--others", "--exclude-standard", "--full-name", "-z")
	if err != nil {
		return "", fmt.Errorf("git ls-files: %w", err)
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return "", err
	}
	for _, name := range bytes.Split(append(changed, untracked...), []byte{0}) {
		if len(name) == 0 {
			continue
		}
		included, err := policy.Includes(behaviorsurface.Engine, string(name), prefix)
		if err != nil {
			return "", err
		}
		if included {
			return enginebuild.DevelopmentStamp(commit), nil
		}
	}
	return commit, nil
}
