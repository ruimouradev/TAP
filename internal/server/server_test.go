// server_test.go: end-to-end integration test of the vertical slice. Real TCP
// clients talk to a live Hub. Run with -race to catch concurrency bugs.
package server

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"the-answer-protocol/internal/game"
)

// startServer runs a Hub on the given world and returns its listening address.
func startServer(t *testing.T, world *game.World) string {
	t.Helper()
	hub := NewHub(world, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go hub.Run()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			hub.Accept(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

// newTestServer starts a Hub on a tiny 2-room world and returns its address.
func newTestServer(t *testing.T) string {
	t.Helper()
	world := &game.World{
		Rooms: map[string]*game.Room{
			"loc.square": {ID: "loc.square", Name: "Square", Description: "a square",
				Exits: map[string]string{"north": "loc.bakery"},
				Items: map[string]*game.Item{}, NPCs: map[string]*game.NPC{}, Players: map[string]*game.Player{}},
			"loc.bakery": {ID: "loc.bakery", Name: "Bakery", Description: "a bakery",
				Exits: map[string]string{"south": "loc.square"},
				Items: map[string]*game.Item{}, NPCs: map[string]*game.NPC{}, Players: map[string]*game.Player{}},
		},
		Players:   map[string]*game.Player{},
		StartRoom: "loc.square",
		Quests:    map[string]*game.QuestDef{},
	}
	return startServer(t, world)
}

type testClient struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

// connect dials, checks the greeting, and does CONNECT <name>.
func connect(t *testing.T, addr, name string) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tc := &testClient{t: t, conn: conn, r: bufio.NewReader(conn)}
	if g := tc.response(); g != "OK hello proto=1" {
		t.Fatalf("greeting = %q", g)
	}
	tc.send("CONNECT " + name)
	if g := tc.response(); g != "OK connected" {
		t.Fatalf("CONNECT %s = %q", name, g)
	}
	return tc
}

func (tc *testClient) send(s string) { fmt.Fprintf(tc.conn, "%s\n", s) }
func (tc *testClient) close()        { tc.conn.Close() }

func (tc *testClient) readLine() (string, error) {
	tc.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := tc.r.ReadString('\n')
	return strings.TrimRight(line, "\n"), err
}

// response returns the next OK/ERR line, skipping asynchronous EVT lines.
func (tc *testClient) response() string {
	tc.t.Helper()
	for {
		line, err := tc.readLine()
		if err != nil {
			tc.t.Fatalf("response read: %v", err)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "ERR") {
			return line
		}
	}
}

// waitEvent reads until a line contains substr, failing on timeout.
func (tc *testClient) waitEvent(substr string) {
	tc.t.Helper()
	for {
		line, err := tc.readLine()
		if err != nil {
			tc.t.Fatalf("waiting for %q: %v", substr, err)
		}
		if strings.Contains(line, substr) {
			return
		}
	}
}

func TestVerticalSlice(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()

	alice.send("LOOK")
	look := alice.response()
	if !strings.HasPrefix(look, "OK {") || !strings.Contains(look, `"id":"loc.square"`) {
		t.Fatalf("LOOK = %q", look)
	}

	alice.send("MOVE north")
	if g := alice.response(); g != "OK room=loc.bakery" {
		t.Fatalf("MOVE north = %q", g)
	}

	alice.send("MOVE west") // no such exit
	if g := alice.response(); g != "ERR 301 NO_EXIT" {
		t.Fatalf("MOVE west = %q", g)
	}

	alice.send("WHO")
	if g := alice.response(); !strings.Contains(g, `"server":1`) {
		t.Fatalf("WHO = %q", g)
	}

	alice.send("BOGUS")
	if g := alice.response(); !strings.HasPrefix(g, "ERR 400") {
		t.Fatalf("unknown command = %q", g)
	}
}

func TestPresenceAndChat(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()

	bob := connect(t, addr, "bob") // bob enters the square → alice sees ENTER
	defer bob.close()
	alice.waitEvent("EVT ROOM PRESENCE ENTER bob")

	bob.send("CHAT ROOM hello there")
	if g := bob.response(); g != "OK" {
		t.Fatalf("CHAT = %q", g)
	}
	alice.waitEvent("EVT ROOM CHAT bob hello there")

	alice.send("MOVE north") // alice leaves the square → bob sees LEAVE
	if g := alice.response(); g != "OK room=loc.bakery" {
		t.Fatalf("MOVE = %q", g)
	}
	bob.waitEvent("EVT ROOM PRESENCE LEAVE alice")
}

// itemsWorld is a one-room world with an item on the floor and a talking NPC.
func itemsWorld() *game.World {
	return &game.World{
		Rooms: map[string]*game.Room{
			"loc.square": {ID: "loc.square", Name: "Square", Description: "a square",
				Exits: map[string]string{},
				Items: map[string]*game.Item{
					"item.coin": {ID: "item.coin", Name: "Gold Coin"},
				},
				NPCs: map[string]*game.NPC{
					"npc.baker": {ID: "npc.baker", Name: "Baker", Dialogue: "Fresh bread!"},
				},
				Players: map[string]*game.Player{}},
		},
		Players:   map[string]*game.Player{},
		StartRoom: "loc.square",
		Quests:    map[string]*game.QuestDef{},
	}
}

func TestItemsAndTalk(t *testing.T) {
	addr := startServer(t, itemsWorld())
	alice := connect(t, addr, "alice")
	defer alice.close()

	// take by multi-word display name
	alice.send("TAKE Gold Coin")
	if g := alice.response(); g != "OK took item.coin" {
		t.Fatalf("TAKE = %q", g)
	}

	// the item is now in the inventory
	alice.send("INVENTORY")
	if g := alice.response(); !strings.Contains(g, `"id":"item.coin"`) {
		t.Fatalf("INVENTORY = %q", g)
	}

	// and gone from the floor
	alice.send("TAKE Gold Coin")
	if g := alice.response(); g != "ERR 404 ITEM_NOT_FOUND" {
		t.Fatalf("second TAKE = %q", g)
	}

	// drop by ID puts it back on the floor
	alice.send("DROP item.coin")
	if g := alice.response(); g != "OK dropped item.coin" {
		t.Fatalf("DROP = %q", g)
	}

	// dropping it again fails: no longer in the inventory
	alice.send("DROP item.coin")
	if g := alice.response(); g != "ERR 404 ITEM_NOT_IN_INVENTORY" {
		t.Fatalf("second DROP = %q", g)
	}

	// talk to the NPC by name
	alice.send("TALK Baker")
	if g := alice.response(); !strings.Contains(g, `"dialogue":"Fresh bread!"`) {
		t.Fatalf("TALK = %q", g)
	}

	// talk to a missing NPC
	alice.send("TALK ghost")
	if g := alice.response(); g != "ERR 404 NPC_NOT_FOUND" {
		t.Fatalf("TALK ghost = %q", g)
	}
}

func TestDuplicateName(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	dup := &testClient{t: t, conn: conn, r: bufio.NewReader(conn)}
	dup.response() // greeting
	dup.send("CONNECT alice")
	if g := dup.response(); g != "ERR 201 NAME_IN_USE" {
		t.Fatalf("duplicate CONNECT = %q", g)
	}
}
