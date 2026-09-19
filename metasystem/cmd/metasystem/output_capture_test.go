package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type capturedStream struct {
	data []byte
	err  error
}

// captureStreams acquires every requested stream at once. Holding one stream
// while waiting for another would deadlock with disjoint captures.
type captureStreams struct {
	mu                     sync.Mutex
	cond                   *sync.Cond
	stdoutHeld, stderrHeld bool
	waiters                int
}

func (streams *captureStreams) conditionLocked() *sync.Cond {
	if streams.cond == nil {
		streams.cond = sync.NewCond(&streams.mu)
	}
	return streams.cond
}

func (streams *captureStreams) acquire(stdout, stderr bool) {
	streams.mu.Lock()
	defer streams.mu.Unlock()
	condition := streams.conditionLocked()
	waiting := false
	for stdout && streams.stdoutHeld || stderr && streams.stderrHeld {
		if !waiting {
			streams.waiters++
			waiting = true
			condition.Broadcast()
		}
		condition.Wait()
	}
	if waiting {
		streams.waiters--
		condition.Broadcast()
	}
	if stdout {
		streams.stdoutHeld = true
	}
	if stderr {
		streams.stderrHeld = true
	}
}

func (streams *captureStreams) release(stdout, stderr bool) {
	streams.mu.Lock()
	defer streams.mu.Unlock()
	if stdout {
		streams.stdoutHeld = false
	}
	if stderr {
		streams.stderrHeld = false
	}
	streams.conditionLocked().Broadcast()
}

var commandCaptureStreams captureStreams

// captureCommandOutput drains both capture pipes while the command runs.
// Callers select which standard streams to redirect so one-stream captures
// can be nested without intercepting the stream owned by an outer capture.
func captureCommandOutput(t *testing.T, captureStdout, captureStderr bool, run func() int) (int, string, string) {
	t.Helper()
	commandCaptureStreams.acquire(captureStdout, captureStderr)
	defer commandCaptureStreams.release(captureStdout, captureStderr)
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create the standard output capture pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		_ = outRead.Close()
		_ = outWrite.Close()
		t.Fatalf("create the standard error capture pipe: %v", err)
	}

	drain := func(reader *os.File) <-chan capturedStream {
		result := make(chan capturedStream, 1)
		go func() {
			data, readErr := io.ReadAll(reader)
			result <- capturedStream{data: data, err: readErr}
		}()
		return result
	}
	outResult := drain(outRead)
	errResult := drain(errRead)

	originalOut, originalErr := os.Stdout, os.Stderr
	var code int
	func() {
		if captureStdout {
			os.Stdout = outWrite
		}
		if captureStderr {
			os.Stderr = errWrite
		}
		defer func() {
			if captureStdout {
				os.Stdout = originalOut
			}
			if captureStderr {
				os.Stderr = originalErr
			}
			_ = outWrite.Close()
			_ = errWrite.Close()
		}()
		code = run()
	}()

	stdout := <-outResult
	stderr := <-errResult
	_ = outRead.Close()
	_ = errRead.Close()
	if stdout.err != nil {
		t.Fatalf("read captured standard output: %v", stdout.err)
	}
	if stderr.err != nil {
		t.Fatalf("read captured standard error: %v", stderr.err)
	}
	return code, string(stdout.data), string(stderr.data)
}

func TestCaptureStreamsWaitWithoutHoldingAStream(t *testing.T) {
	t.Parallel()
	var streams captureStreams
	streams.acquire(false, true)
	done := make(chan struct{})
	go func() {
		streams.acquire(true, true)
		streams.release(true, true)
		close(done)
	}()

	streams.mu.Lock()
	condition := streams.conditionLocked()
	for streams.waiters != 1 {
		condition.Wait()
	}
	stdoutHeld := streams.stdoutHeld
	streams.mu.Unlock()
	if stdoutHeld {
		t.Fatal("stdout is held while a capture waits for stderr")
	}

	streams.acquire(true, false)
	streams.release(true, false)
	streams.release(false, true)
	<-done
}

