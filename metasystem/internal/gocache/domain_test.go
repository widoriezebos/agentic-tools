package gocache_test

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// The two machine caches, one per trust domain (disk-lifetimes 3.1, A7): the
// engine's is Go's and staticcheck's default, the delegates' sits beside it
// under metasystem- names, both a pure function of the user cache dir.
func TestDomainPathsAreAPureFunctionOfTheUserCacheDir(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		domain gocache.Domain
		want   gocache.Paths
	}{
		{gocache.DomainEngine, gocache.Paths{GoCache: "/machine/cache/go-build", StaticcheckCache: "/machine/cache/staticcheck"}},
		{gocache.DomainDelegate, gocache.Paths{GoCache: "/machine/cache/metasystem-delegate-go-build", StaticcheckCache: "/machine/cache/metasystem-delegate-staticcheck"}},
	} {
		got, err := gocache.DomainPathsUsing(test.domain, fixedCacheDir("/machine/cache", nil))
		if err != nil || got != test.want {
			t.Errorf("%s: %+v, %v; want %+v", test.domain, got, err, test.want)
		}
		again, _ := gocache.DomainPathsUsing(test.domain, fixedCacheDir("/machine/cache", nil))
		if again != got {
			t.Errorf("%s: a repeat changed the paths: %+v then %+v", test.domain, got, again)
		}
	}
}

func TestDomainPathsRefuseAnUnknownDomainAndAnUnusableCacheDir(t *testing.T) {
	t.Parallel()
	if _, err := gocache.DomainPathsUsing("seat", fixedCacheDir("/machine/cache", nil)); err == nil || err.Error() != `unknown cache domain "seat": want engine or delegate` {
		t.Fatalf("unknown domain: %v", err)
	}
	if _, err := gocache.DomainPathsUsing(gocache.DomainDelegate, fixedCacheDir("", errors.New("no HOME"))); err == nil {
		t.Fatal("a failed user cache dir was accepted")
	}
	if _, err := gocache.DomainPathsUsing(gocache.DomainDelegate, fixedCacheDir("relative", nil)); err == nil {
		t.Fatal("a relative user cache dir was accepted")
	}
	if domain, err := gocache.ParseDomain("delegate"); err != nil || domain != gocache.DomainDelegate {
		t.Fatalf("ParseDomain(delegate) = %q, %v", domain, err)
	}
}

// Every path of both domains, for sandbox grants: the delegate pair is
// granted, the engine pair denied.
func TestDomainDirectoriesListBothCaches(t *testing.T) {
	t.Parallel()
	paths := gocache.Paths{GoCache: "/c/go", StaticcheckCache: "/c/sc"}
	if got := paths.Directories(); len(got) != 2 || got[0] != "/c/go" || got[1] != "/c/sc" {
		t.Fatalf("Directories = %v", got)
	}
}
