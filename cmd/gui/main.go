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
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"the-answer-protocol/internal/config"
	"the-answer-protocol/internal/protocol"
)

var (
	// Tracks the last command sent to know how to interpret OK responses
	lastSentVerb string
	// Global connection handle so applyResponse can trigger follow-up commands
	globalConn      net.Conn
	activeChatScope = "GLOBAL"
)

// main creates the Fyne app, opens the connection to the server, wires
// the read goroutine to the UI, and starts the event loop.
func main() {
	//   1. parse server addr from os.Args (default from config.Port())
	addr := parseServerAddr(os.Args)

	//   2. Create app and Window("TAP")
	a := app.New()
	w := a.NewWindow("TAP")

	//   3. Channel to communicate between readLoop (Network thread) and the UI
	// using a buffer of up to 100 messages to avoid frezing readLoop
	eventsCh := make(chan string, 100)

	//   4. Dial the server, with clean error handling.
	conn, err := net.Dial("tcp", addr)

	//   5. If connection succeeds, start readLoop.
	if err != nil {
		// log.Fatalf prints the error and calls os.Exit(1) immediately, 
        // preventing a nil-pointer crash later down the line.
        log.Fatalf("❌ Critical Error: Failed to connect to server at %s: %v", addr, err)
	} else {
		globalConn = conn

		//   6. goroutine in background to listen to the server thru conn and wire
		// 		eventsCh → UI updates
		go readLoop(conn, eventsCh)

		//   7. Consumer loop: process incoming lines and update UI
		// anonymous function
		go func() {
			// blocking loop, sits waiting for readLoop to drop a new string into eventsCh
			for line := range eventsCh {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Update UI in the main thread context
				if strings.HasPrefix(line, "EVT ") {
					applyEvent(line[4:])
				} else {
					applyResponse(line)
				}
			}
		// () tells Go to immediately invoke (execute)
		}()

		//   8. Login Dialog: Force CONNECT before starting
		// makes the primary game window visible
		w.Show()
		// Instantiates an interactive, editable text input field
		usernameEntry := widget.NewEntry()
		// Sets a faint, gray ghost visual clue text inside the empty text box.
		usernameEntry.SetPlaceHolder("YourName")
		// Popup form: []*widget.FormItem{...}: array containing the rows of the form
		loginForm := dialog.NewForm("Welcome to TAP", "Connect", "Cancel", []*widget.FormItem{
			{Text: "Username", Widget: usernameEntry},
		// The callback function. Code that sits dormant until player clicks one of the two buttons.
		}, func(ok bool) {
			if ok && usernameEntry.Text != "" {
				logCommand(sendCommand(conn, "CONNECT", usernameEntry.Text))
			} else {
				a.Quit()
			}
			// The parent window (w) this dialog belongs to
		}, w)
		loginForm.Show()
	}

	// Devides the window in 2 columns and creates a vertical window for the right
	// column, that is split in two panels one on top of the other
	mainCenter := container.NewGridWithColumns(2,
		buildRoomPanel(),
		container.NewVBox(
			buildChatPanel(),
			buildInventoryPanel(),
			buildInteractionPanel(),
		),
	)
	//  container.NewBorder(top, bottom, left, right, center)
	w.SetContent(container.NewBorder(buildStatusBar(), buildActionBar(w, conn), nil, nil, mainCenter))

	// Use Run instead of ShowAndRun since we already called w.Show() to display the dialog
	a.Run()
}

func parseServerAddr(args []string) string {
	if len(args) > 1 && args[1] != "" {
		return args[1]
	}
	return "localhost:" + config.Port()
}