func TestCaptureCommandOutputAllowsNestedAndConcurrentDisjointStreams(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := captureCommandOutput(t, true, false, func() int {
		_, _ = os.Stdout.WriteString("outer-before\n")
		innerCode, innerStdout, innerStderr := captureCommandOutput(t, false, true, func() int {
			_, _ = os.Stderr.WriteString("inner\n")
			return 17
		})
		if innerCode != 17 || innerStdout != "" || innerStderr != "inner\n" {
			t.Errorf("nested capture = code %d, stdout %q, stderr %q", innerCode, innerStdout, innerStderr)
		}
		_, _ = os.Stdout.WriteString("outer-after\n")
		return 23
	})
	if code != 23 || stdout != "outer-before\nouter-after\n" || stderr != "" {
		t.Fatalf("outer capture = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	type captureResult struct {
		code, stream   int
		stdout, stderr string
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan captureResult, 2)
	go func() {
		code, stdout, stderr := captureCommandOutput(t, true, false, func() int {
			ready <- struct{}{}
			<-release
			_, _ = os.Stdout.WriteString("stdout\n")
			return 31
		})
		results <- captureResult{code: code, stream: 1, stdout: stdout, stderr: stderr}
	}()
	go func() {
		code, stdout, stderr := captureCommandOutput(t, false, true, func() int {
			ready <- struct{}{}
			<-release
			_, _ = os.Stderr.WriteString("stderr\n")
			return 37
		})
		results <- captureResult{code: code, stream: 2, stdout: stdout, stderr: stderr}
	}()
	<-ready
	<-ready
	close(release)
	for range 2 {
		result := <-results
		if result.stream == 1 && (result.code != 31 || result.stdout != "stdout\n" || result.stderr != "") {
			t.Errorf("concurrent stdout capture = %#v", result)
		}
		if result.stream == 2 && (result.code != 37 || result.stdout != "" || result.stderr != "stderr\n") {
			t.Errorf("concurrent stderr capture = %#v", result)
		}
	}
}

func TestCaptureCommandOutputDrainsLargeStreams(t *testing.T) {
	stdoutWant := bytes.Repeat([]byte("standard output payload\n"), 50_000)
	stderrWant := bytes.Repeat([]byte("standard error payload\n"), 50_000)
	if len(stdoutWant) <= 1<<20 || len(stderrWant) <= 1<<20 {
		t.Fatal("the capture fixture must exceed one megabyte on each stream")
	}

	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		if _, err := os.Stdout.Write(stdoutWant); err != nil {
			t.Fatalf("write the standard output fixture: %v", err)
		}
		if _, err := os.Stderr.Write(stderrWant); err != nil {
			t.Fatalf("write the standard error fixture: %v", err)
		}
		return 23
	})

	if code != 23 {
		t.Fatalf("captured command exit code = %d, want 23", code)
	}
	if !bytes.Equal([]byte(stdout), stdoutWant) {
		t.Fatalf("captured standard output has %d bytes, want %d", len(stdout), len(stdoutWant))
	}
	if !bytes.Equal([]byte(stderr), stderrWant) {
		t.Fatalf("captured standard error has %d bytes, want %d", len(stderr), len(stderrWant))
	}
}

func TestStandardStreamPipesUseSharedCapture(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("list command package tests: %v", err)
	}
	for _, path := range files {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Name.Name == "captureCommandOutput" {
				continue
			}
			pipeValues := pipeResultNames(function.Body)
			ast.Inspect(function.Body, func(node ast.Node) bool {
				assignment, ok := node.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for index, left := range assignment.Lhs {
					if !isStandardOutputOrError(left) || index >= len(assignment.Rhs) {
						continue
					}
					right, ok := assignment.Rhs[index].(*ast.Ident)
					if !ok || !pipeValues[right.Name] {
						continue
					}
					position := fileSet.Position(left.Pos())
					t.Errorf("%s:%d redirects a standard stream to a pipe outside captureCommandOutput", position.Filename, position.Line)
				}
				return true
			})
		}
	}
}

func pipeResultNames(body *ast.BlockStmt) map[string]bool {
	result := make(map[string]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 || !isOSPipeCall(assignment.Rhs[0]) {
			return true
		}
		for _, left := range assignment.Lhs {
			if identifier, ok := left.(*ast.Ident); ok && identifier.Name != "_" {
				result[identifier.Name] = true
			}
		}
		return true
	})
	return result
}

func isOSPipeCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Pipe" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "os"
}

func isStandardOutputOrError(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Stdout" && selector.Sel.Name != "Stderr") {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "os"
}
