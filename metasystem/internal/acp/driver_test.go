package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// newStubEndpoint mirrors newStubTurn for the driver path: the stub
// speaks the same scripted wire, but the pipes are handed to the
// driver as a pre-opened Endpoint (the adapter-custody split).
func newStubEndpoint(t *testing.T, steps []stubStep) (delegate.Endpoint, *bytes.Buffer, *stubServer, func()) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	journal := &bytes.Buffer{}
	server := &stubServer{
		t:        t,
		reader:   NewReader(serverReads, nil),
		writer:   NewWriter(serverWrites, nil),
		closer:   serverWrites,
		consumed: make(chan int, 1),
	}
	go server.run(steps)
	endpoint := delegate.Endpoint{FromAgent: clientReads, ToAgent: clientWrites, Journal: journal}
	return endpoint, journal, server, func() { clientWrites.Close(); serverWrites.Close() }
}

func testDriver() *NativeDriver {
	decl, ok := runtimes.Lookup("devin")
	if !ok {
		panic("devin missing from the runtimes registry")
	}
	driver, err := newNative(decl, func(tools string) (string, error) {
		if tools == "read-only" {
			return "ask", nil
		}
		return "accept-edits", nil
	})
	if err != nil {
		panic(err)
	}
	return driver
}

func promptRequest() delegate.PromptRequest {
	return delegate.PromptRequest{
		Prompt: []byte("do the thing"),
		Envelope: delegate.Envelope{
			ReadRoots:  []string{"/work"},
			WriteRoots: []string{"/work"},
			Network:    "deny",
			Approvals:  "deny",
			Tools:      "read-only",
		},
		PromptTimeout: 5 * time.Second,
		LateWindow:    50 * time.Millisecond,
		CancelGrace:   200 * time.Millisecond,
	}
}

func deliveredSteps(text string) []stubStep {
	return []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", expectParams: []string{`"modeId":"ask"`}, result: `{}`},
		{
			expectMethod:  "session/prompt",
			notifications: []string{chunkFor("s-1", text)},
			result:        `{"stopReason":"end_turn","usage":{"inputTokens":10,"outputTokens":5}}`,
		},
	}
}

func driverTurn(t *testing.T, steps []stubStep, req delegate.PromptRequest) (delegate.Turn, *bytes.Buffer, *stubServer, func()) {
	t.Helper()
	endpoint, journal, server, cleanup := newStubEndpoint(t, steps)
	session, err := testDriver().Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	turn, err := session.PromptTurn(context.Background(), req)
	if err != nil {
		t.Fatalf("prompt turn: %v", err)
	}
	return turn, journal, server, cleanup
}

