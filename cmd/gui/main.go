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

//   3. Build the UI with a “connecting” state or status label (see build* helpers below)
	eventsCh := make(chan string)

//   4. Dial the server, with clean error handling.
	conn, err := net.Dial("tcp", addr)

//   5. If connection succeeds, start readLoop.
	if err != nil {
		log.Printf("failed to connect to %s: %v", addr, err)
	} else {

//   6. wire eventsCh → UI updates via fyne.Do(...) / widget.Refresh()
		go readLoop(conn, eventsCh)
	}

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

// buildRoomPanel returns the widget that shows the current room's name,
// description, exits, items, NPCs, and players-in-room. Refreshed on
// every EVT ROOM PRESENCE * and every LOOK response.
func buildRoomPanel() fyne.CanvasObject {
	return nil // TODO: implement
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
func buildActionBar() fyne.CanvasObject {
	return nil // TODO: implement
}

// buildStatusBar returns the top/bottom bar with HP, players in room,
// and players on server. Refreshed on STATUS responses and EVT STATS.
func buildStatusBar() fyne.CanvasObject {
	return nil // TODO: implement
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
