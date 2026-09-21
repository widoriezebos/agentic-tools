package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func assertExactProcessEnded(t *testing.T, prober identity.Prober, ref identity.Ref, requireReaped bool) {
	t.Helper()
	exact, state, err := prober.Probe(ref.Pid)
	ended := err == nil && (state == identity.Dead || state == identity.Alive &&
		(!identity.SameIdentity(exact, ref) || !requireReaped && (exact.Zombie || exact.Exiting)))
	if !ended {
		t.Fatalf("process %d did not end before the operation returned: state=%s same=%t zombie=%t exiting=%t err=%v", ref.Pid, state,
			identity.SameIdentity(exact, ref), exact.Zombie, exact.Exiting, err)
	}
}

func TestResourceCommandExitedRootOutputHolderDrainsWithAndWithoutLease(t *testing.T) {
	t.Parallel()
	engine := buildResourceCustodyEngine(t)
	for _, completion := range []resourceCommandCompletion{resourceCommandCancelled, resourceCommandCompleted} {
		for _, withLease := range []bool{false, true} {
			name := string(completion) + "-without-lease"
			if withLease {
				name = string(completion) + "-with-lease"
			}
			t.Run(name, func(t *testing.T) {
				directory := t.TempDir()
				conf := ""
				var lease *HostResourceLease
				var err error
				if withLease {
					directory, conf = isolatedHostResources(t)
					lease, err = AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
					if err != nil {
						t.Fatal(err)
					}
				}
				rootPIDPath := filepath.Join(directory, "root.pid")
				childPIDPath := filepath.Join(directory, "child.pid")
				readyPath := filepath.Join(directory, "child.ready")
				rootExitPath := filepath.Join(directory, "root.exit")
				childReleasePath := filepath.Join(directory, "child.release")
				script := `echo $$ >"$1"
sh -c 'echo $$ >"$1"; : >"$2"; while [ ! -e "$3" ]; do sleep 0.02; done' sh "$2" "$3" "$5" &
while [ ! -e "$4" ]; do sleep 0.02; done
exit 0`
				command := exec.Command("sh", "-c", script, "sh", rootPIDPath, childPIDPath, readyPath, rootExitPath, childReleasePath)
				var output synchronizedBuffer
				command.Stdout, command.Stderr = &output, &output
				runCtx, cancel := context.WithCancel(WithResourceCustodyExecutable(context.Background(), engine))
				defer cancel()
				done := make(chan error, 1)
				rootReaped := make(chan struct{})
				deliverWait := make(chan struct{})
				selected := make(chan resourceCommandCompletion, 1)
				events := &resourceCommandEvents{
					afterWait: func(error) {
						close(rootReaped)
						if completion == resourceCommandCancelled {
							<-deliverWait
						}
					},
					selected: func(branch resourceCommandCompletion) {
						selected <- branch
						if completion == resourceCommandCancelled {
							close(deliverWait)
						} else {
							cancel()
						}
					},
				}
				go func() { done <- runResourceCommand(runCtx, command, lease, events) }()

				prober := identity.KernelProber{}
				waitCustodyFile(t, readyPath, 8*time.Second)
				root := waitCustodyRef(t, prober, rootPIDPath, 3*time.Second)
				child := waitCustodyRef(t, prober, childPIDPath, 3*time.Second)
				group, err := syscall.Getpgid(int(root.Pid))
				if err != nil || group == int(root.Pid) {
					t.Fatalf("worker did not join a distinct live custodian group: root=%d group=%d err=%v", root.Pid, group, err)
				}
				custodianExact, state, err := prober.Probe(int64(group))
				if err != nil || state != identity.Alive || custodianExact.Zombie || custodianExact.Exiting {
					t.Fatalf("custodian was not live before root exit: state=%s exact=%+v err=%v", state, custodianExact, err)
				}
				custodian := custodianExact.Ref()
				t.Cleanup(func() {
					for _, ref := range []identity.Ref{child, root, custodian} {
						if liveCustodyRef(prober, ref) {
							_ = identity.SignalExact(prober, ref, syscall.SIGKILL)
						}
					}
					if lease != nil {
						_ = lease.Close()
					}
				})
				if err := os.WriteFile(rootExitPath, []byte("exit\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				select {
				case <-rootReaped:
				case <-t.Context().Done():
					t.Fatalf("command root was not reaped at the controlled wait boundary: %v", context.Cause(t.Context()))
				}
				if completion == resourceCommandCancelled {
					cancel()
				}
				select {
				case runErr := <-done:
					if completion == resourceCommandCancelled && !errors.Is(runErr, context.Canceled) {
						t.Fatalf("cancel-selected output-holding command error=%v output=%q", runErr, output.String())
					}
					if completion == resourceCommandCompleted && (runErr == nil || errors.Is(runErr, context.Canceled) ||
						!strings.Contains(runErr.Error(), "native descendants survived direct worker completion")) {
						t.Fatalf("completion-selected output-holding command lost its custody diagnostic: error=%v output=%q", runErr, output.String())
					}
				case <-t.Context().Done():
					t.Fatalf("command did not drain before test cancellation: %v; output=%q", context.Cause(t.Context()), output.String())
				}
				assertExactProcessEnded(t, prober, child, false)
				assertExactProcessEnded(t, prober, custodian, true)
				if _, err := os.Stat(childReleasePath); !os.IsNotExist(err) {
					t.Fatalf("child release event was created during cleanup: %v", err)
				}
				if withLease {
					dirty, _, stateErr := hostLeaseState(directory)
					if stateErr != nil || dirty != 0 {
						t.Fatalf("settled custody retained dirty lease markers: dirty=%d err=%v", dirty, stateErr)
					}
					if branch := <-selected; branch != completion {
						t.Fatalf("selected completion branch=%s want=%s", branch, completion)
					}
					if err := lease.Close(); err != nil {
						t.Fatal(err)
					}
					lease = nil
					next, err := AcquireHostResources(t.Context(), directory, conf, "heavy", []string{"fixture-db"})
					if err != nil {
						t.Fatalf("clean lease was not reacquirable: %v", err)
					}
					if err := next.Close(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestResourceCommandPreservesSelectedPathAndArgumentZeroWithAndWithoutLease(t *testing.T) {
	t.Parallel()
	engine := buildResourceCustodyEngine(t)
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, withLease := range []bool{false, true} {
		name := "without-lease"
		if withLease {
			name = "with-lease"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			canonicalRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			tools := filepath.Join(root, "tools")
			if err := os.Mkdir(tools, 0o700); err != nil {
				t.Fatal(err)
			}
			selectedPath := filepath.Join(tools, "check")
			if err := os.Symlink(testBinary, selectedPath); err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(root, "witness.json")
			environment := overlayTestEnvironment(os.Environ(), map[string]string{
				"PATH":                        "tools:/usr/bin:/bin",
				custodyExecWitnessEnvironment: resultPath,
				"CUSTODY_EXEC_MARKER":         name,
			})
			command, err := explicitEnvironmentCommand(context.Background(), root, environment, []string{"check", "payload"})
			if err != nil || command.Path != selectedPath {
				t.Fatalf("explicit command path=%q want=%q err=%v", command.Path, selectedPath, err)
			}
			command.Args[0] = "deliberate-argv-zero"
			var lease *HostResourceLease
			var leaseMarker *os.File
			if withLease {
				lease, leaseMarker = invocationLocalHostResourceLease(t)
				defer lease.Close()
			}
			ctx := WithResourceCustodyExecutable(context.Background(), engine)
			if err := RunResourceCommand(ctx, command, lease); err != nil {
				t.Fatalf("run selected explicit command: %v", err)
			}
			data, err := os.ReadFile(resultPath)
			var witness custodyExecWitness
			if err != nil || json.Unmarshal(data, &witness) != nil {
				t.Fatalf("read custody exec witness: data=%q err=%v", data, err)
			}
			if !reflect.DeepEqual(witness.Args, []string{"deliberate-argv-zero", "payload"}) || witness.Directory != canonicalRoot || witness.Marker != name {
				t.Fatalf("custody exec witness=%+v", witness)
			}
			if command.Path != selectedPath || !reflect.DeepEqual(command.Args, []string{"deliberate-argv-zero", "payload"}) {
				t.Fatalf("restored command path=%q args=%v", command.Path, command.Args)
			}
			if leaseMarker != nil {
				record, _, err := readHostLeaseRecord(leaseMarker)
				if err != nil || !record.Cleared {
					t.Fatalf("completed command left invocation-local lease dirty: record=%+v err=%v", record, err)
				}
			}
		})
	}
}

func invocationLocalHostResourceLease(t *testing.T) (*HostResourceLease, *os.File) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	marker, acquired, err := tryHostFile(filepath.Join(directory, "lease-heavy-00000000000000000000000000000000"))
	if err != nil || !acquired {
		t.Fatalf("create invocation-local lease marker: acquired=%t err=%v", acquired, err)
	}
	owner, err := CurrentProcessIdentity(nil)
	if err != nil {
		marker.Close()
		t.Fatal(err)
	}
	record := hostLeaseRecord{Schema: 1, Owner: owner, Class: "heavy", Cleared: false}
	encoded, err := json.Marshal(record)
	if err == nil {
		_, err = marker.Write(encoded)
	}
	if err == nil {
		err = setHostLeaseCleared(marker, encoded, true)
	}
	if err != nil {
		marker.Close()
		t.Fatal(err)
	}
	return &HostResourceLease{files: []*os.File{marker}}, marker
}

func TestResourceCommandAlreadyCancelledStartsNothing(t *testing.T) {
	t.Parallel()
	engine := buildResourceCustodyEngine(t)
	for _, withLease := range []bool{false, true} {
		name := "without-lease"
		if withLease {
			name = "with-lease"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			var lease *HostResourceLease
			if withLease {
				var conf string
				directory, conf = isolatedHostResources(t)
				var err error
				lease, err = AcquireHostResources(context.Background(), directory, conf, "heavy", nil)
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
			}
			marker := filepath.Join(directory, "started")
			ctx, cancel := context.WithCancel(WithResourceCustodyExecutable(context.Background(), engine))
			cancel()
			command := exec.Command("sh", "-c", `printf started >"$1"`, "sh", marker)
			err := RunResourceCommand(ctx, command, lease)
			if !errors.Is(err, context.Canceled) || command.Process != nil {
				t.Fatalf("already-cancelled call err=%v process=%v", err, command.Process)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("already-cancelled command created its side-effect marker: %v", err)
			}
			if withLease {
				dirty, _, stateErr := hostLeaseState(directory)
				if stateErr != nil || dirty != 0 {
					t.Fatalf("already-cancelled call dirtied lease state: dirty=%d err=%v", dirty, stateErr)
				}
			}
		})
	}
}

// Cancellation must drain an ordinary descendant before either the host slot
// or its exclusive named resource can be handed to another command. The child
// deliberately keeps neither stdio nor lease descriptors open.
func TestGLEResourceCommandCancellationDrainsClosedFDDescendantBeforeRelease(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	directory, conf := isolatedHostResources(t)
	first, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	firstClosed := false
	t.Cleanup(func() {
		if !firstClosed {
			_ = first.Close()
		}
	})

	pidPath := filepath.Join(directory, "descendant.pid")
	readyPath := filepath.Join(directory, "descendant.ready")
	script := `(exec 3>&- 4>&- 5>&- 6>&- 7>&- 8>&- 9>&-; exec </dev/null >/dev/null 2>&1; trap '' TERM; : > "$2"; exec sleep 60) & echo $! > "$1"; wait`
	command := exec.Command("sh", "-c", script, "sh", pidPath, readyPath)
	runCtx, cancelRun := context.WithCancel(WithResourceCustodyExecutable(context.Background(), engine))
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- RunResourceCommand(runCtx, command, first) }()

	prober := identity.KernelProber{}
	waitCustodyFile(t, readyPath, 8*time.Second)
	child := waitCustodyRef(t, prober, pidPath, 3*time.Second)
	childGroup, err := syscall.Getpgid(int(child.Pid))
	if err != nil || childGroup == int(child.Pid) {
		t.Fatalf("native descendant was not held by a distinct custodian group: child=%d group=%d err=%v", child.Pid, childGroup, err)
	}
	custodian, state, err := prober.Probe(int64(childGroup))
	if err != nil || state != identity.Alive || custodian.Zombie || custodian.Exiting {
		t.Fatalf("native descendant custodian was not live: group=%d state=%s exact=%+v err=%v", childGroup, state, custodian, err)
	}
	t.Cleanup(func() {
		// Failure cleanup only: the assertion below observes death first.
		if liveCustodyRef(prober, child) {
			_ = identity.SignalExact(prober, child, syscall.SIGKILL)
		}
	})
	if err := identity.SignalExact(prober, child, syscall.SIGTERM); err != nil {
		t.Fatalf("send TERM to exact descendant: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !liveCustodyRef(prober, child) {
		t.Fatal("fixture descendant did not ignore TERM before cancellation")
	}

	contenderCtx, cancelContender := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelContender()
	type contender struct {
		name   string
		done   chan *HostResourceLease
		errors chan error
	}
	startContender := func(name, class string, exclusive []string) contender {
		waiting := contender{name: name, done: make(chan *HostResourceLease), errors: make(chan error, 1)}
		go func() {
			next, acquireErr := AcquireHostResources(contenderCtx, directory, conf, class, exclusive)
			if acquireErr != nil {
				waiting.errors <- acquireErr
				return
			}
			select {
			case waiting.done <- next:
			case <-contenderCtx.Done():
				_ = next.Close()
			}
		}()
		return waiting
	}
	capacity := startContender("capacity", "heavy", nil)
	named := startContender("named resource", "cheap", []string{"fixture-db"})
	for _, waiting := range []contender{capacity, named} {
		select {
		case next := <-waiting.done:
			_ = next.Close()
			t.Fatalf("%s acquired while original descendant was alive", waiting.name)
		case acquireErr := <-waiting.errors:
			t.Fatalf("%s contender refused instead of waiting for custody: %v", waiting.name, acquireErr)
		case <-time.After(150 * time.Millisecond):
		}
	}

	cancelRun()
	select {
	case runErr := <-runDone:
		if runErr == nil || !errors.Is(runErr, context.Canceled) {
			t.Fatalf("cancelled native command must remain non-green: %v", runErr)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("resource command did not settle after cancellation")
	}
	if liveCustodyRef(prober, child) {
		t.Fatalf("exact descendant %d survived cancelled command custody", child.Pid)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	firstClosed = true
	for _, waiting := range []contender{capacity, named} {
		select {
		case next := <-waiting.done:
			if liveCustodyRef(prober, child) {
				_ = next.Close()
				t.Fatalf("%s acquired while exact descendant %d was alive", waiting.name, child.Pid)
			}
			if err := MarkHostResourcesClean(next.Files()); err != nil {
				_ = next.Close()
				t.Fatal(err)
			}
			if err := next.Close(); err != nil {
				t.Fatal(err)
			}
		case acquireErr := <-waiting.errors:
			t.Fatalf("%s contender failed after exact descendant died: %v", waiting.name, acquireErr)
		case <-contenderCtx.Done():
			t.Fatalf("%s remained unavailable after exact descendant died: %v", waiting.name, contenderCtx.Err())
		}
	}
}