// readLoop reads lines from the server and pushes them into the events
// channel for the UI goroutine to consume. Runs in its own goroutine;
// closes events when the connection ends.
func readLoop(conn net.Conn, events chan<- string) {
	// defered execution LIFO stack
	defer close(events)
	defer conn.Close()

	// reads bytes together until it hits a newline character (\n)
	scanner := bufio.NewScanner(conn)
	// allow reasonably large lines (increase buffer to 256KB)
	// By default, Go's text scanner will crash above 64KB
	const maxTokenSize = 256 * 1024
	// Creates an initial memory allocation slice capable of holding
	//  up to $64KB of text
	buf := make([]byte, 0, 64*1024)
	// Applies this customized memory sizing to the scanner configuration
	// overriding Go's default 64KB limit
	scanner.Buffer(buf, maxTokenSize)

	// Infinite Listening Loop
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

	// Initializes a new struct object. Ensure verb is uppercase as per
	//  protocol design
	cmd := protocol.Command{Verb: strings.ToUpper(verb), Args: args}
	lastSentVerb = cmd.Verb

	// Calls String method. Takes the verb and the arguments array and stitch
	// them together into a string + "/n"
	line := cmd.String() + "\n"
	// transmits raw data down the network wire to the server.
	// []byte(line): converts (casts) text line into a slice of raw bytes.
	// returns the number of bytes successfully written, and an error object
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
	roomItemsList        *widget.List
	roomNPCsList         *widget.List
	roomPlayersValue     *widget.Label
	chatGlobalLog        *widget.Entry
	chatRoomLog          *widget.Entry
	chatGroupLog         *widget.Entry
	chatInput            *widget.Entry
	interactionLog       *widget.Label
	interactionScroll    *container.Scroll
	roomItemsData        []protocol.LookItem
	roomNPCsData         []protocol.LookNPC
	inventoryItems       *widget.List
	inventorySelected    *widget.Label
	statusHPValue        *widget.Label
	statusServerValue    *widget.Label
	statusRoomValue      *widget.Label
	moveButtonsContainer *fyne.Container // New: Container for dynamic move buttons
	inventoryData        []protocol.InventoryItem
	selectedRoomItemID   string
	selectedNPCID        string
	selectedItemID       string
)

