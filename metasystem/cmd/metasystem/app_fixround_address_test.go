package main

import (
	"net"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

// A standing address another process holds is refused by name before
// anything is launched.
func TestAppStartRefusesATakenStandingAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address := listener.Addr().String()
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	code, out := bed.run("app", "start")
	if code == 0 || !strings.Contains(out, address) || !strings.Contains(out, "taken") {
		t.Fatalf("a taken standing address is refused by name: %d\n%s", code, out)
	}
	if _, err := applaunch.ReadRecord(bed.installation, applaunch.StandingKey); !os.IsNotExist(err) {
		t.Fatalf("nothing is launched over a taken address: %v", err)
	}
}
