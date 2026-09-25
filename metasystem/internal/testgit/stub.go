package testgit

import (
	"bytes"
	"errors"
	"slices"
	"sync"
)

type Reporter interface {
	Helper()
	Cleanup(func())
	Errorf(string, ...any)
}

type Call struct {
	Dir   string
	Args  []string
	Env   []string
	Stdin []byte
}

type Result struct {
	Stdout []byte
	Stderr []byte
	Err    error
}

type Expectation struct {
	Call   Call
	Result Result
	Check  func(Call) error
}

type Stub struct {
	mu       sync.Mutex
	t        Reporter
	expected []Expectation
	next     int
	calls    []Call
}

var ErrUnexpectedCall = errors.New("unexpected Git call")

func copyCall(call Call) Call {
	call.Args = slices.Clone(call.Args)
	call.Env = slices.Clone(call.Env)
	call.Stdin = bytes.Clone(call.Stdin)
	return call
}

func copyResult(result Result) Result {
	result.Stdout = bytes.Clone(result.Stdout)
	result.Stderr = bytes.Clone(result.Stderr)
	return result
}

func New(t Reporter, expected ...Expectation) *Stub {
	t.Helper()
	s := &Stub{t: t, expected: make([]Expectation, len(expected))}
	for i, expectation := range expected {
		expectation.Call = copyCall(expectation.Call)
		expectation.Result = copyResult(expectation.Result)
		s.expected[i] = expectation
	}
	t.Cleanup(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, expectation := range s.expected[s.next:] {
			t.Errorf("unmet Git call: dir=%q args=%q", expectation.Call.Dir, expectation.Call.Args)
		}
	})
	return s
}

func (s *Stub) Run(call Call) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	call = copyCall(call)
	s.calls = append(s.calls, call)
	if s.next >= len(s.expected) {
		s.t.Errorf("%v: dir=%q args=%q", ErrUnexpectedCall, call.Dir, call.Args)
		return Result{Err: ErrUnexpectedCall}
	}
	want := s.expected[s.next]
	if call.Dir != want.Call.Dir || !slices.Equal(call.Args, want.Call.Args) {
		s.t.Errorf("%v: got dir=%q args=%q; want dir=%q args=%q", ErrUnexpectedCall, call.Dir, call.Args, want.Call.Dir, want.Call.Args)
		return Result{Err: ErrUnexpectedCall}
	}
	if want.Check != nil {
		if err := want.Check(copyCall(call)); err != nil {
			s.t.Errorf("%v: %v", ErrUnexpectedCall, err)
			return Result{Err: ErrUnexpectedCall}
		}
	} else if !slices.Equal(call.Env, want.Call.Env) || !bytes.Equal(call.Stdin, want.Call.Stdin) {
		s.t.Errorf("%v: dir=%q args=%q: got env=%q stdin=%q; want env=%q stdin=%q", ErrUnexpectedCall, call.Dir, call.Args, call.Env, call.Stdin, want.Call.Env, want.Call.Stdin)
		return Result{Err: ErrUnexpectedCall}
	}
	s.next++
	return copyResult(want.Result)
}

func (s *Stub) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := make([]Call, len(s.calls))
	for i, call := range s.calls {
		calls[i] = copyCall(call)
	}
	return calls
}