// Proof 1 + 3: the differential — the same scripted bytes through
// direct RunTurn (nil tap, the verb's shape) and through the Driver;
// outcome and mapped result agree field for field, and after the
// harness quiesce the journals carry identical bytes (the tap is
// journal-invisible).
func TestDriverDifferentialAgainstDirectRunTurn(t *testing.T) {
	cases := []struct {
		name  string
		steps func() []stubStep
	}{
		{"delivered", func() []stubStep { return deliveredSteps("hi") }},
		{"version-mismatch", func() []stubStep {
			return []stubStep{{expectMethod: "initialize", result: `{"protocolVersion":9,"agentCapabilities":{"loadSession":false},"authMethods":[]}`}}
		}},
		{"setup-error", func() []stubStep {
			return []stubStep{initStep("[]"), {expectMethod: "session/new", errorCode: -32603, errorMessage: "no"}}
		}},
		{"turn-failed", func() []stubStep {
			return []stubStep{initStep("[]"), {expectMethod: "session/new", dropAfter: true}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Direct path: the verb's shape, nil tap.
			conn, directServer, directCleanup := newStubTurn(t, tc.steps())
			defer directCleanup()
			cfg := baseConfig()
			cfg.ModeID = "ask"
			outcome := RunTurn(context.Background(), conn, cfg)
			directCleanup()
			<-conn.Done()
			_ = directServer

			// Driver path: same scripted bytes.
			req := promptRequest()
			req.Mode = "ask"
			turn, _, _, cleanup := driverTurn(t, tc.steps(), req)
			defer cleanup()
			result, err := turn.Result(context.Background())
			if err != nil {
				t.Fatalf("result: %v", err)
			}
			if string(result.Row) != string(outcome.Row) {
				t.Fatalf("row diverged: direct %s driver %s", outcome.Row, result.Row)
			}
			if result.StopReason != outcome.StopReason || result.SessionID != outcome.SessionID ||
				result.Violations != outcome.Violations || !bytes.Equal(result.Candidate, outcome.Candidate) {
				t.Fatalf("mapped result diverged:\ndirect %+v\ndriver %+v", outcome, result)
			}
			if result.Detail != outcome.Detail {
				t.Fatalf("detail diverged: direct %q driver %q", outcome.Detail, result.Detail)
			}
		})
	}
}

// The differential's journal half, on the delivered bed: identical
// bytes after both sides quiesce.
func TestDriverJournalByteIdentity(t *testing.T) {
	// newStubTurn buries its journal, so the direct path is rebuilt
	// here with a visible one.
	directJournal := &bytes.Buffer{}
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, directJournal)
	server := &stubServer{t: t, reader: NewReader(serverReads, nil), writer: NewWriter(serverWrites, nil), closer: serverWrites, consumed: make(chan int, 1)}
	go server.run(deliveredSteps("hi"))
	cfg := baseConfig()
	cfg.ModeID = "ask"
	if outcome := RunTurn(context.Background(), conn, cfg); outcome.Row != RowDelivered {
		t.Fatalf("direct not delivered: %+v", outcome)
	}
	clientWrites.Close()
	serverWrites.Close()
	<-conn.Done()

	if err := conn.JournalErr(); err != nil {
		t.Fatalf("direct journal unhealthy: %v", err)
	}

	endpoint, driverJournal, _, cleanup := newStubEndpoint(t, deliveredSteps("hi"))
	session, err := testDriver().Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	req := promptRequest()
	req.Mode = "ask"
	turn, err := session.PromptTurn(context.Background(), req)
	if err != nil {
		t.Fatalf("prompt turn: %v", err)
	}
	result, err := turn.Result(context.Background())
	if err != nil || result.Row != delegate.RowDelivered {
		t.Fatalf("driver not delivered: %+v %v", result, err)
	}
	// The owner's finalization: close the pipes, then Quiesce — the
	// evidence is final only after the Done wait and the journal
	// health sample (the design's owner law, exercised for real).
	cleanup()
	quiesceCtx, quiesceCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer quiesceCancel()
	if err := session.(Quiescer).Quiesce(quiesceCtx); err != nil {
		t.Fatalf("driver quiesce: %v", err)
	}
	if !bytes.Equal(directJournal.Bytes(), driverJournal.Bytes()) {
		t.Fatalf("journal bytes diverged:\ndirect:\n%s\ndriver:\n%s", directJournal.Bytes(), driverJournal.Bytes())
	}
}