// buildRoomPanel returns the widget that shows the current room's name,
// description, exits, items, NPCs, and players-in-room. Refreshed on
// every EVT ROOM PRESENCE * and every LOOK response.
func buildRoomPanel() fyne.CanvasObject {
	// Instantiates a label widget displaying generic placeholder text until
	//  the server synchronizes the live data
	roomNameValue = widget.NewLabel("--")
	// Wraps words into multiple lines when they hit the boundary of the panel container.
	roomNameValue.Wrapping = fyne.TextWrapWord

	roomDescriptionValue = widget.NewLabel("Waiting for LOOK...")
	roomDescriptionValue.Wrapping = fyne.TextWrapWord

	roomExitsValue = widget.NewLabel("--")
	roomExitsValue.Wrapping = fyne.TextWrapWord

	roomItemsList = widget.NewList(
		func() int { return len(roomItemsData) },
		func() fyne.CanvasObject { return widget.NewLabel("item") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(roomItemsData[i].Name)
		},
	)
	roomItemsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(roomItemsData) {
			selectedRoomItemID = roomItemsData[id].ID
		}
	}
	// We give the list a minimum height so it is visible in the VBox
	roomItemsScroll := container.NewVScroll(roomItemsList)
	roomItemsScroll.SetMinSize(fyne.NewSize(0, 100))

	roomNPCsList = widget.NewList(
		func() int { return len(roomNPCsData) },
		func() fyne.CanvasObject { return widget.NewLabel("npc") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(roomNPCsData[i].Name)
		},
	)
	roomNPCsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(roomNPCsData) {
			selectedNPCID = roomNPCsData[id].ID
		}
	}
	// We give the list a minimum height so it is visible in the VBox
	roomNPCsScroll := container.NewVScroll(roomNPCsList)
	roomNPCsScroll.SetMinSize(fyne.NewSize(0, 100))

	roomPlayersValue = widget.NewLabel("--")
	roomPlayersValue.Wrapping = fyne.TextWrapWord

	moveButtonsContainer = container.NewHBox() // Initialize the container for dynamic move buttons

	content := container.NewVBox(
		widget.NewLabelWithStyle("Room", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNameValue,
		widget.NewLabelWithStyle("Description", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomDescriptionValue,
		widget.NewLabelWithStyle("Exits", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomExitsValue,       // This shows the text list of exits
		moveButtonsContainer, // This will hold the buttons for each exit
		widget.NewLabelWithStyle("Items", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomItemsScroll,
		widget.NewLabelWithStyle("NPCs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNPCsScroll,
		widget.NewLabelWithStyle("Players in room", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomPlayersValue,
	)

	// wraps content container inside a widget.NewCard with a border boundary around the
	// panel, global title ("Room") and helper subtitle ("Current room state") at the top.
	return widget.NewCard("Room", "Current room state", content)
}

// buildChatPanel returns the three-tab chat (GLOBAL / ROOM / GROUP)
// plus the input field that builds the right CHAT command.
func buildChatPanel() fyne.CanvasObject {
	chatGlobalLog = widget.NewMultiLineEntry()
	chatGlobalLog.SetText("Global chat will appear here...")
	chatGlobalLog.Disable()
	chatGlobalLog.Wrapping = fyne.TextWrapWord

	chatRoomLog = widget.NewMultiLineEntry()
	chatRoomLog.SetText("Room chat will appear here...")
	chatRoomLog.Disable()
	chatRoomLog.Wrapping = fyne.TextWrapWord

	chatGroupLog = widget.NewMultiLineEntry()
	chatGroupLog.SetText("Group chat will appear here...")
	chatGroupLog.Disable()
	chatGroupLog.Wrapping = fyne.TextWrapWord

	chatInput = widget.NewEntry()
	chatInput.SetPlaceHolder("Type a chat message...")

	chatInput.OnSubmitted = func(s string) {
		if s != "" {
			logCommand(sendCommand(globalConn, "CHAT", activeChatScope, s))
			chatInput.SetText("")
		}
	}

	inputRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("SEND", func() {
			if chatInput.Text != "" {
				logCommand(sendCommand(globalConn, "CHAT", activeChatScope, chatInput.Text))
				chatInput.SetText("")
			}
		}),
		chatInput,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("GLOBAL", chatGlobalLog),
		container.NewTabItem("ROOM", chatRoomLog),
		container.NewTabItem("GROUP", chatGroupLog),
	)

	tabs.OnSelected = func(t *container.TabItem) {
		activeChatScope = t.Text
	}

	return widget.NewCard("Chat", "Global / room / group chat", container.NewVBox(tabs, inputRow))
}

// buildInteractionPanel returns a panel for NPC dialogues and combat feedback.
func buildInteractionPanel() fyne.CanvasObject {
	interactionLog = widget.NewLabel("Interaction details will appear here...")
	interactionLog.SetText("Interaction details will appear here...")
	interactionLog.Wrapping = fyne.TextWrapWord

	interactionScroll = container.NewVScroll(interactionLog)
	interactionScroll.SetMinSize(fyne.NewSize(0, 150))
	return widget.NewCard("Interaction", "Dialogue & Combat feedback", interactionScroll)
}

// buildInventoryPanel returns the inventory list + a DROP button bound
// to the currently selected item.
func buildInventoryPanel() fyne.CanvasObject {
	inventoryData = []protocol.InventoryItem{}
	inventorySelected = widget.NewLabel("Selected item: --")
	inventorySelected.Wrapping = fyne.TextWrapWord

	inventoryItems = widget.NewList(
		func() int { return len(inventoryData) },
		func() fyne.CanvasObject { return widget.NewLabel("item") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(inventoryData[i].Name)
		},
	)

	inventoryItems.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(inventoryData) {
			selectedItemID = inventoryData[id].ID
			inventorySelected.SetText(fmt.Sprintf("Selected item: %s", inventoryData[id].Name))
		}
	}

	inventoryItemsScroll := container.NewVScroll(inventoryItems)
	inventoryItemsScroll.SetMinSize(fyne.NewSize(0, 75)) // Height for ~3 lines

	dropButton := widget.NewButton("DROP", func() {
		if selectedItemID != "" {
			logCommand(sendCommand(globalConn, "DROP", selectedItemID))
		}
	})

	content := container.NewVBox(
		inventoryItemsScroll,
		inventorySelected,
		dropButton,
	)

	return widget.NewCard("Inventory", "Your carried items", content)
}

func promptAndSend(parent fyne.Window, conn net.Conn, title, placeholder, verb string) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(placeholder)

	d := dialog.NewForm(title, "Send", "Cancel", []*widget.FormItem{
		{Text: "Target", Widget: entry},
	}, func(ok bool) {
		if !ok {
			return
		}
		value := strings.TrimSpace(entry.Text)
		if value == "" {
			return
		}
		logCommand(sendCommand(conn, verb, value))
	}, parent)
	d.Show()
}

// buildActionBar returns the row of buttons that map 1:1 to protocol
// verbs (LOOK, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS).
// Each button calls sendCommand with the appropriate verb.
func buildActionBar(parent fyne.Window, conn net.Conn) fyne.CanvasObject {
	return container.NewHBox(
		widget.NewButton("LOOK", func() { logCommand(sendCommand(conn, "LOOK")) }), // LOOK button remains
		// MOVE buttons are now dynamically generated in the room panel
		widget.NewButton("TAKE", func() {
			if selectedRoomItemID != "" {
				logCommand(sendCommand(conn, "TAKE", selectedRoomItemID))
			}
		}),
		widget.NewButton("TALK", func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "TALK", selectedNPCID))
			}
		}),
		widget.NewButton("ATTACK", func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "ATTACK", selectedNPCID))
			}
		}),
		widget.NewButton("STATUS", func() { logCommand(sendCommand(conn, "STATUS")) }),
		widget.NewButton("QUEST", func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "QUEST", selectedNPCID))
			}
		}),
		widget.NewButton("QUESTS", func() { logCommand(sendCommand(conn, "QUESTS")) }),
	)
}

