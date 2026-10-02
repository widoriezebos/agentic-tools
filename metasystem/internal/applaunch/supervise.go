package applaunch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ChildSpec is the application's own process: the contract's start command,
// substituted, in its own directory, with the run's facts in its
// environment, writing to the run's log.
type ChildSpec struct {
	Argv        []string
	Dir         string
	Environment []string
	Log         *os.File
}

// Child is the application the supervisor owns for the run's life.
type Child interface {
	Pid() int
	Wait() (string, error)
}

// ChildSpawn starts the application. The engine's own spawn leaves the child
// in the supervisor's process group deliberately: that group is what makes a
// tree stoppable, and a group whose living leader is provably ours cannot be
// a reused id.
type ChildSpawn func(ChildSpec) (Child, error)

// ExecChild is the engine's spawn.
func ExecChild(spec ChildSpec) (Child, error) {
	if len(spec.Argv) == 0 {
		return nil, errors.New("the start command has no argument vector")
	}
	nullInput, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	command := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	command.Dir = spec.Dir
	command.Env = spec.Environment
	command.Stdin = nullInput
	command.Stdout = spec.Log
	command.Stderr = spec.Log
	if err := command.Start(); err != nil {
		_ = nullInput.Close()
		return nil, err
	}
	_ = nullInput.Close()
	return &execChild{command: command}, nil
}

type execChild struct{ command *exec.Cmd }

func (c *execChild) Pid() int { return c.command.Process.Pid }

func (c *execChild) Wait() (string, error) {
	err := c.command.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return "exit 0", nil
	case errors.As(err, &exit):
		return "exit " + fmt.Sprint(exit.ExitCode()), nil
	default:
		return "unknown: " + err.Error(), err
	}
}

// SuperviseOptions is everything the run's owner needs. The caller resolved
// the contract, the address, the state root, the log and the tree; the
// supervisor owns what happens to the process.
type SuperviseOptions struct {
	Context     context.Context
	StateRoot   string
	Seed        Record
	Contract    Contract
	ProjectRoot string
	Environment []string
	Spawn       ChildSpawn
	Prober      Prober
	Group       GroupReader
	Send        identity.SignalFunc
	SendGroup   func(pgid int64, sig syscall.Signal) error
	LeadsGroup  func() (int64, bool)
	Now         func() time.Time
	// ReadyDeadline starts the end of the readiness wait for the contract's
	// ready wait. Nil is a real timer of that length. A fixture gives one it
	// fires itself, or one that never fires, so that readiness is decided by
	// the application's own signal and never by a clock a loaded host outruns.
	ReadyDeadline func(wait time.Duration) <-chan time.Time
	// Ready and Failed report the one readiness answer back to whoever
	// launched this supervisor, as the interface's launcher is reported to.
	Ready  func(address string)
	Failed func(message string)
	// AfterSpawn runs between the spawn and the child's ref write. The engine
	// never sets it. It exists because that one instant is the only window
	// this design leaves open, and a fixture must be able to be interrupted
	// inside it to prove what the record and the verbs say afterwards.
	AfterSpawn func() error
}

func (o SuperviseOptions) prober() Prober {
	if o.Prober != nil {
		return o.Prober
	}
	return identity.KernelProber{}
}

func (o SuperviseOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o SuperviseOptions) leadsGroup() (int64, bool) {
	if o.LeadsGroup != nil {
		return o.LeadsGroup()
	}
	return ownGroupLeadership()
}

// ownGroupLeadership answers whether this process leads its own process
// group. Only a leader may signal its group: its living identity is the
// proof that the group id is not a reused one.
func ownGroupLeadership() (int64, bool) {
	self := os.Getpid()
	group, err := syscall.Getpgid(self)
	if err != nil {
		return 0, false
	}
	return int64(group), group == self
}