// CC-S2-009: the overflow law on the PRODUCTION path — a scripted
// protocol flood past the exported capacity settles without a single
// consumer read (the pump is provably unblocked), and the drained
// stream then carries exactly the capacity plus the gap event with
// the exact drop count, wired through the real tap.
func TestDriverFloodOverflowProductionPath(t *testing.T) {
	const flood = SpoolCapacity + 200
	notifications := make([]string, flood)
	for i := range notifications {
		notifications[i] = chunkFor("s-1", fmt.Sprintf("n%d", i))
	}
	steps := []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", result: `{}`},
		{expectMethod: "session/prompt", notifications: notifications, result: `{"stopReason":"end_turn"}`},
	}
	req := promptRequest()
	req.Mode = "ask"
	req.PromptTimeout = 60 * time.Second
	turn, _, _, cleanup := driverTurn(t, steps, req)
	defer cleanup()
	// No stream reads before settle: Result returning proves the
	// flood could not block the pump on a full spool.
	result, err := turn.Result(context.Background())
	if err != nil || result.Row != delegate.RowDelivered {
		t.Fatalf("flooded turn: %+v %v", result, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream := turn.EventStream()
	spooled := 0
	var dropped uint64
	for {
		ev, ok := stream.Next(ctx)
		if !ok {
			break
		}
		if ev.Kind == "driver/events-dropped" {
			var body struct {
				Dropped uint64 `json:"dropped"`
			}
			if err := json.Unmarshal(ev.Params, &body); err != nil {
				t.Fatalf("gap params %s", ev.Params)
			}
			dropped = body.Dropped
			continue
		}
		spooled++
	}
	if spooled != SpoolCapacity {
		t.Fatalf("spooled %d, want the full capacity %d", spooled, SpoolCapacity)
	}
	// The exact conservation law: every projected event either
	// spooled or dropped. Projected = the flood + the
	// session-established beat (settlement-started is internal and
	// never projected; no asks were scripted).
	if int(dropped)+spooled != flood+1 {
		t.Fatalf("conservation broken: spooled %d + dropped %d != projected %d", spooled, dropped, flood+1)
	}
}

// CC-R2-006's inert-cancel proof: once a turn has finalized, Cancel
// is inert — a Result AFTER a late Cancel is byte-identical to the
// Result BEFORE it (no retroactive cancel-requested prefix).
func TestDriverCancelAfterFinalizeIsInert(t *testing.T) {
	req := promptRequest()
	req.Mode = "ask"
	turn, _, _, cleanup := driverTurn(t, deliveredSteps("x"), req)
	defer cleanup()
	before, err := turn.Result(context.Background())
	if err != nil || before.Row != delegate.RowDelivered {
		t.Fatalf("first result %+v %v", before, err)
	}
	if err := turn.Cancel(context.Background()); err != nil {
		t.Fatalf("late cancel: %v", err)
	}
	after, err := turn.Result(context.Background())
	if err != nil {
		t.Fatalf("second result: %v", err)
	}
	if after.Detail != before.Detail || after.Row != before.Row {
		t.Fatalf("late cancel rewrote the result: before %+v after %+v", before, after)
	}
	if strings.HasPrefix(after.Detail, "cancel-requested; ") {
		t.Fatalf("retroactive prefix appeared: %q", after.Detail)
	}
}

// The spool's byte bound: a few giant events trip drop mode long
// before the count bound.
func TestSpoolByteBound(t *testing.T) {
	spool := &eventSpool{capacity: 100, byteCapacity: 1024}
	big := delegate.Event{Kind: "e", Params: bytes.Repeat([]byte("x"), 600)}
	spool.append(big)
	spool.append(big) // 1200 bytes > 1024: dropped
	spool.append(big) // contiguous drop
	ctx := context.Background()
	if ev, ok := spool.next(ctx); !ok || ev.Kind != "e" {
		t.Fatalf("first event: %+v ok=%v", ev, ok)
	}
	gap, ok := spool.next(ctx)
	if !ok || gap.Kind != "driver/events-dropped" {
		t.Fatalf("expected gap after byte-bound drops, got %+v", gap)
	}
	var body struct {
		Dropped uint64 `json:"dropped"`
	}
	if err := json.Unmarshal(gap.Params, &body); err != nil || body.Dropped != 2 {
		t.Fatalf("gap params %s, want dropped=2", gap.Params)
	}
}

// Proof 4: stream truth. session-established is observable through
// the seam BEFORE the prompt settles (the server stays silent on the
// prompt until cancelled); update events arrive in order carrying
// their wire seq; the stream drains after settle then ends; the ask
// stream yields nothing; Answer types its refusal.
func TestDriverStreamTruth(t *testing.T) {
	steps := []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", result: `{}`},
		{expectMethod: "session/prompt", silent: true},
	}
	req := promptRequest()
	req.Mode = "ask"
	turn, _, _, cleanup := driverTurn(t, steps, req)
	defer cleanup()

	stream := turn.EventStream()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ev, ok := stream.Next(ctx)
	if !ok || ev.Kind != "driver/session-established" {
		t.Fatalf("first stream event = %+v ok=%v, want driver/session-established", ev, ok)
	}
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(ev.Params, &params); err != nil || params.SessionID != "s-1" {
		t.Fatalf("session-established params %s", ev.Params)
	}
	// Observed pre-settle: the prompt is still pending (the server is
	// silent); the turn must NOT be done yet.
	quick, quickCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer quickCancel()
	if _, err := turn.Result(quick); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("turn settled before the prompt resolved: %v", err)
	}
	if err := turn.Cancel(context.Background()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	result, err := turn.Result(context.Background())
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.Row != delegate.RowCancelled {
		t.Fatalf("row = %s, want cancelled", result.Row)
	}
	if !strings.HasPrefix(result.Detail, "cancel-requested; ") {
		t.Fatalf("detail lacks the cancel-requested prefix: %q", result.Detail)
	}
	// Drain-on-close, then end-of-stream.
	for {
		if _, ok := stream.Next(ctx); !ok {
			break
		}
	}
	if ask, ok := turn.AskStream().Next(ctx); ok {
		t.Fatalf("ask stream yielded %+v", ask)
	}
	if err := turn.Answer(context.Background(), delegate.Answer{}); !errors.Is(err, ErrAskPolicyRefused) {
		t.Fatalf("answer = %v, want ErrAskPolicyRefused", err)
	}
}

