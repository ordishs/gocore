package gocore

import (
	"strings"
	"testing"
)

func TestStatsListenWarningStaysSilentForLoopbackAddresses(t *testing.T) {
	loopback := []string{
		"localhost:9001",
		"LOCALHOST:9001",
		"127.0.0.1:9001",
		"127.0.0.2:9001",
		"[::1]:9001",
	}

	for _, addr := range loopback {
		t.Run(addr, func(t *testing.T) {
			if w := statsListenWarning(addr); w != "" {
				t.Fatalf("got warning %q for loopback address %q, want none", w, addr)
			}
		})
	}
}

func TestStatsListenWarningFiresForAddressesReachableOffHost(t *testing.T) {
	exposed := []string{
		":9001",
		"0.0.0.0:9001",
		"[::]:9001",
		"192.168.1.5:9001",
		"example.com:9001",
		"",
	}

	for _, addr := range exposed {
		t.Run(addr, func(t *testing.T) {
			w := statsListenWarning(addr)
			if w == "" {
				t.Fatalf("got no warning for non-loopback address %q", addr)
			}

			if !strings.Contains(w, "/config") {
				t.Fatalf("warning does not name the configuration exposure: %q", w)
			}
		})
	}
}

// A bare port is the mistake the warning exists to catch. The operator almost
// always means localhost, but the listener binds every interface.
func TestStatsListenWarningNamesTheBarePortMistake(t *testing.T) {
	w := statsListenWarning(":9001")

	if !strings.Contains(w, "every network interface") {
		t.Fatalf("warning does not explain that a bare port binds all interfaces: %q", w)
	}
}

func TestStatsListenWarningHandlesMalformedAddress(t *testing.T) {
	// A malformed address must not be treated as loopback by accident.
	if w := statsListenWarning("not:a:valid:addr"); w == "" {
		t.Fatal("got no warning for a malformed address")
	}
}