// buildStatusBar returns the top/bottom bar with HP, players in room,
// and players on server. Refreshed on STATUS responses and EVT STATS.
func buildStatusBar() fyne.CanvasObject {
	statusHPValue = widget.NewLabel("HP: --")
	statusRoomValue = widget.NewLabel("Room: --")
	statusServerValue = widget.NewLabel("Server: --")

	return container.NewHBox(
		statusHPValue,
		layout.NewSpacer(),
		statusRoomValue,
		layout.NewSpacer(),
		statusServerValue,
	)
}

// updateMoveButtons dynamically creates buttons for each available exit.
func updateMoveButtons(exits map[string]string) {
	// Clear existing buttons
	moveButtonsContainer.RemoveAll()
	if len(exits) == 0 {
		moveButtonsContainer.Add(widget.NewLabel("No exits available."))
	} else {
		for dir := range exits {
			direction := dir // Capture loop variable for the closure
			btn := widget.NewButton("MOVE "+strings.ToUpper(direction), func() {
				logCommand(sendCommand(globalConn, "MOVE", direction))
			})
			moveButtonsContainer.Add(btn)
		}
	}
	moveButtonsContainer.Refresh() // Important to refresh the container after adding/removing children
}

func logCommand(err error) {
	if err != nil {
		log.Printf("Send command failed: %v", err)
	}
}