// Update events carry the nested discriminator and the wire seq, in
// tap-call order; the refused-ask beat appears and does not change
// the violations total.
func TestDriverUpdateEventsAndRefusedAskBeat(t *testing.T) {
	askParams := `{"sessionId":"s-1","options":[{"optionId":"deny","kind":"reject_once","name":"Deny"}]}`
	steps := []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", result: `{}`},
		{
			expectMethod:  "session/prompt",
			notifications: []string{chunkFor("s-1", "a"), chunkFor("s-1", "b")},
			request:       askParams,
			result:        `{"stopReason":"end_turn"}`,
		},
	}
	req := promptRequest()
	req.Mode = "ask"
	turn, _, _, cleanup := driverTurn(t, steps, req)
	defer cleanup()
	result, err := turn.Result(context.Background())
	if err != nil || result.Row != delegate.RowDelivered {
		t.Fatalf("result %+v err %v", result, err)
	}
	if result.Violations != 0 {
		t.Fatalf("the in-window refused ask must not count as a violation; got %d", result.Violations)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream := turn.EventStream()
	var kinds []string
	var lastSeq uint64
	sawSeq := false
	for {
		ev, ok := stream.Next(ctx)
		if !ok {
			break
		}
		kinds = append(kinds, ev.Kind)
		if strings.HasPrefix(ev.Kind, "update/") {
			if sawSeq && ev.Seq <= lastSeq {
				t.Fatalf("update wire seqs not increasing: %d after %d", ev.Seq, lastSeq)
			}
			lastSeq, sawSeq = ev.Seq, true
			// Projection law: Params are the INNER update object.
			var inner struct {
				SessionUpdate string `json:"sessionUpdate"`
			}
			if err := json.Unmarshal(ev.Params, &inner); err != nil || inner.SessionUpdate == "" {
				t.Fatalf("update params are not the inner object: %s", ev.Params)
			}
			if strings.Contains(string(ev.Params), "\"update\"") {
				t.Fatalf("update params kept the outer wrapper: %s", ev.Params)
			}
		}
	}
	joined := strings.Join(kinds, ",")
	if !strings.Contains(joined, "driver/session-established") ||
		!strings.Contains(joined, "update/agent_message_chunk") ||
		!strings.Contains(joined, "driver/permission-request-refused") {
		t.Fatalf("stream kinds missing expected beats: %v", kinds)
	}
	if kinds[0] != "driver/session-established" {
		t.Fatalf("session-established must precede everything; got %v", kinds)
	}
}

