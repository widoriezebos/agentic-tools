package gocache_test

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// The external test package lets this package share testenv.Main, which
// itself resolves the engine cache through this package.
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func fixedCacheDir(dir string, err error) func() (string, error) {
	return func() (string, error) { return dir, err }
}

func TestResolveInheritsAbsoluteElseComputesFromUserCacheDir(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		environment []string
		want        gocache.Paths
	}{
		{"both inherited", []string{"GOCACHE=/outer/go-build", "STATICCHECK_CACHE=/outer/sc"}, gocache.Paths{GoCache: "/outer/go-build", StaticcheckCache: "/outer/sc"}},
		{"unset under a replaced HOME", []string{"HOME=/private/home"}, gocache.Paths{GoCache: "/machine/cache/go-build", StaticcheckCache: "/machine/cache/staticcheck"}},
		{"relative is not inherited", []string{"GOCACHE=rel/go-build", "STATICCHECK_CACHE=./sc"}, gocache.Paths{GoCache: "/machine/cache/go-build", StaticcheckCache: "/machine/cache/staticcheck"}},
		{"empty is not inherited", []string{"GOCACHE=", "STATICCHECK_CACHE="}, gocache.Paths{GoCache: "/machine/cache/go-build", StaticcheckCache: "/machine/cache/staticcheck"}},
		{"one inherited", []string{"GOCACHE=/seat/private"}, gocache.Paths{GoCache: "/seat/private", StaticcheckCache: "/machine/cache/staticcheck"}},
		{"last value wins and is cleaned", []string{"GOCACHE=/first", "GOCACHE=/second/../outer//go-build"}, gocache.Paths{GoCache: "/outer/go-build", StaticcheckCache: "/machine/cache/staticcheck"}},
	} {
		got, err := gocache.ResolveUsing(test.environment, fixedCacheDir("/machine/cache", nil))
		if err != nil || got != test.want {
			t.Errorf("%s: Resolve = %+v, %v; want %+v", test.name, got, err, test.want)
		}
	}
}

func TestResolveNeverAsksUserCacheDirWhenBothInherited(t *testing.T) {
	t.Parallel()
	asked := false
	_, err := gocache.ResolveUsing([]string{"GOCACHE=/a", "STATICCHECK_CACHE=/b"}, func() (string, error) { asked = true; return "/x", nil })
	if err != nil || asked {
		t.Fatalf("err=%v asked=%v; a nested engine with both values must not consult os.UserCacheDir", err, asked)
	}
}

func TestResolveRefusesAnUnusableUserCacheDir(t *testing.T) {
	t.Parallel()
	if paths, err := gocache.ResolveUsing([]string{"GOCACHE=/kept"}, fixedCacheDir("", errors.New("no HOME"))); err == nil || paths.GoCache != "/kept" || paths.StaticcheckCache != "" {
		t.Fatalf("seam error: %+v %v", paths, err)
	}
	if _, err := gocache.ResolveUsing(nil, fixedCacheDir("relative", nil)); err == nil {
		t.Fatal("relative user cache dir accepted")
	}
}

func TestEnvironmentAndCarry(t *testing.T) {
	t.Parallel()
	if got := gocache.Environment(gocache.Paths{GoCache: "/g"}); !slices.Equal(got, []string{"GOCACHE=/g"}) {
		t.Fatalf("Environment = %q", got)
	}
	base := make([]string, 1, 8)
	base[0] = "GOCACHE=/outer/go-build"
	got := gocache.Carry(base)
	if len(got) < 3 || got[len(got)-2] != "GOCACHE=/outer/go-build" || got[len(got)-1][:len("STATICCHECK_CACHE=")] != "STATICCHECK_CACHE=" {
		t.Fatalf("Carry = %q", got)
	}
	if extended := append(base, "X=1"); extended[1] != "X=1" || got[1] != "GOCACHE=/outer/go-build" {
		t.Fatal("Carry aliased its input")
	}
}