// applyResponse routes an OK / ERR response line to the right panel.
// Called from the UI side of the events channel.
func applyResponse(line string) {
	if strings.HasPrefix(line, "ERR ") {
		log.Printf("Server Error: %s", line[4:])
		return
	}

	if !strings.HasPrefix(line, "OK") {
		return
	}

	payload := ""
	if len(line) > 3 {
		payload = line[3:]
	}

	// Protocol improvement: check for specific payload formats regardless of lastSentVerb
	// This handles cases where multiple commands were sent in rapid succession
	if strings.HasPrefix(payload, "room=") {
		roomID := strings.TrimPrefix(payload, "room=")
		statusRoomValue.SetText("Room: " + roomID)
		// Auto-refresh look when entering a new room
		logCommand(sendCommand(globalConn, "LOOK"))
		return
	}

	switch lastSentVerb {
	case "CONNECT":
		// Once connected, sync the UI state immediately
		logCommand(sendCommand(globalConn, "LOOK"))
	case "TAKE", "DROP":
		if lastSentVerb == "DROP" {
			inventoryItems.UnselectAll()
			inventorySelected.SetText("Selected item: --")
			selectedItemID = ""
		}
		if lastSentVerb == "TAKE" {
			roomItemsList.UnselectAll()
			selectedRoomItemID = ""
		}
		logCommand(sendCommand(globalConn, "LOOK"))
	case "LOOK":
		var data protocol.LookResponse
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			roomNameValue.SetText(data.Room.Name)
			roomDescriptionValue.SetText(data.Room.Description)

			exits := []string{}
			for dir := range data.Room.Exits {
				exits = append(exits, dir)
			}
			roomExitsValue.SetText(strings.Join(exits, ", "))
			statusRoomValue.SetText("Room: " + data.Room.ID)

			roomItemsData = data.Items
			roomItemsList.UnselectAll()
			roomItemsList.Refresh()

			roomNPCsData = data.NPCs
			roomNPCsList.UnselectAll()
			roomNPCsList.Refresh()

			roomPlayersValue.SetText(strings.Join(data.Players, ", "))

			updateMoveButtons(data.Room.Exits)

			// Chain refresh: LOOK success triggers INVENTORY sync
			logCommand(sendCommand(globalConn, "INVENTORY"))
		}
	case "STATUS":
		var data protocol.StatusResponse
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			statusHPValue.SetText(fmt.Sprintf("HP: %d/%d", data.HP, data.MaxHP))
		}
	case "INVENTORY":
		var items []protocol.InventoryItem
		if err := json.Unmarshal([]byte(payload), &items); err == nil {
			inventoryData = items
			inventoryItems.Refresh()
			// Chain refresh: INVENTORY success triggers STATUS sync
			logCommand(sendCommand(globalConn, "STATUS"))
		}
	case "ATTACK":
		var data protocol.AttackResponse
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			formatted := fmt.Sprintf("[Combat] You dealt %d damage. Target HP: %d/%d. Status: %s\n",
				data.Damage, data.TargetHP, data.TargetHP+data.Damage, data.Status) // Simplified calculation for display
			currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
			if data.Counter > 0 {
				formatted += fmt.Sprintf("[Combat] NPC countered for %d damage! Your HP: %d\n", data.Counter, data.AttackerHP)
			}
			interactionLog.SetText(currentText + formatted)
			interactionScroll.ScrollToBottom()
			// Also refresh status bar HP
			statusHPValue.SetText(fmt.Sprintf("HP: %d/--", data.AttackerHP))

			roomNPCsList.UnselectAll()
			selectedNPCID = ""
		}
	case "TALK":
		var data protocol.TalkResponse
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
			formatted := fmt.Sprintf("[%s]: \"%s\"\n", data.NPC, data.Dialogue)
			interactionLog.SetText(currentText + formatted)
			interactionScroll.ScrollToBottom()

			roomNPCsList.UnselectAll()
			selectedNPCID = ""
		}
	case "QUEST":
		var data protocol.QuestResponse
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
			formatted := fmt.Sprintf("[Quest] %s\nTarget: %s\nReward: %s\nDesc: %s\n",
				data.QuestID, data.Target, data.Reward, data.Description)
			interactionLog.SetText(currentText + formatted)
			interactionScroll.ScrollToBottom()

			roomNPCsList.UnselectAll()
			selectedNPCID = ""
		}
	case "QUESTS":
		var data []protocol.QuestsEntry
		if err := json.Unmarshal([]byte(payload), &data); err == nil {
			currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
			formatted := "Your Quests:\n"
			for _, q := range data {
				formatted += fmt.Sprintf("- [%s] %s: %s\n", q.State, q.QuestID, q.Description)
			}
			interactionLog.SetText(currentText + formatted)
			interactionScroll.ScrollToBottom()
		}
	}
}

// applyEvent routes an EVT line to the right panel (chat / room presence
// / group / stats).
func applyEvent(line string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return
	}

	category := parts[0]
	data := parts[1]

	switch category {
	case "CHAT":
		handleChatEvent(data)
	case "STATS":
		statusServerValue.SetText("Server: " + data)
	case "ROOM":
		// If room presence changes, we usually trigger a LOOK to refresh the panel
		// but we can also just log it to the room chat.
		chatRoomLog.SetText(chatRoomLog.Text + "\n" + "[System] " + data)
	}
}

func handleChatEvent(payload string) {
	// Format: <SCOPE> <sender> <message>
	parts := strings.SplitN(payload, " ", 3)
	if len(parts) < 3 {
		return
	}

	scope := parts[0]
	sender := parts[1]
	msg := parts[2]

	formatted := fmt.Sprintf("[%s]: %s\n", sender, msg)

	switch scope {
	case "GLOBAL":
		chatGlobalLog.SetText(chatGlobalLog.Text + formatted)
		scrollBottom(chatGlobalLog)
	case "ROOM":
		chatRoomLog.SetText(chatRoomLog.Text + formatted)
		scrollBottom(chatRoomLog)
	case "GROUP":
		chatGroupLog.SetText(chatGroupLog.Text + formatted)
		scrollBottom(chatGroupLog)
	}
}

func scrollBottom(e *widget.Entry) {
	// Helper to keep chat logs visible at the bottom
	e.CursorRow = len(strings.Split(e.Text, "\n"))
	e.Refresh()
}