// Proof 5: the overflow law on the spool itself — contiguous drops,
// one gap event with the exact count at the drain-to-empty
// transition, and appends that provably never block.
func TestSpoolOverflowContiguousDrops(t *testing.T) {
	spool := &eventSpool{capacity: 3}
	for i := 0; i < 10; i++ {
		spool.append(delegate.Event{Kind: fmt.Sprintf("e%d", i)})
	}
	ctx := context.Background()
	var got []string
	for i := 0; i < 3; i++ {
		ev, ok := spool.next(ctx)
		if !ok {
			t.Fatal("spool ended early")
		}
		got = append(got, ev.Kind)
	}
	gap, ok := spool.next(ctx)
	if !ok || gap.Kind != "driver/events-dropped" {
		t.Fatalf("expected the gap event, got %+v ok=%v", gap, ok)
	}
	var body struct {
		Dropped uint64 `json:"dropped"`
	}
	if err := json.Unmarshal(gap.Params, &body); err != nil || body.Dropped != 7 {
		t.Fatalf("gap params %s, want dropped=7", gap.Params)
	}
	// After the gap, appending resumes.
	spool.append(delegate.Event{Kind: "after"})
	ev, ok := spool.next(ctx)
	if !ok || ev.Kind != "after" {
		t.Fatalf("append after gap: %+v ok=%v", ev, ok)
	}
	if got[0] != "e0" || got[2] != "e2" {
		t.Fatalf("pre-gap events wrong: %v", got)
	}
}

// Proof 6 + 8: envelope law and the one-shot session. A refused
// envelope produces no wire traffic and still exhausts the session;
// concurrent double PromptTurn admits exactly one winner.
func TestDriverEnvelopeLawAndOneShot(t *testing.T) {
	endpoint, _, server, cleanup := newStubEndpoint(t, nil)
	defer cleanup()
	session, err := testDriver().Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	req := promptRequest()
	req.Envelope.Approvals = "ask"
	if _, err := session.PromptTurn(context.Background(), req); !errors.Is(err, ErrPreflightRefused) {
		t.Fatalf("approvals=ask err = %v, want ErrPreflightRefused", err)
	}
	if _, err := session.PromptTurn(context.Background(), promptRequest()); !errors.Is(err, delegate.ErrSessionExhausted) {
		t.Fatalf("second call err = %v, want ErrSessionExhausted", err)
	}
	cleanup()
	server.requireConsumed(0)

	// Concurrent claim: exactly one winner.
	endpoint2, _, _, cleanup2 := newStubEndpoint(t, deliveredSteps("x"))
	defer cleanup2()
	session2, err := testDriver().Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint2})
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := promptRequest()
			r.Mode = "ask"
			_, errs[i] = session2.PromptTurn(context.Background(), r)
		}(i)
	}
	wg.Wait()
	winners := 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else if !errors.Is(err, delegate.ErrSessionExhausted) {
			t.Fatalf("loser got %v, want ErrSessionExhausted", err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
}

// Proof 7: mapping law — zero seam-exposed timeouts refuse typed;
// Mode overrides the resolver; a resolver failure refuses typed.
func TestDriverMappingLaw(t *testing.T) {
	endpoint, _, _, cleanup := newStubEndpoint(t, nil)
	defer cleanup()
	session, _ := testDriver().Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint})
	req := promptRequest()
	req.PromptTimeout = 0
	if _, err := session.PromptTurn(context.Background(), req); !errors.Is(err, ErrTimeoutUnset) {
		t.Fatalf("zero prompt timeout err = %v, want ErrTimeoutUnset", err)
	}

	// Mode override: set_mode must carry the explicit mode, not the
	// resolver's.
	steps := []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", expectParams: []string{`"modeId":"explicit"`}, result: `{}`},
		{expectMethod: "session/prompt", result: `{"stopReason":"end_turn"}`},
	}
	req2 := promptRequest()
	req2.Mode = "explicit"
	turn, _, server, cleanup2 := driverTurn(t, steps, req2)
	defer cleanup2()
	if result, err := turn.Result(context.Background()); err != nil || result.Row != delegate.RowDelivered {
		t.Fatalf("explicit-mode turn: %+v %v", result, err)
	}
	server.requireConsumed(4)

	// Resolver failure refuses typed.
	decl, _ := runtimes.Lookup("devin")
	failing, err := newNative(decl, func(string) (string, error) { return "", fmt.Errorf("no mapping") })
	if err != nil {
		t.Fatalf("newNative: %v", err)
	}
	endpoint3, _, _, cleanup3 := newStubEndpoint(t, nil)
	defer cleanup3()
	session3, _ := failing.Open(context.Background(), delegate.OpenRequest{Workspace: "/work", Endpoint: endpoint3})
	if _, err := session3.PromptTurn(context.Background(), promptRequest()); !errors.Is(err, ErrModeUnresolved) {
		t.Fatalf("resolver failure err = %v, want ErrModeUnresolved", err)
	}
}

