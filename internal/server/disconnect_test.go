package server

import (
	"strings"
	"testing"
)

func TestCleanDisconnectWhileBroadcasting(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()
	bob := connect(t, addr, "bob")
	
	// Ensure bob is registered
	alice.waitEvent("EVT ROOM PRESENCE ENTER bob")
	
	// Disconnect bob abruptly while an event is expected.
	// We'll perform an action that broadcasts to everyone (like a global chat)
	// while bob is in the process of disconnecting.
	
	bob.close() // Close connection
	
	// Server should continue operating and allow alice to continue
	alice.send("CHAT GLOBAL hello")
	resp := alice.response()
	if resp != "OK" {
		t.Fatalf("Expected OK after other client disconnected, got: %s", resp)
	}
}
