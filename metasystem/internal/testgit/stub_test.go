package testgit

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

type reporter struct {
	mu      sync.Mutex
	cleanup func()
	errors  []string
}

func (*reporter) Helper()            {}
func (r *reporter) Cleanup(f func()) { r.cleanup = f }
func (r *reporter) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
func (r *reporter) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.errors)
}

func TestDeclaredResultAndCopies(t *testing.T) {
	r := &reporter{}
	errDeclared := errors.New("declared failure")
	args, env, stdin := []string{"show"}, []string{"A=B"}, []byte("input")
	stdout, stderr := []byte("out"), []byte("err")
	s := New(r, Expectation{Call: Call{Dir: "repo", Args: args, Env: env, Stdin: stdin}, Result: Result{Stdout: stdout, Stderr: stderr, Err: errDeclared}})
	args[0], env[0], stdin[0], stdout[0], stderr[0] = "changed", "C=D", 'X', 'X', 'X'
	call := Call{Dir: "repo", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("input")}
	got := s.Run(call)
	if got.Err != errDeclared || string(got.Stdout) != "out" || string(got.Stderr) != "err" {
		t.Fatalf("result=%+v", got)
	}
	call.Args[0], call.Env[0], call.Stdin[0] = "changed", "C=D", 'X'
	got.Stdout[0], got.Stderr[0] = 'X', 'X'
	log := s.Calls()
	if log[0].Args[0] != "show" || log[0].Env[0] != "A=B" || string(log[0].Stdin) != "input" {
		t.Fatalf("recorded call changed: %+v", log)
	}
	log[0].Args[0], log[0].Env[0], log[0].Stdin[0] = "changed", "C=D", 'X'
	if next := s.Calls()[0]; next.Args[0] != "show" || next.Env[0] != "A=B" || string(next.Stdin) != "input" {
		t.Fatalf("call log escaped: %+v", next)
	}
	r.cleanup()
	if len(r.messages()) != 0 {
		t.Fatalf("reports=%v", r.messages())
	}
}

func TestMismatchesRefuseWithoutConsuming(t *testing.T) {
	for _, row := range []struct {
		name string
		call Call
	}{
		{"directory", Call{Dir: "else", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("x")}},
		{"argv", Call{Dir: "repo", Args: []string{"status"}, Env: []string{"A=B"}, Stdin: []byte("x")}},
		{"environment", Call{Dir: "repo", Args: []string{"show"}, Env: []string{"A=C"}, Stdin: []byte("x")}},
		{"stdin", Call{Dir: "repo", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("y")}},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := &reporter{}
			s := New(r, Expectation{Call: Call{Dir: "repo", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("x")}})
			if result := s.Run(row.call); !errors.Is(result.Err, ErrUnexpectedCall) {
				t.Fatalf("result=%+v", result)
			}
			r.cleanup()
			if len(r.messages()) != 2 || !strings.Contains(r.messages()[0], "unexpected") || !strings.Contains(r.messages()[1], "unmet") {
				t.Fatalf("reports=%v", r.messages())
			}
		})
	}
}

func TestUnexpectedCallAndValidator(t *testing.T) {
	r := &reporter{}
	s := New(r, Expectation{Call: Call{Dir: "repo", Args: []string{"show"}}, Check: func(call Call) error {
		if !slices.Equal(call.Env, []string{"A=B"}) || string(call.Stdin) != "x" {
			return errors.New("generated inputs invalid")
		}
		return nil
	}})
	if result := s.Run(Call{Dir: "repo", Args: []string{"show"}}); !errors.Is(result.Err, ErrUnexpectedCall) {
		t.Fatalf("validator accepted invalid input: %+v", result)
	}
	if result := s.Run(Call{Dir: "else", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("x")}); !errors.Is(result.Err, ErrUnexpectedCall) {
		t.Fatalf("validator bypassed directory: %+v", result)
	}
	if result := s.Run(Call{Dir: "repo", Args: []string{"other"}, Env: []string{"A=B"}, Stdin: []byte("x")}); !errors.Is(result.Err, ErrUnexpectedCall) {
		t.Fatalf("validator bypassed argv: %+v", result)
	}
	if result := s.Run(Call{Dir: "repo", Args: []string{"show"}, Env: []string{"A=B"}, Stdin: []byte("x")}); result.Err != nil {
		t.Fatalf("valid input refused: %+v", result)
	}
	if result := s.Run(Call{Dir: "repo", Args: []string{"show"}}); !errors.Is(result.Err, ErrUnexpectedCall) {
		t.Fatalf("extra call accepted: %+v", result)
	}
	r.cleanup()
	if len(r.messages()) != 4 {
		t.Fatalf("reports=%v", r.messages())
	}
}

func TestNilAndEmptyInputsMatch(t *testing.T) {
	r := &reporter{}
	s := New(r, Expectation{Call: Call{Dir: "repo", Args: []string{"show"}}})
	if result := s.Run(Call{Dir: "repo", Args: []string{"show"}, Env: []string{}, Stdin: []byte{}}); result.Err != nil {
		t.Fatalf("result=%+v", result)
	}
	r.cleanup()
}

func TestConcurrentCalls(t *testing.T) {
	const count = 128
	r := &reporter{}
	expected := make([]Expectation, count)
	for i := range expected {
		expected[i].Call = Call{Dir: "repo", Args: []string{"show"}}
	}
	s := New(r, expected...)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if result := s.Run(Call{Dir: "repo", Args: []string{"show"}}); result.Err != nil {
				t.Errorf("result=%+v", result)
			}
			_ = s.Calls()
		}()
	}
	wg.Wait()
	r.cleanup()
	if len(s.Calls()) != count || len(r.messages()) != 0 {
		t.Fatalf("calls=%d reports=%v", len(s.Calls()), r.messages())
	}
}
