/*
cmd/gui — Owner: Alexandre.

Fyne-based GUI client for TAP.
Connects directly to the TAP server over TCP (no bridge needed) and
runs two concurrent goroutines:
  - the Fyne event loop on the main goroutine (drives the UI)
  - a network reader goroutine that consumes lines from the server
    and forwards them to the UI via a channel

The UI is composed of panels that the spec requires:
  - room panel (name, description, exits, items, NPCs, players in room)
  - chat panel with three scopes (GLOBAL / ROOM / GROUP)
  - inventory panel
  - action bar (buttons for LOOK, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS)
  - status bar (HP, players in room, players on server)

All commands are formatted via the shared internal/protocol package and
sent over the TCP connection — the GUI holds no game logic, only render
and input wiring.
*/
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"the-answer-protocol/internal/protocol"
)

// main creates the Fyne app, opens the connection to the server, wires
// the read goroutine to the UI, and starts the event loop.
func main() {
//   1. parse server addr from os.Args (default "localhost:4242")
	addr := parseServerAddr(os.Args)

//   2. Create app and Window("TAP")
	a := app.New()
	w := a.NewWindow("TAP")

//   3. Channel to communicate between readLoop (Network thread) and the UI
	eventsCh := make(chan string)

//   4. Dial the server, with clean error handling.
	conn, err := net.Dial("tcp", addr)

//   5. If connection succeeds, start readLoop.
	if err != nil {
		log.Printf("failed to connect to %s: %v", addr, err)
	} else {

//   6. goroutine in background to listen to the server thru conn and wire 
// 		eventsCh → UI updates
		go readLoop(conn, eventsCh)
	}

	//  container.NewBorder(top, bottom, left, right, center)
	w.SetContent(container.NewBorder(buildStatusBar(), buildActionBar(conn), nil, nil, buildRoomPanel()))
//  Shows the window and starts the Main UI Event Loop
	w.ShowAndRun()
}

func parseServerAddr(args []string) string {
	if len(args) > 1 && args[1] != "" {
		return args[1]
	}
	return "localhost:4242"
}

// readLoop reads lines from the server and pushes them into the events
// channel for the UI goroutine to consume. Runs in its own goroutine;
// closes events when the connection ends.
func readLoop(conn net.Conn, events chan<- string) {
	defer close(events)
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	// allow reasonably large lines (increase buffer to 256KB)
	const maxTokenSize = 256 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		// send line to events channel; block if UI is slow
		events <- line
	}

	if err := scanner.Err(); err != nil {
		log.Printf("readLoop: scanner error: %v", err)
	}
}

// sendCommand formats a verb + args via internal/protocol and writes it
// to the connection. Called from button click handlers and the chat input.
func sendCommand(conn net.Conn, verb string, args ...string) error {
	if conn == nil {
		return fmt.Errorf("no connection")
	}

	// Ensure verb is uppercase as per protocol design
	cmd := protocol.Command{Verb: strings.ToUpper(verb), Args: args}
	line := cmd.String() + "\n"
	_, err := conn.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("sendCommand write: %w", err)
	}
	return nil
}

var (
	roomNameValue        *widget.Label
	roomDescriptionValue *widget.Label
	roomExitsValue       *widget.Label
	roomItemsValue       *widget.Label
	roomNPCsValue        *widget.Label
	roomPlayersValue     *widget.Label
)

// buildRoomPanel returns the widget that shows the current room's name,
// description, exits, items, NPCs, and players-in-room. Refreshed on
// every EVT ROOM PRESENCE * and every LOOK response.
func buildRoomPanel() fyne.CanvasObject {
	roomNameValue = widget.NewLabel("--")
	roomNameValue.Wrapping = fyne.TextWrapWord

	roomDescriptionValue = widget.NewLabel("Waiting for LOOK...")
	roomDescriptionValue.Wrapping = fyne.TextWrapWord

	roomExitsValue = widget.NewLabel("--")
	roomExitsValue.Wrapping = fyne.TextWrapWord

	roomItemsValue = widget.NewLabel("--")
	roomItemsValue.Wrapping = fyne.TextWrapWord

	roomNPCsValue = widget.NewLabel("--")
	roomNPCsValue.Wrapping = fyne.TextWrapWord

	roomPlayersValue = widget.NewLabel("--")
	roomPlayersValue.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle("Room", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNameValue,
		widget.NewLabelWithStyle("Description", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomDescriptionValue,
		widget.NewLabelWithStyle("Exits", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomExitsValue,
		widget.NewLabelWithStyle("Items", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomItemsValue,
		widget.NewLabelWithStyle("NPCs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNPCsValue,
		widget.NewLabelWithStyle("Players in room", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomPlayersValue,
	)

	return widget.NewCard("Room", "Current room state", content)
}

// buildChatPanel returns the three-tab chat (GLOBAL / ROOM / GROUP)
// plus the input field that builds the right CHAT command.
func buildChatPanel() fyne.CanvasObject {
	return nil // TODO: implement
}

// buildInventoryPanel returns the inventory list + a DROP button bound
// to the currently selected item.
func buildInventoryPanel() fyne.CanvasObject {
	return nil // TODO: implement
}

// buildActionBar returns the row of buttons that map 1:1 to protocol
// verbs (LOOK, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS).
// Each button calls sendCommand with the appropriate verb.
func buildActionBar(conn net.Conn) fyne.CanvasObject {
	return container.NewHBox(
		widget.NewButton("LOOK", func() { logCommand(sendCommand(conn, "LOOK")) }),
		widget.NewButton("MOVE", func() { logCommand(sendCommand(conn, "MOVE", "north")) }),
		widget.NewButton("TAKE", func() { logCommand(sendCommand(conn, "TAKE", "item")) }),
		widget.NewButton("DROP", func() { logCommand(sendCommand(conn, "DROP", "item")) }),
		widget.NewButton("TALK", func() { logCommand(sendCommand(conn, "TALK", "npc")) }),
		widget.NewButton("ATTACK", func() { logCommand(sendCommand(conn, "ATTACK", "npc")) }),
		widget.NewButton("STATUS", func() { logCommand(sendCommand(conn, "STATUS")) }),
		widget.NewButton("QUEST", func() { logCommand(sendCommand(conn, "QUEST")) }),
		widget.NewButton("QUESTS", func() { logCommand(sendCommand(conn, "QUESTS")) }),
	)
}

// buildStatusBar returns the top/bottom bar with HP, players in room,
// and players on server. Refreshed on STATUS responses and EVT STATS.
func buildStatusBar() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewLabel("HP: --"),
		layout.NewSpacer(),
		widget.NewLabel("Room: --"),
		layout.NewSpacer(),
		widget.NewLabel("Server: --"),
	)
}

func logCommand(err error) {
	if err != nil {
		log.Printf("send command failed: %v", err)
	}
}

// applyResponse routes an OK / ERR response line to the right panel.
// Called from the UI side of the events channel.
func applyResponse(line string) {
	// TODO: implement — branch on the verb of the last command sent
}

// applyEvent routes an EVT line to the right panel (chat / room presence
// / group / stats).
func applyEvent(line string) {
	// TODO: implement — branch on the EVT category (ROOM / GLOBAL / GROUP / STATS)
}
