package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Conn is one ACP connection over pre-opened pipes (launch and
// custody stay with scripts — the client owns only the wire). The
// read loop routes each frame by its JSON-RPC shape: responses to
// their pending calls, server requests and notifications to their
// channels. A malformed frame is a protocol death: the loop
// records it, closes the channels, and every pending call fails —
// the failure matrix's "protocol error; record; teardown" row.
type Conn struct {
	reader   *Reader
	writer   *Writer
	nextID   int64
	mu       sync.Mutex
	pending  map[string]chan Frame
	requests chan Frame
	notes    chan Frame
	done     chan struct{}
	err      error
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

// NewConn starts the read loop over the pipe ends. journal receives
// every frame in both directions and is required for production
// turns (settlement evidence).
func NewConn(readEnd io.Reader, writeEnd io.Writer, journal io.Writer) *Conn {
	c := &Conn{
		reader:   NewReader(readEnd, journal),
		writer:   NewWriter(writeEnd, journal),
		pending:  map[string]chan Frame{},
		requests: make(chan Frame, 64),
		notes:    make(chan Frame, 4096),
		done:     make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Conn) readLoop() {
	defer func() {
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
			if errors.Is(err, io.EOF) {
				c.setErr(io.EOF)
			} else {
				c.setErr(err)
			}
			return
		}
		seq++
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
			c.requests <- frame
		case KindNotification:
			c.notes <- frame
		default:
			c.setErr(fmt.Errorf("acp: malformed frame shape (jsonrpc=%q id=%v method=%q)", msg.JSONRPC, msg.ID, msg.Method))
			return
		}
	}
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

// Done closes when the read loop has terminated.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Requests delivers server→client requests in arrival order.
func (c *Conn) Requests() <-chan Frame { return c.requests }

// Notifications delivers notifications in arrival order.
func (c *Conn) Notifications() <-chan Frame { return c.notes }

// JournalErr surfaces the first journal failure on either
// direction — settlement evidence must not silently thin.
func (c *Conn) JournalErr() error {
	if err := c.reader.JournalErr(); err != nil {
		return err
	}
	return c.writer.JournalErr()
}

// Call sends a request and waits for its response, the context
// bounding the wait (the lifecycle loop's deadline while I/O is
// blocked in either direction).
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

	if err := c.send(&Message{JSONRPC: "2.0", ID: id, Method: method, Params: mustMarshal(params)}); err != nil {
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

// Respond answers a server request with a result.
func (c *Conn) Respond(id *RequestID, result any) error {
	return c.send(&Message{JSONRPC: "2.0", ID: id, Result: mustMarshal(result)})
}

// RespondError answers a server request with a JSON-RPC error —
// the fail-closed reply for capabilities this client never
// advertised.
func (c *Conn) RespondError(id *RequestID, code int64, message string) error {
	return c.send(&Message{JSONRPC: "2.0", ID: id, Error: &WireError{Code: code, Message: message}})
}

// Notify sends a client notification (session/cancel is one).
func (c *Conn) Notify(method string, params any) error {
	return c.send(&Message{JSONRPC: "2.0", Method: method, Params: mustMarshal(params)})
}

func (c *Conn) send(m *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.Send(m)
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