// Supervise is the run's owner. It writes the record before it spawns
// anything, writes the application's identity as the very next act after the
// spawn, waits for readiness, and then waits on the application for the run's
// life. It never deletes the record: a run ends into an ended record, which
// only stop, reset or the next start of that ref removes.
func Supervise(o SuperviseOptions) error {
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	prober := o.prober()
	fail := func(message string) error {
		if o.Failed != nil {
			o.Failed(message)
		}
		return errors.New(message)
	}
	self, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return fail("the application supervisor cannot read its own identity")
	}
	supervisor, err := identity.EncodeRef(self.Ref())
	if err != nil {
		return fail("the application supervisor cannot record its own identity: " + err.Error())
	}
	group, _ := o.leadsGroup()

	record := o.Seed
	record.SchemaVersion = RecordSchemaVersion
	record.Supervisor, record.Group = supervisor, group
	record.ContractDigest = o.Contract.Digest()
	record.ReadyForm = o.Contract.ReadyKind()
	record.Data = o.Contract.DataWord()
	record.StartedAt = o.now().UTC().Format(time.RFC3339)
	record.Child, record.ReadyAt, record.Ended = "", "", nil
	if record.Log == "" {
		record.Log = DefaultLogPath(o.StateRoot, record.Key)
	}
	if err := os.MkdirAll(filepath.Dir(record.Log), 0o755); err != nil {
		return fail("cannot make the run's log directory: " + err.Error())
	}
	log, err := os.OpenFile(record.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fail("cannot open the run's log: " + err.Error())
	}
	defer log.Close()
	// The log readiness form is scoped to this run: the pattern is sought
	// from the offset at which this run's supervisor opened the log, so a
	// line an earlier run wrote can never make this one ready.
	offset := int64(0)
	if info, statErr := log.Stat(); statErr == nil {
		offset = info.Size()
	}

	// The record is written before anything is spawned. An engine that dies
	// before this has left no run; a supervisor that dies after it leaves a
	// record whose supervisor ref is dead, and every verb reads that.
	if err := WriteRecord(o.StateRoot, record); err != nil {
		return fail("cannot write the run record: " + err.Error())
	}

	facts := FactsFor(record.Address)
	spawn := o.Spawn
	if spawn == nil {
		spawn = ExecChild
	}
	child, err := spawn(ChildSpec{
		Argv:        facts.Argv(o.Contract.Start),
		Dir:         filepath.Join(o.ProjectRoot, filepath.FromSlash(o.Contract.Start.CWD)),
		Environment: facts.Environment(o.Environment, record.StateRoot, record.Log),
		Log:         log,
	})
	if err != nil {
		message := "the start command could not be run: " + err.Error()
		record.Ended = &Ended{At: o.now().UTC().Format(time.RFC3339), ExitStatus: message}
		_ = WriteRecord(o.StateRoot, record)
		return fail(message)
	}
	if o.AfterSpawn != nil {
		if err := o.AfterSpawn(); err != nil {
			return err
		}
	}
	// The very next act after the spawn: the application's own identity.
	childExact, childState, err := prober.Probe(int64(child.Pid()))
	if err == nil && childState == identity.Alive {
		if encoded, encodeErr := identity.EncodeRef(childExact.Ref()); encodeErr == nil {
			record.Child = encoded
			_ = WriteRecord(o.StateRoot, record)
		}
	}

	var exitStatus atomic.Value
	exited := make(chan struct{})
	go func() {
		status, _ := child.Wait()
		exitStatus.Store(status)
		close(exited)
	}()
	alive := func() bool {
		select {
		case <-exited:
			return false
		default:
			return true
		}
	}

	readyWait := time.Duration(o.Contract.ReadyWaitMS()) * time.Millisecond
	var expired <-chan time.Time
	if o.ReadyDeadline != nil {
		expired = o.ReadyDeadline(readyWait)
	} else {
		timer := time.NewTimer(readyWait)
		expired = timer.C
		defer timer.Stop()
	}
	readyErr := AwaitReady(ctx, o.Contract, record.Address, record.Log, offset, alive, expired)
	if readyErr != nil {
		message := readyErr.Error() + "; see " + record.Log
		if o.Failed != nil {
			o.Failed(message)
		}
		// A readiness failure ends the application by its own recorded ref,
		// never only the supervisor: the run must not outlive the engine's
		// belief that it never started.
		o.endChild(record, exited)
		o.finish(ctx, record, exitStatus, exited, log, groupReadSayer(log, record.Group), false)
		return errors.New(message)
	}
	record.ReadyAt = o.now().UTC().Format(time.RFC3339)
	_ = WriteRecord(o.StateRoot, record)
	if o.Ready != nil {
		o.Ready(record.Address)
	}

	select {
	case <-exited:
	case <-ctx.Done():
		// A signal to the supervisor ends the owned tree, not just this
		// process: the application by its re-proven ref first.
		o.endChild(record, exited)
	}
	// The supervisor does not leave while any process of its own group
	// remains: that reads as "child ended, descendants alive" and it stays
	// the owner. What ends such a group is a stop, and only this process may
	// signal that group, because only its living identity as the group's
	// leader proves the group is ours and not a reused id.
	say := groupReadSayer(log, record.Group)
	groupEnded := false
	if !o.awaitGroupEmpty(ctx, record, say) {
		o.endGroup(record)
		groupEnded = true
	}
	o.finish(ctx, record, exitStatus, exited, log, say, groupEnded)
	return nil
}

// groupReadSayer writes into the run's log why the supervisor is still
// waiting on a group it cannot read, once per distinct reason.
func groupReadSayer(log *os.File, group int64) func(error) {
	last := ""
	return func(err error) {
		if err == nil || err.Error() == last {
			return
		}
		last = err.Error()
		_, _ = fmt.Fprintf(log, "application supervisor: group %d could not be inspected (%v); it is not taken as empty, still waiting\n", group, err)
	}
}

// stateSayer writes into the run's log the state a supervisor past its wait
// is in, once per distinct state.
func stateSayer(log *os.File) func(string) {
	last := ""
	return func(state string) {
		if state == last {
			return
		}
		last = state
		_, _ = fmt.Fprintf(log, "application supervisor: %s; the supervisor stays the owner, still waiting\n", state)
	}
}

