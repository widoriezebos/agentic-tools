package testenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// A5's nested-run witness, rerun through the cache context (disk-lifetimes
// A8, DL4A-04): a test-namespace child started with a replaced HOME, no
// GOCACHE, no STATICCHECK_CACHE and no proof locator reports `go env
// GOCACHE`, STATICCHECK_CACHE and an engine resolution equal to this
// process's pair, because this binary issued the context; the child's HOME
// is its own.
func TestANestedChildUnderAReplacedHomeResolvesTheOuterPairThroughTheContext(t *testing.T) {
	t.Parallel()
	const helper = "TESTENV_CACHE_CONTEXT_HELPER"
	if os.Getenv(helper) == "1" {
		output, err := Go("env", "GOCACHE").Output()
		if err != nil {
			t.Fatal(err)
		}
		resolution, err := cachedomain.Resolve(os.Environ(), "")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("child GOCACHE=%s STATICCHECK_CACHE=%s HOME=%s RESOLVED=%s|%s|%s",
			strings.TrimSpace(string(output)), os.Getenv("STATICCHECK_CACHE"), os.Getenv("HOME"),
			resolution.Paths.GoCache, resolution.Paths.StaticcheckCache, resolution.Rule)
		return
	}
	outer := gocache.Paths{GoCache: os.Getenv("GOCACHE"), StaticcheckCache: os.Getenv("STATICCHECK_CACHE")}
	if !filepath.IsAbs(outer.GoCache) || os.Getenv(gocache.ContextEnv) == "" {
		t.Fatalf("this test binary carries no pair or context: %+v", outer)
	}
	replaced := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestANestedChildUnderAReplacedHomeResolvesTheOuterPairThroughTheContext$", "-test.count=1", "-test.v")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case name == "GOCACHE", name == "STATICCHECK_CACHE", name == "HOME", name == "XDG_CACHE_HOME",
			strings.HasPrefix(name, "METASYSTEM_PROOF_"), name == supervisionRegistryHome, name == registryOwnerNonce:
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "HOME="+replaced, helper+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("nested child: %v\n%s", err, output)
	}
	var line string
	for _, candidate := range strings.Split(string(output), "\n") {
		if _, found := strings.CutPrefix(strings.TrimSpace(candidate), "cache_context_test.go"); found || strings.Contains(candidate, "child GOCACHE=") {
			line = candidate
		}
	}
	want := "child GOCACHE=" + outer.GoCache + " STATICCHECK_CACHE=" + outer.StaticcheckCache
	if !strings.Contains(line, want) || !strings.Contains(line, "RESOLVED="+outer.GoCache+"|"+outer.StaticcheckCache+"|context") {
		t.Fatalf("nested child reported %q, want %q resolved through the context\n%s", line, want, output)
	}
	if strings.Contains(line, "HOME="+os.Getenv("HOME")+" ") {
		t.Fatalf("the child's HOME was not its own: %q", line)
	}
}
