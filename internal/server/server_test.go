// server_test.go: end-to-end integration test of the vertical slice. Real TCP
// clients talk to a live Hub. Run with -race to catch concurrency bugs.
package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"the-answer-protocol/internal/game"
)

// startServer runs a Hub on the given world with logs discarded and returns its address.
func startServer(t *testing.T, world *game.World) string {
	return startServerLog(t, world, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// startServerLog is like startServer but sends logs to the given logger, so a
// test can capture them, for example to assert abuse warnings are emitted.
func startServerLog(t *testing.T, world *game.World, log *slog.Logger) string {
	t.Helper()
	hub := NewHub(world, log)
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

// testWorld builds a tiny 2-room world used by the integration tests.
func testWorld() *game.World {
	return &game.World{
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
}

// newTestServer starts a Hub on a tiny 2-room world and returns its address.
func newTestServer(t *testing.T) string {
	t.Helper()
	return startServer(t, testWorld())
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

	bob := connect(t, addr, "bob") // bob enters the square -> alice sees ENTER
	defer bob.close()
	alice.waitEvent("EVT ROOM PRESENCE ENTER bob")

	bob.send("CHAT ROOM hello there")
	if g := bob.response(); g != "OK" {
		t.Fatalf("CHAT = %q", g)
	}
	alice.waitEvent("EVT ROOM CHAT bob hello there")

	alice.send("MOVE north") // alice leaves the square -> bob sees LEAVE
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
	if g := alice.response(); g != "OK taken=item.coin" {
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
	if g := alice.response(); g != "OK dropped=item.coin" {
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

// combatWorld is a one-room world with a weak hostile NPC and a peaceful guard.
func combatWorld() *game.World {
	return &game.World{
		Rooms: map[string]*game.Room{
			"loc.square": {ID: "loc.square", Name: "Square", Description: "a square",
				Exits: map[string]string{},
				Items: map[string]*game.Item{},
				NPCs: map[string]*game.NPC{
					"npc.goblin": {ID: "npc.goblin", Name: "Goblin", Hostile: true, HP: 1, MaxHP: 1},
					"npc.guard":  {ID: "npc.guard", Name: "Guard", Hostile: false, HP: 30, MaxHP: 30},
				},
				Players: map[string]*game.Player{}},
		},
		Players:   map[string]*game.Player{},
		StartRoom: "loc.square",
		Quests:    map[string]*game.QuestDef{},
		Items:     map[string]*game.Item{},
	}
}

func TestCombatAndStatus(t *testing.T) {
	addr := startServer(t, combatWorld())
	alice := connect(t, addr, "alice")
	defer alice.close()

	// a fresh player is healthy at full HP
	alice.send("STATUS")
	if g := alice.response(); !strings.Contains(g, `"hp":100`) || !strings.Contains(g, `"status":"healthy"`) {
		t.Fatalf("STATUS = %q", g)
	}

	// attacking a non-hostile NPC is refused
	alice.send("ATTACK Guard")
	if g := alice.response(); g != "ERR 405 NPC_NOT_HOSTILE" {
		t.Fatalf("ATTACK Guard = %q", g)
	}

	// attacking a missing NPC is refused
	alice.send("ATTACK dragon")
	if g := alice.response(); g != "ERR 404 NPC_NOT_FOUND" {
		t.Fatalf("ATTACK dragon = %q", g)
	}

	// one hit kills the 1-HP goblin -> victory
	alice.send("ATTACK Goblin")
	if g := alice.response(); !strings.Contains(g, `"status":"victory"`) {
		t.Fatalf("ATTACK Goblin = %q", g)
	}

	// the defeated goblin is gone - attacking again returns NPC_NOT_FOUND
	alice.send("ATTACK Goblin")
	if g := alice.response(); g != "ERR 404 NPC_NOT_FOUND" {
		t.Fatalf("re-ATTACK Goblin = %q", g)
	}
}

func TestCombatBroadcast(t *testing.T) {
	addr := startServer(t, combatWorld())
	alice := connect(t, addr, "alice")
	defer alice.close()
	bob := connect(t, addr, "bob")
	defer bob.close()
	alice.waitEvent("EVT ROOM PRESENCE ENTER bob")

	// alice attacks the goblin -> bob (same room) sees the combat event
	alice.send("ATTACK Goblin")
	alice.response()
	bob.waitEvent("EVT ROOM COMBAT alice Goblin")
}

func TestGroups(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()
	bob := connect(t, addr, "bob")
	defer bob.close()
	alice.waitEvent("EVT ROOM PRESENCE ENTER bob")

	// alice creates a group (named after the leader)
	alice.send("GROUP CREATE")
	if g := alice.response(); g != "OK group=alice" {
		t.Fatalf("GROUP CREATE = %q", g)
	}

	// alice invites bob -> bob gets an invite event
	alice.send("GROUP INVITE bob")
	if g := alice.response(); g != "OK invited" {
		t.Fatalf("GROUP INVITE = %q", g)
	}
	bob.waitEvent("EVT GROUP INVITE alice")

	// bob joins -> alice sees the join
	bob.send("GROUP JOIN")
	if g := bob.response(); g != "OK group=alice" {
		t.Fatalf("GROUP JOIN = %q", g)
	}
	alice.waitEvent("EVT GROUP JOIN bob")

	// group chat reaches bob
	alice.send("CHAT GROUP hello team")
	if g := alice.response(); g != "OK" {
		t.Fatalf("CHAT GROUP = %q", g)
	}
	bob.waitEvent("EVT GROUP CHAT alice hello team")

	// bob leaves -> alice sees the leave
	bob.send("GROUP LEAVE")
	if g := bob.response(); g != "OK left" {
		t.Fatalf("GROUP LEAVE = %q", g)
	}
	alice.waitEvent("EVT GROUP LEAVE bob")

	// bob is no longer in a group
	bob.send("CHAT GROUP anyone?")
	if g := bob.response(); g != "ERR 401 NOT_IN_GROUP" {
		t.Fatalf("CHAT GROUP after leave = %q", g)
	}
}

// questWorld is a one-room world with a quest-giver, the fetch target item, and
// the reward item in the catalog.
func questWorld() *game.World {
	return &game.World{
		Rooms: map[string]*game.Room{
			"loc.square": {ID: "loc.square", Name: "Square", Description: "a square",
				Exits: map[string]string{},
				Items: map[string]*game.Item{
					"item.herbs": {ID: "item.herbs", Name: "Healing Herbs"},
				},
				NPCs: map[string]*game.NPC{
					"npc.elder": {ID: "npc.elder", Name: "Elder", Role: "quest-giver", QuestID: "quest.herbs"},
				},
				Players: map[string]*game.Player{}},
		},
		Players:   map[string]*game.Player{},
		StartRoom: "loc.square",
		Quests: map[string]*game.QuestDef{
			"quest.herbs": {ID: "quest.herbs", Description: "Bring herbs", Type: "fetch", TargetID: "item.herbs", Reward: "item.gold"},
		},
		Items: map[string]*game.Item{
			"item.herbs": {ID: "item.herbs", Name: "Healing Herbs"},
			"item.gold":  {ID: "item.gold", Name: "Gold"},
		},
	}
}

func TestQuestFlow(t *testing.T) {
	addr := startServer(t, questWorld())
	alice := connect(t, addr, "alice")
	defer alice.close()

	// request the quest from the elder
	alice.send("QUEST Elder")
	if g := alice.response(); !strings.Contains(g, `"quest_id":"quest.herbs"`) || !strings.Contains(g, `"type":"fetch"`) {
		t.Fatalf("QUEST = %q", g)
	}

	// it shows as active
	alice.send("QUESTS")
	if g := alice.response(); !strings.Contains(g, `"quest.herbs"`) || !strings.Contains(g, `"status":"active"`) {
		t.Fatalf("QUESTS active = %q", g)
	}

	// asking again -> no quest available
	alice.send("QUEST Elder")
	if g := alice.response(); g != "ERR 406 NO_QUEST_AVAILABLE" {
		t.Fatalf("QUEST again = %q", g)
	}

	// do the objective: pick up the herbs
	alice.send("TAKE Healing Herbs")
	if g := alice.response(); g != "OK taken=item.herbs" {
		t.Fatalf("TAKE = %q", g)
	}

	// QUESTS now completes it and grants the reward
	alice.send("QUESTS")
	if g := alice.response(); !strings.Contains(g, `"status":"completed"`) {
		t.Fatalf("QUESTS completed = %q", g)
	}
	alice.send("INVENTORY")
	if g := alice.response(); !strings.Contains(g, `"id":"item.gold"`) {
		t.Fatalf("reward not in inventory: %q", g)
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

// readEvent reads lines until one starts with prefix, then returns it.
func (tc *testClient) readEvent(prefix string) string {
	tc.t.Helper()
	for {
		line, err := tc.readLine()
		if err != nil {
			tc.t.Fatalf("waiting for %q: %v", prefix, err)
		}
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
}

// syncBuffer is a goroutine-safe buffer: the Hub logs from its own goroutine
// while the test reads, so the writes need to be guarded.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestGroupFlow exercises the whole party lifecycle and the group-scoped events.
func TestGroupFlow(t *testing.T) {
	addr := newTestServer(t)
	leader := connect(t, addr, "leader")
	defer leader.close()
	member := connect(t, addr, "member") // leader sees member ENTER
	defer member.close()

	leader.send("GROUP CREATE")
	if g := leader.response(); g != "OK group=leader" {
		t.Fatalf("CREATE = %q", g)
	}

	leader.send("GROUP INVITE member")
	if g := leader.response(); g != "OK invited" {
		t.Fatalf("INVITE = %q", g)
	}
	member.readEvent("EVT GROUP INVITE leader")

	member.send("GROUP JOIN")
	if g := member.response(); g != "OK group=leader" {
		t.Fatalf("JOIN = %q", g)
	}
	leader.readEvent("EVT GROUP JOIN member")

	leader.send("CHAT GROUP hello")
	if g := leader.response(); g != "OK" {
		t.Fatalf("CHAT GROUP = %q", g)
	}
	if e := member.readEvent("EVT GROUP CHAT"); e != "EVT GROUP CHAT leader hello" {
		t.Fatalf("group chat event = %q", e)
	}

	member.send("GROUP LEAVE")
	if g := member.response(); g != "OK left" {
		t.Fatalf("LEAVE = %q", g)
	}
	leader.readEvent("EVT GROUP LEAVE member")
}

// TestGroupDisbandOnLeaderLeave checks that the group dissolves when the leader
// leaves: the remaining member is notified and is no longer in a group.
func TestGroupDisbandOnLeaderLeave(t *testing.T) {
	addr := newTestServer(t)
	leader := connect(t, addr, "leader")
	defer leader.close()
	member := connect(t, addr, "member")
	defer member.close()

	leader.send("GROUP CREATE")
	leader.response()
	leader.send("GROUP INVITE member")
	leader.response()
	member.readEvent("EVT GROUP INVITE")
	member.send("GROUP JOIN")
	member.response()
	leader.readEvent("EVT GROUP JOIN")

	// leader leaves -> group disbands -> member gets a LEAVE for the leader
	leader.send("GROUP LEAVE")
	if g := leader.response(); g != "OK left" {
		t.Fatalf("leader LEAVE = %q", g)
	}
	member.readEvent("EVT GROUP LEAVE leader")

	// member is now group-less, so leaving again is an error
	member.send("GROUP LEAVE")
	if g := member.response(); g != "ERR 401 NOT_IN_GROUP" {
		t.Fatalf("member LEAVE after disband = %q", g)
	}
}

// TestChatSanitizesControlChars makes sure control characters are stripped from
// chat before being broadcast to other players.
func TestChatSanitizesControlChars(t *testing.T) {
	addr := newTestServer(t)
	alice := connect(t, addr, "alice")
	defer alice.close()
	bob := connect(t, addr, "bob")
	defer bob.close()

	alice.send("CHAT GLOBAL hi\x07there") // embedded BEL control char
	if g := alice.response(); g != "OK" {
		t.Fatalf("CHAT = %q", g)
	}

	got := bob.readEvent("EVT GLOBAL CHAT")
	if strings.ContainsAny(got, "\x00\x07\x1b") {
		t.Fatalf("broadcast still has control chars: %q", got)
	}
	if got != "EVT GLOBAL CHAT alice hithere" {
		t.Fatalf("sanitized chat = %q", got)
	}
}

// TestCommandFloodLogged verifies that crossing the per-client command rate
// limit emits a single abuse warning.
func TestCommandFloodLogged(t *testing.T) {
	buf := &syncBuffer{}
	addr := startServerLog(t, testWorld(), slog.New(slog.NewTextHandler(buf, nil)))
	alice := connect(t, addr, "alice")
	defer alice.close()

	for i := 0; i < floodLimit+2; i++ {
		alice.send("LOOK")
		alice.response()
	}
	if !strings.Contains(buf.String(), "possible command flood") {
		t.Fatalf("expected flood warning, log = %q", buf.String())
	}
}

// TestRapidConnectionsLogged verifies that too many connections from one IP emit
// a rapid-connection warning. All test clients dial from 127.0.0.1.
func TestRapidConnectionsLogged(t *testing.T) {
	buf := &syncBuffer{}
	addr := startServerLog(t, testWorld(), slog.New(slog.NewTextHandler(buf, nil)))
	for i := 0; i <= connLimit; i++ { // connLimit+1 connections crosses the limit
		c := connect(t, addr, fmt.Sprintf("u%d", i))
		defer c.close()
	}
	if !strings.Contains(buf.String(), "possible rapid connections") {
		t.Fatalf("expected rapid-connection warning, log = %q", buf.String())
	}
}

// TestRoomItemBroadcast verifies that taking/dropping an item notifies the other
// players in the room with an EVT ROOM ITEM event (the actor is not notified).
func TestRoomItemBroadcast(t *testing.T) {
	world := testWorld()
	world.Rooms["loc.square"].Items["item.coin"] = &game.Item{ID: "item.coin", Name: "Gold Coin"}
	addr := startServer(t, world)

	alice := connect(t, addr, "alice")
	defer alice.close()
	bob := connect(t, addr, "bob") // both spawn in loc.square
	defer bob.close()

	alice.send("TAKE Gold Coin")
	if g := alice.response(); g != "OK taken=item.coin" {
		t.Fatalf("TAKE = %q", g)
	}
	if e := bob.readEvent("EVT ROOM ITEM"); e != "EVT ROOM ITEM TAKEN alice item.coin" {
		t.Fatalf("take event = %q", e)
	}

	alice.send("DROP item.coin")
	if g := alice.response(); g != "OK dropped=item.coin" {
		t.Fatalf("DROP = %q", g)
	}
	if e := bob.readEvent("EVT ROOM ITEM"); e != "EVT ROOM ITEM DROPPED alice item.coin" {
		t.Fatalf("drop event = %q", e)
	}
}
