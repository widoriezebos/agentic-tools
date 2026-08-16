package acp

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Conn is one ACP connection over pre-opened pipes (launch and
// custody stay with scripts — the client owns only the wire). The
// read loop routes each frame by its JSON-RPC shape; the write
// loop owns the blocking writes so that EVERY caller wait is
// context-bounded — a server that stops reading can wedge the
// writer goroutine, never a deadline (slice-two critique F1). A
// malformed frame, a mismatched response ID, or a write failure is
// a connection death: recorded, channels closed, pending calls
// failed — the matrix's "protocol error; record; teardown" row.
type Conn struct {
	reader  *Reader
	writer  *Writer
	nextID  int64
	lastSeq atomic.Uint64

	mu      sync.Mutex
	pending map[string]chan Frame
	err     error

	sends    chan sendRequest
	requests chan Frame
	notes    chan Frame
	done     chan struct{}
	closed   chan struct{}
	dieOnce  sync.Once
}

// Frame pairs a message with its wire arrival sequence. Channel
// hand-off loses ordering BETWEEN channels (a post-response
// notification can be selected before the response), so the
// sequence is the only truthful arrival order — the prompt window
// is a sequence range, never a drain race.
type Frame struct {
	Msg *Message
	Seq uint64
}

type sendRequest struct {
	msg  *Message
	done chan error
}

// NewConn starts the read and write loops over the pipe ends.
// journal receives every frame in both directions and is required
// for production turns (settlement evidence); the shared sink is
// serialized so concurrent directions can never interleave into
// invalid evidence (critique F9).
func NewConn(readEnd io.Reader, writeEnd io.Writer, journal io.Writer) *Conn {
	if journal != nil {
		journal = &lockedWriter{w: journal}
	}
	c := &Conn{
		reader:   NewReader(readEnd, journal),
		writer:   NewWriter(writeEnd, journal),
		pending:  map[string]chan Frame{},
		sends:    make(chan sendRequest),
		requests: make(chan Frame, 64),
		notes:    make(chan Frame, 4096),
		done:     make(chan struct{}),
		closed:   make(chan struct{}),
	}
	go c.readLoop()
	go c.writeLoop()
	return c
}

// lockedWriter serializes the shared journal sink; each journal
// line is a single Write (wire.go builds whole lines), so the lock
// is the whole interleaving story.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}

func (c *Conn) readLoop() {
	defer func() {
		c.die()
		c.mu.Lock()
		for key, ch := range c.pending {
			close(ch)
			delete(c.pending, key)
		}
		c.mu.Unlock()
		close(c.requests)
		close(c.notes)
		close(c.done)
	}()
	seq := uint64(0)
	for {
		msg, err := c.reader.Next()
		if err != nil {
			c.setErr(err)
			return
		}
		seq++
		c.lastSeq.Store(seq)
		frame := Frame{Msg: msg, Seq: seq}
		switch msg.Classify() {
		case KindResponse:
			c.mu.Lock()
			ch := c.pending[msg.ID.Key()]
			delete(c.pending, msg.ID.Key())
			c.mu.Unlock()
			if ch == nil {
				// A response nobody asked for is a mismatched ID —
				// the matrix's protocol-error row.
				c.setErr(fmt.Errorf("acp: response for unknown id %s", msg.ID.Key()))
				return
			}
			ch <- frame
		case KindServerRequest:
			select {
			case c.requests <- frame:
			case <-c.closed:
				return
			}
		case KindNotification:
			select {
			case c.notes <- frame:
			case <-c.closed:
				return
			}
		default:
			c.setErr(fmt.Errorf("acp: malformed frame shape (jsonrpc=%q id=%v method=%q)", msg.JSONRPC, msg.ID, msg.Method))
			return
		}
	}
}

// writeLoop owns the blocking writes. A write error is a
// connection death; the loop keeps serving so enqueued callers
// always get an answer.
func (c *Conn) writeLoop() {
	for {
		select {
		case request := <-c.sends:
			err := c.writer.Send(request.msg)
			if err != nil {
				c.setErr(fmt.Errorf("acp: write failed: %w", err))
				c.die()
			}
			request.done <- err
		case <-c.closed:
			return
		}
	}
}

// die trips the closed signal exactly once so senders and callers
// stop waiting. The read loop's defer owns channel closing.
func (c *Conn) die() {
	c.dieOnce.Do(func() { close(c.closed) })
}

func (c *Conn) setErr(err error) {
	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

// Err reports why the connection died; io.EOF is a clean peer
// close, anything else is a protocol error. Valid after Done.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Done closes when the read loop has terminated and every channel
// is closed.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Requests delivers server→client requests in arrival order.
func (c *Conn) Requests() <-chan Frame { return c.requests }

// Notifications delivers notifications in arrival order.
func (c *Conn) Notifications() <-chan Frame { return c.notes }

// LastSeq reports the newest routed inbound sequence — the fence
// sample point for the prompt window's lower bound.
func (c *Conn) LastSeq() uint64 { return c.lastSeq.Load() }

// JournalErr surfaces the first journal failure on either
// direction — settlement evidence must not silently thin.
func (c *Conn) JournalErr() error {
	if err := c.reader.JournalErr(); err != nil {
		return err
	}
	return c.writer.JournalErr()
}

// enqueue hands a frame to the write loop, bounded by the caller's
// context even when the physical write is wedged (the writer
// goroutine stays stuck; the CALLER never does).
func (c *Conn) enqueue(ctx context.Context, m *Message) error {
	select {
	case <-c.closed:
		if err := c.Err(); err != nil {
			return err
		}
		return io.ErrClosedPipe
	default:
	}
	request := sendRequest{msg: m, done: make(chan error, 1)}
	select {
	case c.sends <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		if err := c.Err(); err != nil {
			return err
		}
		return io.ErrClosedPipe
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		// The write may still complete later; the caller's wait is
		// what the deadline bounds.
		return ctx.Err()
	}
}

// Call sends a request and waits for its response, the context
// bounding both the write hand-off and the response wait.
func (c *Conn) Call(ctx context.Context, method string, params any) (*Message, error) {
	frame, err := c.CallSeq(ctx, method, params)
	return frame.Msg, err
}

// CallSeq is Call carrying the response's arrival sequence, for
// callers that bound a window by it.
func (c *Conn) CallSeq(ctx context.Context, method string, params any) (Frame, error) {
	c.mu.Lock()
	c.nextID++
	id := NewRequestID(c.nextID)
	ch := make(chan Frame, 1)
	c.pending[id.Key()] = ch
	c.mu.Unlock()

	if err := c.enqueue(ctx, &Message{JSONRPC: "2.0", ID: id, Method: method, Params: mustMarshal(params)}); err != nil {
		c.mu.Lock()
		delete(c.pending, id.Key())
		c.mu.Unlock()
		return Frame{}, err
	}
	select {
	case frame, ok := <-ch:
		if !ok {
			if err := c.Err(); err != nil {
				return Frame{}, err
			}
			return Frame{}, io.EOF
		}
		return frame, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id.Key())
		c.mu.Unlock()
		return Frame{}, ctx.Err()
	}
}

// Respond answers a server request with a result. The error is
// NEVER discardable: an unanswered mandatory request breaks the
// protocol (critique F2), so callers must fail the turn on it.
func (c *Conn) Respond(ctx context.Context, id *RequestID, result any) error {
	return c.enqueue(ctx, &Message{JSONRPC: "2.0", ID: id, Result: mustMarshal(result)})
}

// RespondError answers a server request with a JSON-RPC error —
// the fail-closed reply for capabilities this client never
// advertised.
func (c *Conn) RespondError(ctx context.Context, id *RequestID, code int64, message string) error {
	return c.enqueue(ctx, &Message{JSONRPC: "2.0", ID: id, Error: &WireError{Code: code, Message: message}})
}

// Notify sends a client notification (session/cancel is one).
func (c *Conn) Notify(ctx context.Context, method string, params any) error {
	return c.enqueue(ctx, &Message{JSONRPC: "2.0", Method: method, Params: mustMarshal(params)})
}

func mustMarshal(v any) []byte {
	if v == nil {
		return nil
	}
	b, err := jsonMarshal(v)
	if err != nil {
		// Client-constructed params are compile-time shapes; a
		// marshal failure is a programming error, not a wire event.
		panic(err)
	}
	return b
}