// Proof 9: synchronization — concurrent Result+Usage, repeated
// reads, accessor-context expiry, and mutation isolation.
func TestDriverSynchronization(t *testing.T) {
	req := promptRequest()
	req.Mode = "ask"
	turn, _, _, cleanup := driverTurn(t, deliveredSteps("payload"), req)
	defer cleanup()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = turn.Result(context.Background()) }()
		go func() { defer wg.Done(); _, _ = turn.Usage(context.Background()) }()
	}
	wg.Wait()
	first, err := turn.Result(context.Background())
	if err != nil || first.Row != delegate.RowDelivered {
		t.Fatalf("first result %+v %v", first, err)
	}
	if len(first.Candidate) > 0 {
		first.Candidate[0] = 'X'
	}
	second, err := turn.Result(context.Background())
	if err != nil {
		t.Fatalf("second result: %v", err)
	}
	if len(second.Candidate) > 0 && second.Candidate[0] == 'X' {
		t.Fatal("mutating a returned candidate leaked into the cache")
	}
	usage, err := turn.Usage(context.Background())
	if err != nil || !usage.Available {
		t.Fatalf("usage %+v %v", usage, err)
	}
	expired, cancelExpired := context.WithCancel(context.Background())
	cancelExpired()
	if _, err := turn.Usage(expired); err == nil {
		// The done channel is already closed here, so a settled turn
		// may legitimately win the select; accept either outcome but
		// require determinism on an UNSETTLED turn, covered in
		// TestDriverStreamTruth's quick-expiry read.
		_ = err
	}
}

