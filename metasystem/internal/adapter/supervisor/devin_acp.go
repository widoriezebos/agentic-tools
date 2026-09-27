package supervisor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

// The Devin ACP wire plumbing shared by the delegate supervisor and the
// mission host turn: the per-attempt fifo pair and the server child. The
// shell adapters launched the server as a subshell that opened its stdout
// fifo FIRST and its stdin fifo SECOND, blocking in each open until the
// `acp turn` client opened the matching side; that pairing was the deadlock
// contract. The client is now in process (acp.RunFileTurn in a goroutine),
// so the supervisor cannot block in an open before the server exists. It
// holds "keeper" ends instead: a read-only keeper on the server's stdout
// fifo (so the server's stdout open cannot block and its early writes
// buffer rather than break), and a write-only keeper on the server's stdin
// fifo (so the server never reads a premature EOF before the client opens
// its write side). Neither keeper is a writer on the client's read side, so
// the client still sees EOF the moment the server dies; the keepers are
// released once the client has finished, which lets the server see EOF on
// its stdin exactly as it did when the client verb exited.

// ACPPipes is one attempt's fifo pair, named with a fresh nonce so it can
// never collide with an earlier blocked generation's endpoints.
type ACPPipes struct {
	ServerOut, ServerIn string

	mu        sync.Mutex
	keepers   []*os.File
	childEnds []*os.File
}

// TokenHex returns n random bytes as lowercase hex (the former
// `util token-hex --bytes n`).
func TokenHex(n int) (string, error) {
	if n < 1 {
		return "", errors.New("token-hex: --bytes must be positive")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("token-hex: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ACPPipeNames draws the per-attempt nonce and names the pair inside dir:
// DIR/acp-NONCE-out and DIR/acp-NONCE-in.
func ACPPipeNames(dir string) (*ACPPipes, error) {
	nonce, err := TokenHex(6)
	if err != nil {
		return nil, err
	}
	return &ACPPipes{
		ServerOut: filepath.Join(dir, "acp-"+nonce+"-out"),
		ServerIn:  filepath.Join(dir, "acp-"+nonce+"-in"),
	}, nil
}

// Make creates the fifo pair (`mkfifo OUT IN`). A pair that could not be
// completed is removed, so no half-made pipe is left as round evidence.
func (p *ACPPipes) Make() error {
	if err := syscall.Mkfifo(p.ServerOut, 0o666); err != nil {
		return fmt.Errorf("mkfifo %s: %w", p.ServerOut, err)
	}
	if err := syscall.Mkfifo(p.ServerIn, 0o666); err != nil {
		os.Remove(p.ServerOut)
		return fmt.Errorf("mkfifo %s: %w", p.ServerIn, err)
	}
	return nil
}

// ServerCommand builds the ACP server child: DEVIN acp, run in dir with
// argv0 set to argv0 (the census signature reads it; the binary ignores
// it), stdout into the server-out fifo, stdin from the server-in fifo,
// stderr to the given log. The fifo ends are opened here, without blocking,
// with their keepers; call Started after the command starts.
func (p *ACPPipes) ServerCommand(devinPath, argv0, dir string, env []string, stderr io.Writer) (*exec.Cmd, error) {
	outKeeper, err := os.OpenFile(p.ServerOut, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	p.keepers = append(p.keepers, outKeeper)
	serverStdout, err := os.OpenFile(p.ServerOut, os.O_WRONLY, 0)
	if err != nil {
		p.Release()
		return nil, err
	}
	p.childEnds = append(p.childEnds, serverStdout)
	// A write-only keeper needs a reader present to open without
	// blocking: a transient non-blocking reader provides it.
	transient, err := os.OpenFile(p.ServerIn, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		p.Release()
		return nil, err
	}
	inKeeper, err := os.OpenFile(p.ServerIn, os.O_WRONLY, 0)
	if err != nil {
		transient.Close()
		p.Release()
		return nil, err
	}
	p.keepers = append(p.keepers, inKeeper)
	serverStdin, err := os.OpenFile(p.ServerIn, os.O_RDONLY, 0)
	transient.Close()
	if err != nil {
		p.Release()
		return nil, err
	}
	p.childEnds = append(p.childEnds, serverStdin)
	command := exec.Command(devinPath, "acp")
	command.Args[0] = argv0
	command.Dir = dir
	command.Env = env
	command.Stdin = serverStdin
	command.Stdout = serverStdout
	command.Stderr = stderr
	return command, nil
}

// Started drops this process's copies of the server's own fifo ends.
func (p *ACPPipes) Started() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started()
}

func (p *ACPPipes) started() {
	for _, end := range p.childEnds {
		end.Close()
	}
	p.childEnds = nil
}

// Release closes every end this process still holds (the keepers and any
// child end of a server that never started).
func (p *ACPPipes) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started()
	for _, keeper := range p.keepers {
		keeper.Close()
	}
	p.keepers = nil
}

// Remove is the transport hygiene (KI-42): the fifo pair is wire plumbing,
// not evidence, and a named pipe left in a round or turn directory breaks
// any later evidence-tree copy. It releases the held ends and removes both
// names.
func (p *ACPPipes) Remove() {
	if p == nil {
		return
	}
	p.Release()
	os.Remove(p.ServerOut)
	os.Remove(p.ServerIn)
}

// startInProcessChild runs fn as a supervised child of this process: the
// in-process ACP client. It answers alive/wait/terminate like a process
// child; terminating it cancels its context (the courtesy session/cancel
// and the typed cancelled outcome still happen) and waits for it to return.
// Its pid is the supervisor's own: no separate process exists.
func startInProcessChild(pid int, fn func(ctx context.Context) int) *child {
	ctx, cancel := context.WithCancel(context.Background())
	c := &child{pid: pid, done: make(chan struct{}), stop: cancel}
	go func() {
		c.status = fn(ctx)
		cancel()
		close(c.done)
	}()
	return c
}
