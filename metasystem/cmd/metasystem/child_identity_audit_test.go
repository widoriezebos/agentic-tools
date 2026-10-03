package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// childIdentityReadAllowance names the functions, keyed "file#Function"
// relative to the module root, that may read a raw-started child's command
// line, each with its reason. Every other function starts such a child with
// testexec.StartReady (or testutil.StartHeldProcess).
var childIdentityReadAllowance = map[string]string{}

// TestNoTestReadsAChildsCommandLineBeforeItIsReady: on Linux, exec.Cmd.Start
// returns once the exec has passed its point of no return, but the child's
// /proc/<pid>/cmdline reads empty until the new image has laid out its
// arguments. A read of the child's argv, command or tag right after Start
// sees nothing under load ("announcement identity is not a live, readable
// process", "holder process start time is unreadable", "argv=[] does not
// carry fixture tag"). No Go file in the module starts a child with a plain
// Start and then, in the same function, reads that child's command line:
// lease.Announce*, ProcessIdentity, ProcessCommand, census.AuthIdentity,
// KernelProber.ReadArgv, or a Probe whose Argv, Exe or Environ it uses. Such a
// child starts with testexec.StartReady, which returns after the child's own
// image reported. A start-only read (StartedAt, ReadStart, a Probe's Ref) is
// readable from fork on and is not refused.
func TestNoTestReadsAChildsCommandLineBeforeItIsReady(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	used := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && auditSkipsDirectory(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, read := range earlyChildCommandReads(fileSet, parsed) {
			key := relative + "#" + read.function
			if reason, ok := childIdentityReadAllowance[key]; ok {
				used[key] = reason
				continue
			}
			t.Errorf("%s:%d %s reads the command line of %s, which a plain Start started; start it with testexec.StartReady so its image has published argv",
				relative, read.line, read.reader, read.child)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
	if !reflect.DeepEqual(used, childIdentityReadAllowance) {
		t.Errorf("used allowances = %v, want exactly %v; remove an allowance whose site is gone", used, childIdentityReadAllowance)
	}
}

type earlyChildCommandRead struct {
	function, reader, child string
	line                    int
}

// childCommandReaders are the identity readers that need a process's command
// line, by function or method name.
var childCommandReaders = map[string]bool{
	"Announce": true, "AnnounceWithPair": true, "AnnounceWithProof": true, "AnnounceWithProofAt": true,
	"ProcessIdentity": true, "ProcessCommand": true, "AuthIdentity": true, "ReadArgv": true,
}

// probeCommandFields are the fields of a Probe result that come from the
// command line or the image, not the start record.
var probeCommandFields = map[string]bool{
	"Argv": true, "ArgvKnown": true, "Exe": true, "ExeKnown": true, "Environ": true, "EnvironKnown": true,
}

// readyStarters start a child and return once its own image reported, with
// the index of the command argument.
var readyStarters = map[string]int{"StartReady": 0, "StartHeldProcess": 1}

// earlyChildCommandReads finds, per top-level function, every command-line
// read (see childCommandReaders and probeCommandFields) whose pid argument
// is X.Process.Pid, or a variable assigned from an expression holding it,
// where the function starts X with X.Start() and not with a ready starter.
func earlyChildCommandReads(fileSet *token.FileSet, file *ast.File) []earlyChildCommandRead {
	var reads []earlyChildCommandRead
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.FuncDecl)
		if !ok || decl.Body == nil {
			continue
		}
		function := decl.Name.Name
		if decl.Recv != nil && len(decl.Recv.List) == 1 {
			function = receiverTypeName(decl.Recv.List[0].Type) + "." + function
		}
		plain, ready := map[string]bool{}, map[string]bool{}
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Start" && len(call.Args) == 0 {
				if child, ok := selector.X.(*ast.Ident); ok {
					plain[child.Name] = true
				}
			}
			if index, ok := readyStarters[calledName(call)]; ok && index < len(call.Args) {
				if child, ok := ast.Unparen(call.Args[index]).(*ast.Ident); ok {
					ready[child.Name] = true
				}
			}
			return true
		})
		raw := map[string]bool{}
		for child := range plain {
			if !ready[child] {
				raw[child] = true
			}
		}
		if len(raw) == 0 {
			continue
		}
		// pidOf names the raw child whose pid an expression carries.
		pidVars := map[string]string{}
		pidOf := func(expression ast.Expr) string {
			found := ""
			ast.Inspect(expression, func(node ast.Node) bool {
				switch typed := node.(type) {
				case *ast.SelectorExpr:
					if typed.Sel.Name != "Pid" {
						return true
					}
					process, ok := typed.X.(*ast.SelectorExpr)
					if !ok || process.Sel.Name != "Process" {
						return true
					}
					if child, ok := process.X.(*ast.Ident); ok && raw[child.Name] {
						found = child.Name
					}
				case *ast.Ident:
					if child, ok := pidVars[typed.Name]; ok {
						found = child
					}
				}
				return found == ""
			})
			return found
		}
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != len(assign.Rhs) {
				return true
			}
			for index, left := range assign.Lhs {
				if name, ok := left.(*ast.Ident); ok {
					if child := pidOf(assign.Rhs[index]); child != "" {
						pidVars[name.Name] = child
					}
				}
			}
			return true
		})
		// Probe results whose command fields the function reads.
		commandUsed := map[string]bool{}
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || !probeCommandFields[selector.Sel.Name] {
				return true
			}
			if result, ok := selector.X.(*ast.Ident); ok {
				commandUsed[result.Name] = true
			}
			return true
		})
		record := func(call *ast.CallExpr, reader, child string) {
			reads = append(reads, earlyChildCommandRead{function: function, reader: reader, child: child,
				line: fileSet.Position(call.Pos()).Line})
		}
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				if len(typed.Rhs) != 1 || len(typed.Lhs) == 0 {
					return true
				}
				call, ok := ast.Unparen(typed.Rhs[0]).(*ast.CallExpr)
				if !ok || calledName(call) != "Probe" || len(call.Args) != 1 {
					return true
				}
				result, ok := typed.Lhs[0].(*ast.Ident)
				if !ok || !commandUsed[result.Name] {
					return true
				}
				if child := pidOf(call.Args[0]); child != "" {
					record(call, "Probe (its Argv, Exe or Environ)", child)
				}
			case *ast.CallExpr:
				name := calledName(typed)
				if !childCommandReaders[name] {
					return true
				}
				for _, argument := range typed.Args {
					if child := pidOf(argument); child != "" {
						record(typed, name, child)
						break
					}
				}
			}
			return true
		})
	}
	return reads
}