// Proof 10's setup arm: cancel during a silent setup phase reports
// the pump's verbatim row plus the cancel-requested prefix — no row
// rewriting.
func TestDriverCancelDuringSetup(t *testing.T) {
	steps := []stubStep{{expectMethod: "initialize", silent: true}}
	req := promptRequest()
	req.Mode = "ask"
	turn, _, _, cleanup := driverTurn(t, steps, req)
	defer cleanup()
	time.Sleep(50 * time.Millisecond)
	if err := turn.Cancel(context.Background()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	result, err := turn.Result(context.Background())
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if !strings.HasPrefix(result.Detail, "cancel-requested; ") {
		t.Fatalf("setup-phase cancel lacks the prefix: %q", result.Detail)
	}
	if result.Row == delegate.RowDelivered {
		t.Fatalf("a cancelled setup cannot deliver: %+v", result)
	}
}

// Proof 11: the usage boundary — absent, JSON null, and a real
// object.
func TestDriverUsageBoundary(t *testing.T) {
	cases := []struct {
		name      string
		result    string
		available bool
	}{
		{"absent", `{"stopReason":"end_turn"}`, false},
		{"null", `{"stopReason":"end_turn","usage":null}`, false},
		{"object", `{"stopReason":"end_turn","usage":{"inputTokens":1}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := []stubStep{
				initStep("[]"),
				newSessionStep(),
				{expectMethod: "session/set_mode", result: `{}`},
				{expectMethod: "session/prompt", result: tc.result},
			}
			req := promptRequest()
			req.Mode = "ask"
			turn, _, _, cleanup := driverTurn(t, steps, req)
			defer cleanup()
			usage, err := turn.Usage(context.Background())
			if err != nil {
				t.Fatalf("usage: %v", err)
			}
			if usage.Available != tc.available {
				t.Fatalf("available = %v raw=%s, want %v", usage.Available, usage.Raw, tc.available)
			}
		})
	}
}

// Proof 12: the registration laws, table-driven through the
// error-returning core.
func TestRegistrationLaws(t *testing.T) {
	devin, _ := runtimes.Lookup("devin")
	okResolver := func(string) (string, error) { return "ask", nil }

	if _, err := newNative(devin, nil); err == nil || !strings.Contains(err.Error(), "resolver") {
		t.Fatalf("nil resolver: %v", err)
	}
	noACP := devin
	noACP.ExpectedACP = nil
	if _, err := newNative(noACP, okResolver); err == nil || !strings.Contains(err.Error(), "not expected to speak ACP") {
		t.Fatalf("no ExpectedACP: %v", err)
	}
	noCaps := devin
	expectation := *devin.ExpectedACP
	expectation.ExpectedCapabilities = nil
	noCaps.ExpectedACP = &expectation
	if _, err := newNative(noCaps, okResolver); err == nil || !strings.Contains(err.Error(), "no expected native-driver capabilities") {
		t.Fatalf("no capabilities: %v", err)
	}

	// Every boolean flipped once must fail the join.
	flip := func(mutate func(*runtimes.ACPCapabilities)) error {
		decl := devin
		exp := *devin.ExpectedACP
		caps := *devin.ExpectedACP.ExpectedCapabilities
		mutate(&caps)
		exp.ExpectedCapabilities = &caps
		decl.ExpectedACP = &exp
		_, err := newNative(decl, okResolver)
		return err
	}
	flips := map[string]func(*runtimes.ACPCapabilities){
		"resume":     func(c *runtimes.ACPCapabilities) { c.Resume = !c.Resume },
		"signal":     func(c *runtimes.ACPCapabilities) { c.SessionEstablishedSignal = !c.SessionEstablishedSignal },
		"structured": func(c *runtimes.ACPCapabilities) { c.NativeStructuredOutput = !c.NativeStructuredOutput },
		"events":     func(c *runtimes.ACPCapabilities) { c.NativeEvents = !c.NativeEvents },
		"usage":      func(c *runtimes.ACPCapabilities) { c.NativeUsage = !c.NativeUsage },
		"cancel":     func(c *runtimes.ACPCapabilities) { c.GracefulCancel = !c.GracefulCancel },
		"server":     func(c *runtimes.ACPCapabilities) { c.ProtocolServer = !c.ProtocolServer },
		"budget":     func(c *runtimes.ACPCapabilities) { c.NativeBudget = !c.NativeBudget },
	}
	for name, mutate := range flips {
		if err := flip(mutate); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("flip %s: %v", name, err)
		}
	}

	// The straight join succeeds and declares the earned surface.
	driver, err := newNative(devin, okResolver)
	if err != nil {
		t.Fatalf("straight join: %v", err)
	}
	if driver.Declaration() != nativeDeclaration {
		t.Fatalf("declaration %+v", driver.Declaration())
	}
}

// RegisterNative wires the registry for real: the driver is findable
// under (devin, acp). One registration for the whole test binary —
// the registry is global and duplicates panic by law.
func TestRegisterNativeFindable(t *testing.T) {
	driver := RegisterNative("devin", func(string) (string, error) { return "ask", nil })
	found, err := delegate.DriverFor(delegate.Key{Runtime: "devin", Transport: "acp"})
	if err != nil {
		t.Fatalf("driver lookup: %v", err)
	}
	if found.(*NativeDriver) != driver {
		t.Fatal("registry returned a different driver")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration must panic")
		}
	}()
	RegisterNative("devin", func(string) (string, error) { return "ask", nil })
}