// awaitGroupEmpty waits out the run's descendants. It answers false when the
// wait was ended by a signal to this supervisor instead of by the group
// emptying, which is the moment the group itself must be ended. A group that
// cannot be read is never an empty group: the wait goes on and says why.
func (o SuperviseOptions) awaitGroupEmpty(ctx context.Context, record Record, say func(error)) bool {
	for {
		members, err := o.ownedMembers(record)
		if err != nil {
			say(err)
		} else if len(members) == 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// finish waits out the run's descendants and writes the ended record. The
// supervisor never leaves while a process of its own group remains, and a
// group it cannot inspect is never taken as empty: past its wait it says the
// state as it is and keeps waiting, so no ended record is ever written over
// something it started that still runs. A signal to it while it waits ends
// its group, once, by the leader's own signal.
func (o SuperviseOptions) finish(ctx context.Context, record Record, exitStatus atomic.Value, exited <-chan struct{}, log *os.File, say func(error), groupEnded bool) {
	select {
	case <-exited:
	case <-time.After(time.Duration(o.Contract.StopWaitMS()) * time.Millisecond):
	}
	sayState := stateSayer(log)
	deadline := time.Now().Add(time.Duration(o.Contract.StopWaitMS()) * time.Millisecond)
	for {
		members, err := o.ownedMembers(record)
		if err == nil && len(members) == 0 {
			break
		}
		if !time.Now().Before(deadline) {
			if err != nil {
				say(err)
				sayState(fmt.Sprintf("group not inspectable: %v", err))
			} else {
				sayState(fmt.Sprintf("child ended, descendants alive: %d member(s)", len(members)))
			}
		}
		if !groupEnded && ctx.Err() != nil {
			o.endGroup(record)
			groupEnded = true
			continue
		}
		time.Sleep(200 * time.Millisecond)
	}
	status, _ := exitStatus.Load().(string)
	if status == "" {
		status = "unknown"
	}
	record.Ended = &Ended{At: o.now().UTC().Format(time.RFC3339), ExitStatus: status}
	_ = UpdateRecord(o.StateRoot, record.Key, func(current *Record) {
		current.Ended = record.Ended
		current.Child = record.Child
	})
}

// ownedMembers reports the live members of the supervisor's own group other
// than the supervisor itself, which leads the group and is in it. A process
// that does not lead the recorded group owns none of it: it can neither wait
// for it nor signal it, because nothing there is provably its own.
func (o SuperviseOptions) ownedMembers(record Record) ([]Member, error) {
	group, leads := o.leadsGroup()
	if !leads || record.Group < 1 || group != record.Group {
		return nil, nil
	}
	reader := o.Group
	if reader == nil {
		reader = KernelGroup
	}
	members, err := reader(record.Group)
	if err != nil {
		return nil, err
	}
	return livingBesides(members, int64(os.Getpid())), nil
}

// endChild ends the application by its recorded ref, re-proven immediately
// before each signal, and never by a bare number.
func (o SuperviseOptions) endChild(record Record, exited <-chan struct{}) {
	ref, recorded, err := record.ChildRef()
	if !recorded || err != nil {
		return
	}
	prober := o.prober()
	if err := signal(prober, ref, syscall.SIGTERM, o.Send); err != nil {
		return
	}
	select {
	case <-exited:
		return
	case <-time.After(time.Duration(o.Contract.StopWaitMS()) * time.Millisecond):
	}
	if identity.AliveRef(prober, ref) == identity.Alive {
		_ = signal(prober, ref, syscall.SIGKILL, o.Send)
	}
}

// endGroup ends what the application left behind. Only this process may do
// it, and only while it leads the group: a KILL here ends the supervisor
// too, so the ended record and the evidence copy after it belong to whoever
// asked for the stop.
func (o SuperviseOptions) endGroup(record Record) {
	group, leads := o.leadsGroup()
	if !leads || record.Group < 1 || group != record.Group {
		return
	}
	// A group that cannot be read is not an empty one; the leader's own
	// signal to its group needs no census of who is in it.
	if members, err := o.ownedMembers(record); err == nil && len(members) == 0 {
		return
	}
	send := o.SendGroup
	if send == nil {
		send = func(pgid int64, sig syscall.Signal) error { return syscall.Kill(int(-pgid), sig) }
	}
	_ = send(record.Group, syscall.SIGTERM)
	deadline := time.Now().Add(time.Duration(o.Contract.StopWaitMS()) * time.Millisecond)
	for time.Now().Before(deadline) {
		if members, err := o.ownedMembers(record); err == nil && len(members) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = send(record.Group, syscall.SIGKILL)
}

// signal re-proves the identity immediately before it sends, so a recorded
// number a different process has taken is never signalled.
func signal(prober Prober, ref identity.Ref, sig syscall.Signal, send identity.SignalFunc) error {
	if send != nil {
		return identity.SignalExact(prober, ref, sig, send)
	}
	return identity.SignalExact(prober, ref, sig)
}
