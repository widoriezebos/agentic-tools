package supervisor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// The protocol transport's wire plumbing (ACP), shared by every runtime that
// declares a protocol launch, in the delegate round and the mission host
// turn: the per-attempt fifo pair and the server child. The shell adapters launched the server as a subshell that opened its stdout
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

// ServerCommand builds the protocol server child: program with args, run in
// dir with argv[0] set to argv0 (the census signature reads it; the binary
// ignores it), stdout into the server-out fifo, stdin from the server-in
// fifo, stderr to the given log. The fifo ends are opened here, without
// blocking, with their keepers; call Started after the command starts.
func (p *ACPPipes) ServerCommand(program, argv0 string, args []string, dir string, env []string, stderr *os.File) (*exec.Cmd, error) {
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
	command := exec.Command(program, args...)
	command.Args[0] = argv0
	command.Dir = dir
	command.Env = env
	command.Stdin = serverStdin
	command.Stdout = serverStdout
	if stderr != nil {
		// Only a real file: a nil *os.File in the interface would be
		// written to, and a non-file writer would make Wait a pipe drain.
		command.Stderr = stderr
	}
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

// Unblock frees a client stuck opening its ends after the server is gone
// (a server that died, or never started, before the client's open). A FIFO
// open blocks until its counterpart opens, and a blocked open cannot be
// cancelled, so until done closes this repeatedly opens and closes a
// momentary counterpart without blocking: a writer on the server-out fifo
// (the client's read open completes and reads EOF) and a reader on the
// server-in fifo (the client's write open completes and its writes fail).
// Neither carries data.
func (p *ACPPipes) Unblock(done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if writer, err := os.OpenFile(p.ServerOut, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			writer.Close()
		}
		if reader, err := os.OpenFile(p.ServerIn, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			reader.Close()
		}
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
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
