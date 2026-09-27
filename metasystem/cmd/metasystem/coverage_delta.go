package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// coverageDeltaRelaunchedVariable marks the admitted proof's suite command:
// the coverage delta relaunched under proof-run launch.
const coverageDeltaRelaunchedVariable = "METASYSTEM_COVERAGE_DELTA_RELAUNCHED"

// runProofRunCoverageDelta checks coverage only for the packages a landing
// names or touches (proofrun.CoverageDelta):
// `proof-run coverage-delta [--base REF | --staged | PACKAGE ...] [--ratchet PATH] [--root INSTALLATION]`.
func runProofRunCoverageDelta(args []string) int {
	flags := flag.NewFlagSet("proof-run coverage-delta", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := pathFlag(flags, "root", "", "installation whose module, registry and ratchets are used (default: this engine's)")
	base := flags.String("base", "", "select packages touched since this ref")
	staged := flags.Bool("staged", false, "select packages with staged Go files")
	ratchet := flags.String("ratchet", "", "coverage ratchet (default: the installation's, per platform)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, proofrun.CoverageDeltaUsage)
			return 0
		}
		fmt.Fprintln(os.Stderr, "coverage delta:", err)
		fmt.Fprintln(os.Stderr, proofrun.CoverageDeltaUsage)
		return 2
	}
	installation := *root
	if installation == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "coverage delta:", err)
			return 1
		}
		installation = filepath.Dir(filepath.Dir(exe))
	}
	installation, err := canonicalPath(installation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverage delta:", err)
		return 1
	}
	invocationDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverage delta:", err)
		return 1
	}
	engine := os.Getenv("METASYSTEM_BIN")
	if engine == "" {
		engine = filepath.Join(installation, "bin", "metasystem")
	}
	relaunched := os.Getenv(coverageDeltaRelaunchedVariable) == "1"
	return proofrun.CoverageDelta(proofrun.CoverageDeltaOptions{
		Root: installation, InvocationDir: invocationDir,
		Base: *base, Staged: *staged, Packages: flags.Args(), Ratchet: *ratchet,
		Engine: engine, GOOS: runtime.GOOS, Relaunched: relaunched,
		Git: func(args ...string) (string, error) {
			command := exec.Command("git", append([]string{"-C", installation}, args...)...)
			command.Env = gittree.ScrubbedEnviron()
			output, err := command.Output()
			return string(output), err
		},
		Reuse: func(ratchet string, packages []string) ([]string, bool, error) {
			return coverageDeltaReuse(installation, ratchet, packages)
		},
		WorkerAuthorized: func() bool { return coverageDeltaWorkerAuthorized(installation, engine) },
		Launch: func(ratchet string, packages []string) int {
			return coverageDeltaLaunch(installation, engine, ratchet, packages)
		},
		GoTest: func(pkg string) (string, int) {
			// A hang bound for one package without the race detector, with a
			// wide margin above the slowest package.
			command := exec.Command("go", "test", "-cover", "-timeout", "30m", pkg)
			command.Dir = installation
			command.Env = withoutEnvironment(os.Environ(), coverageDeltaRelaunchedVariable)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			switch {
			case err == nil:
				return string(output), 0
			case errors.As(err, &exit):
				return string(output), exit.ExitCode()
			default:
				return string(output) + err.Error() + "\n", 1
			}
		},
		Out: os.Stdout, Err: os.Stderr,
	})
}

// coverageDeltaReuse projects retained full-gate coverage under the control
// root a proof names, judging the toolchain with module downloads disabled.
func coverageDeltaReuse(executionRoot, ratchet string, packages []string) ([]string, bool, error) {
	controlRoot := ""
	for _, candidate := range []string{os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_RUN_ROOT"), os.Getenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")} {
		if candidate != "" {
			controlRoot = candidate
			break
		}
	}
	if controlRoot == "" {
		controlRoot = executionRoot
	}
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil {
		return nil, false, err
	}
	environment := append(withoutEnvironment(os.Environ(), "GOFLAGS"), "GOFLAGS=-mod=readonly")
	return coverageReuseLines(canonicalControl, executionRoot, ratchet, packages, environment)
}

// coverageDeltaWorkerAuthorized asks the proof's authenticating engine
// (METASYSTEM_PROOF_AUTH_BIN, else this installation's) whether this process
// is a worker of a live admitted proof.
func coverageDeltaWorkerAuthorized(root, engine string) bool {
	authenticator := os.Getenv("METASYSTEM_PROOF_AUTH_BIN")
	if authenticator == "" {
		authenticator = engine
	}
	if info, err := os.Stat(authenticator); err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return false
	}
	command := exec.Command(authenticator, "proof-run", "worker-authorized", "--root", root)
	return command.Run() == nil
}

// coverageDeltaLaunch admits the coverage delta as one affected-package proof
// in this process, exactly as the script's exec into proof-run launch did,
// with the relaunched delta as its suite command.
func coverageDeltaLaunch(root, engine, ratchet string, packages []string) int {
	run := fmt.Sprintf("%s-%d-%d", time.Now().UTC().Format("20060102T150405Z"), os.Getpid(), rand.IntN(32768))
	args, err := coverageDeltaLaunchArguments(root, engine, ratchet, run, packages)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverage delta: could not bind the selected ratchet")
		return 1
	}
	return runProofRunLaunch(args)
}

// coverageDeltaLaunchArguments is the proof-run launch argv for one run: the
// proof's identity binds the ratchet digest and every package, and the suite
// command relaunches the delta as an authenticated worker of that proof.
func coverageDeltaLaunchArguments(root, engine, ratchet, run string, packages []string) ([]string, error) {
	progress := filepath.Join(root, "artifacts", "agents", "supervision", "coverage-delta-"+run+".progress.jsonl")
	logPath := filepath.Join(root, "artifacts", "agents", "supervision", "suite-logs", "coverage-delta-"+run+".log")
	ratchetBytes, err := os.ReadFile(ratchet)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(ratchetBytes)
	args := []string{"--suite", "coverage-delta", "--root", root, "--conf", filepath.Join(root, "metasystem.conf"),
		"--progress", progress, "--log", logPath, "--banner", proofRunBannerText("coverage-delta", root, progress, logPath),
		"--scope", "coverage", "--command-class", "coverage-delta",
		"--identity-input", "coverage-ratchet=" + hex.EncodeToString(digest[:])}
	for _, pkg := range packages {
		args = append(args, "--identity-input", "coverage-package="+pkg)
	}
	args = append(args, "--", "env", coverageDeltaRelaunchedVariable+"=1", "METASYSTEM_PROOF_AUTH_BIN="+engine,
		engine, "internal", "proof-run", "coverage-delta", "--root", root, "--ratchet", ratchet, "--")
	return append(args, packages...), nil
}

func withoutEnvironment(environment []string, name string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, name+"=") {
			kept = append(kept, entry)
		}
	}
	return kept
}