// calledName is a call's function or method name, "" for any other callee.
func calledName(call *ast.CallExpr) string {
	switch callee := call.Fun.(type) {
	case *ast.Ident:
		return callee.Name
	case *ast.SelectorExpr:
		return callee.Sel.Name
	}
	return ""
}

// TestEarlyChildCommandReadsFindsEveryShape is the audit's own witness: an
// announcement, a ProcessIdentity, an AuthIdentity and a ReadArgv on a
// plainly started child's pid, directly or through a pid variable, and a
// Probe whose Argv is used are found; the same reads of a StartReady or
// StartHeldProcess child, a start-only read, a Probe whose Ref alone is
// used, and a read of the test's own pid are not.
func TestEarlyChildCommandReadsFindsEveryShape(t *testing.T) {
	t.Parallel()
	source := `package p

func announced(root string) {
	holder := exec.Command("bash")
	_ = holder.Start()
	pid := int64(holder.Process.Pid)
	started, _ := lease.StartedAt(pid, nil)
	_, _ = lease.Announce(root, "s", pid, started, "t", "r", "l")
}

func identities() {
	child := exec.Command("sleep")
	child.Start()
	_, _ = lease.ProcessIdentity(int64(child.Process.Pid), nil)
	_, _ = census.AuthIdentity(int64(child.Process.Pid), nil)
	_, _ = identity.KernelProber{}.ReadArgv(int64(child.Process.Pid))
}

func probed(tag string) {
	command := exec.Command("sh")
	command.Start()
	exact, state, err := identity.KernelProber{}.Probe(int64(command.Process.Pid))
	_ = identity.HasExactToken(exact.Argv, tag)
	_, _ = state, err
}

func ready(root string) {
	holder := exec.Command("bash")
	_ = testexec.StartReady(holder)
	pid := int64(holder.Process.Pid)
	_, _ = lease.Announce(root, "s", pid, 1, "t", "r", "l")
	held := exec.Command("sh")
	testutil.StartHeldProcess(t, held)
	exact, _, _ := identity.KernelProber{}.Probe(int64(held.Process.Pid))
	_ = exact.Argv
}

func startOnly() {
	command := exec.Command("sh")
	command.Start()
	_, _ = lease.StartedAt(int64(command.Process.Pid), nil)
	exact, _, _ := identity.KernelProber{}.Probe(int64(command.Process.Pid))
	_ = exact.Ref()
	_, _ = lease.ProcessIdentity(int64(os.Getpid()), nil)
}
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "fixture_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := earlyChildCommandReads(fileSet, parsed)
	want := []earlyChildCommandRead{
		{function: "announced", reader: "Announce", child: "holder", line: 8},
		{function: "identities", reader: "ProcessIdentity", child: "child", line: 14},
		{function: "identities", reader: "AuthIdentity", child: "child", line: 15},
		{function: "identities", reader: "ReadArgv", child: "child", line: 16},
		{function: "probed", reader: "Probe (its Argv, Exe or Environ)", child: "command", line: 22},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reads = %+v, want %+v", got, want)
	}
}
